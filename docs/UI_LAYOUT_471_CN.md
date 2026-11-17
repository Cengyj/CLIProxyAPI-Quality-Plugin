# UI 4.7.1：左侧题面与右侧执行选项

2026-11-15 按最新要求调整布局：凭证选择位于题目选择左侧，编辑框位于左列下方；模型、思考强度、回答数量和开始测试集中在右列。开始按钮位于设置下方，重置题目移至编辑框底部。

只调整 `plugin/ui/index.html` 和 `style.css`；JS、模型目录、能力联动、任务提交与质量暂停逻辑均沿用4.7.0。手机先显示凭证/题目和编辑框，再显示模型设置与开始按钮；允许自然滚动，保留44px主按钮。

## 已完成验证

- 插件模块 `go test ./...` 通过并重建原生共享库；未修改宿主或插件Go业务代码。
- 实际原生生命周期验证通过，实际宿主浏览器17项通过。
- 假管理API浏览器61项检查通过；实际CPA外壳16项检查通过，覆盖27个主题/视口/侧栏组合、18个桌面首屏检查及200%缩放。
- 1280×720、展开侧栏时，右侧开始按钮底部约421px，实际短回答从约592px开始且完整可见。三种主题和侧栏收起均通过。
- 键盘顺序：凭证→题目→编辑框→模型→思考强度→回答数量→开始；内置题修改后重置按钮进入编辑框之后的Tab顺序。
- 源码、静态构建与原生宿主返回资源哈希一致。全部模型测试使用隔离假凭证与本机假上游。

证据目录：`artifacts/prompt-layout-4.7.1/`，包括 `plugin-tests.log`、`native-lifecycle.log`、`fixtures/ui-results.json`、`host/host-ui-results.json` 和 `native/native-ui-results.json`。独立界面复核与本地更新状态分别以 `finish-review.md` 和 `deployment-checks.json` 为准。

主截图：`host/after-embedded-light-1280x720-expanded.png`；手机截图：`fixtures/paired-setup-mobile.png`。

独立界面复核结论为 **ship**。本地8317已更新至4.7.1；`deployment-checks.json`确认配置、凭证文件和质量限制/规则状态保持一致，升级提交模型调用为0，旧库和质量状态备份已保留。
