# UI 11.13.0：精简测试设置底部

已于2026-11-17 **19:45:12（Asia/Shanghai）** 部署到 [8317插件页面](http://127.0.0.1:8317/management.html#/plugin-pages/model-quality/0)：**UI11.13.0 / 插件0.17.1 / 运行时2.1.2**。

移除测试设置底部“预计生成 N 份独立回答 / 不自动评分，也不会暂停凭证”整块信息栏及留白，同时删除不再使用的 `updatePlan` 和对应样式。开始测试按钮的禁用条件、准入校验、`title` 提示和字段错误说明保留；回答数量仍由原输入项设置，模型执行及质量行为不变。

[离线界面核验](../artifacts/ui-rework-11.13.0/layout-verification.json) 确认底部信息栏元素为0，正常配置的3份回答可开始测试；人工禁用凭证仍阻止提交，按钮提示与字段原因保留。见 [离线截图](../artifacts/ui-rework-11.13.0/offline-manual.jpg)。[原生测试、vet与构建](../artifacts/ui-rework-11.13.0/package-verification.json) 通过；本轮无真实模型调用或账号操作。

8317实际页面的 [测量](../artifacts/ui-rework-11.13.0/deployed-layout-verification.json) 确认底部信息栏元素为0、配置区仅保留两组内容，未选凭证时开始按钮仍禁用；滚动核对卡片底部后确认灰色信息栏及留白已消失。见 [实际页面顶部截图](../artifacts/ui-rework-11.13.0/deployed-manual.jpg)，未提交测试。

[部署](../artifacts/ui-rework-11.13.0/deployment.json) 与 [重启核验](../artifacts/ui-rework-11.13.0/verification-after-restart.json) 通过：六份UI摘要匹配、未鉴权返回401。本轮开始时既有的1个账号、2条历史任务、1个CPA凭证及全部设置快照完整相等，运行/规则/动作均为0，配置、凭证文件和宿主镜像不变。安装包：[Linux arm64 ZIP](../artifacts/ui-rework-11.13.0/plugin-package/model-quality-v0.17.1-linux-arm64.zip)，核对 [清单](../artifacts/ui-rework-11.13.0/plugin-package/build-manifest.json) 的 `ui_version=11.13.0` 与 [SHA256SUMS](../artifacts/ui-rework-11.13.0/plugin-package/SHA256SUMS)。
