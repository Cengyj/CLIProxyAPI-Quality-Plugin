# UI 12.8.0：重置确认卡片细节调整

2026-11-18 21:45（北京时间），8317 已部署插件 0.18.11 / UI 12.8.0。原账号、任务、凭证、设置与日志保留，宿主镜像和配置未变。

按用户要求，本版调整重置 2FA 卡片的邮箱行、选项反馈与流程区域。

- 移除重置卡片的邮箱行及邮件图标，不保留空白占位。提交仍绑定账号列表中选定的 ID 和 revision。独立退出操作继续显示其账号邮箱。
- 功能选项保持 54px 高度，选中、悬停与选中后悬停分别采用主题强调色的不同浓度；禁用选项不出现悬停反馈。浅色、白色及深色都继承现有主题。
- 执行流程增加 1px 边框、10px 圆角和内部留白，保留序号、方向连线和步骤说明。
- 流程下方增加贯穿卡片宽度的分隔线，将取消和确认按钮划为独立操作区。选择切换保持卡片与按钮位置稳定。

## 验证与交付

[111 组布局检查、44 张截图](../artifacts/ui-rework-12.8.0/browser/verification.json)覆盖三个主题、桌面与窄屏、两类悬停反馈、四种选项组合、键盘操作、焦点恢复、短窗口滚动和账号 revision 过期。

[提交保护检查](../artifacts/ui-rework-12.8.0/browser/security-submit.json)通过四种重置参数、独立退出、忙碌防重和未知提交阻断，模拟写入全部被截获。没有对真实账号执行登录、安全操作或模型调用。

[宿主基线](../artifacts/ui-rework-12.8.0/host-baseline.json)验证 2,413 个文件。宿主[编译测试](../artifacts/ui-rework-12.8.0/host-check.log)、[WebRTC 私网检查](../artifacts/ui-rework-12.8.0/host-live.log)、[插件和引擎测试／vet／共享库构建](../artifacts/ui-rework-12.8.0/native-build.log)及[隔离宿主加载与重启](../artifacts/ui-rework-12.8.0/native-smoke.json)通过。

[8317 升级记录](../artifacts/ui-rework-12.8.0/deployment.json) · [实际 iframe 检查](../artifacts/ui-rework-12.8.0/deployed-iframe.json) · [桌面效果](../artifacts/ui-rework-12.8.0/installed-synthetic-1440.png) · [手机完整流程](../artifacts/ui-rework-12.8.0/installed-synthetic-390-full-flow.png)。截图使用合成账号，实际面板业务读取由内存 fixture 提供，写请求全部阻断。

[Linux arm64 安装包](../artifacts/ui-rework-12.8.0/plugin-package/model-quality-v0.18.11-linux-arm64.zip) · [构建清单](../artifacts/ui-rework-12.8.0/plugin-package/build-manifest.json) · [源码差异](../artifacts/ui-rework-12.8.0/ui-source.diff)。
