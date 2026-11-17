# OpenAI 2FA 登录源码调研

调研日期：2026-11-14；实施补充：2026-11-15。第 1–8 节保留首次调研的已读源码事实与当时状态；新增实现见第 9 节及 [OPENAI_2FA_IMPLEMENTATION_CN.md](OPENAI_2FA_IMPLEMENTATION_CN.md)。首次调研未登录真实账号、未提交密码或 TOTP、未调用模型，也未运行外部登录程序。后续仅用离线假认证页面验证新本地运行时，仍未验证真实账号。

## 1. 固定参考与核查范围

| 来源 | 实际版本 | 用途 |
| --- | --- | --- |
| sub2api | `ccf997c73868083d96c0bec56e71c1a34d26c98a` | 首次导入、重登任务、Worker、加密、身份核验、轮换与导出 |
| toSub2 | `8548397e89bf80e508eda64a87e0d556d43abc84` | sub2api 固定依赖的真实密码/TOTP 协议程序 |
| CPA | 工作区内 8.0.67 归档副本及现有 quality v2 补丁 | OAuth、凭证写入、调度、暂停、插件接入 |
| 当前插件 | `0.3.0`，quality schema 2 | 两页 UI、原生接口、任务和权限边界 |

sub2api 的 `tools/reauth-runtime/Dockerfile:4` 固定 toSub2 提交，`:11` 在构建时下载。该依赖的 `src/protocol-login.mjs` 不在 sub2api Git 树内；本次通过固定提交的 codeload 源码包补齐调研，选取的源码保存于 `workspaces/tosub2-reference/`，逐文件 SHA-256 在其 `SOURCE_MANIFEST.json`。

协议入口 Git blob SHA-1 为 `ff2db2507e401c29fa78705e44f5b40a3108a8c6`，与 sub2api `tools/openai_codex_oauth_adapter.py:18` 的固定值一致。下载源码包 SHA-256 为 `ddb061cff7c87b5c931615ef10e904795852a6bfef8f23d9998634ca77ade9a0`。没有使用最新上游替换任何现有参考基线。

许可：sub2api 根目录为 LGPL-3.0；本次取得的 toSub2 固定提交为 MIT，版权归属见其 LICENSE。参考不等于复制；以后如复用协议源码，必须保留 MIT 通知、固定提交和修改清单；若复用 sub2api 的代码则另行满足其许可，不能把其 Python adapter 当成 MIT。

尝试读取 OpenAI 官方 Codex auth/security 文档时，官方站点在命令行与浏览器均返回 403。本报告不据此断言官方当前是否支持某种自动登录协议；下述非公开网页流程只是固定源码中的实现，未证明当前真实账号可用。

## 2. “2FA”对应四种不同业务

| 业务 | 实际作用 | 当前插件方案是否纳入 |
| --- | --- | --- |
| 首次 2FA 登录导入 | 邮箱、密码、TOTP 换取 OAuth 凭证并创建账号 | 纳入，但重做执行与落库边界 |
| 已有账号重登 | 重新获取授权，匹配身份后更新已有凭证 | 纳入，仅管理员手动发起 |
| 2FA 轮换 | 修改上游账号绑定的 TOTP 长期密钥 | 不纳入登录功能 |
| ChatGPT Session 导出 | 取得并导出 Web Session/Cookie 等敏感结果 | 不纳入 |

这也不同于 sub2api 用户登录管理后台时的 TOTP。上游 OpenAI 账号二次验证与管理后台二次验证不是同一种凭证，也不能互换权限。

## 3. 首次导入的真实链路

```text
OpenAITwoFAImport.vue
  → 每行邮箱 / 密码 / TOTP，前端最多 3 条并发流水线
  → POST two-fa-login（credential_target=operations）
  → Go 内存 job
  → 配置的外部 ReloginEndpoint
  → GET job 返回 OAuth token 结果
  → 前端 importWithUpstream 去重并导入
  → 前端登记加密 password/TOTP，启用巡检与自动重登
  → 清理内存 job 和页面秘密
```

关键事实：

- `frontend/src/api/admin/accountTokenGuard.ts:11`：批量最多 100 行，支持 `----` 或逗号；邮箱去重，要求三个字段。该 parser 会 trim 密码；当前插件不能照搬这种会改变密码原文的处理。
- `frontend/src/components/account/OpenAITwoFAImport.vue:69`：流水线并发为 3，覆盖登录、导入和凭证运营登记。轮询错误保留 job ID，重试复用它，删除未确认时不释放该身份的槽位。
- `backend/internal/handler/admin/account_token_guard_handler.go:128`：请求体上限 16 KiB、响应 no-store；operations 入口首先检查加密器是否就绪。
- `backend/internal/service/openai_two_fa_login.go:44`：最多 32 个内存 job，执行期限 25 分钟、结果保留 30 分钟；重启不恢复这些 job。DELETE 取消本地 context 并删除结果，不能证明外部服务没有完成登录。
- `backend/internal/service/account_token_guard.go:1236`：请求携带 `action=start`、`auth_mode=password_2fa`、email、password、mfa_secret。默认远端地址位于同文件 `:262`，不是 OpenAI 官方域名。
- `openai_two_fa_login.go:38`：operations 入口明确不另写 legacy guard 明文设置。旧兼容入口仍是另一条行为路径，不能混为一谈。
- `frontend/src/components/account/CreateAccountModal.vue:7126`：使用 `update_existing=false`、`skip_existing=true`；拿到 created/skipped 的真实 account ID 后另行登记登录资料，设置 `enabled=true`、`auto_relogin_enabled=true`。

结论：首次登录、账号导入、登录资料登记是多个独立步骤，不是一个数据库事务。存在“登录完成但导入失败”“导入完成但登记失败”的不同结果。token 会短暂经过浏览器。这些都是迁移到当前插件时应调整的边界，不能称为已经统一由本地 Worker 完成。

## 4. 已有账号重登

源码主线：`backend/internal/service/openai_oauth_reauth.go`、`repository/openai_oauth_reauth_repo.go`、`tools/openai_oauth_reauth_worker.py`。

1. 校验账号类型、登录配置和加密状态，捕获旧凭据快照。
2. PostgreSQL 保存任务；partial unique index 限制每账号一个活动任务。
3. Worker 使用专用权限领取任务；服务端按需解密 password/TOTP，提供选定代理和预期 workspace。
4. `local_worker` 调用本地协议包；`session_studio` 调外部服务。两种引擎不是同一信任边界。
5. Worker 回传 callback 或 credentials；服务端校验 task、worker、状态、旧凭证快照及候选身份。
6. 同一数据库事务写账号、完成 task 并插入 scheduler outbox。

关键行号：

| 证据 | 位置 |
| --- | --- |
| 创建持久任务、原凭证快照 | `service/openai_oauth_reauth.go:618` |
| 活动任务唯一约束 | `migrations/254_openai_oauth_reauth.sql:40` |
| FIFO、SKIP LOCKED、worker/attempt 归属 | `repository/openai_oauth_reauth_repo.go:172` |
| callback/direct credentials 入口的 CAS | `service/openai_oauth_reauth.go:957` |
| account/user/email 严格匹配 | `service/openai_oauth_reauth.go:1218` |
| ID/access token 间冲突检查 | `service/openai_oauth_reauth_identity.go:61` |
| 事务性写入及 outbox | `repository/account_repo.go:1242` |

其身份解析明确不是 JWT 签名验证：只是在可信 Worker/服务端 token exchange 边界中解析 claim，不能把解码结果当成任意上传 token 的认证证明。已有身份字段必须在本次 token 中存在并匹配；不能用 Worker metadata 或管理员输入补齐缺失字段。`sid` 可能是新会话标识，不能当稳定 workspace；明确 `chatgpt_account_id` 优先，相关修正见 `docs/reauth-identity-validation.md`。

两项不能迁移的行为：

- `account_repo.go:1259` 的重登会设置 active、清除 rate-limit/overload/temp-unschedulable，service 另有 runtime blocker 清理。当前插件必须保留人工禁用、质量暂停、原生冷却及其他 owner。
- 任务表无 cancelled，Worker API 未提供等价的逐任务取消；过旧 running 允许被重新领取。不能把“租约过期”视为旧进程和旧网络请求已经停止，更不能因此重发登录。

## 5. 密码/TOTP 协议到底做了什么

以下均来自取得的 toSub2 固定提交，未实际执行。

`src/protocol-login.mjs:989` 的 `loginChatgptWeb` 先创建独立 Cookie/session 上下文，取得登录所需 CSRF 信息，跟随 ChatGPT→认证站点的正常登录流程。它可遇到密码、邮箱 OTP 或已有 Web session，不是所有账号都能走密码登录。

`verifyPassword`（`:1111`）提交密码；成功后检查是否有 MFA challenge。`completeTotpMfaIfNeeded`（`:1135`）从 challenge 中选 TOTP factor，取得该 factor 的挑战，用已有 seed 生成动态验证码，提交验证后继续跳转。没有 TOTP factor、额外验证或未知页面需要单独处理，不能当成“2FA 已通过”。

`generateTotp`（`:1363`）使用 Base32 seed、30 秒计数、HMAC-SHA1 和六位结果。**长效 TOTP seed 不是六位验证码，也不是 OpenAI API Key。** 原程序支持交互式重新输入密码/验证码；sub2api Codex adapter 将其改为一次失败即停止。

Web 登录成功后，`runCodexOauth`（`:1394`）再创建另一条 Codex OAuth 授权：自行生成 verifier、challenge、state，选择 session/workspace，取得 callback。最后 `buildSub2apiOauthExport`（`:1674`）调用 `exchangeOAuthCode`（`:1746`），换取 access/refresh/id token，写 sub2api 导入格式。

这解释了三个重要差异：

1. Web session 成功不等于已取得 Codex OAuth，更不等于模型推理可用。
2. 原程序自行持有 PKCE verifier 并交换 token；**没有现成的“接受 CPA 授权会话、只返回 code”的稳定 Worker 契约**。接入 CPA 必须适配这一边界。
3. 原导出 `:1705` 使用 `claims.sid` 填 account ID；不能复制这个字段推断，应由宿主从本次可信 token 响应中按明确身份字段核验。

原工具还包含添加密码、设置 TOTP、账号资料创建、手机号绑定、Session/检查点输出、代理重试等路径。它们不属于本插件登录需求。不能简单调用原 CLI 加几个参数便宣称已经满足作用域限制。

sub2api `tools/openai_codex_oauth_adapter.py` 对固定 blob 做逐点改造：关闭原始 console 输出，固定单次出口、不启用 Cloudflare solver，密码/TOTP 拒绝时结束、锁定 workspace、验证 callback，提供仅包含枚举的诊断通道。这些设计可以参考，但 adapter 自身仍属于 sub2api 许可边界。

还核实了原程序不能直接复用的传输细节：`tls-transport.mjs:316` 的超时只结束 pending Promise，没有取消 Python 网络请求；`:358` 的 stdout 行缓冲没有同等有界保护；`:389` 的 close 发信号后不等待退出；`:557` 对本地代理关闭 TLS 证书验证。新的认证进程必须保持证书验证，必要的自定义 CA 单独受管理配置，取消必须终止并回收整棵进程树。`protocol-login.mjs:2336` 只按部分 localhost/path 判断 callback，宿主必须额外校验完整 scheme/host/port/path、state 和重复参数。

## 6. 运行包与秘密处理

- sub2api 托管包：Node 22.22.2、Python 3.12.12、curl_cffi 0.15.0、jsdom 26.1.0，来源见 `tools/reauth-runtime/Dockerfile` 与 `requirements.txt`。另一个外部 Worker Dockerfile 使用不同依赖版本，两者不能混用为一份“统一锁定版本”。
- 托管 runtime 在 `backend/internal/reauthruntime/runtime.go:67` 只接受 Linux amd64/arm64 和 release 版本。当前插件的 macOS arm64 动态库验收不能证明该运行包在 macOS 可用。
- `openai_oauth_reauth_worker.py:1355` 把邮箱/代理放 argv、password/TOTP 放子进程环境；新方案使用私有 IPC 并限制继承环境，不能继承管理密钥或 Worker token。
- `repository/credential_encryptor.go` 与 `aes_encryptor.go` 保存加密登录资料，AES-256-GCM、随机 nonce，文件 key 0600。已有密文但 key 丢失时禁止生成替代 key。该结论不表示 sub2api 所有 OAuth token 都加密。
- TOTP 轮换先保存候选 secret，再激活并验证；不确定时只对账，不重复 enroll。这种“外部动作不确定”的处理值得借鉴，轮换功能本身不迁入。

## 7. CPA 已有能力和真正缺口

| 已有能力/缺口 | 源码证据 | 对实施的影响 |
| --- | --- | --- |
| v8 OAuth start/status/cancel | `internal/api/server_management_v8.go:57` | 可复用认证权限与业务服务，不让插件保存管理密钥回环调用 |
| callback 单独注册、按 state 验证 | 同文件 `:14` | 公共静态资源仍无副作用；OAuth callback 不是普通公开管理接口 |
| Codex PKCE、授权 URL、token exchange | `internal/auth/codex/openai_auth.go:25` | 不在插件或 Worker 再复制 token 交换器 |
| 指定 auth_id 重登、身份匹配 | `auth_files_v8.go:143`；`auth_files_provider_oauth.go:475` | 保留原凭证 ID，不能靠文件名覆盖完成导入 |
| 没有启动时持久身份 CAS | `oauth_sessions.go:40`；`auth_files_provider_oauth.go:558` | 需防旧登录覆盖期间的新授权/删除重建 |
| 取消不等于终止在途 token exchange | `auth_files_provider_oauth.go:258`；`oauth_sessions.go:440` | 需会话 cancel context、进程树退出确认 |
| 保存与运行时采用分别记录 | `auth_files_provider_oauth.go:590` | UI 不把 saved=true 当成业务已可用 |
| 普通 manager 更新可忽略持久化失败 | `sdk/cliproxy/auth/conductor_lifecycle.go:588` | 必须有可靠提交路径，写失败不能显示成功 |
| 普通 Disabled 更新分支会清 cooldown | 同文件 `:569` | 专用登录提交不得顺带清原生限制 |
| 普通替换 grant 改质量身份，refresh 不改 | `sdk/cliproxy/auth/quality.go:162` | 重登与刷新不能混称为同一次状态更新 |
| 暂停仍按 auth_id 阻断 | 同文件 `:420` | 新授权不会自动解除旧暂停 |
| 旧身份暂停的宿主管理释放 | 同文件 `:356`、`:395` | 需独立管理员动作，不能拿新授权当恢复证据 |
| 登录完成可选 VerifyInference | `auth_files_provider_oauth.go:613` | 新登录服务必须硬性禁用，避免暂停后复测 |
| SDK AuthProvider 是反向扩展 | `sdk/pluginapi/types.go:311` | 它不是插件调用内置 OAuth 的桥 |
| host.auth.save 是原始凭证写入 | `internal/pluginhost/auth_callbacks.go:118` | 不是安全登录、身份 CAS 或事务提交服务 |
| Home 管理权优先 | `internal/api/server_management.go:213` | start 与 commit 均拒绝不支持的 Home 操作 |

## 8. 本轮验证结论

已核对 sub2api HEAD、相关源码与测试定义，并补齐固定依赖的协议源码及许可证；协议文件 blob 与 adapter 固定校验匹配。源码证据哈希保存在 `artifacts/research/openai-2fa/source-evidence.json`。

未运行 sub2api 单测、未安装登录运行时、未验证真实登录成功率。测试源码中的离线通过条件不能代替现场账号验收。当前交付仍是 model-quality 0.3.0；本轮新增的是调研与实施方案，尚未新增可运行的登录功能。

## 9. 2026-11-15 实施补充：官方网页表单运行时

当前已新增宿主登录服务、插件登录工作台和 `login-runtime/`，不再停留于设计阶段。实际本地引擎使用固定 `playwright-core@1.62.1` 与管理员安装的 Chromium，沿宿主授权 URL 操作官方邮箱/密码/TOTP 表单；没有把原始 toSub2 CLI 直接包成生产 worker。

采用这条路径是因为固定协议源码包含动态 Sentinel、第二套 PKCE、设置账户资料/手机号、2FA enrollment 与 token/session 导出，完整移植会扩展本次作用域。新 worker 仅改编 MIT 的 TOTP 计算逻辑，保留来源、许可证和修改说明；正常网页 JavaScript 由真实浏览器执行，遇到验证码或额外安全挑战即停止，不自动解决挑战。

宿主仍是 PKCE verifier、token 兑换、候选身份与提交的唯一所有者。worker 私有 stdin 读取一次任务，只输出匹配 state/redirect/job/attempt 的 callback URL。打包和宿主校验固定 Node/worker/Playwright 文件摘要，并记录系统浏览器主文件摘要；不在打开插件页面时安装任何依赖。

离线测试已经实际启动本机 Chrome，在本地假页面完成邮箱、密码、TOTP和回调流程；还覆盖错误停止、取消、缓冲上限与进程组继承。该证据证明实现可执行及约束有效，不证明 OpenAI 当前线上表单不会变化，不证明真实账号登录或模型质量。具体安装、测试范围和未验证边界见 [OPENAI_2FA_RUNTIME_CN.md](OPENAI_2FA_RUNTIME_CN.md)。
