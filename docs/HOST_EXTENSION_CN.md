# 宿主扩展契约 v2

本扩展基于所记录的8.0.67源码归档与/v8公共SDK。C ABI形状与既有schema_version=10保持兼容，产品能力通过 `host.quality.capabilities` 协商。0.2.0要求version=2、manual_release_only=true及禁止复测；原版宿主及旧v1能力不满足当前产品语义。

## v8管理分发

management.register的routes_v8相对于 `/v8/management/plugins/<pluginID>`，只接受精确路径，不支持通配符、参数或目录穿越。注册上下文包含V8BasePath。既有Routes/Resources兼容行为保留，没有新增v0管理业务。

v8动态路由使用宿主管理鉴权、remote-access与Home控制策略。公共resource GET只提供静态资源，不拥有人工释放权限。

## 权威状态回调

| 回调 | 行为 |
|---|---|
| host.quality.capabilities | v2能力、手动释放策略和已验证证据路径 |
| host.quality.targets | 稳定ID、模型/reasoning、独立状态、身份和对照摘要；不返回秘密 |
| host.quality.actions | 当前宿主权威动作 |
| host.quality.action_status | active/released/absent及绑定，用于跨库对账 |
| host.quality.pause | 幂等创建凭证级限制；真实native owner/身份/版本检查 |
| host.quality.release | 仅在活动鉴权管理请求作用域内人工释放匹配绑定 |
| host.quality.release_verified | 已废弃，明确拒绝自动恢复 |

quality_retest请求和旧复测上下文入口同样明确拒绝，不再提供暂停绕过能力。保留旧字段/符号只是为了返回准确的不支持错误，不代表能力仍可用。暂停影响普通业务、手动测试、规则检测、control和judge等新执行。

Binding为auth_id、owner_plugin、rule_id、rule_revision、action_id、action_revision、evidence_run_id、identity_version、reason、created_at。revision为uint64，首次action_revision=1，时间UTC/RFC3339。owner来自真实native实例，不能靠请求body冒充其他插件。

comparison_identity根据稳定账号/凭证材料生成，不包含provider路由别名，从而排除跨provider标签的自身别名。它不替代每条auth的持久identity_version。缺稳定身份时不能作为独立随机对照或安全暂停目标。

## 人工释放权限

插件管理处理器只把当前鉴权management.handle的host_callback_id用于该次人工释放。宿主验证该句柄仍有效、属于同一native实例且确为受保护管理路由；后台任务、过期句柄、其他实例和公共resource不能借用。回调句柄不保存到任务、数据库或后台定时器。

人工释放只删匹配owner/action/revision的质量限制，不改Disabled、不清冷却，不保证凭证有效可用。插件先持久化相关检测停止状态；成功后也不自动恢复规则。宿主以下接口不依赖插件，可供管理员处理卸载后的孤立限制：

- GET `/v8/management/credentials/quality-pauses`
- POST `/v8/management/credentials/quality-pauses/release`

管理员CAS入口可处理已经删除/替换凭证的旧绑定，仍需完整匹配动作。

## 执行、取消与传输准入

host.model.operation_open返回实例隔离operation_id；execute/execute_stream携带该ID。host.model.cancel可在首包前、流中、非流式调用中取消；重复关闭安全，完成/quiesce/卸载回收。后台执行不保存管理RPC回调句柄。

质量限制独立持久化并参与普通选择、固定auth、重试、会话与最终HTTP RoundTrip/WS发送检查。准备期间出现暂停不能继续提交，已通过最终传输准入的业务可自然结束。不新增推理网络deadline，不改变payload语义。

## 最终条件证据与来源

模型回调evidence包含verified、reason、requested_model、resolved_model、auth_id、payload_fingerprint、payload_config_version、conditions_changed、path。流式打开和读取可返回不同快照，最后快照为准；后续尝试能使早期证据失效。

OpenAI同协议转换返回裸JSON事件并移除SSE [DONE]。upstream_complete只有执行器确实观察到上游终止标记才为true；不能把Done或合成终止当完整证据。payload_config_version取实际配置快照，最终指纹排除独立会话/传输装饰，保留问题、有效参数与解析模型。

当前最终条件观测仅覆盖原生OpenAI-compatible的chat/completions输入路径，位于thinking、映射与payload规则之后，只读不改写。其他路径在未暂停时可手动观察，不能形成自动暂停证据。

UsageRecord.TrafficSource从内部context传递，取production/manual_test/scheduled_probe/judge/internal_other/unknown；缺失不默认production，旧Source不参与分类。EventKind=attempt，重试尝试不等于入站请求。元数据不写入上游业务payload。

## 验证

宿主及插件均单独运行模块测试、必要race与编译；原生动态库通过MODEL_QUALITY_LIBRARY执行安装/生命周期测试。0.2.0的核心断言是暂停后所有检测入口都不产生上游调用、旧授权拒绝、只有人工释放有效、释放后规则仍停止。最新命令、平台与结果见TEST_REPORT_CN.md；旧0.1.0“复测后自动恢复”的日志不代表v2能力。
