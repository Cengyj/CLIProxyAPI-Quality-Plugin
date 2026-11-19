# UI 12.2.0：重置 2FA 的左右分区与移动端排版

2026-11-18 20:12（北京时间），[8317 官方管理面板](http://127.0.0.1:8317/management.html#/plugin-pages/model-quality/0)已升级为 **UI 12.2.0 / 插件 0.18.5**。官方宿主 CPA 8.0.76、内嵌引擎 2.1.3 保持原版本。刷新整个标签页，在账号行打开「重置 2FA」；连接信息可核对 UI 版本。

完整的内容分析、行为对照、排版规范及实施方案见 [重置 2FA 专项设计](RESET_2FA_REDESIGN_CN.md)。

## 最终排版

| 区域 | 新版处理 | 目的 |
| --- | --- | --- |
| 标题 | 安全图标、标题与简短副标题，关闭按钮独立 | 明确当前操作，避免把账号及全部说明挤在标题下 |
| 左侧说明 | 当前账号完整邮箱、旧密钥失效、新密钥确认后保存 | 把操作对象及必然影响集中在同一区域 |
| 右侧选项 | 两条纵向复选项，统一标题、说明与勾选框位置 | 明确这是两个独立的可选操作；说明文字由 12px 提升到 13px |
| 选择时序 | 「可选 · 新密钥保存后执行」 | 表达后续操作依赖已确认的新密钥保存，套餐排除只影响上传 |
| 失败说明 | 结果不明会停止，后续失败不会恢复旧密钥 | 保留操作边界，避免误以为整组事务可以回滚 |
| 底栏 | 实际选中顺序、取消与确认；选择变化时位置稳定 | 提交前再次核对本次范围，短窗口也能取消 |

主体宽度 680px，内容区按约 0.76:1 分配。卡片内部宽度不超过 570px 时，账号/影响与后续选项改为上下排列；旧／新密钥保持 13px 可读正文。340px 以下去除选项中的装饰图标，优先保留说明文字。标题在桌面 22px、最窄卡片 20px，选项标题 14px；沿用现有三种主题、中性选中态、细边框和圆角。

本轮换取更清晰的内容分区与更大的说明字号，手机内容比 12.1.3 更高；高度不足时只滚动主体，确认栏始终可见。另清除了上一版散落在通用样式区的 330px 重置规则，避免新副标题被挤成竖排。

截图均为虚构账号：[上一版桌面](../artifacts/ui-rework-12.1.3/installed-synthetic-1440.png) · [本版 8317 桌面](../artifacts/ui-rework-12.2.0/installed-synthetic-1440.png) · [本版 8317 手机](../artifacts/ui-rework-12.2.0/installed-synthetic-390.png) · [暖色](../artifacts/ui-rework-12.2.0/browser/reset-1064-light.png) · [深色](../artifacts/ui-rework-12.2.0/browser/reset-1064-dark.png) · [320px](../artifacts/ui-rework-12.2.0/browser/reset-320-light.png) · [短窗口错误](../artifacts/ui-rework-12.2.0/browser/reset-short-error.png)。

## 行为与验证

退出设备默认不选，重新授权默认选中；两项都取消时显示「仅重置 2FA」。保留完整邮箱、原生复选框、取消优先焦点、关闭后的焦点返回、账号版本检查、提交期间互斥、不确定结果阻断和现有请求结构。独立退出保留自身授权选项和流程，不出现重置影响。

- [106 组浏览器布局检查](../artifacts/ui-rework-12.2.0/browser/verification.json)通过，其中 38 组针对重置：主题、单双栏临界宽度、390/320px、四种选择不位移、长邮箱、短窗口、过期状态、键盘和关闭重开。另有 [34 项前端测试](../artifacts/ui-rework-12.2.0/frontend-tests.log)通过。
- [提交交互验证](../artifacts/ui-rework-12.2.0/browser/security-submit.json)：四种组合的实际表单请求参数、独立退出、忙碌时禁止二次提交、Esc 保护和结果不明后阻断均通过。使用临时虚构账号，六次模拟写请求都在浏览器中截获，向服务转发的写请求为零。
- [官方宿主基线](../artifacts/ui-rework-12.2.0/host-baseline.json)核对 2,413 个文件；[宿主编译与测试](../artifacts/ui-rework-12.2.0/host-check.log)使用同基线缓存，[WebRTC 测试](../artifacts/ui-rework-12.2.0/host-live.log)使用隔离私有网络。插件、引擎的[测试、vet 与原生构建](../artifacts/ui-rework-12.2.0/native-build.log)以及[隔离加载、重启](../artifacts/ui-rework-12.2.0/native-smoke.json)通过。
- [部署核验](../artifacts/ui-rework-12.2.0/deployment.json)：确认无活动任务或自动化，停服备份后仅替换插件库。1 个插件账号、2 条历史任务、日志、设置及 1 个 CPA 凭证保留；配置、凭证文件、宿主镜像未变。六份在线 UI 资源摘要与包一致。
- [实际面板核验](../artifacts/ui-rework-12.2.0/deployed-iframe.json)：加载 8317 已安装代码，插件业务读取由临时内存 fixture 提供，检查桌面/手机分区、版本、选项反馈及确认按钮无遮挡。没有真实重置、退出、账号登录或模型调用。

交付：[Linux arm64 插件 ZIP](../artifacts/ui-rework-12.2.0/plugin-package/model-quality-v0.18.5-linux-arm64.zip) · [构建清单](../artifacts/ui-rework-12.2.0/plugin-package/build-manifest.json) · [相对 12.1.3 的 UI 差异](../artifacts/ui-rework-12.2.0/ui-source.diff)。
