# 0.3.0 鉴权 API 与存储

业务前缀为 `/v8/management/plugins/model-quality`。公共 `/v0/resource/plugins/model-quality/{index.html,app.js,client.js,view.js,style.css}` 只返回静态 GET，无模型调用和状态变更。业务路由精确匹配，管理鉴权与 remote-access 策略由宿主执行；管理密钥仅存在页面内存，不保存到插件或回环请求。

## 当前规则

质量暂停后，主体、control、judge 三种角色都不能执行模型请求；自动或手动复测均不支持。旧复测/自动恢复配置不再生效。人工释放完成后，相关规则保持停止：没有剩余质量限制时为 paused，否则继续 awaiting_release。必须显式恢复规则才能产生新的检测。

## 路由

| 方法 | 路径 | 内容 |
|---|---|---|
| GET | /targets | 脱敏目标、身份、模型/reasoning、人工禁用/质量暂停/有效状态 |
| GET | /questions | 题目列表，不含参考答案 |
| GET / POST / DELETE | /presets | 保存/加载/删除配置，不执行 |
| GET / POST | /runs | 历史 / 异步提交；暂停依赖在入队前拒绝 |
| GET | /runs/detail?id= | 冻结配置、样本、control 样本和独立状态 |
| POST | /runs/cancel | `{id}`，幂等取消 |
| POST | /runs/review | `{id,sample_index,verdict,note}`，人工评审 |
| GET / POST / PUT / DELETE | /rules | 规则 CRUD；修改/删除检查 revision |
| POST | /rules/run | `{id}`，仅未暂停且版本有效的规则可运行 |
| POST | /rules/pause | `{id,revision,paused}`，停止/显式恢复检测规则 |
| GET | /actions | 宿主权威暂停动作 |
| POST | /actions/preview | `{rule_id,action_id,action_revision}`，人工释放预览 |
| POST | /actions/release | 相同动作绑定，人工 CAS 释放；始终停住相关规则 |
| GET | /history | 运维审计 |
| GET | /metrics | 质量率、覆盖率、来源尝试、用量、延迟与预算 |
| GET / PUT | /settings | 当前配置与更新 |

分页为 offset（默认0）、limit（默认50，最大100），返回 `{items,total,offset,limit}`。GET /runs 支持 source=manual_test或scheduled_probe，其他值返回400；相同时间以ID稳定排序，先分页后复制输出。创建运行返回202与 `{run_id}`，再鉴权轮询；不是 SSE。JSON body 最大128KiB，拒绝重复字段和未知字段。

错误结构 `{error:{code,message}}`；参数错误400，not_found为404，版本/幂等键冲突409，暂停状态冲突409，队列/预算429，不可用503，存储准入超限507，不支持能力422，幂等结果已过保留期410。对暂停凭证的新调用不能返回202后再消耗额度。

`pause_rule` 仅作为旧客户端兼容输入；false 不会继续启用检测。当前 UI 不提供“解除后继续自动检测”选项。

若人工释放已在宿主完成、随后剩余限制查询失败，返回503并明确“owned pause released; remaining restrictions await reconciliation”。此时相关规则继续停止，保留待对账标记；刷新动作和目标状态确认结果，不把错误响应解释为宿主必然未释放。

## 运行和规则配置

```json
{
  "auth_id":"offline-a", "provider":"offline", "models":["real-model"],
  "question_id":"logic-boxes", "samples":1,
  "parameters":{"max_tokens":1024,"stream":true},
  "judge":{"mode":"deterministic"}, "idempotency_key":"operator-job-001"
}
```

每模型 samples为1..8，总数还受 max_samples 限制；模型数、temperature、reasoning_effort、输出预算均服务端检查。custom 题可提供 prompt、review_standard，二者共同冻结；不得改内建题面后继承原答案。视觉/自定义题默认没有确定性 oracle。

可选 control_auth_id/control_provider 与 seed 用于随机题有效对照。judge mode为 deterministic/manual/llm；LLM 需要独立 auth_id/provider/model。任何依赖被质量暂停都会阻止整个运行。答案仅提供给独立 judge，不发送给被测模型。

规则增加 name、action（observe_only或pause_credential）、enabled、scheduling_enabled、automatic_pause、interval_seconds或cron、timezone、fail_threshold、retention_days。默认失败门槛3，范围1..100；间隔60..2678400秒；保留1..365天。规则 status为 detecting/paused/awaiting_release，由服务端状态导出，不能由客户端修改来绕过限制。

automatic_recovery、pass_threshold、pass_count、retest_seconds 不再是新 API 的可配置能力。旧数据库/旧宿主 YAML 在迁移时忽略其运行意义；新 JSON 设置不接受这些已移除参数。action_id、归属、身份与计数由服务端维护，编辑仍保留既有动作归属并执行 revision 检查。

全局可配置开关仅 scheduling_enabled与automatic_pause。workers、queue、data_dir 修改需要宿主重配。各资源字段范围以 types.go为准；示例在 artifacts/config。

## 存储与迁移

宿主 auth-dir/quality-state.db保存独立身份、活动暂停和已释放墓碑，插件卸载后继续生效。插件 data_dir/quality.db为schema2的bbolt，保存meta/runs/rules/audit/usage/idempotency/presets。新目录0700、文件0600。

schema1升级保留历史与动作，不删除数据库，不发起复测，不自动释放。旧queued/running标记interrupted。已暂停主体及相关control/judge依赖规则停止；schema2规则不能因旧恢复开关、历史通过结果或到期计时器自动恢复。

跨库使用持久意图与host action_status对账；释放墓碑不会被当成“尚未暂停”而重放。人工释放和对账都不会恢复规则启用。UTC每日预算计入主体/control/judge调用，未知usage单独显示；被动生产仅保存脱敏事件，原始Source、凭证和生产全文不落盘。

审计或被动指标数据损坏/读取失败时返回503，不再伪装成空历史或零指标。缺失或非法的历史流量来源归入unknown，不能自动归入production。

每日预算按调用准入时的UTC日期记账，跨日完成仍更新原日，meta内budget:YYYY-MM-DD保留配置天数并保护在途日期。主动任务缺usage仍显示未知；被动ABI缺少零用量存在性标记，因此无法证实的0和旧事件未标记tokens_known时按unknown_usage统计，不伪造零成本。
