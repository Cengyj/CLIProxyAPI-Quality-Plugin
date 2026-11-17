# 官方 CPA 单插件安装包

当前构建交付一个原生动态库。Go＋Sobek 登录运行时、来源清单和依赖许可压缩后嵌入动态库，官方 CPA 的插件安装器只需要安装这个库；不需要替换 CPA 程序，也不需要安装 Node、Chrome、Playwright 或单独填写登录运行时路径。

版本对应：插件 `0.15.0`，原生登录运行时 `2.1.0`。引擎标识为 `go_sobek_protocol`。参考实现来自本地 `toSub2-go-port`，源文件摘要见 `login-runtime-go/UPSTREAM_SOURCE_MANIFEST.json` 和 `SOURCE_MANIFEST.json`。协议适配保留在独立 Go module 中，插件只使用 CPA 公共 SDK。

## 构建与产物

在目标平台使用 Go 1.26 和匹配的 C 工具链执行：

```sh
python3 scripts/build_official_plugin.py
```

脚本要求插件的 SDK `replace` 指向 `workspaces/CLIProxyAPI-official`，并逐文件核对该目录与 CPA 8.0.67 的 2303 个源码/构建资源基线。存在改动或额外文件时拒绝构建；不会修改或格式化官方源码。脚本依次构建原生引擎、校验清单、在空 `PATH` 下运行 Sobek 自检、生成内嵌压缩包，再使用 `-buildmode=c-shared` 构建插件。

默认 macOS arm64 产物：

- `artifacts/official-plugin/model-quality.dylib`：可安装的单个插件库。
- `artifacts/official-plugin/model-quality-v0.15.0-darwin-arm64.zip`：ZIP 根目录只包含上述动态库，符合官方安装器规则。
- `artifacts/official-plugin/SHA256SUMS`、`build-manifest.json`：库、安装包和内嵌运行时的摘要。

当前启动器支持 macOS/Linux 的 arm64/amd64，Linux 输出 `.so`。构建脚本拒绝 Windows、FreeBSD 和跨平台构建，避免生成无法执行登录任务的安装包。当前已实际验证的平台为 macOS arm64，Linux 和其他支持的架构仍需各自完成编译和生命周期验证。

`--embed-only` 仅准备嵌入资源；`--runtime-bundle <目录>` 指定运行时目录，不存在时创建，存在时逐文件校验后复用；`--output <目录>` 指定插件产物位置。运行时目录不会被原地覆盖。使用默认目录时还会检查运行时源码是否与产物来源摘要一致；修改源码后需指定新的运行时目录构建，不能静默复用旧二进制。

普通 `go build` 不带构建标签时仍可编译源码，但登录会返回明确的缺少内嵌引擎错误。正式库必须由上述脚本使用 `loginbundle_embedded` 标签构建，不能用没有内嵌资源的开发库替代。

## 首次运行与持久化

插件在自己的数据目录释放运行时，路径为：

```text
<plugin-data-dir>/login-runtime/2.1.0-<os>-<arch>-<archive-sha256前12位>/
  manifest.json
  bin/login-runtime
  NOTICE.md
  SOURCE_MANIFEST.json
  UPSTREAM_SOURCE_MANIFEST.json
  licenses/
```

目录和执行文件权限为 `0700`，普通清单和许可文件为 `0600`。摘要和版本共同确定不可变目录；发现已有文件被修改时拒绝使用，不会覆盖正在运行的文件。安装过程先在同级临时目录完整校验，再原子改名。路径穿越、符号链接、硬链接、重复文件、清单外文件和缺失文件都会被拒绝。

这一目录只保存引擎程序与来源资料。账号、密码、2FA 种子、代理和插件设置由插件自己的持久化模块管理，不写进安装包或运行时来源清单。卸载动态库不会自动删除持久化数据。

## 进程归属与退出

登录任务由插件启动原生 Go helper；helper 使用同一个二进制的 `--auth-js-worker` 模式执行 Sobek。HTTP 会话和代理由 Go 层管理，没有浏览器回退。

Unix 启动器把独立的所有权管道传到 FD 3，并设置 `MQ_LOGIN_OWNER_FD=3`。这条管道与一次性登录 JSON 输入分开：正常输入关闭不会误取消登录；插件关闭或 CPA 进程退出时管道 EOF 会取消登录上下文，继而停止 HTTP 和 Sobek 子任务。取消、超时和插件卸载还会走任务的进程组清理路径。

已通过的离线回归包括：模拟启动器被强制终止，确认原生 helper 和真实 Sobek 忙循环子进程均退出；嵌入包提取后在空 `PATH` 中完成 Sobek 自检；安装损坏、路径穿越、链接、缺失文件及并发提取检查。此类自检不调用真实账号，不等同于真实 OpenAI 登录成功。

## 官方宿主兼容边界

官方安装器仅安装 ZIP 根目录的一个动态库，不会保留旁边的 helper，因此采用内嵌释放方式。官方自定义插件业务接口走鉴权后的 `/v0/management/plugins/model-quality/` 扩展通道；静态资源路径只提供公开页面文件。插件不新增或修改 CPA 的接口实现。

独立质量暂停/解除不属于该版本官方 SDK 的能力，不能通过修改人工禁用状态进行模拟。当前官方兼容版本保留模型测试、账号登录、账号检查和观察能力，明确拒绝不受支持的暂停动作。
