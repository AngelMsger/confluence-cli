# confluence-cli 技术设计文档

## 1. 目标与范围

`confluence-cli` 是一个 Go 编写的命令行工具，让 Coding Agent（Claude Code 等）把
Confluence 当作外部知识库进行检索、读取与维护。

- **同时支持** Confluence Cloud 与 Confluence Data Center / Server（self-hosted），
  并兼容多个 REST API 版本。
- **面向 Agent**：默认输出 JSON，错误结构化，正文支持分级 / 分段读取，错误信息携带
  可执行的下一步建议。
- **配置多来源**：CLI 参数 / 环境变量 / `.env` / 配置文件，含交互式 `init` 引导。
- **操作范围**：读与写。读覆盖取页面、CQL 搜索、列空间、子页 / 后代、附件、评论、
  标签、版本历史。写覆盖页面（创建 / 编辑 / 删除 / 移动 / 复制 / 恢复历史版本）、
  附件（上传 / 替换 / 删除）、标签（增 / 删）、评论（发布 / 编辑 / 删除）、关注
  （watch / unwatch）；另有 `whoami` 查询当前凭据对应的用户。每个写命令支持
  `--dry-run` 预览将要发出的请求。

非目标：空间的创建 / 删除 / 归档、页面权限（restrictions）、内容属性、博客内容类型、
PDF 导出、OAuth 2.0 三方授权（预留扩展点，本期不做）。

## 2. API Flavor 差异矩阵

CLI 用 `Flavor` 区分两类后端：

| Flavor | 说明 | REST 基址 |
|--------|------|-----------|
| `cloud` | Confluence Cloud (`*.atlassian.net`) | v2 `/wiki/api/v2`，v1 `/wiki/rest/api` |
| `datacenter` | Data Center / Server（self-hosted，如 7.19.x） | `/rest/api` |

每个操作在不同 flavor 下的端点 / 分页 / body 参数差异如下（`{base}` 为站点根 URL）：

| 操作 | cloud | datacenter |
|------|-------|------------|
| 取页面 | `GET {base}/wiki/api/v2/pages/{id}?body-format=storage` + 单独取 ancestors | `GET {base}/rest/api/content/{id}?expand=body.storage,version,ancestors,space` |
| 子页面 | `GET {base}/wiki/api/v2/pages/{id}/children`（cursor） | `GET {base}/rest/api/content/{id}/child/page?expand=...&start&limit` |
| 后代页面 | `GET {base}/wiki/api/v2/pages/{id}/descendants`（cursor） | `GET {base}/rest/api/content/{id}/descendant/page?start&limit` |
| CQL 搜索 | `GET {base}/wiki/rest/api/content/search?cql=&start&limit`（用 v1） | `GET {base}/rest/api/content/search?cql=&start&limit` |
| 列空间 | `GET {base}/wiki/api/v2/spaces`（cursor） | `GET {base}/rest/api/space?start&limit` |
| 取空间 | `GET {base}/wiki/api/v2/spaces?keys={key}` | `GET {base}/rest/api/space/{key}` |
| 列评论 | `GET {base}/wiki/api/v2/pages/{id}/footer-comments`（cursor） | `GET {base}/rest/api/content/{id}/child/comment?expand=body.storage,version&depth=all` |
| 加评论 | `POST {base}/wiki/rest/api/content`（type=comment，用 v1） | `POST {base}/rest/api/content`（type=comment） |
| 列附件 | `GET {base}/wiki/api/v2/pages/{id}/attachments`（cursor） | `GET {base}/rest/api/content/{id}/child/attachment?start&limit` |
| 下载附件 | 跟随附件的 `downloadLink` | 跟随附件的 `_links.download` |
| 探活 | `GET {base}/wiki/api/v2/spaces?limit=1` | `GET {base}/rest/api/space?limit=1` |

**分页**：cloud v2 为游标分页（响应 `_links.next` 含 `cursor` 查询参数）；cloud v1 与
datacenter 为 offset 分页（`start` / `limit`，响应 `_links.next` 或 `size < limit` 判终止）。
`PaginationKind` 枚举 `Offset` / `Cursor` 统一抽象。

**body 格式**：datacenter / cloud-v1 用 `expand=body.storage`；cloud-v2 用 `body-format=storage`。
归一化后统一为 `Body{Representation:"storage", Value:"<xhtml>"}`。

**flavor 检测**：显式 `--flavor` / 配置 > URL 启发式（host `*.atlassian.net` 或 path 含 `/wiki/`
判 cloud）> `auto` 时 `Ping` 探测 v2 失败回退 v1。探测结果可写回配置 `detected_flavor` 缓存。

## 3. 归一化数据模型

所有 API 方法返回下列与 flavor 无关的模型（`internal/apiclient/models.go`）：

```
ServerInfo { Flavor, BaseURL, Version, Reachable }
Space      { ID, Key, Name, Type, URL }
Page       { ID, Type, Title, SpaceKey, Status, Version, URL,
             Ancestors[]PageRef, Body *Body }
PageRef    { ID, Title }
Body       { Representation, Value }                // Representation 恒为 "storage"
Version    { Number, When, By }                     // By 为作者显示名
Comment    { ID, PageID, ParentID, Body *Body, Version, URL }
Attachment { ID, Title, MediaType, FileSize, DownloadURL, Version }
SearchHit  { ID, Type, Title, SpaceKey, URL, Excerpt, LastModified }
```

JSON 输出字段用 snake_case；时间统一 RFC3339 字符串。

## 4. 配置与认证

### 4.1 配置结构

```
Config {
  BaseURL  string                 // 站点根 URL
  Flavor   string                 // cloud | datacenter | auto
  Auth     AuthConfig
  Defaults Defaults
  DetectedFlavor string            // auto 探测缓存
}
AuthConfig { Scheme string         // pat | basic
             Username string }     // basic 用；secret 不入此结构
Defaults   { Format string         // json（默认）
             PageSize int          // 默认 25
             Timeout  duration      // 默认 30s
             MaxRetries int }       // 默认 3
```

### 4.2 来源与优先级

高 → 低：CLI 参数 > 环境变量（`CONFLUENCE_*`）> `.env` 文件 > `~/.confluence/config.yaml`
> 内置默认值。实现为有序 `mergeLayers([]Config)`：每层为稀疏 `Config`，非零字段覆盖低层。
每个字段记录来源，供 `config show --explain`。

环境变量映射：

| 变量 | 字段 |
|------|------|
| `CONFLUENCE_SERVER` | `BaseURL` |
| `CONFLUENCE_FLAVOR` | `Flavor` |
| `CONFLUENCE_PERSONAL_ACCESS_TOKEN` | PAT 密钥（scheme=pat） |
| `CONFLUENCE_USERNAME` | `Auth.Username` |
| `CONFLUENCE_PASSWORD` | basic 密钥 |
| `CONFLUENCE_API_TOKEN` | basic 密钥（cloud：与邮箱组合） |
| `CONFLUENCE_FORMAT` | `Defaults.Format` |

`.env` 经 `godotenv` 读入临时 map，不写进程环境，保证「环境变量优先于 `.env`」成立。

### 4.3 认证

- **pat**：`Authorization: Bearer <token>`。Data Center 7.9+ 推荐。
- **basic**：`Authorization: Basic base64(user:secret)`。datacenter 为用户名 + 密码；
  cloud 为邮箱 + API token。

密钥永不写入 `config.yaml`。`config init` 写入时存入 keychain（`go-keyring`，service
`confluence-cli`，account `<host>:<scheme>`），失败回退 `~/.confluence/credentials`
（文件 0600，目录 0700）。运行期密钥若来自 env / `.env` / flag，仅临时使用，不持久化。

### 4.4 init 向导

输入 base URL → 探测并确认 flavor → 选认证方式与凭证 → `Ping` 实时校验 → 选密钥存储方式
→ 写非密字段到 `config.yaml`、密钥入 keychain / 文件 → 打印下一步建议命令。

## 5. 命令规格

全局持久 flag：`--base-url`、`--flavor`、`--format`(json|table|ndjson)、`--fields`、
`--timeout`、`--config`、`--use-context`、`--verbose`。

命令树按资源分组：`page`、`search`、`space`、`comment`、`attachment`、`label`、
`config`、`auth`、`doctor`、`whoami`、`skill`、`version`。共同约定：

- **ID 解析**：接受页面的入参也接受页面 URL，经 `pkg/urlref` 解析；评论入参接受
  评论 ID 或带 `focusedCommentId` 的评论永久链接（普通页面 URL 会被拒绝）；附件
  入参只接受附件内容 ID。
- **写操作**：`page create/update/delete/move/copy/restore`、`page watch/unwatch`、
  `attachment upload/update/delete`、`label add/remove`、`comment add/update/delete`
  均为写命令；每个写命令支持 `--dry-run` 预览将发出的请求，删除类命令需 `--yes`。
- **分页**：list 命令（`search`、`page children/descendants/history`、`comment list`、
  `attachment list`、`label list`、`space list`）接受 `--limit/--all/--cursor`，
  输出 `{items, next, has_more}` 信封。
- **正文格式**：写命令用 `--body-format` 指定正文格式，与全局 `--format`（输出
  格式）互不冲突。`page create/update` 支持 `storage|wiki|markdown`（markdown
  在客户端转 storage）；`comment add/update` 支持 `storage|wiki`。

完整的命令、flag 与示例由命令树自动生成，见 [docs/cli/](cli/)（`make docs` 生成、
CI 校验不漂移）—— 本节不再维护并行的命令清单，以杜绝文档与实现脱节。

## 6. 输出与错误模型

### 6.1 输出

`Formatter` 接口三实现：`json`（默认，面向 Agent，stdout）、`table`（人类可读）、
`ndjson`（流式大结果集）。`--fields a,b.c` 按点路径投影。list 命令输出分页信封
`{items, next, has_more}`，`--cursor` 可从上一页的 `next` 续读下一页。

成功输出统一为 stdout 上的 JSON，但有三处刻意的 raw-output 例外：

- `version` 打印纯文本版本行（与 `--version` flag 对齐）。
- `attachment download --output -` 把附件原始字节写入 stdout（用于管道）。
- `skill show` 把内嵌的 `SKILL.md` 原样打印。

交互式向导（`config init`、`auth login`）的提示走 stderr；错误只走 stderr。

### 6.2 错误

错误以 JSON 写 **stderr**：

```json
{"error":{"category":"auth","code":"AUTH_INVALID_CREDENTIALS",
  "message":"...","hint":"...","next_steps":["..."],
  "retryable":false,"http_status":401}}
```

category：`usage config auth not_found permission conflict rate_limit network server parse internal`。

### 6.3 退出码

| 码 | category | 码 | category |
|----|----------|----|----------|
| 0 | success | 6 | not_found |
| 1 | internal | 7 | rate_limit |
| 2 | usage | 8 | network |
| 3 | config | 9 | server |
| 4 | auth | 10 | parse |
| 5 | permission | 11 | conflict |

`conflict`（HTTP 409，exit 11）用于 `page update` / `comment update` 等写操作的
版本冲突 —— 重新读取目标内容拿到当前版本号后再重试。

`hints.go` 把 category 映射为 next_steps，引导 Agent 自我纠正。

## 7. 正文渲染管线

storage XHTML → `golang.org/x/net/html` 解析为节点树（Confluence 宏特殊处理：
`ac:structured-macro` code → 围栏代码块，info / note / warning panel → 引用块，
`ac:link` / `ri:*` → 链接文本）→ 抽取 `h1..h6` 标题树并赋稳定 section ID（`sec-1`、`sec-1-2`）
→ scope 切片 → detail 分级 → 渲染。

- **scope**：`full` 全文 / `outline` 仅标题树 / `section`（需 `--section`）标题子树 /
  `keyword`（需 `--keyword`）命中块 + 所属标题路径。
- **detail**：`simple` 纯文本、宏压平 / `with-ids` 标注 section ID / `full` 含宏细节。
- **as**：`markdown`（Agent 默认）/ `text`。

结果结构 `RenderedBody{Outline, Body, ScopeApplied, Truncated}`。

## 8. CQL 构造

`search` 无位置参数时由 flag 拼 CQL（`internal/apiclient/cql.go`）：

| flag | CQL 片段 |
|------|----------|
| `--text` | `text ~ "<v>"` |
| `--author` | `creator = "<v>"` |
| `--contributor` | `contributor = "<v>"` |
| `--space` | `space = "<v>"` |
| `--label` | `label = "<v>"` |
| `--type` | `type = <v>`（page / blogpost / comment / attachment） |
| `--after` | `lastmodified >= "<v>"` |
| `--before` | `lastmodified <= "<v>"` |

各片段以 `AND` 连接；字符串值转义内部引号。给定位置参数 `<cql>` 时直接透传。

## 9. Skill 大纲

`skills/confluence/SKILL.md`（YAML frontmatter：`name: confluence`、触发词描述、
`metadata.requires.bins`、`metadata.cliHelp`）+ `references/`：

- `getting-started.md` — 配置 / 认证检查、`doctor`、flavor 概念。
- `reading-pages.md` — `--scope` / `--detail` 决策树：先 outline 再 section，full 谨慎。
- `searching-cql.md` — CQL 参数表与 flag 映射、大结果集分页。
- `comments.md` — 评论读写，唯一写操作的确认提醒。
- `attachments.md` — 列后下载。
- `errors-and-exit-codes.md` — 退出码表 + 按 category 的恢复动作。

核心黄金法则：操作前先把 URL / 名称解析成 ID。

同一份 SKILL.md 同时适配 **Claude Code** 与 **Codex**（两者都只要求 frontmatter 的
`name` + `description`）。`skill install` 用一张 agent 路径表（`internal/app/skill.go`
的 `agentSpecs`）描述各 Agent 的全局 / 项目 skills 目录与探测标记：Claude Code 用
`~/.claude/skills`、`./.claude/skills`；Codex 用 `~/.codex/skills`、`./.agents/skills`。
无 flag 时探测目录是否存在，装入 / 移除每个命中的 Agent；`--agent` 显式指定，`--dir`
为 agent 无关的显式路径。

## 10. 测试策略

- **单元测试**：标准库 `testing`，表驱动，`t.Parallel()`。覆盖 config 优先级、auth 解析与
  文件权限、cql 构造、分页 offset/cursor、mapping 两 flavor 归一、render 各 scope/detail、
  output 各格式与 `--fields`、errors 映射、urlref。
- **HTTP 层测试**：`httptest.Server` 驱动各 Client 方法，断言路径 / 参数 / 认证头、v2→v1 回退。
- **契约 / golden 测试**：`testdata/fixtures/{cloud,datacenter}/*.json` 驱动 mapping 与渲染。
- **端到端**：`scripts/e2e.sh` 构建二进制 + 内置 mock Confluence（覆盖 v1/v2/DC 路由），
  跑全部命令断言 stdout 输出契约与退出码（多数命令为 JSON，raw-output 例外见 §6.1）。
- **只读 live 验证**：`make e2e-live` 仅跑 `page get` / `search` / `space list` / `doctor`。
