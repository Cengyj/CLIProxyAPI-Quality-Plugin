# UI 11.4.0：账号操作列单行重排

> 历史记录：当前账号操作列与部署已由 [UI 11.5.0](UI_REWORK_11_5_0_CN.md) 替代。以下保留当时的实现与验证。

已部署到 [8317 插件页面](http://127.0.0.1:8317/management.html#/plugin-pages/model-quality/0)。当前版本为 **CPA 8.0.76 / 插件 0.16.0 / UI 11.4.0 / Go＋Sobek 2.1.1**。

账号列表“操作”列改为单行：**登录 · 资料 · 日志 · ⋯**。登录保留当前状态文案；资料和日志使用简短文字及完整无障碍名称，省略号打开既有更多菜单。移除双行分组、前三项图标和常态边框，悬停与键盘焦点仍有反馈。

| 项目 | 规格 |
| --- | --- |
| 控件 | 字号 12px、圆角 4px，桌面高 28px、≤600px 高 36px |
| 排列 | 登录弹性列，后续 36/36/28px 固定列，间距 2px，组宽不超过 192px |
| 更多图标 | 16px，描边 2.2 |
| 桌面操作列 | 220px；≤1000px 时 212px |

本轮仅调整 `login.js` 的操作列标记、`style.css` 和构建版本。动作标识、事件分发、权限检查、未知结果保护及焦点恢复逻辑保持原样。

## 验证

- 1064px 桌面六行合成账号：四项宽度 **82/36/36/28px**、高 **28px**，均在同一行并按列对齐，见 [桌面测量](../artifacts/ui-rework-11.4.0/desktop-action-measurements.json)。390px 手机布局宽度为 **86/36/36/28px**、高 **36px**，见 [手机测量](../artifacts/ui-rework-11.4.0/mobile-action-measurements.json)。文字无裁切，页面无横向溢出。
- 合成账号核对资料弹窗、行内日志和更多菜单；关闭资料后焦点回到资料入口，Escape 关闭菜单后回到更多入口，见 [交互检查](../artifacts/ui-rework-11.4.0/interaction-checks.json)。
- JavaScript 语法检查通过。该改动限于呈现标记，本轮未重复运行 Node 客户端/导入测试套件。
- 原生登录运行时、插件测试、`go vet` 和 Linux arm64 原生库构建通过，见 [构建日志](../artifacts/ui-rework-11.4.0/native-build.log)。
- 原始 CPA 与构建副本的 **2,413** 个基线文件全部一致；构建副本无额外文件，见 [独立基线核验](../artifacts/ui-rework-11.4.0/source-baseline-verification.json)。

## 部署

[部署记录](../artifacts/ui-rework-11.4.0/deployment.json) 确认 **2026-11-17 15:59:31（Asia/Shanghai）** 完成更新。停机备份后替换插件库；[再次重启核验](../artifacts/ui-rework-11.4.0/verification-after-restart.json) 确认插件启用、运行时可用，六份静态资源与源码一致，未鉴权管理请求返回 401。

升级保留 **1 个插件账号**，任务、模型运行、规则、暂停归属和 CPA 凭证均为 0。账号和设置逻辑快照、配置与凭证文件、宿主镜像保持一致。未执行真实登录、模型调用、重置 2FA 或退出设备。

[实际 CPA 截图](../artifacts/ui-rework-11.4.0/deployed-cpa-accounts.jpg) · [手机截图](../artifacts/ui-rework-11.4.0/mobile-inline-actions.jpg)

安装包：[Linux arm64 ZIP](../artifacts/ui-rework-11.4.0/plugin-package/model-quality-v0.16.0-linux-arm64.zip)。核对 [构建清单](../artifacts/ui-rework-11.4.0/plugin-package/build-manifest.json) 中的 `ui_version=11.4.0` 与 [SHA256SUMS](../artifacts/ui-rework-11.4.0/plugin-package/SHA256SUMS)。上轮说明保留于 [UI 11.3 历史记录](UI_REWORK_11_3_0_CN.md)。
