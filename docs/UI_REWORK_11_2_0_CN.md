# UI 11.2.0：账号操作按钮对齐

> 历史记录：本版已由 [UI 11.3.0 紧凑操作按钮](UI_REWORK_11_3_0_CN.md) 替代。以下尺寸与部署证据对应 UI 11.2.0。

2026-11-17 已部署到 [CPA 插件页面](http://127.0.0.1:8317/management.html#/plugin-pages/model-quality/0)，并完成再次重启核验。当时版本为 **CPA 8.0.76 / 插件 0.16.0 / UI 11.2.0 / Go＋Sobek 2.1.1**。

本轮唯一界面变化是账号行的四个操作按钮：统一为等宽 **2×2 网格**和相同的中性样式，上排登录、账号资料，下排查看日志、更多操作。桌面按钮高 **34px**，手机高 **44px**，图标 **14px**，水平与垂直间距均为 **6px**。登录、登录中、重新登录和待核对等文案使用同一网格，不再改变相邻按钮的宽度或对齐位置。

固定网格的思路参考只读文件 `/Users/ceng/Downloads/toSub2-go-port-2026.11.17/web/src/components/account-actions.css`，按插件现有四个操作改为两列；未修改参考源码。JavaScript 仅调整呈现标记、图标和标签，原有操作标识、事件分发、权限检查与任务锁保持不变。页面导航、账号资料、日志、菜单、安全操作和其他工作区沿用 UI 11.1。

## 尺寸与验证

| 检查 | 结果与证据 |
| --- | --- |
| 桌面六行合成账号 | 每个按钮均为 **102×34px**、图标 14px、间距 6px，见 [桌面测量](../artifacts/ui-rework-11.2.0/desktop-button-measurements.json) |
| 手机账号行 | 四个按钮均为 **151.5×44px**、图标 14px、间距 6px，见 [手机测量](../artifacts/ui-rework-11.2.0/mobile-button-measurements.json) |
| 前端 | JavaScript 语法检查通过；客户端与导入 **16 项回归通过**，见 [node-tests.log](../artifacts/ui-rework-11.2.0/node-tests.log) |
| 原生构建 | 登录运行时测试、插件测试、`go vet` 和 Linux arm64 原生库构建通过，见 [native-build.log](../artifacts/ui-rework-11.2.0/native-build.log) |
| 宿主源码 | 2,413 个记录文件在原始宿主与构建快照中无变更、无缺失，构建快照无额外文件，见 [基线核验](../artifacts/ui-rework-11.2.0/source-baseline-verification.json) |

以上宽度为相应页面的实际测量值；按钮列仍按可用空间等分。尺寸测量使用合成账号，不代表真实账号登录或安全操作验证。

[实际 CPA 账号页截图](../artifacts/ui-rework-11.2.0/deployed-cpa-accounts.jpg) · [手机布局截图](../artifacts/ui-rework-11.2.0/mobile-aligned-actions.jpg)

## 部署与数据保留

[部署记录](../artifacts/ui-rework-11.2.0/deployment.json) 确认 **2026-11-17 15:37:55（Asia/Shanghai）** 完成更新。停机备份 70 个运行文件后替换插件库；[再次重启前](../artifacts/ui-rework-11.2.0/verification-before-restart.json) 和 [再次重启后](../artifacts/ui-rework-11.2.0/verification-after-restart.json) 均确认插件注册并启用、运行时 2.1.1 可用，六份静态资源摘要与源码一致，未鉴权管理请求返回 401。

升级保留 **1 个插件账号**，登录任务、模型运行、规则、暂停归属和 CPA 凭证均为 0。账号与设置逻辑快照、配置及凭证文件内容、宿主镜像保持一致，持久化文件保留。未执行真实登录、模型调用、重置 2FA 或退出设备。

安装包：[Linux arm64 ZIP](../artifacts/ui-rework-11.2.0/plugin-package/model-quality-v0.16.0-linux-arm64.zip)。安装前核对 [构建清单](../artifacts/ui-rework-11.2.0/plugin-package/build-manifest.json) 的 `ui_version=11.2.0` 和 [SHA256SUMS](../artifacts/ui-rework-11.2.0/plugin-package/SHA256SUMS)。导航与账号功能的上一轮记录保留在 [UI 11.1 历史说明](UI_REWORK_11_1_0_CN.md)。
