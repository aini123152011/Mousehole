"""通过独立 API 进程重启验证已受理回执仍可恢复，不依赖服务实例缓存。"""

from __future__ import annotations

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


def request(url: str, method: str = "GET", body: dict | None = None) -> tuple[int, dict]:
    """向本测试创建的 loopback API 发请求，绝不根据错误自动重发命令。"""
    data = json.dumps(body, ensure_ascii=False).encode("utf-8") if body else None
    headers = {"X-Mousehole-Development": "true", "Content-Type": "application/json"}
    with urllib.request.urlopen(urllib.request.Request(url, data=data, headers=headers, method=method), timeout=3) as response:
        return response.status, json.load(response)


def start(binary: Path, environment: dict[str, str], base: str, log) -> subprocess.Popen:
    """只启动本测试二进制；失败会清理其子进程，不影响已运行的开发 API。"""
    process = subprocess.Popen([str(binary)], env=environment, stdout=log, stderr=log)
    deadline = time.monotonic() + 20
    while time.monotonic() < deadline and process.poll() is None:
        try:
            status, _ = request(base + "/healthz")
            if status == 200:
                return process
        except (OSError, urllib.error.URLError):
            pass
        time.sleep(0.1)
    stop(process)
    raise RuntimeError("重启检查的 API 未能启动；请检查迁移、数据库和开发环境配置。")


def stop(process: subprocess.Popen) -> None:
    """只终止本检查拥有的进程；强制退出后已提交记录也必须保留。"""
    if process.poll() is None:
        process.terminate()
        try:
            process.wait(timeout=10)
        except subprocess.TimeoutExpired:
            process.kill()
            process.wait(timeout=5)


def main() -> None:
    if os.environ.get("MOUSEHOLE_MODE") != "development" or not os.environ.get("MOUSEHOLE_DATABASE_URL"):
        raise RuntimeError("重启验证需要显式 development 模式与已迁移的专用开发数据库。")
    go = check_go_version()
    with socket.socket() as allocated:
        allocated.bind(("127.0.0.1", 0))
        port = allocated.getsockname()[1]
    base = f"http://127.0.0.1:{port}"
    environment = dict(os.environ, MOUSEHOLE_LISTEN_ADDR=f"127.0.0.1:{port}")
    project_id = "00000000-0000-4000-8000-000000000002"
    command_id = str(uuid.uuid4())
    url = f"{base}/api/v1/projects/{project_id}/commands/{command_id}"
    with tempfile.TemporaryDirectory(prefix="mousehole-restart-") as directory:
        binary = Path(directory) / ("api.exe" if os.name == "nt" else "api")
        run([go, "build", "-o", str(binary), "./cmd/api"], ROOT / "backend")
        with (Path(directory) / "api.log").open("wb") as log:
            process = start(binary, environment, base, log)
            try:
                status, original = request(url, "PUT", {"kind": "create_task_draft", "payload": {"title": "进程重启持久化验收"}})
                if status != 201 or original.get("state") != "accepted" or original.get("task_state") != "draft":
                    raise RuntimeError("首次受理回执不符合合同。")
            finally:
                stop(process)
            process = start(binary, environment, base, log)
            try:
                status, recovered = request(url)
                if status != 200 or recovered != original:
                    raise RuntimeError("进程重启后查询未恢复原始回执。")
            finally:
                stop(process)
    print(f"全进程重启验证通过：{command_id}，回执逐字段一致。")


if __name__ == "__main__":
    try:
        main()
    except (RuntimeError, subprocess.CalledProcessError, OSError, urllib.error.URLError) as error:
        print(f"重启验证失败：{error}", file=sys.stderr)
        raise SystemExit(1) from error
