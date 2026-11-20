# 0.29.30 质量处置与历史清理审查

本地候选为插件 0.29.30、UI 13.6.29，适用于官方 CPA 8.0.76、Linux arm64。对照版本为上一轮已验收的 0.29.29。测试使用合成凭证和隔离宿主，未部署到生产实例。[候选安装包](../artifacts/quality-audit-0.29.30/plugin-package/model-quality-v0.29.30-linux-arm64.zip) · [构建清单](../artifacts/quality-audit-0.29.30/plugin-package/build-manifest.json)

## 第一轮 · 判题与凭证写入

用两模型、每模型两份回答运行停用规则。前三份答对，第四份的过程消息为 `5`、最终回答为 `4`，标准答案为 `4`。旧包把两段拼为 `54`，隔离 CPA 实际收到一次停用 PATCH。候选包记录最终回答 `4`，四份全对，凭证保持启用，写入为零。只有过程消息、多个无法区分的回答、或明确标记为用户消息的文本均保留为未知。Chat Completions 的流式和非流式读取也会拒绝明确的用户或工具角色。[旧包写入日志](../artifacts/quality-audit-0.29.30/policy-before.log) · [候选逐轮结果](../artifacts/quality-audit-0.29.30/policy.json) · [角色回归](../artifacts/quality-audit-0.29.30/answer-after.log)

旧上游只有一条消息、没有声明阶段时仍可读取。官方样例中也存在终态含最终文本、消息项缺少完成状态的格式。管理员现在能看到最终文本及缺失状态的原因，规则判定为未知。停用与移除两条路径均验证了这种响应，写入为零。解析逻辑位于 [Responses](../plugin/quality/management_client.go) 和 [Chat Completions](../plugin/quality/collector.go)。

## 第二轮 · 保留策略与重启

| 旧版复现 | 候选结果 |
| --- | --- |
| 旧按天规则存在历史独立暂停或受其联动阻断，运行和相关审计仍被清理。运行记录已缺失时，待核对处置的剩余审计也可能被删 | 按运行及处置标识保护证据；关闭数据库再打开，受保护记录仍可读，普通到期记录照常清理。进入持久化故障状态后停做清理 |
| 主测中途耗尽预算，四份对照已完成。下一轮按条数裁剪旧主测时，只剩一份对照 | 保留三份有用对照，其中两份对应尚未执行的主测。裁掉的主测及其独有对照正常淘汰 |
| 12 个已结算过期预算日分布在相邻账本键上，一次清理漏掉 6 个 | 一次清完 12 个，保留当日和未结算日期。这项修复针对旧账本行的保留数量 |

每小时的历史清理、全局默认 30 天和新规则默认每凭证 100 条沿用原配置。预算账本另在预留及结算路径整理。对照留存在 14 次合成调用的两轮执行中验证；存储另测了 400 组跨页键和事务尾部故障回滚。[旧版存储记录](../artifacts/quality-audit-0.29.30/storage/before.log) · [候选回归](../artifacts/quality-audit-0.29.30/storage/after.log) · [预算回归](../artifacts/quality-audit-0.29.30/storage/budget-after.log) · [对照裁剪复现](../artifacts/quality-audit-0.29.30/retention/control-before.log) · [修复后](../artifacts/quality-audit-0.29.30/retention/control-after.log)

在同一份 2,752 轮、24,000 个用量事件的合成库上，例行清理中位数为 28.809 → 28.922 毫秒，清理期间的提交等锁中位数为 27.026 → 25.730 毫秒。例行清理的 P95 为 32.368 → 51.080 毫秒；一次全量过期清理为 32.125 → 35.880 毫秒，两版均清空所有应过期的行。此次修改优先解决证据丢失；高峰延迟仍需在更长负载下复测。[旧版计时](../artifacts/quality-audit-0.29.30/storage/history-before.log) · [候选计时](../artifacts/quality-audit-0.29.30/storage/history-after.log)

## 第三轮 · 管理员操作与发布复验

全局清空已在服务端完成、浏览器丢失回执时，旧确认框还允许再次提交，第二次 DELETE 会删去两次点击之间刚结束的运行。删除规则后遇到局部读取失败、或删除一条手动记录后丢失回执，也能复现重复请求。候选版对一次确认只发送一次删除；随后重新读取，明确显示回执是否确认。全局清空的确认框显示当前筛选与实际影响范围。候选测试保住了稍后才结束的运行、处置证据、手动记录，以及另一条正在打开的历史详情。[旧版六个场景](../artifacts/quality-audit-0.29.30/ui/cleanup/before/cleanup-receipts.json) · [候选结果](../artifacts/quality-audit-0.29.30/ui/cleanup/after/cleanup-receipts.json) · [旧确认框](../artifacts/quality-audit-0.29.30/ui/cleanup/before/clear-confirm.png) · [新确认框](../artifacts/quality-audit-0.29.30/ui/cleanup/after/clear-confirm.png) · [390 像素删除反馈](../artifacts/quality-audit-0.29.30/ui/cleanup/after/rule-receipt.png)

最终 ZIP 在隔离 CPA 上完成 34 次规则请求、120 次合成模型调用。19 轮证据不足的凭证写入均为零，3 次停用、2 次恢复、2 次移除各写一次。扣住一次已执行停用的回执再强制终止，连续两次启动后仍为“待核对”，累计 PATCH 维持一次。完整插件测试、质量模块竞态检查及 vet 均通过。浏览器在离线官方管理页加载从隔离 CPA 取出的候选资源，覆盖桌面、短屏、390 和 320 像素视口。[模块测试](../artifacts/quality-audit-0.29.30/plugin-tests.log) · [竞态测试](../artifacts/quality-audit-0.29.30/quality-race.log) · [原生装载与重启](../artifacts/quality-audit-0.29.30/native-smoke.json) · [中断复验](../artifacts/quality-audit-0.29.30/native-intent.json) · [嵌入视口](../artifacts/quality-audit-0.29.30/ui/iframe/candidate-iframe.json)

手动测试的迟到回执、重连、取消与导出复验通过。账号兼容脚本改从当前的“更多 → 账号资料”进入编辑页，保留原有断言，三个屏宽下均通过。合成夹具只收到一次确认删除。[手动回执](../artifacts/quality-audit-0.29.30/manual-receipts.log) · [账号兼容测试](../artifacts/quality-audit-0.29.30/ui/account-regression/account-delete.json)

官方 CPA 状态写入和删除仍缺少原子条件检查。写前最后一次校验到实际写入之间存在并发窗口，结果不确定时保留待核对记录。已经被旧版本清掉的证据无法通过升级找回。
