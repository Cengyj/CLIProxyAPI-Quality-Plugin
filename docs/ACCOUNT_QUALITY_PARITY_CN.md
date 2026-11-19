# sub2api 质量运维与官方 CPA 的对应关系

对照版本：sub2api `production@a71bcd6238777630bb74f18106097689ce7a5836`；本地官方 CPA 8.0.76；插件 0.25.1 / UI 12.17.0。范围是 `/admin/account-quality`，不是独立的手动模型测试或账号登录运维。

2026-11-19 追加核对布局版本 `production@feb761541918d25a73779c6ea42ee28f61b5591e`，插件 0.27.0 / UI 13.0.0 已实现同页概况、规则卡片、运行记录与独立编辑弹窗。当前工作台没有质量子标签页；选择规则后由服务端筛选历史再分页。详见 [布局与实现](QUALITY_WORKSPACE_13_0_CN.md)。原有功能边界见下表。

| 环节 | sub2api 的行为 | 本插件的做法与边界 |
|---|---|---|
| 账号与范围 | 手选账户，或将模板同步到现在和以后匹配的账户，可按组、类型、状态等筛选。 | 使用 CPA 凭证而非 sub2api 账户。固定单凭证；或保存一条动态规则，每轮按活动凭证与已注册模型的交集生成子任务。不复制模板子规则；空目录可以保存。 |
| 规则复用 | 从已有规则复制、保存预设、按字段批量编辑；预设与会分发的范围模板是两种对象。 | 当前按条创建/编辑；浏览器中的模型快捷项只复用模型 ID，不是规则模板。手动智商测试预设不等于运维规则预设。本次没有移植批量写入。 |
| 模型与负载 | 普通题支持多个模型、一档公共强度、每模型 1–8 个样本。默认 `gpt-6-astra`、`high`、一份；另外两项内置快捷模型为 `gpt-6.1-sol`、`gpt-5.6-sol`。 | 对齐这些默认值。按插件实例预算校验，保存不发起调用。编辑器只选模型、强度和样本，不展示探测次数。仅在手动执行动态范围前提示按当前目录预计的主测量，不含判题或重试。 |
| 题目与判题 | 固定糖果题、随机推理题；普通 LLM 排除被测账号，Jev 是另一种专用判题通道；还有与题目检测不同的单模型订阅状态探针。 | 糖果题和内置整数题按完整整数判定；随机题执行时生成并本地求解。固定题的题面与参考答案可在编辑器里按需预览；自定义题使用自己的题面/评审标准。官方回调不能核验判题/对照凭证是否独立，不能仅凭两个不同的文件名声称二者独立。官方新规则不提供该配置；兼容的旧依赖不静默移除。没有移植 Jev、BPS 或订阅状态探针。 |
| 运行与历史 | 普通题默认五段 Cron `*/30 * * * *`，按计划保留检测结果；全局运营记录按轮次展示，详情提供样本、请求条件和判定理由。新提交还区分了结果写入成功与旧历史清理成功。 | 服务端时区；表单只用一个定时开关，默认仅手动；开启只授权当前规则，不开启全局许可。按规则、按凭证保留结果，默认 100 条；范围批次可翻页。列表显示当前规则请求档位，历史显示**那一轮冻结的值**，不根据今天的规则回填。样本先持久化，失败不冒充成功；中断任务不重放。 |
| 触发与处置 | 观察、移出分组、单模型隔离、指定模型移除等策略；还有上游 5xx 触发、429 条件、401 清理、复测、恢复与归属核对。不是每个通道都允许处置。 | 官方 CPA 的凭证运行状态、冷却和人工禁用是不同条件。质量观察不变更凭证；不移植上述自动处置，不清理历史暂停归属，也不把错误/未知当成答错。 |

## 思考强度的证据分别在哪里

1. **用户请求。** 对照版本的普通题目规则含 `minimal`；CPA 插件把这一项替换为 `max`。可选档位依次为 `low / medium / high / xhigh / max`，默认仍是 `high`，支持键盘方向键。已有的 `max` 直接回显；历史 `minimal`、`none`、`auto` 或未指定值可以沿用，但不是新规则选项。`SaveRule` 校验规则档位；派发时将 `parameters.reasoning_effort` 冻结到各个子任务。
2. **CPA 转换。** 插件把请求值放入固定凭证的 `HostModelExecutionRequest.Body`。官方 `sdk/pluginapi.HostAuthModels` 只含 `id`、`auth_index`、`provider`、`models`；不从空能力表推断“不支持”，也不伪造各模型的档位表。CPA 的 `internal/thinking` 自行校验、转换，插件不绕过；`HostModelExecutionResponse`/流没有逐样本实际档位的可信回执。
3. **实际观测。** 在隔离 CPA 上对两份合成凭证、两个 Kimi 模型、每模型两份样本测试五档，共 40 次本地模拟请求。在此版本的 `kimi-k2.5` 和 `kimi-k2.6` 上，`low / medium` 的模拟上游请求为 `low`，`high / xhigh / max` 为 `high`。各轮历史仍分别保存五个原始请求值，重启后不被当前规则覆盖。这是模拟端读到的载荷，不是对供应商内部计算量的测量，也不外推到其他模型或 CPA 版本。

## 源码核对入口

- [页面与表单](https://github.com/ranxi2001/sub2api/blob/a71bcd6238777630bb74f18106097689ce7a5836/frontend/src/views/admin/AccountQualityView.vue) · [质量运维 API](https://github.com/ranxi2001/sub2api/blob/a71bcd6238777630bb74f18106097689ce7a5836/frontend/src/api/admin/accountQuality.ts) · [规则、预设和历史的加载](https://github.com/ranxi2001/sub2api/blob/a71bcd6238777630bb74f18106097689ce7a5836/frontend/src/stores/accountQuality.ts)。
- [调度与校验](https://github.com/ranxi2001/sub2api/blob/a71bcd6238777630bb74f18106097689ce7a5836/backend/internal/service/scheduled_test_service.go) · [题目/探针分支、多模型执行与保存](https://github.com/ranxi2001/sub2api/blob/a71bcd6238777630bb74f18106097689ce7a5836/backend/internal/service/pelican_scheduled.go) · [判题服务](https://github.com/ranxi2001/sub2api/blob/a71bcd6238777630bb74f18106097689ce7a5836/backend/internal/service/quality_judge.go)。
- [模板同步](https://github.com/ranxi2001/sub2api/blob/a71bcd6238777630bb74f18106097689ce7a5836/backend/internal/service/quality_rule_template.go) · [质量判定与多模型结论](https://github.com/ranxi2001/sub2api/blob/a71bcd6238777630bb74f18106097689ce7a5836/backend/internal/service/account_quality.go) · [归属、处置与历史](https://github.com/ranxi2001/sub2api/blob/a71bcd6238777630bb74f18106097689ce7a5836/backend/internal/repository/account_quality.go) · [上游运维文档](https://github.com/ranxi2001/sub2api/blob/a71bcd6238777630bb74f18106097689ce7a5836/docs/account-quality-operations.md)。
- 本插件：[规则保存](../plugin/quality/rules.go) · [动态范围与冻结](../plugin/quality/scope.go) · [CPA 模型请求](../plugin/quality/host.go) · [执行与证据](../plugin/quality/execution.go) · [离线原生包验证](../artifacts/quality-rule-workbench-0.25.1/native-smoke.json)。
