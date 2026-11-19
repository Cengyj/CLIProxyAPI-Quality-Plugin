# UI 12.1：恢复横向导航与紧凑账号目录

2026-11-18 12:34（北京时间），本机 **CPA 8.0.76 / 8317** 已从插件 0.17.3、UI 11.18.0 升级至**插件 0.18.1、UI 12.1.0**；内嵌运行时仍为 **2.1.3**。打开 [CPA 内嵌插件页面](http://127.0.0.1:8317/management.html#/plugin-pages/model-quality/0)，刷新整个管理标签页即可；插件内的「刷新」只重新读取数据。

## 界面决定

以 [11.18 安装包清单](../artifacts/ui-rework-11.18.0/plugin-package/build-manifest.json) 逐项核对六份前端资源后，用该版本的 HTML、布局和样式作基准。12.0 的深绿侧栏、英文编号、大号账号计数和单行操作不再使用。独立页和 CPA 内嵌页都有相同的横向导航，浅色、纯白和深色分别沿用原有的页面与表格底色；主按钮仅调整文字与背景的对比。

| 工作区 | 桌面与手机的实际变化 | 桌面 / 手机截图（合成数据） |
| --- | --- | --- |
| 账号管理 | 筛选、搜索、巡检和导入回到一条工具栏。登录、重置、删除与资料、日志、更多分两行排列；确认、状态未知时锁定和取消后的焦点返回都保留。手机按钮加高至 44px，连接状态不只靠圆点表示。 | [嵌入 1064px](../artifacts/ui-rework-12.1.0/browser/embedded-1064-login.png) / [390px](../artifacts/ui-rework-12.1.0/browser/embedded-390-login.png) |
| 智商测试 | 960px 及以上左侧配置、右侧阅读；窄屏用「测试设置 / 本次回答 / 测试记录」切换同一份表单。读历史回答再返回不会清空题面，手机仍可展开任务操作下载结果。 | [回答对比](../artifacts/ui-rework-12.1.0/browser/manual-desktop.png) / [320px 回答](../artifacts/ui-rework-12.1.0/browser/manual-answers-320.png) |
| 质量观察 | 恢复原来的概览、规则和审计排版。统计区单列「结论未知」，不将缺失证据计为答错；历史暂停数量放在归属列表标题，不与评测指标混用。不启用官方宿主不支持的暂停或释放。 | [嵌入 1064px](../artifacts/ui-rework-12.1.0/browser/embedded-1064-operations.png) / [320px](../artifacts/ui-rework-12.1.0/browser/embedded-320-operations.png) |

同样本、同视口也可直接对照：[11.18 账号列表](../artifacts/ui-rework-12.0.0/browser/mq-ui12-before-accounts.png) 与 [12.1 账号列表](../artifacts/ui-rework-12.1.0/browser/accounts-reference-size.png)。

## 验证与交付

- [33 项前端测试](../artifacts/ui-rework-12.1.0/frontend-tests.log)通过。[浏览器报告](../artifacts/ui-rework-12.1.0/browser/verification.json)含 68 项布局检查和 18 张截图。检查了三套主题、导航与行按钮文字、确认与取消、草稿保留及读取失败后的恢复；浏览器未发出外部请求或管理写入。复验：`scripts/build_ui.sh`，`node --test scripts/tests/*.test.cjs`，`node scripts/verify_workspace_ui.cjs`。
- [Linux arm64 ZIP](../artifacts/ui-rework-12.1.0/plugin-package/model-quality-v0.18.1-linux-arm64.zip) 仅含 `model-quality.so`。[构建日志](../artifacts/ui-rework-12.1.0/native-build.log)与[清单](../artifacts/ui-rework-12.1.0/plugin-package/build-manifest.json)对应本轮源码。用 CPA 8.0.76、临时空目录、无外网、无发布端口做[加载及重启检查](../artifacts/ui-rework-12.1.0/native-smoke.json)：插件已注册并启用；六份资源摘要与源码相符；未鉴权管理请求返回 401，两次账号和任务均为空。
- 官方宿主原档与构建快照均匹配 2,413 个基线文件，未改动宿主。其[独立编译和测试记录](../artifacts/ui-rework-12.1.0/host-check.log)显示编译通过；在 `--network=none` 下，只有 WebRTC 数据通道测试未能建立连接。使用无外网的 Docker 内部网卡运行同一份源码构建的[完整 live 测试包](../artifacts/ui-rework-12.1.0/host-relay-internal.log)通过，未跳过该用例。

## 8317 实际部署与核验

- 停服备份并逐文件核验后，只替换 `model-quality.so`，再启动原容器。插件注册且已启用，实际返回的六份前端资源摘要与构建清单一致；未鉴权管理请求返回 401。没有再次重启，宿主镜像、配置和凭证文件未变。见 [部署记录](../artifacts/ui-rework-12.1.0/deployment.json)。
- 前后核对保留了 1 个插件账号、2 条历史登录任务、1 个 CPA 凭证、原始日志事件和设置。没有重放任务、执行真实账号登录、模型调用、重置 2FA 或退出设备。
- [实际管理页浏览器核验](../artifacts/ui-rework-12.1.0/deployed-iframe.json)：在临时浏览器中通过 8317 官方面板鉴权，从用户使用的管理路由进入。面板嵌入的同源文档标记为 `12.1.0`，默认打开「账号管理」；桌面显示「质量观察 → 结论未知」，390px 下「智商测试」显示「测试设置 / 本次回答 / 测试记录」。仅放行本机 GET 请求，没有保存真实账号页面截图。

**18417 是预览，不是部署入口。** 需要演示数据时，可启动 `node scripts/login_ui_fixture.cjs --port 18417` 后打开 [内存样本演示](http://127.0.0.1:18417/embedded.html?demo=1&page=login)，无须输入真实密钥。[UI 11.18](UI_REWORK_11_18_0_CN.md) 保留的是 8317 上一版的部署记录。
