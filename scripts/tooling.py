"""提供工作区内工具定位与参数化命令执行，不修改全局环境。"""

from __future__ import annotations

import json
import os
from pathlib import Path
import shutil
import subprocess
import sys

# Windows 终端与 CI 统一输出中文 UTF-8，不依赖宿主默认代码页。
sys.stdout.reconfigure(encoding="utf-8")
sys.stderr.reconfigure(encoding="utf-8")

ROOT = Path(__file__).resolve().parents[1]
VERSIONS = json.loads((ROOT / "scripts/toolchain.json").read_text(encoding="utf-8"))


def run(command: list[str], cwd: Path = ROOT, capture: bool = False) -> str:
    """直接执行参数数组；失败向上传递，不隐藏缺失依赖或检查失败。"""
    print(f"执行：{' '.join(command)}", flush=True)
    result = subprocess.run(
        command, cwd=cwd, check=True, text=True, encoding="utf-8", capture_output=capture
    )
    return result.stdout if capture else ""


def go_command() -> str:
    """优先使用本地校验过的工具；版本由调用者另行核对。"""
    suffix = ".exe" if os.name == "nt" else ""
    local = ROOT / f".tools/go{VERSIONS['go']}/go/bin/go{suffix}"
    found = str(local) if local.is_file() else shutil.which("go")
    if not found:
        raise RuntimeError("缺少 Go，请按 README 准备锁定版本。")
    return found


def pnpm_command() -> str:
    """Windows 使用 cmd 入口，其他系统使用可执行入口。"""
    found = shutil.which("pnpm.cmd" if os.name == "nt" else "pnpm")
    if not found:
        raise RuntimeError("缺少 pnpm，请按 README 准备锁定版本。")
    return found


def check_go_version() -> str:
    """拒绝未锁定的 Go，避免本地与 CI 用不同格式器或语言版本。"""
    go = go_command()
    actual = run([go, "env", "GOVERSION"], capture=True).strip()
    if actual != f"go{VERSIONS['go']}":
        raise RuntimeError(f"Go 版本不匹配：需要 {VERSIONS['go']}，实际 {actual}")
    return go


def check_web_versions() -> str:
    """同时核对 Node 与 pnpm，不能仅依赖 package.json 的提示。"""
    node = shutil.which("node")
    if not node:
        raise RuntimeError("缺少 Node，请按 README 准备锁定版本。")
    pnpm = pnpm_command()
    for command, expected in ((node, f"v{VERSIONS['node']}"), (pnpm, VERSIONS["pnpm"])):
        actual = run([command, "--version"], capture=True).strip()
        if actual != expected:
            raise RuntimeError(f"工具版本不匹配：需要 {expected}，实际 {actual}")
    return pnpm
