"""按同一入口验证源码格式、生成结果及前后端行为。"""

from __future__ import annotations

import argparse
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile

from tooling import ROOT, VERSIONS, check_go_version, check_web_versions, run


def check_text() -> None:
    """只检查产品与本次工程文件，避免改写 Trellis 或第三方内容。"""
    roots = [ROOT / p for p in ("api", "backend", "web", "scripts", "deploy", "docs", ".github")]
    excluded = {"node_modules", "dist", "coverage", "test-results", "playwright-report", "__pycache__", "bin"}
    extensions = {".go", ".sql", ".md", ".json", ".yaml", ".yml", ".ts", ".tsx", ".css", ".html", ".mjs", ".py", ".toml"}
    files = [ROOT / name for name in ("README.md", ".editorconfig", ".gitattributes", ".gitignore", ".prettierrc.json", ".env.example")]
    for folder in roots:
        if folder.exists():
            files.extend(p for p in folder.rglob("*") if p.is_file() and p.suffix in extensions and not excluded.intersection(p.relative_to(ROOT).parts))
    bad = []
    for path in files:
        if not path.is_file():
            raise RuntimeError(f"缺少交付文件：{path.relative_to(ROOT)}")
        data = path.read_bytes()
        data.decode("utf-8")
        if data.startswith(b"\xef\xbb\xbf") or b"\r" in data or not data.endswith(b"\n") or data.endswith(b"\n\n") or any(line.endswith((b" ", b"\t")) for line in data.splitlines()):
            bad.append(str(path.relative_to(ROOT)))
    if bad:
        raise RuntimeError("编码或换行不符合规范：" + ", ".join(bad))
    print(f"文本格式通过：{len(files)} 个文件", flush=True)


def snapshot(folder: Path) -> dict[str, bytes]:
    """比较生成文件名及字节，发现新增、删除与内容漂移。"""
    return {str(p.relative_to(folder)): p.read_bytes() for p in folder.rglob("*") if p.is_file()}


def check_sqlc(go: str) -> None:
    """在临时输入副本生成，检查漂移时不覆写可审查的工作区源码。"""
    backend = ROOT / "backend"
    with tempfile.TemporaryDirectory(prefix="mousehole-sqlc-") as directory:
        copied = Path(directory)
        shutil.copy2(backend / "sqlc.yaml", copied / "sqlc.yaml")
        shutil.copytree(backend / "db", copied / "db")
        run([go, "run", VERSIONS["sqlc"], "generate", "-f", str(copied / "sqlc.yaml")], backend)
        if snapshot(copied / "internal/store/sqlc") != snapshot(backend / "internal/store/sqlc"):
            raise RuntimeError("sqlc 生成结果漂移，请重新生成并审查差异。")


def check_backend(integration: bool, race: bool) -> None:
    """先检查静态规则，再执行真实行为测试；集成依赖不得静默跳过。"""
    go = check_go_version()
    backend = ROOT / "backend"
    gofiles = [str(p.relative_to(backend)) for p in backend.rglob("*.go") if "bin" not in p.parts]
    output = run([str(Path(go).with_name("gofmt.exe" if os.name == "nt" else "gofmt")), "-l", *gofiles], backend, True)
    if output.strip():
        raise RuntimeError("Go 格式漂移：\n" + output)
    output = run([go, "run", VERSIONS["goimports"], "-l", *gofiles], backend, True)
    if output.strip():
        raise RuntimeError("Go 导入或格式漂移：\n" + output)
    check_sqlc(go)
    run([go, "mod", "verify"], backend)
    run([go, "vet", "./..."], backend)
    test = [go, "test"] + (["-race"] if race else [])
    run([*test, "-count=1", "./..."], backend)
    if integration:
        if not os.environ.get("MOUSEHOLE_TEST_DATABASE_URL"):
            raise RuntimeError("集成检查缺少 MOUSEHOLE_TEST_DATABASE_URL；检查失败，不跳过。")
        run([*test, "-tags=integration", "-count=1", "./..."], backend)


def check_root_format(pnpm: str) -> None:
    """产品根文件与 Web 共用 Prettier；Trellis 历史模板不纳入格式改写。"""
    inputs = ["../README.md", "../.prettierrc.json", "../api/**/*.yaml", "../deploy/**/*.yaml", "../docs/**/*.md", "../.github/**/*.md", "../.github/**/*.yml", "../scripts/*.json"]
    run([pnpm, "exec", "prettier", "--check", "--no-error-on-unmatched-pattern", *inputs], ROOT / "web")


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--scope", choices=("all", "backend", "web", "text"), default="all")
    parser.add_argument("--integration", action="store_true", help="真实数据库检查，缺依赖失败")
    parser.add_argument("--race", action="store_true", help="Go 竞态检测，需要支持的 C 工具链")
    parser.add_argument("--e2e", action="store_true", help="真实浏览器测试，需要已经启动并迁移的 API")
    args = parser.parse_args()
    if args.integration and args.scope not in ("all", "backend"):
        parser.error("--integration 必须检查 backend 或 all")
    if args.race and args.scope not in ("all", "backend"):
        parser.error("--race 必须检查 backend 或 all")
    if args.e2e and args.scope not in ("all", "web"):
        parser.error("--e2e 必须检查 web 或 all")
    if args.scope in ("all", "text"):
        check_text()
    if args.scope in ("all", "backend"):
        check_backend(args.integration, args.race)
    if args.scope in ("all", "web"):
        pnpm = check_web_versions()
        check_root_format(pnpm)
        for action in ("format:check", "lint", "typecheck", "check:generated", "test", "build"):
            run([pnpm, action], ROOT / "web")
        if args.e2e:
            run([pnpm, "e2e"], ROOT / "web")
    print("所选检查全部通过。", flush=True)


if __name__ == "__main__":
    try:
        main()
    except (RuntimeError, subprocess.CalledProcessError, UnicodeError) as error:
        print(f"验证失败：{error}", file=sys.stderr)
        raise SystemExit(1) from error
