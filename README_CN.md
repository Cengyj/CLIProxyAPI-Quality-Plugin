# model-quality 官方 CPA 原生插件

**保留官方 CPA，只安装插件。** 当前插件使用官方 CPA 已有的原生插件接口，本次容器已验证 CPA 8.0.76，不需要修改 CPA 源码、替换 CPA 程序或安装补丁。Go＋Sobek 登录引擎已经嵌入动态库，无需另装 Node、Chrome 或登录服务。

**当前版本为插件 0.29.27、UI 13.6.26、Go＋Sobek 2.2.1。** 在质量运维的运行记录中点击「详情」，即可在弹窗内核对凭证邮箱、答案判定、参考答案和模型回答。关闭弹窗会返回原来的筛选、分页和滚动位置。

账号管理继续沿用 [无提示卡片的行内登录反馈](docs/LOGIN_FEEDBACK_13_6_3_CN.md)、[五按钮分组工具栏](docs/ACCOUNT_TOOLBAR_13_6_2_CN.md)、[紧凑资料表单](docs/PROFILE_FORM_13_6_1_CN.md)与[历史/设置弹窗布局](docs/ACCOUNT_SURFACES_13_6_CN.md)。

智商测试保留 [等高双栏和固定底栏](docs/MANUAL_WORKSPACE_13_4_CN.md)，质量运维继续采用 [同高双栏工作台](docs/QUALITY_WORKSPACE_13_3_CN.md)；规则中的“答错后的处理”提供仅检测、暂停凭证、移除凭证，以及暂停后整轮通过自动恢复。策略使用 CPA 已有管理 API，不修改 CPA 源码；默认不处理凭证。支持范围、管理连接、人工操作保护与并发边界见 [答错处理说明](docs/QUALITY_CREDENTIAL_ACTIONS_CN.md)。

默认使用「全部已启用凭证」，不必逐个选账号。下一轮按当时可用凭证和模型目录重新匹配。默认模型为 `gpt-6-astra`，每模型 1 份；`low / medium / high / xhigh / max` 五档直接可选，默认 `high`。旧规则可沿用 `minimal`，不会自动换档。运行记录保留当轮请求的档位，不代表上游实际生效档位。「检测周期（五段 Cron，服务端时区）」只有一个输入框，默认 `*/30 * * * *`。唯一的「启用定时检测」默认关闭；打开后保存，仅本条规则从下个周期开始。每条规则默认按凭证保留 100 条结果。打开、预览或保存规则不会立即调用模型。

[质量运维使用说明](docs/QUALITY_SCOPE_CN.md) · [sub2api 质量运维与 CPA 对照](docs/ACCOUNT_QUALITY_PARITY_CN.md) · [CPA 8.0.76 容器构建与部署](docs/DOCKER_8_0_76_CN.md)。

账号管理：[账号资料表单](docs/PROFILE_FORM_13_6_1_CN.md) · [历史记录与全局设置](docs/ACCOUNT_SURFACES_13_6_CN.md) · [转换凭证](docs/CREDENTIAL_CONVERSION_CN.md) · [重置确认框与流程](docs/UI_REWORK_12_8_0_CN.md) · [2FA 功能对齐说明](docs/RESET_2FA_REFERENCE_ALIGNMENT_CN.md)。

此前 8317 安装的是插件 0.17.3 / UI 11.18.0。该版的账号管理、登录任务、请求级诊断和完整排查报告在 12.1 中保留。见 [11.18 历史部署记录](docs/UI_REWORK_11_18_0_CN.md)。

账号列表沿用 **UI 11.7 的紧凑布局：**账号身份、CPA 状态与最近操作各占两行，完整信息保留在提示、资料与日志中。账号操作按两行三列排列，桌面按钮高 29px，手机上至少 44px。

“全局设置”保留 **CPA 上传排除套餐**（`cpa_upload_excluded_plans`），默认为空，适用于本插件所有 CPA 凭证上传和更新。套餐来自授权快照，未知明确显示为“未知套餐”，不会推定为 Free。功能来源与早期部署证据见 [UI 11.7.0](docs/UI_REWORK_11_7_0_CN.md)；早期布局作为历史保留。

插件 0.16.0 的登录与账号能力继续保留：读取失败保留列表并恢复、进行中筛选、导入前校验、实际代理来源、安全步骤与部分结果、账号独立历史，以及 SOCKS5H 远端 DNS。详见 [对照研究](docs/REFERENCE_LOGIN_20261117_CN.md) 和 [0.16.0 实现与验证](docs/LOGIN_EXPERIENCE_0_16_0_CN.md)。

**工作区选择修复已部署（0.17.1 / 运行时 2.1.2）：**未绑定账号按参考项目优先选择第一个组织工作区，否则选择第一个有效工作区；已有绑定始终精确匹配目标。覆盖网页登录与 OAuth，旧任务不会自动重试。用户需在账号列表手动发起新登录以使用修复。实现、离线验证及部署证据见 [工作区自动选择](docs/WORKSPACE_SELECTION_CN.md)；本轮未尝试真实登录。

**安全弹窗沿用 UI 11.12.0：**底部流程去掉“必选/执行/跳过”，只显示步骤名、箭头与视觉状态；原生选项和无障碍语义保留。重置弹窗为480×486px，默认选择及安全业务不变。见 [验证与截图](docs/UI_REWORK_11_12_0_CN.md)。

**测试设置已更新至 UI 13.4.0：**保留 UI 11.13.0 的准入校验、按钮提示和字段错误说明；底部使用紧凑操作栏显示回答数量和开始测试，不恢复已移除的整块说明区。早期调整见 [历史记录](docs/UI_REWORK_11_13_0_CN.md)。

**保留 UI 11.14.0 的设置精简：**全局设置中不再显示底部“服务状态”；上传排除保留30px高原生复选框胶囊。此前套餐配色与尺寸见 [历史记录](docs/UI_REWORK_11_14_0_CN.md)，当前套餐配色以 UI 11.16.0 为准。

**保留 UI 11.15.0 的紧凑排版：**套餐使用 CPA 的“套餐＋名称”胶囊；CPA 状态与最近操作采用统一高度的圆点徽章，三列保持两行对齐。桌面、390/320px窄屏和三套主题的布局证据见 [历史记录](docs/UI_REWORK_11_15_0_CN.md)，其中套餐配色已由本轮替代。

**沿用 UI 11.16.0 配色：**套餐按 toSub2 配色：Pro 银色，Pro Lite／Business Premium 金色，Plus 蓝、Go 绿、Team 紫；上传排除选项同步配色，保持现有22px胶囊和两行对齐。浅色／深色配色、原生构建和部署重启核验通过。见 [本轮记录](docs/UI_REWORK_11_16_0_CN.md)。

**保留 UI 11.17.0 的账号日志能力：**移除账号行“手动登录”兜底标记，操作日志增加任务摘要、历史任务选择、阶段／代码搜索、毫秒时间、事件间隔和排查报告导出；明确早期记录及截断，历史事件不混入最终安全结果。插件 0.17.2 增加日志保留／分页元数据，运行时仍为 2.1.2。离线测试、三主题与窄屏、原生构建、部署重启核验通过，见 [本轮记录](docs/UI_REWORK_11_17_0_CN.md)。

## 功能

- **智商测试**：选择 CPA 凭证、题目、模型及支持的思考强度，手动生成 1–8 份回答；支持自定义题、测试记录、复制、下载和 HTML 预览。打开页面或保存题目不会调用模型。
- **OpenAI 账号管理**：按 `账号----密码----2FA长期密钥` 导入，提供账号列表、资料编辑、行内日志、单账号/默认代理与直接 2FA 登录。2FA 使用目标项目适配的 Go＋Sobek 引擎，不走浏览器自动化。
- **转换凭证**：导入或粘贴 sub2api OpenAI / Codex OAuth JSON（最多 100 条、2 MiB），逐条核对转换结果、下载 CPA JSON，或一键上传到当前 CPA 凭证列表。保留禁用状态，遵守套餐排除策略；已有凭证不覆盖，不确定写入不重试。详见 [转换说明](docs/CREDENTIAL_CONVERSION_CN.md)。
- **账号安全**：账号行直接提供重置 2FA，“更多”提供退出所有设备，可选完成后重新授权并启用 CPA；重置后自动保存新密钥，未知结果不重复修改。
- **套餐与 CPA 上传策略**：账号显示授权套餐快照，可按套餐排除手动登录、自动重登及安全操作后授权的 CPA 写入。排除默认关闭；命中时保留当前会话的授权供核对，不上传、不自动补传；安全操作已确认的新 2FA 密钥仍保存。
- **账号巡检**：保存后可每 5–1440 分钟检查 CPA 已记录的认证状态，默认 15 分钟且关闭。自动重登需同时启用全局和账号许可，并有明确认证失效证据。从账号列表发起登录后，成功且未命中套餐排除时自动保存并启用 CPA 凭证；已授权的巡检重登沿用同一流程。进度和结果留在账号行，无需跳转或再次确认。
- **质量运维**：默认每轮按已注册模型匹配全部已启用凭证，支持单凭证规则、批次结果和可选定时。输出上限不再作为建规则字段，既有规则和后台预算保留。官方 SDK 不提供独立质量暂停、归属释放和可信最终证据，不使用人工禁用来模拟。

账号密码、2FA、代理和设置按用户要求明文保存在插件数据目录。CPA 管理密钥在成功连接后明文保存在当前浏览器，刷新自动重连；主动断开或鉴权失败会清除。详见 [持久化说明](docs/PERSISTENCE_CN.md)。

## 安装与构建

本次 CPA 8.0.76 容器安装与重建见 [Docker 部署](docs/DOCKER_8_0_76_CN.md)。其他既有官方 CPA 的原生构建流程见 [安装说明](docs/INSTALL_CN.md)：

```sh
./scripts/build.sh
./scripts/verify.sh
```

旧 8.0.67 原生构建流程校验 2,303 个基线文件；本次 8.0.76 Docker 构建校验 2,413 个基线文件，编译并内嵌原生登录引擎，输出单个动态库和符合官方安装器格式的 ZIP。SDK Go module 路径中的 `/v8` 是模块版本；插件自定义业务接口使用官方已有且受管理鉴权保护的 `/v0/management/plugins/model-quality/` 通道，两者不是一回事。

官方凭证保存接口没有跨进程原子比较更新能力。插件在写入前复查身份和原 JSON，保留未知策略字段，按本次账号登录的授权启用凭证，并在写入前重新检查最新套餐排除设置；写入后检查物理文件及运行时的启用状态；结果不确定时保留状态，不盲目重试。仅支持 standalone 本地文件凭证；不宣称支持 Home 或远端存储同步。详细边界见 [官方单插件方案](docs/OFFICIAL_PLUGIN_CN.md)。

## 文档与证据

- [sub2api → CPA 凭证转换、使用与验证](docs/CREDENTIAL_CONVERSION_CN.md)

- [0.19.2 严格审查、修复与验证](docs/STRICT_AUDIT_0_19_2_CN.md)

- [重置 2FA、退出设备与恢复边界](docs/OPENAI_ACCOUNT_SECURITY_CN.md)
- [一键登录、上传与启用](docs/LOGIN_ONECLICK_CN.md)

- [安装、升级与卸载](docs/INSTALL_CN.md)
- [官方兼容架构、API 与能力边界](docs/OFFICIAL_PLUGIN_CN.md)
- [账号、管理密钥及凭证的持久化](docs/PERSISTENCE_CN.md)
- [单库打包与运行时生命周期](docs/PLUGIN_BUNDLE_CN.md)
- [Go＋Sobek 来源与此前引擎验证](docs/OPENAI_LOGIN_GO_ENGINE_CN.md)
- [质量运维工作台截图](artifacts/quality-model-card-0.27.1/installed-workspace-1440.png)（合成数据）
- [检测模型卡片截图](artifacts/quality-model-card-0.27.1/installed-model-card-1440.png)（合成数据）
- 规则编辑器：[桌面截图](artifacts/quality-model-card-0.27.1/installed-sheet-1440.png) · [手机截图](artifacts/quality-model-card-0.27.1/installed-sheet-390.png) · [三种宽度与三套主题的交互回归](artifacts/quality-model-card-0.27.1/browser/quality-scope.json) · [隔离 CPA 多模型与重启验证](artifacts/quality-model-card-0.27.1/native-smoke.json) · [8317 已安装界面验证](artifacts/quality-model-card-0.27.1/deployed-iframe.json)
- [账号删除回归验证](artifacts/account-delete-12.10.1/browser/account-delete.json) · [隔离宿主加载与重启](artifacts/account-delete-12.10.1/native-smoke.json) · [8317 管理页核验](artifacts/account-delete-12.10.1/deployed-iframe.json)
- [UI 12.10.0 构建摘要](artifacts/credential-conversion-0.20.0/plugin-package/build-manifest.json) · [文件校验值](artifacts/credential-conversion-0.20.0/plugin-package/SHA256SUMS)
- [UI 12.2.0 历史分栏设计](docs/UI_REWORK_12_2_0_CN.md)
- [UI 12.1.3 历史确认框](docs/UI_REWORK_12_1_3_CN.md)
- [UI 12.1.2 历史确认框](docs/UI_REWORK_12_1_2_CN.md)
- [UI 12.1.1 工作区改进记录](docs/UI_REWORK_12_1_1_CN.md)
- [UI 12.1.0 历史记录](docs/UI_REWORK_12_1_0_CN.md) · [容器升级记录](artifacts/ui-rework-12.1.0/deployment.json)
- [UI 11.18.0 历史验证](docs/UI_REWORK_11_18_0_CN.md) · [重启核验](artifacts/ui-rework-11.18.0/verification-after-restart.json) · [容器升级记录](artifacts/ui-rework-11.18.0/deployment.json)
- [UI 11.17.0 历史验证](docs/UI_REWORK_11_17_0_CN.md) · [重启核验](artifacts/ui-rework-11.17.0/verification-after-restart.json) · [容器升级记录](artifacts/ui-rework-11.17.0/deployment.json)
- [UI 11.17.0 构建摘要](artifacts/ui-rework-11.17.0/plugin-package/build-manifest.json) · [文件校验值](artifacts/ui-rework-11.17.0/plugin-package/SHA256SUMS)

8317 本次从插件 0.29.4 / UI 13.6.4 升级前已停服备份，只替换插件库并启动原 CPA 8.0.76 容器。六份已安装页面资源与 UI 13.6.5 安装包一致。升级前已有的 2 个插件账号、7 条历史登录任务、1 次模型运行记录、1 条规则、1 份 CPA 凭证、设置及日志均保留；宿主镜像、配置与凭证文件未变。桌面、短屏和手机的内嵌交互用合成数据验证；未在此实例创建规则、启用定时、调用模型或变更账号。

历史原生实例曾保留 2 个账号、9 条任务，并完成 1 个旧绑定转换；它与当前容器是不同部署记录。见 [历史本机升级](docs/LOCAL_OFFICIAL_UPGRADE_CN.md)、[补丁宿主历史](docs/INSTALL_PATCHED_HOST_HISTORY_CN.md)。

源码在 `plugin/`；原生引擎在 `login-runtime-go/`；本次未修改的 8.0.76 构建快照在 `artifacts/docker-8.0.76/build-context/host/`，历史 8.0.67 流程使用 `workspaces/CLIProxyAPI-official/`。旧 `workspaces/CLIProxyAPI/` 与 `host-patches/model-quality.patch` 为历史产物，不是当前安装依赖。原始 `/Users/ceng/Downloads/CLIProxyAPI-8.0.76` 和历史 8.0.67 参考目录均保持只读。
