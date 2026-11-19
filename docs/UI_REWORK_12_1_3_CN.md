# UI 12.1.3：重置 2FA 的账号、选择与确认

2026-11-18 19:06（北京时间），[8317 官方管理面板](http://127.0.0.1:8317/management.html#/plugin-pages/model-quality/0)已从 UI 12.1.2 / 插件 0.18.3 升级到 **UI 12.1.3 / 插件 0.18.4**。宿主 CPA 8.0.76 和内嵌引擎 2.1.3 不变。刷新整个管理标签页，进入「账号管理」，在账号行打开「重置 2FA」。仅打开、勾选或取消不会提交任务。

## 如何核对一次重置

- **先看账号和影响。** 标题旁的图标对应重置操作，下方显示完整邮箱。旧验证器将无法验证此账号；新密钥确认生效后先保存到本机，再处理其他选项。窄框长邮箱独占一行并自然换行，不用悬停，也不会挤走确认按钮。
- **再选后续操作。** 桌面将「退出所有设备」和「重新授权 CPA」并排，手机改为上下两行，图标、标题、说明和空白区域都能点选。前者默认不选，后者默认选中；两项都可取消。授权成功后上传并启用凭证，但已排除的套餐不上传。结果不明时停止；后续失败不会恢复旧密钥。
- **最后看底栏。** 不再用独立灰底流程区重复列举步骤和数量。按钮旁的「确认后」只显示此次选择的顺序，两项都取消时显示「仅重置 2FA」。初始焦点在取消；短窗口只滚动内容，键盘和读屏也能核对选项与摘要。独立「退出所有设备」不出现重置说明或重置摘要。

| 真实面板视口 | 12.1.2（虚构账号） | 12.1.3（同一虚构账号） |
| --- | --- | --- |
| 1440px | [468 × 491，纵向复选框](../artifacts/ui-rework-12.1.2/installed-synthetic-1440.png) | [548 × 419，并排选项与底栏摘要](../artifacts/ui-rework-12.1.3/installed-synthetic-1440.png) |
| 390px | [351 × 507](../artifacts/ui-rework-12.1.2/installed-synthetic-390.png) | [351 × 504，整行选项与常驻按钮](../artifacts/ui-rework-12.1.3/installed-synthetic-390.png) |

另见[暖色](../artifacts/ui-rework-12.1.3/browser/reset-1064-light.png)、[深色](../artifacts/ui-rework-12.1.3/browser/reset-1064-dark.png)、[短窗口长邮箱](../artifacts/ui-rework-12.1.3/browser/reset-short-long-email.png)和[账号状态变化](../artifacts/ui-rework-12.1.3/browser/reset-short-error.png)。截图不含实际账号或密钥。

## 验证与交付

- [浏览器回归](../artifacts/ui-rework-12.1.3/browser/verification.json)：98 组布局，其中 30 组专测重置卡片。检查单双列切换、整块点击区域、键盘与焦点、四种选择及底栏位置、长邮箱和过期版本错误；无管理写请求。另有 [34 项前端测试](../artifacts/ui-rework-12.1.3/frontend-tests.log)通过。
- [宿主基线](../artifacts/ui-rework-12.1.3/host-baseline.json)校验 2,413 个源文件。[宿主编译与测试](../artifacts/ui-rework-12.1.3/host-check.log)复用同基线缓存，[WebRTC 测试](../artifacts/ui-rework-12.1.3/host-live.log)在隔离私有网卡运行。[插件及引擎测试、vet 和构建](../artifacts/ui-rework-12.1.3/native-build.log)与[隔离宿主加载、重启](../artifacts/ui-rework-12.1.3/native-smoke.json)均通过。[Linux arm64 安装包](../artifacts/ui-rework-12.1.3/plugin-package/model-quality-v0.18.4-linux-arm64.zip)仅含插件库。
- [8317 部署](../artifacts/ui-rework-12.1.3/deployment.json)：预检确认无活动任务或自动化，停服备份校验后只替换插件库。1 个插件账号、2 条历史任务、日志、设置及 1 个 CPA 凭证与升级前一致。宿主镜像、配置与凭证文件未变，6 份在线静态资源与安装包一致。
- [实际 iframe 核验](../artifacts/ui-rework-12.1.3/deployed-iframe.json)：从 8317 真实面板加载已安装资源，浏览器中的插件业务读取只使用内存 fixture。桌面、手机都核对了版本、勾选响应与按钮无遮挡。没有执行真实重置、退出、账号登录或模型调用。
