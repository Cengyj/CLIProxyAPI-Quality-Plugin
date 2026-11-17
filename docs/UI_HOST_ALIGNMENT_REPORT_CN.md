# UI 4.3.0 宿主风格对齐与本地候选验收

实施日期：2026-11-14（Asia/Shanghai）。后端插件 0.3.1，界面 4.3.0。工作基线为根仓库 `82e0b4b908cb12b3be89ab5df4f8c4f98dbbd01a`。开始时已有 `docs/UI_COMMERCIAL_POLISH_PROMPT_CN.md` 修改与未跟踪的 `docs/UI_HOST_ALIGNMENT_ANALYSIS_CN.md`；两项原始工作均保留，没有为制造基线提交或重置文件。

## 宿主依据与实际实现

视觉来源为独立管理前端 `workspaces/cpa-management-reference/`，固定提交 `d0126752201a35ff999bd2a1d4741215542258a8`。CPA 原始 Go 归档的 `config.example.yaml` 指向该独立仓库；原始 Go 目录保持只读。本轮没有修改宿主 Go 业务实现，仍对现有可复现补丁执行测试、编译与基线校验。

| 组件或布局 | 参考 | 本轮实现 |
| --- | --- | --- |
| 页面与容器 | PluginsPage、themes.scss | page/card/input/floating/hover 独立角色；28px/700 标题、14px 说明；普通卡片 24px 内边距、12px 圆角 |
| 按钮与字段 | Button、Input、Select、components.scss | primary/secondary/ghost/danger 固定职责；8px 圆角、统一间距、loading 宽度保持、关联字段错误 |
| 导航 | ProviderTabs | 选中下划线、hover、focus 共用语言；主任务与局部导航层级清楚，窄屏局部滚动 |
| 开关与表格 | ToggleSwitch、Table | 44×24 轨道和 18px 滑块，仍由原生 checkbox 承载；三种规则许可分别保存；统一表头、行边界、hover 与手机标签行 |
| 规则分组 | SectionCard | 页内五组：目标与题目、判定条件、检测调度、凭证处置、调用预算与保留 |
| 弹窗 | Modal / Sheet 的结构 | 保留原生 dialog，统一头部、分隔、滚动和底部动作；Esc、背景隔离与返回焦点不改变 |
| 外壳与主题 | PluginResourcePage、layout.scss、useThemeStore | 不重复 CPA 侧栏和全局工具；移除面包屑行与 194px 右侧预留；只读跟随可识别同源父页已应用主题 |

独立页面保留 Model Quality 身份与三主题选项。识别同源 CPA 后，父根元素无 `data-theme` 映射为 light，white 与 dark 保持独立，监听属性变化；宿主 auto 在系统浅色时实际应用 white。无法识别、跨源或异常时回退插件自己的内存主题选择。没有读取父页密钥、认证 store 或浏览器存储，没有修改父页 DOM，也没有声称存在主题消息协议。宿主登录不意味着插件已连接。

内嵌工具避让采用固定参考的纵向安全带：宿主桌面浮动工具位于 y=24–76，手机位于 y=12–60；插件任务导航分别从 88px / 76px 后开始。它是针对固定宿主布局的适配，不是宿主几何 API。检查使用实际 iframe 宽度，侧栏宽度不重复加入插件。

## 有理由保留的偏差

- 主按钮使用 `#726d67` 白字，约 5.12:1；未直接采用宿主 `#8b8680` 白字约 3.61:1。字段默认边界也加深。主题状态的实际成对检查保存在 contrast-results.json；不宣称无障碍认证。
- 手机字段字号至少 16px、主要触摸目标至少 44px；不直接缩小到宿主桌面紧凑尺寸。
- 保留原生 select、datalist、details、checkbox 和 dialog，没有引入 React、组件库、CDN 或远程字体。组件为原生实现，来源与许可见 `plugin/NOTICE.md`。
- 常规反馈 150ms，开关与进度 200ms，尊重 reduced-motion。没有复制宿主的页面入场效果，轮询不会触发入场动画。
- “已完成”和“已释放”使用中性状态，质量正确、规则启用和凭证可用仍各有明确标签。解除反馈不保证整个凭证恢复可用。

## 两个工作区

智商测试保留凭证→题目→可编辑题面→单模型/思考强度/1–8 份回答→显式开始→同页结果。桌面将凭证与题目放在配置工作面的同一行，题面占主要宽度，执行选项占次要宽度；手机按任务顺序堆叠。输出上限与 Temperature 在次级设置中。回答/记录单独组成阅读工作面。执行后可查看回答或收起配置；收起不销毁草稿，冻结条件另行展示。短回答不产生人为高度，长内容可展开并保持原文复制和导出；手机长回答有局部样本身份提示。

质量运维先展示暂停归属、影响对象与剩余限制，统计位于后面。规则列表、凭证表与审计使用相同密度与边界。规则编辑使用五组页内结构，三个许可仍互相独立，保存不调用模型。人工解除仍核对归属、身份、动作版本与剩余限制，冲突、结果不确定及重新核对不伪装成功；解除后规则继续停止，恢复规则是另一个显式动作。

断连、未加载、加载中、已加载无数据和加载失败有分别说明。失败刷新保留已有数据并标记陈旧。已结束任务触发运维刷新时使用按对象键更新，保留规则披露、焦点和阅读状态。浏览器 bfcache 返回会清除旧连接与受保护数据，要求重新连接。

## 验证证据与边界

当前证据目录为 `artifacts/host-alignment-4.3.0/verification/`。只使用独立假凭证、loopback fake upstream 与隔离演示目录；没有操作用户现有 CPA 或演示，没有真实模型账户调用。

| 检查 | 实际执行入口与记录 |
| --- | --- |
| 静态资源与题目逻辑 | `sh scripts/build_ui.sh`；`scripts/test_question_test.mjs`，使用本机 Node runtime；`question-logic.json` |
| 插件独立 module | plugin-tests / plugin-race / plugin-vet / plugin-build，重建实际 C shared library |
| 宿主与补丁 | host-tests-fresh / host-race-fresh / host-compile / baseline / patch；宿主全量测试使用 -count=1 |
| 原生生命周期 | native-lifecycle.json，实际动态库加载、停用与生命周期 |
| 当前题目 API | native-api-results.json：资源与鉴权、单模型多回答、编辑内置题/自定义题、HTML、保存不调用、三轮答错暂停及解除后规则停止 |
| 独立 UI 浏览器 | ui-results.json：当前题目流程、异常、重复提交、401/迟到响应、草稿与轮询、原文/安全预览、解除冲突与结果不确定 |
| 实际原生宿主浏览器 | native-ui-results.json：真实宿主与动态库、假上游的执行、取消、历史、原文/安全预览、观察、凭证暂停、禁止依赖调用、人工解除与停止规则 |
| 实际宿主外壳 | host-ui-results.json：固定管理前端、相邻插件页、内嵌三主题、侧栏、用户视口、主题回退及自动主题映射 |
| 解包安装 | 候选目录的 INSTALL_SMOKE.json，绑定候选归档哈希，用包内实际宿主和动态库运行隔离冒烟 |

原有 verify_ui.cjs / verify_native_ui.cjs 中 4.1 配置台、手动多模型与预设选择器已迁移到当前题目流程；适用的业务、安全、错误、草稿、暂停与人工释放断言继续保留。开发期间的失败输出留在 preview 目录，候选包只纳入最终受哈希绑定的证据。

视口覆盖独立与内嵌的 1440×900、1280×800、768px、390px、360px，并加入本会话实际 1280×720。内嵌另检查展开/折叠侧栏和工具安全区；布局判断以报告中的实际 iframe 尺寸为准。三主题检查包含普通内容、规则、暂停、忙碌、错误、冲突与不确定状态。桌面自动化并不等同于真实手机、屏幕阅读器或跨浏览器认证；未实测的设备和平台不被标记通过。

最终独立 UI 报告包含 56 项检查和 87 张截图，原生题目 API 为 8 项，原生浏览器为 13 项。宿主外壳报告包含 13 项检查、27 组常规布局测量和 51 张截图，并另行验证三主题的真实 200% 浏览器缩放。前端执行浏览器为 Chrome 157.0.8103.78，构建平台为 macOS arm64。原生 API、原生浏览器和宿主外壳分别消耗 7 次、10 次和 2 次明确的本机假上游调用；这些数字不是生产模型成本。旧记录的多模型兼容压力检查覆盖 32 样本、每份 256KiB；当前手动入口仍限制一个模型、最多八份回答。

宿主 200% 缩放使用独立临时 Chrome profile 的浏览器缩放设置，不是 CSS zoom：验证外层 1440px、内容 CSS 视口 720px、devicePixelRatio=2，根元素 CSS zoom=1，并检查三主题截图。假 API 套件另有 720×450 CSS 视口的重排测试，二者明确区分。原生静态资源保留 CSP `frame-ancestors 'self'`，响应头已核对；本机 Chrome 157 的跨源嵌入请求先被本地网络访问检查拦截，插件未运行，不能据此宣称浏览器执行到了 CSP 拦截。跨源父页的 JS 回退仅在明确无该响应头的合成夹具中测量，不以它宣称原生跨源嵌入可用。

## 视觉复核与检测说明

Impeccable context 本会话执行一次。公共组件迁移后执行一次机械 detector，原始 11 条结果保留在 design-detector.json，没有删改成空列表：

- 两条 table-scroll 内边距提示：表格外围不再加额外 padding，文本由 th 的 10×14px、td 的 12×14px 留白；手机转为标签行。
- 两条 dialog 底部提示：静态扫描未解析 safe-area/max 表达式；实际五种 dialog 在 360/390px 测量，见 dialog-layout-results.json。
- 五条宽阴影提示对应互斥的原生弹窗；普通卡片只有 3px 轻阴影，不是成排弹窗效果。
- 一条嵌套卡片提示对应规则处置范围说明；其为有业务后果的局部提示，不是第二层导航或嵌套工作台。
- 一条 17px 品牌字体未登记提示：已补充 DESIGN 前置 token 与 sidecar 的 standalone brand 角色。

独立最终视觉复核结论为 `ship`，没有需要阻止交付的视觉问题；`finish-review.md` 与 JSON 绑定本轮五项 UI 源码和 138 张独立/宿主截图。检测器原始提示及其解释仍保留。机械扫描只提供线索，不能代替实际渲染、交互和业务验证。

## 截图与安装

前后对照使用相同假数据、主题和视口，基线为冻结的 UI 4.2.0。`baseline/baseline.json` 绑定起始源码；`baseline/paired-clean/` 与最终 verification 中八张同名 paired 图片构成独立页面对照。宿主内嵌对照和邻接插件管理页由 host-ui-results.json 逐项说明来源、视口、主题及截图哈希。两类数据来源分别记录，不混称生产数据。

便于浏览的对照为 `artifacts/host-alignment-4.3.0/comparison-desktop.png`、`comparison-mobile.png`；comparison-provenance.json 记录组成图片与输出哈希。4.1 与原有 4.2 截图继续保留为历史，不作为本轮通过依据。

本轮仅生成本地候选包，准确文件与校验路径见 [CANDIDATE_RELEASE_CN.md](CANDIDATE_RELEASE_CN.md)，部署步骤见 [INSTALL_CN.md](INSTALL_CN.md)。包内需要 quality capability v2 的补丁宿主，动态库仅已验证 macOS arm64。升级前备份宿主质量状态与插件数据库；替换库不会解除暂停或恢复规则。没有外部发布，也没有替换用户当前部署。

打包源文件白名单补齐了 `.mjs` 与 `.txt`，包括题目逻辑回归、Go embed 使用的原始糖果题和 LGPL/GPL 许可证文本。插件源码清单变化后重新记录受影响的 module/原生检查；动态库重建哈希保持一致。候选解包冒烟另检查包内源码能够使用本机既有 Go 模块缓存、关闭网络下载后编译，防止只验证二进制而遗漏重建所需文件；这不意味着候选包内置了所有第三方依赖。
