# td/029 实施 Review

Review 时间：2026-05-24

Review 范围：

- 当前工程：`/home/sen/work/yamdc`
- 参考工程：`/home/sen/work/fire-manager`
- 跨仓库集成对象：`/home/sen/work/yamdc-plugin`、`/home/sen/work/yamdc-script`
- 本次重点：td/029 实施后的 FE / BE / UIUX / devcontainer / E2E / 工程规范一致性

已确认通过的检查：

- `go test ./internal/web` 通过。
- `cd web && npm run lint` 通过。
- `cd web && npm run test` 通过，65 个测试文件、1054 个用例通过。
- `AGENTS.md` 已聚焦当前工程本身，未再引入 `/home/sen/work/fire-manager`、`/home/sen/work/yamdc-plugin`、`/home/sen/work/yamdc-script`、`mcp-feedback-enhanced` 或后续工程化目标。

## P1：CORS 默认行为与最终约束不一致

涉及文件：

- `internal/web/router.go`
- `internal/web/router_test.go`

问题：

td/029 的最终约束是：`YAMDC_ALLOWED_ORIGINS` 不填时默认为 `*`，用户本地启动的安全影响可接受；测试时可通过 `localhost` 访问，上线后用户本地仍可能通过域名访问。

当前实现相反：

- `defaultAllowedOrigins` 固定为 `http://localhost:3000`、`http://127.0.0.1:3000`。
- `YAMDC_ALLOWED_ORIGINS` 为空时回退到该白名单。
- 未命中白名单的 `POST / PUT / PATCH / DELETE / OPTIONS` 会返回 HTTP 403。
- 注释明确写了“不允许 `*`”，与最终用户约束冲突。

触发条件：

1. 不设置 `YAMDC_ALLOWED_ORIGINS`。
2. 用户通过自定义本地域名访问前端，例如 `https://yamdc.local` 或局域网域名。
3. 前端发起带 `Origin` 的状态变更请求，例如 `POST /api/scan`、`PUT /api/review/jobs/:id`、`POST /api/media-library/sync`。
4. 后端返回 HTTP 403 + `errCodeOriginForbidden`，用户无法完成操作。

确定修复方案：

- `YAMDC_ALLOWED_ORIGINS` 为空或 trim 后为空时进入 wildcard 模式。
- wildcard 模式下：
  - 对所有跨域响应写入 `Access-Control-Allow-Origin: *`。
  - `OPTIONS` 预检返回 204。
  - 不做 Origin 白名单拦截。
- `YAMDC_ALLOWED_ORIGINS` 非空时才进入显式白名单模式：
  - 命中白名单时回显请求 Origin。
  - 未命中白名单的状态变更请求返回 403。
  - 未命中白名单的预检返回 403。
- 移除“不允许 `*`”相关注释，改为说明“默认 wildcard，显式配置后启用白名单”。

测试建议：

- 修改 `TestLoadAllowedOriginsDefault`，断言默认是 wildcard，而不是 localhost 白名单。
- 新增默认 wildcard 下的测试：
  - `OPTIONS` + 任意 Origin 返回 204 且 `Access-Control-Allow-Origin: *`。
  - `POST` + 任意 Origin 不被 CORS middleware 拦截。
  - `GET` + 任意 Origin 返回 `Access-Control-Allow-Origin: *`。
- 保留并调整显式白名单测试：
  - `YAMDC_ALLOWED_ORIGINS=http://desk.local:3001` 时命中 Origin 通过。
  - 非命中 Origin 的 `POST` / `OPTIONS` 返回 403。

## P1：新增 E2E 的 API 契约断言与真实后端返回结构不一致

涉及文件：

- `web/e2e/04-library.spec.ts`
- `web/e2e/06-media-library.spec.ts`
- `web/e2e/07-media-library-sync.spec.ts`
- `internal/web/library.go`
- `internal/web/media_library.go`
- `web/src/lib/api/library.ts`
- `web/src/lib/api/media-library.ts`

问题：

后端和前端真实 API 客户端的契约是：

- `GET /api/library` 的 `data` 是 `LibraryListItem[]`。
- `GET /api/media-library` 的 `data` 是 `MediaLibraryItem[]`。
- `GET /api/media-library/sync/logs` 的 `data` 是 `MediaLibrarySyncLogEntry[]`。

当前 E2E 错误地把这些接口断言为：

- `data.items` 是数组。
- `env.data?.items` 是数组。

这会导致 E2E 在接口正常返回 `[]` 时失败。

触发条件：

1. 在 devcontainer 内运行 `make e2e-test`。
2. `04-library.spec.ts` 调用 `apiGet<LibraryListResponse>("/api/library")`。
3. 后端返回 `{ code: 0, data: [] }`。
4. 用例执行 `Array.isArray(data.items)`，结果为 `false`，测试失败。

同类触发条件也存在于：

- `06-media-library.spec.ts` 的 `GET /api/media-library`。
- `07-media-library-sync.spec.ts` 的 `GET /api/media-library/sync/logs`。

确定修复方案：

- 将 `04-library.spec.ts` 的类型改为 `LibraryItem[]`，断言 `Array.isArray(data)`。
- 将 `06-media-library.spec.ts` 的类型改为 `MediaLibraryItem[]`，断言 `Array.isArray(data)`。
- 将 `07-media-library-sync.spec.ts` 的类型改为 `SyncLogEntry[]`，断言 `Array.isArray(env.data)`。
- 删除这些 E2E 文件注释中“`data.items`”的错误描述，统一改为“`data` 是数组”。

测试建议：

- 在 devcontainer 内运行 `make e2e-test`。
- 为 `web/e2e/helpers/api.ts` 增加一个轻量单测或 E2E 内部断言，确保 `apiGet<T>` 返回 envelope 的 `data` 本身，而不是额外包一层 `{items}`。

## P2：E2E 覆盖仍偏冒烟，未达到 td/029 的核心用户路径覆盖要求

涉及文件：

- `web/e2e/01-processing.spec.ts`
- `web/e2e/02-review.spec.ts`
- `web/e2e/03-review-assets.spec.ts`
- `web/e2e/04-library.spec.ts`
- `web/e2e/06-media-library.spec.ts`
- `web/e2e/09-plugin-editor.spec.ts`

问题：

td/029 要求补齐更多 E2E 用例，覆盖关键用户交互和修改后的逻辑。当前 E2E 数量虽然增加，但大量用例仍停留在页面可达、元素可见、接口 envelope 的 smoke 层：

- Processing：未覆盖扫描 fixture、生成 job、运行 job、日志查看、删除确认。
- Review：未覆盖字段编辑、保存、导入、拒绝、删除、错误提示恢复。
- Review assets：未覆盖 UI 上传、超限文件前端拦截、上传成功后预览更新、裁剪 poster。
- Library：未覆盖有 fixture 的详情页、NFO 字段编辑、variant 切换、图片替换、fanart 删除。
- Media library：未覆盖有 fixture 的卡片详情、筛选组合、同步后日志 UI 展示。
- Plugin editor：注释中明确 XPath inspector E2E 未覆盖，仅依赖单测。

触发条件：

1. 发生真实用户流程回归，例如“Review 字段无法保存”或“Library 图片替换后 UI 不刷新”。
2. 当前 E2E 仍可能通过，因为它们没有执行这些用户动作。

确定修复方案：

- 在 devcontainer fixture 中准备最小稳定数据集：
  - 1 个可扫描视频文件。
  - 1 个待 Review job。
  - 1 个已入库 Library item。
  - 1 个 Media Library item。
  - 1 组小尺寸 PNG/JPEG 资产。
- E2E 必须补齐以下确定路径：
  - Processing：触发扫描 -> job 出现在列表 -> 执行 job -> 查看日志 -> 删除二次确认。
  - Review：打开 job -> 修改标题/番号/演员 -> 保存 -> 刷新后值仍存在 -> 导入入库 -> 异常路径提示可恢复。
  - Review assets：上传 cover/poster/fanart -> 预览更新 -> 超 32 MiB 文件被前端拦截 -> cover 裁剪 poster。
  - Library：打开详情 -> 切换 variant -> 编辑 NFO 字段 -> 替换 poster/cover -> 删除 fanart。
  - Media library：执行同步 -> 查看同步日志 -> 按年份/大小/标题筛选排序 -> 打开详情。
  - Plugin editor：覆盖 XPath inspector 从 DOM 节点选择到 selector 写回输入框的浏览器交互。
- 所有 fixture 初始化必须在 devcontainer 数据目录内完成，不写入用户宿主机目录。

测试建议：

- 在 devcontainer 内运行 `make e2e-test`。
- 在 CI 中将 `make e2e-test` 作为独立 job，失败时上传 Playwright trace、screenshot、backend/web 日志。

## P2：dev-start / integration-test / e2e-test 未强制运行在 devcontainer 内

涉及文件：

- `Makefile`
- `scripts/devcontainer/start-dev.sh`
- `scripts/devcontainer/run-integration-test.sh`
- `scripts/devcontainer/run-e2e-test.sh`
- `.devcontainer/devcontainer.json`

问题：

用户最终要求补齐 devcontainer，避免直接在用户机器上启动进程。当前 Makefile 注释写了“默认必须在 devcontainer 内执行”，但脚本没有实际 guard：

- `make dev-start` 会直接在当前机器执行 `go run ./cmd/yamdc server` 和 `npm run dev`。
- `make integration-test` 会直接在当前机器启动 backend。
- `make e2e-test` 会直接在当前机器启动 backend、frontend 和 Playwright。

触发条件：

1. 用户在宿主机项目目录执行 `make e2e-test`。
2. 脚本在宿主机启动 8080 / 3000 进程，并写入 `.devcontainer-data`、`.cache`、`web/node_modules` 等目录。
3. 与“避免直接在用户机器上启动进程”的目标不一致。

确定修复方案：

- 在 `.devcontainer/devcontainer.json` 的 `containerEnv` 中增加 `YAMDC_DEVCONTAINER=1`。
- 新增 `scripts/devcontainer/require-devcontainer.sh`：
  - 检查 `YAMDC_DEVCONTAINER=1`。
  - 不满足时直接退出，并提示先执行 `make devcontainer-up` / `make devcontainer-shell` / `make devcontainer-check`。
- 在以下脚本开头强制调用该 guard：
  - `scripts/devcontainer/start-dev.sh`
  - `scripts/devcontainer/run-integration-test.sh`
  - `scripts/devcontainer/run-e2e-test.sh`
- `devcontainer-up`、`devcontainer-rebuild`、`devcontainer-shell`、`devcontainer-check` 保持可在宿主机运行，因为它们的职责就是进入容器或在容器内执行命令。

测试建议：

- 宿主机执行 `make e2e-test`：必须立即失败，且不得启动 8080 / 3000 进程。
- devcontainer 内执行 `make integration-test`：必须正常启动 backend 并通过。
- devcontainer 内执行 `make e2e-test`：必须正常启动 backend/frontend 并跑完 Playwright。

## P2：上传大小限制用 request body 作为 32 MiB 硬上限，会误拒绝合法文件

涉及文件：

- `internal/web/jobs_routes.go`
- `web/src/lib/upload-limits.ts`
- `web/src/components/review-shell/use-review-asset-actions.ts`
- `web/src/components/library-shell/use-library-asset-actions.ts`

问题：

前端按 `file.size <= 32 MiB` 做限制，后端 `readUploadImageData` 却用 `http.MaxBytesReader(..., maxUploadImageBytes)` 限制整个 multipart request body。

multipart body 包含 boundary 和 header，体积一定大于文件本身。因此一个刚好 32 MiB 的合法图片文件：

- 前端会允许上传。
- 后端可能在解析 multipart 时因 request body 超过 32 MiB 而返回过大错误。

触发条件：

1. 用户选择一个大小等于或略小于 32 MiB 的 PNG/JPEG 文件。
2. 前端 `validateUploadSize` 通过。
3. 浏览器用 multipart/form-data 上传。
4. multipart overhead 让 request body 超过 32 MiB。
5. 后端 `MaxBytesReader` 提前拒绝。

确定修复方案：

- 后端 request body 上限改为 `maxUploadImageBytes + maxUploadMultipartOverheadBytes`。
- `maxUploadMultipartOverheadBytes` 固定为 `1 << 20`。
- 文件本身的上限仍只通过 `header.Size` 和 `len(data)` 严格执行 `<= maxUploadImageBytes`。
- 错误文案继续表述为“upload file exceeds 32 MiB limit”，因为业务限制仍是文件大小 32 MiB。

测试建议：

- Go 单测新增：
  - 文件大小刚好 `maxUploadImageBytes` 的 multipart 上传成功。
  - 文件大小 `maxUploadImageBytes + 1` 返回 upload too large。
  - multipart header 很大但文件未超过 32 MiB 时，只要总 body 未超过 33 MiB，应成功。
- 前端 `upload-limits` 单测保持：
  - `32 MiB` 通过。
  - `32 MiB + 1 byte` 拦截。

## P3：工程注释仍引用临时 td 编号和外部模板语义

涉及文件：

- `Makefile`
- `.devcontainer/docker-compose.yml`

问题：

当前 `AGENTS.md` 已明确要求禁止在代码、测试、注释中引用 `td/` 或 `tdxxx` 临时编号。`Makefile` 中仍有“`029-fe-be-uiux-review` 验收要求”的注释，这会把临时设计文档泄漏到工程代码注释里。

`.devcontainer/docker-compose.yml` 中还有“对齐 fire-manager 模板”的注释。该信息不是当前工程运行所需，且没有给出 `/home/sen/work/fire-manager` 的具体路径。外部参考工程只应出现在 review / 设计文档中，不应成为运行配置注释的一部分。

触发条件：

1. 后续维护者只读 Makefile / devcontainer 配置。
2. 需要反向查找临时 td 文档或外部模板，才能理解注释来源。
3. 违反当前工程文档与注释自洽要求。

确定修复方案：

- 将 Makefile 注释中的 `029-fe-be-uiux-review` 改为功能语义描述，例如“跨仓库集成测试默认路径”。
- 删除 `.devcontainer/docker-compose.yml` 中“对齐 fire-manager 模板”的描述，只保留当前工程为什么挂载 docker sock 的说明。

测试建议：

- 执行 `rg -n "td/|td[0-9]{3}|fire-manager|mcp-feedback|/home/sen/work" AGENTS.md Makefile .devcontainer scripts web internal docs`。
- 除 `td/` 目录内 review 文档外，不应出现临时 td 编号或外部参考工程路径。

## 建议修复顺序

1. 先修 P1 CORS 默认 wildcard 行为，否则上线后自定义域名访问会被默认拦截。
2. 再修 P1 E2E API 契约错误，否则新增 E2E 自身不可作为质量闸口。
3. 补 devcontainer guard，确保 e2e / integration 不在宿主机启动进程。
4. 修上传 32 MiB 边界，避免 FE/BE 限制不一致。
5. 扩充 E2E fixture 与用户故事路径。
6. 清理工程注释中的临时编号和外部模板语义。

## 本轮结论

td/029 当前实施未达到“无 P1/P2 bug”的退出条件，不能视为 review 通过。后续修复必须以本文件列出的确定方案为准，并在修复 PR / commit 中补齐对应单元测试、集成测试和 E2E 验证。
