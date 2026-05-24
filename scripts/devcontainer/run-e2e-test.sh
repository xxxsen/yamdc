#!/usr/bin/env bash
# E2E 测试入口: 起 backend + frontend, 跑 Playwright, 退出时统一收尾.
# trap 在 shell 里设, 不能丢给 Makefile (Make 多行 recipe 拆成多个
# shell, trap 跨不到下一行).

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$repo_root"

# guard: E2E 会起 backend (8080) + frontend (3000), 必须在 devcontainer 内执行.
"$repo_root/scripts/devcontainer/require-devcontainer.sh"

cleanup() {
  "$repo_root/scripts/devcontainer/stop-dev.sh"
}
trap cleanup EXIT

PLAYWRIGHT_BROWSERS_PATH="${PLAYWRIGHT_BROWSERS_PATH:-$repo_root/.cache/ms-playwright}"
export PLAYWRIGHT_BROWSERS_PATH

# 在启动 backend / frontend 之前先铺好 E2E fixture 数据 (scan / save /
# library 三处), 让 spec 里"用户故事级"路径有真实数据可走. 多次运行幂等.
"$repo_root/scripts/devcontainer/seed-e2e-fixtures.sh"

"$repo_root/scripts/devcontainer/start-dev.sh"

# 启动后再注入 DB fixture (reviewing job + scrape_data). DB 在 backend
# 第一次启动时才完成 sqlite migration, 因此必须放在 start-dev 之后. 多次
# 运行幂等.
"$repo_root/scripts/devcontainer/seed-e2e-db.sh"

cd "$repo_root/web"
npx playwright test "$@"
