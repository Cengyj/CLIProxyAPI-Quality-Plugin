# OpenAI Go＋Sobek 登录运行时

当前默认实现是 `login-runtime-go/`；本文保留运行时 **2.0.0** 的基础说明，当前安装版本见 [README](../README_CN.md)，工作区选择更新见 [2.1.2 修复说明](WORKSPACE_SELECTION_CN.md)。引擎标识 **go_sobek_protocol**。宿主直接启动 `bin/login-runtime`，不加载 Node、Playwright 或 Chromium，也没有浏览器自动回退。原 `login-runtime/` 仅保留历史源码和显式回滚兼容；旧版说明见 [浏览器运行时历史](OPENAI_2FA_BROWSER_RUNTIME_CN.md)。

## 与目标项目的关系

从用户指定的 `/Users/ceng/Downloads/toSub2-go-port` 实际移植 `legacyruntime`、`jshost`、`transport`、`authmaterial` 和相关日志代码。Go 使用目标的 tls-client 传输、Chrome 146 协议配置、Cookie jar、重定向处理；Sobek 执行官方动态 SDK 和网站 JavaScript。这里的 Chrome 146 是 HTTP/TLS 配置，不代表启动 Chrome 进程。

业务流程采用目标项目的已有账号路径：ChatGPT 首页 → providers / CSRF → signin → 密码验证 → 从真实认证响应读取 TOTP factor ID → issue_challenge → TOTP 验证 → 确认网页会话邮箱 → 同会话 OAuth → 返回回调。所有步骤共用设备标识、Cookie 和登录代理。

CPA 保留自己的 PKCE、state、授权码兑换、身份核验与候选凭证提交。运行时只返回回调，不创建第二套 PKCE、不直接上传凭证。没有引入目标项目的注册、密码重置、MFA 开通/重置、手机号绑定、用量或探针逻辑。2.0.0 当时在多工作区缺少明确目标时停止；2.1.2 已改为明确绑定优先、未绑定时组织优先再选择首个有效工作区，详见 [新规则](WORKSPACE_SELECTION_CN.md)。

`UPSTREAM_SOURCE_MANIFEST.json` 记录参考 checkout 的版本、本地改动状态及逐文件摘要；`SOURCE_MANIFEST.json` 记录适配源码。参考目录保持只读。该 checkout 没有根项目许可证，NOTICE 不将整个项目推定为 MIT；依赖声明和许可证位于 `licenses/`。

## 构建和安装

需要 Go 1.26 或以上与 Python 3。运行时本身不要求 Node 或浏览器；首次构建可能需要下载 go.sum 固定的 Go 依赖。

```sh
cd login-runtime-go
go test ./...
python3 generate_manifest.py
cd ..
python3 login-runtime-go/build.py --output /absolute/path/to/new-runtime-directory
```

构建器生成 schema 2 manifest，声明 `engine: go_sobek_protocol`、平台、架构、入口和每个文件 SHA-256。目录及执行文件权限 `0700`，普通文件 `0600`。新二进制在只含系统 PATH 的临时环境中运行 `--self-test`，实际执行 Sobek 脚本。自检不连接 OpenAI。

完整目录安装到 `<AuthDir>/.credential-login/runtime/`。宿主验证版本、引擎、平台、文件清单、摘要、权限和符号链接后直接执行入口。schema 2 拒绝 Node/browser 配置。宿主能力接口返回验证后的 `runtime` 元数据；登录页显示 **Go + Sobek · 2.0.0**。

## IPC、代理和生命周期

stdin 为宿主生成的单个 v1 JSON 输入，上限 16 KiB；stdout 为与 job_id / attempt 绑定的 v1 NDJSON 阶段事件和一个最终结果。重复字段、未知字段、类型错误及非法授权参数会被拒绝。阶段使用 `runtime_start`，历史 `browser_start` 仅用于查看旧记录。`browser_verification` 沿用历史诊断名称，当前表示 Go/Sobek 处理网站验证，不表示浏览器自动化。

密码、TOTP 密钥、代理认证和 Cookie 不进入命令行或日志。错误仅返回固定 code/stage/reason。账号与管理密钥的明文持久化方式保持原产品设置，见 [持久化说明](PERSISTENCE_CN.md)。

登录代理由宿主冻结并传给 Go 传输，授权码兑换使用相同出口。默认、单账号、导入批次的优先级保持不变。HTTP、HTTPS 和现有 SOCKS5 配置继续通过该路径。直连不继承进程代理环境。

宿主为每个任务创建独立进程组和临时 HOME/TMP。Sobek 以同一个 Go 二进制的私有 worker 模式执行，继承宿主进程组；正常结束关闭 worker，取消/超时先终止整组，2 秒后强制回收。worker stdin 关闭时最多 250ms 后退出，即使脚本同步循环也不会永久残留。业务登录默认 180 秒，宿主总期限 190 秒，仅作用于凭证获取。

网络始终正常验证 TLS。只允许固定认证/网页来源、合法 CPA OAuth 和回环回调。传输仅重试可以证明尚未发送的写请求；密码/TOTP 被拒绝或结果未知后停止。官方验证不能完成时保留明确失败状态，不伪造验证结果。

## 验证范围

离线 fake upstream 验证真实 Go 传输、Sobek SDK、Cookie、密码/TOTP、代理和 OAuth 回调；宿主验证真实原生包加载、元数据、输出限额、取消与子进程回收。插件模块独立测试并构建原生共享库，另运行宿主全量测试、编译及原生生命周期检查。

离线流程成功不等于真实账号已登录。当前安装与真实诊断结果记录于 [Go 引擎切换](OPENAI_LOGIN_GO_ENGINE_CN.md)。
