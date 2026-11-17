# OpenAI 授权页验证诊断与目标项目对照

日期：2026-11-16。当前插件 0.11.0 / UI 8.9.0，登录运行时 1.1.1。

## 本次故障证据

用户提供任务 `2bbcd21f-6660-47d4-ad71-08503bb4b52e`：进入 authorization 后约 1.806 秒返回 `interaction_required / unsupported_request`。旧记录没有保存请求路径，不能追溯该任务当时的精确 URL。

使用该账号当前生效的代理，只打开官方授权页、不填写邮箱或密码进行复现，确认旧运行时会阻断两类必要请求：

1. `auth.openai.com/cdn-cgi/challenge-platform/…` 的 POST XHR。
2. `challenges.cloudflare.com/turnstile/…/api.js` 及验证子域的相关资源。

仅允许第一类请求时，页面明确显示浏览器扩展或网络配置阻止安全验证；两类请求均正常加载后，页面显示人工勾选 “Verify you are human”。复现证据在 `artifacts/login-fix-1.1.1/authorization-observation.json` 与 `authorization-page.png`。诊断记录不保存查询参数、请求体、Cookie 或验证 token。

这是授权页网站验证阶段的问题，尚未验证账号密码或 2FA，因此不能把它归类为密码错误、TOTP 错误或账号异常。

## toSub2-go-port 实际实现

只读核查目录：`/Users/ceng/Downloads/toSub2-go-port`。当前默认认证引擎是 Go HTTP/TLS 与 Sobek JavaScript worker；`legacyruntime` 是包名，不代表当前仍启动 Node、Python 或 Chromium。默认 `chrome146` 是 TLS/HTTP 客户端配置名称。

| 环节 | 目标项目行为 | 当前插件行为 |
| --- | --- | --- |
| 认证入口 | 默认先完成 ChatGPT 协议会话，再推进 Codex OAuth | 直接使用 CPA 签发的 Codex OAuth 会话打开真实 Chromium |
| 会话 | Go transport、权威 CookieJar、代理和设备信息共用 | 每次任务创建独立浏览器上下文并使用账号生效代理 |
| Sentinel | 下载官方 loader 和动态 SDK，在 Sobek worker 中运行；网络请求交回同一 transport | 由官方页面在真实浏览器中加载和执行 |
| Cloudflare | 特定挑战响应触发 parent/child worker；验证新的 clearance 后才继续原请求 | 由官方页面在浏览器中执行；1.1.1 修复本地对必要资源的误拦截 |
| 密码和 TOTP | 解析协议状态与真实 factor ID，发送一次性验证请求 | 操作官方登录表单，页面负责其协议请求 |
| 失败诊断 | 独立网站验证阶段、请求数、HTTP 状态和脚本错误类别 | 固定阶段与原因枚举；本次不再把已识别的网站验证请求误报为未知业务请求 |

默认入口与统一会话绑定见 `cmd/tosub2/job_runner.go:277`、`:292`、`:336`；代理初始化见 `cmd/tosub2/authentication_runtime.go:26`。默认授权方式见 `internal/protocol/authorization_method.go:16`，网页协议入口见 `internal/protocol/flow.go:280`。

### Sentinel 与网站验证是不同机制

Sentinel 从官方 loader 发现同源版本化 SDK，执行 `token` 和 `sessionObserverToken`，不是写死 token。其 fetch/XHR 和 Cookie 更新通过父进程回到认证会话。见 `internal/legacyruntime/sentinel.go:57`、`:108`、`:160`。

Cloudflare 的识别依赖响应头或特定状态下的挑战 HTML，普通 JSON 错误、业务拒绝和限流不都当作网站验证。见 `internal/protocol/client.go:335`。

`internal/legacyruntime/challenge.go:69` 的处理顺序为：

1. 读取本次会话中缓存的、包含 `_cf_chl_opt` 的挑战响应。
2. 创建 parent JavaScript worker；出现验证 frame 后创建 child worker。
3. 通过父进程处理网络、Cookie 及 frame 消息。
4. 等待新的 `cf_clearance`，再用同一会话回访同源主页。
5. 确认回访响应不再是挑战后，协议层才允许按规则继续原请求。

它不以 Cookie 存在或 HTTP 200 单独判定成功。没有新 clearance、多 frame、不支持的脚本能力、回访仍是挑战都会失败。默认预算 120 秒。一次性授权导航和禁止重放的操作仍受限制。见 `challenge.go:249` 与 `internal/protocol/client.go:189`。

### 密码与 2FA

密码验证带有当前 `password_verify` Sentinel 上下文，必须确认协议状态前进才算通过。TOTP 从返回会话中选择真实 factor，先 issue challenge，再生成当前验证码并提交 verify；明确拒绝后不会无限重交同一个自动验证码。见 `internal/protocol/flow.go:476`、`:551`、`:621`。

### 能力与证据边界

目标项目包含网站验证脚本运行链路，但不是完整浏览器，也没有 CAPTCHA 图像/点击挑战的人工接管 UI。Sobek 宿主不提供所有 Web 平台能力；缺失能力可能导致脚本失败。文本输入通道用于密码、邮箱验证码、TOTP 和电话/SMS，不能把它当成 Cloudflare 浏览器接管。

其文档 `docs/pure-go-runtime/README.md:48` 区分合成夹具测试与真实线上验证。`docs/pure-go-runtime/relogin-challenge-2026-11-14.md:106` 还记录一次真实重登在会话建立阶段超时，未进入账号输入或网站验证阶段。因此，源码中存在 Challenge 实现并不证明照搬后当前账号就能成功。

## 已落地修复与验证

运行时 1.1.1 只为官方验证资源开放所需来源和路径：授权源验证目录的 POST fetch/XHR，Cloudflare 验证域及其子域的指定脚本/子 frame 资源。未知账号写接口、注册/重置等路径仍受原有限制，验证来源不能替换主授权页面。

自动浏览器检查页面有最多 30 秒的等待窗口。官方仍要求人工验证时返回 `human_verification`；不会生成或模拟人机验证答案，也不会误把该页面当成邮箱/TOTP 表单。测试用等待时长和来源覆盖只通过内部依赖注入提供，外部 worker 输入不能启用。

运行时 31 项离线测试通过，包括 8 项新增浏览器验证回归。宿主全量测试、编译、补丁可复现检查、插件独立测试/共享库构建、原生生命周期和已构建运行包的宿主加载检查通过。证据位于 `artifacts/login-fix-1.1.1/verification-final/`。

本次真实网页检查未填写账号秘密，未产生授权候选或上传凭证；实际账号登录尚未成功。目标项目值得借鉴的是统一会话、分阶段诊断和严格的成功判据，不能把更换认证引擎本身当作成功证据。
