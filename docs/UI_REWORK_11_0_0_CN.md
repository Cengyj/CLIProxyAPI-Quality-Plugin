# UI 11.0.0：与实际 CPA 一致的界面与部署（历史）

日期：2026-11-17。状态：**曾部署并完成重启核验，现已被 UI 11.1.0 替代**。本文保留当时 CPA 8.0.76 / 插件 0.16.0 / UI 11.0.0 / Go＋Sobek 2.1.1 的改动与证据；当前版本见 [UI 11.1.0 说明](UI_REWORK_11_1_0_CN.md)。当次宿主源码及运行镜像保持不变。

入口：[CPA 内嵌插件页面](http://127.0.0.1:8317/management.html#/plugin-pages/model-quality/0)。安装包：[Linux arm64 ZIP](../artifacts/ui-rework-11.0.0/plugin-package/model-quality-v0.16.0-linux-arm64.zip)，内含单个 `model-quality.so`。后端版本未递增，安装前同时核对 [构建清单](../artifacts/ui-rework-11.0.0/plugin-package/build-manifest.json) 中的 `ui_version` 与 [SHA256SUMS](../artifacts/ui-rework-11.0.0/plugin-package/SHA256SUMS)。

## 配色与布局

本轮读取实际运行的 CPA 面板样式，保存于 [host-styles.css](../artifacts/ui-rework-11.0.0/host-styles.css)，按页面、卡片、输入框、悬浮层、文字和边框的用途映射到插件。此前自定义蓝色改为宿主现有的暖灰体系，保留浅色、纯白、深色三种主题；内嵌页继续跟随宿主公开的主题属性。

| 用途 | 浅色 | 纯白 | 深色 |
| --- | --- | --- | --- |
| 页面与输入框 | `#faf9f5` | `#ffffff` | `#151412` |
| 卡片 | `#f0eee8` | `#ffffff` | `#1d1b18` |
| 悬浮层 | `#fffdf9` | `#ffffff` | `#2a2723` |
| 主文字 | `#2d2a26` | `#2d2a26` | `#f6f4f1` |
| 辅助文字 | `#6d6760` | `#6d6760` | `#c9c3bb` |
| 边框 | `#e3e1db` | `#e5e5e5` | `#3a3530` |
| 主操作 | `#8b8680` | `#8b8680` | `#8b8680` |

| 区域 | 本轮调整 |
| --- | --- |
| 导航与共享组件 | 主导航使用中性分组与选中背景，统一按钮、表单、卡片、状态和弹窗；按实际 CPA 工具栏预留空间，移动断点对齐 768px |
| 账号列表 | 导入及代理设置按钮、搜索、状态筛选、表格和分页统一放入列表面板；保留账号、CPA 状态、最近操作、操作四列和行内 2FA 登录、资料、更多入口；窄屏改为标签分组 |
| 智商测试 | 从 iframe 内容宽度 960px 起，320px 设置栏与回答区域并排；更窄时上下排列；开始测试仍关联原表单，展开阅读保留原草稿 |
| 质量观察 | 凭证状态收敛为“凭证、可用状态、限制说明”三列，清理重复能力文案；规则、审计、历史暂停归属及官方观察模式继续保留 |
| 文案与信息层次 | 主流程移除参考版本标签；编辑题目仍提示“已修改”。登录引擎移至设置的“服务状态”，任务编号与机器错误代码收进“排障信息”，代理格式与容器网络说明按需展开 |

隐藏或收起实现文案不等于删除审计数据。冻结配置、身份版本、指纹、关联规则、参考答案和原始 JSON 仍保存在既有详情、证据和导出路径中。错误的可读原因、恢复建议、危险操作后果和自动修复权限继续明确展示。原有 **251 个控件 ID 全部保留**，`manual`、`operations`、`login` 路由及事件挂接点保持有效。

一步账号登录、历史候选确认、未知提交隔离、安全操作新密钥优先保存、敏感草稿清理、读取恢复和禁止写入自动重试的语义保持不变。HTML/SVG 预览仍使用空 sandbox 与严格内容安全策略。官方宿主不提供独立质量暂停、释放或自动暂停；界面调整未启用这些能力，也不使用人工禁用来模拟。

## 验证与证据

| 检查 | 已记录结果 |
| --- | --- |
| 客户端与导入回归 | 16 项通过，见 [node-tests.log](../artifacts/ui-rework-11.0.0/node-tests.log) |
| 题目与结果 | 题目绑定、人工结果、转义和规则视图兼容通过，见 [question-tests.log](../artifacts/ui-rework-11.0.0/question-tests.log) |
| 原生构建 | Linux arm64 登录引擎测试、插件测试、`go vet` 和共享库构建通过，见 [native-build.log](../artifacts/ui-rework-11.0.0/native-build.log) |
| 宿主基线 | 2,413 个记录文件在原始目录与构建快照中均无变更、无缺失；构建快照无额外文件，见 [source-baseline-verification.json](../artifacts/ui-rework-11.0.0/source-baseline-verification.json) |
| 实际运行资源 | 插件注册、启用、运行时 2.1.1 可用；六份静态资源摘要与本次源码一致，未鉴权管理请求返回 401 |
| 再次重启 | 数据快照、配置/凭证摘要和镜像核对继续通过，见 [重启前核验](../artifacts/ui-rework-11.0.0/verification-before-restart.json) 与 [重启后核验](../artifacts/ui-rework-11.0.0/verification-after-restart.json) |

本轮已有页面记录：[回答区域](../artifacts/ui-rework-11.0.0/answers-desktop.jpg)、[质量观察](../artifacts/ui-rework-11.0.0/quality-desktop.jpg)。截图只呈现相应页面状态，不替代上述静态资源与运行状态核验，也不证明真实账号认证或模型调用结果。浏览器尺寸、溢出和控制台结论应以实际检查记录为准，本文不从截图推断未记录的结果。

## 部署与数据保留

[deployment.json](../artifacts/ui-rework-11.0.0/deployment.json) 记录部署完成时间为 **2026-11-17 06:43:48 UTC（14:43:48 Asia/Shanghai）**。沿用 `cli-proxy-api` 容器和原镜像；停机后完整备份 70 个运行文件，再替换插件共享库、启动、核验并再次重启。私有备份路径与摘要保存在部署记录中，不放入公开安装包。

[升级脚本](../artifacts/ui-rework-11.0.0/upgrade_container.py) 在停机前校验安装库摘要、`ui_version=11.0.0` 和当前六份 UI 源文件摘要，防止陈旧包进入替换步骤；启动检查等待插件注册、启用及内嵌运行时就绪。原有运行中任务、自动化和未知凭证写入检查，以及失败时恢复旧共享库的流程继续保留。

升级前后保留 **1 个插件账号**，登录任务、模型运行、规则、暂停归属和 CPA 凭证均为 0。账号、任务、设置等逻辑快照保持一致，配置与凭证文件内容未变，持久化文件保留，宿主镜像未变。宿主 KV 的启动事务和加载器缓存不以物理字节不变作为账号保留依据。

未执行真实登录、模型调用、重置 2FA 或退出所有设备，也未重放历史任务。UI 10.0.0 的部署和截图作为 [历史记录](UI_REWORK_10_0_0_CN.md) 保留；当时入口使用 UI 11.0.0，现已由 UI 11.1.0 替代。

## 正式页面核验

已在 `http://127.0.0.1:8317/management.html#/plugin-pages/model-quality/0` 核验最终部署。1280×800 窗口下插件内容宽 1064px，账号列表可视宽与滚动宽均为 974px，没有横向溢出；主色为宿主的 `#8b8680`。连接弹窗可正常打开，设置页服务状态显示“2FA 登录可用”。测试配置与回答左右并排，题目版本展示为空，未显示参考项目标识。浏览器未报告控制台错误。

![正式 CPA 账号页面](../artifacts/ui-rework-11.0.0/deployed-cpa-accounts.jpg)

![正式 CPA 测试页面](../artifacts/ui-rework-11.0.0/deployed-cpa-manual.jpg)

最终复核恢复了内嵌页的连接入口，并让服务状态依据实际 2FA 能力显示；修正后重新构建部署，最终摘要以 [部署记录](../artifacts/ui-rework-11.0.0/deployment.json) 为准，第一次 UI11 部署证据保留在 `initial-deployment/`。
