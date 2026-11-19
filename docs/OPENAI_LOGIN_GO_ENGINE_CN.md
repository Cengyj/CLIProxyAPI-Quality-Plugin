# 切换目标项目的 Go＋Sobek 登录引擎

> 本文保留运行时 2.0.0 的移植和当时部署记录。当前工作区选择规则以 [插件 0.17.1 / 运行时 2.1.2 的修复说明](WORKSPACE_SELECTION_CN.md) 为准；旧版“不自动选择多个工作区”的限制已由该规则覆盖。当前安装版本见 [README](../README_CN.md)。

## 当时已安装的执行链

当时本机 8317 运行插件 **0.12.0 / UI 8.10.0**，登录运行时 **2.0.0**。能力接口验证后返回：

```json
{"engine":"go_sobek_protocol","runtime_version":"2.0.0","schema_version":"2"}
```

实际执行链是 CPA → `bin/login-runtime` → Go TLS/HTTP 与 Cookie jar → 同一二进制的私有 JS worker → Sobek。目标项目中的 `legacyruntime` 是 Go 包名称，不代表继续使用旧 Node 运行时。当前运行包只有 Go 入口和来源/许可证清单，不包含 Node、Playwright、worker.mjs 或浏览器路径。

## 目标源码与适配边界

实际移植 `/Users/ceng/Downloads/toSub2-go-port` 的 100 个核心源文件，参考 checkout HEAD 为 `2c2c841199b9ec7ba36588f6bf4ca129dd24f54e`；checkout 含本地变动，因此同时记录每个源文件摘要。包包括 `legacyruntime`、`jshost`、`transport`、`authmaterial`、`logging`。采用其 Sobek、tls-client、Chrome 146 HTTP/TLS 配置、SDK 加载、Cookie 和网站脚本执行代码。

已有账号的密码/TOTP流程按目标 `internal/protocol/flow.go`、`client.go` 与 `cmd/tosub2/sentinel.go` 适配。MFA 的 HTTP Referer 为 challenge URL，而 `password_verify` SDK 上下文保持 `/log-in/password`，与目标实现一致。

CPA 原有管理协议、任务日志、OAuth PKCE/state、授权码兑换、身份核验和候选确认保持由宿主管理。2.0.0 当时的重新授权工作区来自已有凭证的 account_id；未绑定且存在多个工作区时停止。此限制已由 [2.1.2 的明确绑定优先、组织优先策略](WORKSPACE_SELECTION_CN.md) 覆盖。账号注册、改密、手机号、MFA 开通/重置没有纳入本插件。

Sobek 子进程适配宿主的进程组管理：取消时可回收全部子进程；父管道关闭时能中断脚本死循环。这个适配避免直接原样复制目标独立进程组后产生残留。

## 验证与安装

`artifacts/login-go-2.0.0/verification-final/` 保存 11 份绑定源码摘要的通过记录：宿主全量测试与构建、补丁复现、插件独立测试与共享库构建、原生生命周期、运行时全量测试与构建、真实原生包宿主加载、UI 构建和登录 UI 检查。另有 `runtime-login-race.log` 记录登录包竞态检查通过，宿主要求的 test-output 编译检查已执行。

完整离线测试实际运行 Go 传输与 Sobek，覆盖 Cookie/设备标识、密码/TOTP、宿主原始 OAuth 参数、错误/断连不重放、HTTP 认证代理、身份和回调校验、输入限制、取消及固定错误输出。公共线上 SDK smoke 测试不在 routine 测试中启用。

`deployment-checks.json` 验证实际安装包、能力接口、静态资源和版本；安装前后配置、账号、代理、凭证及质量业务摘要相同。运行包自检明确 `node_required:false` 和 `browser_required:false`。

## 真实账号结果

2026-11-17 本地服务更新后，对此前失败的同一账号发起一次真实登录诊断。任务 `e241d17a-b489-496e-891f-3c5b54ba6395` 已完成密码、TOTP、OAuth、工作区核对和回调，宿主完成授权码兑换及账号身份核验，状态为 **awaiting_commit（等待确认导入）**。

实际观察到的登录执行文件仅为 `login-runtime`；日志包含 `runtime_start`，没有 `browser_start`。整个登录过程没有打开浏览器窗口。已得到候选凭证且确认邮箱与请求账号一致，尚未执行手动提交，也没有调用模型或改变质量暂停。

证据：`artifacts/login-go-2.0.0/real-login-result.json`、`verification-summary.json`。这是本次账号的真实成功记录，不代表所有账号或未来上游版本都已验证。

安装后的 UI 检查确认显示 **Go + Sobek · 2.0.0**，管理密钥刷新自动重连、账号与代理界面仍正常。该 UI 检查使用的 headless Chrome 仅访问本地管理页；它不是登录引擎的一部分。
