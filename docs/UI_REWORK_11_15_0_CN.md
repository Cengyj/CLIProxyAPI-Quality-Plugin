# UI 11.15.0：CPA 套餐与状态徽章

已于2026-11-17 **20:11:48（Asia/Shanghai）** 部署到 [8317插件页面](http://127.0.0.1:8317/management.html#/plugin-pages/model-quality/0)：**UI11.15.0 / 插件0.17.1 / 运行时2.1.2**。

套餐依据 [CPA真实组件研究](../artifacts/ui-rework-11.15.0/CPA_BADGE_REFERENCE_CN.md) 与 [原始CSS/JS摘录](../artifacts/ui-rework-11.15.0/cpa-badge-source-evidence.json)：指定的Go宿主源码不含前端组件，实际样式来自8317提供的独立管理页面。列表使用22px高的“套餐＋名称”胶囊，名称为11px等宽字体；Pro／Pro Lite金色，其他套餐包括Business Premium使用中性色，未知显示“套餐 未知”。上传排除选项保持30px原生复选框胶囊及相同配色，套餐枚举、授权快照和写入策略不变。

CPA状态与最近操作统一为22px圆点徽章，成功绿、失败红、待处理琥珀、未知中性；账号身份、CPA状态、最近操作各占48px双行区。仅 `awaiting_inspection` 配合 `unknown` 或 `not_checked` 显示“待巡检”，其他未知状态不变。时间靠近操作状态，600px及以下隐藏日期、360px及以下隐藏行内时间，完整值保留在提示与详情中。服务状态折叠区和测试底部说明继续移除。

[布局测量](../artifacts/ui-rework-11.15.0/layout-verification.json) 确认六条桌面账号的套餐、CPA状态及操作徽章均高22px，三列双行区均高48px且垂直对齐；套餐名称未截断。390px窄屏的关键状态和套餐完整，320px检查无横向溢出或关键文字截断，浅色、白色、深色检查通过。见 [桌面](../artifacts/ui-rework-11.15.0/desktop.jpg)、[深色](../artifacts/ui-rework-11.15.0/dark.jpg) 与 [手机](../artifacts/ui-rework-11.15.0/mobile.jpg) 截图。

8317 实际页面的 [测量](../artifacts/ui-rework-11.15.0/deployed-layout-verification.json) 与 [账号列表截图](../artifacts/ui-rework-11.15.0/deployed-accounts.jpg) 已保存，用于核对 Business Premium 的中性套餐样式、待巡检与操作成功的区分及两行对齐。

[原生测试、vet与构建](../artifacts/ui-rework-11.15.0/package-verification.json)、[部署](../artifacts/ui-rework-11.15.0/deployment.json) 与 [重启核验](../artifacts/ui-rework-11.15.0/verification-after-restart.json) 通过：六份UI摘要匹配、未鉴权返回401，1个账号、2条历史任务、1个CPA凭证及全部设置快照完整相等；运行/规则/动作均为0。配置、凭证文件和宿主镜像不变，未进行真实账号操作或模型调用。安装包：[Linux arm64 ZIP](../artifacts/ui-rework-11.15.0/plugin-package/model-quality-v0.17.1-linux-arm64.zip)，核对 [清单](../artifacts/ui-rework-11.15.0/plugin-package/build-manifest.json) 的 `ui_version=11.15.0` 与 [SHA256SUMS](../artifacts/ui-rework-11.15.0/plugin-package/SHA256SUMS)。
