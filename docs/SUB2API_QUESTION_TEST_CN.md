# sub2api 智商测试：输出设置与说明

**结论：sub2api 的手动「题目测试」没有「输出上限」或 `Temperature` 输入框，也没有统一给所有上游套一项输出 token 限额。它不是无限制：单轮并行数、题目的输出格式，以及各上游分支实际发送的参数是三件不同的事。**

核查范围：账号菜单「智商测试 → 题目测试」，不把同一弹窗的状态探针、ModelTrace、定时测试或质量规则混进来。2026-11-15 核对了 `ranxi2001/sub2api` 默认分支 `production` 的 `e90811e27bc442763a496027f935a7bb1fb4396b`，并与本地参考提交 `ccf997c73868083d96c0bec56e71c1a34d26c98a` 比较。下文涉及的题测界面、题库、输入参数和上游构造逻辑一致；本地参考工作树未切换。

## 原版界面和执行方式

| 设置 | 实际行为 |
| --- | --- |
| 账号 | 从某个账号进入。整轮请求固定使用这个账号和一个模型，不是按样本从账号池调度。 |
| 题目 | 默认是「糖果逻辑测试」，也可以选「鹈鹕骑自行车（HTML）」。两者题面均可自定义；HTML 题另外支持随机题库。 |
| 测试消息 | 自定义模式下会修剪首尾空白，再根据题型附加输出格式要求；编辑题面不会取消原题型的格式要求。模型收到的不只是文本框里的文字。 |
| 模型 | 可从账号/上游目录选择，也可手工输入 ID。打开时 `anthropic` 账号先选 `claude-opus-5-5`，其他先选 `gpt-6-astra`；目录加载后可能改用列表首项。能输入 ID 不代表上游支持该模型。 |
| 思考强度 | 根据账号和模型查询候选档与默认档。返回空列表时 UI 使用 `none`；查询失败时 UI 回退到低、中、高。表单的选择不保证每种上游协议都会发送这个字段。 |
| 并行数 | 默认 1。前端将数值取整并限制为 1–8，再同时发起 N 个独立的流式请求。`8` 是这个界面单轮的限制，不是题测接口接收的一项并行参数。 |

流程是：**冻结账号、模型、推理档与实发题面 → 每份回答单独 `POST /api/v1/admin/accounts/:id/pelican-test` → 逐个消费 SSE → 保存和比较结果。** 每个请求传 `model_id`、`prompt`、`prompt_mode`、可选的 `pelican_question` 和 `reasoning_effort`；前端还发 `mode: default`，后端题测请求结构不读取它。后端不从这个请求读取 `temperature`、`max_tokens` 或样本数。关闭弹窗会取消浏览器发出的这些请求。手动历史按账号保存在该浏览器的 `localStorage`，写入时最多保留 8 轮，与一轮最多 8 份是两个限制。

随机 HTML 题先从管理接口取一份带题号、版本、轮次、实发题面与评审项的快照，这一步不调用模型。同一浏览器标签页内，同一管理员切换账号时可复用该题；点换题会请求新轮次，不保证抽到不同题。后端按轮次确定题目，提交时重建实际提示词，校验题号及版本，而不是拿客户端改写的题面冒充题库原题。

## 上游有没有输出限制？

| 已核对的请求分支 | 输出 token 参数 | `temperature` | 思考强度 |
| --- | --- | --- | --- |
| OpenAI Responses 常规题测 | 不发送 `max_output_tokens`，由上游默认与模型限制决定 | 不发送 | 已识别的档位通过 `reasoning.effort` 发送；具体是否可用取决于上游 |
| 走 OpenAI 兼容 Chat Completions 的账号 | 该构造不发送 `max_tokens` 或 `max_completion_tokens` | 不发送 | 该构造也不发送 `reasoning_effort` |
| 使用 `createPelicanClaudePayload` 的 Claude Messages 路径 | 固定 `max_tokens: 32000`；思考也可能消耗这个额度 | 继承基础请求的 `temperature: 1` | 支持所选档位才设置 `output_config.effort`，支持自适应思考时一并构造 `thinking` |

Claude 的 `32000` 不是输入框默认值，也不是对所有平台的统一设置。同一个后端会按账号与协议分流；例如 Bedrock 测试路径仍走固定问候和较短的连接测试预算，不能用弹窗是否提供某项设置推断其实际请求。另一个细节是，OpenAI Responses 分支接受显式 `none`；这与完全省略 `reasoning.effort` 不同。题测没有专门的提示词字符输入上限，但服务端仍有全局请求体限制，上游也有自己的上下文、输出及限流边界。

## 输出格式与“成功”的含义

- **糖果题**：提示词要求仅答一个整数；原题参考答案是 **21**。手动题测实际只要求 SSE 正常完成且文本非空，不验证它是不是整数或是不是 `21`。答 `20` 仍可能显示生成成功。原版定时测试对未改动的内置糖果题另有答案检查，不应当作手动题测的逻辑。
- **HTML/SVG 题**：提示词要求返回独立 HTML，不用 Markdown 围栏、外部依赖或额外解释。前端提取却会兼容代码围栏和裸 SVG，找不到 HTML/SVG 时标为失败；能提取只说明格式可预览，不代表图形内容正确或动画达标。预览用隔离 iframe 和阻止外部资源加载的 CSP，原始回答仍可查看；文本与作品分别可下载为 `.txt` 或 `.html`。
- **执行和质量分开**：手动题测给人比较回答，不输出 IQ 分数，也不会因答错就认定账号永久“降智”。例如 Claude 报 `max_tokens` 或拒绝时是生成未完成，不是答错。答错不触发账号状态处置，但原版复用连接测试逻辑，部分上游 HTTP 错误仍可写入账号运行状态；这不是判题动作。

## 与本插件 UI 4.6.0 对照

| 项目 | CPA Model Quality 当前行为 | 与原版的差别 |
| --- | --- | --- |
| 输出上限 | 移除手动输入框。新任务的表单不提交 `max_tokens`；服务端冻结 `min(2048, max_output_tokens)` 供请求与预算使用。 | 两边都不提供该输入框；本插件仍保留全局上限、预算预留与每份响应的大小保护。 |
| `Temperature` | 移除手动输入框，不再把 0 注入新任务。 | 不覆盖新任务的温度；同配置重跑仍使用原先明确设置的值，包括 0。 |
| 回答数量 | 同样默认 1、最多 8；同一任务内逐份执行，各自为独立请求。 | 原版是同轮并行发起，本插件的「回答数量」不是并发度。 |
| 模型 / 思考强度 | 模型栏可键入，但后端要求该模型在所选凭证的宿主模型列表；思考强度默认不显式提交，由宿主的模型/协议能力与配置处理。 | 不等于原版对手输模型直接尝试的行为；页面参数也不是上游最终生效参数。 |
| 题面与预览 | 修改内置题后会按自定义题保存，不沿用内置参考答案。预览剥离脚本。 | 原版修改题面时仍按原题型附加格式说明，其沙箱预览允许内联脚本。两边的 `生成完成` 都不是答对。 |
| 资源和暂停 | 默认还限制每份响应为 256 KiB，并执行全局调用和 token 预算；质量暂停的凭证禁止手动和自动复测。 | 这些是插件的资源保护和凭证准入，不是参考项目的题目设置，不应因界面对齐而删除。 |

## 已采用的对齐方式

- **移除整个“输出设置与说明”**，不增加新的说明面板或高级设置入口。宽屏采用主栏题面、右栏模型与两项核心选项；窄屏依次排列。题库本身的输出格式要求原样保留。
- **删除客户端覆盖，不是取消服务端限制。** 新任务的冻结详情仍记录实际提交给宿主的请求预算；宿主或上游的最终参数可能不同。
- **质量规则、凭证限制、证据判定和历史不变。** 不能将截断、上游拒绝或请求错误当成答错。

## 源码定位

原版以下路径相对于 `workspaces/sub2api/`，行号以本地参考提交为准：

| 证据 | 路径 |
| --- | --- |
| 表单、并行钳制、SSE 请求、同轮快照 | `frontend/src/components/admin/account/IQTestModal.vue:125`、`:519`、`:610`、`:684` |
| 题面/格式、随机轮次、HTML 提取和预览 CSP | `frontend/src/utils/intelligenceTest.ts:3`、`frontend/src/utils/pelicanQuestionRound.ts:5`、`frontend/src/utils/pelicanHtml.ts:23` |
| 管理路由、题测请求结构与处理 | `backend/internal/server/routes/admin.go:523`、`backend/internal/handler/admin/account_handler.go:1385`、`:1451` |
| 模型推理能力、随机题库、定时测试规则 | `backend/internal/service/account_test_reasoning.go:15`、`backend/internal/service/pelican_questions.go:34`、`backend/internal/service/pelican_scheduled.go:519` |
| OpenAI / Claude 参数 | `backend/internal/service/pelican_test_service.go:88`、`:141`、`backend/internal/service/account_test_service.go:332`、`:3129`、`:3157` |

本插件：[`index.html`](../plugin/ui/index.html#L22) 手动表单；[`app.js`](../plugin/ui/app.js#L498) 请求封装；[`admission.go`](../plugin/quality/admission.go#L101) 手动默认；[`types.go`](../plugin/quality/types.go#L28) 全局默认；[`execution.go`](../plugin/quality/execution.go#L115) 样本执行与预算；[`host.go`](../plugin/quality/host.go#L117) 宿主请求；[`view.js`](../plugin/ui/view.js#L159) 安全预览。
