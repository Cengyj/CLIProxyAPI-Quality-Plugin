# UI 4.1.0 商用品质精修交付记录

日期：2026-11-14（Asia/Shanghai）。本轮使用 impeccable 的 Operate / polish 流程，以 UI 4.0.0 为基线；插件后端保持 0.3.0，quality capability v2 / schema 2 不变。仅本地离线假凭证和 fake upstream，无真实模型账户调用，无外部发布或正式部署替换。

## 基线复核与优先级

本轮开始根仓库实际为干净状态，HEAD `8ff82b2f9e8b3fac698c92832ff48f585e411038`；提示词中“大量文件未跟踪”已不再成立。记录以 [baseline.json](../artifacts/polish-4.1.0/baseline/baseline.json) 的原始状态和逐文件 SHA-256 为准，没有为本轮额外提交基线。

优先处理误导状态和交互稳定性，再处理阅读密度与组件，最后封存发布证据：

| 复核问题 | 实际发现与修改 | 验收证据 |
| --- | --- | --- |
| 消息跨区残留 | 原解除消息出现在比较页；普通成功消息六秒消退，切换工作区清理，迟到反馈核对发起分区，复制/评审反馈靠近按钮，进度读取失败留在任务内 | UI 回归与前后截图 |
| 轮询重建回答 | 原评审备注/折叠/筛选已保持，但焦点、输入文字选择、回答展开及内部滚动丢失；改为保留稳定节点更新变化内容 | baseline/polling/polling-probe.json 与 verification/polling-probe.json |
| 编辑时序 | 预算迟到读取可能覆盖后打开的草稿；加读取序号隔离；保存中锁定字段与关闭，失败保留输入并在解锁后聚焦错误字段，真实未保存关闭才确认 | 延迟 API 与保存锁定回归 |
| 首屏层级过多 | 当前任务直接使用“回答与证据”标题；精简摘要，任务 ID 进入冻结条件，手机次要操作折叠，比较控件紧凑同排 | 相同 fixture/主题/视口对照 |
| 回答阅读 | 短答不再保留150px空白；自然语言16px无衬线，代码13px等宽；完整模型 ID 可复制；执行/质量/证据分别标示，诊断信息折叠 | 比较、原生短答、长回答手机截图 |
| 分散交互 | 明确三主题选择；产品内预设命名、规则删除、放弃编辑对话框；字段错误与焦点关联 | Escape、焦点归还、单次提交等回归 |
| 安全预览空白 | 开发实测发现隐藏 dialog 中启动 srcdoc 导航产生零布局；改为显示对话框后设置 srcdoc | 原有脚本、网络、表单、导航、inert、CSP 断言全部保留通过 |
| 发布不一致 | 旧归档仍包含UI3.0.0；旧验证哈希不匹配当前库，旧manifest为38项；独立候选改用真实命令、源码/库/报告/截图哈希门槛 | 新候选清单、验证记录、解包冒烟 |

## 同条件前后截图

[桌面对照总览](../artifacts/polish-4.1.0/comparison-desktop.png) · [手机对照总览](../artifacts/polish-4.1.0/comparison-mobile.png)。总览仅将下表原始截图等比例并排，PNG元数据记录输入SHA；原尺寸证据仍在下表。

数据来自本机 fake 管理 API，固定 `comparison-run`、两模型、样本2、一侧证据不足、一侧明确错误，时区 Asia/Shanghai。截图在页面顶部捕获，主题、视口与数据相同；动态“更新于”时间不同。实际视图导航也通过键盘/点击流程验证，未只依赖全页截图。

| 视口 / 状态 | UI 4.0.0 | UI 4.1.0 |
| --- | --- | --- |
| 1440×900 浅色比较 | [之前](../artifacts/polish-4.1.0/baseline/paired-clean/paired-comparison-desktop-light.png) | [之后](../artifacts/polish-4.1.0/verification-final2/paired-comparison-desktop-light.png) |
| 1440×900 深色比较 | [之前](../artifacts/polish-4.1.0/baseline/paired-clean/paired-comparison-desktop-dark.png) | [之后](../artifacts/polish-4.1.0/verification-final2/paired-comparison-desktop-dark.png) |
| 1440×900 纯白比较 | [之前](../artifacts/polish-4.1.0/baseline/paired-clean/paired-comparison-desktop-white.png) | [之后](../artifacts/polish-4.1.0/verification-final2/paired-comparison-desktop-white.png) |
| 390×844 浅色比较 | [之前](../artifacts/polish-4.1.0/baseline/paired-clean/paired-comparison-mobile-light.png) | [之后](../artifacts/polish-4.1.0/verification-final2/paired-comparison-mobile-light.png) |
| 1440×900 配置 | [之前](../artifacts/polish-4.1.0/baseline/paired-clean/paired-setup-desktop.png) | [之后](../artifacts/polish-4.1.0/verification-final2/paired-setup-desktop.png) |
| 390×844 配置 | [之前](../artifacts/polish-4.1.0/baseline/paired-clean/paired-setup-mobile.png) | [之后](../artifacts/polish-4.1.0/verification-final2/paired-setup-mobile.png) |
| 1440×900 暂停运维 | [之前](../artifacts/polish-4.1.0/baseline/paired-clean/paired-operations-paused-desktop.png) | [之后](../artifacts/polish-4.1.0/verification-final2/paired-operations-paused-desktop.png) |
| 390×844 暂停运维 | [之前](../artifacts/polish-4.1.0/baseline/paired-clean/paired-operations-paused-mobile.png) | [之后](../artifacts/polish-4.1.0/verification-final2/paired-operations-paused-mobile.png) |

桌面回答区域开头从 **936.02px → 674.97px**，提前261px；手机从 **1164.36px → 794.66px**，提前370px。长模型名条件下两个视口均可看到回答正文开头。短答不再靠固定空白撑开。手机长阅读有单一样本标题吸附，[实际滚动状态](../artifacts/polish-4.1.0/verification-final2/mobile-reading-context.png)保持模型与样本定位。

初次 paired 截图遇到 Chrome resize 后旧画面重影，原材料保留供追溯；交付采用 `paired-clean`：冻结4.0.0源码、相同fixture、禁用GPU并等待字体/两帧渲染，重新跑44项基线检查生成。没有修图抹除 UI 问题。

## 本轮实际验证

平台：macOS arm64、Go 1.27.1、Chrome 157.0.8103.78。命令记录位于 [verification](../artifacts/polish-4.1.0/verification)，包含时间、完整命令、退出码、日志哈希、执行前后源码哈希和构建输出哈希。

| 验证 | 实际结果 |
| --- | --- |
| `sh scripts/build_ui.sh` | 通过；5项资源与静态manifest一致 |
| fake API 浏览器回归 | **65项通过**；[ui-results.json](../artifacts/polish-4.1.0/verification-final2/ui-results.json) |
| 插件独立module | `go test -count=1 ./...`、`go test -race -count=1 ./...`、`go vet ./...` 均通过 |
| 原生共享库 | `go build -buildmode=c-shared` 通过，最终库SHA-256 `0dcd0c2f02a964733c02d93ef13c836c2ac94d3d3e78463dc4484467b600b7cd` |
| 真实装载/生命周期 | `TestModelQualityNativeLifecycle` 显式启用最终库并 `-count=1` 通过；8次离线调用，包含取消、暂停、禁止复测、卸载/重启持久与人工解除 |
| 实际宿主浏览器 | **10项通过**、6次fake调用；含5资源 source=静态发布=实际宿主返回字节，暂停期间零新增调用；[native-ui-results.json](../artifacts/polish-4.1.0/verification-final2/native-ui-results.json) |
| 宿主 | 本轮未改Go源码；全量测试和auth/pluginhost/usage/httptransport race均 `-count=1` 通过；test-output要求编译及发行宿主构建通过 |
| 原始参考与补丁 | 2303文件只读基线校验、干净副本补丁复现通过 |
| 压力 | 8模型×4样本共32份、每份256KiB，总8MiB；完整原文和ID复制正确，另覆盖每模型8样本。打开到筛选/复制累计约365ms，仅本机一次观测，不作性能承诺 |

原生宿主二进制 SHA-256 仍为 `3cd77bfd7a1f1f671187f0bd784b52ee9b253ef3d60771d16af20fc362a78d2b`，本轮重新测试而非把历史状态写为 passed。最初一条 host-race 命令路径错误的失败记录保留，最终使用正确路径的 `host-race-fresh`。开发过程中源码变化导致的哈希拒绝同样保留，未手改验证哈希。最终代码复核补充字段校验焦点与迟到反馈分区保护后，在 `verification-final2` 重新执行插件测试、构建、原生和浏览器检查；之前记录保留，发行门槛只采用最终记录。宿主源码未变的本轮fresh记录复用原始时间与日志哈希，没有改写。

覆盖360/390/768/1024/1280/1440宽度，浅色/深色/纯白真实渲染，空、加载、运行、完成、明确错误、证据不足、暂停、解除冲突和结果不确定。键盘跳转、Escape、焦点归还、reduced-motion、手机字段16px、主要触控目标44px、原生subgrid均有实际检查。

[contrast-results.json](../artifacts/polish-4.1.0/verification-final2/contrast-results.json) 记录三主题63对实际计算颜色：普通文字最小4.832:1，必要控件边界/焦点最小3.280:1；placeholder按实际computed颜色核对。这只证明记录中的颜色对，不是全站无障碍认证。

Impeccable未接入自动hook，本轮运行一次手动detector，原始结果为2个dialog padding warning和6个字号advisory（exit2），保留 [design-detector.json](../artifacts/polish-4.1.0/verification-final2/design-detector.json)，没有宣称零告警。随后对五类dialog×360/390实测：底部padding20px、真实内容留白20–26.39px，[dialog-layout-results.json](../artifacts/polish-4.1.0/verification-final2/dialog-layout-results.json) 及截图证明未贴边，静态检测未解析动态safe-area表达式。手机表格字段名已改13px；保留的17px品牌/对话框、15px模型标题、11px页脚/折叠任务ID已在最终DESIGN中明确，关键状态不使用11px。

## 发布、安装和预览

本地候选目录为 `artifacts/candidates/polish-4.1.0-rc1/`，含 `model-quality-0.3.0-ui-4.1.0-polish-4.1.0-rc1-darwin-arm64.tar.gz`、`release-manifest.json`、`validated-build.json`、`PAYLOAD_SHA256SUMS`、`SHA256SUMS`。候选归档与解包实际运行的最终结果绑定在同目录 `INSTALL_SMOKE.json`；该报告放在包外，避免归档与其验收报告循环哈希。安装请按 [CANDIDATE_RELEASE_CN.md](CANDIDATE_RELEASE_CN.md) 先校验，再按 [INSTALL_CN.md](INSTALL_CN.md) 备份和升级。

打包器拒绝任何新增/变更源码、动态库、报告或截图与验证不一致的情况。源代码交付保留必要宿主源码、补丁/基线、PRODUCT/DESIGN、许可证和明确白名单的脱敏证据，不包含真实配置、密钥、auths、数据库、演示状态、运行日志或浏览器数据。旧同名0.3.0归档、旧发布清单和原有演示均保留。

可运行的原生离线预览：先使用候选包内宿主与库，在解包根目录按 [DEMO_CN.md](DEMO_CN.md) 设置独立 `MQ_DEMO_DIR` 后执行 `python3 scripts/demo.py start`；返回页面地址和仅用于本机演示的假密钥。停止时使用同一目录执行 `stop`。本轮测试启动的临时服务已清理，用户原有演示没有被停止。

## 未验证范围与保留边界

没有已知阻塞本轮交付的功能缺陷。手机测试为桌面Chrome模拟视口；720×450仅模拟1440×900在200%下的重排空间，未声称实际浏览器缩放、iOS软键盘、安全区真机或屏幕阅读器已验收。Safari、Firefox、真实Android/iOS以及Linux/Windows/FreeBSD未验证；subgrid回退只提供普通块布局，不承诺同等行对齐。WCAG 2.2 AA适用目标按已列项目提供证据，不作整站认证。

所有原安全与业务语义保留：手动评测不自动处置；质量暂停覆盖整个凭证且禁止目标/对照/判题复测；只有完整、可比较、明确错误证据推进失败；仅管理员解除匹配归属，相关规则仍停止、旧计数清零；人工禁用、冷却、其他owner继续独立有效。原文转义、空sandbox、inert、CSP、内存密钥、同源v8、会话代际隔离及原生取消链路保留。自动证据仍限原生OpenAI-compatible chat/completions，Home质量动作不支持。
