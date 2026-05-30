#!/usr/bin/env bash
# Poll an HTTP URL until it returns a 2xx response. integration-test /
# e2e-test 用来在 setsid 启动 backend / web 后等真正可达, 不依赖固定
# sleep. 可选 --pgid 让 backend 启动失败时秒败而不是等满 60 次.

set -euo pipefail

if [[ "$#" -lt 1 ]]; then
  echo "usage: $0 <url> [--pgid <pgid>]" >&2
  exit 2
fi

url="$1"
shift
watched_pgid=""
while [[ "$#" -gt 0 ]]; do
  case "$1" in
    --pgid)
      watched_pgid="${2:-}"
      if [[ -z "$watched_pgid" ]]; then
        echo "wait-ready: --pgid requires an argument" >&2
        exit 2
      fi
      shift 2
      ;;
    *)
      echo "wait-ready: unknown option $1" >&2
      exit 2
      ;;
  esac
done

for _ in $(seq 1 60); do
  if curl -fsS "$url" >/dev/null; then
    exit 0
  fi
  if [[ -n "$watched_pgid" ]] && ! kill -0 -- "-$watched_pgid" 2>/dev/null; then
    echo "wait-ready: watched pgid $watched_pgid is no longer alive" >&2
    exit 1
  fi
  sleep 1
done

echo "wait-ready: timed out waiting for $url" >&2
exit 1
