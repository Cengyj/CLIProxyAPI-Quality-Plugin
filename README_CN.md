# model-quality 0.15.0 官方 CPA 原生插件

**保留官方 CPA，只安装插件。** 当前插件使用 CPA 8.0.67 已有的原生插件接口，不需要修改 CPA 源码、替换 CPA 程序或安装补丁。Go＋Sobek 登录引擎已经嵌入动态库，无需另装 Node、Chrome 或登录服务。

版本：**插件 0.15.0 / UI 9.2.0 / Go＋Sobek 2.1.0**。当前可安装包为 [macOS arm64 ZIP](artifacts/official-plugin/model-quality-v0.15.0-darwin-arm64.zip)，也可直接安装其中的 `model-quality.dylib`。其他平台需要在目标系统原生构建并验证。

## 功能

- **智商测试**：选择 CPA 凭证、题目、模型及支持的思考强度，手动生成 1–8 份回答；支持自定义题、测试记录、复制、下载和 HTML 预览。打开页面或保存题目不会调用模型。
- **OpenAI 账号管理**：按 `账号----密码----2FA长期密钥` 导入，提供账号列表、资料编辑、行内日志、单账号/默认代理与直接 2FA 登录。2FA 使用目标项目适配的 Go＋Sobek 引擎，不走浏览器自动化。
- **账号安全**：账号行“更多”支持重置 2FA、退出所有设备，可选完成后重新授权并启用 CPA；重置后自动保存新密钥，未知结果不重复修改。
- **账号巡检**：保存后可每 5–1440 分钟检查 CPA 已记录的认证状态，默认 15 分钟且关闭。自动重登需同时启用全局和账号许可，并有明确认证失效证据。从账号列表发起登录后，成功即自动保存并启用 CPA 凭证；已授权的巡检重登沿用同一流程。进度和结果留在账号行，无需跳转或再次确认。
- **质量观察**：保留检测规则、结果与历史。官方 SDK 不提供独立质量暂停、归属释放和可信最终证据，因此这些动作在界面和服务端均禁用，不使用人工禁用来模拟。

账号密码、2FA、代理和设置按用户要求明文保存在插件数据目录。CPA 管理密钥在成功连接后明文保存在当前浏览器，刷新自动重连；主动断开或鉴权失败会清除。详见 [持久化说明](docs/PERSISTENCE_CN.md)。

## 安装与构建

请按 [安装说明](docs/INSTALL_CN.md) 将动态库装入现有官方 CPA；源码构建执行：

```sh
./scripts/build.sh
./scripts/verify.sh
```

正式构建会校验官方源码的全部 2,303 个基线文件，编译并内嵌原生登录引擎，输出单个动态库和符合官方安装器格式的 ZIP。SDK Go module 路径中的 `/v8` 是模块版本；插件自定义业务接口使用官方已有且受管理鉴权保护的 `/v0/management/plugins/model-quality/` 通道，两者不是一回事。

官方凭证保存接口没有跨进程原子比较更新能力。插件在写入前复查身份和原 JSON，保留未知策略字段，按本次账号登录的授权启用凭证；写入后检查物理文件及运行时的启用状态；结果不确定时保留状态，不盲目重试。仅支持 standalone 本地文件凭证；不宣称支持 Home 或远端存储同步。详细边界见 [官方单插件方案](docs/OFFICIAL_PLUGIN_CN.md)。

## 文档与证据

- [重置 2FA、退出设备与恢复边界](docs/OPENAI_ACCOUNT_SECURITY_CN.md)
- [一键登录、上传与启用](docs/LOGIN_ONECLICK_CN.md)

- [安装、升级与卸载](docs/INSTALL_CN.md)
- [官方兼容架构、API 与能力边界](docs/OFFICIAL_PLUGIN_CN.md)
- [账号、管理密钥及凭证的持久化](docs/PERSISTENCE_CN.md)
- [单库打包与运行时生命周期](docs/PLUGIN_BUNDLE_CN.md)
- [Go＋Sobek 来源与此前引擎验证](docs/OPENAI_LOGIN_GO_ENGINE_CN.md)
- [最终验证汇总](artifacts/official-plugin/verification-summary.json) · [官方安装实测](artifacts/account-security-0.15.0/official-install/report.json)
- [构建摘要](artifacts/official-plugin/build-manifest.json) · [文件校验值](artifacts/official-plugin/SHA256SUMS)

本机 **8317 已更新至插件 0.15.0 / UI 9.2.0 / Go＋Sobek 2.1.0**。原有 2 个账号、9 条历史登录任务及凭证文件保持不变；没有执行真实重置或退出设备。[本次部署核验](artifacts/account-security-0.15.0/deployment/deployment.json)。

账号登录全流程使用假账号、假授权交换和官方回调替身测试；原生安装验证使用独立官方 CPA 与本地假上游；不把离线验证表述为本版本已完成真实账号登录。本机 `8317` 已于 2026-11-17 02:33（北京时间）按用户确认切换为未修改源码构建的官方 CPA 与插件 0.14.0 / UI 9.1.0。2 个账号、9 条登录记录、代理及巡检设置保留，1 个旧绑定完成核验转换，原凭证文件内容未变。详情见 [本机升级结果](docs/LOCAL_OFFICIAL_UPGRADE_CN.md)。[本机历史部署记录](docs/LOCAL_CPA_CN.md) 与 [补丁宿主安装历史](docs/INSTALL_PATCHED_HOST_HISTORY_CN.md) 只供溯源。

源码在 `plugin/`；原生引擎在 `login-runtime-go/`；未修改官方源码副本在 `workspaces/CLIProxyAPI-official/`。旧 `workspaces/CLIProxyAPI/` 与 `host-patches/model-quality.patch` 为历史产物，不是当前安装依赖。原始 `/Users/ceng/Downloads/CLIProxyAPI-8.0.67` 始终只读。
