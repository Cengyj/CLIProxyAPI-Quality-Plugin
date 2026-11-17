# 官方 CPA 单插件安装说明

当前版本：插件 **0.15.0**、UI **9.2.0**、内嵌 Go＋Sobek **2.1.0**。安装目标是具备原生插件功能的官方 CPA；已验证基线为本地 CPA **8.0.67** 原始源码。无需修改宿主源码或替换 CPA 程序。

## 安装文件

macOS Apple Silicon 使用：

- `artifacts/official-plugin/model-quality.dylib`
- `artifacts/official-plugin/model-quality-v0.15.0-darwin-arm64.zip`

ZIP 根目录只有一个动态库。Go 登录引擎及许可材料已嵌入库中，不需要复制独立 helper。校验安装文件：

```sh
cd artifacts/official-plugin
shasum -a 256 -c SHA256SUMS
```

此 dylib 仅用于 macOS arm64。Linux/其他支持的架构需在目标机器用匹配的 Go 与 C 工具链构建并验证，不能直接重命名使用。构建范围和流程见 [打包说明](PLUGIN_BUNDLE_CN.md)。

## 安装到已有官方 CPA

1. 在 CPA 配置中查看现有 `plugins.dir`。若尚未启用原生插件，按下面示例合并插件配置，保留其他 CPA 配置。
2. 停止 CPA，把动态库复制到该插件目录，文件名保持 `model-quality.dylib`（Linux 为 `model-quality.so`）。已有同名库时先备份。
3. 配置独立、可写的绝对 `data_dir`，然后启动 CPA。
4. 在官方插件管理页面检查 `model-quality` 已加载、已启用。打开插件页面或同源 `/v0/resource/plugins/model-quality/index.html`，输入 CPA 管理密钥连接。
5. “OpenAI 登录”中导入账号，按需配置代理，从账号行点击 2FA 登录。成功后自动导入/更新并启用 CPA 凭证，账号行展示结果；无需再打开任务页确认。

配置示例中的路径需替换为实际路径，**只合并 `plugins` 配置**：

```yaml
plugins:
  enabled: true
  dir: /absolute/path/to/cpa/plugins
  configs:
    model-quality:
      enabled: true
      data_dir: /absolute/path/to/cpa/model-quality-data
      scheduling_enabled: false
```

插件会在首次加载时创建自己的数据目录并释放内嵌引擎，无需设置登录运行时路径。账号数据目录权限要求 `0700`、数据库 `0600`。相对 `data_dir` 按 CPA 工作目录解析，推荐绝对路径，避免启动方式变化后看不到原数据。

也可以通过官方插件商店安装同一 ZIP，但需要该商店源有此插件条目和对应平台的下载地址。本项目尚未发布到公共商店；不能把本地验证源当作可用公共源。本轮离线验证通过本地假商店实际执行了官方安装 API。直接复制动态库不依赖商店发布。

账号行“更多”中的“重置 2FA”和“退出所有设备”需主动确认；可选完成后重新授权并启用 CPA。升级本身不执行这些操作，详见 [账号安全](OPENAI_ACCOUNT_SECURITY_CN.md)。

## 使用范围

- 保留手动模型测试、账号管理、Go＋Sobek 2FA、代理和定时巡检。
- 质量功能仅观察；自动暂停、独立质量暂停和释放由服务端明确拒绝。
- 自动巡检默认关闭。自动重登只在明确认证错误且许可齐备时启动，成功后自动保存并启用；人工禁用、冷却和登录中的策略变化仍会阻止后台修复。
- 凭证更新使用官方回调，保存前复查、保存后核对文件和运行时记录。官方没有原子 CAS；登录期间请避免在其他管理页面同时修改同一凭证。`commit_unknown` / `saved_not_adopted` 需先核对实际 CPA 状态，不应直接重复提交。
- 只验证 standalone、本地文件型 Codex 凭证。不宣称支持 Home 或远端数据库/Git/对象存储。
- 停止模型测试可取消后续工作和已取得的流；官方接口不能保证首包前或非流式上游请求立即终止。

接口和持久化详情见 [官方单插件方案](OFFICIAL_PLUGIN_CN.md) 与 [持久化说明](PERSISTENCE_CN.md)。

## 升级、卸载与历史实例

升级前停止 CPA，备份配置、整个插件数据目录和 CPA 凭证目录，然后替换动态库，保留原 `data_dir`。仅换动态库不会删除账号和设置。登录中的内存候选不能跨重启继续使用，升级前应先处理活动任务。

卸载时先停止/禁用插件；只移除动态库不会删除账号库、测试记录或 CPA 凭证。关闭浏览器不等于关闭插件服务，已开启的巡检会随 CPA 继续运行。

本机 8317 已在用户明确批准后完成一次性备份、绑定转换和官方宿主切换，详见 [升级记录](LOCAL_OFFICIAL_UPGRADE_CN.md)。旧账号库原先属于宿主，新库属于插件；安装包本身不自动迁移其他旧实例，不能仅替换动态库就认为旧绑定兼容。已有质量暂停必须先在原补丁宿主中按归属处理，不能删除旧库或换官方宿主来解除；插件看到历史暂停记录时继续保留并阻止相关操作。旧安装流程见 [历史说明](INSTALL_PATCHED_HOST_HISTORY_CN.md)。

## 从源码构建与验证

```sh
./scripts/build.sh
./scripts/verify.sh
```

默认构建入口是 `scripts/build_official_plugin.py`，SDK 只引用 `workspaces/CLIProxyAPI-official/`。构建逐文件校验原官方基线，拒绝修改或额外文件。`verify.sh` 分别测试插件和原生登录引擎，并启动独立官方 CPA 进行离线安装与生命周期验证，不操作 8317。
