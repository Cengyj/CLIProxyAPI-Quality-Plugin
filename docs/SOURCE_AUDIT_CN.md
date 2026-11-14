# sub2api 源码复核与设计取舍

初次核查日期：2026-11-13；题目测试复核日期：2026-11-14。实际参考目录为 `workspaces/sub2api/`，来源仓库为 <https://github.com/ranxi2001/sub2api>。

`git rev-parse HEAD` 返回 `ccf997c73868083d96c0bec56e71c1a34d26c98a`；检出时为 detached HEAD，未改用最新分支。仓库 `LICENSE` 为 GNU LGPL v3。插件 UI 和业务实现独立编写，没有复制其 Vue 组件、Go 实现或 UI 资产。

0.3.1 新增的糖果逻辑测试复用该提交 `frontend/src/utils/intelligenceTest.ts` 的 `CANDY_PROMPT` 原文，保存在 `plugin/quality/questiondata/candy-sub2api.txt`，并嵌入动态库。原题文件 SHA-256（不含末尾换行）为 `cd771ba38f8f75696ba703c7d3e91e5273588c91cb8c4145e3ce4b077c075c3f`。该文本仍适用 LGPL-3.0，不因插件主体使用 MIT 许可而改授权；来源记录见 `plugin/NOTICE.md`，完整许可见 `plugin/licenses/sub2api-LGPL-3.0.txt` 和 `plugin/licenses/GPL-3.0.txt`。

题目 ID 为 `logic-candy`，版本为 `sub2api-ccf997c7-v1`；发送前在原文后附加“只输出最终整数，不要解释。”交付约定，完整冻结题面 SHA-256 为 `df1a06950b3883d44cb2f1046164281bd7e6ba09c6792c3042e6658dfbb30eb5`。答案 21 只绑定这份准确题面；独立有界穷举测试验证圆形 9 个加五角星形 12 个可以保证目标，而少于 21 个的各形状取数都存在失败组合。修改题面必须转成自定义题，不沿用答案。手动题目测试默认不自动评分。

新增 `visual-pelican-html` 的独立 HTML / 内联 SVG 动画题面自行编写。旧 `logic-boxes`、`random-linear`、`visual-pelican` 题目 ID、版本和题面均保留，已有规则和历史仍可读取。

## 已检查文件

- 前端：`frontend/src/components/admin/account/IQTestModal.vue`、`frontend/src/utils/intelligenceTest.ts`、`frontend/src/utils/pelicanHtml.ts`、`frontend/src/views/admin/AccountQualityView.vue`、`frontend/src/api/admin/accountQuality.ts`。
- 服务：`backend/internal/service/pelican_questions.go`、`pelican_test_service.go`、`pelican_scheduled.go`、`account_quality.go`、`quality_judge.go`、`quality_random_question.go`、`quality_shared_batch.go`。
- 存储：`backend/internal/repository/account_quality.go`、`account_quality_isolation.go`。
- 说明及相关测试：`docs/account-quality-operations.md`、`account_quality_test.go`、`quality_judge_test.go`、`quality_shared_batch_test.go`，以及上述模块的测试清单。

## 可以借鉴的设计

1. 手动评测冻结题面、交付约定与参数。`pelican_questions.go` 将题目版本、具体实例及评审标准写入结果；HTTP 成功不是绘图质量结论。
2. `quality_judge.go` 区分 `correct` / `incorrect` / `unknown`，独立判题账号排除被测账号；候选与参考答案以数据形式传递。解析器拒绝重复字段、额外输出、非法枚举及过长响应。
3. `quality_random_question.go` 使用本地有界求解器。路径题限制网格规模、记忆化计算；异或题枚举有限赋值。随机实例和答案应被可靠保存，不让评测模型看到答案。
4. `quality_shared_batch.go` 将题面、答案、模型映射、已知执行路径、推理参数和判题条件纳入比较键。通过对照按其他账号去重；没有对照的错误回答仍是错误，但不构成自动处置证据。
5. `account_quality_isolation.go` 和运维文档将动作归属、版本冲突、规则删除保护、人工恢复与暂停规则分开处理；不能仅凭当前布尔状态判断所有权。
6. `pelicanHtml.ts` 使用 sandbox iframe 和前置 CSP，原文与预览分开。当前插件进一步采用禁用所有脚本的预览，并移除外部资源、导航及表单属性。

## 必须与参考实现区分的部分

- 参考实现存在账号组、按模型隔离、移除组、BPS、Jev、特定 Codex 探针等逻辑，本插件不迁移这些能力。CPA 的实际动作是独立的、宿主持久化的**凭证级质量暂停**。
- `qualityOutcome` 的参考测试允许同轮“明确错误 + unknown”归为 failed。本任务要求全部计划样本完整且可判定才能形成有效失败轮次，插件使用更严格归约；不能照搬参考判定器。
- “所有选定模型连续 3 个完整失败轮次暂停”是本插件的保守默认值，不是参考项目的既有行为。最新产品要求禁止暂停后的全部复测和自动恢复，只允许人工解除。
- `quality_judge.go` 使用总体 90 秒、单次 30 秒 `context.WithTimeout`。CPA AGENTS 禁止连接建立后的新增网络 timeout，本插件不迁移这些 deadline，改用显式 operation/cancel 及停机取消。
- 参考账号复测可以直接走账号 ID 路径。当前 CPA 插件 v0.2.0 禁止全部暂停中复测，不提供质量暂停绕过；只允许管理鉴权下核验归属及版本的人工解除。不能复制供应商执行器或临时全局启用。
- 参考 PostgreSQL 事务可覆盖其账号状态和业务表。插件与 CPA 宿主暂停状态属于两个独立存储，本项目使用幂等/CAS/对账，不能宣称跨库原子事务。
- 参考 `intelligenceTest.ts` 的糖果题答案必须与其完整题面匹配。0.3.1 将原题和版本固定绑定；编辑后以自定义题提交，服务端拒绝携带改动题面的内置题 ID。

## UI 实现边界

`plugin/ui/` 是无需外部 CDN、无需 JavaScript 框架的独立静态应用，编入原生插件。静态 GET 不调用模型、加载凭证或启动后台规则。管理员显式输入密钥后才发起同源 v8 管理 GET；密钥只在闭包内存，退出或刷新后清除。

两个页面共用同一 API。UI 4.2.0 智商测试只提供题目测试：每次一个模型、1–8 份回答、可编辑题面、进度与停止、同配置再测试、原文复制、脱敏下载和隔离预览；题面与结果在同一页面，测试记录另有入口。手动页移除预设、对照和独立判题配置，不自动评分，也不加入探针或 ModelTrace。旧多模型、预设、判题和规则数据保留兼容。质量运维仍提供规则编辑、独立开关、预算、凭证状态、动作预览、CAS 释放和运营历史。生产可用性以宿主返回为准，不以 UI 颜色代替调度限制。

`scripts/verify_ui.cjs` 和 `scripts/verify_native_ui.cjs` 保留 UI 4.1.0 的旧交互断言，不作为 UI 4.2.0 验收入口。4.1.0 的截图和测试报告属于历史材料。4.2.0 使用实际补丁宿主、原生动态库、本机 fake upstream 和 CUA 浏览器交互验证，材料保存在 `artifacts/question-test-4.2.0/`；只有本轮实际日志、报告和截图能够证明相应检查结果，未记录的项目不视为通过。
