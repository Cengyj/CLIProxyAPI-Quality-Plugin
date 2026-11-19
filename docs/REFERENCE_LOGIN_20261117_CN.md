# 2026-11-17 登录与账号管理对照研究

本轮根据用户要求，重新阅读当前插件及 `/Users/ceng/Downloads/toSub2-go-port-2026.11.17` 的界面、账号流程和相关实现，并据此优化正在 CPA 8.0.76 容器中运行的插件。

参考目录按用户提供的归档名称记录。本文不据此推断上游版本号或 Git 提交。参考项目只读；未复制其运行配置、账号、数据库或日志。现有插件与参考项目均有较长的历史文档，具体行为优先以本轮阅读的源码为准。

## 两个项目的工作模型

当前插件是 CPA 原生扩展。`plugin/ui/` 提供不依赖前端框架的页面；`plugin/login/` 负责账号、任务、巡检、安全操作与凭证提交；`login-runtime-go/` 提供内嵌 Go/Sobek 登录引擎。插件使用公开 SDK 回调，由 CPA 负责管理鉴权及正式凭证存储。官方宿主的质量能力仅为观察。

参考项目是独立 Go 控制台，前端由 React 组件与 hooks 组成。它同时管理登录、授权文件、Sub2API/CPA 推送、账号用量、套餐、到期信息、回收站及号池监控。参考页面上的某个按钮通常背后还有独立的服务端状态、恢复流程与存储约束，不能仅移植按钮就宣称插件支持对应能力。

两者都把批量导入与实际认证分开。当前插件已明确采用账号行的一键 2FA 登录：点击即授权成功后保存并启用 CPA 凭证。参考项目的重登弹窗还可选择授权方式和推送目标；本轮保留当前插件的一键语义。

## 功能和交互对照

| 领域 | 当前插件在本轮开始时的行为 | 参考项目的做法 | 本轮取舍 |
| --- | --- | --- | --- |
| 账号列表 | 邮箱、CPA 状态、当前操作、时间、操作类型及行按钮；20 条分页 | 在相似表格中还提供选择、用量、套餐、有效期和多种账号操作 | 保留 CPA 工作区结构，修正状态筛选与操作反馈，不添加缺乏后端支持的列 |
| 当前状态 | 账号状态、最近任务与诊断已分开；部分展示逻辑散落在渲染函数中 | `describeJob` 统一生成当前操作、状态、摘要、历史提醒和流程结果 | 提高当前操作及筛选的一致性，继续区分成功、失败和结果待核对 |
| 读取故障 | 任一账号/设置读取失败会隐藏账号表；不完整 `items` 可被当成空列表 | 验证完整快照后更新，网络故障保留最近数据并标出同步中断 | 保留最后成功内容，展示读取错误和恢复入口；失效数据不授权新的写操作 |
| 轮询 | 活动任务约 2 秒、空闲约 6 秒，使用 epoch/序号拒绝过期返回 | 单次读取、失败退避、页面隐藏或离线暂停、恢复后重新读取 | 保留已有过期响应保护，关注失败恢复、请求重叠及局部故障隔离 |
| 筛选 | 全部、待处理、已绑定；“登录中”实际跳至全部任务 | 统计、筛选、选择与执行状态分别管理 | 将进行中筛选与实际活动任务对应，包含安全操作和保存阶段 |
| 登录出口 | 服务端返回实际代理及来源；界面主要显示账号覆盖值与全局设置 | 登录请求、账号出口与推送配置有明确归属 | 展示当前生效出口及来源，说明账号、默认登录、凭证和直连的优先级 |
| 代理协议 | 部分界面文案仍声称 SOCKS 不支持认证；不同登录阶段需核对同一语义 | Go 网络层支持 SOCKS 与代理认证，区分连接和目标地址解析 | 核对并补齐 SOCKS5H/认证的一致性，避免验证阶段与 OAuth 阶段走不同语义 |
| 行内日志 | 本次/历史、异常筛选、原始记录、复制、跟随最新已存在 | 在这些能力外按账号读取历史并分页，读取失败保留内容 | 优先修复账号历史受全局最近 200 条任务截断的问题，已有交互不重复建设 |
| 安全操作 | 重置 2FA、退出所有设备及后续重登选项；未知结果隔离 | 弹窗显示所选执行顺序；任务区逐步展示完成、未执行和待核实 | 增加执行顺序预览与分阶段结果，让新密钥已保存和后续失败同时可见 |
| 资料查看 | 按现有用户要求显示完整邮箱、密码和 2FA，关闭时清除临时内容 | 支持逐项复制、整条复制、隐藏/显示，关闭时清除读取内容 | 保留当前明确的明文展示偏好，不照搬默认隐藏行为 |
| 批量导入 | 1–100 条，支持密码中的分隔符与空格；重复账号整批拒绝 | 更多输入格式、最多 500 条及部分保存回执，失败保留内容核对 | 保留当前简单格式与原子导入约束，改进实际行号、数量、重复输入及提交前反馈 |
| 批量执行 | 当前是逐账号登录；全量巡检已有入口 | 跨页选择、批量重登/重置/上传及受理、跳过、未知回执 | 作为独立后续功能，不用前端循环冒充完整批量事务 |

## 本轮采用的改进范围

### 读取恢复与筛选准确性

“还没加载”“读取暂时失败”“没有账号”是三个不同状态。只有收到结构有效的空账号列表，才能显示真正的空状态；请求失败或结构不完整时应保留上次成功的账号与设置。页面应给出可见的更新状态和重新读取操作，恢复后移除过期的错误提示。

保留旧表格不代表继续允许提交。登录、修改资料、安全操作和设置保存仍需要当前有效状态与版本。断开连接、切换工作区和新请求覆盖旧请求时，旧响应不得重新填回秘密或覆盖新选择。

“进行中”按活动状态筛选，包括排队、登录、兑换、校验、保存和取消中的任务。待处理应覆盖需要人工关注的账号与确定的异常结果；不将正常人工取消一律标红，也不把后续步骤未完成误称为旧授权一定失效。后端通常已经为手动失败设置 `attention_required`，前端补齐的是一致性与防御性展示。

### 实际登录代理与 SOCKS5H

账号响应已有 `effective_proxy_url`、`proxy_source`、`proxy_error`。界面应展示实际来源，避免账号覆盖值为空时让用户猜测最终网络出口。常见来源包括 `account`、`login_default`、`credential`、`cpa`、`direct`；是否存在 CPA 全局代理层，以当前公开宿主适配实际能提供的配置为准，不虚构可读取的宿主能力。

SOCKS5H 的目标域名应交给代理解析；认证能力要在参数校验、原生引擎以及最终 OAuth 兑换中保持一致。代理用户名和密码不应散落在常驻列表、错误消息或日志中。保存代理设置只影响后续登录，不触发网络探测或实际登录。

当前部署运行在 Docker Desktop 中，`127.0.0.1` 指容器本身。使用 Mac 上的代理时，可配置 `host.docker.internal:<端口>`；这是一条部署帮助，不应自动修改用户保存的代理。

### 安全操作流程预览

重置 2FA 的流程按选项展示：确认并保存新密钥，随后可退出所有设备，再重新授权并保存启用 CPA 凭证。退出所有设备的流程省略重置步骤。界面必须区分用户选择执行的步骤、已经确认完成的步骤、失败后未执行的步骤与结果待核对的步骤。

保留原安全边界：新密钥确认后先可靠保存；结果未知不得重放远端修改；重启、刷新、巡检或本地收尾均不能重新执行重置/退出。`完成本地保存` 只完成已有可信证据对应的本地保存，不追加登录或设备退出操作。

### 账号范围历史

本轮开始时，账号日志从浏览器已读取的全局最近 200 条任务筛选。其他账号任务很多时，当前账号的旧任务可能完全不在该快照中，导致“历史记录”缺失，甚至最近任务无法映射。

应由账号范围的管理读取返回对应任务，例如 `/jobs?account_id=<账号 ID>&limit=200`，由客户端对同一账号读取日志。全局任务列表继续用于全局工作区；账号历史不再依赖全局排序窗口。筛选须在服务端限制数量之前进行，并保留管理鉴权。界面仍如实说明每个账号和每次操作的保留上限，不声称能恢复已清理历史。

### 导入校验

仍使用 `邮箱----密码----2FA`，每次 1–100 个账号。解析可以跳过空行，但错误位置必须使用用户输入的原始行号。空内容、超限、重复邮箱及必填项不完整应在提交前给予清楚反馈；对服务端已存在账号的提示须与当前读取可信度配合。

密码中的空格及分隔符按既有解析规则保留。长期密钥不是六位动态验证码。客户端校验帮助管理员定位输入问题，服务端继续负责规范化、身份重复检查与原子保存；客户端不得把已知失败或未知写入结果自动重新提交。

## 源码依据

参考根目录：`/Users/ceng/Downloads/toSub2-go-port-2026.11.17`。

| 路径与符号 | 本轮研究内容 |
| --- | --- |
| `README.md`、`README-Go.md` | 独立控制台能力、构建方式和实际操作边界 |
| `web/src/main.jsx`：`createBatch`、`submitReloginAction`、账号工具栏 | 导入与执行分离、选择、批量受理、连接故障提示 |
| `web/src/components/accounts.jsx`：`JobRow`、`TotpActionDialog`、`LoginMethodBadge` | 表格、行操作、验证输入、安全工作流预览 |
| `web/src/components/relogin-action-dialog.jsx`：`ReloginActionDialog` | 显式重登与可选推送、能力限制及流程说明 |
| `web/src/components/account-operation.jsx`：`AccountOperationSummary` | 操作类型与本次结果组合展示、日志入口 |
| `web/src/lib/jobPresentation.js`：`describeJob`、`describeWorkflow`、`describeBatchSubmission` | 当前与历史诊断、部分成功、未知结果与受理回执 |
| `web/src/hooks/serverState.js`：`validateAccountSnapshot`、`useAccountPolling` | 完整响应验证、旧快照保留、失效请求拒绝 |
| `web/src/lib/polling.js`：`createPollingTask` | 单次读取、退避、页面/网络可见性、只重试读取 |
| `web/src/hooks/jobLogs.js`：`useJobLogPages` | 账号日志读取、分页和读取失败保留 |
| `web/src/components/job-logs.jsx`：`JobLogs` | 本次/历史、异常筛选、原始日志和自动跟随 |
| `web/src/components/account-info-dialog.jsx` | 资料读取、秘密生命周期及复制反馈 |
| `internal/transport/proxy.go`、`socks.go`、`tlsclient.go` | 登录网络出口、SOCKS 与 TLS 传输适配 |
| `internal/console/` 与 `cmd/tosub2/` 中的账号、重登、2FA 流程 | 按服务端能力与持久化证据解释 UI，不从按钮推断成功 |

当前根目录：`/Users/ceng/Downloads/CLIProxyAPI-Quality-Plugin-main`。

| 路径与符号 | 对照点 |
| --- | --- |
| `plugin/ui/login.js`：`createLoginWorkbench`、`loadAccounts`、`renderAccounts`、`schedule` | 账号状态、轮询、读取失败、筛选和按钮可用性 |
| `plugin/ui/login.js`：`jobsForAccount`、`loadAccountLogs`、`renderAccountLogs` | 全局任务窗口与账号日志范围 |
| `plugin/ui/login.js`：`showAccountSecurity`、`renderDetail`、`parseAccountImport` | 安全流程、诊断、导入和资料生命周期 |
| `plugin/ui/index.html`、`plugin/ui/style.css` | 三个登录子工作区、弹窗、表格、主题及移动布局 |
| `plugin/ui/client.js`：`ManagementClient` | 管理鉴权、接口与查询参数允许列表、过期请求拒绝 |
| `plugin/login/accounts.go`：`Account`、`accountView`、`ImportAccountsWithProxy`、任务状态同步 | 实际代理来源、重复导入、版本、关注状态及任务回写 |
| `plugin/login/routes.go`、`account_routes.go` | 官方鉴权插件通道、查询参数和写入约束 |
| `plugin/login/proxy.go`、`oauth/oauth.go`、`login-runtime-go/internal/transport/` | 登录参数与各阶段代理一致性 |
| `plugin/login/security.go`、`login-runtime-go/internal/login/security*.go` | 安全动作、确认结果、本地证据和未知结果隔离 |
| `scripts/verify_account_ui.cjs` | 已有合成 API 数据约定与账号 UI 验证场景 |

本轮参考的是交互与工程做法。参考归档没有可据以推断整个项目开源许可的根许可证；不将整个参考项目宣称为 MIT，也不直接替换当前 UI 技术栈。

### 本轮选定参考文件的摘要

已为上述研究涉及的 **37 份源码与文档**记录独立 SHA256 基线：[reference-baseline.json](../artifacts/login-experience-0.16.0/reference-baseline.json)。清单包含实际参考绝对路径、采集时间、每个文件的相对路径、字节数和摘要，按 UI 工作流、协议与诊断分组。它覆盖本轮使用的研究证据，不代表对整个参考项目或其运行账号进行了完整验证，也不根据归档名称推定上游 Git 提交。

当前 `login-runtime-go/generate_manifest.py` 的 `REFERENCE` 仍指向旧路径 `/Users/ceng/Downloads/toSub2-go-port`。该路径在 Docker 构建上下文不存在时，生成的 `protocol_reference_sha256` 会是空对象。**空对象表示没有从该路径采集参考文件，不能作为已核验本轮参考归档的证据。** 本轮研究使用上面的独立清单；运行时自己的 `source_sha256` 由最终构建流程重新生成，两者用途不同。

### 部署前的独立只读检查

本轮独立预检时，`cli-proxy-api` 仍运行 CPA 8.0.76 和 model-quality 0.15.0，插件已注册并启用，自动重启次数为 0。管理读取返回账号 0、登录任务 0、模型运行 0、CPA 凭证 0；这些数字仅代表预检时的状态，不能替代实际升级前的再次核对。脱敏结果见 [preflight-readonly.json](../artifacts/login-experience-0.16.0/preflight-readonly.json)。预检只读取容器配置与管理 GET，没有停止容器或执行账号、模型、安全操作。

配置、CPA 凭证、插件库、插件数据库、日志及静态页面均通过独立目录挂载保存在 `artifacts/docker-8.0.76/runtime/`。容器仅发布 `127.0.0.1:8317`，重启策略为 `unless-stopped`。升级应先停止 CPA，对整个运行目录做一致性备份并记录摘要，然后仅替换已经校验的 `plugins/model-quality.so`；保留全部挂载目录与旧引擎目录。运行中的 bbolt 文件复制不能视为一致性备份。重启后核对鉴权、版本、能力、账号和任务计数，以及配置/凭证摘要；不得重放旧任务或覆盖账号数据库。

## 独立后续功能

- 批量登录需定义账号快照、逐项授权、队列容量、部分受理、未知提交停止以及跨页选择；不能只串行调用单账号按钮。
- 回收站需定义凭证和历史保留、恢复冲突及永久删除，不与当前“删除插件账号但保留 CPA 凭证”的语义混合。
- 邮件/手机验证码、BPS 授权、账号注册、套餐和额度查询均依赖新的服务端与上游能力，应单独设计和验证。
- 质量暂停/解除仍受官方宿主 SDK 能力限制，不使用人工禁用模拟。
- 更换秘密展示默认值、自动执行偏好及登录确认步骤需尊重当前明确的产品决定，不借对齐参考页面隐式改变。

## 离线 UI 验证服务

`scripts/login_ui_fixture.cjs` 仅使用 Node 内置模块，直接提供当前 `plugin/ui/` 源文件与合成 API。它不使用 Playwright、不启动浏览器、不调用外网，也不读取真实配置或账号库；全部状态保存在内存。绑定固定为 `127.0.0.1`。

```sh
node scripts/login_ui_fixture.cjs --port 0
```

启动输出为 JSON，包含实际端口、PID、页面 URL、控制 URL 和停止 URL。页面管理密钥为合成专用的 `offline-key`。当前机器未把 Node 放入 PATH 时，可使用 Codex bundled Node 的绝对路径启动。

预置账号均以 `fixture-` 开头、邮箱域名为 `example.test`、备注带“离线演示”，分别覆盖未绑定、失败、进行中、启用成功、安全操作结果未知和旧历史。全局共有超过 200 条任务，`fixture-history` 的 `fixture-job-old` 位于全局最近 200 条之外；账号范围读取仍可返回它。

控制 API 只作用于此 fixture：请求必须携带 `X-Fixture-Key: offline-control`。对实际启动端口发送：

```http
POST /__fixture/control
Content-Type: application/json
X-Fixture-Key: offline-control

{"action":"inject","route":"/accounts","kind":"failure"}
```

`kind` 可为 `failure`、`malformed`、`delay`；`count` 指定后续读取次数（1–10），`delay_ms` 指定慢响应时间（最多 30 秒）。可指定 `/accounts/settings`、`/accounts/detail`、`/jobs`、`/logs` 等读取路径。注入只匹配 GET，不重放写操作。

`{"action":"reset"}` 恢复种子；`{"action":"job","id":"fixture-job-active","status":"committed"}` 改变合成任务状态；`GET /__fixture/state` 查看脱敏调用路径和计数。`POST /__fixture/stop` 或向启动 PID 发送 SIGTERM 停止服务。服务不写数据库，也不会产生真实登录授权。

fixture 的启动、鉴权、种子数量、全局截断、账号范围历史、一次性失败、不完整响应、慢响应及静态文件已通过接口级检查。实际界面布局与交互需由主任务通过 CUA 在当前页面核验；接口检查不替代视觉验证。
