#!/usr/bin/env bash
# Devcontainer postCreateCommand entry point.
#
# Docker named volumes (.devcontainer/docker-compose.yml) 在第一次挂载时
# 会以 root 身份创建挂载点, 导致非 root 的 vscode 用户无法写入 go pkg /
# web/node_modules / .devcontainer-data. 在跑 bootstrap (go install /
# npm ci / playwright install) 之前先用免密 sudo 把所有权交还给当前
# UID, 然后再交给 Make 做真正的 toolchain 安装.

set -euo pipefail

volume_mounts=(
  /go/pkg
  /workspace/yamdc/web/node_modules
  /workspace/yamdc/.devcontainer-data
)

sudo chown -R "$(id -u):$(id -g)" "${volume_mounts[@]}"

exec make devcontainer-bootstrap
