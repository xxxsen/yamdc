#!/usr/bin/env bash
# 后端集成测试: 起 backend, 用 HTTP API 覆盖
#   - GET  /api/healthz                (基础)
#   - POST /api/scan                   (扫描入口)
#   - GET  /api/jobs                   (job 列表)
#   - POST /api/jobs/:id/run           (单 job 触发, 仅在有 job 时跑)
#   - GET  /api/media-library/status   (媒体库状态)
# 协议契约: HTTP 2xx + body { code: 0, message, data }; 任何 code != 0
# 直接 fail. 所有断言都打印诊断信息.
#
# trap cleanup 确保 backend 一定被停掉, 避免 8080 残留挡下一次测试.

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$repo_root"

# guard: 集成测试也会起 backend (8080), 必须在 devcontainer 内执行.
"$repo_root/scripts/devcontainer/require-devcontainer.sh"

cleanup() {
  "$repo_root/scripts/devcontainer/stop-dev.sh"
}
trap cleanup EXIT

API_BASE="${API_BASE:-http://localhost:8080}"
DATA_ROOT="${DATA_ROOT:-$repo_root/.devcontainer-data}"

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "integration-test: required command '$1' not found in PATH" >&2
    exit 1
  }
}
require_cmd curl
require_cmd jq

call_api() {
  # call_api METHOD PATH [BODY_JSON] [--allow-fail]
  # 默认要求 HTTP 2xx + envelope.code == 0; --allow-fail 时仍解 envelope
  # 但 code != 0 不当致命错误, 把 envelope 原样返回给调用方.
  local method="$1" path="$2"
  local body="" allow_fail=0 tmp http_status code message
  shift 2
  while [[ "$#" -gt 0 ]]; do
    case "$1" in
      --allow-fail) allow_fail=1; shift ;;
      *) body="$1"; shift ;;
    esac
  done
  tmp="$(mktemp)"
  if [[ -n "$body" ]]; then
    http_status="$(curl -sS -o "$tmp" -w '%{http_code}' \
      -X "$method" -H 'Content-Type: application/json' \
      -d "$body" "$API_BASE$path")"
  else
    http_status="$(curl -sS -o "$tmp" -w '%{http_code}' \
      -X "$method" "$API_BASE$path")"
  fi
  if [[ "$http_status" -lt 200 || "$http_status" -ge 300 ]]; then
    echo "integration-test: $method $path failed with HTTP $http_status" >&2
    echo "response:" >&2
    cat "$tmp" >&2
    rm -f "$tmp"
    exit 1
  fi
  code="$(jq -r '.code // empty' < "$tmp")"
  if [[ -z "$code" ]]; then
    echo "integration-test: $method $path missing envelope.code" >&2
    cat "$tmp" >&2
    rm -f "$tmp"
    exit 1
  fi
  if [[ "$code" != "0" && "$allow_fail" -ne 1 ]]; then
    message="$(jq -r '.message // empty' < "$tmp")"
    echo "integration-test: $method $path returned code=$code message=$message" >&2
    echo "response:" >&2
    cat "$tmp" >&2
    rm -f "$tmp"
    exit 1
  fi
  cat "$tmp"
  rm -f "$tmp"
}

"$repo_root/scripts/devcontainer/start-dev.sh" --backend-only

echo "→ healthz"
call_api GET /api/healthz | jq -e '.data.status == "ok"' >/dev/null

echo "→ media-library status"
call_api GET /api/media-library/status | jq -e '.data.configured == true' >/dev/null

echo "→ scan (空 scan 目录, 应返回 success + 0 个 job 创建)"
call_api POST /api/scan >/dev/null

echo "→ jobs list"
jobs_envelope="$(call_api GET '/api/jobs?page=1&page_size=10')"
echo "$jobs_envelope" | jq -e '.data.items | type == "array"' >/dev/null

echo "→ review save 异常路径 (不存在的 job_id 必须 200 + 业务错)"
review_resp="$(call_api PUT /api/review/jobs/999999 '{"meta":{}}' --allow-fail)"
review_code="$(echo "$review_resp" | jq -r '.code')"
if [[ "$review_code" == "0" ]]; then
  echo "integration-test: review save against nonexistent job 不应该成功" >&2
  echo "$review_resp" >&2
  exit 1
fi

echo "→ review import 异常路径 (不存在的 job_id 必须 200 + 业务错)"
import_resp="$(call_api POST /api/review/jobs/999999/import '' --allow-fail)"
import_code="$(echo "$import_resp" | jq -r '.code')"
if [[ "$import_code" == "0" ]]; then
  echo "integration-test: review import against nonexistent job 不应该成功" >&2
  echo "$import_resp" >&2
  exit 1
fi

echo "→ review job run 异常路径 (job_id=999999 必须 200 + 业务错)"
run_resp="$(call_api POST /api/jobs/999999/run '' --allow-fail)"
run_code="$(echo "$run_resp" | jq -r '.code')"
if [[ "$run_code" == "0" ]]; then
  echo "integration-test: job run against nonexistent job 不应该成功" >&2
  echo "$run_resp" >&2
  exit 1
fi

cat <<EOF
integration-test: ok
  api_base       = $API_BASE
  data_root      = $DATA_ROOT
EOF
