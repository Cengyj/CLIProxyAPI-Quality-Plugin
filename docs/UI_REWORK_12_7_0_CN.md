# UI 12.7.0：参考选项控件、紧凑排版与操作按钮

2026-11-18 21:37（北京时间），8317 已部署插件 0.18.10 / UI 12.7.0，内嵌引擎仍为 2.1.3。1 个插件账号、2 条历史任务、1 个 CPA 凭证、设置与日志保留，宿主镜像、配置和凭证文件未变。

本版参考 `/Users/ceng/Downloads/toSub2-go-port-2026.11.17` 的 `TotpWorkflowOption`、账号操作列和相关样式。用户明确选择仅参考控件与排版，保留当前「退出所有设备」「重新授权 CPA」两项功能；不拆分重新授权与上传，也不新增独立上传入口。

## 选项与流程

功能选项采用独立薄边框、图标底座和明确的选中反馈。根据用户对臃肿感的后续反馈，进一步压缩为 54px 单行选项块：上下内边距 8px，标题 12px／500 字重，说明 10.5px／16px 行高，图标底座 28px，开关 32×18px。两项间隔 6px，说明精简，320px 窄屏也可完整阅读。整行仍可点击，键盘与禁用反馈保留。

上方密钥对照卡片保持移除。底部延续圆形序号流程，调整为浅色节点、带方向的连接线、操作名称和简短结果说明。步骤只包含选中操作，动态编号；两项均关闭时显示「仅重置 2FA」。流程和确认按钮在四种选择组合间保持固定高度。色彩继续来自插件的暖灰、白色和深色主题变量。

## 账号操作列

两排三列继续按 1.12:1.12:0.8 对齐，间隔 6px。参照目标项目，全部六个按钮使用统一底色和薄边框，恢复第二排完整按钮形态；圆角 8px，桌面高度 29px，字体 10.5px、图标 14px，移动端保留 44px 点击高度。删除保留危险色，日志展开、更多菜单、键盘焦点和禁用状态保持明确。

现有动作仍是登录／重置／删除与资料／日志／更多。目标项目的「上传」位置按插件既有功能使用「账号资料」；当前插件成功授权后已经自动上传并启用，不引入额外确认步骤。

## 验证与材料

- [参考文件摘要](../artifacts/ui-rework-12.7.0/reference-source.json)：目标项目仅只读研究；插件使用原生 HTML/CSS/JS 实现。
- [111 组布局与 40 张截图](../artifacts/ui-rework-12.7.0/browser/verification.json)：桌面、手机、三主题、窄屏、长邮箱、短窗口、四种组合、流程顺序、键盘、焦点恢复和独立退出。
- [提交保护](../artifacts/ui-rework-12.7.0/browser/security-submit.json)：原四种重置参数、独立退出、忙碌防重和未知提交阻断均通过，模拟写入未转发至网络。
- [34 项前端测试](../artifacts/ui-rework-12.7.0/frontend-tests.log)通过；[宿主基线](../artifacts/ui-rework-12.7.0/host-baseline.json)验证 2,413 个文件；[宿主编译测试](../artifacts/ui-rework-12.7.0/host-check.log)、[WebRTC 私网测试](../artifacts/ui-rework-12.7.0/host-live.log)、[插件与引擎测试、vet、构建](../artifacts/ui-rework-12.7.0/native-build.log)及[隔离加载重启](../artifacts/ui-rework-12.7.0/native-smoke.json)通过。
- [8317 升级记录](../artifacts/ui-rework-12.7.0/deployment.json) · [实际面板核验](../artifacts/ui-rework-12.7.0/deployed-iframe.json)。截图只含虚构账号；未执行真实登录、重置、退出或模型调用。

[桌面卡片](../artifacts/ui-rework-12.7.0/installed-synthetic-1440.png) · [手机完整流程](../artifacts/ui-rework-12.7.0/installed-synthetic-390-full-flow.png) · [桌面操作按钮](../artifacts/ui-rework-12.7.0/installed-actions-1440.png) · [手机操作按钮](../artifacts/ui-rework-12.7.0/installed-actions-390.png)。

[Linux arm64 插件安装包](../artifacts/ui-rework-12.7.0/plugin-package/model-quality-v0.18.10-linux-arm64.zip) · [构建清单](../artifacts/ui-rework-12.7.0/plugin-package/build-manifest.json) · [源码差异](../artifacts/ui-rework-12.7.0/ui-source.diff)。
