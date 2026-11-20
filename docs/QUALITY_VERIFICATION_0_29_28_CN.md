# 智商测试与质量运维 · 0.29.28 本地候选

本轮从工作区提交 `9591c3b`、插件 0.29.27、UI 13.6.26 开始。候选使用 UI 13.6.27、内嵌 Go＋Sobek 2.2.1，目标平台为 Linux arm64。核对了只读 CPA 8.0.76 清单中的 2,413 个文件。所有下述运行使用临时库、合成凭证和隔离上游。

## 复核异常记录

| 管理员操作 | 原版本实测 | 候选版本实测 |
| --- | --- | --- |
| 手动或规则编辑器输入 5,462 个“题” | 可提交 16,386 字节，合法的长 ASCII 输入还会被浏览器截短 | 题旁提示至少删减 2 字节，超限提交 0 次。16,383 和 16,384 字节原样提交。规则自定义评审标准也按 4,096 字节校验 |
| 点击开始后回执丢失，再点开始、断线重连 | 2 个提交标识，入队 2 次 | 先在已有鉴权详情接口按原提交标识查询，入队 1 次。请求未抵达时，显式重试沿用同一标识 |
| 打开旧手动记录，再打开、关闭并重开质量详情 | 晚到的手动回执可以改变正在阅读的记录 | 正在执行的手动任务继续被跟踪，取消、导出与关闭后焦点都指向对应记录；导出含当轮题面和完整回答 |
| 管理探测的终态消息把数值 `5` 放进文本字段 | 旧解析器把数字当作回答，向 CPA 发出一次停用 | 两种处置策略均记录证据不足，写入 0 次。已停用凭证复检后仍未全对时，记录显示“保持停用” |
| 清空记录、按数量裁剪或按天过期 | 三条路径都可能移除待核对写入指向的运行 | 原题、回答与处置日志均可从库中重读。已确认停用的普通历史仍可清理，恢复使用单独保存的快照 |

实现分别在 `plugin/ui/app.js` 与 `index.html` 的表单、任务跟踪和分页读取，`plugin/quality/admission.go` 与 `api.go` 的回执查找，`management_client.go` 与 `credential_policy.go` 的答题证据和处置结果，以及 `result_limit.go` 与 `reconcile.go` 的历史清理。

题面边界、阅读位置与回执丢失均使用相同合成数据做前后对照。[手动题面修改前](../artifacts/quality-optimization-0.29.28/ui/prompt-before/prompt-320.png) · [修改后](../artifacts/quality-optimization-0.29.28/ui/prompt-after/prompt-320.png) · [规则题面修改前](../artifacts/quality-optimization-0.29.28/ui/prompt-before/rule-prompt-1440.png) · [修改后](../artifacts/quality-optimization-0.29.28/ui/prompt-after/rule-prompt-1440.png) · [回执修改前](../artifacts/quality-optimization-0.29.28/ui/receipts-before/lost-receipt.png) · [修改后](../artifacts/quality-optimization-0.29.28/ui/receipts-after/lost-receipt.png)

## 历史读取与质量统计

每种来源各有 41 条记录，每条有一份 70,000 字节的长回答。下表为未压缩的管理接口响应体字节数。列表调用显式使用 `GET /runs?view=summary`；详细内容从现有 `GET /runs/detail` 读取。默认列表仍提供完整字段。[基线浏览器请求记录](../artifacts/quality-optimization-0.29.28/ui/history-reads-before.json) · [候选浏览器请求记录](../artifacts/quality-optimization-0.29.28/ui/history-reads-after.json)

| 路径 | 修改前 | 修改后 | 候选请求数 |
| --- | ---: | ---: | --- |
| 首次连接或手动刷新，两类历史第一页合计 | 2,138,218 | 16,978 | 2 次列表，0 次详情 |
| 手动历史第二页，10 条 | 713,035 | 5,955 | 1 次列表 |
| 规则历史第二页，20 条 | 1,425,185 | 11,025 | 1 次列表 |
| 点开一条记录 | 含全文 | 含全文 | 1 次详情 |

顶部概况汇总两个来源，规则和批次筛选作用于右侧记录。混合测试库中有 1 个正确、1 个答错、2 个未知，扣除裁剪后共计划 5 个；通过率为 1/2，覆盖率为 2/5。手动显示核对出的“正确”只用于阅读。此回归在 `plugin/quality/queries_test.go` 中同时验证统计和显示结果。

## CPA 写入与恢复

官方 CPA 8.0.76 在无外部网络的容器中装载候选。每轮针对一份合成凭证、两个模型、每模型两份样本。24 次规则请求共触发 80 次合成调用。逐轮记录包括 CPA 实际收到的 PATCH/DELETE、文件是否存在、最终停用位，以及各次探测时的停用位。[旧版误停用复现](../artifacts/quality-optimization-0.29.28/credential-policy-before.json) · [候选逐轮结果](../artifacts/quality-optimization-0.29.28/credential-policy.json)

| 场景 | 实际写入 |
| --- | --- |
| 三对一错，所有样本完整 | 仅检测 0 次，停用 1 次 PATCH，移除 1 次单文件 DELETE |
| 两对一错，另一份为上游 504、拒答、截断、未知或畸形内容 | 两种处置策略共 10 轮，每轮 0 次 |
| 规则拥有的停用凭证，或获准参与的人工停用凭证 | 复检期间始终停用。部分正确写入 0 次；四份全对且快照匹配后，各 1 次启用 PATCH |
| 重启后重新读取 | 新模型调用和处置写入均为 0 |

另在 Go 引擎隔离测试中验证了缺样本、重复样本、取消、另一规则的停用归属、最后一个启用凭证停用后的定时恢复及写入待核对后的重启。CPA 标准管理接口缺少原子条件写入，外部管理员仍可在最终快照检查与写入之间改变凭证。

## 打包与复验

完整插件模块测试、质量引擎竞态测试、内嵌构建的测试与 vet 均通过。官方宿主编译与常规测试使用相同清单的 BuildKit 缓存，WebRTC 测试在隔离内部网络重新执行。单库 ZIP 在独立 CPA 中验证了鉴权、六份嵌入页面资源、加载及重启。账号页用合成数据复验了列表、筛选、资料、历史和设置弹窗。[构建清单](../artifacts/quality-optimization-0.29.28/plugin-package/build-manifest.json) · [原生装载与重启](../artifacts/quality-optimization-0.29.28/native-smoke.json) · [模块测试](../artifacts/quality-optimization-0.29.28/plugin-tests.log) · [质量竞态测试](../artifacts/quality-optimization-0.29.28/quality-race.log)

官方管理面板的离线静态页面嵌入了从候选 CPA 取得的六份资源。1440、1280 短屏、390 和 320 像素视口均实际打开并关闭了规则编辑器和检测详情，验证了草稿、列表滚动和焦点恢复。详情正文可滚动的实测距离分别为 678、938、774、1,057 像素。[嵌入视口记录](../artifacts/quality-optimization-0.29.28/ui/iframe/candidate-iframe.json) · [桌面截图](../artifacts/quality-optimization-0.29.28/ui/iframe/quality-1440.png) · [手机截图](../artifacts/quality-optimization-0.29.28/ui/iframe/manual-320.png)

下次值得测量历史库接近配置容量时的请求延迟与引擎锁等待。当前验证量化了响应字节数和请求次数，可作为进一步测量的基线。
