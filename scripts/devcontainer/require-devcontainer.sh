#!/usr/bin/env bash
# require-devcontainer.sh
#
# Guard: 仅在 devcontainer 内部允许执行调用方脚本.
# 通过检测 YAMDC_DEVCONTAINER=1 环境变量来判断, 该变量由
# .devcontainer/devcontainer.json 的 containerEnv 注入.
#
# 设计目标: 阻止 dev-start / integration-test / e2e-test 在用户宿主机上
# 直接启动 backend (8080) / frontend (3000) 进程, 避免污染宿主机端口与
# 数据目录. devcontainer-up / devcontainer-rebuild / devcontainer-shell /
# devcontainer-check 这类目标本身的职责就是进入容器, 因此 *不* 应该
# source 本 guard.

set -euo pipefail

if [[ "${YAMDC_DEVCONTAINER:-}" != "1" ]]; then
  cat >&2 <<'EOF'
错误: 当前命令必须在 devcontainer 内执行 (检测不到 YAMDC_DEVCONTAINER=1).

请先进入 devcontainer 再重试:
  make devcontainer-up        # 启动 devcontainer (后台)
  make devcontainer-shell     # 进入 devcontainer shell, 然后再跑想要的命令
  make devcontainer-check     # 在 devcontainer 内一键跑 ci-check

直接在宿主机执行 dev-start / integration-test / e2e-test 会污染宿主机
端口 (8080 / 3000) 与数据目录 (.devcontainer-data / .cache /
web/node_modules), 已被项目策略禁止.
EOF
  exit 1
fi
