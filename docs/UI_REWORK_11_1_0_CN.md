# UI 11.1.0：导航与账号操作排版（历史）

2026-11-17 曾部署到 [8317 的 CPA 插件页面](http://127.0.0.1:8317/management.html#/plugin-pages/model-quality/0)，并完成再次重启核验，**现已被 UI 11.2.0 替代**。本文保留当时 CPA 8.0.76 / 插件 0.16.0 / UI 11.1.0 / Go＋Sobek 2.1.1 的导航与账号排版记录。当前版本见 [UI 11.2.0 说明](UI_REWORK_11_2_0_CN.md)。

| 区域 | 调整后 |
| --- | --- |
| 页面标题与主导航 | 标题、说明在顶部，主导航位于标题下方；刷新与连接入口留在顶部工具区，避免导航先于页面内容 |
| 账号次级导航 | 账号列表、登录任务、代理与巡检融入账号面板；补齐 tablist、tab、tabpanel 关联及选中状态，支持左右方向键、Home、End 切换与焦点移动 |
| 列表工具栏 | 搜索、巡检全部、导入账号统一排列；删除重复的代理设置跳转按钮，原代理配置仍在“代理与巡检”页 |
| 账号行操作 | 主操作为 2FA 登录与账号资料；次操作为日志与更多。日志可直接展开行内记录，“更多”不再重复提供日志入口，继续保留任务、单账号巡检、安全操作与删除 |

测试的 960px 左右分栏、三列凭证状态、可选诊断详情和完整审计元数据继续保留。控件 ID 由 251 个变为 **254 个**：删除旧代理跳转 `account-settings-open`，新增三个账号页签 ID 与 `page-description`。既有账号操作分发、权限检查、任务锁、表单草稿和未知写入处理保持有效。

一步账号登录、历史候选确认、未知提交隔离和安全操作语义未变。打开导航、查看日志和导入资料不会自动发起登录或模型调用；巡检仅读取 CPA 已记录状态。官方宿主仍只提供质量观察，不支持独立质量暂停、释放或自动暂停。

## 验证与部署

- 客户端及导入回归 **16 项通过**：[node-tests.log](../artifacts/ui-rework-11.1.0/node-tests.log)；题目绑定、人工结果、转义与规则视图通过：[question-tests.log](../artifacts/ui-rework-11.1.0/question-tests.log)。
- Linux arm64 登录运行时测试、插件测试、`go vet` 和原生库构建通过：[native-build.log](../artifacts/ui-rework-11.1.0/native-build.log)。
- 原始宿主和构建快照的 **2,413 个基线文件无变更、无缺失**，构建快照无额外文件：[基线核验](../artifacts/ui-rework-11.1.0/source-baseline-verification.json)。
- 插件注册并启用、运行时 2.1.1 可用；六份实际静态资源摘要与源码一致，未鉴权管理请求返回 401；[再次重启前](../artifacts/ui-rework-11.1.0/verification-before-restart.json) 和 [再次重启后](../artifacts/ui-rework-11.1.0/verification-after-restart.json) 检查均通过。

[部署记录](../artifacts/ui-rework-11.1.0/deployment.json) 确认 **2026-11-17 15:15:08（Asia/Shanghai）** 完成更新。停机后备份 70 个运行文件，再替换共享库并核验。升级前后保留 **1 个插件账号**；登录任务、模型运行、规则、暂停归属及 CPA 凭证均为 0。逻辑快照、配置与凭证文件内容保持一致，持久化文件保留，宿主镜像未变。未执行真实登录、模型调用、重置 2FA 或退出设备。

安装包：[Linux arm64 ZIP](../artifacts/ui-rework-11.1.0/plugin-package/model-quality-v0.16.0-linux-arm64.zip)；核对 [构建清单](../artifacts/ui-rework-11.1.0/plugin-package/build-manifest.json) 的 `ui_version=11.1.0` 和 [SHA256SUMS](../artifacts/ui-rework-11.1.0/plugin-package/SHA256SUMS)。部署脚本继续在停机前检查版本及六份 UI 源文件摘要，并保留备份、状态快照、就绪检查与旧库恢复流程。

## 实际页面核验

[验证汇总](../artifacts/ui-rework-11.1.0/verification-summary.json) 记录：正式 CPA 窗口宽 1280px 时，插件 iframe 宽 1064px；账号表可视宽与滚动宽均为 974px，没有横向溢出。重复代理跳转元素已删除，账号行依次提供“2FA 登录、账号资料、日志、更多”。更多菜单按 Escape 关闭后焦点返回原按钮，浏览器控制台无错误。

另用合成页面检查 1064px、390px 布局、账号页签 ArrowRight/Home/End 切换和直接日志入口。这些交互检查未执行真实登录或模型调用。

![实际 CPA 账号页面](../artifacts/ui-rework-11.1.0/deployed-cpa-accounts.jpg)

UI 11.0.0 的配色来源、截图和部署证据保留在 [历史说明](UI_REWORK_11_0_0_CN.md)。
