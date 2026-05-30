#!/usr/bin/env bash
# seed-e2e-fixtures.sh
#
# 在 devcontainer 数据目录里铺一份最小稳定的 E2E fixture 数据集. 仅写入
# .devcontainer-data/, 不会污染宿主机用户目录. 多次运行幂等.
#
# 内容:
#   - .devcontainer-data/scan/yamdc-e2e-scan.mp4
#       占位视频文件, 让 /api/scan 能至少识别出 1 个候选, 后续生成
#       reviewing job (E2E 不强制走完整 scrape 路径, 只验"扫描->列表"流转).
#   - .devcontainer-data/save/E2E-FIXTURE-001/
#       1 条已 scrape 完成的 library item: NFO + 假视频 + poster + cover
#       图片. 用于 /library 详情页 / variant / NFO 字段编辑 / poster 替换
#       的真实路径.
#   - .devcontainer-data/library/E2E-FIXTURE-002/
#       1 条已 move 到媒体库的 item: 同样结构. 媒体库 sync 时会把它读进
#       SQLite. 用于 /media-library 详情 / 同步日志 / 筛选用户路径.
#
# 用法:
#   - 进入 devcontainer 后手动: bash scripts/devcontainer/seed-e2e-fixtures.sh
#   - E2E 入口脚本 (run-e2e-test.sh) 会自动调一次, 启动 backend 之前.
#
# guard: 仅在 devcontainer 内执行 (避免在宿主机污染数据目录).

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
"$repo_root/scripts/devcontainer/require-devcontainer.sh"

data_root="${YAMDC_DATA_ROOT:-$repo_root/.devcontainer-data}"
scan_dir="$data_root/scan"
save_dir="$data_root/save"
library_dir="$data_root/library"

mkdir -p "$scan_dir" "$save_dir" "$library_dir"

# tiny placeholder video: 这是一个最小的 MP4 ftyp+moov 片段, 不是可播放
# 视频, 但扩展名为 .mp4, 后端 medialib videoExts 会识别. 任何依赖 ffmpeg
# 的真实视频解码不在 fixture 职责范围.
write_placeholder_video() {
  local target="$1"
  if [[ -s "$target" ]]; then
    return 0
  fi
  printf '\x00\x00\x00\x18ftypmp42\x00\x00\x00\x00mp42isom\x00\x00\x00\x08mdat' > "$target"
}

# write_tiny_png 写出一个 1x1 透明 PNG. http.DetectContentType 会识别为
# image/png, 后端 / 前端预览路径都能消费.
write_tiny_png() {
  local target="$1"
  if [[ -s "$target" ]]; then
    return 0
  fi
  base64 -d > "$target" <<'EOF'
iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAv8B/avnLZUA
AAAASUVORK5CYII=
EOF
}

# write_nfo 渲染一个最小但合法的 NFO XML, mov.ID 即 number 字段.
# 调用者负责 mkdir.
write_nfo() {
  local target="$1" number="$2" title="$3" actor="$4" year="$5"
  if [[ -s "$target" ]]; then
    return 0
  fi
  cat > "$target" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<movie>
  <title>${title}</title>
  <originaltitle>${title}</originaltitle>
  <id>${number}</id>
  <year>${year}</year>
  <releasedate>${year}-01-01</releasedate>
  <runtime>60</runtime>
  <studio>YAMDC E2E Studio</studio>
  <plot>E2E fixture seed plot for ${number}.</plot>
  <genre>fixture</genre>
  <actor>
    <name>${actor}</name>
    <role>lead</role>
  </actor>
  <poster>poster.png</poster>
  <cover>fanart.png</cover>
  <fanart>fanart.png</fanart>
</movie>
EOF
}

seed_item() {
  local root="$1" number="$2" title="$3" actor="$4" year="$5"
  local item_dir="$root/$number"
  mkdir -p "$item_dir"
  write_placeholder_video "$item_dir/$number.mp4"
  write_nfo "$item_dir/$number.nfo" "$number" "$title" "$actor" "$year"
  write_tiny_png "$item_dir/poster.png"
  write_tiny_png "$item_dir/fanart.png"
}

# seed_item_multi_variant 在 seed_item 基础上额外铺一个 -cd1 副 variant.
# 让 /library 详情页能渲染 LibraryVariantSwitcher (variants.length > 1 才显示),
# 以及一张 extrafanart 资源, 供 E2E 覆盖 "删除 fanart" 用户故事.
seed_item_multi_variant() {
  local root="$1" number="$2" title="$3" actor="$4" year="$5"
  seed_item "$root" "$number" "$title" "$actor" "$year"
  local item_dir="$root/$number"
  write_placeholder_video "$item_dir/${number}-cd1.mp4"
  write_nfo "$item_dir/${number}-cd1.nfo" "${number}-cd1" "${title} (CD1)" "$actor" "$year"
  mkdir -p "$item_dir/extrafanart"
  write_tiny_png "$item_dir/extrafanart/fanart-001.png"
  write_tiny_png "$item_dir/extrafanart/fanart-002.png"
}

write_placeholder_video "$scan_dir/yamdc-e2e-scan.mp4"
# seed-e2e-db.sh 注入的 reviewing 行 rel_path 是
# e2e-review-fixture/E2E-REVIEW-001.mp4. scanner.cleanupMissingJobs 在
# 每次 /api/scan 之后会用扫到的 rel_path 集合反向校验 DB, 没扫到的
# reviewing job 会被强制 mark failed. 因此 seed 阶段必须把这个 rel_path
# 对应的占位文件也写进 scan_dir, 否则 01-processing /api/scan 之后,
# 02-review 看到的状态就被破坏成 failed 了 (跨 spec 污染).
mkdir -p "$scan_dir/e2e-review-fixture"
write_placeholder_video "$scan_dir/e2e-review-fixture/E2E-REVIEW-001.mp4"
seed_item_multi_variant "$save_dir" "E2E-FIXTURE-001" "E2E Fixture Library" "E2E Star" "2024"
seed_item "$library_dir" "E2E-FIXTURE-002" "E2E Fixture Media" "E2E Star" "2024"

echo "[seed-e2e-fixtures] data_root=$data_root"
echo "[seed-e2e-fixtures] scan: yamdc-e2e-scan.mp4 + e2e-review-fixture/E2E-REVIEW-001.mp4"
echo "[seed-e2e-fixtures] save: E2E-FIXTURE-001/"
echo "[seed-e2e-fixtures] library: E2E-FIXTURE-002/"
