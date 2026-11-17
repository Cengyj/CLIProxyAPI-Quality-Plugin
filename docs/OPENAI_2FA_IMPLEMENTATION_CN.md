# OpenAI 2FA 登录实现与验收说明

> 存储更新：插件 0.7.0 / UI 8.1.0 改为本机明文保存登录资料，默认可用，无需加密初始化；当前直接使用 schema 2，不实现旧版数据迁移，初始化接口已移除。下文原实现中的 AES/vault.key 段落保留为历史设计，当前持久化行为以 [PERSISTENCE_CN.md](PERSISTENCE_CN.md) 为准。

状态：基础登录已实现并通过离线及本机安装验收，2026-11-15。插件0.5.0增加账号管理及定时巡检，当前扩展行为见 [OPENAI_ACCOUNTS_CN.md](OPENAI_ACCOUNTS_CN.md)；下文的5.1.0记录为历史版本。本文描述当前源码行为；原设计文档中的建议参数、拟新增字段和未来能力不构成已交付承诺。固定参考研究见 [OPENAI_2FA_RESEARCH_CN.md](OPENAI_2FA_RESEARCH_CN.md)，运行包构建和限制见 [OPENAI_2FA_RUNTIME_CN.md](OPENAI_2FA_RUNTIME_CN.md)。

## 1. 用户操作

当前插件在顶部提供独立的“OpenAI 登录”标签页，与“智商测试”“质量运维”并列，包括浏览器授权与本地“密码 + TOTP”登录。已有 Codex 凭证可按真实 `auth_id` 重新授权；新增凭证走导入流程。登录资料默认只用于当前任务；独立初始化资料库并明确选择保存后，才加密存储密码/TOTP，供以后手动重登。

授权完成先生成候选，管理员查看脱敏身份与限制后明确提交。token 不经过插件 UI；手动浏览器模式只提交完整 callback URL。登录过程不调用模型，不进行智商/质量测试，不解除质量暂停，不恢复规则，不清除原生冷却。新导入凭证默认禁用，既有凭证重登保留原有禁用和暂停条件。

UI 5.1.0 支持直接进入 `#login`；未连接时显示连接入口，连接成功后在当前页加载。切出标签或断连会清除页面秘密输入，宿主任务不因此取消。插件0.5.0/UI6.0.0另增持久化账号管理与可选定时重登；首次和手动授权仍保留候选确认，详见账号管理说明。

质量暂停凭证仍禁止复测。管理员可以执行认证维修，但此操作不能作为恢复质量或释放暂停的证据。

## 2. 当前职责边界

| 组件 | 已实现职责 |
| --- | --- |
| 插件嵌入 UI | 登录输入、方式选择、任务状态、候选确认、显式提交、资料库管理 |
| `internal/api/handlers/management/credential_logins.go` | v8 管理鉴权路由、16 KiB 请求限制、no-store、固定错误、幂等入口 |
| `internal/authlogin/` | 宿主 PKCE/state、任务生命周期、单次 token 兑换、可信响应身份检查、候选、可选资料库 |
| `internal/authlogin/runtime*.go` | 本机运行包/摘要校验、有限 stdin/stdout、环境隔离、进程组回收 |
| `login-runtime/` | 官方网页邮箱/密码/TOTP 表单操作，仅返回绑定授权会话的 callback |
| `sdk/cliproxy/auth/credential_login*.go` | 稳定身份与政策快照、专用提交 gate、文件保存/运行态采用、质量身份与提交对账 |
| 现有 quality 状态与插件规则 | 保持暂停所有权；授权变化后相关规则停止，需要显式恢复 |

插件仍为独立 Go 模块，不从插件导入宿主 `internal` 包。新的授权写操作在宿主内完成，不新增让插件后台任意保存 token 的通用 RPC。现有宿主参考目录不改动，变更仅在工作区宿主副本中。

## 3. 已注册 API

统一前缀为 `/v8/management/credential-logins`，沿用当前 v8 management 鉴权；不改 deprecated v0 路由。下列写请求必须带 `Idempotency-Key`，长度 8–128；不得把秘密放 query。JSON 拒绝未知字段和重复键。

| 方法 / 路径 | 当前行为 |
| --- | --- |
| GET `/capabilities` | 宿主/插件可用性、browser/password_totp 方法可用性及 vault 状态；不下载运行包 |
| GET `/targets` | Codex 目标、脱敏邮箱、身份版本、可更新原因、人工禁用与质量暂停 |
| GET `/jobs` | 最多最近 200 个脱敏任务；当前没有 cursor 分页契约 |
| GET `/detail?id=...` | 任务状态；waiting_browser 时可返回内存中的授权链接 |
| GET `/candidate?id=...` | 候选身份、revision、动作和限制摘要，不返回 token |
| POST `/start` | 创建单个异步任务，当前返回 HTTP 200 和 Job；并非同步登录结果 |
| POST `/callback` | 提交 `{job_id, callback_url}`，绑定 state/redirect，唯一消费 code |
| POST `/cancel` | 取消任务；运行中先进入 cancelling，等待进程/交换退出 |
| POST `/discard` | 丢弃未提交候选；不改变上游账号 |
| POST `/commit` | `{job_id, candidate_id, revision}` 显式提交，返回提交中或已有结果 |
| GET `/vault/status?auth_id=...` | 就绪状态、条目 revision、脱敏邮箱与 has_password/has_totp |
| POST `/vault/initialize` | 独立初始化本机加密 key，不发起登录 |
| PUT `/vault/entry` | 按 `{auth_id, revision, credentials}` 保存匹配已验证身份的资料 |
| DELETE `/vault/entry` | 按 `{auth_id, revision}` 删除本机登录资料，不撤销上游授权 |

`/start` 支持 `consumer_plugin="model-quality"`、`method="browser"|"password_totp"`、`intent="import"|"reauthorize"`；重登还需 `target_auth_id` 和 `expected_identity_version`。本地输入为 `credentials: {email,password,totp_secret}`，可显式选择 `remember_credentials`；已有目标可选择 `use_saved_credentials`。浏览器方式不接受密码、保存资料或读取资料选项。没有独立批量写接口，小批量交互仍调用相同单任务入口。

所有响应为 `Cache-Control: no-store`。重复幂等请求返回已记录结果；同 key 不同参数返回冲突；未决请求返回 unknown，不能自动再做一次登录或提交。普通任务与幂等记录不保存授权 URL、密码、seed、code 或 token。

## 4. 配置与本地文件

本期没有新 YAML 开关或自定义远端登录地址。能否登录取决于当前 standalone 文件型 Codex store、model-quality 插件实例、post-auth 采用 hook 和宿主可用性；Home、未知 store、配置 API Key、虚拟/派生目标等不支持的来源会拒绝。没有可从前端覆盖的 OAuth 域名、client、callback 或代理地址。

重登使用目标已有代理，否则冻结宿主默认代理；没有默认代理则直连。运行过程中不切换出口、不接收浏览器传入任意代理配置。

| 路径 | 内容与权限 |
| --- | --- |
| `<AuthDir>/.credential-login/` | 私有登录目录 `0700` |
| `credential-login.db` | 独立 bbolt schema 1，任务/幂等/HMAC 元数据/可选 vault 密文，`0600` |
| `vault.key` | 明确初始化后创建的 AES-256 key，`0600` |
| `runtime/manifest.json` | 显式安装的运行包 manifest，`0600` |
| `runtime/` 其他文件 | 已固定 SHA-256 的 worker、Node、Playwright 和许可证 |

Vault 使用 AES-256-GCM 随机 nonce，附加认证数据绑定 `auth_id`、稳定 principal、身份版本与条目 revision。没有读取明文资料 API。有密文但 key 缺失时拒绝初始化替代 key；身份变化、损坏或 revision 不匹配也拒绝采用。OAuth 提交成功而资料保存失败单独报告 `vault_error`，不会假称 OAuth 未提交。

默认固定运行边界：密码任务全局同时运行 1 个，活动任务最多 16，同目标/身份限制并发，每小时最多 3 次手动尝试；会话 15 分钟，候选确认期 5 分钟。worker 默认 180 秒，宿主总时限 190 秒，token exchange 60 秒。以上均只用于凭证获取链路。

登录库终态记录与已完成幂等记录保留 30 天；未决提交、未决幂等和不确定结果不按普通 TTL 删除。容量目标 256 MiB，其中保留维护写入空间；容量不足拒绝新工作。清理记录不等于缩小 bbolt 文件，不能通过删除数据库处理容量问题。

## 5. 提交和恢复语义

宿主从固定 token exchange 响应解析 ID/access token 的稳定账号、用户与 email，检查 issuer、client audience、有效期和 token 间冲突。此逻辑没有新增 JWT/JWKS 签名验证器，可信边界仍是宿主与官方认证端点的 TLS 兑换。输入邮箱、文件名、worker 声明或 `sid` 不能替代稳定身份。

首次导入对已存在 principal 拒绝重复创建；重登必须匹配原身份与预期版本。提交时重新检查来源、质量身份和政策；保留人工禁用、质量暂停、冷却和其他 owner。新授权不继承旧授权的“推理已验证”。

保存、质量身份和运行态采用不是跨数据库原子事务。专用提交日志与临时 fence 用于防止不确定状态被业务采用；启动对账不重新登录，也不重复兑换。`saved_not_adopted` 表示已保存但未确认采用；`commit_unknown` 需要对账，不能当作失败后无害重试。进入 committing 后取消请求不会宣称已经撤回提交。

重启时未提交内存候选丢弃，旧任务标记 interrupted；插件实例失效会取消其未提交任务。规则恢复和质量暂停释放仍是各自独立的显式动作。

## 6. 相对原方案的实际取舍

| 原方案建议 / 参考路径 | 当前交付 |
| --- | --- |
| toSub2 Node→Python→curl_cffi 协议适配 | 独立 Playwright 官方网页表单；不加载原注册/设置/导出路径 |
| 进度帧与长度前缀 IPC | 每次一个 EOF 定界 stdin JSON / 单个 stdout JSON，16 KiB / 12 KiB，固定状态和错误 |
| Node/Python 多运行时与 Linux 包移植 | Node + 锁定 Playwright；系统 Chrome 由管理员显式指定；macOS/Linux 支持 |
| 可配置并发上限与细粒度内部阶段 | 固定保守边界；UI显示授权/兑换/候选/提交阶段，不伪造密码/MFA实时进度 |
| 游标分页任务列表 | 最多最近 200 条任务列表 |
| 独立 90 天审计系统 | 登录任务与提交记录；未宣称已有独立审计导出服务 |
| 全套平台包 | 本机 macOS arm64 构建自检；Linux 代码支持但未在本机执行 Linux 浏览器验收；Windows不可用 |

## 7. 验证范围

本次没有使用真实账号、没有访问真实登录会话、没有调用任何模型。运行时验证包括本机 Chrome 实际执行离线假认证页面，RFC 6238 向量、一次密码/TOTP提交、错误/挑战停止、回调绑定、取消、有限 IPC 与进程组归属；宿主测试覆盖摘要/权限/路径检查、输出溢出、错误隔离和进程树终止。运行包自检只验证能启动/关闭浏览器及其版本摘要。

源码中的服务、提交、vault、API 与插件测试用于验证模拟场景。最终宿主全套测试、插件构建、共享库加载与实际部署结果由本次整体验收记录给出；本文不会把未执行的平台或真实 OpenAI 账号兼容性写成已通过。


## 私有登录目录边界

`auth-dir/.credential-login/` 是宿主保留目录，包含登录数据库、可选密钥和运行包。凭证存储、启动扫描、文件监听、插件凭证回调和管理文件接口统一排除此目录及指向它的符号链接。运行包中的 JSON 清单不是 OAuth 凭证，不能出现在凭证列表；管理员也不能通过凭证上传、下载或删除接口操作登录资料库。


## 2026-11-15 首版登录功能验收结果

首版在 8317 安装插件 0.4.0 / UI 5.0.0、补丁宿主与本机运行包。宿主全套测试、插件全套测试、相关 race、vet、必需编译与 c-shared 构建通过；源补丁从 8.0.67 只读归档复现通过。原生联调覆盖首次导入、显式确认、质量暂停中的同身份重登、规则停止及卸载取消。浏览器离线运行包12项、登录UI15项、已有UI66项、原生UI17项、CPA外壳19项检查通过。最终可核查记录位于 `artifacts/openai-login-0.4.0/verification-final/`。

第一次安装核查发现宿主将运行包 JSON 清单误识别为凭证，已回退、恢复原状态并补齐保留目录隔离。修复后全套复验和再次安装通过，配置、凭证文件、凭证列表、规则、质量限制、设置与历史摘要一致，保留资料库未自动初始化。最终安装记录为 `artifacts/openai-login-0.4.0/deployment-checks.json`。真实 OpenAI 账号登录尚未验证，不能以这些离线结果宣称线上成功率。

## UI 5.1.0 独立登录页验证

独立登录页通过 21 项离线浏览器检查，包括三页导航、直接 `#login`、未连接空态、当前页重连、延迟加载期间禁用操作、保留有效目标、切页清除秘密且不取消宿主任务、重复点击当前页不清空表单、旧请求错误隔离，以及连接进行中切入登录页。三种主题和手机布局截图、脱敏报告检查见 `artifacts/login-tab-5.1.0/login-ui/`；输入摘要与执行记录见 `artifacts/login-tab-5.1.0/verification-final/login-ui.json`。这组验证继续只使用离线假管理接口。
