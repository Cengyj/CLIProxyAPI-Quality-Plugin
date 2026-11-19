# Model Quality UI 重构交付记录

日期：2026-11-14（Asia/Shanghai）。界面模式：Operate。静态 UI 版本：4.0.0；后端 API / 插件版本仍为 0.3.0。

## 实际设计

- 顶部只有手动评测、质量运维两项主导航。删除旧侧栏、装饰性工具导航、旧双栏工作台和全部旧 CSS。
- 手动评测拆为配置评测、当前任务、任务历史。配置按目标与模型、题目与标准、执行参数分组；预设和高级判题按需展开。提交后进入结果，配置不占据回答阅读区域。
- 回答按模型分组，选择最多两个模型并比较同编号样本；选择顺序决定阅读顺序。桌面对应标题与样本行对齐，手机逐模型显示。长模型名换行，长回答可滚动或完整展开。
- 执行状态、质量判定、可比较性和动作分开。冻结条件与样本证据使用可读字段；JSON 放入深入排查入口。参考答案、推理配置、模型映射、身份、指纹与未知值保持真实语义。
- 运维划分需要关注、检测规则、审计记录。先呈现暂停归属及凭证限制，质量通过率和覆盖率保留分母。规则与预算使用页内编辑，解除保留受保护的确认对话框。
- 解除展示身份、归属、动作版本、整个凭证作用范围和剩余限制。409 使原预览失效；503 / 网络结果不确定时禁用再次提交，以读取核对结果。刷新或关闭过程使用预览代际隔离。解除后相关规则仍停止、旧失败证据清零，需另行显式恢复。

设计方向与功能—状态—操作—反馈清单在 `docs/UI_DESIGN_BRIEF.md`。最终系统见根目录 `DESIGN.md` 与 `docs/design-system.json`。使用说明见 `docs/UI_CN.md`。

## 颜色来源

只读宿主 8.0.67 的管理资源 updater 和示例配置指向 `router-for-me/Cli-Proxy-API-Management-Center`。固定前端参考提交为 `d0126752201a35ff999bd2a1d4741215542258a8`，以 `src/styles/themes.scss` 的暖灰、纯白、暖黑角色为依据。

本轮还只读 GET 了 `http://127.0.0.1:8317/management.html` 的公共静态资源；标题为 CLI Proxy API Management Center，七个关键颜色值与固定参考一致。静态文件 SHA-256 为 `650ca63b43487bcf2ebda6e6d66b8ac8429e5c1715c5f00f50405e9addf0bf70`。没有读取该部署的管理密钥或凭证，也不从静态色值推断其精确提交。

插件自行定义语义 token。主行动浅色使用 `#726d67` 配白字、深色使用 `#c9c3bb` 配暖黑字；次级文字调整为 `#68625b`，使悬停背景上的对比度达到要求。来源与 MIT 许可保留于 `plugin/NOTICE.md`。

## 修改文件

| 文件 | 变化 |
| --- | --- |
| `plugin/ui/index.html` | 完整替换页面、导航、任务区、表单组织与对话框 |
| `plugin/ui/style.css` | 完整替换颜色、排版、组件、响应式、焦点与 reduced-motion |
| `plugin/ui/app.js` | 视图切换、比较筛选、页内编辑、解除状态与请求隔离、进度反馈 |
| `plugin/ui/view.js` | 分层证据、模型分组、缺失样本、规则/限制/归属与解除预览渲染 |
| `scripts/verify_ui.cjs` | 同步 DOM 选择器，保留原业务与安全断言并扩展新交互检查 |
| `scripts/verify_native_ui.cjs` | 同步原生 UI 导航和解除对账流程 |
| `docs/UI_CN.md`、`plugin/NOTICE.md` | 新界面使用方法与准确配色来源 |
| `PRODUCT.md` | 同步当前工作区名称和本次确定的配色承诺 |
| `DESIGN.md`、`docs/design-system.json` | 从最终实现记录设计系统 |

`client.js` 的内存密钥、鉴权、AbortController、连接代际隔离继续使用。未修改 Go 源码、宿主源码或业务路由；新静态文件已重新嵌入共享库。

## 实际执行的验证

| 检查 | 本轮结果 | 证据 |
| --- | --- | --- |
| `sh scripts/build_ui.sh` | JavaScript 语法及静态发布通过 | `artifacts/ui/manifest.json` |
| fake 管理 API 浏览器回归 | 44 项通过，222 次本机假 API 请求，无浏览器异常 | `artifacts/verification/ui-results.json` |
| 插件 `go test ./...` | 通过 | `artifacts/verification/redesign-plugin-tests.log` |
| 插件 `go test -race ./...` | 通过 | `artifacts/verification/redesign-plugin-race.log` |
| `go build -buildmode=c-shared` | macOS arm64 共享库重建完成 | `artifacts/model-quality.dylib` |
| 宿主 `TestModelQualityNativeLifecycle` | 加载、流式/心跳、取消、失败暂停、禁止复测、重载、解除、卸载、重启通过；8 次离线调用 | `artifacts/verification/redesign-native-lifecycle.log` |
| 原生静态资源一致性 | 已加载共享库返回的 5 个资源与最终源码逐字节一致 | `artifacts/verification/redesign-native-static.json` |
| 实际宿主 native UI | 9 项通过；6 次明确的离线模型调用；暂停期间无调用 | `artifacts/verification/native-ui-results.json` |
| 只读宿主基线校验 | 2303 个源码/构建文件保持一致 | `python3 scripts/verify_baseline.py --reference` |
| impeccable detector | 仅一项对比度建议，已修正对应浅色次级文字 | `artifacts/verification/redesign-detector.json` |
| 最终语义对比度 | 13 对文字/背景均 ≥ 4.5:1，最低 4.83:1 | `artifacts/verification/redesign-contrast.json` |

浏览器：Chrome 157.0.8103.78，macOS arm64。UI 与原生宿主检查分开记录。没有 Go 源码改动，因此本轮没有重跑整个宿主 `go test ./...` 或全库 gofmt；宿主集成使用现有带补丁可执行文件及本轮重建的原生插件。

原有密钥仅内存、同源 v8 鉴权、打开/连接/保存不调用模型、重复提交合并、迟到响应隔离、轮询恢复、断连撤销、401 清理、HTML/SVG 转义、sandbox/CSP/脚本网络导航限制全部保留断言。新增模型筛选顺序、样本评审索引、长回答展开、键盘焦点、reduced-motion、解除冲突、不确定对账、失败核对和关闭后的迟到预览检查。

源码及二进制最终校验和在 `artifacts/verification/redesign-validated-build.json`。

## 本轮截图

21 张 fake API 真实浏览器截图已逐项核查；原生验证另有 3 张截图。全部是假数据，不能作为真实模型质量或生产凭证证据。截图没有使用概念图代替实际界面。

- 桌面浅色比较：`artifacts/verification/comparison-page.png`
- 桌面深色比较：`artifacts/verification/comparison-dark-page.png`
- 暖黑运维：`artifacts/verification/operations-dark-page.png`
- 平板：`artifacts/verification/tablet-page.png`
- 手机比较：`artifacts/verification/mobile-comparison-page.png`
- 手机配置：`artifacts/verification/mobile-setup-page.png`
- 规则编辑：`artifacts/verification/rule-editor-page.png`
- 证据详情：`artifacts/verification/evidence-detail-page.png`
- 解除冲突、结果不确定、解除后仍受限：对应 `release-conflict-page.png`、`release-uncertain-page.png`、`released-still-restricted-page.png`
- 完整状态清单见 `ui-results.json`，联系表为 `redesign-capture-contact-sheet.png`。

## 独立设计复核

独立子代理按 impeccable 完成复核，结论为 **ship**，没有 material fixes。其报告核查 25 份截图输入（含要求视口的复核副本）、方向契约和关键状态代码；确认任务分区、CPA 暖色主题、对齐比较、移动重排和解除结果表达与契约一致。详见 `artifacts/verification/redesign-finish-review.md`。这是 code-led Operate 实现，没有批准的概念图或选中的 catalog QUALITY BAR 卡；视觉判断依据实际界面、产品约束与方向契约。当前工具没有命名 agent profile 调度，使用独立新上下文子代理读取 skill 复核规范执行。

## 可运行预览与边界

本轮启动的隔离预览信息在 `artifacts/redesign/preview.json`，已准备两项离线任务：多模型、多样本逻辑评测，以及视觉题。页面连接使用假管理密钥 `mq-demo-admin-not-a-live-secret`。需要停止时执行 `python3 scripts/demo.py stop`；需要重新启动时执行 `python3 scripts/demo.py start`，以新返回的端口为准。

本轮没有使用真实账号、复制生产凭证或修改持续运行的正式部署。当前原生构建和浏览器验证范围是 macOS arm64 / Chrome；其他 OS、浏览器和正式无障碍认证仍需独立验证。暂停中的凭证无复测或绕过入口；人工解除不会清除其他限制，不提供分组、按模型隔离、自动恢复或通用 IQ 分数。
