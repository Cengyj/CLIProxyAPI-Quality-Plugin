# UI 11.6.0：账号列表工具栏重排

> 历史记录：当前部署已更新为 [UI 11.7.0 / 插件 0.17.0](UI_REWORK_11_7_0_CN.md)，以下保留 UI 11.6.0 当时的实现与验证证据。

已部署到 [8317 插件页面](http://127.0.0.1:8317/management.html#/plugin-pages/model-quality/0)，版本为 **CPA 8.0.76 / 插件 0.16.0 / UI 11.6.0 / Go＋Sobek 2.1.1**。

本轮只调整用户截图中的搜索、状态筛选和巡检/导入区域：

- 桌面合为一行：左侧分段状态筛选与计数徽标，右侧搜索、巡检和导入；搜索最大宽度 260px。
- 搜索与操作按钮统一 36px 高，筛选按钮在 36px 容器内保持 28px 高，减少原先两排控件之间的留白。
- ≤900px 改为两行；≤600px 的第一行是四个筛选，第二行保留搜索、巡检图标及“导入”短标签。完整 `aria-label` 与提示仍为“巡检全部”和“导入账号”。

全部 JavaScript 与既有元素 ID 均未变更。账号操作列继续沿用 [UI 11.5 的参考布局](UI_REWORK_11_5_0_CN.md)。

## 验证与部署

[桌面测量](../artifacts/ui-rework-11.6.0/desktop-toolbar-measurements.json) 确认 1064px 视口下工具栏高 **68px**，三组控件同高、同基线；[手机测量](../artifacts/ui-rework-11.6.0/mobile-toolbar-measurements.json) 确认 390px 下工具栏高 **110px**，两行布局正常。840px 深色布局也已核对，工具栏为两行、高 116px。320px 只验证工具栏自身适配，不据此声称整个页面不存在横向溢出。

[交互检查](../artifacts/ui-rework-11.6.0/interaction-checks.json) 使用合成账号验证待处理筛选为 3 条、搜索失败账号为 1 条、无结果状态、进行中筛选为 1 条，以及导入窗口取消后焦点回到导入按钮。未提交导入、执行巡检或调用模型。

`scripts/build_ui.sh` 语法检查通过；由于 JavaScript 未变，本轮未重复 Node 客户端/导入回归。登录运行时、插件测试、`go vet` 及 Linux arm64 原生构建通过，见 [构建日志](../artifacts/ui-rework-11.6.0/native-build.log)。[宿主基线](../artifacts/ui-rework-11.6.0/source-baseline-verification.json) 确认原始 CPA 与构建副本各 2,413 个记录文件一致，构建副本无额外文件。

[部署记录](../artifacts/ui-rework-11.6.0/deployment.json) 确认 **2026-11-17 16:47:12（Asia/Shanghai）** 完成更新；[再次重启核验](../artifacts/ui-rework-11.6.0/verification-after-restart.json) 确认插件已启用、运行时可用，六份静态资源摘要与源码一致，未鉴权管理请求返回 401。

升级保留 **1 个插件账号、1 条既有终态登录任务**；模型运行、规则、暂停归属与 CPA 凭证均为 0。账号和设置逻辑快照、配置与凭证文件、宿主镜像保持一致，未执行真实账号操作或模型调用。

[实际 CPA 截图](../artifacts/ui-rework-11.6.0/deployed-cpa-accounts.jpg)

安装包：[Linux arm64 ZIP](../artifacts/ui-rework-11.6.0/plugin-package/model-quality-v0.16.0-linux-arm64.zip)。核对 [构建清单](../artifacts/ui-rework-11.6.0/plugin-package/build-manifest.json) 中的 `ui_version=11.6.0` 与 [SHA256SUMS](../artifacts/ui-rework-11.6.0/plugin-package/SHA256SUMS)。
