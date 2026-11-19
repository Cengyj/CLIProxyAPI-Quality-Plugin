# UI 10.0.0 全界面重构与实际部署（历史）

日期：2026-11-17（Asia/Shanghai）。状态：**曾部署并完成重启核验，现已被 UI 11.0.0 替代**。本文保留当时 CPA 8.0.76 / 插件 0.16.0 / UI 10.0.0 / Go＋Sobek 2.1.1 的变更与证据；当前版本见 [UI 11.0.0 说明](UI_REWORK_11_0_0_CN.md)。当次 CPA 宿主源码与运行镜像保持不变。

实际入口：[CPA 内嵌插件页面](http://127.0.0.1:8317/management.html#/plugin-pages/model-quality/0)。原生安装包：[Linux arm64 ZIP](../artifacts/ui-rework-10.0.0/plugin-package/model-quality-v0.16.0-linux-arm64.zip)，其中包含单个 `model-quality.so`。后端版本仍为 0.16.0，识别 UI 版本时应同时核对 [构建清单](../artifacts/ui-rework-10.0.0/plugin-package/build-manifest.json) 的 `ui_version` 与 [SHA256SUMS](../artifacts/ui-rework-10.0.0/plugin-package/SHA256SUMS)。

## 页面与交互

本次重写共享样式及三个工作区的布局，统一浅色、纯白、深色主题、字体层级、边框、按钮、状态与弹窗。CPA 内嵌页仅为右上角宿主工具栏留出位置，收紧标题与导航之间的空白。页面使用连续滚动，窄屏保留内容与操作入口。

| 工作区 | 当前呈现 |
| --- | --- |
| 智商测试 | 宽屏将配置与回答组织为两栏，窄屏按配置、回答顺序排列；统一题目、模型选项、回答工具栏、样本状态、历史与证据展示 |
| 质量观察 | 统一指标、凭证限制、规则、历史归属及审计记录；规则编辑与预算按内容分区，保持官方观察模式 |
| 账号管理 | 列表收敛为账号、CPA 状态、最近操作、操作四列；时间与操作类型合并展示，去除重复目录标题；平板和手机切换为账号卡片 |
| 账号操作 | 行内保留登录、资料、更多；日志、任务、巡检及安全操作集中到“更多”，行内日志继续支持当前/历史、异常筛选和跟随 |
| 弹窗与状态 | 连接、导入、资料、安全操作、确认、证据及隔离预览使用一致的标题、内容、反馈与操作区；内容过长可滚动，保留取消与焦点恢复 |

本次保留现有控件 ID、事件挂接点和 `manual`、`operations`、`login` 路由。账号操作仅提取共享分发函数；原有 77 个命名函数全部保留。菜单入口允许在读取失败、仍连接且非加载时查看已有日志与任务，写入动作继续受读取状态、进行中锁与版本检查约束。

一步登录、历史候选确认、未知提交与安全操作隔离、敏感草稿清理、设置草稿保护和禁止自动写重试的语义保持不变。HTML 预览继续使用空 sandbox 与内容安全策略。业务接口仍为受鉴权的 `/v0/management/plugins/model-quality/`；官方宿主仅提供质量观察，独立质量暂停、释放及自动暂停仍不可用。

## 验证

| 检查 | 结果与证据 |
| --- | --- |
| 管理客户端、导入验证 | 16 项全部通过，见 [node-tests.log](../artifacts/ui-rework-10.0.0/node-tests.log) |
| 题目与结果兼容 | 题目绑定、人工结果、转义与规则视图通过，见 [question-tests.log](../artifacts/ui-rework-10.0.0/question-tests.log) |
| 原生构建 | Linux arm64 登录引擎测试、插件内嵌构建测试、`go vet` 与共享库构建通过，见 [native-build.log](../artifacts/ui-rework-10.0.0/native-build.log) |
| 离线浏览器 | 桌面 1280/1440、约 1065px 内嵌内容宽度、平板 768、手机 390/360；浅色、纯白、深色；检查布局及主要交互 |
| 合成状态 | 导入校验、资料与安全弹窗、更多菜单、日志/任务/巡检、读取失败恢复、一步登录反馈与隔离 HTML 预览；使用假账号和假上游 |
| 实际实例 | 插件注册并启用、运行时 2.1.1 可用、管理未鉴权请求返回 401、六份实际静态资源摘要与源码一致；再次重启后仍通过 |

浏览器检查与最终页面记录见 [verification-summary.json](../artifacts/ui-rework-10.0.0/verification-summary.json)。离线代表截图：[账号桌面](../artifacts/ui-rework-10.0.0/accounts-desktop.jpg)、[账号手机深色](../artifacts/ui-rework-10.0.0/accounts-mobile-dark.jpg)、[回答区域](../artifacts/ui-rework-10.0.0/answers-desktop.jpg)、[质量观察](../artifacts/ui-rework-10.0.0/quality-desktop.jpg)。这些合成数据截图不代表真实账号登录结果。

[实际 CPA 内嵌账号页截图](../artifacts/ui-rework-10.0.0/deployed-cpa-accounts.jpg) 已在 1280×720 宿主窗口核对：iframe 宽 1064px，账号表可见宽度与滚动宽度均为 997px，没有横向溢出；主题跟随宿主纯白模式，浏览器控制台错误为 0。

## 部署与数据保留

[deployment.json](../artifacts/ui-rework-10.0.0/deployment.json) 记录于 2026-11-17 14:10:07 完成更新。沿用 `cli-proxy-api` 容器及 `cliproxyapi:8.0.76-local` 镜像，停服后完整备份 70 个运行文件，再替换插件共享库并启动。私有备份路径及摘要保存在部署记录中，不纳入公开安装包。

升级前后保留 **1 个插件账号**；登录任务、模型运行、检测规则、暂停归属和 CPA 凭证均为 0。账号、任务、设置等逻辑快照保持一致，配置与凭证文件内容未变，持久化文件保留。宿主 `kv.db` 的启动写事务与加载器缓存不以物理字节不变作为账号保留依据。

[再次重启前核验](../artifacts/ui-rework-10.0.0/verification-before-restart.json) 和 [再次重启后核验](../artifacts/ui-rework-10.0.0/verification-after-restart.json) 均确认 UI 10.0.0、六份资源摘要、数据保留和镜像不变。未执行真实登录、模型调用、重置 2FA 或退出设备，也未重放历史任务。

UI 9.4.0 于同日较早完成部署，随后被本版本替代；其功能地图、合成测试与历史安装证据保留在 [UI 9.4.0 说明](UI_REWORK_9_4_0_CN.md)。此前 UI 9.3.0 的登录功能验证见 [0.16.0 实现与验证](LOGIN_EXPERIENCE_0_16_0_CN.md)。
