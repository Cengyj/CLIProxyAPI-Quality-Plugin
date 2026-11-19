# UI 11.5.0：按参考项目重排账号操作列

> 历史记录：当前已部署 UI 11.6.0；账号操作列沿用本文设计，工具栏更新见 [UI 11.6.0](UI_REWORK_11_6_0_CN.md)。

已部署到 [8317 插件页面](http://127.0.0.1:8317/management.html#/plugin-pages/model-quality/0)，版本为 **CPA 8.0.76 / 插件 0.16.0 / UI 11.5.0 / Go＋Sobek 2.1.1**。

本轮对照只读项目 `/Users/ceng/Downloads/toSub2-go-port-2026.11.17` 的 `web/src/components/accounts.jsx`（约 470 行）、`account-actions.css` 和 `docs/pure-go-runtime/ui/row-actions-polished.jpg`，采用两排三列：

| 第一列 | 第二列 | 第三列 |
| --- | --- | --- |
| 2FA 登录／重新登录／当前状态 | 重置 2FA | 删除 |
| 账号资料 | 日志 | 更多 ⌄ |

目标项目的独立上传位置改放账号资料：本插件登录成功即保存并启用 CPA 凭证，不增加重复上传步骤。更多菜单保留查看任务、巡检此账号、退出所有设备，移除与行内重复的重置、删除入口。

按钮统一 **29px** 高，手机端也保持此尺寸；字号 **10.5px**、图标 **14px**、间距 **6px**、圆角 **8px**。列比例为 **1.12:1.12:0.8**，操作列 242px、按钮组 218px。浅边框及底色使用 CPA 配色，删除使用可读的警示文字色；`appearance: none` 消除 WebKit 原生按钮厚边框。

重置与删除按钮只打开既有确认窗口。重置动态复查能力、进行中/待核对状态、资料与代理错误；删除保留任务状态检查和二次确认。取消后焦点返回相应行按钮，读取失败后重试保留入口来源。日志和更多菜单保留只读访问逻辑。

## 验证与部署

- 1064px 桌面与 390px 手机六行合成账号均测得约 **75.89/75.89/54.22px** 列宽、统一 29px 高，上下对应列对齐，无裁切或横向溢出：[桌面测量](../artifacts/ui-rework-11.5.0/desktop-action-measurements.json) · [手机测量](../artifacts/ui-rework-11.5.0/mobile-action-measurements.json)。
- [交互检查](../artifacts/ui-rework-11.5.0/interaction-checks.json) 确认重置与删除窗口取消后返回原按钮，日志展开收起、菜单 Escape 返回正常；退出设备窗口取消返回更多入口。[合成环境核验](../artifacts/ui-rework-11.5.0/fixture-readonly-verification.json) 确认写请求为 **0**，账号与任务数量未变，未实际执行安全或删除操作。
- 语法检查、Node 客户端/导入 **16 项测试**通过，见 [前端测试日志](../artifacts/ui-rework-11.5.0/node-tests.log)。登录运行时、插件测试、`go vet` 与 Linux arm64 原生库构建通过，见 [构建日志](../artifacts/ui-rework-11.5.0/native-build.log)。
- 原始 CPA 与构建副本的 **2,413** 个基线文件均一致，构建副本无额外文件，见 [宿主基线核验](../artifacts/ui-rework-11.5.0/source-baseline-verification.json)。

[部署记录](../artifacts/ui-rework-11.5.0/deployment.json) 确认 **2026-11-17 16:30:30（Asia/Shanghai）** 完成更新。停机备份并替换插件库后，[再次重启核验](../artifacts/ui-rework-11.5.0/verification-after-restart.json) 确认插件启用、运行时可用，六份静态资源与源码一致，未鉴权管理请求返回 401。

升级保留 **1 个插件账号和 1 条既有终态登录任务**；模型运行、规则、暂停归属及 CPA 凭证均为 0。账号和设置逻辑快照、配置与凭证文件、宿主镜像保持一致。验证未执行真实登录、模型调用、重置 2FA、退出设备或删除账号。

[实际 CPA 截图](../artifacts/ui-rework-11.5.0/deployed-cpa-accounts.jpg) · [手机截图](../artifacts/ui-rework-11.5.0/mobile-reference-actions.jpg)

安装包：[Linux arm64 ZIP](../artifacts/ui-rework-11.5.0/plugin-package/model-quality-v0.16.0-linux-arm64.zip)。核对 [构建清单](../artifacts/ui-rework-11.5.0/plugin-package/build-manifest.json) 中的 `ui_version=11.5.0` 与 [SHA256SUMS](../artifacts/ui-rework-11.5.0/plugin-package/SHA256SUMS)。
