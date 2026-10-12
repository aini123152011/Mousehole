"""启动显式本地开发服务或迁移；每项动作独立，避免隐式后台服务。"""

from __future__ import annotations

import argparse
import os
import subprocess
import sys

from tooling import ROOT, check_go_version, check_web_versions, run


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("action", choices=("database", "migrate", "migration-status", "oidc", "api", "web"))
    args = parser.parse_args()
    if args.action == "database":
        compose = ["docker", "compose", "-f", "deploy/compose.yaml"]
        run([*compose, "up", "-d", "--wait", "--wait-timeout", "45"])
        exists = run([*compose, "exec", "-T", "postgres", "psql", "-U", "mousehole", "-d", "postgres", "-Atc", "SELECT 1 FROM pg_database WHERE datname = 'mousehole_test'"], capture=True)
        if exists.strip() != "1":
            # 只创建本工程固定测试库；不接收任意库名，也不删除已有数据。
            run([*compose, "exec", "-T", "postgres", "psql", "-U", "mousehole", "-d", "postgres", "-c", "CREATE DATABASE mousehole_test"])
        return
    if args.action == "web":
        run([check_web_versions(), "dev"], ROOT / "web")
        return
    if args.action == "oidc":
        run([check_go_version(), "run", "./cmd/test-oidc"], ROOT / "backend")
        return
    if os.environ.get("MOUSEHOLE_MODE") != "development" or not os.environ.get("MOUSEHOLE_DATABASE_URL"):
        raise RuntimeError("必须显式设置 MOUSEHOLE_MODE=development 和 MOUSEHOLE_DATABASE_URL；参见 .env.example。")
    go = check_go_version()
    if args.action == "api":
        run([go, "run", "./cmd/api"], ROOT / "backend")
    else:
        command = "up" if args.action == "migrate" else "status"
        run([go, "run", "./cmd/migrate", command], ROOT / "backend")


if __name__ == "__main__":
    try:
        main()
    except KeyboardInterrupt:
        print("开发命令已停止。")
        raise SystemExit(130)
    except (RuntimeError, subprocess.CalledProcessError) as error:
        print(f"开发命令失败：{error}", file=sys.stderr)
        raise SystemExit(1) from error
