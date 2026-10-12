"""通过完整 OIDC 登录与独立 API 进程重启验证回执和服务端会话可恢复。"""

from __future__ import annotations

import base64
import http.cookiejar
import json
import os
from pathlib import Path
import socket
import subprocess
import sys
import tempfile
import time
import urllib.error
import urllib.request
import uuid

from tooling import ROOT, check_go_version, run


class NoRedirect(urllib.request.HTTPRedirectHandler):
    """保留每一步重定向，便于检查 state 回调且不要求另起 Web 服务。"""

    def redirect_request(self, request, file_pointer, code, message, headers, new_url):
        return None


def allocate_port() -> int:
    """向操作系统申请当前空闲 loopback 端口；服务随后立即绑定。"""
    with socket.socket() as allocated:
        allocated.bind(("127.0.0.1", 0))
        return allocated.getsockname()[1]


def request(
    opener: urllib.request.OpenerDirector,
    url: str,
    method: str = "GET",
    body: dict | None = None,
    headers: dict[str, str] | None = None,
) -> tuple[int, dict, urllib.response.addinfourl]:
    """发送单次请求；PUT 未确认时绝不自动生成新 ID 或重发。"""
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
    payload: dict = {}
    if response.headers.get_content_type() == "application/json":
        payload = json.load(response)
    return response.status, payload, response


def wait_ready(process: subprocess.Popen, url: str, description: str) -> None:
    """等待本测试拥有的进程就绪；退出或超时都保留失败证据。"""
    deadline = time.monotonic() + 20
    while time.monotonic() < deadline and process.poll() is None:
        try:
            with urllib.request.urlopen(url, timeout=1) as response:
                if response.status == 200:
                    return
        except (OSError, urllib.error.URLError):
            pass
        time.sleep(0.1)
    stop(process)
    raise RuntimeError(f"{description}未能启动；请检查迁移、数据库和日志。")


def start(binary: Path, environment: dict[str, str], log) -> subprocess.Popen:
    """只启动本检查构建的二进制，调用者负责等待和停止。"""
    return subprocess.Popen([str(binary)], env=environment, stdout=log, stderr=log)


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


def main() -> None:
    database_url = os.environ.get("MOUSEHOLE_DATABASE_URL")
    if os.environ.get("MOUSEHOLE_MODE") != "development" or not database_url:
        raise RuntimeError("重启验证需要显式 development 模式与已迁移的专用开发数据库。")
    go = check_go_version()
    api_port, issuer_port = allocate_port(), allocate_port()
    api_base = f"http://127.0.0.1:{api_port}"
    issuer_base = f"http://127.0.0.1:{issuer_port}"
    actor_id, project_id = str(uuid.uuid4()), str(uuid.uuid4())
    subject = "restart-" + uuid.uuid4().hex
    command_id = str(uuid.uuid4())
    environment = dict(
        os.environ,
        MOUSEHOLE_MODE="development",
        MOUSEHOLE_LISTEN_ADDR=f"127.0.0.1:{api_port}",
        MOUSEHOLE_PUBLIC_URL=api_base,
        MOUSEHOLE_WEB_ORIGIN=api_base,
        MOUSEHOLE_OIDC_ISSUER=issuer_base,
        MOUSEHOLE_OIDC_CLIENT_ID="mousehole-restart-check",
        MOUSEHOLE_OIDC_CLIENT_SECRET="mousehole-restart-secret",
        MOUSEHOLE_AUTH_KEY=base64.b64encode(os.urandom(32)).decode("ascii"),
        MOUSEHOLE_TEST_OIDC_LISTEN_ADDR=f"127.0.0.1:{issuer_port}",
        MOUSEHOLE_TEST_OIDC_SUBJECT=subject,
    )
    with tempfile.TemporaryDirectory(prefix="mousehole-restart-") as directory:
        output = Path(directory)
        suffix = ".exe" if os.name == "nt" else ""
        api_binary = output / ("api" + suffix)
        admin_binary = output / ("admin" + suffix)
        issuer_binary = output / ("test-oidc" + suffix)
        run([go, "build", "-o", str(api_binary), "./cmd/api"], ROOT / "backend")
        run([go, "build", "-o", str(admin_binary), "./cmd/admin"], ROOT / "backend")
        run([go, "build", "-o", str(issuer_binary), "./cmd/test-oidc"], ROOT / "backend")
        with (output / "services.log").open("wb") as log:
            issuer_process = start(issuer_binary, environment, log)
            api_process: subprocess.Popen | None = None
            try:
                wait_ready(
                    issuer_process,
                    issuer_base + "/.well-known/openid-configuration",
                    "合成 OIDC issuer",
                )
                subprocess.run(
                    [
                        str(admin_binary),
                        "provision-user",
                        "--database-url",
                        database_url,
                        "--issuer",
                        issuer_base,
                        "--subject",
                        subject,
                        "--user-id",
                        actor_id,
                        "--display-name",
                        "重启验收用户",
                        "--project-id",
                        project_id,
                        "--project-name",
                        "重启验收项目",
                        "--role",
                        "analyst",
                    ],
                    env=environment,
                    check=True,
                    stdout=log,
                    stderr=log,
                )
                api_process = start(api_binary, environment, log)
                wait_ready(api_process, api_base + "/healthz", "重启检查 API")
                cookies = http.cookiejar.CookieJar()
                opener = urllib.request.build_opener(
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
                stop(api_process)
                api_process = start(api_binary, environment, log)
                wait_ready(api_process, api_base + "/healthz", "重启后的 API")
                status, recovered, _ = request(opener, url)
                if status != 200 or recovered != original:
                    raise RuntimeError("进程重启后查询未恢复原始回执。")
            finally:
                stop(api_process)
                stop(issuer_process)
    print(f"完整登录与全进程重启验证通过：{command_id}，回执逐字段一致。")


if __name__ == "__main__":
    try:
        main()
    except (
        RuntimeError,
        subprocess.CalledProcessError,
        OSError,
        urllib.error.URLError,
        json.JSONDecodeError,
    ) as error:
        print(f"重启验证失败：{error}", file=sys.stderr)
        raise SystemExit(1) from error
