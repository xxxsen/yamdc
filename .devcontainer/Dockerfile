FROM mcr.microsoft.com/devcontainers/go:1.25-bookworm

# 必装系统包:
# - bash / make / curl / git / jq: 全 Make / scripts/devcontainer 都依赖
# - ripgrep: agent + 仓内文档检索 (仓库自带 AGENTS.md 鼓励 rg)
# - ffmpeg: yamdc run 流水线对视频做缩略图 / 元数据探测
# - sqlite3: integration-test 直接读后端落盘 db 校验状态
# - tzdata: Asia/Shanghai 落盘时间一致
RUN apt-get update && apt-get install -y --no-install-recommends \
    bash \
    ca-certificates \
    curl \
    ffmpeg \
    git \
    jq \
    make \
    procps \
    ripgrep \
    sqlite3 \
    tzdata \
  && rm -rf /var/lib/apt/lists/*
