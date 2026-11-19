# UI 11.14.0：套餐胶囊与设置精简

已于2026-11-17 **19:54:16（Asia/Shanghai）** 部署到 [8317插件页面](http://127.0.0.1:8317/management.html#/plugin-pages/model-quality/0)：**UI11.14.0 / 插件0.17.1 / 运行时2.1.2**。

移除登录设置底部“服务状态”整块折叠区，以及对应的 `login-engine` 渲染和样式。套餐参考目标项目的 [PlanBadge.jsx](/Users/ceng/Downloads/toSub2-go-port-2026.11.17/web/src/components/PlanBadge.jsx) 与 `web/src/styles.css` 第617行附近规则，并结合 CPA 实际 AuthFileQuota 的圆胶囊形状。列表标签20px高、10px字号，排除选项30px高，保留原生复选框、选中、焦点与禁用状态。Free灰、Plus蓝、Go绿、Team紫、Enterprise灰蓝、Edu玫瑰、Pro柔银，Pro Lite及Business Premium淡金；未知套餐为中性虚线。授权快照及排除保存语义不变。

[布局与交互](../artifacts/ui-rework-11.14.0/layout-verification.json) 通过：六个列表样例身份区仍高45px，标签20px且无截断；10个排除项均高30px。390px窄屏的375px内容无横向溢出，浅色/白色/暗色与键盘空格选中通过，服务状态元素为0。见 [列表](../artifacts/ui-rework-11.14.0/offline-accounts.jpg)、[设置](../artifacts/ui-rework-11.14.0/offline-settings.jpg)、[暗色](../artifacts/ui-rework-11.14.0/offline-settings-dark.jpg) 和 [手机](../artifacts/ui-rework-11.14.0/offline-settings-mobile.jpg) 截图。[原生测试、vet与构建](../artifacts/ui-rework-11.14.0/package-verification.json) 通过；未保存真实设置，未进行真实账号操作或模型调用。

8317实际页面的 [测量](../artifacts/ui-rework-11.14.0/deployed-layout-verification.json) 确认 Business Premium 标签高20px、无截断，10个排除项均高30px，“服务状态”元素为0。见 [实际账号列表](../artifacts/ui-rework-11.14.0/deployed-accounts.jpg) 和 [实际设置底部](../artifacts/ui-rework-11.14.0/deployed-settings.jpg)；检查未保存设置或执行账号操作。

[部署](../artifacts/ui-rework-11.14.0/deployment.json) 与 [重启核验](../artifacts/ui-rework-11.14.0/verification-after-restart.json) 通过：六份UI摘要匹配、未鉴权返回401。1个账号、2条历史任务、1个CPA凭证及全部设置快照完整相等，运行/规则/动作均为0，配置、凭证文件和宿主镜像不变。安装包：[Linux arm64 ZIP](../artifacts/ui-rework-11.14.0/plugin-package/model-quality-v0.17.1-linux-arm64.zip)，核对 [清单](../artifacts/ui-rework-11.14.0/plugin-package/build-manifest.json) 的 `ui_version=11.14.0` 与 [SHA256SUMS](../artifacts/ui-rework-11.14.0/plugin-package/SHA256SUMS)。
