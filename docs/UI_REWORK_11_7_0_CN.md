# UI 11.7.0：账号行精简与套餐上传策略

> 本文保留 UI 11.7.0 在插件 0.17.0 下的界面与部署记录。当前插件已升级为 [0.17.1 / Go＋Sobek 2.1.2](WORKSPACE_SELECTION_CN.md)，六份 UI 源文件不变，以下界面证据继续适用。

已部署到 [8317 插件页面](http://127.0.0.1:8317/management.html#/plugin-pages/model-quality/0)，版本为 **CPA 8.0.76 / 插件 0.17.0 / UI 11.7.0 / Go＋Sobek 2.1.1**。宿主源码和镜像保持不变；[UI 11.6.0](UI_REWORK_11_6_0_CN.md) 为历史记录。

## 账号行展示

- 账号身份收敛为两行：第一行邮箱，第二行套餐徽标与备注或登录策略。长邮箱只在列表里省略显示，完整值仍在提示和账号资料中。
- 最近操作收敛为两行：第一行状态与时间，第二行操作和原因摘要。点击摘要查看原有行内日志；待确认授权的状态仍可进入任务核对。完整摘要保留在提示和无障碍名称中。
- CPA 状态同样限定两行：状态标签与一行补充说明；提示保留完整绑定 ID、状态原因、最近及下次巡检时间。套餐徽标沿用 CPA 的中性与强调色，避免重新引入独立配色。
- 账号操作列保留 [UI 11.5 的两行三列按钮](UI_REWORK_11_5_0_CN.md)，工具栏保留 [UI 11.6 的响应式排版](UI_REWORK_11_6_0_CN.md)。

## 套餐来源与字段

账号、登录任务、候选授权分别通过 `Account.plan_type`、`Job.plan_type`、`Candidate.plan_type` 返回统一值：`free`、`go`、`plus`、`pro`、`prolite`、`business_premium`、`team`、`enterprise`、`edu`、`unknown`。页面显示 Free、Go、Plus、Pro、Pro Lite、Business Premium、Team、Enterprise、Edu 与“未知套餐”。

套餐来自已获得的授权元数据或经身份核对的本地 CPA 绑定。读取页面不会查询远端订阅。缺失、未知或无法识别的值保持 `unknown`，不能默认为 Free，也不代表当前订阅实时有效。账号搜索包含显示的套餐名称；候选核对页同时显示授权套餐。

## CPA 上传排除设置

“登录设置”页增加 **CPA 上传排除套餐**，字段为 `cpa_upload_excluded_plans`，默认 `[]`。勾选后保存才生效；未提供该字段的旧客户端保留现有设置。非法套餐值会被拒绝，合法别名归一化并去重。

本轮按用户要求，对本插件**每一次 CPA 凭证上传或更新**应用排除策略：账号行一键登录、历史候选手动确认、定时自动重登，以及重置 2FA 或退出设备后选择重新授权的写入。参考项目只在自动修复流程使用套餐排除；本插件覆盖范围更广，并非声称参考已有此行为。参考证据来自只读目录 `/Users/ceng/Downloads/toSub2-go-port-2026.11.17/`：`web/src/components/shell.jsx:570` 的“自动重登排除套餐”控件、`:583` 的“不限制手动重登或手动上传”，以及 `internal/jobs/cpa_auto_repair.go:81` 附近只对自动修复生效的检查；套餐归一化和多来源排除分别见 `internal/accountplan/plan.go` 与 `internal/accountplan/policy.go`。

本地账号授权快照、新候选授权、当前身份匹配的 CPA 凭证提供的已知套餐都会参与检查，任一命中即阻止写入；只有各来源都无已知套餐时，才按 `unknown` 判断。在进入提交及实际写入前重新核对最新设置。官方回调仍不提供跨进程原子比较更新，不能将这些复查描述为原子保障。

命中时显示“套餐已排除”，任务保留为当前会话内可核对的候选授权，不写入或启用 CPA。修改设置不会自动补传；候选仍有效时可明确确认，或放弃保存。候选过期、重启或历史任务不会自动重放上传。排除策略不撤销已完成的账号安全动作：已确认的新 2FA 密钥仍保存，未知安全结果仍保留隔离状态。

## 升级与验证边界

仅允许新增默认字段的快照兼容：在升级前快照的账号和任务缺少 `plan_type` 时补 `unknown`，在设置缺少 `cpa_upload_excluded_plans` 时补 `[]`；升级后响应必须与该预期完整相等。不能移除字段比较，不能把已经存在的 `null`、空串或已知套餐替换为默认值。候选详情不嵌套在现有任务列表快照里。

本轮证据目录为 `artifacts/ui-rework-11.7.0/`。验证使用合成账号与离线上游，不执行真实登录、模型请求、重置 2FA 或退出设备。

### 界面验证

[布局测量](../artifacts/ui-rework-11.7.0/layout-checks.json) 覆盖 1064、840、390 和 320px 四种视口：六条合成账号的身份区均为 45px、最近操作区均为 41px，操作按钮保持 29px。各视口套餐名称均能完整显示，记录的文档宽度未超出视口。light、white、dark 三套主题均已检查。截图包括 [桌面账号列表](../artifacts/ui-rework-11.7.0/fixture-accounts.jpg)、[手机账号列表](../artifacts/ui-rework-11.7.0/mobile-accounts.jpg) 和 [套餐排除设置](../artifacts/ui-rework-11.7.0/fixture-upload-settings.jpg)。

[验证汇总](../artifacts/ui-rework-11.7.0/verification-summary.json) 记录合成界面交互：套餐排除保存及重新读取、命中排除后的任务入口、清空排除后明确确认提交成功。Node 客户端与导入回归 16 项通过。合成提交仅说明插件界面与离线 fixture 的交互，不代表真实授权或 CPA 写入验证。

### 后端、构建与部署

后端套餐专项与 race 检查通过（同见验证汇总），覆盖套餐归一化和未知值、设置的保留/清空/持久化、已知套餐多来源排除、候选保留与明确重试、自动修复权限、当前绑定套餐展示，以及安全操作后已确认密钥保留。

[原生构建日志](../artifacts/ui-rework-11.7.0/native-build.log) 记录登录运行时与插件模块测试、`go vet` 及 Linux arm64 原生打包。新安装包为 [model-quality-v0.17.0-linux-arm64.zip](../artifacts/ui-rework-11.7.0/plugin-package/model-quality-v0.17.0-linux-arm64.zip)；核对 [构建清单](../artifacts/ui-rework-11.7.0/plugin-package/build-manifest.json) 中的 `ui_version=11.7.0` 和 [SHA256SUMS](../artifacts/ui-rework-11.7.0/plugin-package/SHA256SUMS)。[宿主基线](../artifacts/ui-rework-11.7.0/source-baseline-verification.json) 确认原始 CPA 与构建快照的 2,413 个记录文件摘要各自保持一致。

[部署记录](../artifacts/ui-rework-11.7.0/deployment.json) 确认 **2026-11-17 17:18:21（Asia/Shanghai）** 完成升级。已做停服备份，并通过 [首次启动核验](../artifacts/ui-rework-11.7.0/verification-before-restart.json) 和 [再次重启核验](../artifacts/ui-rework-11.7.0/verification-after-restart.json)：插件注册及启用、内嵌运行时可用、六份静态资源摘要与源码一致，未鉴权管理接口返回 401。

保留 **1 个插件账号、1 条既有终态登录任务**，模型运行、规则、暂停归属与 CPA 凭证均为 0。逻辑快照仅接受本报告所列新增默认字段；账号、其他设置与记录一致，配置/凭证文件及宿主镜像均未改变。未执行真实账号操作或模型调用。

最近操作第二行去除重复的阶段名称。实际 CPA 页面截图见 [账号列表](../artifacts/ui-rework-11.7.0/deployed-cpa-accounts.jpg) 和 [登录设置](../artifacts/ui-rework-11.7.0/deployed-upload-settings.jpg)。已核对 UI 11.7.0、两行摘要和默认未勾选的套餐排除项；此检查未修改真实设置。`initial-*` 文件保留此前部署核验，以上 `deployment.json` 与重启核验均对应最终资源。
