#!/usr/bin/env bash
# E2E 测试入口: 起 backend + frontend, 跑 Playwright, 退出时统一收尾.
# trap 在 shell 里设, 不能丢给 Makefile (Make 多行 recipe 拆成多个
# shell, trap 跨不到下一行).

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$repo_root"

cleanup() {
  "$repo_root/scripts/devcontainer/stop-dev.sh"
}
trap cleanup EXIT

PLAYWRIGHT_BROWSERS_PATH="${PLAYWRIGHT_BROWSERS_PATH:-$repo_root/.cache/ms-playwright}"
export PLAYWRIGHT_BROWSERS_PATH

"$repo_root/scripts/devcontainer/start-dev.sh"

cd "$repo_root/web"
npx playwright test "$@"
