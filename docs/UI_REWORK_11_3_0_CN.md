# UI 11.3.0：紧凑的账号操作按钮

> 历史记录：当前账号操作列已改为 UI 11.4.0 的单行文字操作，见 [当前说明](UI_REWORK_11_4_0_CN.md)。以下保留本轮原始规格与部署证据。

2026-11-17 已部署到 [CPA 插件页面](http://127.0.0.1:8317/management.html#/plugin-pages/model-quality/0)，并完成再次重启核验。版本为 **CPA 8.0.76 / 插件 0.16.0 / UI 11.3.0 / Go＋Sobek 2.1.1**。

账号行的四个操作继续使用等宽 **2×2 网格**，尺寸与视觉重量进一步收紧：

| 项目 | 当前规格 |
| --- | --- |
| 桌面按钮 | 高 28px，字号 11px，行高 16px |
| ≤600px 按钮 | 高 36px，字号 12px |
| 图标与间距 | 图标 13px，水平与垂直间距 4px |
| 外观 | 透明背景，CPA 边框色 70% 不透明度，5px 圆角 |
| 动作组 | 最大宽度 192px，靠右排列 |
| 桌面操作列 | 220px；≤1000px 时 212px |

本轮只修改 `style.css` 与 `index.html` 中的构建版本；四个 JavaScript 文件与 UI 11.2.0 完全一致。既有导航、账号操作与权限检查继续沿用。固定网格参考来源见 [UI 11.2 历史说明](UI_REWORK_11_2_0_CN.md)。

## 验证与部署

- 六行合成账号的按钮均为 **92×28px**，包括登录中和待核对状态，见 [桌面测量](../artifacts/ui-rework-11.3.0/desktop-button-measurements.json)。390px 手机布局中均为 **94×36px**，见 [手机测量](../artifacts/ui-rework-11.3.0/mobile-button-measurements.json)。文字未裁切，页面未横向溢出。
- 已检查浅色、白色、深色主题，并用合成账号核对日志及更多菜单的展开与关闭。
- `scripts/build_ui.sh` 语法检查通过。JavaScript 未变更，本轮未重复运行 Node 客户端/导入回归；上一轮结果保留于 UI 11.2 记录。
- 登录运行时与插件测试、`go vet`、Linux arm64 原生库构建通过，见 [构建日志](../artifacts/ui-rework-11.3.0/native-build.log)。
- 原始 CPA 与构建快照中的 2,413 个基线文件均无变更、无缺失；构建快照没有额外文件，见 [独立基线核验](../artifacts/ui-rework-11.3.0/source-baseline-verification.json)。

[部署记录](../artifacts/ui-rework-11.3.0/deployment.json) 确认 **2026-11-17 15:50:40（Asia/Shanghai）** 完成更新。停机备份 70 个运行文件后替换插件库；[重启前核验](../artifacts/ui-rework-11.3.0/verification-before-restart.json) 与 [再次重启后核验](../artifacts/ui-rework-11.3.0/verification-after-restart.json) 确认插件注册、启用及运行时正常，六份静态资源摘要与源码一致，未鉴权管理请求返回 401。

升级保留 **1 个插件账号**，登录任务、模型运行、规则、暂停归属与 CPA 凭证均为 0。账号和设置逻辑快照、配置与凭证文件、宿主镜像保持一致。验证未执行真实登录、模型调用、重置 2FA 或退出设备。

[实际 CPA 账号页截图](../artifacts/ui-rework-11.3.0/deployed-cpa-accounts.jpg) · [手机布局截图](../artifacts/ui-rework-11.3.0/mobile-compact-actions.jpg)

安装包：[Linux arm64 ZIP](../artifacts/ui-rework-11.3.0/plugin-package/model-quality-v0.16.0-linux-arm64.zip)。安装前核对 [构建清单](../artifacts/ui-rework-11.3.0/plugin-package/build-manifest.json) 的 `ui_version=11.3.0` 和 [SHA256SUMS](../artifacts/ui-rework-11.3.0/plugin-package/SHA256SUMS)。
