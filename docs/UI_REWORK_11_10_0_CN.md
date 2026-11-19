# UI 11.10.0：横向安全操作流程

已于 **2026-11-17 18:55:06（Asia/Shanghai）** 部署到 [8317 插件页面](http://127.0.0.1:8317/management.html#/plugin-pages/model-quality/0)：**UI 11.10.0 / 插件 0.17.1 / Go＋Sobek 2.1.2**。CPA 8.0.76 宿主保持不变。

按用户要求，在原有选项卡片下恢复“执行流程”，用横向节点与箭头展示 **重置 2FA → 退出设备 → 授权 CPA**。三个节点等分可用宽度；未选步骤保留位置，显示“跳过”并使用虚线边框与连接线，避免流程随勾选重排。必选主动作显示“必选”，被选中的后续步骤显示“已选择”，这些是提交前的选择状态，不表示动作已经完成。

独立退出设备弹窗显示 **退出设备 → 授权 CPA** 两个节点。保留 UI 11.9 的整行可点选项、原生复选框、底部操作计数、固定窗口和默认 `logout=false / reauthorize=true`。修改选项会同步卡片、横向流程和计数，只有点击确认才提交。

本轮只调整 `plugin/ui/index.html`、`style.css`、`login.js`，不改变后端、安全动作顺序、工作区选择、套餐排除或 CPA 宿主。[12组布局测量](../artifacts/ui-rework-11.10.0/layout-verification.json) 已通过：桌面四种重置组合均为520×624px，正文471px且无需滚动，68px高的流程条始终为三个等宽节点；390px手机高624px、320px手机高618px，节点仍同排且文字无横向溢出；短窗口高432px，正文内部滚动。独立退出设备的两种选择均为520×528px，显示两个节点，状态及实线/虚线正确；Escape返回“更多”按钮。

已检查三套主题，截图：[桌面](../artifacts/ui-rework-11.10.0/flow-desktop.jpg)、[深色](../artifacts/ui-rework-11.10.0/flow-dark.jpg)、[退出设备](../artifacts/ui-rework-11.10.0/flow-logout.jpg)、[390px](../artifacts/ui-rework-11.10.0/flow-390x844.jpg)、[320px](../artifacts/ui-rework-11.10.0/flow-320x640.jpg)、[短窗口](../artifacts/ui-rework-11.10.0/flow-800x480.jpg)。Node16项回归、原生测试、`go vet` 与构建通过。本轮仅验证预览变化，未提交真实或模拟安全动作，也未登录或调用模型。[安装包核验](../artifacts/ui-rework-11.10.0/package-verification.json)、[宿主基线](../artifacts/ui-rework-11.10.0/source-baseline-verification.json)、[部署记录](../artifacts/ui-rework-11.10.0/deployment.json) 和 [再次重启核验](../artifacts/ui-rework-11.10.0/verification-after-restart.json) 已通过。六份静态资源摘要与源码一致，管理接口未鉴权返回401。升级前已停服备份；1个账号、1条终态任务及全部设置快照完整相等，其他运行/规则/暂停归属/CPA凭证为0，配置、凭证文件和宿主镜像不变。

安装包：[Linux arm64 ZIP](../artifacts/ui-rework-11.10.0/plugin-package/model-quality-v0.17.1-linux-arm64.zip)，请同时核对 [构建清单](../artifacts/ui-rework-11.10.0/plugin-package/build-manifest.json) 的 `ui_version=11.10.0` 与 [SHA256SUMS](../artifacts/ui-rework-11.10.0/plugin-package/SHA256SUMS)。

[最终验证汇总](../artifacts/ui-rework-11.10.0/verification-summary.json) 与 [实际 CPA 测量](../artifacts/ui-rework-11.10.0/deployed-layout-verification.json) 确认四种重置组合均为520×624px、x=264.5/y=48，节点y=505始终同排。检查后取消，真实提交为0，账号和配置不变。见 [实际页面截图](../artifacts/ui-rework-11.10.0/deployed-dialog.jpg)。
