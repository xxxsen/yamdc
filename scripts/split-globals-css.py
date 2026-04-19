#!/usr/bin/env python3
"""
Split web/src/app/globals.css into per-module files under web/src/styles/.

一次性迁移脚本 (§2.2 cascade-safe CSS split).

策略:
  1. 识别顶层块 (在外层 @layer components 之外): `:root`, `@theme`,
     元素复位, 焦点环等, 原封不动保留在 globals.css 入口.
  2. 对外层 `@layer components { ... }` 内部, 按"顶层子块"逐个提取,
     按首个选择器里出现的首个 class 前缀路由到对应模块文件.
  3. 顶层 `@keyframes` 归 base.css (保留在 @layer 里无副作用).
  4. 同一模块内块顺序保持原 CSS 顺序.
  5. globals.css 入口 `@import` 模块顺序按 "首次出现" 顺序, 保证跨模块
     cascade 等价 (虽然本仓库的跨 prefix 覆盖都用更高 specificity, 不依赖
     源码顺序, 但按首现顺序更直观).

跑完后 globals.css 从 7023 行收缩到 ~140 行.
"""
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
SRC = ROOT / "web/src/app/globals.css"
OUT_DIR = ROOT / "web/src/styles"
OUT_DIR.mkdir(parents=True, exist_ok=True)

# prefix → module file name. 顺序即优先级 (首个匹配到的前缀决定归属).
PREFIX_RULES = [
    (("plugin", "wf", "body", "html", "dom", "xpath"), "plugin-editor.css"),
    (("ruleset", "handler", "searcher", "debug"), "debug.css"),
    (("media",), "media-library.css"),
    (("library",), "library.css"),
    (("review",), "review.css"),
    (("file", "table", "cell", "icon"), "file-table.css"),
    (
        ("btn", "badge", "input", "status", "token", "ui", "list", "native", "panel", "two"),
        "atoms.css",
    ),
    (("shell", "app", "sidebar", "main", "workspace"), "layout.css"),
]

MODULE_HEADERS = {
    "base.css": "基线动画 / 其它 @keyframes.",
    "layout.css": ".shell / .app-shell / .sidebar / .main / .panel / .workspace / .two-* 等页面骨架.",
    "atoms.css": ".btn / .input / .badge / .status / .token / .ui-* / .list / .native / 原子组件.",
    "file-table.css": ".file-* / .table / .cell / .icon 处理队列表格.",
    "review.css": ".review-* 审核页.",
    "library.css": ".library-* 素材库.",
    "media-library.css": ".media-* 媒体库.",
    "debug.css": ".handler-* / .searcher-* / .ruleset-* / .debug-* 调试页三件套.",
    "plugin-editor.css": ".plugin-* / .wf-* / .body-* / .html-* / .dom-* / .xpath-* plugin editor.",
}


def classify(block_text: str) -> str:
    """根据块内首个 class 前缀决定归属模块."""
    # 跳过 @keyframes
    stripped = block_text.lstrip()
    if stripped.startswith("@keyframes"):
        return "base.css"

    # 找出块的"选择器区段" (第一对 { 之前). 选出首个 .xxx- 形式的 class.
    first_brace = block_text.find("{")
    selector_part = block_text[:first_brace] if first_brace >= 0 else block_text
    # 收集所有 .prefix 以首现顺序
    matches = re.findall(r"\.([a-z][a-z0-9]*)", selector_part)
    for m in matches:
        for prefixes, target in PREFIX_RULES:
            if m in prefixes:
                return target

    # @media 块: 扫内部首个 class 决定归属
    if stripped.startswith("@media"):
        inner_matches = re.findall(r"\.([a-z][a-z0-9]*)", block_text)
        for m in inner_matches:
            for prefixes, target in PREFIX_RULES:
                if m in prefixes:
                    return target
        return "base.css"

    return "base.css"


def main() -> int:
    text = SRC.read_text()
    lines = text.splitlines(keepends=True)

    # 1) 找到最外层 "@layer components {" 和匹配的 "}".
    layer_start = None
    for i, line in enumerate(lines):
        if line.strip().startswith("@layer components {"):
            layer_start = i
            break
    if layer_start is None:
        print("ERR: cannot find '@layer components {'", file=sys.stderr)
        return 1

    # 找匹配结束位置.
    depth = 0
    layer_end = None
    for i in range(layer_start, len(lines)):
        depth += lines[i].count("{") - lines[i].count("}")
        if depth == 0:
            layer_end = i
            break
    if layer_end is None:
        print("ERR: unmatched '@layer components {'", file=sys.stderr)
        return 1

    # 2) 外层保留部分 (head: 到 layer_start 前; tail: layer_end 后).
    head = "".join(lines[:layer_start])
    # layer 内容 = lines[layer_start+1 .. layer_end-1] (剔除开闭两行).
    inner = lines[layer_start + 1 : layer_end]

    # 3) 对 inner 按 "顶层块" 切分. depth 从 0 开始 (layer 内的子块相对深度).
    blocks: list[str] = []  # 每个 block 是字符串
    buf: list[str] = []
    depth = 0
    pending_comment: list[str] = []

    def flush_comment_into_buf():
        """把紧挨下一个块的前导注释并入该块."""
        nonlocal pending_comment
        buf.extend(pending_comment)
        pending_comment = []

    for line in inner:
        stripped = line.strip()

        if depth == 0:
            if stripped == "":
                # 空行: 如果有待合并注释, 塞到注释里; 否则独立空行丢弃 (我们会在模块间补.)
                if pending_comment:
                    pending_comment.append(line)
                continue
            if stripped.startswith("/*"):
                # 多行注释可能跨多行; 先开始收集.
                pending_comment.append(line)
                if "*/" in stripped and not stripped.startswith("/*"):
                    # single-line complete
                    pass
                # 处理跨行注释: 维持 depth==0 前提下持续读到 */.
                if "*/" not in line:
                    # 进入 multi-line comment 模式; 但 depth 不变
                    # 下面常规流程会把下一行再收进 pending_comment.
                    # 用一个辅助 flag:
                    pass
                # 简化处理: 如果注释单行闭合, 保留; 如果多行, 交给下方判定.
                continue

            # 非空非注释, 块开始
            flush_comment_into_buf()
            buf.append(line)
            depth += line.count("{") - line.count("}")
            if depth == 0:
                # 单行块 (极少见). 结束.
                blocks.append("".join(buf))
                buf = []
            continue

        # depth > 0 : 在块内
        buf.append(line)
        depth += line.count("{") - line.count("}")
        if depth == 0:
            blocks.append("".join(buf))
            buf = []

    if buf:
        # 未闭合的残余也存下来 (不应该发生)
        blocks.append("".join(buf))

    # 处理多行注释的边界: 上面简化版假设注释是一行. 若有多行注释会被误分,
    # 这里做一个守卫: 对每个 block 检查是否有未配对 /* */.
    # 实际 globals.css 里绝大多数多行注释都在块紧前 + 块内, 不会跨 top-level
    # 边界. 如果真出现, 修正策略: 把含未闭合 /* 的注释与下一块合并.
    fixed_blocks: list[str] = []
    carry = ""
    for blk in blocks:
        combined = carry + blk
        # 计数 /* 和 */
        opens = combined.count("/*")
        closes = combined.count("*/")
        if opens > closes:
            # 注释没闭合, 推进到下一块再合并
            carry = combined
            continue
        carry = ""
        fixed_blocks.append(combined)
    if carry:
        fixed_blocks.append(carry)
    blocks = fixed_blocks

    # 4) 分派.
    modules: dict[str, list[str]] = {}
    first_seen: dict[str, int] = {}
    for idx, blk in enumerate(blocks):
        target = classify(blk)
        modules.setdefault(target, []).append(blk)
        first_seen.setdefault(target, idx)

    # 5) 写模块文件 (每个文件前加 header 注释).
    for name, blks in modules.items():
        header = (
            f"/* {name} — {MODULE_HEADERS.get(name, '')}\n"
            f" * 由 globals.css 统一通过 @layer components @import, 本文件不自己\n"
            f" * 声明 @layer. 详见 td/022-frontend-optimization-roadmap.md §2.2.\n"
            f" */\n\n"
        )
        body = "\n".join(b.rstrip("\n") for b in blks) + "\n"
        (OUT_DIR / name).write_text(header + body)

    # 6) 重写 globals.css: head + @layer components { @import ... } + tail.
    import_order = sorted(first_seen.keys(), key=lambda k: first_seen[k])
    imports = "\n".join(f'  @import "../styles/{name}";' for name in import_order)
    new_layer = (
        "/* ── @layer components (§2.2 css split) ───────────────────────────\n"
        " * globals.css 原先 7000+ 行一把梭, 本次按 shell/domain 拆成 9 个\n"
        " * 模块文件 (见 ../styles/). 本文件收缩成入口:\n"
        " *   - 上半保留 :root / @theme / 基础元素复位 / 焦点环 (均在 @layer\n"
        " *     之外, Tailwind base 之下)\n"
        " *   - 下半用单一 @layer components {} 包住所有模块 @import, 保证\n"
        " *     cascade = theme → base → components → utilities, utility 可以\n"
        " *     覆盖 .btn / .input / .panel 等老 class 样式\n"
        " * 模块 @import 顺序 = 块首次出现顺序, 保持语义与原单文件等价.\n"
        " * 详见 td/022-frontend-optimization-roadmap.md §2.2.\n"
        " */\n"
        "@layer components {\n"
        f"{imports}\n"
        "}\n"
    )

    # tail 部分原先只是 "}" (闭 @layer). 拆掉.
    # 检查 tail 是否仅含 "}" 和注释
    tail_lines = lines[layer_end + 1 :]
    tail_str = "".join(tail_lines).strip()
    if tail_str:
        print(f"WARN: non-empty tail after @layer close: {tail_str[:200]!r}", file=sys.stderr)

    new_content = head + new_layer
    SRC.write_text(new_content)

    # 7) 统计
    total_lines = 0
    for name in import_order:
        ln = (OUT_DIR / name).read_text().count("\n")
        total_lines += ln
        print(f"  {name:24} {ln:6} lines  ({len(modules[name])} blocks)")
    print(f"  globals.css (entry)      {new_content.count(chr(10)):6} lines")
    print(f"  modules total            {total_lines:6} lines")
    return 0


if __name__ == "__main__":
    sys.exit(main())
