# CPA 8.0.76 容器与插件部署

本机 8317 的官方 CPA 8.0.76 容器最初安装于 2026-11-17。**当前已部署插件 0.18.6 / UI 12.3.0 / Go＋Sobek 2.1.3**。2026-11-18 20:41（北京时间）完成停服备份，只替换插件库后启动原容器；详见 [12.3.0 重置确认框与实际页面核验](UI_REWORK_12_3_0_CN.md)。上一版见 [12.2.0 历史部署](UI_REWORK_12_2_0_CN.md)。

- 宿主源码：`/Users/ceng/Downloads/CLIProxyAPI-8.0.76`，未修改。
- 镜像：`cliproxyapi:8.0.76-local`，容器：`cli-proxy-api`。
- 平台：Linux arm64 / Debian bookworm；构建工具链 Go 1.26.11。
- 当前插件包：[model-quality 0.18.6（Linux arm64）](../artifacts/ui-rework-12.3.0/plugin-package/model-quality-v0.18.6-linux-arm64.zip)。
- 管理密钥已设置为用户指定值；CPA 启动后将配置中的明文转换为哈希。

此前 UI 11.17.0 移除账号行“手动登录”兜底标记并重构操作日志，套餐继续使用 toSub2 配色。插件 0.17.2 增加日志保留窗口和分页元数据，账号执行流程与运行时保持原样。离线测试、界面、原生构建与部署重启核验通过。见 [本轮记录](UI_REWORK_11_17_0_CN.md)。

入口：

- [CPA 面板](http://127.0.0.1:8317/management.html)
- [CPA 内嵌插件页面](http://127.0.0.1:8317/management.html#/plugin-pages/model-quality/0)：更新后刷新整个管理标签页。18417 是独立的合成预览。
- [插件页面](http://127.0.0.1:8317/v0/resource/plugins/model-quality/index.html)

容器内允许经 Docker 网桥访问管理接口，宿主机仅发布 `127.0.0.1:8317`。本轮升级保留开始核验时既有的 1 个插件账号、2 条历史登录任务和 1 个 CPA 凭证，未迁移其他实例的账号。模型 API 客户端密钥单独随机生成，保存在私有运行配置中。

## 运行与持久化

从本插件仓库根目录执行：

```sh
docker compose -f deploy/docker/compose.yaml -p cpa-quality ps
docker compose -f deploy/docker/compose.yaml -p cpa-quality restart cpa
docker compose -f deploy/docker/compose.yaml -p cpa-quality stop cpa
docker compose -f deploy/docker/compose.yaml -p cpa-quality up -d
docker logs --tail 80 cli-proxy-api
```

`restart: unless-stopped` 已配置。运行数据位于本仓库 `artifacts/docker-8.0.76/runtime/`，通过目录挂载持久化：

| 宿主目录 | 内容 |
| --- | --- |
| `config/` | 私有 CPA 配置 |
| `auths/` | CPA OAuth 凭证及 KV |
| `plugins/` | 已安装 `model-quality.so` |
| `plugin-data/model-quality/` | 账号库、测试历史、设置及内嵌引擎 |
| `static/` | 官方面板资源 |
| `logs/` | 日志目录 |

重建或重启容器会保留这些目录；删除运行目录会丢失对应数据。备份数据库前先停止容器，不要把私有运行目录加入发布包。

## UI 12.3.0 实例核验

[8317 升级记录](../artifacts/ui-rework-12.3.0/deployment.json)核对了宿主镜像、插件版本、六份静态资源以及升级前后的账号、任务和凭证。[桌面与手机的真实管理页核验](../artifacts/ui-rework-12.3.0/deployed-iframe.json)在临时浏览器中仅使用合成账号，打开新确认框，核对选项布局、勾选反馈与底栏后取消；实际账号安全接口未被调用。

## UI 12.1.1 历史核验

[8317 升级记录](../artifacts/ui-rework-12.1.1/deployment.json)确认插件已启用，六份静态资源与构建摘要一致；账号、任务、日志、设置、配置和凭证文件均保留。实际容器仅停启一次；[再次重启测试](../artifacts/ui-rework-12.1.1/native-smoke.json)在隔离空实例上完成。[8317 管理页只读检查](../artifacts/ui-rework-12.1.1/deployed-iframe.json)确认桌面和手机的连接弹窗均显示 UI 12.1.1，统计栏分别为四列和两列。

## UI 11.18 历史部署：登录任务与详细诊断

主导航改为账号管理、智商测试、质量观察；空或无效 hash 默认打开账号管理，明确的测试／观察链接仍有效。登录任务使用紧凑历史列表、搜索与进行中／需关注筛选，详情显示结果概要、横向安全流程和常驻日志。

插件 0.17.3 / 运行时 2.1.3 为协议请求记录固定端点标签、方法、HTTP 状态、耗时及传输失败，保留运行时错误码、超时前阶段、OAuth 失败诊断和已确认安全结果。报告导出全部已读取事件，不受当前筛选影响，并标明读取不完整或任务／日志版本不同步。不记录密码、2FA 密钥、令牌、URL、响应正文或原始运行时输出。

每任务最多保留最近 200 条事件；请求明细最多 128 条，达到上限后明确标记并继续记录阶段和最终结果。终态任务保留 30 天；旧日志原样保留，不能补回过去未记录的请求。十七项 JavaScript 测试、后端测试／vet 和离线桌面／390px 深色布局检查已通过；原生构建、停服备份、部署与再次重启核验均通过；2026-11-17 21:26:35（Asia/Shanghai）完成更新，账号、任务、原日志、配置、CPA 凭证及宿主镜像保持不变。详见 [本轮记录](UI_REWORK_11_18_0_CN.md)。

## UI 11.17 操作日志排查

UI 11.17.0 / 插件 0.17.2 已于2026-11-17 20:49:02（Asia/Shanghai）部署。移除账号“手动登录”兜底标记，日志按所选任务展示摘要、毫秒时间、事件间隔、搜索、错误建议和固定字段报告。日志接口增加保留窗口／分页元数据，运行时仍为 2.1.2。离线测试、三主题与窄屏、原生构建及部署重启核验通过；账号、任务、设置及原日志事件完全保留。详见 [本轮记录](UI_REWORK_11_17_0_CN.md)。

## UI 11.16 历史 toSub2 套餐配色

Pro 改为银色，Pro Lite／Business Premium 为金色，其他套餐恢复参考项目各自色系；上传排除选项同步，选中时保留套餐背景。仅配色和隐藏版本标记改变，状态语义与排版不变。已于2026-11-17 20:24:46（Asia/Shanghai）部署并通过重启核验。见 [本轮记录](UI_REWORK_11_16_0_CN.md)。

## UI 11.15 历史 CPA 套餐与状态样式

源码已按 CPA 实际套餐层级和紧凑状态徽章更新；列表套餐与状态统一22px，账号三列保持48px双行对齐。仅明确待巡检的状态显示“待巡检”，不改变健康判定或套餐排除。已于2026-11-17 20:11:48（Asia/Shanghai）部署并通过重启核验；三套主题、390/320px窄屏和原生测试/vet/构建通过。见 [本轮记录](UI_REWORK_11_15_0_CN.md)。

## UI 11.14 历史套餐样式与设置精简

移除登录设置底部“服务状态”整区及对应渲染/样式；套餐徽标改为20px高、10px字号圆胶囊，排除选项改为30px高同类胶囊并保留原生复选框状态。授权快照与排除保存语义不变。已于2026-11-17 19:54:16（Asia/Shanghai）部署并通过重启核验；三套主题、窄屏、键盘交互和原生测试/vet/构建通过。见 [本轮记录](UI_REWORK_11_14_0_CN.md)。

## UI 11.13 历史测试设置精简

移除测试设置底部整块回答数量与自动评分/暂停说明，以及对应留白、更新逻辑和样式。开始测试的禁用、准入校验、按钮提示与字段错误说明保留；插件0.17.1/运行时2.1.2不变。已于2026-11-17 19:45:12（Asia/Shanghai）部署并通过重启核验；离线交互、原生测试/vet/构建通过。见 [本轮记录](UI_REWORK_11_13_0_CN.md)。

## UI 11.12 历史流程文字精简

底部流程移除“必选/执行/跳过”，保留名称、箭头、视觉状态及隐藏无障碍语义，框高减少14px。已于2026-11-17 19:22:39（Asia/Shanghai）部署并通过重启核验；480×486px重置四组合与手机/退出弹窗、静态和原生测试/vet/构建通过。见 [UI 11.12说明](UI_REWORK_11_12_0_CN.md)。

## UI 11.11 历史精简弹窗

UI 11.11.0 合并两条等高选项、精简介绍为单行，并把横向节点改成浅灰流程带；移除重复主动作卡、编号和选中标签。后端0.17.1/运行时2.1.2不变。已于2026-11-17 19:09:48（Asia/Shanghai）部署并通过重启核验；12组布局确认说明完整单行、选项等高与复选框对齐，Node16/static/native/vet/build通过，无真实安全提交。详见 [本轮说明](UI_REWORK_11_11_0_CN.md)。

## UI 11.10 历史横向流程

UI 11.10.0 在现有选项卡片下恢复横向节点与箭头，重置为三节点、独立退出设备为两节点；未选步骤保留位置并显示“跳过”，选中状态不冒充执行结果。插件和运行时仍为0.17.1/2.1.2。已于2026-11-17 18:55:06（Asia/Shanghai）部署并通过再次重启核验。12组布局、三主题、Node16项及原生测试/vet/构建通过，本轮未提交真实或模拟安全动作。详见 [本轮说明](UI_REWORK_11_10_0_CN.md)。

## UI 11.9 历史选项设计

UI 11.9.0 将安全弹窗的必做主动作与可选后续操作合并为单一有序流程；选项整行可点、显式显示选中状态，底部显示操作计数。默认选择、固定窗口和安全业务保留，后端版本仍为 0.17.1、运行时仍为 2.1.2。已于2026-11-17 18:34:29（Asia/Shanghai）部署并通过再次重启核验。桌面/手机/短窗口、三主题、全行点击与键盘、错误保护、六条模拟提交、Node16项和原生测试/vet/构建均通过；真实安全操作为0。详见 [UI 11.9.0](UI_REWORK_11_9_0_CN.md)。

## UI 11.8 历史弹窗记录

UI 11.8.0 调整重置 2FA / 退出设备确认弹窗的稳定高度、固定步骤预览及内部滚动。插件仍为 0.17.1，内嵌运行时仍为 2.1.2；后端与宿主不变。已于 2026-11-17 18:12:16（Asia/Shanghai）完成部署及再次重启核验。桌面/手机/短窗口、三套主题、错误与焦点处理、两条模拟提交、16 项 Node 回归及原生构建通过；实际 CPA 页面四种勾选组合仍保持 520×584px，检查后取消，未提交真实安全操作。详见 [UI 11.8.0](UI_REWORK_11_8_0_CN.md)。

## 工作区选择修复 0.17.1

插件 0.17.1 / Go＋Sobek 2.1.2 已于 2026-11-17 17:39:38（Asia/Shanghai）完成部署并通过再次重启核验。新增任务采用明确绑定优先、否则组织优先再取首个有效工作区的规则；历史任务不会重放。该次工作区修复保持 UI 11.7.0 六份源文件不变。详见 [工作区自动选择](WORKSPACE_SELECTION_CN.md)。本轮无真实登录，之前 16:14 的任务仍保留原结果；用户需手动发起新登录以验证新流程。

## 沿用的 UI 11.7 内容

插件 0.17.0 / UI 11.7.0 的本轮实现增加套餐标识、账号/CPA 状态/最近操作两行摘要，以及默认空列表的 `cpa_upload_excluded_plans`。该策略覆盖本插件所有 CPA 凭证上传和更新，包括手动登录、自动重登和安全操作后的授权写入；未知套餐单独配置，不视为 Free。详见 [UI 11.7.0](UI_REWORK_11_7_0_CN.md)。“登录设置”页包含上传排除、代理和巡检。四种视口的布局、三套主题、合成设置/候选交互及 16 项 Node 回归已通过，见该报告。原生构建、测试与部署均已通过；2026-11-17 17:18:21（Asia/Shanghai）完成更新，并通过再次重启核验。

UI 11.7 首次升级曾仅允许新增套餐默认字段。历史 UI 11.17 升级只为日志响应增加只读保留／分页元数据，账号、任务与设置快照以及原日志事件均按完整相等核对，没有任何默认值例外。

## 构建与安装记录

`artifacts/docker-8.0.76/build-context/host/` 是本次从指定源码复制的构建快照，仅包含源码和构建资源，排除了凭证、运行配置和数据库。`source-baseline.json` 记录 2,413 个源文件摘要，已逐个核对原目录及快照一致。[本轮独立基线核验](../artifacts/ui-rework-11.18.0/source-baseline-verification.json) 确认两处各 2,413 个记录文件摘要均一致。未使用历史 8.0.67 基线冒充 8.0.76；原目录没有 Git 元数据，未声称对应某个上游提交。

本次构建在复制的插件模块中将 SDK replacement 指向上述 8.0.76 快照，不修改原插件模块配置或宿主源码。登录引擎与插件均在原生 Linux arm64 容器中构建。基础镜像摘要已固定。

`artifacts/docker-8.0.76/build-context/host/` 可复用；它旁边的 `plugin/` 是 UI 11.18.0 的旧快照，**不能直接用来构建 12.1**。安装本轮版本请使用上方已验证的 ZIP。如需自行重建，先准备只包含已核验宿主源码、目标插件源码、登录引擎和构建脚本的新构建上下文，勿复制运行目录。在仓库根目录执行以下构建，不会替换已安装的插件：

```sh
: "${BUILD_CONTEXT:?请指定已核对的目标版本构建上下文}"
OUT=$(mktemp -d)
docker build --platform linux/arm64 --target plugin-artifacts -f deploy/docker/Dockerfile \
  --output "type=local,dest=$OUT" "$BUILD_CONTEXT"
cat "$OUT/build-manifest.json"
```

历史 UI 11.18 保留既有账号表格、操作列和 toSub2 套餐配色，修改主导航、登录任务排版及诊断展示／导出，并在插件和 Go/Sobek 运行时增加受控请求日志。登录与安全动作的授权、隔离和不确定结果处理边界不变；不修改官方 CPA 源码。套餐仍来自授权快照；上传排除不会触发真实订阅查询、补传或已有凭证改写。

插件安装采用官方支持的直接动态库安装：停服备份，将核验过的 `plugin-package/model-quality.so` 替换到 `runtime/plugins/`，再启动。全局插件及该插件均已启用，数据目录为 `/CLIProxyAPI/plugin-data/model-quality`。升级包需核对目标 `ui_version`、动态库及六份 UI 资源的 SHA-256；12.3.0 的核对值见[安装包清单](../artifacts/ui-rework-12.3.0/plugin-package/build-manifest.json)。

若修改 `deploy/docker/build_plugin.py`，重建前同步到构建上下文的 `build_plugin.py`；若更新插件或宿主源码，必须重新生成并核对相应快照和基线。

## UI 11.18 历史验证

源码与离线验证见 [UI 11.18.0](UI_REWORK_11_18_0_CN.md)、[17 项前端检查](../artifacts/ui-rework-11.18.0/frontend-tests.json) 和 [后端测试记录](../artifacts/ui-rework-11.18.0/backend-status.txt)。[原生安装包核验](../artifacts/ui-rework-11.18.0/package-verification.json) 确认运行时与插件全量测试／vet、原生构建和摘要校验通过。[部署记录](../artifacts/ui-rework-11.18.0/deployment.json) 与 [再次重启核验](../artifacts/ui-rework-11.18.0/verification-after-restart.json) 确认 1 个账号、2 条历史任务、1 个 CPA 凭证、设置及原事件完整相等，配置／凭证文件和镜像未变；六份资源摘要一致，未鉴权请求为 401。UI 11.17 的结果单独保留在下方历史章节。 [8317 实际页面核验](../artifacts/ui-rework-11.18.0/deployed-layout-verification.json) 确认默认账号管理、三个导航、27 条原事件、横向步骤及历史异常筛选正常；[截图](../artifacts/ui-rework-11.18.0/deployed-tasks.jpg) 已保存。

## UI 11.17 历史验证

- CPA 使用既有已验证镜像，本轮未更换宿主；登录引擎全量离线测试通过。
- 插件带内嵌引擎的全量测试及 `go vet` 通过。
- 官方管理列表确认插件已注册、已启用且版本正确。
- 未鉴权管理请求被拒绝；指定密钥可访问管理接口。
- 官方面板 HTTP 200；插件六份静态资源摘要与源码一致。
- 容器内登录引擎能力读取正常；空 PATH 自检属于此前安装验证记录。
- 重启后插件、登录能力及已保存设置正常，1 个插件账号、2 条历史任务及 1 个 CPA 凭证的逻辑快照完全一致；持久化文件及原日志事件保留，新增日志保留元数据已验证，配置与凭证文件摘要不变。
- [布局测量](../artifacts/ui-rework-11.7.0/layout-checks.json) 覆盖 1064/840/390/320px：六条合成账号的身份区均高 45px、最近操作区均高 41px，操作按钮高 29px，套餐名称完整显示。三套主题与设置保存/保留、排除任务入口、清空排除后显式提交均已核对；Node 客户端/导入回归 16 项通过。
- [本轮安装包核验](../artifacts/ui-rework-11.17.0/package-verification.json) 确认运行时测试、插件内嵌模式测试、`go vet`、Linux arm64 构建及包摘要正确。[日志后端测试](../artifacts/ui-rework-11.17.0/backend-status.txt) 覆盖保留窗口、分页、旧记录与固定诊断投影；[UI回归16项](../artifacts/ui-rework-11.17.0/ui-tests.log) 通过。14 个 Web/OAuth 场景和 race 检查属于之前工作区选择修复记录。
- [本轮界面核验](../artifacts/ui-rework-11.17.0/layout-verification.json) 覆盖三主题、手机与320px窄屏、200项历史选择、搜索/异常筛选、复制和导出、展开状态保持及503读取错误保留。筛选只显示1条时，导出仍保留所选任务全部5条合成事件且无秘密金丝雀。320px最终文档宽度与滚动宽度均305px，无横向溢出。历史套餐几何证据见 [UI11.16](UI_REWORK_11_16_0_CN.md) 与 [UI11.15](UI_REWORK_11_15_0_CN.md)。
- [8317实际页面核验](../artifacts/ui-rework-11.17.0/deployed-layout-verification.json) 确认UI11.17、账号行无“手动登录”、套餐22px、所选任务27条原事件、搜索/复制/导出入口齐全且无控制台错误；[截图](../artifacts/ui-rework-11.17.0/deployed-logs.jpg)已保存。
- 未执行真实登录、模型调用、重置 2FA 或退出设备。

调度、巡检、自动重登默认关闭。官方兼容版只提供质量观察，不提供独立质量暂停或释放。

宿主基线与首次安装证据位于 `artifacts/docker-8.0.76/`：`source-baseline.json`、`runtime-self-test.json`、`verification-before-restart.json`、`verification-after-restart.json` 及 `plugin-package/build-manifest.json`。

上一版插件 0.17.3 / UI 11.18.0 / Go＋Sobek 2.1.3 的产物目录为 `artifacts/ui-rework-11.18.0/`。其中的 [Linux arm64 ZIP](../artifacts/ui-rework-11.18.0/plugin-package/model-quality-v0.17.3-linux-arm64.zip)、[构建清单](../artifacts/ui-rework-11.18.0/plugin-package/build-manifest.json)及[部署记录](../artifacts/ui-rework-11.18.0/deployment.json)仅对应历史版本。

此前已部署的插件 0.17.2 / UI 11.17.0 的原生安装包与升级证据位于 `artifacts/ui-rework-11.17.0/`，包含 [部署记录](../artifacts/ui-rework-11.17.0/deployment.json)、[构建清单](../artifacts/ui-rework-11.17.0/plugin-package/build-manifest.json)、停服备份、构建日志及重启前后核验。`artifacts/ui-rework-11.16.0/`、`artifacts/ui-rework-11.15.0/`、`artifacts/ui-rework-11.14.0/`、`artifacts/ui-rework-11.13.0/`、`artifacts/ui-rework-11.12.0/`、`artifacts/ui-rework-11.11.0/`、`artifacts/ui-rework-11.10.0/`、`artifacts/ui-rework-11.9.0/`、`artifacts/ui-rework-11.8.0/`、`artifacts/workspace-selection-0.17.1/`、`artifacts/ui-rework-11.7.0/`、`artifacts/ui-rework-11.6.0/`、`artifacts/ui-rework-11.5.0/`、`artifacts/ui-rework-11.4.0/`、`artifacts/ui-rework-11.3.0/`、`artifacts/ui-rework-11.2.0/`、`artifacts/ui-rework-11.1.0/`、`artifacts/ui-rework-11.0.0/` 和 `artifacts/ui-rework-10.0.0/` 为已被替代版本的历史记录；`artifacts/login-experience-0.16.0/` 对应 UI 9.3.0；`artifacts/ui-rework-9.4.0/` 对应随后已部署的 UI 9.4.0；`artifacts/docker-8.0.76/plugin-package/` 是首次安装 0.15.0 的历史包。同一插件后端版本的 ZIP 可能包含不同 UI，安装前应以目录、构建清单中的 `ui_version` 和 SHA256 核对。
