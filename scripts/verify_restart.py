"""通过真实登录与独立 API 重启验证旧草稿及授权切片的冻结回执。"""

from __future__ import annotations

import argparse
import base64
from contextlib import contextmanager
from datetime import datetime, timedelta, timezone
import http.cookiejar
import ipaddress
import json
import os
from pathlib import Path
import re
import signal
import socket
import subprocess
import sys
import tempfile
import time
import urllib.error
import urllib.request
import urllib.parse
import uuid
from typing import BinaryIO, Iterator

from tooling import ROOT, check_go_version


class NoRedirect(urllib.request.HTTPRedirectHandler):
    """保留每一步重定向，便于检查 state 回调且不要求另起 Web 服务。"""

    def redirect_request(self, request, file_pointer, code, message, headers, new_url):
        return None


def allocate_port() -> int:
    """向操作系统申请当前空闲 loopback 端口；服务随后立即绑定。"""
    with socket.socket() as allocated:
        if os.name == "nt":
            allocated.setsockopt(socket.SOL_SOCKET, socket.SO_EXCLUSIVEADDRUSE, 1)
        allocated.bind(("127.0.0.1", 0))
        return allocated.getsockname()[1]


def load_environment(path: Path | None) -> dict[str, str]:
    """仅合并调用者显式指定的环境映射，不读取 .env，也不输出配置内容。"""
    environment = dict(os.environ)
    if path is None:
        return environment
    try:
        configured = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, UnicodeError, json.JSONDecodeError) as error:
        raise RuntimeError("显式环境映射无法读取或不是有效 UTF-8 JSON。") from error
    if not isinstance(configured, dict) or any(
        not isinstance(key, str)
        or not key.startswith("MOUSEHOLE_")
        or not isinstance(value, str)
        or "\x00" in value
        for key, value in configured.items()
    ):
        raise RuntimeError("环境映射必须只包含 MOUSEHOLE_ 开头的字符串键值。")
    environment.update(configured)
    return environment


def safe_run(
    arguments: list[str],
    environment: dict[str, str],
    log,
    description: str,
    cwd: Path = ROOT,
    capture: bool = False,
    timeout: int = 120,
) -> str:
    """参数数组不经 shell；异常只报告步骤与退出码，禁止回显参数和原始 stderr。"""
    print(description, flush=True)
    process: subprocess.Popen | None = None
    try:
        process = subprocess.Popen(
            arguments,
            cwd=cwd,
            env=environment,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            start_new_session=os.name != "nt",
            creationflags=(subprocess.CREATE_NO_WINDOW | subprocess.CREATE_NEW_PROCESS_GROUP) if os.name == "nt" else 0,
        )
        stdout, stderr = process.communicate(timeout=timeout)
    except (subprocess.TimeoutExpired, KeyboardInterrupt) as error:
        stop_tree(process)
        if process is not None:
            try:
                stdout, stderr = process.communicate(timeout=10)
                log.write(redact_output(stdout + stderr, environment))
                log.flush()
            except subprocess.TimeoutExpired:
                process.kill()
                # 若依赖没有归还管道，停止等待并明确失败，不因收尾长时间阻塞协作。
                if process.stdout is not None:
                    process.stdout.close()
                if process.stderr is not None:
                    process.stderr.close()
        if isinstance(error, KeyboardInterrupt):
            raise
        raise RuntimeError(f"{description}超时；请检查本轮诊断日志。") from error
    except OSError as error:
        raise RuntimeError(f"{description}未能完成；请检查本轮诊断日志。") from error
    log.write(redact_output(stderr + (b"" if capture else stdout), environment))
    log.flush()
    if process.returncode != 0:
        raise RuntimeError(f"{description}失败（退出码 {process.returncode}）；请检查本轮诊断日志。")
    return stdout.decode("utf-8") if capture else ""


def redact_output(output: bytes, environment: dict[str, str]) -> bytes:
    """测试依赖输出入日志前移除已知 DSN 和认证秘密，不把原始异常带到用户侧。"""
    for key in (
        "MOUSEHOLE_DATABASE_URL", "MOUSEHOLE_TEST_DATABASE_URL",
        "MOUSEHOLE_OIDC_CLIENT_SECRET", "MOUSEHOLE_AUTH_KEY",
    ):
        value = environment.get(key)
        if value:
            output = output.replace(value.encode("utf-8"), "[已脱敏]".encode("utf-8"))
            if "DATABASE_URL" in key:
                try:
                    password = urllib.parse.urlsplit(value).password
                    if password:
                        output = output.replace(urllib.parse.unquote(password).encode("utf-8"), "[已脱敏]".encode("utf-8"))
                except ValueError:
                    pass
    text = output.decode("utf-8", errors="replace")
    text = re.sub(r"\x1b\[[0-?]*[ -/]*[@-~]", "", text)
    # Playwright 的网络错误可能附带请求头；诊断只保留字段名，不保留会话值。
    text = re.sub(
        r"(?im)^([ \t-]*(?:cookie|set-cookie|x-csrf-token|authorization)\s*:).*$",
        r"\1 [已脱敏]", text,
    )
    text = re.sub(
        r'(?i)(["\'](?:csrf_token|access_token|id_token|client_secret)["\']\s*:\s*["\'])[^"\']*',
        r"\1[已脱敏]", text,
    )
    text = re.sub(r"(?i)([?&](?:code|state|access_token|id_token)=)[^&\s\"']*", r"\1[已脱敏]", text)
    return text.encode("utf-8")


@contextmanager
def validation_log(path: Path, environment: dict[str, str]) -> Iterator[BinaryIO]:
    """保留本轮可复核日志；异常或正常收尾均脱敏服务输出后再交付诊断。"""
    with path.open("w+b") as log:
        try:
            yield log
        finally:
            log.flush()
            log.seek(0)
            safe_output = redact_output(log.read(), environment)
            log.seek(0)
            log.truncate()
            log.write(safe_output)


def stop_tree(process: subprocess.Popen | None) -> None:
    """超时只终止本脚本创建的子进程树，包括其 Playwright Web 子进程。"""
    if process is None:
        return
    if os.name == "nt":
        if process.poll() is None:
            subprocess.run(
                ["taskkill", "/PID", str(process.pid), "/T", "/F"],
                stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
                creationflags=subprocess.CREATE_NO_WINDOW, timeout=10,
            )
    else:
        try:
            os.killpg(process.pid, signal.SIGKILL)
        except ProcessLookupError:
            pass


def build_binary(
    go: str, name: str, output: Path, environment: dict[str, str], log
) -> Path:
    """只在本轮临时目录构建固定入口，不安装全局工具或改写 PATH。"""
    binary = output / (name + (".exe" if os.name == "nt" else ""))
    safe_run(
        [go, "build", "-o", str(binary), "./cmd/" + name],
        environment,
        log,
        "构建验收入口：" + name,
        ROOT / "backend",
    )
    return binary


def provision(
    binary: Path,
    environment: dict[str, str],
    actor_id: str,
    project_id: str,
    project_name: str,
    log,
) -> None:
    """预置本轮合成成员；既有撤销状态或异参冲突均失败，不重置历史。"""
    safe_run(
        [
            str(binary), "provision-user",
            "--database-url", environment["MOUSEHOLE_DATABASE_URL"],
            "--issuer", environment["MOUSEHOLE_OIDC_ISSUER"],
            "--subject", environment["MOUSEHOLE_TEST_OIDC_SUBJECT"],
            "--user-id", actor_id, "--display-name", "端到端验收用户",
            "--project-id", project_id, "--project-name", project_name,
            "--role", "analyst",
        ],
        environment,
        log,
        "预置合成项目成员",
    )


def loopback_url(value: str) -> None:
    """验收网络仅允许字面 loopback HTTP 地址，避免目标或代理网络被误调用。"""
    try:
        parsed = urllib.parse.urlsplit(value)
        valid = (
            parsed.scheme == "http"
            and parsed.hostname is not None
            and ipaddress.ip_address(parsed.hostname).is_loopback
            and parsed.username is None
            and parsed.password is None
            and parsed.port is not None
            and 1 <= parsed.port <= 65535
        )
    except ValueError:
        valid = False
    if not valid:
        raise RuntimeError("验收请求必须使用显式 loopback HTTP 地址。")


def request(
    opener: urllib.request.OpenerDirector,
    url: str,
    method: str = "GET",
    body: dict | None = None,
    headers: dict[str, str] | None = None,
) -> tuple[int, dict, urllib.response.addinfourl]:
    """发送单次请求；PUT 未确认时绝不自动生成新 ID 或重发。"""
    loopback_url(url)
    data = json.dumps(body, ensure_ascii=False).encode("utf-8") if body is not None else None
    request_headers = dict(headers or {})
    if data is not None:
        request_headers["Content-Type"] = "application/json"
    http_request = urllib.request.Request(
        url, data=data, headers=request_headers, method=method
    )
    try:
        response = opener.open(http_request, timeout=5)
    except urllib.error.HTTPError as error:
        response = error
    with response:
        payload: dict = {}
        if response.headers.get_content_type() == "application/json":
            payload = json.load(response)
            if not isinstance(payload, dict):
                raise RuntimeError("验收接口未返回 JSON 对象。")
    return response.status, payload, response


def wait_ready(process: subprocess.Popen, url: str, description: str) -> None:
    """等待本测试拥有的进程就绪；退出或超时都保留失败证据。"""
    loopback_url(url)
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
    deadline = time.monotonic() + 20
    while time.monotonic() < deadline and process.poll() is None:
        try:
            with opener.open(url, timeout=1) as response:
                if response.status == 200 and process.poll() is None:
                    return
        except (OSError, urllib.error.URLError):
            pass
        time.sleep(0.1)
    stop(process)
    raise RuntimeError(f"{description}未能启动；请检查迁移、数据库和日志。")


def start(binary: Path, environment: dict[str, str], log) -> subprocess.Popen:
    """只启动本检查构建的二进制，调用者负责等待和停止。"""
    return subprocess.Popen(
        [str(binary)], env=environment, stdout=log, stderr=log,
        creationflags=subprocess.CREATE_NO_WINDOW if os.name == "nt" else 0,
    )


def stop(process: subprocess.Popen | None) -> None:
    """只终止本检查拥有的进程，不影响已有开发服务。"""
    if process is not None and process.poll() is None:
        process.terminate()
        try:
            process.wait(timeout=10)
        except subprocess.TimeoutExpired:
            process.kill()
            process.wait(timeout=5)


def login(opener: urllib.request.OpenerDirector, api_base: str) -> dict:
    """逐步完成浏览器等价的登录跳转，并返回已校验 session 投影。"""
    status, _, response = request(opener, api_base + "/api/v1/auth/login")
    if status != 302 or not response.headers.get("Location"):
        raise RuntimeError("OIDC 登录入口未返回授权跳转。")
    status, _, response = request(opener, response.headers["Location"])
    if status != 302 or not response.headers.get("Location"):
        raise RuntimeError("合成 issuer 未返回带 state 的回调跳转。")
    status, _, response = request(opener, response.headers["Location"])
    if status != 303 or not response.headers.get("Location"):
        raise RuntimeError("OIDC 回调未创建服务端会话。")
    status, session, _ = request(opener, api_base + "/api/v1/session")
    if status != 200 or not isinstance(session.get("csrf_token"), str):
        raise RuntimeError("登录后无法取得会话绑定 CSRF。")
    return session


def submit_scope(
    opener: urllib.request.OpenerDirector,
    base: str,
    project_id: str,
    headers: dict[str, str],
    body: dict,
    expected_type: str,
) -> dict:
    """先确定命令 ID，仅发送一次；受理失败不换 ID 重发或伪造确认结果。"""
    command_id = str(uuid.uuid4())
    url = f"{base}/api/v1/projects/{project_id}/scope-commands/{command_id}"
    status, receipt, _ = request(opener, url, "PUT", body, headers)
    result = receipt.get("result")
    if (
        status != 201
        or receipt.get("command_id") != command_id
        or receipt.get("project_id") != project_id
        or receipt.get("kind") != body["kind"]
        or receipt.get("state") != "accepted"
        or not isinstance(receipt.get("accepted_at"), str)
        or not isinstance(result, dict)
        or result.get("type") != expected_type
    ):
        raise RuntimeError("新领域首次受理回执不符合合同。")
    return {"url": url, "body": body, "receipt": receipt}


def scope_admin(
    binary: Path,
    environment: dict[str, str],
    project_id: str,
    operation: str,
    command_id: str,
    version_id: str,
    revision: str,
    log,
) -> tuple[list[str], dict]:
    """受控 CLI 显式绑定 ID、引用和旧 revision；成功回执仅保留内存供重放比较。"""
    arguments = [
        str(binary), "scope", operation,
        "--database-url", environment["MOUSEHOLE_DATABASE_URL"],
        "--project-id", project_id, "--command-id", command_id,
        "--operator-ref", "restart-validation", "--expected-policy-revision", revision,
        "--version-id", version_id,
    ]
    output = safe_run(arguments, environment, log, "管理授权状态：" + operation, capture=True)
    receipt = json.loads(output)
    if not isinstance(receipt, dict) or receipt.get("command_id") != command_id:
        raise RuntimeError("授权管理 CLI 未返回原命令 JSON 回执。")
    return arguments, receipt


def prepare_scope_history(
    opener: urllib.request.OpenerDirector,
    api_base: str,
    project_id: str,
    session: dict,
    admin_binary: Path,
    environment: dict[str, str],
    log,
) -> dict:
    """创建合成资产与候选、激活并静态检查，再撤销；不连接任何目标网络。"""
    headers = {"Origin": api_base, "X-CSRF-Token": session["csrf_token"]}
    project_base = f"{api_base}/api/v1/projects/{project_id}"
    domain = "restart-" + uuid.uuid4().hex + ".invalid"
    imported = submit_scope(
        opener, api_base, project_id, headers,
        {
            "kind": "import_assets",
            "payload": {
                "entries": [
                    {"kind": "domain", "value": domain.upper() + "."},
                    {"kind": "domain", "value": domain},
                    {"kind": "url", "value": "HTTP://" + domain + ":80"},
                ],
                "source": "manual", "notes": "纯合成重启验收来源声明", "external_ref": "",
            },
        },
        "assets_imported",
    )
    result = imported["receipt"]["result"]
    if result.get("unique_count") != "2" or result.get("duplicate_count") != "1":
        raise RuntimeError("重启验收资产未按统一域名和 URL 合同去重。")
    now = datetime.now(timezone.utc)
    content = {
        "title": "纯合成重启验收授权", "basis_ref": "restart-validation", "contact": "合成验收联系人",
        "valid_from": (now - timedelta(minutes=5)).isoformat().replace("+00:00", "Z"),
        "expires_at": (now + timedelta(hours=1)).isoformat().replace("+00:00", "Z"),
        "algorithm_version": "target-v1", "policy_version": "policy-v1", "catalog_version": "precheck-v1",
        "max_operation_level": "non_destructive", "redirect_policy": "deny", "credential_policy": "none",
        "budget": {"max_requests": "10", "max_concurrency": "1", "max_requests_per_second": "1"},
        "entries": [{
            "kind": "ip", "value": "10.42.0.9", "include_subdomains": False,
            "operations": ["tcp_connect"], "ports": [443], "resolved_ip_cidrs": [],
        }],
    }
    candidate = submit_scope(
        opener, api_base, project_id, headers,
        {"kind": "create_authorization_version", "payload": content}, "authorization_version",
    )
    version = candidate["receipt"]["result"].get("version")
    if not isinstance(version, dict) or version.get("state") != "candidate" or not isinstance(version.get("id"), str):
        raise RuntimeError("保存的授权不是只读候选版本。")
    version_id = version["id"]
    precheck_body = {
        "kind": "precheck_target",
        "payload": {
            "authorization_version_id": version_id, "operation": "tcp_connect",
            "target": "10.42.0.9", "parameters": {"port": 443},
        },
    }
    candidate_check = submit_scope(opener, api_base, project_id, headers, precheck_body, "precheck")
    if candidate_check["receipt"]["result"]["decision"].get("outcome") != "deny":
        raise RuntimeError("未激活候选版本没有默认拒绝。")
    status, before, _ = request(opener, project_base + "/policy")
    if status != 200 or not isinstance(before.get("project_revision"), str):
        raise RuntimeError("无法读取激活前的当前策略 revision。")
    activate_arguments, activate_receipt = scope_admin(
        admin_binary, environment, project_id, "activate", str(uuid.uuid4()), version_id,
        before["project_revision"], log,
    )
    active_check = submit_scope(opener, api_base, project_id, headers, precheck_body, "precheck")
    decision = active_check["receipt"]["result"].get("decision")
    if (
        not isinstance(decision, dict)
        or decision.get("outcome") != "requires_runtime_checks"
        or not isinstance(decision.get("checked_at"), str)
        or not isinstance(decision.get("project_revision"), str)
        or not decision.get("runtime_checks")
    ):
        raise RuntimeError("完整私网静态匹配未返回时间、revision 与运行时待检项。")
    status, active_policy, _ = request(opener, project_base + "/policy")
    if status != 200 or active_policy.get("active_version_id") != version_id:
        raise RuntimeError("管理激活后当前授权指针不正确。")
    revoke_arguments, revoke_receipt = scope_admin(
        admin_binary, environment, project_id, "revoke", str(uuid.uuid4()), version_id,
        active_policy["project_revision"], log,
    )
    # 当前规则已变化后重启，原命令必须保留历史，而不是按撤销状态重算。
    status, assets, _ = request(opener, project_base + "/assets")
    if status != 200 or assets.get("page_count") != "2":
        raise RuntimeError("重启前无法读取两项规范化项目资产。")
    status, revoked_version, _ = request(opener, project_base + "/authorization-versions/" + version_id)
    if status != 200 or revoked_version.get("state") != "revoked" or revoked_version.get("content") != version["content"]:
        raise RuntimeError("撤销改变了不可变授权内容或状态不正确。")
    return {
        "commands": [imported, candidate, candidate_check, active_check],
        "assets": assets, "version": revoked_version, "decision": decision,
        "precheck_body": precheck_body,
        "admin_commands": [(activate_arguments, activate_receipt), (revoke_arguments, revoke_receipt)],
    }


def verify_scope_history(
    opener: urllib.request.OpenerDirector,
    api_base: str,
    project_id: str,
    session: dict,
    history: dict,
) -> None:
    """重启后逐字段比较新对象和冻结回执；旧决策不得被当前撤销状态改写。"""
    headers = {"Origin": api_base, "X-CSRF-Token": session["csrf_token"]}
    project_base = f"{api_base}/api/v1/projects/{project_id}"
    for command in history["commands"]:
        status, recovered, _ = request(opener, command["url"])
        if status != 200 or recovered != command["receipt"]:
            raise RuntimeError("新领域进程重启查询改变了原命令回执。")
        status, replayed, _ = request(opener, command["url"], "PUT", command["body"], headers)
        if status != 200 or replayed != command["receipt"]:
            raise RuntimeError("新领域同参数重放改变了原结果、checked_at 或 revision。")
    for path, original in (
        ("/assets", history["assets"]),
        ("/authorization-versions/" + history["version"]["id"], history["version"]),
        ("/prechecks/" + history["decision"]["id"], history["decision"]),
    ):
        status, recovered, _ = request(opener, project_base + path)
        if status != 200 or recovered != original:
            raise RuntimeError("重启后的资产、授权内容或决策详情未逐字段恢复。")
    current = submit_scope(opener, api_base, project_id, headers, history["precheck_body"], "precheck")
    current_decision = current["receipt"]["result"]["decision"]
    if current_decision.get("outcome") != "deny" or current_decision.get("project_revision") == history["decision"]["project_revision"]:
        raise RuntimeError("新预检查没有使用撤销后的当前 revision 并默认拒绝。")


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--environment-file", type=Path, help="显式 UTF-8 JSON 环境映射，不自动读取 .env")
    args = parser.parse_args()
    configured = load_environment(args.environment_file)
    if configured.get("MOUSEHOLE_MODE") != "development" or not configured.get("MOUSEHOLE_DATABASE_URL"):
        raise RuntimeError("重启验证需要显式 development 模式与已迁移的专用开发数据库。")
    go = check_go_version()
    api_port, issuer_port = allocate_port(), allocate_port()
    api_base = f"http://127.0.0.1:{api_port}"
    issuer_base = f"http://127.0.0.1:{issuer_port}"
    actor_id, project_id = str(uuid.uuid4()), str(uuid.uuid4())
    subject = "restart-" + uuid.uuid4().hex
    command_id = str(uuid.uuid4())
    environment = dict(
        configured,
        MOUSEHOLE_MODE="development",
        MOUSEHOLE_LISTEN_ADDR=f"127.0.0.1:{api_port}",
        MOUSEHOLE_PUBLIC_URL=api_base,
        MOUSEHOLE_WEB_ORIGIN=api_base,
        MOUSEHOLE_OIDC_ISSUER=issuer_base,
        MOUSEHOLE_OIDC_CLIENT_ID="mousehole-restart-check",
        MOUSEHOLE_OIDC_CLIENT_SECRET=uuid.uuid4().hex,
        MOUSEHOLE_AUTH_KEY=base64.b64encode(os.urandom(32)).decode("ascii"),
        MOUSEHOLE_TEST_OIDC_LISTEN_ADDR=f"127.0.0.1:{issuer_port}",
        MOUSEHOLE_TEST_OIDC_SUBJECT=subject,
    )
    with tempfile.TemporaryDirectory(prefix="mousehole-restart-") as directory:
        output = Path(directory)
        log_directory = ROOT / ".local/validation"
        log_directory.mkdir(parents=True, exist_ok=True)
        log_path = log_directory / ("restart-" + uuid.uuid4().hex + ".log")
        print("本轮诊断日志：" + str(log_path.relative_to(ROOT)), flush=True)
        with validation_log(log_path, environment) as log:
            api_binary = build_binary(go, "api", output, environment, log)
            admin_binary = build_binary(go, "admin", output, environment, log)
            issuer_binary = build_binary(go, "test-oidc", output, environment, log)
            issuer_process = start(issuer_binary, environment, log)
            api_process: subprocess.Popen | None = None
            try:
                wait_ready(
                    issuer_process,
                    issuer_base + "/.well-known/openid-configuration",
                    "合成 OIDC issuer",
                )
                provision(admin_binary, environment, actor_id, project_id, "重启验收项目", log)
                api_process = start(api_binary, environment, log)
                wait_ready(api_process, api_base + "/healthz", "重启检查 API")
                cookies = http.cookiejar.CookieJar()
                opener = urllib.request.build_opener(
                    urllib.request.ProxyHandler({}),
                    urllib.request.HTTPCookieProcessor(cookies), NoRedirect()
                )
                session = login(opener, api_base)
                url = f"{api_base}/api/v1/projects/{project_id}/commands/{command_id}"
                status, original, _ = request(
                    opener,
                    url,
                    "PUT",
                    {"kind": "create_task_draft", "payload": {"title": "进程重启持久化验收"}},
                    {"Origin": api_base, "X-CSRF-Token": session["csrf_token"]},
                )
                if (
                    status != 201
                    or original.get("state") != "accepted"
                    or original.get("task_state") != "draft"
                ):
                    raise RuntimeError("首次受理回执不符合合同。")
                scope_history = prepare_scope_history(
                    opener, api_base, project_id, session, admin_binary, environment, log
                )
                stop(api_process)
                api_process = start(api_binary, environment, log)
                wait_ready(api_process, api_base + "/healthz", "重启后的 API")
                status, recovered, _ = request(opener, url)
                if status != 200 or recovered != original:
                    raise RuntimeError("进程重启后查询未恢复原始回执。")
                status, replayed, _ = request(
                    opener, url, "PUT",
                    {"kind": "create_task_draft", "payload": {"title": "进程重启持久化验收"}},
                    {"Origin": api_base, "X-CSRF-Token": session["csrf_token"]},
                )
                if status != 200 or replayed != original:
                    raise RuntimeError("重启后的旧草稿同参数重放改变了原回执。")
                verify_scope_history(opener, api_base, project_id, session, scope_history)
                for arguments, original_receipt in scope_history["admin_commands"]:
                    output = safe_run(arguments, environment, log, "重启后重放原管理命令", capture=True)
                    if json.loads(output) != original_receipt:
                        raise RuntimeError("原管理命令因旧 revision 或已撤销状态改变了幂等回执。")
                    output = safe_run(
                        [
                            str(admin_binary), "scope-command", "get",
                            "--database-url", environment["MOUSEHOLE_DATABASE_URL"],
                            "--project-id", project_id,
                            "--command-id", original_receipt["command_id"],
                            "--operator-ref", "restart-validation",
                        ],
                        environment, log, "重启后查询原管理命令", capture=True,
                    )
                    if json.loads(output) != original_receipt:
                        raise RuntimeError("管理 CLI 查询没有恢复原冻结回执。")
            finally:
                stop(api_process)
                stop(issuer_process)
    print("真实登录与全进程重启通过：旧草稿、资产、授权及预检查回执逐字段一致。")


if __name__ == "__main__":
    try:
        main()
    except KeyboardInterrupt:
        print("重启验收已停止。", file=sys.stderr)
        raise SystemExit(130)
    except (
        RuntimeError,
        subprocess.SubprocessError,
        OSError,
        urllib.error.URLError,
        json.JSONDecodeError,
        KeyError,
        TypeError,
        ValueError,
        UnicodeError,
    ) as error:
        # 子进程、URL 和解析异常可能含秘密，只有本脚本构造的诊断可公开。
        message = str(error) if isinstance(error, RuntimeError) else "依赖或协议检查失败；请检查本轮诊断日志。"
        print(f"重启验证失败：{message}", file=sys.stderr)
        raise SystemExit(1) from error
