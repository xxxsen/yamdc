# 外部 bundle 冒烟验证记录

**关联设计**: [design.md](./design.md) 第 "PR #7" 节。

本次验证只读、只跑测试, 不对 `yamdc-plugin` / `yamdc-script` 仓库
产生任何 commit, 目的是确认主仓的标识符 / schema / 字段改造没有
破坏既有发布物。

---

## 环境

| 仓库 | commit | 备注 |
|---|---|---|
| `yamdc` (本仓) | `HEAD` (含 PR #1 ~ #6) | 已 rebuild `./yamdc` 二进制 |
| `yamdc-plugin` | `origin/master` | 未修改 |
| `yamdc-script` | `origin/master` | 未修改 |

验证所用命令入口: `./yamdc ruleset-test` + `./yamdc plugin-test`。

---

## Step 1: ruleset bundle + cases 回放

### 命令

```bash
./yamdc ruleset-test \
    --ruleset=/home/sen/work/yamdc-script/ruleset \
    --casefile=/home/sen/work/yamdc-script/cases/default.json
```

### 结果

- **通过**: 25 / 27 用例。
- **失败**: 2 用例 (`onepondo_with_suffix`, `onepondo_uncensor_plain`)。
  - 失败原因: 期望输出 `011516_227` (下划线) vs 实际 `011516-227`
    (连字符)。该差异由 `normalize_template: '${1}_${2}'` 与
    `post_processors.normalize_hyphen` 冲突造成, 与本次改造无关。
- **预存在性核对**: 以 pre-refactor commit 同样命令复测,
  `passed=25, failed=2`, 失败列表完全一致。确认为 pre-existing。

### YAML 兼容层命中情况

`yamdc-script/ruleset/006-matchers.yaml` 中大量存在 `uncensor: true`
的历史字段, 加载期由 `MatcherRule.UncensorDeprecated` → `Unrated`
的 promotion 逻辑接管, 未出现 schema parse 错误, 也未影响任何用例的
pass/fail 判定。

### JSON cases 双读命中情况

`yamdc-script/cases/default.json` 中的 `"uncensor": true/false` 断言
由 `normalizeRulesetCaseOutput` 归一化到 `unrated`, 相关用例
(如 `fc2_with_chinese_suffix` / `uncensor_n_code` / `uncensor_negative_*`)
全部 pass, 说明双读兼容路径工作正常。

---

## Step 2: plugin bundle 端到端回放

### 命令

```bash
./yamdc plugin-test \
    --plugin=/home/sen/work/yamdc-plugin \
    --casefile=/home/sen/work/yamdc-plugin/cases/default.json
```

### 结果

- **通过**: 15 / 15 用例, `pass=true`。
- 涉及插件覆盖四种形态: one-step HTML (`jav321`), two-step HTML
  (`javdb`), JSON API (`missav`), 以及带 workflow 的 (`fc2`)。
- 加载期无 schema 错误。过程中有一条来自
  `parser/duration_parser.go` 的 "decode duration failed / data: \"\"" 日志,
  是被测插件在某些页面遇到空 duration 字段时的既有行为, 不影响
  用例 pass 判定, 与本次改造无关。

---

## 结论

| 验证维度 | 结论 |
|---|---|
| 主仓 Go 层编译 / lint / test | 通过 (`make ci-check`) |
| 外部 ruleset bundle 加载 | 成功, YAML 兼容层生效 |
| 外部 ruleset cases 判定 | 25/27 pass, 2 failure 与改造无关 (pre-existing) |
| 外部 plugin bundle 加载 | 成功, 15/15 用例通过 |

主仓改造对 `yamdc-plugin` / `yamdc-script` 发布物零破坏; 两个外部
仓库可以在后续独立节奏上迁移字段命名, 本次无需跟动。
