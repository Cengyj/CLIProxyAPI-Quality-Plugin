# 智商测试与质量运维 · 0.29.29 本地候选

本轮在上一轮未提交的 0.29.28 / UI 13.6.27 工作区上继续。交付物为 Linux arm64 单库 ZIP，UI 版本为 13.6.28。构建时逐文件核对了官方 CPA 8.0.76 的 2,413 项宿主基线。浏览器、模型调用和凭证写入均使用合成数据。[单库包](../artifacts/quality-optimization-0.29.29/plugin-package/model-quality-v0.29.29-linux-arm64.zip) · [构建清单](../artifacts/quality-optimization-0.29.29/plugin-package/build-manifest.json)

## 管理员可复核的变化

| 用户路径 | 旧版复现 | 候选行为 |
| --- | --- | --- |
| 长期规则的一轮留有重复样本，展开或复制回答 | 三条记录只能看到两条，导出与页面数量不同 | 三条均可复核。缺失、重复、历史裁剪能分别展示，新裁剪保存模型和样本序号，旧裁剪的位置可标为未知 |
| 给第二条同序号回答写评审，后台恰好裁掉另一条记录 | 草稿可移到相邻回答，点击保存会评错对象 | 草稿和保存请求始终绑定同一条原始记录。缺省或空的评审位置会被拒绝，显式传入位置 0 仍有效 |
| 提交评审后切到另一条历史，再收到旧回执 | 迟到的回执可使刚打开的详情失效 | 保留阅读选择，重查、复制和导出仍指向当前记录 |
| 已登录时单个资源返回 503 | 指标或历史失败可清空连接，旧值可能继续显示为最新，翻页失败可能错标页码 | 单独显示失败资源；保留可用数据，旧统计标明上次读取，失败翻页能原页重试。凭证目录失效立即拦截依赖它的测试，401 仍会清空会话 |
| CPA 已停用凭证，插件尚未收到成功回执时进程中断 | 凭证文件已停用，重启后详情仍显示“正在暂停凭证”并持续轮询 | 显示“停用结果待核对”，保留原凭证快照、四份回答及本地处置归属，停止轮询。移除和恢复意图经过相同的启动处理 |

回答与评审的实现涉及 `quality/result_limit.go`、`quality/api.go`、`ui/view.js` 和 `ui/app.js`；局部刷新由 `ui/app.js` 分资源维护；中断意图在 `quality/engine.go` 加载期间由 `quality/credential_policy.go` 完成本地状态转换。CPA 最终快照检查与实际写入之间仍存在并发窗口。

同一组合成场景的截图和交互记录如下。

- 样本位置和评审草稿。[裁剪前版](../artifacts/quality-optimization-0.29.29/ui/evidence/before/known-pruning-1440.png) · [修复后](../artifacts/quality-optimization-0.29.29/ui/evidence/after/known-pruning-1440.png) · [按钮请求与导出核对](../artifacts/quality-optimization-0.29.29/ui/evidence/after/sample-evidence.json)
- 局部读取错误。[统计错误修改前](../artifacts/quality-optimization-0.29.29/ui/reads/before/metrics.png) · [修改后](../artifacts/quality-optimization-0.29.29/ui/reads/after/metrics.png) · [六组交互](../artifacts/quality-optimization-0.29.29/ui/reads/after/reads.json)
- 中断写入。[详情修改前](../artifacts/quality-optimization-0.29.29/ui/interrupted/before.png) · [修改后](../artifacts/quality-optimization-0.29.29/ui/interrupted/after.png) · [原包复现](../artifacts/quality-optimization-0.29.29/native-intent-before.log) · [两次启动后的实际 PATCH 数与文件状态](../artifacts/quality-optimization-0.29.29/native-intent.json)

## 统计与历史清理

通过率继续使用已保存的正确、错误判定数。覆盖率按冻结计划中的模型和序号去重，重复回答不会多覆盖一个位置。已知裁剪位置只在相应位置没有留存记录时扣减。旧记录仅保留裁剪总数时，界面标明覆盖率下限与位置未知数。人工阅读时生成的匹配结果仍独立于质量统计。质量页使用 `GET /metrics?view=summary`，旧调用方可以继续读取完整 `/metrics`。

性能对照在 macOS arm64 上运行，使用 Go 1.27.1，设置 `GOMAXPROCS=4`。两个版本读取同一份 bbolt 数据库，包含 2,752 轮、11,008 个样本和 24,000 个用量事件。下表为单机实测中位数，读取各 21 次，清理与提交重叠各 11 次。[基线原始计时](../artifacts/quality-optimization-0.29.29/history/baseline-comparison.log) · [候选原始计时](../artifacts/quality-optimization-0.29.29/history/after.log) · [复现脚本](../artifacts/quality-optimization-0.29.29/history/reproduce.sh)

| 操作 | 0.29.28 | 0.29.29 |
| --- | ---: | ---: |
| 质量页所需概况 | 14.197 ms | 0.536 ms |
| 例行清理 | 141.016 ms | 21.004 ms |
| 清理期间提交等待引擎锁 | 142.859 ms | 20.998 ms |
| 清理期间完成提交 | 156.845 ms | 26.950 ms |
| 完整指标接口 | 14.034 ms | 15.591 ms |
| 概况读取期间提交等待引擎锁 | 0.707 ms | 3.590 ms |

清理复用已提交运行的完成时间，避免对每条长回答重新反序列化。旧记录完成时间为零时，内存与持久化存储都保留该记录。概况摘要省去了用量事件的完整扫描；按位置去重仍需要在锁内检查样本。下一轮值得验证常规、无重复样本能否走更快的统计路径，并同步测量重复与裁剪场景。

## 验证边界

Go 完整插件模块测试、质量引擎竞态测试、构建期间的 vet 和原生装载均通过。测试在隔离 CPA 已执行停用 PATCH 后扣住成功回执，强制终止进程。连续两次启动，累计 PATCH 保持 1 次，文件始终停用，原有四份回答均可读。常规处置矩阵另行执行了 24 次规则请求和 80 次合成模型调用。最终源码离线重建得到的 `model-quality.so` 与上述三项原生验收使用的 ZIP 内库逐字节一致。[模块测试](../artifacts/quality-optimization-0.29.29/module.log) · [竞态测试](../artifacts/quality-optimization-0.29.29/history/quality-race.log) · [处置矩阵](../artifacts/quality-optimization-0.29.29/credential-policy.json) · [原生资源及重启](../artifacts/quality-optimization-0.29.29/native-smoke.json)

浏览器通过离线官方管理页的 iframe 加载从隔离 CPA 提取的候选资源，接口返回合成数据。桌面、短屏、390 和 320 像素四种视口均核对了操作、焦点和滚动。本轮只在本地验证，未部署到生产实例。[嵌入视口](../artifacts/quality-optimization-0.29.29/ui/iframe/candidate-iframe.json)
