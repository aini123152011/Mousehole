"""显式准备合成身份、迁移与本轮服务，执行真实授权工作台浏览器验收。"""

from __future__ import annotations

import argparse
import base64
import os
from pathlib import Path
import socket
import subprocess
import sys
import tempfile
import urllib.error
import uuid

from tooling import ROOT, check_go_version, check_web_versions
from verify_restart import (
    build_binary,
    load_environment,
    loopback_url,
    provision,
    safe_run,
    start,
    stop,
    validation_log,
    wait_ready,
)


def fixture_id(environment: dict[str, str], key: str) -> str:
    """接受显式标准 UUID，未设置时每轮生成；不复用或恢复被撤销的成员。"""
    value = environment.get(key, str(uuid.uuid4()))
    try:
        if str(uuid.UUID(value)) != value:
            raise ValueError()
    except ValueError as error:
        raise RuntimeError(key + " 必须是标准小写 UUID。") from error
    return value


def free_port(port: int, description: str) -> None:
    """仅检查本轮所需端口；已有监听立即失败，不终止或复用其他服务。"""
    with socket.socket() as allocated:
        if os.name == "nt":
            allocated.setsockopt(socket.SOL_SOCKET, socket.SO_EXCLUSIVEADDRUSE, 1)
        try:
            allocated.bind(("127.0.0.1", port))
        except OSError as error:
            raise RuntimeError(description + "端口已被占用；请显式选择空闲测试环境。") from error


def fixture_environment(configured: dict[str, str]) -> dict[str, str]:
    """只初始化合成测试值；开发数据库与模式必须由调用者明确提供。"""
    if configured.get("MOUSEHOLE_MODE") != "development" or not configured.get("MOUSEHOLE_DATABASE_URL"):
        raise RuntimeError("浏览器验收必须显式设置 development 模式和专用开发数据库。")
    environment = dict(configured)
    # 本轮检查要求 Playwright 自建 Web，不复用检查端口后才出现的其他开发服务。
    environment["CI"] = "1"
    defaults = {
        "MOUSEHOLE_LISTEN_ADDR": "127.0.0.1:8080",
        "MOUSEHOLE_PUBLIC_URL": "http://127.0.0.1:8080",
        "MOUSEHOLE_WEB_ORIGIN": "http://127.0.0.1:5173",
        "MOUSEHOLE_TEST_OIDC_LISTEN_ADDR": "127.0.0.1:19090",
        "MOUSEHOLE_OIDC_CLIENT_ID": "mousehole-browser-check",
        "MOUSEHOLE_OIDC_CLIENT_SECRET": uuid.uuid4().hex,
        "MOUSEHOLE_AUTH_KEY": base64.b64encode(os.urandom(32)).decode("ascii"),
        "MOUSEHOLE_TEST_OIDC_SUBJECT": "browser-" + uuid.uuid4().hex,
    }
    for key, value in defaults.items():
        environment.setdefault(key, value)
    issuer_address = environment["MOUSEHOLE_TEST_OIDC_LISTEN_ADDR"]
    environment.setdefault("MOUSEHOLE_OIDC_ISSUER", "http://" + issuer_address)
    loopback_url(environment["MOUSEHOLE_OIDC_ISSUER"])
    if (
        environment["MOUSEHOLE_LISTEN_ADDR"] != "127.0.0.1:8080"
        or environment["MOUSEHOLE_PUBLIC_URL"] != "http://127.0.0.1:8080"
        or environment["MOUSEHOLE_WEB_ORIGIN"] != "http://127.0.0.1:5173"
        or environment["MOUSEHOLE_OIDC_ISSUER"] != "http://" + issuer_address
        or not issuer_address.startswith("127.0.0.1:")
    ):
        raise RuntimeError("浏览器夹具必须匹配现有 Vite 的 8080 API、5173 Web 和独立 loopback issuer。")
    for key in ("MOUSEHOLE_E2E_ACTOR_ID", "MOUSEHOLE_E2E_PROJECT_ID", "MOUSEHOLE_E2E_REVOKE_PROJECT_ID"):
        environment[key] = fixture_id(environment, key)
    if environment["MOUSEHOLE_E2E_PROJECT_ID"] == environment["MOUSEHOLE_E2E_REVOKE_PROJECT_ID"]:
        raise RuntimeError("撤销测试项目必须与主项目不同。")
    return environment


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--environment-file", type=Path, help="显式 UTF-8 JSON 环境映射，不自动读取 .env")
    args = parser.parse_args()
    environment = fixture_environment(load_environment(args.environment_file))
    issuer_port = int(environment["MOUSEHOLE_TEST_OIDC_LISTEN_ADDR"].rsplit(":", 1)[1])
    if issuer_port in (8080, 5173):
        raise RuntimeError("合成 issuer 不能占用 API 或 Web 的端口。")
    for port, description in ((8080, "API "), (5173, "Web "), (issuer_port, "合成 issuer ")):
        free_port(port, description)
    go, pnpm = check_go_version(), check_web_versions()
    log_directory = ROOT / ".local/validation"
    log_directory.mkdir(parents=True, exist_ok=True)
    log_path = log_directory / ("authorization-e2e-" + uuid.uuid4().hex + ".log")
    print("本轮诊断日志：" + str(log_path.relative_to(ROOT)), flush=True)
    with tempfile.TemporaryDirectory(prefix="mousehole-authorization-e2e-") as directory:
        output = Path(directory)
        with validation_log(log_path, environment) as log:
            api_binary = build_binary(go, "api", output, environment, log)
            admin_binary = build_binary(go, "admin", output, environment, log)
            issuer_binary = build_binary(go, "test-oidc", output, environment, log)
            migrate_binary = build_binary(go, "migrate", output, environment, log)
            safe_run([str(migrate_binary), "up"], environment, log, "显式迁移专用开发数据库")
            # 两个项目共用真实 issuer/subject；撤销第二项目不会影响其他验收的主项目。
            for key, name in (
                ("MOUSEHOLE_E2E_PROJECT_ID", "端到端主项目"),
                ("MOUSEHOLE_E2E_REVOKE_PROJECT_ID", "端到端撤销项目"),
            ):
                provision(admin_binary, environment, environment["MOUSEHOLE_E2E_ACTOR_ID"], environment[key], name, log)
            environment["MOUSEHOLE_E2E_ADMIN_BIN"] = str(admin_binary)
            issuer_process: subprocess.Popen | None = None
            api_process: subprocess.Popen | None = None
            try:
                issuer_process = start(issuer_binary, environment, log)
                wait_ready(issuer_process, environment["MOUSEHOLE_OIDC_ISSUER"] + "/.well-known/openid-configuration", "合成 issuer")
                api_process = start(api_binary, environment, log)
                wait_ready(api_process, environment["MOUSEHOLE_PUBLIC_URL"] + "/healthz", "浏览器验收 API")
                # Playwright 拥有 Web 生命周期；不保存 trace，以免诊断产物包含 Cookie/CSRF。
                safe_run([pnpm, "e2e"], environment, log, "执行真实授权工作台浏览器检查", ROOT / "web", timeout=600)
            finally:
                stop(api_process)
                stop(issuer_process)
    print("真实浏览器验收通过；主项目与撤销项目独立，历史数据保留。")


if __name__ == "__main__":
    try:
        main()
    except KeyboardInterrupt:
        print("浏览器验收已停止。", file=sys.stderr)
        raise SystemExit(130)
    except (RuntimeError, OSError, subprocess.SubprocessError, urllib.error.URLError, UnicodeError) as error:
        # 禁止格式化 subprocess 异常；其 command/env 可能包含数据库与认证秘密。
        message = str(error) if isinstance(error, RuntimeError) else "依赖检查失败；请检查本轮诊断日志。"
        print("浏览器验收失败：" + message, file=sys.stderr)
        raise SystemExit(1) from error
