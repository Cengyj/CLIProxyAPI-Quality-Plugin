# UI 11.11.0：精简账号安全弹窗

已于 **2026-11-17 19:09:48（Asia/Shanghai）** 部署到 [8317 插件页面](http://127.0.0.1:8317/management.html#/plugin-pages/model-quality/0)：**UI 11.11.0 / 插件 0.17.1 / Go＋Sobek 2.1.2**，CPA 8.0.76 宿主不变。

弹窗移除重复的主动作卡、步骤编号和重复选中状态。简短的主动作说明置于顶部；两条等高后续选项合入一个分组，整行可点，保留真实复选框。标题和帮助说明使用精简文案，单行完整显示。

底部保留“重置 2FA → 退出设备 → 授权 CPA”，将三个独立方框改为一条浅灰流程带；操作计数移到流程标题旁。主动作显示“必选”，后续步骤显示“执行/跳过”，仅表示待执行计划；未选节点保留位置与虚线连接。独立退出设备显示两个节点。

保留固定窗口、内部滚动、取消与错误焦点、版本/能力/提交锁定、默认 `logout=false / reauthorize=true`。本轮仅改变三个 UI 文件，后端、安全顺序、套餐排除与 CPA 宿主不变。[12组布局测量](../artifacts/ui-rework-11.11.0/layout-verification.json) 已通过：桌面四组合480×500px、正文353px；320/390px手机高500px、正文347px；800×480短窗口为480×432px；独立退出设备为480×434px。桌面、手机与独立退出弹窗正文均无需滚动。所有说明高度等于单行行高、文字宽度不超容器，无省略或换行；两选项同为66px高且复选框对齐，横向流程保持同排。

整行点击、空格切换、默认false/true、取消焦点返回已核对；背景滚动位置前后保持49px。账号版本变化时错误可见，选项和提交禁用、取消可用。Node16项、静态检查、原生测试、`go vet` 与构建通过，本轮无安全提交、真实登录或模型调用。

截图：[桌面](../artifacts/ui-rework-11.11.0/polish-desktop.jpg)、[390px](../artifacts/ui-rework-11.11.0/polish-390x844.jpg)、[320px](../artifacts/ui-rework-11.11.0/polish-320x640.jpg)、[短窗口](../artifacts/ui-rework-11.11.0/polish-800x480.jpg)。[安装包核验](../artifacts/ui-rework-11.11.0/package-verification.json)、[宿主基线](../artifacts/ui-rework-11.11.0/source-baseline-verification.json)、[部署](../artifacts/ui-rework-11.11.0/deployment.json) 与 [重启核验](../artifacts/ui-rework-11.11.0/verification-after-restart.json) 均通过。六份 UI 摘要匹配，未鉴权管理请求返回401。停服备份后更新，1个账号、1条既有任务及全部设置快照完整相等，其他运行/规则/暂停归属/CPA凭证为0；配置、凭证文件和宿主镜像不变。

安装包：[Linux arm64 ZIP](../artifacts/ui-rework-11.11.0/plugin-package/model-quality-v0.17.1-linux-arm64.zip)。请核对 [构建清单](../artifacts/ui-rework-11.11.0/plugin-package/build-manifest.json) 的 `ui_version=11.11.0` 及 [SHA256SUMS](../artifacts/ui-rework-11.11.0/plugin-package/SHA256SUMS)。

[最终验证汇总](../artifacts/ui-rework-11.11.0/verification-summary.json) 与 [实际 CPA 测量](../artifacts/ui-rework-11.11.0/deployed-layout-verification.json) 确认四种重置组合均为480×500px、x=284.5/y=110，两选项均高66px，所有说明单行完整。检查后取消，真实提交为0。见 [实际页面截图](../artifacts/ui-rework-11.11.0/deployed-dialog.jpg)。
