# 0.3.1 / UI 4.2.0 题目测试验收

2026-11-14，macOS arm64。当前证据目录为 `artifacts/question-test-4.2.0/`，与下方历史报告分开。只使用本机 fake upstream，没有调用真实模型或复制生产凭证。

- 独立插件 `go test -count=1 ./...` 通过；`go test -race -count=1 ./quality ./cmd/model-quality` 通过。糖果题新增穷举与准确题面绑定测试，证明这版题面答案为 21。
- UI 静态构建、`node scripts/test_question_test.mjs` 和实际 `go build -buildmode=c-shared` 通过。宿主 Go 源码没有改动，本轮未重跑宿主全量测试或重新编译宿主。
- `python3 scripts/verify_question_test.py` 的 8 组实际宿主/C ABI检查通过，最终报告为 `native-api.json`，绑定最终源码、库、宿主和实际返回资源的哈希。覆盖题库不泄漏答案、手动双样本不评分、改题无答案继承、HTML、旧规则暂停、暂停拒绝测试、人工解除后规则仍停止。共 7 次 mock 调用。
- CUA 浏览器验证使用另一个独立实际宿主：多回答、首包前取消、取消后历史立即更新、HTML沙箱、同页结果、390px手机布局及深色主题通过；最终实例3个任务、4次mock调用。截图与范围记录在 `browser-verification.json`。初次发现的历史陈旧及手机留白已修复并复验。
- 已将同一动态库安装到常驻 `127.0.0.1:8317` CPA，5个静态资源与源码逐字节一致，插件有效启用。常驻实例没有创建测试任务，配置和数据保留；旧库备份在 `previous-installation/`。两个独立mock实例均已停止，常驻CPA继续后台运行。

最终截图：`installed-cpa.jpg`、`final-desktop.jpg`、`final-mobile.jpg`、`final-dark.jpg`、`final-html-preview.jpg`、`final-cancel-history.jpg`。只读审阅结论通过，无交付阻断项。没有进行真实模型质量测量、实体手机或 Windows/Linux验证。

复跑入口为 `scripts/test_question_test.mjs` 和 `scripts/verify_question_test.py`；旧 UI 4.1 浏览器脚本不作为当前版本验收。API脚本首次自身属性访问错误的失败记录及后续旧构建报告保存在 `native-api-history/`，没有覆盖为通过。

---

# UI 4.1.0 本轮验收与历史记录

2026-11-14 的界面精修验收：65项fake API浏览器检查、10项实际宿主浏览器检查、独立插件tests/race/vet、实际共享库构建和装载生命周期、宿主fresh tests/race/compile均通过。完整命令、哈希、前后截图、检测告警处理与未验证范围见 [UI_POLISH_REPORT_CN.md](UI_POLISH_REPORT_CN.md)。本轮证据根为 `artifacts/polish-4.1.0/verification-final2/`，候选解包报告单独绑定安装归档哈希。

以下为保留的0.3.0历史验收说明；其中38/9检查数量及 `artifacts/verification/` 路径不代表本轮结果。

---

# 0.3.0 验收记录

本轮语义：暂停后禁止全部复测，仅人工解除；quality capability v2 / 插件 schema 2。旧版复测恢复验证不作为本轮验收。

日期：2026-11-14（Asia/Shanghai）。平台：macOS Darwin arm64；Go 1.27.1；Chrome 157。仅使用本地 fake upstream、临时凭证和独立数据库，没有真实账号调用或生产凭证复制。

## 实际执行

| 验证 | 结果材料 |
|---|---|
| 插件独立 module `go test ./...` | artifacts/verification/plugin-tests.log |
| 插件 `go vet ./...` | artifacts/verification/plugin-vet.log（零诊断） |
| 插件 `go test -race ./...` | artifacts/verification/plugin-race.log |
| 实际 `go build -buildmode=c-shared` | artifacts/model-quality.dylib、model-quality.h |
| 宿主 `go test ./...` | artifacts/verification/host-go-test.log |
| 宿主相关 race | artifacts/verification/host-race.log（auth/pluginhost/usage/httptransport 全包）；manual-only-release-scope-race.log 补充人工请求作用域 |
| 宿主要求的 test-output 编译检查 | artifacts/cli-proxy-api-quality；artifacts/verification/host-build.log |
| 真实 C ABI 装载/生命周期 | artifacts/verification/native-e2e.log |
| 前端静态构建与交互 | artifacts/verification/ui-results.json、artifacts/ui/manifest.json |
| 真实 standalone + 浏览器 | artifacts/verification/native-ui-results.json |
| 原始归档不变 | scripts/verify_baseline.py --reference；baseline-verify.log |
| 补丁干净基线可应用且文件相同 | artifacts/verification/patch-reproduction.log |

根 module 测试不会覆盖插件独立 module，因此两者均单独执行。原生生命周期测试是 opt-in 的实际动态库测试，不能用普通模块测试的 skip 代替。

## 本轮新增覆盖

- 同凭证长请求不占用其他空闲 worker，全部依赖原子准入；取消任务回收容量，无网络调用重发。
- 定时任务捕获规则版本；编辑规则或关闭定时开关后，旧 tick 不会使用新配置调用模型。
- 最终质量动作重新验证主体、对照和 judge 身份/状态，身份改变打断连续证据。
- 关闭/故障拒绝变更；并发 Close 等待同一结果；原生实例重配等待已有管理请求租约释放，失败重配不发布已关闭实例。
- 跨 UTC 日迟到用量更新原日账本；旧预算兼容、时钟回退、在途日期保留、超大token溢出均有确定性回归。
- 分页稳定、当前页输出快照无共享可变对象；审计/指标损坏返回503；错误分类不受模型名中 incidental 单词影响。
- 预取消不创建 operation；操作打开后取消不提交模型；流异常与取消完成竞态释放一次；RPC错误保留HTTP状态且不泄露完整宿主错误；结束后追加文本、无效usage被拒绝或标记未知。
- 前端按官方管理UI来源重构，覆盖会话断连清理、旧响应、重复操作与轮询错误。参考来源见 REFACTOR_CN.md。

## 保留的核心覆盖

- 原生测试固定 auth/provider，不换号；标准 host SDK 请求映射、thinking、payload 末屏障测试保留。最终条件或配置摘要变化使样本 non-comparable。深复制冻结参数防止调用方后续修改。
- 多片流、心跳、原始 JSON ABI 分片、SSE [DONE] 被 translator 移除后的真实上游终止证明；缺终止、空输出、截断、拒答、错误事件均不作为答错。首包前、流式及非流式取消，实例隔离、完成/关闭竞态和生命周期清理均有回归测试。
- 本地随机生成可复现且枚举解唯一，严格整数/LLM judge 格式，重复键、null、额外输出拒绝；标准答案不发送给被测目标。随机对照缺失、全错、路径/映射/指纹不一致、陈旧/窗口外、被替换或自身别名均不能动作。
- 401、429、网络错误、unknown、缺样本、mixed/部分正常、旧规则、手动任务不推进凭证暂停；暂停后即使收到通过结果也不触发恢复。连续身份/条件变化重置计数。
- 三轮明确失败→真实凭证级暂停；固定请求、所有模型、重试、粘性/WS 后续业务轮次、最终传输检查受限。准备中暂停不再发上游；已准入传输自然结束。
- 全面禁止暂停中复测：手动、定时、规则、control、judge 入口均受限，不存在私有绕过；旧 quality_retest 与 verified release 显式拒绝。人工释放须绑定当前插件实例、有效鉴权管理回调和动作版本；缺省、错误、过期、关闭实例和已取消请求均拒绝。人工释放只删自身限制，人工禁用、原生冷却和其他 owner 保留。
- 集中事务先持久化停止状态再执行宿主动作，写失败不发布虚假归属；取消依赖任务，清空计数，使旧规则版本失效。释放后即使 pause_rule=false 也保持规则停止；剩余 control/judge/其他 owner 暂停仍显示 awaiting_release。多 blocker 重复对账不抖动版本，全部解除也不自动启用规则。
- schema 1 升级 2 保留动作绑定、历史和已完成样本，移除旧恢复字段并停止相关规则；排队旧任务及被替换 judge 均不能消费新额度。
- 刷新不改变持久 incarnation；已观察删除重建、材料替换、旧 action/revision、退役动作重放均被保护。runtime 无稳定身份明确不支持动作，同源子凭证限制按 ID，不扩大源文件修改。
- 队列满、重复提交、相同幂等键不同内容、重复 tick、取消、重启 interrupted、不重发、预算、跨库 active/released/absent 对账、孤立暂停、宿主人工释放入口。
- 未鉴权 v8 返回401；公共页面、连接、保存规则/预设不调用模型。管理密钥仅内存。原文转义与严格 sandbox/CSP 测试包括外部资源、脚本、表单、导航和 SVG 动态属性。

## 真实链路

C ABI 测试使用真实 `.dylib` 和原生 OpenAI-compatible 执行器，共8次离线调用，验证手动、流式/心跳/用量、缺终止拒绝、首包取消、三轮暂停、业务阻断、全部复测入口零新增调用、有效管理请求人工释放、释放后规则保持停止、卸载后保持、权威库重启后保持及管理员释放。

浏览器测试使用打补丁的完整 standalone server、配置式 fake API keys、真实管理鉴权和插件静态资源。双样本正确回答与别名/reasoning 可见；三轮明确失败后显示暂停；暂停期间普通请求没有到达上游；目标/规则/control/judge 请求均在入队和预算消耗前返回409；预览并人工释放后规则仍暂停且旧失败计数清零；只有另行恢复规则才新执行。浏览器共9项断言、6次显式 fake upstream 调用，暂停期间零新增调用。Mock UI另有38项检查，包含会话、明暗主题与移动端回归。最终报告和截图以 native-ui-results.json 为准。测试结束停止 demo。

## 基线测试问题

全量验证发现两项原归档问题并在独立未修改基线复现：no-copy 守卫把 `[]weightedPoolMember` 结构体切片移动误当字节写入；Home heartbeat 用过窄的墙钟等待导致波动。分别补充明确的结构体复制审核记录，以及用重连事件同步替代负向时间窗口。另一个 WebSocket 测试退出时未等待 hijacked handler，补充等待防止 TempDir 清理竞态。均保留原语义断言，没有绕过测试或关闭检查。历史失败日志用于问题追溯，最终命令结果见最终日志。

## 明确未验收或限制

仅 macOS arm64 实机验证。Linux、Windows、FreeBSD 未在本次运行验证。Home 质量动作不支持。最终 payload 自动证据首版只覆盖原生 OpenAI-compatible chat/completions，其他执行路径为观察。停机期间完全相同材料的文件重建无法从当前宿主身份 API 识别；详细边界见 DESIGN_CN.md。没有通用 IQ 分数、自动删除或外部通知。
