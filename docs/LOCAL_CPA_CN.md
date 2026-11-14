# 完整 CPA 本地部署与插件安装

部署日期：2026-11-14。此实例运行完整 standalone CPA 管理后台，并在 CPA 的原生插件目录安装 model-quality 0.3.0。使用 8.0.67 归档源码加当前 quality v2 补丁，管理前端固定为 `d0126752201a35ff999bd2a1d4741215542258a8`；没有切换宿主上游基线。平台为 macOS arm64。

## 使用入口

- CPA 管理后台：http://127.0.0.1:8317/management.html
- CPA 插件管理：http://127.0.0.1:8317/management.html#/plugins
- CPA 内嵌插件：http://127.0.0.1:8317/management.html#/plugin-pages/model-quality/0
- 业务 API：http://127.0.0.1:8317/v1

管理密钥与业务 API Key 保存在本机私有文件：

```text
/Users/ceng/Downloads/CLIProxyAPI-Quality-Plugin/artifacts/local-cpa/LOGIN_CN.txt
```

登录 CPA 后，在“插件管理”可看到 model-quality 已加载、已启用；侧边栏“插件页面”包含“Model Quality · 模型质量”。从该菜单打开的页面仍由 CPA 同源托管。插件不会读取父页面管理密钥，首次进入需在插件自己的“管理连接”中输入同一个管理密钥；刷新插件后需重新连接。

初始没有上游凭证，也没有 mock 提供商。你可以在 CPA 后台配置 API Key 提供商，或通过 CPA 现有凭证/OAuth 功能添加自己的测试凭证，再到插件选择对应凭证与模型。自动调度和自动暂停默认关闭。插件新增的 OpenAI 密码/TOTP 登录工作台尚未实现，已有 CPA OAuth 与该待实施功能分别说明。

## 安装布局

所有运行文件固定在工作区的 `artifacts/local-cpa/`，不引用原 CPA 参考目录的运行数据：

```text
local-cpa/
  bin/cli-proxy-api              # Verified host with quality v2 patch
  config.yaml                   # Standalone configuration
  static/management.html        # Built CPA management frontend
  plugins/model-quality.dylib   # Installed native plugin
  auths/                        # Credentials and host quality state
  plugin-data/quality.db        # Plugin rules, history and audit
  kv.db                         # Host local KV state
  logs/                         # Host stdout/stderr
  local.cpa.model-quality.plist # Background service definition
  LOGIN_CN.txt                  # Local access credentials, mode 0600
  access-info.json               # Startup health-check credentials, mode 0600
  deployment.json               # Versions and artifact SHA-256
  deployment-checks.json        # Deployment verification
```

CPA 从 `plugins/model-quality.dylib` 按 C ABI 真实加载插件，通过 `/v8/management/plugins` 返回注册、启用状态及插件菜单。管理前端由已有固定源码使用 Bun 1.4.3、冻结 lockfile 构建，再安装单文件产物；未修改管理前端源码。

独立配置仅监听 `127.0.0.1:8317`，management remote-access 关闭；禁用面板自动升级和服务发现。启动使用 `--standalone --local-model`，不自动刷新远程模型目录。管理页面允许按需手动配置或刷新。

## 后台运行

服务由当前用户的 macOS launchd 管理，label 为 `local.cpa.model-quality`。已启动后台运行，关闭终端、浏览器不会停止；异常退出由 launchd 重启。没有写入系统或用户登录自动启动目录，退出系统登录或重启电脑后手动 start 即可。

```sh
cd /Users/ceng/Downloads/CLIProxyAPI-Quality-Plugin
./scripts/local_cpa.sh status
./scripts/local_cpa.sh stop
./scripts/local_cpa.sh start
./scripts/local_cpa.sh restart
```

启动/重启会等待插件鉴权接口就绪。所有命令复用同一配置、端口、凭证目录和数据库，不生成新的测试环境、不清理数据。如果修改了管理密钥，需要同步更新私有 `access-info.json` 的 `management_key`，用于启动就绪检查；也更新自己的登录记录。

## 验证范围与维护

已验证管理前端构建、CPA 页面 200、未鉴权管理接口 401、model-quality 注册和有效启用、菜单注册、空凭证列表、默认开关关闭，以及停止后原目录重启。浏览器核验 CPA 登录、插件列表和内嵌插件页面。部署验证不调用任何上游模型。

原 `57788` mock 演示已停止，历史文件保留。日常使用当前 `8317` 完整 CPA 实例。需要离线专门验证时可以另行运行 `scripts/demo.py`，不要把 demo 状态当作此 CPA 实例的状态。

配置、私有登录文件、auths 和数据库均为运行数据，不能加入发布包或源码提交。升级前停止服务并备份数据，再替换二进制/插件；不要覆盖 config.yaml。质量暂停的卸载保留、人工释放和旧宿主回滚规则仍见 INSTALL_CN.md；不要删除数据库作为解除暂停或回滚步骤。
