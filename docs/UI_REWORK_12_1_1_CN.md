# UI 12.1.1：在原有界面上提高扫读和比较效率

2026-11-18 13:16（北京时间）部署到[本机 8317 的 CPA 管理页](http://127.0.0.1:8317/management.html#/plugin-pages/model-quality/0)：插件 **0.18.2**，宿主 CPA **8.0.76**，内嵌运行时 **2.1.3**。刷新整个管理标签页，点击「已连接」或「连接管理接口」，弹窗左下角可直接核对 `UI 12.1.1`，不必重新输入密钥。18417 仍只用于合成预览。

## 实际变化

| 工作区 | 保留与调整 | 合成数据截图 |
| --- | --- | --- |
| 账号管理 | 保留横向导航、三种主题、原有状态与套餐配色。收紧页头、筛选工具栏和行间距；账号记录与工具栏分色。两行三列操作的位置、文字和确认流程未变，资料、日志、更多以较轻的样式显示。 | [原版桌面](../artifacts/ui-rework-12.1.0/browser/embedded-1064-login.png) · [本版桌面](../artifacts/ui-rework-12.1.1/browser/embedded-1064-login.png) · [本版手机](../artifacts/ui-rework-12.1.1/browser/embedded-390-login.png) |
| 智商测试 | 回答正文与工具、耗时和用量分色。阅读栏够宽时两份回答并排，不足 640px 就上下排列，不再挤成两窄列；短回答的操作紧跟自己的正文。原有草稿、历史和手机三种视图均保留。 | [桌面双栏](../artifacts/ui-rework-12.1.1/browser/manual-desktop.png) · [320px 回答](../artifacts/ui-rework-12.1.1/browser/manual-answers-320.png) |
| 质量观察 | 同一统计栏并列展示通过率、覆盖率、结论未知与已知用量；小屏排成两列，数字不与说明分离。用量有值时显示 token 单位；没有数据的横线、实际零值与未知结论仍分别展示，统计口径未改。 | [桌面统计栏](../artifacts/ui-rework-12.1.1/browser/embedded-1064-operations.png) · [320px 统计栏](../artifacts/ui-rework-12.1.1/browser/embedded-320-operations.png) |

[同数据、同视口实测](../artifacts/ui-rework-12.1.1/browser/baseline-comparison.json)：1064px 嵌入页的首行起点从 378px 提前到 314px，账号行从 97px 减到 85px；390px 的账号记录从 291px 减到 272px，操作按钮仍至少 44px 高。测量的是合成数据，不是真实账号截图。

## 验证与交付

- [33 项前端测试](../artifacts/ui-rework-12.1.1/frontend-tests.log)、[68 组布局检查和 18 张截图](../artifacts/ui-rework-12.1.1/browser/verification.json)通过。覆盖三个工作区、三种主题、窄屏、并排与堆叠阅读、操作列对齐、草稿、焦点返回及读取失败。交互测试未调用模型或写入管理接口。
- [Linux arm64 安装包](../artifacts/ui-rework-12.1.1/plugin-package/model-quality-v0.18.2-linux-arm64.zip)仅包含 `model-quality.so`。[构建和模块测试](../artifacts/ui-rework-12.1.1/native-build.log)、[构建清单](../artifacts/ui-rework-12.1.1/plugin-package/build-manifest.json)及[隔离宿主加载与重启](../artifacts/ui-rework-12.1.1/native-smoke.json)均通过。官方宿主未改；[2,413 项文件摘要](../artifacts/ui-rework-12.1.1/host-baseline.json)、[宿主编译及测试](../artifacts/ui-rework-12.1.1/host-check.log)和[私有网卡上的 WebRTC 测试](../artifacts/ui-rework-12.1.1/host-live.log)均已验证。
- [8317 升级记录](../artifacts/ui-rework-12.1.1/deployment.json)：无运行中任务或已启用的自动化；停服备份并核对后，只替换插件库。1 个插件账号、2 条历史登录任务、原始日志、设置和 1 个 CPA 凭证均保留，配置、凭证文件与宿主镜像未变。插件已启用，六份在线静态资源与本轮构建摘要一致。没有执行账号登录、安全操作或模型调用。
- [实际面板核验](../artifacts/ui-rework-12.1.1/deployed-iframe.json)：在临时浏览器中认证进入 8317 官方面板，桌面和 390px 窗口都从内嵌插件弹窗读到 `UI 12.1.1`，宿主工具未覆盖连接或导航。只允许本机 GET，没有读取插件的账号业务数据或保存真实页面截图。
