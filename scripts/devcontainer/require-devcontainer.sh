#!/usr/bin/env bash
# require-devcontainer.sh
#
# Guard: dev-start / integration-test / e2e-test 这类会监听 8080 / 3000
# 端口、写 .devcontainer-data / web/node_modules / .cache 的入口脚本必须
# 跑在隔离环境里. 本脚本接受两类合法环境:
#
#   - YAMDC_DEVCONTAINER=1
#       本地 devcontainer 内. 由 .devcontainer/docker-compose.yml 的
#       services.dev.environment 注入, 同时通过 devcontainer.json 的
#       remoteEnv 透到 VS Code 终端.
#
#   - YAMDC_ALLOW_NON_DEVCONTAINER_TESTS=1
#       CI runner 显式放行. 用于 GitHub Actions 这类一次性隔离环境,
#       由 workflow step-level env 注入, 不会泄漏到本地宿主机.
#
# 任一变量缺失就当作"用户在自己宿主机上误执行", 直接 exit 1.
# devcontainer-up / devcontainer-rebuild / devcontainer-shell /
# devcontainer-check 这类"进入容器入口"型 target 的职责就是从宿主机
# 进容器, 因此 *不* 应该 source 本 guard.

set -euo pipefail

if [[ "${YAMDC_DEVCONTAINER:-}" == "1" ]]; then
  exit 0
fi

if [[ "${YAMDC_ALLOW_NON_DEVCONTAINER_TESTS:-}" == "1" ]]; then
  echo "require-devcontainer: YAMDC_ALLOW_NON_DEVCONTAINER_TESTS=1, running in CI mode" >&2
  exit 0
fi

cat >&2 <<'EOF'
错误: 当前命令必须在 devcontainer 内执行 (检测不到 YAMDC_DEVCONTAINER=1).

请先进入 devcontainer 再重试:
  make devcontainer-up        # 启动 devcontainer (后台)
  make devcontainer-shell     # 进入 devcontainer shell, 然后再跑想要的命令
  make devcontainer-check     # 在 devcontainer 内一键跑 ci-check

直接在宿主机执行 dev-start / integration-test / e2e-test 会污染宿主机
端口 (8080 / 3000) 与数据目录 (.devcontainer-data / .cache /
web/node_modules), 已被项目策略禁止.

如果你确实在一次性隔离环境 (例如 CI runner) 里, 请显式设置:
  YAMDC_ALLOW_NON_DEVCONTAINER_TESTS=1
EOF
exit 1
