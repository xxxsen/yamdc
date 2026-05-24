FROM mcr.microsoft.com/devcontainers/go:1.25-bookworm

# 必装系统包:
# - bash / make / curl / git / jq: 全 Make / scripts/devcontainer 都依赖
# - ripgrep: agent + 仓内文档检索 (仓库自带 AGENTS.md 鼓励 rg)
# - ffmpeg: yamdc run 流水线对视频做缩略图 / 元数据探测
# - sqlite3: integration-test 直接读后端落盘 db 校验状态
# - tzdata: Asia/Shanghai 落盘时间一致
#
# Playwright Chromium headless 运行时依赖 (Debian bookworm):
# 基础镜像不含这些库, 缺任意一个 chrome-headless-shell 都会
# `error while loading shared libraries`, 导致 e2e 全量在 browser launch 阶段失败.
# 该清单与 `npx playwright install-deps chromium` 在 bookworm 上拉的子集对齐,
# 只挑 chrome-headless-shell ldd 实际依赖的运行时 + 一组中英日文渲染字体.
# 字体缺失虽然不会让 launch 失败, 但会让任何含 CJK 的页面回退到 tofu, e2e
# 截图诊断价值会被吃掉, 因此一并装入.
RUN apt-get update && apt-get install -y --no-install-recommends \
    bash \
    ca-certificates \
    curl \
    ffmpeg \
    fonts-liberation \
    fonts-noto-color-emoji \
    fonts-wqy-zenhei \
    git \
    jq \
    libasound2 \
    libatk-bridge2.0-0 \
    libatk1.0-0 \
    libatspi2.0-0 \
    libcairo2 \
    libcups2 \
    libnspr4 \
    libnss3 \
    libpango-1.0-0 \
    libxcomposite1 \
    libxdamage1 \
    make \
    procps \
    ripgrep \
    sqlite3 \
    tzdata \
  && rm -rf /var/lib/apt/lists/*
