#!/usr/bin/env bash
# 收尾 start-dev.sh 留下的所有进程组. 见同目录 start-dev.sh 注释:
# 我们记录的是 process-group id, 不是单个 wrapper pid, 才能可靠停掉
# go run / npm run dev 派生出来的真正占端口的服务器进程.
#
# 行为:
#   * 给每条 .pgid 发 TERM, 等最多 ~10s 优雅退出.
#   * 还活着的统一升级 KILL.
#   * 结束后清理 pgid 文件 (进程已不在视为成功).
#   * 没 pid 目录 / 没 .pgid 文件 -> 直接成功退出, 给调用方做幂等清场.

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$repo_root"

DEV_PID_DIR=${DEV_PID_DIR:-.devcontainer-data/pids}
pid_dir="$repo_root/${DEV_PID_DIR}"

if [[ ! -d "$pid_dir" ]]; then
  exit 0
fi

shopt -s nullglob
pgid_files=("$pid_dir"/*.pgid)
shopt -u nullglob

if [[ ${#pgid_files[@]} -eq 0 ]]; then
  exit 0
fi

for f in "${pgid_files[@]}"; do
  if [[ ! -s "$f" ]]; then
    rm -f "$f"
    continue
  fi
  pgid="$(cat "$f")"
  if [[ -z "$pgid" ]]; then
    rm -f "$f"
    continue
  fi
  kill -TERM -- "-$pgid" 2>/dev/null || true
done

for _ in $(seq 1 10); do
  any_alive=0
  for f in "${pgid_files[@]}"; do
    [[ -s "$f" ]] || continue
    pgid="$(cat "$f")"
    [[ -n "$pgid" ]] || continue
    if kill -0 -- "-$pgid" 2>/dev/null; then
      any_alive=1
      break
    fi
  done
  if [[ "$any_alive" -eq 0 ]]; then
    break
  fi
  sleep 1
done

for f in "${pgid_files[@]}"; do
  if [[ -s "$f" ]]; then
    pgid="$(cat "$f")"
    if [[ -n "$pgid" ]] && kill -0 -- "-$pgid" 2>/dev/null; then
      kill -KILL -- "-$pgid" 2>/dev/null || true
    fi
  fi
  rm -f "$f"
done
