# 历史浏览器登录运行时（1.x，已由 Go＋Sobek 替代）

> 以下为 1.2.2 历史行为；当前工作区规则见 [Go＋Sobek 2.1.2 工作区自动选择](WORKSPACE_SELECTION_CN.md)。1.2.2：默认流程已按目标项目重整为 ChatGPT 网页会话 → 密码 → TOTP → 同会话 CPA OAuth，并提供实时阶段日志。1.2.1 的网站验证请求与等待状态修复见 [验证修复](OPENAI_LOGIN_VERIFICATION_FIX_CN.md)。详见 [参考登录流程](OPENAI_LOGIN_REFERENCE_FLOW_CN.md)。下方直接 OAuth 表单及单 JSON 描述保留为 1.0/1.1 历史实现；1.2.0 的进度协议以新文档为准。

## 已实现的范围

`login-runtime/` 是宿主调用的本地 Node.js + Playwright Chromium worker。它打开宿主生成的 OpenAI OAuth 授权地址，在官方表单中填写邮箱、密码与已有 TOTP 密钥生成的验证码，截获回调地址并交回宿主。PKCE、state、token 兑换、身份校验、候选凭证确认与最终写入均由宿主完成。worker 不产生第二套 PKCE，不兑换或导出 token。

这不是把登录链接展示给用户的占位实现：离线测试实际启动本机 Chrome，完整执行邮箱、密码、TOTP 表单与重定向。但尚未使用真实 OpenAI 账号验证线上表单兼容性。官方页面或挑战方式变化时会停止并返回需要人工处理；不能把离线通过描述为真实账号登录成功。

## 与 sub2api 的关系

研究基线是 sub2api `ccf997c73868083d96c0bec56e71c1a34d26c98a` 及其关联 toSub2 `8548397e89bf80e508eda64a87e0d556d43abc84`。源码记录和许可证位于 `login-runtime/SOURCE_MANIFEST.json` 与 `login-runtime/licenses/`。

参考实现先通过网页协议取得会话，再创建自己的 Codex OAuth PKCE，会涉及动态 Sentinel JavaScript、额外账户设置、独立 token 兑换和文件导出。当前运行时采用官方网页表单，让真实 Chromium 执行正常网页 JavaScript，并直接使用宿主的授权会话。只改编了 MIT 许可的 TOTP 计算逻辑，未复制整套协议登录、账户注册、2FA 设置、恢复或导出路径。

## 运行条件与打包

- 支持 macOS / Linux；其他系统返回运行时不可用。Windows 尚未实现对子进程树的可靠回收，因此不会宣称支持。
- Node.js 22 或以上，通过显式参数复制已安装的可执行文件。
- 固定 `playwright-core@1.62.1`。`package-lock.json` 锁定 npm tarball 完整性，`playwright-files.json` 记录 111 个发行文件的 SHA-256；打包前验证所有文件，无自动依赖升级。
- 已安装 Chrome / Chromium。运行时不自带浏览器，也不会在打开 UI 或登录时下载它。
- 浏览器应使用受信任系统软件包安装。manifest 固定浏览器主可执行文件的路径、SHA-256 和版本；该摘要不覆盖整个系统浏览器安装目录，系统软件包权限与来源仍由管理员管理。外部浏览器拒绝符号链接和任何人可写的文件；接受 macOS 正常安装时的本机管理员组可写权限。运行包自身仍禁止组/其他用户写入。

在项目根目录执行以下模板，路径由管理员替换：

```sh
python3 login-runtime/build.py \
  --node /absolute/path/to/node \
  --playwright /absolute/path/to/node_modules/playwright-core \
  --browser /absolute/path/to/chrome \
  --node-license /absolute/path/to/matching-node-LICENSE \
  --output /absolute/path/to/new-runtime-directory
```

打包器完全使用本地输入，先校验固定 Playwright 发行文件、复制 Node 与许可证，再用新目录运行无账号的浏览器启动自检。所有文件均写入 SHA-256 manifest；目录权限为 `0700`，普通文件 `0600`，Node `0700`。输出目录必须尚不存在，避免直接覆盖正在使用的运行时。

管理员把完整目录安装为 `<AuthDir>/.credential-login/runtime/`，宿主从该目录的 `manifest.json` 读取能力。不要只复制 worker。宿主拒绝文件摘要变化、未列出的额外文件、符号链接、过宽写权限、平台不匹配和浏览器摘要变化。浏览器正常升级后需要显式重新打包、替换运行时，不会自动接受新摘要。

示例 Node 24.22.1 的完整许可证保留在 `login-runtime/licenses/node-v24.22.1-LICENSE.txt`；使用其他版本时必须指定与之对应的许可证。

## IPC 与运行生命周期

启动命令固定为 `bin/node worker.mjs`。一次登录读取 stdin 至 EOF 的单个 JSON 对象，输入上限 16 KiB；stdout 只返回一个 JSON 对象，输出上限 12 KiB。密码、TOTP 密钥、代理凭证均不放在命令行、环境变量、日志或持久配置中。固定成功结构：

```json
{"version":1,"job_id":"opaque","attempt":1,"callback_url":"http://localhost:1455/auth/callback?code=...&state=..."}
```

运行时 1.1.0 的失败包含固定 `error_code`，以及有限枚举的 `error_stage` / `error_reason`，不会透传网页正文、浏览器异常堆栈或上游响应。宿主丢弃 stderr 内容并限制其大小；只接受约定错误码和诊断枚举，重新检查回调来源、路径、唯一 state/code 与任务标识。宿主不继承 `NODE_OPTIONS`、代理环境、Playwright 调试或其他进程环境；每个任务使用临时 HOME/TMP 目录。

宿主创建独立进程组。worker 在加载 Playwright 前包装本进程的 `child_process.spawn`，强制 Chromium 保留同一进程组，避免默认 detached 浏览器在 worker 取消后残留。Playwright 依赖本身未修改。正常退出主动关闭 context/browser；取消先终止整组，最多 2 秒后强制回收，宿主完成后清理临时目录。此机制不能承诺操作系统交换区、崩溃转储或异常断电后的物理擦除。

worker 默认授权时限 180 秒，宿主额外设置 190 秒总时限，仅用于凭证获取。网络使用正常 TLS 校验，不设置忽略证书错误。浏览器使用临时 context，不导出 cookies、storageState、HAR、trace 或截图；也不创建持久用户配置。

登录代理由宿主解析并冻结，浏览器表单和授权码兑换使用相同出口。HTTP/HTTPS 支持代理认证，SOCKS5/SOCKS5H 支持无认证代理；不支持的 SOCKS 用户名密码组合在保存时明确拒绝，运行时也进行校验。Chromium 中 SOCKS5H 映射为 SOCKS5 的远端 DNS 行为；直连显式使用 `--no-proxy-server`，不沿用操作系统代理。配置入口与优先级见 [登录代理说明](OPENAI_LOGIN_PROXY_CN.md)。

## 登录约束

- 仅接受固定 OpenAI 授权域名、Codex client ID、S256 PKCE、合法回环回调与宿主匹配的 state。
- 同一任务最多提交一次密码与一次 TOTP；错误、过期或限流均停止，没有自动重试。
- 支持 Base32 和 `otpauth://totp`，固定 SHA-1、6 位、30 秒；拒绝 HOTP、不同算法或位数、无效 Base32 与重复参数。
- 不注册账户、不修改密码、不绑定手机号、不启用/关闭/轮换 MFA，也不自动解 CAPTCHA、邮箱验证码、短信验证码或其他人工挑战。
- 不任意选择工作区；出现未能证明目标的工作区选择时要求人工操作。
- HTTP 表单请求只允许已知登录、验证、会话/工作区选择和授权路径。意外接口或表单返回需要人工处理。

## 本地验证

```sh
/absolute/path/to/node --test login-runtime/test/*.test.mjs
/absolute/path/to/node login-runtime/worker.mjs --self-test
```

测试使用本机离线 HTTP 服务模拟官方表单，测试注入入口只在模块 API 中提供，worker stdin 不接受自定义域名。覆盖 RFC 6238 时间向量、授权参数与回调拒绝、完整 Chrome 表单流程、单次密码/TOTP 提交、错误挑战、取消、IPC 限长与无敏感输出、真实 Chromium 进程组继承。宿主 Go 测试另覆盖 manifest/hash/平台权限、有限 IPC、固定错误、回调绑定及进程组回收。

运行时 1.1.0 增加正常 `/log-in-or-create-account` 入口和带备用邮箱/SMS 选项的 TOTP 表单回归，避免子串误判；并用离线认证 HTTP 代理验证完整表单链路、直连设置和固定诊断字段。已有失败记录缺少阶段时不补造历史原因。

`--self-test` 只启动并关闭浏览器，报告版本和摘要，不连接 OpenAI、不登录，也不会检查真实账号可用性。


## 1.1.1 授权页网站验证

修复授权源 `/cdn-cgi/challenge-platform/` 的 POST fetch/XHR 和 Cloudflare 验证域指定资源被误拦截的问题。由官方页面执行自己的浏览器检查；自动检查等待最多 30 秒，仍需人工操作时返回 `human_verification`，不会误提交账号表单。未知业务写接口及验证源主页面导航继续拒绝。宿主明确接受 1.1.1，未放宽为任意版本。

运行时 31 项离线测试通过。本机真实授权页可进入人工验证画面；真实账号尚未完成登录。与 toSub2-go-port 的协议、Sentinel、挑战运行链路及证据边界对照见 [登录诊断对照](OPENAI_LOGIN_CHALLENGE_ANALYSIS_CN.md)。
