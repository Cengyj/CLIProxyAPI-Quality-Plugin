# UI 11.12.0：仅保留流程名称与箭头

已于2026-11-17 **19:22:39（Asia/Shanghai）** 部署到 [8317插件页面](http://127.0.0.1:8317/management.html#/plugin-pages/model-quality/0)：**UI11.12.0 / 插件0.17.1 / 运行时2.1.2**。

按用户要求移除底部流程里的“必选 / 执行 / 跳过”文字，只显示步骤名称、箭头和选中视觉差异。原生复选框、计数与业务行为不变，可选流程节点通过不显示的 `aria-label` 保留已选/未选语义。重置与退出弹窗分别缩短14px，高度为486px和420px，短窗口仍受视口限制。

[布局核验](../artifacts/ui-rework-11.12.0/layout-verification.json) 已通过：四种重置组合均为480×486px，流程状态文字元素为0，步骤名称同排；320px手机为281×486px且正文无溢出，独立退出设备为480×420px、两个节点。见 [手机截图](../artifacts/ui-rework-11.12.0/mobile.jpg)。静态检查、原生全量测试、`go vet`与构建通过。本轮无安全操作、真实登录或模型调用。

8317实际页面的 [截图](../artifacts/ui-rework-11.12.0/deployed-dialog.jpg) 与 [测量](../artifacts/ui-rework-11.12.0/deployed-layout-verification.json) 确认重置弹窗为480×486px、可见流程状态文字元素为0；检查后取消，真实安全提交为0。完整结论见 [验收摘要](../artifacts/ui-rework-11.12.0/verification-summary.json)。

[部署](../artifacts/ui-rework-11.12.0/deployment.json) 与 [重启核验](../artifacts/ui-rework-11.12.0/verification-after-restart.json) 通过：六份UI摘要匹配、未鉴权返回401，1个账号/1条既有任务及全部设置快照完整相等，配置、凭证文件和宿主镜像不变。安装包：[Linux arm64 ZIP](../artifacts/ui-rework-11.12.0/plugin-package/model-quality-v0.17.1-linux-arm64.zip)，核对 [清单](../artifacts/ui-rework-11.12.0/plugin-package/build-manifest.json) 的 `ui_version=11.12.0` 与 [SHA256SUMS](../artifacts/ui-rework-11.12.0/plugin-package/SHA256SUMS)。
