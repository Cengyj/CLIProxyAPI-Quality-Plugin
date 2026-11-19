# 工作区自动选择：插件 0.17.1 / 运行时 2.1.2

已于 **2026-11-17 17:39:38（Asia/Shanghai）** 部署到 [8317 插件页面](http://127.0.0.1:8317/management.html#/plugin-pages/model-quality/0)：**插件 0.17.1 / Go＋Sobek 2.1.2 / UI 11.7.0**。CPA 8.0.76 宿主和六份界面源文件均保持不变。

## 原因与修复

此前 16:14 的登录任务停在 `workspace_selection`。旧版在未绑定目标工作区时，只接受唯一有效工作区；遇到多个工作区就返回需要选择。该历史结果并不包含上游返回的完整工作区列表，因此不能仅凭错误码断言当时具体有几个工作区。

本轮按用户指定的只读参考项目 `/Users/ceng/Downloads/toSub2-go-port-2026.11.17/internal/protocol/flow.go` 中 `workspace` 方法（约第 787 行）对齐默认策略：

1. 已明确指定 `WorkspaceID`：只能选择 ID 完全匹配的工作区。目标缺失就停止，不转选其他组织或个人工作区。
2. 未指定目标：按上游列表顺序选择第一个 ID 有效且 `kind=organization` 的工作区。
3. 没有组织工作区：选择列表中第一个 ID 有效的工作区。
4. 没有有效 ID：保留 `interaction_required / workspace / workspace_selection`，不提交选择。

该规则同时用于 TOTP 后的网页登录续步与后续 Codex OAuth。运行时识别工作区页面类型、`/workspace` 续步路径及工作区列表；缺少续步地址时使用当前阶段的已知认证页面作为上下文。已收到可信最终回调时优先处理回调，不因残留的工作区列表重复提交。工作区续步有次数上限，服务器重复返回同一选择状态时停止，不重放已完成的密码或 TOTP 验证。

## 绑定与安全边界

已有 CPA 绑定重新授权时，插件重新读取目标凭证并核对身份，固定其 `account_id` 作为工作区。目标不可读、缺失或被替换时停止，不能以首次登录的组织优先策略绕过绑定。OAuth 结果仍需核对邮箱、工作区及既有身份快照。

重置 2FA 或退出设备后重新授权时，先前安全步骤已验证的账号工作区会被带入新的网页登录及 OAuth；插件也会核对最终候选仍属于该工作区。已确认的新 2FA 密钥继续保存，未知安全变更继续隔离。此修复没有授权自动重放安全操作。

自动选择只推进当前已发起的登录任务。现有套餐上传排除、人工禁用保护、历史质量暂停、自动修复全局/账号许可和不确定写入保护继续生效。它不发起模型请求，也不代表已取得授权或已写入 CPA。

## 历史任务与使用方式

升级保留此前 16:14 任务的原状态和日志，不把失败改写成成功，也不自动重试。用户要验证新流程，需在账号列表手动发起一次新的 2FA 登录；新任务才会使用本轮工作区选择逻辑。本轮开发和部署不执行真实账号登录。

## 验证与部署

[验证汇总](../artifacts/workspace-selection-0.17.1/verification-summary.json) 记录以下通过结果：

- Web 与 OAuth 两阶段共 14 个离线工作区场景，覆盖组织优先、首个有效 ID、精确绑定、缺失绑定、无效列表、缺失续步上下文，以及旧列表与最终回调的优先关系；另核对重复响应上限与安全操作后固定工作区。
- 运行时全模块 race 测试及 `go vet` 通过，见 [运行时测试](../artifacts/workspace-selection-0.17.1/runtime-test.log) 和 [vet 记录](../artifacts/workspace-selection-0.17.1/runtime-vet.log)。插件登录模块 race 与新运行时版本能力兼容检查通过。
- [原生构建](../artifacts/workspace-selection-0.17.1/native-build.log) 完成登录运行时和插件全模块测试、插件 `go vet`、内嵌运行时自检及 Linux arm64 打包。2.1.2 的重置 2FA / 退出设备能力仍可识别；验证没有执行这些真实动作。
- [源码基线](../artifacts/workspace-selection-0.17.1/source-baseline-verification.json) 确认原始 CPA 与构建快照各 2,413 个记录文件摘要一致。

安装包：[model-quality-v0.17.1-linux-arm64.zip](../artifacts/workspace-selection-0.17.1/plugin-package/model-quality-v0.17.1-linux-arm64.zip)。核对 [构建清单](../artifacts/workspace-selection-0.17.1/plugin-package/build-manifest.json) 和 [SHA256SUMS](../artifacts/workspace-selection-0.17.1/plugin-package/SHA256SUMS)。

[部署记录](../artifacts/workspace-selection-0.17.1/deployment.json) 保存停服备份及最终版本；[首次启动](../artifacts/workspace-selection-0.17.1/verification-before-restart.json) 和 [再次重启](../artifacts/workspace-selection-0.17.1/verification-after-restart.json) 均确认插件已注册、启用、运行时可用，六份静态资源摘要正确，未鉴权管理请求返回 401。

升级前后 **1 个账号、1 条既有终态任务**及所有设置的逻辑快照完全相同，无新增默认字段例外。模型运行、规则、暂停归属与 CPA 凭证均为 0；配置/凭证文件、宿主镜像均未改动。真实登录尝试与模型调用均为 0，历史任务未重放。UI 源码未变，本轮不重复 UI 浏览器验证；布局证据继续使用 [UI 11.7.0 报告](UI_REWORK_11_7_0_CN.md)。
