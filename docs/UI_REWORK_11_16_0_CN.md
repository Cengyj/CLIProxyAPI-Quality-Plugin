# UI 11.16.0：恢复 toSub2 套餐配色

已于2026-11-17 **20:24:46（Asia/Shanghai）** 部署到 [8317插件页面](http://127.0.0.1:8317/management.html#/plugin-pages/model-quality/0)：**UI 11.16.0 / 插件0.17.1 / 运行时2.1.2**。

按用户最新偏好恢复 [toSub2 PlanBadge](/Users/ceng/Downloads/toSub2-go-port-2026.11.17/web/src/components/PlanBadge.jsx) 与 [样式配色](/Users/ceng/Downloads/toSub2-go-port-2026.11.17/web/src/styles.css:617)：Pro 银色，Pro Lite／Business Premium 金色，Plus 蓝、Go 绿、Team 紫、Enterprise 灰蓝、Edu 玫瑰。金色使用参考项目的深色覆盖，普通套餐与主题混色，未知继续使用虚线边框。

保持当前22px套餐胶囊、48px两行对齐、状态显示和业务语义；上传排除选项仍为30px原生复选框胶囊，选中时加强边框并保留套餐背景。此次仅修改配色与隐藏版本标记，不改变套餐枚举、授权快照、排除策略或账号状态。

[本轮配色测量](../artifacts/ui-rework-11.16.0/layout-verification.json) 核对浅色、深色列表与上传排除选项：已显示套餐均高22px、名称无溢出，Pro 保持银色，Business Premium 与 Pro Lite 使用金色；选中边框不覆盖套餐背景，控制台无错误。见 [浅色列表](../artifacts/ui-rework-11.16.0/desktop.jpg)、[深色列表](../artifacts/ui-rework-11.16.0/dark.jpg)、[浅色排除选项](../artifacts/ui-rework-11.16.0/plan-options-light.jpg) 与 [深色排除选项](../artifacts/ui-rework-11.16.0/plan-options-dark.jpg)。本轮未保存设置或执行账号操作；三套主题与390/320px几何测量属于 [UI 11.15保留布局的验证](UI_REWORK_11_15_0_CN.md)，没有作为本轮重测。

8317 实际页面的 [测量](../artifacts/ui-rework-11.16.0/deployed-layout-verification.json) 和 [账号列表截图](../artifacts/ui-rework-11.16.0/deployed-accounts.jpg) 确认已加载 UI 11.16.0：Business Premium 使用参考金色渐变、套餐高22px，账号身份／CPA状态／最近操作三列双行区均高48px且起点对齐，控制台无错误。

[原生测试、vet与构建](../artifacts/ui-rework-11.16.0/package-verification.json) 通过，包与内嵌运行时摘要、六份UI源文件摘要匹配。[宿主基线](../artifacts/ui-rework-11.16.0/source-baseline-verification.json) 确认原目录和构建快照各2,413个源文件无差异。安装包为 [Linux arm64 ZIP](../artifacts/ui-rework-11.16.0/plugin-package/model-quality-v0.17.1-linux-arm64.zip)，以 [清单](../artifacts/ui-rework-11.16.0/plugin-package/build-manifest.json) 的 `ui_version=11.16.0` 与 [SHA256SUMS](../artifacts/ui-rework-11.16.0/plugin-package/SHA256SUMS) 核对。

[部署记录](../artifacts/ui-rework-11.16.0/deployment.json) 与 [重启核验](../artifacts/ui-rework-11.16.0/verification-after-restart.json) 通过：停服备份完成、插件已注册启用、六份UI资源摘要匹配、未鉴权管理请求返回401。开始核验时的1个账号、2条历史任务、1个CPA凭证与全部设置逻辑快照完全相同，没有默认字段例外；运行／规则／动作均为0。配置、凭证文件和宿主镜像保持不变；没有执行真实登录、重置2FA、退出设备或模型调用。
