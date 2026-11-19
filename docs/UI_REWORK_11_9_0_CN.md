# UI 11.9.0：合并安全操作选项与执行流程

已于 **2026-11-17 18:34:29（Asia/Shanghai）** 部署到 [8317 插件页面](http://127.0.0.1:8317/management.html#/plugin-pages/model-quality/0)：**UI 11.9.0 / 插件 0.17.1 / Go＋Sobek 2.1.2**。CPA 8.0.76 宿主保持不变。

## 内容与选项

重置 2FA 弹窗改为一个按执行顺序排列的流程，不再把“后续操作”选项和独立执行预览重复展示。第一项说明“重置并保存新 2FA”，标为“必选”，并直接说明旧密钥失效；第二项“退出所有设备”和第三项“重新授权并启用 CPA”是独立可选操作，各自包含后果说明。

可选项的整行标签均可点击，继续使用真实复选框保留焦点、空格键和选中/禁用语义。复选框采用与 CPA 配色一致的样式，并用“已选择 / 未选择”文字表示状态。选择变化只更新对应项样式、状态和底部“将执行 N 项操作”，不重建流程列表或改变行数。

退出所有设备弹窗复用同一布局：第一项为必选的退出设备，第二项为可选的重新授权。底部计数包含必做主动作；重置操作为 1–3 项，退出设备为 1–2 项。重新授权说明仍明确上传遵循套餐排除设置。

## 稳定布局与行为

保留受视口约束的固定弹窗框、固定标题及底部按钮区；窄屏或短窗口中只滚动正文。默认设置仍为不选择退出设备、选择重新授权。关闭后重新打开恢复默认值。勾选只是编辑本次任务选项，只有确认提交才执行。

顺序仍为先保存新密钥，再按选择退出设备，最后重新授权。后续失败不撤销已完成操作；未知结果仍停止等待核对。原有账号版本、能力、提交锁定、取消/Escape 焦点返回和错误仅滚动弹窗正文等保护保留。

源码范围为 `plugin/ui/index.html`、`style.css`、`login.js`；后端安全动作、工作区选择、套餐上传排除及 CPA 宿主均不改变。此前 UI 11.8 的独立步骤预览是历史设计，本轮以合并后的流程为准。

## 验证与部署

[布局与交互测量](../artifacts/ui-rework-11.9.0/layout-verification.json) 保存在 `artifacts/ui-rework-11.9.0/`。本轮验证使用合成账号，不执行真实重置 2FA、退出设备、登录或模型请求。

| 场景 | 已核对结果 |
| --- | --- |
| 1280 × 720 桌面，重置 2FA | 四种选项组合均为 520 × 552px，x=372.5/y=84，底部高65px，正文高399px且无需滚动。 |
| 390 × 844、320 × 640 手机 | 弹窗固定552px高；切换选项不改变窗口尺寸，没有横向溢出，正文在内部纵向滚动。 |
| 800 × 480 短窗口 | 弹窗固定520 × 432px，正文内部滚动。 |
| 独立退出设备弹窗 | 两种授权选择均为520 × 452px，两个可见步骤，授权步骤编号为02，计数为1或2项。 |

已验证点击整行说明和按空格切换复选框、独立选项和计数、取消/Escape返回来源按钮，以及重开恢复 `logout=false / reauthorize=true`。浅色、深色与纯白主题已目测检查。截图：[桌面](../artifacts/ui-rework-11.9.0/reset-desktop.jpg)、[390px](../artifacts/ui-rework-11.9.0/reset-390x844.jpg)、[320px](../artifacts/ui-rework-11.9.0/reset-320x640.jpg)、[短窗口](../artifacts/ui-rework-11.9.0/reset-800x480.jpg)、[深色](../artifacts/ui-rework-11.9.0/reset-dark.jpg)、[纯白退出设备](../artifacts/ui-rework-11.9.0/logout-white.jpg)。

原生全模块测试、`go vet` 和构建通过；Node客户端/导入16项回归通过，见 [最终验证汇总](../artifacts/ui-rework-11.9.0/verification-summary.json)。[模拟提交核验](../artifacts/ui-rework-11.9.0/fixture-submission-verification.json) 覆盖四种重置组合和两种独立退出设备组合，恰好 6 次 POST，操作类型与两个布尔参数逐一正确，真实安全操作为 0。

320px 窄屏的账号版本变化场景也已通过：错误完整可见，页面滚动位置前后均为192px，弹窗仍高552px；两个选项与提交禁用，取消仍可用。待核对安全操作的账号继续禁用重置入口。

[安装包核验](../artifacts/ui-rework-11.9.0/package-verification.json) 确认运行时测试、插件内嵌模式测试、`go vet`、Linux arm64 原生构建、包校验值和六份 UI 摘要通过。[宿主基线](../artifacts/ui-rework-11.9.0/source-baseline-verification.json) 确认原始 CPA 与构建快照各2,413个记录文件摘要一致。[部署记录](../artifacts/ui-rework-11.9.0/deployment.json)、[首次启动](../artifacts/ui-rework-11.9.0/verification-before-restart.json) 和 [再次重启](../artifacts/ui-rework-11.9.0/verification-after-restart.json) 确认插件已注册并启用、UI 11.9.0 已载入、运行时可用、六份资源摘要匹配，未鉴权管理接口返回401。

升级前已停服备份。升级后保留 **1个插件账号、1条既有终态任务**，模型运行、规则、暂停归属与 CPA 凭证均为0。账号、任务与设置逻辑快照完整相等，不接受默认字段例外；配置/凭证文件和宿主镜像均未改变。后端仍为0.17.1、运行时仍为2.1.2，没有执行真实账号动作或模型请求。

安装包：[Linux arm64 ZIP](../artifacts/ui-rework-11.9.0/plugin-package/model-quality-v0.17.1-linux-arm64.zip)。应核对 [构建清单](../artifacts/ui-rework-11.9.0/plugin-package/build-manifest.json) 的 `ui_version=11.9.0` 与 [SHA256SUMS](../artifacts/ui-rework-11.9.0/plugin-package/SHA256SUMS)，区分后端版本相同但 UI 不同的安装包。

[实际 CPA 页面核验](../artifacts/ui-rework-11.9.0/deployed-layout-verification.json) 确认四种重置组合均为520×552px、x=264.5/y=84，底部按钮区y=570、高65px，选择状态与计数一致。检查后取消，真实提交为0。见 [最终实际截图](../artifacts/ui-rework-11.9.0/deployed-dialog.jpg)。
