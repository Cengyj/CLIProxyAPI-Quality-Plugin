# 官方 CPA 单插件方案

用户于 2026-11-17 明确选择：保留官方 CPA，只安装插件；使用官方已有的鉴权插件通道，保留模型测试、账号管理、Go＋Sobek 2FA 和巡检，质量运维改为观察。此决定覆盖历史 v8 专用宿主扩展和独立质量暂停的交付要求。

## 架构

```text
官方 CPA 8.0.67（未修改）
  └─ model-quality 原生插件 0.15.0
       ├─ 嵌入 UI 9.2.0、手动模型测试和质量观察
       ├─ 账号库、巡检、登录任务、自动保存并启用
       ├─ 官方 host.auth.* / host.model.* 回调适配
       └─ 内嵌 Go＋Sobek 2.1.0 原生运行包
```

插件是独立 Go module，使用官方 `/v8` 公共 SDK，不导入宿主 `internal`。登录服务由 `plugin/login/` 所有，使用原生 Go 引擎驱动密码/TOTP 协议；OAuth PKCE、交换和候选校验也在插件内完成。CPA 只负责加载插件、管理鉴权、提供公开回调和存储正式凭证。

正式包只有一个动态库，符合官方 ZIP 安装器只提取根目录动态库的规则。引擎启动前由插件解压到自己的私有目录并逐文件校验；Go helper 的 Sobek worker 也是同一个原生二进制的子进程，不依赖 Chrome、Playwright 或 Node。插件关闭时取消任务；CPA 异常退出时，helper 通过 FD 3 所有权管道结束会话和子进程。详见 [运行包说明](PLUGIN_BUNDLE_CN.md)。

## 接口

| 用途 | 路径 |
| --- | --- |
| 官方插件安装与管理 | 官方现有 `/v8/management/plugins/...` |
| 插件业务 | `/v0/management/plugins/model-quality/...` |
| 插件账号与登录 | `/v0/management/plugins/model-quality/login/...` |
| 静态 UI | `/v0/resource/plugins/model-quality/...` |

业务接口全部由官方管理鉴权中间件保护，继承管理密钥及远程访问限制；静态公开 GET 只返回页面资源。插件不修改官方接口，也不保存管理密钥后向 CPA 发起回环管理 HTTP 请求。`routes_v8` 是历史补丁扩展，当前不再注册。

账号 API 包括 `/accounts`、`/accounts/import`、`/accounts/entry`、`/accounts/detail`、`/accounts/start`、`/accounts/inspect`、`/accounts/settings`。登录状态包括 `/capabilities`、`/jobs`、`/detail`、`/logs`、`/candidate`，明确操作包括 `/cancel`、`/discard` 和 `/commit`。写入需 `Idempotency-Key`。2FA 和账号登录固定走 `password_totp`；保留的显式 `method=browser` 人工 OAuth API 仅用于调用方主动选择的授权回调，不是浏览器自动化或失败回退，当前账号 UI 不调用它。

## 官方接口的能力边界

| 功能 | 当前行为 |
| --- | --- |
| 选择凭证和模型、手动题目测试 | 通过官方公开回调执行，凭证列表以官方 `host.auth.list` 可见范围为准 |
| 已取得流后的停止 | 调用官方 `stream_close`，取消后续工作 |
| 首包前/非流式强制取消 | 无宿主 operation/cancel 扩展，不能保证立即终止上游调用 |
| 质量观察与检测规则 | 支持，默认为观察；调度需显式开启 |
| 可信最终证据 | 官方回调缺少历史补丁的最终证据，保留为未验证，不推进自动暂停 |
| 独立质量暂停/人工释放 | 不支持；API 返回能力错误，界面禁止配置 |
| 历史暂停归属 | 保留为待核对记录，相关规则保持停止；不静默清理或冒充释放 |
| 账号、代理、登录、巡检持久化 | 插件自己的明文数据库 |
| 账号登录/已授权巡检提交 | 成功后自动保存并启用，无需再次确认；历史候选不自动重放 |
| Home、远端凭证存储 | 当前不宣称支持；验证范围是 standalone 本地文件 |

官方质量能力返回 `mode=official_observation`、`quality_pause=false`、`manual_release=false`、`verified_evidence=false`、`model_operation_cancel=false`、`stream_close=true`。登录能力返回 `automatic_commit=true`、`commit_mode=account_auto_commit`、`manual_oauth_commit=true`。账号登录任务标记 `auto_commit`、`enable_credential`；只有核对 CPA 记录启用状态后才标记 `credential_enabled`。自动暂停不是仅在 UI 隐藏：配置、规则创建/恢复、调度和动作接口都拒绝不受支持的设置。

质量停用和人工禁用是不同语义，不能用 `disabled` 模拟质量暂停。历史 `ActionID` / `BlockedAuthID` 原样保留；官方模式下存在这些暂停记录的凭证不能提交重登候选，避免通过凭证覆盖绕过历史限制。没有将旧补丁宿主的数据自动迁入官方实例。

## 自动保存与启用的保证和限制

官方 `host.auth.save` 接收名称和 JSON，写入文件后更新运行时。它不提供跨进程原子 CAS，返回错误时也可能已经写入文件。因此当前实现：

1. 冻结本次登录目标的稳定身份和原文件策略指纹；保存前重新读取并核对。
2. 在原始 JSON 上更新授权字段和本次授权的启用状态，保留未知策略、大整数和高精度数值。手动账号登录授权启用目标；后台修复仍拒绝人工禁用目标。启动后发生禁用/策略变化会使快照失效。
3. 通过质量模块停止关联规则并使旧证据失效；历史质量暂停阻止提交。
4. 调用官方保存前记录未决收据，最多调用一次；写入后核对物理文件内容，并确认 CPA 运行时记录身份与启用状态一致。官方不公开运行时 token，因此不能独立证明新 token 已被内存执行器完整采用。
5. 结果不确定时保留 `commit_unknown` 或 `saved_not_adopted`，重启不自动重放写入。

这不等同于宿主原子事务。登录期间应避免其他管理页面同时修改同一凭证。账号登录成功会自动保存并启用；如果保存或启用状态无法核实，停止并显示异常，不再重复写入。保留原 JSON 的其他策略，但不能承诺官方回调未暴露的全部内存状态原样维持。启用成功不等同于模型调用验证。

巡检读取 CPA 已记录的认证诊断，不通过调用模型测试账号。只将精确匹配的认证失效作为自动登录依据，例如 `unauthorized (refresh token invalid)`；额度不足、冷却、禁用、网络错误及未知状态不触发重登。默认巡检、全局自动登录、账号自动登录均关闭。

## 来源与验证

- 原始只读参考：`/Users/ceng/Downloads/CLIProxyAPI-8.0.67`。
- 未修改构建副本：`workspaces/CLIProxyAPI-official/`。
- 基线：`host-patches/baseline.json` 的 2,303 个源码/构建资源，原参考无 Git 元数据，不虚构上游提交。
- SDK 引用只指向官方副本，正式构建逐文件检查基线和额外文件。
- 产物及摘要：`artifacts/official-plugin/`。
- 离线安装验证脚本：`scripts/verify_official_plugin.py`，使用独立 CPA、临时端口、假商店、假账号和假模型上游。

本轮兼容验证不调用真实 OpenAI 账号，也不更换本机 8317。此前 Go 引擎真实登录结果属于旧插件部署的证据，不能据此宣称新版完整插件已完成真实登录。当前实际验证平台为 macOS arm64。

## 0.13.0 官方兼容版安装基线结果

最终 ZIP 已通过未修改官方 CPA 的插件商店安装接口安装。验证覆盖管理鉴权、六份静态资源、内嵌引擎自检、流式与非流式模型测试、账号/代理/巡检/质量配置的重启持久化，以及不支持的暂停动作拒绝。两次临时 CPA 进程均正常退出；原参考、官方副本的 2,303 个文件和 CPA 二进制摘要保持一致。

插件全量测试与 race、原生引擎全量测试通过；登录 UI 16 项和账号 UI 53 项离线检查与最终静态资源摘要一致。最终安装包 SHA256：`222cf4c66e2f49d75fa02a2097d2c662556be72002d4fb480e088c7ce09464d1`。

证据：[汇总](../artifacts/official-plugin/verification-summary.json)、[官方安装报告](../artifacts/official-plugin/smoke/official-install-final/report.json)。

## 0.15.0 一键登录结果

账号登录改为成功后自动保存并启用，保留并发策略复查、取消与未决写入不重放保护。插件全量 race、带内嵌引擎的全量测试、账号/登录页 70 项检查及未修改官方 CPA 的安装/重启回归通过。详见 [一键流程说明](LOGIN_ONECLICK_CN.md) 和 [本轮报告](../artifacts/login-oneclick-0.15.0/verification-summary.json)。

## 本机 8317 已切换

用户随后明确批准保留现有账号更新 8317，已完成备份、一次性绑定转换和官方宿主安装。原 CPA 凭证内容未更改，未调用真实登录或模型。当前运行与备份证据见 [本机官方升级记录](LOCAL_OFFICIAL_UPGRADE_CN.md)。上文“不更换本机 8317”的说明指此前独立安装验证阶段。
