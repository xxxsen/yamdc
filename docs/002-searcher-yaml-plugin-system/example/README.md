# 示例说明

本目录用于展示当前 YAML 插件系统的最小必要示例：

1. `one-step.yaml`
   单次请求 + HTML 抓取
2. `two-step.yaml`
   搜索页选择 + 详情页抓取
3. `multi-request.yaml`
   多候选请求回退
4. `json-scrape.yaml`
   JSON API + `jsonpath` 抓取

以及更复杂的组合示例：

5. `advanced-html-workflow.yaml`
   多候选请求 + `search_select` + `item_variables` + `expect_count`
6. `advanced-transform.yaml`
   复杂 transform 组合，包括 `split`、`regex_extract`、`postprocess`
7. `advanced-json-api.yaml`
   API 请求头 + JSON 抓取 + `duration_mmss` + 日期切分
