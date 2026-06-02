#!/usr/bin/env bash
# Boot the Go API (yamdc server) and (optionally) Next.js dev server in
# the background. 用 setsid 让每个真实子进程都获得自己的 process group,
# stop-dev.sh 通过 .pgid 文件统一收尾, 避免 `go run` / `npm run dev`
# 这种"包一层"的进程在被 kill 主 pid 时把真正占着 8080 / 3000 的子进程
# 留下来.
#
# Usage:
#   start-dev.sh                 # boot backend + frontend
#   start-dev.sh --backend-only  # boot only backend (used by integration-test)

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$repo_root"

# guard: 仅允许在 devcontainer 内启动 8080 / 3000 进程, 避免污染宿主机.
"$repo_root/scripts/devcontainer/require-devcontainer.sh"

DEV_BACKEND_CONFIG=${DEV_BACKEND_CONFIG:-.devcontainer/config/devcontainer.json}
DEV_PID_DIR=${DEV_PID_DIR:-.devcontainer-data/pids}
DEV_LOG_DIR=${DEV_LOG_DIR:-.devcontainer-data/logs}

backend_config="$repo_root/${DEV_BACKEND_CONFIG}"
pid_dir="$repo_root/${DEV_PID_DIR}"
log_dir="$repo_root/${DEV_LOG_DIR}"

mkdir -p "$pid_dir" "$log_dir"

# 预先把 backend / save / library / scan 目录建出来. 后端 server 不会
# 自动 mkdir, integration-test 又要求"临时目录可用", 所以这里直接铺好.
data_root="$repo_root/.devcontainer-data"
mkdir -p \
  "$data_root/scan" \
  "$data_root/save" \
  "$data_root/library" \
  "$data_root/app/logs"

"$repo_root/scripts/devcontainer/stop-dev.sh"

# Pre-build the backend binary instead of `go run`. 在 GitHub Actions runner
# 网络抖动 / Go module proxy 慢的时候, `go run ./cmd/yamdc server` 会把
# module download 时间算进 wait-ready 的 60s 窗口, 导致 integration-test /
# e2e-test 偶发 timeout (Backend still alive but /api/healthz never responded).
# 这里把"拉依赖 + 编译"挪到独立步骤, 失败立即 fail-fast (set -e),
# 让 wait-ready 60s 真正只用于"server 已起 + 监听 8080".
backend_bin="$pid_dir/yamdc-server"
echo "Building backend binary so wait-ready window only times the server boot..." >&2
go build -o "$backend_bin" ./cmd/yamdc

setsid "$backend_bin" server --config "$backend_config" \
  > "$log_dir/backend.log" 2>&1 &
backend_pgid=$!
echo "$backend_pgid" > "$pid_dir/backend.pgid"

# wait-ready 监听 healthz; 若 backend 在启动阶段就退出 (端口占用 / 配置
# 错误等) 立即 fail-fast, 不再等满 60s.
if ! "$repo_root/scripts/devcontainer/wait-ready.sh" \
    "http://localhost:8080/api/healthz" --pgid "$backend_pgid"; then
  if ! kill -0 -- "-$backend_pgid" 2>/dev/null; then
    echo "Backend exited before becoming ready. Last 200 log lines:" >&2
  else
    # 进程还活着但 healthz 没响应 (常见于在某个 bootstrap action 卡住,
    # 例如外网拉依赖 / browser client 启动). 也把日志倾倒出来, 没有日志
    # CI 调试只能盲猜.
    echo "Backend still alive but /api/healthz never responded. Last 200 log lines:" >&2
  fi
  tail -n 200 "$log_dir/backend.log" >&2 || true
  exit 1
fi

if [[ "${1:-}" == "--backend-only" ]]; then
  echo "Backend ready: http://localhost:8080"
  echo "  logs: $log_dir/backend.log"
  exit 0
fi

(
  cd "$repo_root/web"
  setsid env API_PROXY_TARGET=http://localhost:8080 \
    npm run dev -- --hostname 0.0.0.0 \
    > "$log_dir/web.log" 2>&1 &
  echo "$!" > "$pid_dir/web.pgid"
)

"$repo_root/scripts/devcontainer/wait-ready.sh" "http://localhost:3000"

echo "Dev services ready:"
echo "  frontend: http://localhost:3000"
echo "  backend : http://localhost:8080"
echo "  logs    : $log_dir"
