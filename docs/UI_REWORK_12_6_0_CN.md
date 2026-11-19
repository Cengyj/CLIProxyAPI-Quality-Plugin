# UI 12.6.0：重置 2FA 卡片与执行流程

2026-11-18 21:21（北京时间），8317 已部署 UI 12.6.0 / 插件 0.18.9。原 1 个账号、2 条任务和 1 个 CPA 凭证及设置、日志均保留，宿主镜像、配置、凭证文件未变。未执行真实账号操作或模型调用。

本版针对重置确认卡片重构内容与控件，保持插件原有暖灰、白色及深色配色。账号数据、提交参数和安全操作执行语义保持原有约束。

## 最终内容与排版

1. **账号区**：盾牌标记、标题、完整邮箱和圆形关闭按钮。邮箱在窄屏完整换行。
2. **影响提示**：账号下方直接说明旧验证码失效、新密钥确认后保存。按用户最新要求移除内部上方的密钥对照卡片，不保留它的边框、背景或占位。
3. **后续操作**：设备退出和 CPA 授权使用独立开关、功能图标及简短结果说明。整行可点击；选中、未选中、键盘焦点及禁用有对应反馈。退出默认关闭，授权默认开启。
4. **操作须知**：折叠展示新密钥保存、上传排除和未知结果处理。每次重新打开恢复折叠状态。
5. **执行流程**：底部采用圆形序号、连接线和步骤名称。仅呈现本次所选步骤，按重置、退出、授权的实际顺序排列；关闭两项后只显示「仅重置 2FA」。节点、数量和辅助技术摘要同步更新。它表示确认后的执行计划，不表示操作已完成。
6. **确认区**：取消与确认按钮独立排列。切换任意选项不移动卡片和按钮。短窗口只滚动内容，流程和按钮持续可见。

全部颜色使用现有主题变量；使用新的开关、圆形关闭控件和节点流程保留组件区分。默认打开不会发送任务；只有确认才提交。

## 核验

- [浏览器核验](../artifacts/ui-rework-12.6.0/browser/verification.json)：111 组布局、37 张截图，覆盖三主题、桌面和手机、320px 窄屏、完整长邮箱、短窗口、展开说明、四种组合、键盘、焦点恢复、过期账号版本和独立退出操作。
- [提交核验](../artifacts/ui-rework-12.6.0/browser/security-submit.json)：四种重置参数、独立退出、重复提交保护及结果不明后的阻断通过。全部模拟写入在浏览器截获，未转发网络。
- [原生构建](../artifacts/ui-rework-12.6.0/native-build.log)：引擎及插件模块测试、插件 vet 和共享库构建通过；[隔离宿主检查](../artifacts/ui-rework-12.6.0/native-smoke.json)核对注册、鉴权、六份嵌入资源与重启。
- [宿主基线](../artifacts/ui-rework-12.6.0/host-baseline.json)：原始目录、既有构建快照和临时构建目录均核对 2,413 个文件。宿主未修改；本轮相同基线已通过的宿主编译、测试及 WebRTC 私网测试按[复用记录](../artifacts/ui-rework-12.6.0/verification-reuse.json)保留，34 项既有前端模块测试同理。
- 实际安装结果见[升级记录](../artifacts/ui-rework-12.6.0/deployment.json)，实际面板布局见[iframe 核验](../artifacts/ui-rework-12.6.0/deployed-iframe.json)。面板截图使用虚构账号；插件业务读取来自隔离内存 fixture，写请求全部阻断。

## 截图与交付

[桌面默认](../artifacts/ui-rework-12.6.0/installed-synthetic-1440.png) · [桌面完整流程](../artifacts/ui-rework-12.6.0/installed-synthetic-1440-full-flow.png) · [手机](../artifacts/ui-rework-12.6.0/installed-synthetic-390.png) · [手机完整流程](../artifacts/ui-rework-12.6.0/installed-synthetic-390-full-flow.png) · [深色](../artifacts/ui-rework-12.6.0/browser/reset-1064-dark.png)。

[Linux arm64 安装包](../artifacts/ui-rework-12.6.0/plugin-package/model-quality-v0.18.9-linux-arm64.zip) · [构建清单](../artifacts/ui-rework-12.6.0/plugin-package/build-manifest.json) · [相对 12.5.0 的源码差异](../artifacts/ui-rework-12.6.0/ui-source.diff)。
