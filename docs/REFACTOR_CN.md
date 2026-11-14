# 0.3.0 架构与代码质量改进

本轮保留“质量暂停后禁止全部复测，仅人工解除”的产品语义，集中修复执行、公平性、生命周期、查询和前端状态的问题。没有更换 CPA 宿主基线，也没有引入另一套供应商执行器或前端框架。

## 从实际缺陷调整职责

| 原问题 | 现在的行为 | 主要代码 |
|---|---|---|
| worker 取任务后等待凭证锁，相同凭证的慢请求可耗尽全部 worker | 调度器先同时占用所有依赖；忙凭证任务留在有界队列，独立凭证仍可执行 | scheduler.go |
| 取消/阻断的队列条目占用容量 | 入队前清理终态条目；不重发网络调用 | admission.go、scheduler.go |
| 最后一次模型调用完成后到处置前，凭证状态或 judge 身份可能已经变化 | 最终判定重新验证全部依赖；连续失败签名绑定主体、对照和 judge 身份 | rules.go、transitions.go |
| 重配/关闭与正在使用旧 Engine 的管理请求竞争 | 原生实例租约阻止新请求进入旧实例，等待已有请求退出后关闭存储；重复 Close 等待同一结果 | cmd/model-quality/lifecycle.go、engine.go |
| 快照复制反复编码和解析完整模型输出，错误被丢弃 | 明确复制可变切片和指针，保留不可变字符串；请求页之外不复制历史输出 | snapshots.go、queries.go |
| 审计/指标读取失败仍返回空正常数据；错误按消息子串猜测 | 数据读取失败返回503，错误按明确服务代码分类 | queries.go、metrics.go、errors.go |
| 已取消调用仍可能打开 operation，异常流 ID 未统一关闭 | 调用前检查上下文，幂等释放 operation/stream；取消与完成竞态返回取消 | host.go |
| 午夜前开始的请求完成后记入新日预算 | 以准入 UTC 日绑定账本，保留在途日期，token累加防溢出 | budget.go |
| 解析器接受结束后的文本或无效 token 用量 | 完成后的追加答案被拒绝；负数、小数、不完整 usage 显示未知 | collector.go |
| 页面断连保留管理内容，旧响应覆盖新连接状态 | 会话代际、AbortController、重复操作保护和独立轮询 | ui/client.js、ui/app.js |

## 文件边界

Engine 保管生命周期与共享状态；admission 负责校验、冻结、持久化入队；scheduler 负责依赖准入与时钟调度；execution 负责调用与样本完成；rules/transitions/reconcile 负责质量处置、事务状态与权威对账；queries/metrics 只负责读模型。host 与 collector 分别处理原生回调和模型输出解析。

原生入口将运行时替换、RPC 封包与业务分发分开；ManagementRoutes 集中声明鉴权路由，静态资源注册与精确文件查找共用资源清单。

不把这些边界做成通用策略框架，也不增加分布式组件。持久化继续使用 bbolt，宿主暂停权威库与插件业务库仍分别拥有各自状态，不声称跨库原子事务。

## UI 风格参考

只读 `/Users/ceng/Downloads/CLIProxyAPI-8.0.67` 归档没有管理前端 HTML/CSS/JS 源码。其 `internal/managementasset/updater.go` 和 `config.example.yaml` 指向 `router-for-me/Cli-Proxy-API-Management-Center`。本轮依据这个真实来源，记录前端参考提交 `d0126752201a35ff999bd2a1d4741215542258a8`；这只是 UI 风格参考，不是更换宿主版本，也不表示该归档自带此提交。

采用其暖灰纸色、侧栏层级、紧凑表格、卡片与输入控件、明暗主题和低干扰操作按钮。两页分别呈现手动测试与质量运维，保留明确的暂停范围和人工释放说明。具体交互及截图见 UI_CN.md 和 TEST_REPORT_CN.md。

## 验证边界

测试仍以本地 fake upstream 为主，包含真实 c-shared 装载与 standalone 浏览器。首包、流中和非流式取消使用明确上下文取消，不添加推理网络超时。Linux/Windows 等未实测平台、Home 不支持及身份重建限制没有因重构而改变。
