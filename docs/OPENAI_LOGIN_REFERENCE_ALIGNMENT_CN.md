# toSub2-go-port 登录实现对照与 1.2.2 调整

2026-11-16。参考目录 `/Users/ceng/Downloads/toSub2-go-port`，HEAD 为 `2c2c841199b9ec7ba36588f6bf4ca129dd24f54e`，工作目录有本地修改。本轮以实际文件内容为准；相关 SHA-256 已记录到 `login-runtime/SOURCE_MANIFEST.json`。

## 源码核查结论

目标项目默认使用 ChatGPT 网页会话，再取得 Codex OAuth。协议请求、Sentinel SDK、CookieJar 和代理绑定在同一次操作中。它使用 Go HTTP/TLS 与 Sobek JavaScript 运行器，而当前插件使用 Chromium 作为网络、Cookie 和官方页面 SDK 的载体。当前实现没有完整移植目标底层引擎。

目标 `internal/transport/cookie.go` 明确拒绝 Cookie JSON 持久化；`cmd/tosub2/authentication_runtime.go` 为新认证准备新 CookieJar。它并不是依靠跨任务保存浏览器会话避开每次登录验证。当前插件也只保留当次任务的浏览器上下文。

## 已按目标补齐

| 环节 | 目标源码 | 1.2.2 行为 |
| --- | --- | --- |
| 操作设备标识 | `internal/legacyruntime/sentinel.go` 在 SDK 调用前将同一 device ID 写入认证来源 | 保留 ChatGPT 已有的 `oai-did`，进入认证页前同步同一标识；官方 OpenAI 来源使用其共享 Cookie 域，离线来源仅写各自主机 |
| 已完成网页会话 | `internal/protocol/flow.go` 的 `LoginWeb` 接受授权导航直接回到 ChatGPT 同源根目录 | 接受该分支，再通过 `/api/auth/session` 核验账号邮箱，成功后继续宿主 OAuth；不强制重复密码或 TOTP，邮箱不符仍停止 |
| 网站验证与业务错误 | `internal/protocol/client.go` 的 `isBrowserChallenge` 排除普通 JSON 和限流，并检查明确响应头、状态和 HTML 标记 | JSON 中出现验证资源名称不再误报网站验证；支持两个明确的 mitigation 响应头并处理大小写/空格；保留普通限流和服务错误分类 |

既有密码和 TOTP 验证仍为单次提交，使用服务器返回的真实 TOTP factor；OAuth 保留 CPA 签发的 PKCE 与 state。没有引入注册、改密、重设 2FA、持久 Cookie 或任意工作区选择。

## 回归验证

新增测试使用两个不同的主机名和本地认证代理，检查 ChatGPT 的登录参数、认证 Cookie 和 SDK 读取到的设备标识一致，避免同一 `127.0.0.1` 的不同端口共享 Cookie 导致假通过。

已有会话成功与邮箱不符分别验证，并确认该分支不发送密码和 TOTP。响应测试覆盖普通 JSON 拒绝、误标 HTML 的 JSON、JSON/HTML 限流、普通服务错误、明确验证响应头、HTML 验证冲突及包含资源名称的正常成功响应。

验证与部署记录在 `artifacts/login-reference-1.2.2/`。未变更的插件与 UI 检查只在输入摘要仍一致时复用；宿主和运行时检查重新执行。

## 真实登录边界

目标项目的 `docs/pure-go-runtime/relogin-challenge-2026-11-14.md` 记录了网站验证脚本失败，以及另一次在建立协议会话时超时的真实操作；它没有提供“复制运行器即可通过当前官方人工验证”的证据。

当前生效代理下的页面证据见 `artifacts/login-verification-1.2.1/session-rendered.png`，官方显示“Verify you are human”。本轮修改发生在首页验证后的正常认证阶段，没有证据表明它们改变了该页面结果，因此未重复消耗账号登录次数。实际账号尚未生成新授权候选，不能宣称真实登录成功。

本轮没有新增验证窗口，也没有改变用户此前的不弹窗偏好。

## 安装结果

本机 8317 已安装运行时 1.2.2，插件 0.11.2 / UI 8.9.2。78 项运行时测试通过，11 份验证记录匹配源码（其中4份未修改插件/UI验证复用）。部署前后账号、配置、凭证和业务状态摘要一致；没有进行新的真实账号登录尝试。
