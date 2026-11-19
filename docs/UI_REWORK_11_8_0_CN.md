# UI 11.8.0：稳定的账号安全确认弹窗

已于 **2026-11-17 18:12:16（Asia/Shanghai）** 部署到 [8317 插件页面](http://127.0.0.1:8317/management.html#/plugin-pages/model-quality/0)：**UI 11.8.0 / 插件 0.17.1 / Go＋Sobek 2.1.2**。宿主继续使用未修改的 CPA 8.0.76。

## 弹窗排版

此前重置 2FA 确认弹窗会随“同时退出所有设备”和“完成后重新授权”的勾选增减步骤，改变弹窗高度并使底部按钮移动。本轮将弹窗改为受视口高度约束的稳定布局：标题、账号身份和底部确认区保持位置，中间内容超高时只在弹窗正文内滚动。

重置 2FA 的两个附加选项使用紧凑行，退出设备弹窗仅保留后续授权选项；继续使用原生复选框、完整名称与后果说明。重置 2FA 的步骤预览始终占三行：重置并保存新 2FA、退出所有设备、重新授权并保存到 CPA；退出设备弹窗始终占两行：退出所有设备、重新授权并保存到 CPA。每行显示“执行”或“跳过”，并给出本次执行步数。切换选项只改变步骤的执行状态和计数，不通过增加或删除预览行改变整体高度。

执行顺序仍为先保存新 2FA，再退出设备，最后重新授权。未选中的附加步骤明确标为跳过，不能仅靠颜色区分。窄屏和短窗口允许正文滚动，确认与取消按钮保持可访问；长账号和提示内容在分配区域中处理。

## 交互和功能边界

弹窗继续使用原生 `dialog`。取消、关闭及错误反馈保留既有焦点处理；提交中禁止重复提交和关闭。账号版本变化、能力不足、待核对的安全操作、代理或登录资料错误仍由原有检查阻止提交。步骤预览避免在每次普通刷新时重复重建 `aria-live` 内容，仅在当前操作或选项组合变化时更新反馈。

勾选只编辑本次操作选项，不执行安全动作；只有明确提交确认才发起任务。此轮只修改 `plugin/ui/index.html`、`style.css`、`login.js`，不修改后端安全操作、工作区选择、套餐排除、CPA 凭证写入或宿主。已确认的新密钥保存、未知结果隔离和禁止重放的规则继续生效。

## 验证与部署

证据目录为 `artifacts/ui-rework-11.8.0/`。[布局与交互测量](../artifacts/ui-rework-11.8.0/layout-verification.json) 使用合成账号，没有提交真实安全操作、登录或模型请求。

| 视口 / 操作 | 已观察结果 |
| --- | --- |
| 1064 × 800，重置 2FA | 四种选项组合均为 520 × 584px，外框位置与底部按钮区不变；三行预览完整显示，无正文溢出。 |
| 390 × 844、320 × 640，重置 2FA | 切换选项不改变窗口宽高；正文在内部纵向滚动，无横向溢出。 |
| 800 × 480，重置 2FA | 窗口收敛为 520 × 432px，顶部距视口 24px；正文滚动时底部按钮仍可见。 |
| 桌面，退出所有设备 | 两种授权选项组合均为 520 × 482px，固定两行预览。 |

桌面测量中，重置窗口的左上位置固定为 x=264.5、y=108，底部按钮区 y=626、高 65px。取消与 Escape 都将焦点还给对应来源按钮；重新打开恢复默认值：退出设备不勾选、重新授权勾选。浅色、深色和纯白主题均已目测检查。

截图：[桌面全选](../artifacts/ui-rework-11.8.0/desktop-all-selected.jpg)、[390px 手机](../artifacts/ui-rework-11.8.0/reset-390x844.jpg)、[320px 手机](../artifacts/ui-rework-11.8.0/reset-320x640.jpg)、[短窗口](../artifacts/ui-rework-11.8.0/reset-800x480.jpg)、[深色重置](../artifacts/ui-rework-11.8.0/reset-dark.jpg)、[纯白退出设备](../artifacts/ui-rework-11.8.0/logout-white.jpg)。

账号版本变化的错误场景已验证：只滚动弹窗正文使错误完整可见，底部保留约 11.81px；背景页面滚动位置前后均为 192px，弹窗尺寸不变。提交和选项同时禁用，取消仍可操作。

[模拟提交](../artifacts/ui-rework-11.8.0/fixture-submission-verification.json) 记录重置 2FA 的 `logout_all_devices=true / reauthorize=true` 与退出设备的 `false / false` 两条准确请求，共 2 次安全操作 POST，真实安全操作为 0。`scripts/build_ui.sh` 和 Node 客户端/导入 16 项回归通过，见 [最终验证汇总](../artifacts/ui-rework-11.8.0/verification-summary.json)。

[安装包核验](../artifacts/ui-rework-11.8.0/package-verification.json) 确认 Linux arm64 原生构建、登录运行时测试、插件内嵌模式测试和 `go vet` 通过，包及内嵌运行时校验值、六份 UI 源摘要匹配。[宿主基线](../artifacts/ui-rework-11.8.0/source-baseline-verification.json) 确认原始 CPA 和构建快照各 2,413 个记录文件摘要一致。[部署记录](../artifacts/ui-rework-11.8.0/deployment.json)、[首次启动](../artifacts/ui-rework-11.8.0/verification-before-restart.json) 和 [再次重启核验](../artifacts/ui-rework-11.8.0/verification-after-restart.json) 确认 UI 11.8.0 已载入，插件已注册并启用，登录运行时可用，六份资源摘要与源码一致，未鉴权管理接口返回 401。安装包核验是在部署前写入，实际部署状态以这些记录为准。

升级前已停服备份；升级后保留 1 个插件账号和 1 条既有终态任务，模型运行、规则、暂停归属与 CPA 凭证均为 0。账号、任务及设置逻辑快照完整相等，没有默认值例外，配置/凭证文件与宿主镜像均未变化。

[实际 CPA 测量](../artifacts/ui-rework-11.8.0/deployed-layout-verification.json) 再次确认四种重置组合均为 520×584px、x=264.5/y=68，固定三行预览，计数分别随选择显示 1–3 项。检查后取消弹窗，真实安全提交为 0。见 [最终实际页面截图](../artifacts/ui-rework-11.8.0/deployed-dialog.jpg)。

安装包：[Linux arm64 ZIP](../artifacts/ui-rework-11.8.0/plugin-package/model-quality-v0.17.1-linux-arm64.zip)。由于后端版本仍为 0.17.1，应同时核对 [构建清单](../artifacts/ui-rework-11.8.0/plugin-package/build-manifest.json) 的 `ui_version=11.8.0` 和 [SHA256SUMS](../artifacts/ui-rework-11.8.0/plugin-package/SHA256SUMS)，避免误装同名的旧 UI 安装包。
