#!/usr/bin/env bash
# seed-e2e-db.sh
#
# 在 backend 启动 (并跑完 sqlite 迁移) 之后, 直接对 yamdc app DB 写入
# 一份最小稳定的 reviewing job + scrape_data 行, 让 /review 列表 /
# review 详情 / asset 上传 / poster 裁剪等用户故事能在 E2E 里跑通,
# 而不必走真实 scrape (会触外网请求, 不适合 E2E).
#
# 注入的 fixture (固定 rel_path, 多次运行幂等):
#   - rel_path:  e2e-review-fixture/E2E-REVIEW-001.mp4
#   - status:    reviewing
#   - number:    E2E-REVIEW-001
#   - title:     E2E Review Fixture
#   - actor:     E2E Star
#
# 写入完成后通过 GET /api/jobs?status=reviewing 校验一次, 把当前
# fixture 的 job id 输出到 stdout, 方便排错.
#
# 用法:
#   - 进入 devcontainer 后手动: bash scripts/devcontainer/seed-e2e-db.sh
#   - run-e2e-test.sh 在 start-dev.sh 之后自动调一次.

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
"$repo_root/scripts/devcontainer/require-devcontainer.sh"

data_root="${YAMDC_DATA_ROOT:-$repo_root/.devcontainer-data}"
# OpenAppDB 把 sqlite 落在 ${data_dir}/app/app.db; data_dir 来自
# .devcontainer/config/devcontainer.json 的 "data_dir" 字段, 当前为
# ${data_root}/app, 因此 db 实际位于 ${data_root}/app/app/app.db.
db_path="$data_root/app/app/app.db"

if [[ ! -f "$db_path" ]]; then
  echo "[seed-e2e-db] fatal: DB 不存在 (${db_path}), backend 是否已启动?" >&2
  exit 1
fi

if ! command -v sqlite3 >/dev/null 2>&1; then
  echo "[seed-e2e-db] fatal: 需要 sqlite3 命令; devcontainer 镜像应预装" >&2
  exit 1
fi

review_number="E2E-REVIEW-001"
review_rel_path="e2e-review-fixture/${review_number}.mp4"
# abs_path 主要用于 importer 移动文件, E2E 不会真的走 import 落盘 (那条路径
# 涉及实际 mv + medialib 入库, 已被 /review reject 路径覆盖). 这里给一个
# scan_dir 下的合法子路径, 即便后续操作触到也只会落在 .devcontainer-data 内.
review_abs_path="${data_root}/scan/${review_rel_path}"
review_uid="e2e-review-fixture-${review_number}"
ts_ms=$(date +%s%3N)

# raw_data: 模拟 scrape 抓到的原始 meta. 字段对齐 internal/model.MovieMeta
# (见 internal/model/model.go), 仅填会被 review-shell 显示 / 校验的字段, 其余
# 留空 / 零值, 跟生产落盘行为一致 (新影片 ID 没有 cover/poster 之前就是 null).
read -r -d '' raw_data <<EOF || true
{
  "number": "${review_number}",
  "title": "E2E Review Fixture",
  "title_translated": "",
  "plot": "Seeded by seed-e2e-db.sh; not from external scrape.",
  "plot_translated": "",
  "actors": ["E2E Star"],
  "release_date": 1704067200,
  "duration": 3600,
  "studio": "YAMDC E2E Studio",
  "label": "",
  "series": "",
  "genres": ["fixture"],
  "director": "",
  "cover": null,
  "poster": null,
  "sample_images": []
}
EOF

# review_data 初始与 raw_data 完全相同, 模拟"刚进入 review 状态尚未编辑"的快照.
review_data="$raw_data"

# 用 INSERT OR IGNORE + UPDATE 兜住幂等: 已存在不再覆盖 timestamp / number,
# 否则会造成 fixture 在跑 spec 之间偷偷漂移. 状态强制拍回 reviewing 才能保
# 证 spec 间 (例如 02-review reject 把 fixture 改成 failed) 不互相污染.
sqlite3 "$db_path" <<SQL
PRAGMA foreign_keys = ON;
INSERT OR IGNORE INTO yamdc_job_tab (
  job_uid, file_name, file_ext, conflict_key,
  rel_path, abs_path, number, raw_number,
  cleaned_number, number_source,
  number_clean_status, number_clean_confidence, number_clean_warnings,
  file_size, status, error_msg,
  retry_count, scrape_started_at, scrape_finished_at,
  reviewed_at, imported_at, deleted_at,
  created_at, updated_at
) VALUES (
  '${review_uid}', '${review_number}.mp4', 'mp4', '${review_number}',
  '${review_rel_path}', '${review_abs_path}', '${review_number}', '${review_number}',
  '${review_number}', 'manual',
  '', '', '',
  1024, 'reviewing', '',
  0, ${ts_ms}, ${ts_ms},
  0, 0, 0,
  ${ts_ms}, ${ts_ms}
);
UPDATE yamdc_job_tab
SET status = 'reviewing', deleted_at = 0, error_msg = '',
    updated_at = ${ts_ms}
WHERE rel_path = '${review_rel_path}';
SQL

# 取出 job id 后写 scrape_data. raw_data / review_data 通过 sqlite3 stdin
# 的 .parameter 设置以避免 shell 引号 / JSON 嵌套引号互相打架.
job_id=$(sqlite3 "$db_path" "SELECT id FROM yamdc_job_tab WHERE rel_path = '${review_rel_path}';")
if [[ -z "$job_id" ]]; then
  echo "[seed-e2e-db] fatal: 找不到刚插入的 reviewing job (rel_path=${review_rel_path})" >&2
  exit 1
fi

sqlite3 "$db_path" <<SQL
INSERT OR IGNORE INTO yamdc_scrape_data_tab (
  job_id, source, version, raw_data, review_data, final_data, status,
  created_at, updated_at
) VALUES (
  ${job_id}, 'e2e-fixture-seed', 1,
  '$(printf '%s' "$raw_data" | sed "s/'/''/g")',
  '$(printf '%s' "$review_data" | sed "s/'/''/g")',
  '', 'reviewed',
  ${ts_ms}, ${ts_ms}
);
UPDATE yamdc_scrape_data_tab
SET review_data = '$(printf '%s' "$review_data" | sed "s/'/''/g")',
    status = 'reviewed', updated_at = ${ts_ms}
WHERE job_id = ${job_id};
SQL

echo "[seed-e2e-db] reviewing fixture job_id=${job_id} rel_path=${review_rel_path}"
