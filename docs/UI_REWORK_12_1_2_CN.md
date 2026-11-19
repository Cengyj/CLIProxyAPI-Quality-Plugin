# UI 12.1.2：重置 2FA 确认框

2026-11-18 13:55（北京时间）已安装到[本机 8317 的 CPA 管理页](http://127.0.0.1:8317/management.html#/plugin-pages/model-quality/0)：插件 **0.18.3**，官方宿主 **8.0.76**，内嵌运行时 **2.1.3**。刷新整个标签页，在「账号管理」的账号行打开「重置 2FA」。仅查看或取消不会提交任务。「已连接」或「连接管理接口」弹窗中可核对界面版本。

| 核对事项 | 改动 |
| --- | --- |
| 哪个账号会受影响？ | 标题下的目标账号独占一行，窄屏长邮箱换行，不依赖悬停查看。旧验证器失效的提示与本机保存新密钥的时序放在一起。 |
| 还会做什么？ | 两个后续操作用整行可点的复选框。退出设备默认不选，重新授权 CPA 默认选中。套餐排除只影响凭证上传，不会跳过已经确认的新密钥保存。 |
| 确认后按什么顺序？ | 预览只列出这次会执行的步骤，不用完成标记，不显示未选步骤。四种组合都不改变弹窗和按钮位置。 |
| 小窗口或状态变化怎么办？ | 内容随高度滚动，取消和确认留在底栏。账号版本变化会展示错误并禁止提交，仍可取消；关闭后焦点回到原账号行。独立「退出所有设备」没有重置步骤。 |

**截图均使用虚构账号**：[改前桌面](../artifacts/ui-rework-12.1.2/baseline/reset-1064.png) · [改前 320px](../artifacts/ui-rework-12.1.2/baseline/reset-320.png) · [8317 桌面](../artifacts/ui-rework-12.1.2/installed-synthetic-1440.png) · [8317 手机](../artifacts/ui-rework-12.1.2/installed-synthetic-390.png) · [深色](../artifacts/ui-rework-12.1.2/browser/reset-1064-dark.png) · [短窗口错误](../artifacts/ui-rework-12.1.2/browser/reset-short-error.png)。

## 验证与交付

- [浏览器回归](../artifacts/ui-rework-12.1.2/browser/verification.json)覆盖 90 组布局，其中 22 组专测重置框：三主题、桌面、390/320px、四种组合、长邮箱及账号版本变化。还核验勾选、空格、取消、关闭、Esc 和再次打开的默认值。浏览器未尝试业务写入。另有 [34 项前端测试](../artifacts/ui-rework-12.1.2/frontend-tests.log)通过。
- [插件和内嵌运行时测试、vet 与构建](../artifacts/ui-rework-12.1.2/native-build.log)通过。[官方源码基线](../artifacts/ui-rework-12.1.2/host-baseline.json) 2,413 个文件一致；[宿主编译与测试](../artifacts/ui-rework-12.1.2/host-check.log)复用同一基线构建缓存，[WebRTC 测试](../artifacts/ui-rework-12.1.2/host-live.log)在隔离私有网卡上运行。 [Linux arm64 安装包](../artifacts/ui-rework-12.1.2/plugin-package/model-quality-v0.18.3-linux-arm64.zip)仅含插件库，[隔离宿主加载和重启](../artifacts/ui-rework-12.1.2/native-smoke.json)均通过。
- [部署记录](../artifacts/ui-rework-12.1.2/deployment.json)：无运行中任务或启用的自动化，停服备份核对后仅替换插件库。1 个插件账号、2 条历史登录任务、日志、设置及 1 个 CPA 凭证与升级前一致。镜像和受保护的配置、凭证文件未变；六份在线资源摘要与包一致。没有真实登录、重置、退出或模型调用。
- [实际页面核验](../artifacts/ui-rework-12.1.2/deployed-iframe.json)：从 8317 的官方面板及真实插件 iframe 加载代码，仅在临时浏览器中用内存 fixture 提供业务读取，不把测试写入或实际账号送入 fixture。1440px 和 390px 视口的弹窗分别为 468×491 与 351×507px，两个底栏按钮均不被宿主遮挡。
