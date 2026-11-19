# UI 12.3.0：重置 2FA 的紧凑单栏

2026-11-18 20:41（北京时间），[8317 管理面板](http://127.0.0.1:8317/management.html#/plugin-pages/model-quality/0)已更新为 **UI 12.3.0 / 插件 0.18.6**。官方 CPA 8.0.76、内嵌引擎 2.1.3 保持原版本。刷新整个管理标签页，账号行打开「重置 2FA」；连接信息可核对 UI 版本。

## 用户选择与最终设计

用户否定 12.2.0 的左右分区，明确选择 **「紧凑极简：单栏、少边框、重点突出」**。因此替换上一版结构与相应 CSS：

- 移除左右栏、账号说明框、旧／新密钥对照栏、选项卡片、装饰图标及副标题。
- 完整邮箱紧接标题，重置影响与新密钥保存顺序用两行说明表达。
- 两个复选项纵向排列，复选框在文字左侧；整行可点击，只有悬停或键盘聚焦时出现辅助反馈。取消重复的「可选」「后续操作」视觉标题，保留辅助技术的分组名称。
- 简化授权描述，保留排除套餐不上传、新密钥不回滚、不确定结果停止等实际边界。
- 底部只有一条分隔线、一行实际执行摘要和右对齐按钮；取消使用轻量文字样式，确认保留强调色。四种组合不改变弹窗和按钮位置。
- 桌面与手机均为单栏，不依靠降低字号塞入多列。正文及说明使用 12–14px，标题 21px；窄屏操作按钮保持 44px，短窗口滚动主体。

| 同一虚构账号 | 12.2.0 | 12.3.0 |
| --- | --- | --- |
| 1440px 视口 | 680 × 469 | 432 × 423 |
| 390px 视口 | 351 × 645 | 351 × 431 |

手机高度减少约 33%，320px 默认内容也无需滚动。长邮箱和极短窗口仍完整保留内容，通过内部滚动阅读。仅查看、勾选、取消、关闭不提交任务。

截图：[上一版桌面](../artifacts/ui-rework-12.2.0/installed-synthetic-1440.png) · [新版 8317 桌面](../artifacts/ui-rework-12.3.0/installed-synthetic-1440.png) · [新版 8317 手机](../artifacts/ui-rework-12.3.0/installed-synthetic-390.png) · [深色](../artifacts/ui-rework-12.3.0/browser/reset-1064-dark.png) · [320px](../artifacts/ui-rework-12.3.0/browser/reset-320-light.png) · [短窗口错误](../artifacts/ui-rework-12.3.0/browser/reset-short-error.png)。全部使用虚构账号。

## 验证与部署

- [106 组布局检查、30 张截图](../artifacts/ui-rework-12.3.0/browser/verification.json)：三主题、窄屏、长邮箱、短窗口、四种选择、键盘、焦点返回、账号版本过期和独立退出；[34 项前端测试](../artifacts/ui-rework-12.3.0/frontend-tests.log)通过。
- [提交保护](../artifacts/ui-rework-12.3.0/browser/security-submit.json)：四种重置参数、独立退出、忙碌时禁止重复提交及结果不明后阻断均通过。模拟写入全部在浏览器截获，未转发至服务。退出默认关闭、授权默认开启及原确认提交语义保留。
- [宿主基线](../artifacts/ui-rework-12.3.0/host-baseline.json)核对 2,413 个文件；[宿主编译测试](../artifacts/ui-rework-12.3.0/host-check.log)使用相同基线缓存，[WebRTC 测试](../artifacts/ui-rework-12.3.0/host-live.log)在隔离私有网络运行。插件与引擎[测试、vet、构建](../artifacts/ui-rework-12.3.0/native-build.log)及[隔离加载和重启](../artifacts/ui-rework-12.3.0/native-smoke.json)通过。
- [部署记录](../artifacts/ui-rework-12.3.0/deployment.json)：停服备份校验后仅替换插件库。1 个账号、2 条历史任务、日志、设置及 1 个 CPA 凭证保留，宿主镜像、配置和凭证文件未变。6 份在线资源与包摘要一致。
- [实际 iframe 核验](../artifacts/ui-rework-12.3.0/deployed-iframe.json)：8317 加载已安装代码，插件业务读取由内存 fixture 提供，核对桌面/手机单栏、勾选响应、版本与按钮可点击。没有真实账号重置、退出、登录或模型调用。

交付：[Linux arm64 插件 ZIP](../artifacts/ui-rework-12.3.0/plugin-package/model-quality-v0.18.6-linux-arm64.zip) · [构建清单](../artifacts/ui-rework-12.3.0/plugin-package/build-manifest.json) · [相对 12.2.0 的源码差异](../artifacts/ui-rework-12.3.0/ui-source.diff)。
