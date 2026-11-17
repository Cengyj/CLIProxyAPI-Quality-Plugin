> 2026-11-17：官方兼容版已改为独立插件，详见 [官方单插件方案](OFFICIAL_PLUGIN_CN.md)。本文保留补丁宿主的历史实现/部署记录；本机 8317 已按用户确认更新为官方 CPA + 插件 0.14.0，详见 [本机升级记录](LOCAL_OFFICIAL_UPGRADE_CN.md)。

# 已核实的架构事实

当前 CPA 参考路径：`/Users/ceng/Downloads/CLIProxyAPI-8.0.67`。

sub2api 参考提交：`ccf997c73868083d96c0bec56e71c1a34d26c98a`，仓库为 https://github.com/ranxi2001/sub2api 。实施前重新核实实际源码。

## v0.2.0 产品决策覆盖

用户最新确认：**全部禁止复测，仅人工解除**。本节覆盖下文早期方案中关于“暂停期间复测”和“验证后恢复”的产品决策，不改变对 CPA 与 sub2api 原始源码的事实记录。凭证一旦质量暂停，停止依赖它的检测规则、排队任务与进行中的评测（包括作为对照凭证或判题凭证）；删除自动恢复开关、复测周期与恢复计数，宿主拒绝旧复测授权及自动恢复接口。解除只能通过管理鉴权入口明确执行，并且只解除指定归属的暂停。解除后相关规则仍停止、旧失败计数清零，需要管理员显式恢复规则。插件 v0.2.0 使用数据库 schema 2，迁移保留动作归属与历史，不重发旧任务。当前实现与安装要求以 `ARCHITECTURE_REVIEW_CN.md`、`DESIGN_CN.md` 和 `INSTALL_CN.md` 为准。

## 现有 CPA 能力

- `sdk/pluginapi/types.go` 的 HostModelExecutionRequest 支持 `auth_id` 和 `forced_provider`，可以精确绑定凭证。`sdk/api/handlers/model_execution_test.go` 的 TestExecuteModelPinsExactAuthAcrossPrioritiesWithoutFallback 验证了固定凭证及禁用凭证不回退语义。
- `internal/api/server_management_v8.go` 已注册 `PATCH /v8/management/credentials/status`，处理器位于 `internal/api/handlers/management/auth_files_fields.go` 的 PatchAuthFileStatus。
- 上述状态操作并非统一的 JSON disabled 写入：普通凭证经 authManager.Modify 与同步 hook 更新；配置式 API Key 经 excluded-models 配置更新；虚拟凭证按源文件及其展开的运行时凭证处理，虚拟子项不能独立切换。插件不能读取一个 JSON 文件改 disabled 后认为所有来源都支持。
- `sdk/cliproxy/auth/conductor_execution.go` 的 VerifyInference 明确拒绝 Disabled 或 StatusDisabled 的凭证。这是最小推理验证，也不是任意评测题目的通用复测接口。
- `host.auth.list/get_runtime/models` 可以用于枚举目标和模型；`host.auth.models` 不提供完整的 reasoning 能力。现有 ABI 未发现带 owner/revision 的凭证质量暂停与恢复接口。
- `internal/pluginhost/auth_callbacks.go` 的凭证条目构造会隐藏部分已禁用 runtime-only 目标。暂停后的查询需区别列表隐藏与实际删除，不能只依赖普通列表。
- Auth 的 RegistrationEpoch/CredentialVersion/Generation 可辅助并发保护，但需要核查生命周期；它们不是现成的暂停所有权 API，普通状态变化与进程重启不能被误当成管理员接管或凭证替换。

## 需要明确补强的边界

- `internal/pluginhost/management.go` 和 `internal/api/server_management.go` 的插件自定义接口仍使用 v0 分发，新增功能需要独立的 v8 插件注册/分发能力。保留 v0 兼容，不给废弃端点增加新业务。
- `/v0/resource/plugins/...` 是公共 GET，只能用于静态页面外壳。示例中的公共 GET 调模型行为不应作为本插件的生产设计。
- 插件 ManagementResponse 是缓冲的完整字节响应，首版使用异步任务和鉴权轮询。路由为精确匹配，不支持动态占位符。
- `host.model.stream_close` 在有 stream_id 后可关闭流，但模型回调缺少与 HTTP operation_open/cancel 对等的完整取消契约。首包前与非流式取消需要补强。
- 后台回调不得复用已结束 RPC 的 host_callback_id。模型 operation、流及插件实例需要明确生命周期。
- UsageRecord.Source 不是可靠测试来源，可能包含账号信息或密钥。测试/判题来源需要可信内部元数据；usage 不一定等于一个入站请求，需区分尝试和最终请求。
- 插件 SDK 示例存在旧 /v7 module 元信息；新插件必须按照实际 /v8 SDK 配置。独立 module 不能依赖根模块测试自动覆盖，也不能 import 宿主 internal 包。

## 初版产品决策（复测/恢复部分已被顶部 v0.2.0 决定覆盖）

- 不引入分组系统，不以按模型隔离作为首期动作。使用凭证级质量暂停，效果覆盖该凭证所有模型的新业务执行。
- 质量暂停与人工禁用分开记录，生产可用性同时考虑两者。单纯在插件页面标记“暂停”不满足要求，宿主调度必须实际阻止使用。
- 复测只能绕过本规则的质量暂停，不得临时将凭证全局启用，也不得绕过人工禁用、其他 owner 或原生冷却。
- 同一凭证首期只允许一条自动暂停规则，减少恢复归属冲突；其他手动测试和观察规则可以并存。
- 默认采用保守的多模型门槛：所有选定检测模型本轮均有完整、明确失败证据，连续达到门槛后才暂停；部分模型通过或证据不足则仅展示疑似异常。恢复要求全部必选模型连续完整通过。
- 若使用独立质量暂停记录，原生 Disabled 仍反映人工禁用；插件和宿主查询要展示有效暂停及来源，不制造状态已同步的假象。

## sub2api 值得借鉴的部分

- 手动「题目测试」的界面字段、不同上游的实际输出参数及 CPA 插件移除输出设置后的差异，见 [sub2api 题目测试对照](SUB2API_QUESTION_TEST_CN.md)。
- 手动题目测试主要面向人工评审：逻辑题、HTML/SVG、冻结参数、多样本、安全预览。生成完成和质量正确分开。
- 质量运维采用 correct/incorrect/unknown，明确答错与网络、鉴权、限流、判题失败区分。
- 随机题使用本地求解答案、版本化题目和同批同条件通过对照。
- 所有权、陈旧任务版本保护、复测恢复、审计比复制账号分组/BPS 动作更适用于本项目。
- Codex 门票探针是特定路径的启发式，不是通用质量证明；ModelTrace、BPS、Jev 均不属于本次核心范围。
- sub2api 的固定网络超时与 CPA AGENTS 不同，不直接搬用。
