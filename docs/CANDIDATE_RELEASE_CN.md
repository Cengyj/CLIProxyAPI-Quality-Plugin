# UI 4.1.0 本地候选包与验证记录

本轮候选采用插件后端 0.3.0、UI 4.1.0、quality capability v2、schema 2。版本号分别表达后端接口与界面版本，不要求相同。旧 `artifacts/model-quality-0.3.0-darwin-arm64.tar.gz`、旧发布清单和旧校验文件保留为历史材料，不是本轮安装产物。

## 完整性门槛

`record_verification.py` 实际执行命令，把退出码、时间、命令、日志 SHA-256、输入源码 SHA-256 写入独立 JSON。验证期间输入发生变化即失败；已存在记录不能被静默覆盖。构建记录通过 `--artifact` 绑定实际输出。打包器对照完整输入集合，新增、删除或修改受验证源码均使旧记录失效；宿主若含补丁基线之外的文件也会拒绝打包。修改受验证源码后，必须在新目录重新记录相应检查，不能手改哈希。

本轮证据目录为 `artifacts/polish-4.1.0/verification-final2`。打包器要求以下记录全部通过：

- `host-tests-fresh`：宿主全量测试；`host-race-fresh`：auth、pluginhost、usage、httptransport 相关 race。两项均使用 `-count=1`，强制本轮实际执行。
- `host-compile`：宿主要求的 test-output 编译，以及发行二进制构建、输出哈希。
- `baseline`：只读原始归档校验；`patch`：干净副本补丁复现。
- `plugin-tests`、`plugin-race`、`plugin-vet`：插件独立 module 检查。
- `plugin-build`：实际 C shared library 构建、动态库与头文件输出哈希。
- `native-lifecycle`：显式启用实际动态库的宿主生命周期测试。
- `ui`、`native-ui`：fake API 浏览器检查、实际宿主浏览器检查；对应结果 JSON 必须包含最终 UI 源码哈希，原生报告还必须绑定实际库与宿主二进制。

浏览器命令记录通过 `--artifact` 绑定结果报告；`ui` 另外绑定 `contrast-results.json`、`polling-probe.json`、`stress-results.json` 和 `dialog-layout-results.json`。结果报告中的 `screenshot_sha256` 逐项绑定截图。基线图片使用 `baseline/paired-clean/` 的同条件八张截图，已核对其源文件哈希与冻结 UI 4.0.0 相同；同名最终截图缺失时拒绝打包。

首次 `host-race` 命令中的 httptransport 路径拼写错误导致 setup failed，失败记录保留；纠正为 `./sdk/httptransport` 的 `host-race-final` 通过后，再用 `-count=1` 的 `host-race-fresh` 作为发行检查。未将失败结果改写为通过。宿主无 Go 源码修改，本轮重新执行测试和编译，没有将历史状态统一写为 passed。

## 可复现命令

先完成最终 UI 静态构建和全部验证。记录示例：

```sh
python3 scripts/record_verification.py \
  --output artifacts/polish-4.1.0/verification-final2 \
  --name plugin-tests --scope plugin --cwd plugin -- go test ./...
```

相应构建命令添加输出文件：

```sh
python3 scripts/record_verification.py \
  --output artifacts/polish-4.1.0/verification-final2 \
  --name plugin-build --scope plugin --cwd plugin \
  --artifact artifacts/model-quality.dylib \
  --artifact artifacts/model-quality.h \
  -- go build -buildmode=c-shared -o ../artifacts/model-quality.dylib ./cmd/model-quality
```

已有同名记录时，为下一次完整验证采用新证据目录。浏览器脚本通过 `UI_VERIFY_OUTPUT` 选择同一证据目录。原生浏览器测试用独立 `MQ_DEMO_DIR`，不接管或停止用户已经启动的演示。

最终候选生成示例（build-id 必须是新的独立标识）：

```sh
python3 scripts/package_release.py \
  --build-id polish-4.1.0-rc1 \
  --evidence artifacts/polish-4.1.0/verification-final2
```

输出位于 `artifacts/candidates/polish-4.1.0-rc1/`，含安装归档、`release-manifest.json`、`validated-build.json`、`PAYLOAD_SHA256SUMS`、`SHA256SUMS`。打包前检查源码、静态发布目录、构建输出、测试报告与日志哈希；打包后逐文件重新读归档并核对哈希。此步骤不是实际装载验证，必须接着运行：

```sh
python3 scripts/smoke_candidate.py \
  artifacts/candidates/polish-4.1.0-rc1/model-quality-0.3.0-ui-4.1.0-polish-4.1.0-rc1-darwin-arm64.tar.gz \
  --report artifacts/candidates/polish-4.1.0-rc1/INSTALL_SMOKE.json
```

该步骤在系统临时目录解包，通过包内实际宿主、动态库和演示脚本启动独立 fake upstream。验证所有载荷哈希、宿主返回的五项 UI 资源字节、管理鉴权、手动流式评测、保存无调用、三轮明确错误暂停、暂停中拒绝执行、人工解除后规则仍停止；完成后只停止和移除这一临时安装。报告绑定候选归档 SHA-256，保存在归档外，避免报告与归档的循环哈希依赖。报告与归档放在同一候选目录时，其哈希会追加到该目录的 `SHA256SUMS`。

## 安装与升级

先在候选目录运行 `shasum -a 256 -c SHA256SUMS`，再解包安装归档。解包后用外部 `PAYLOAD_SHA256SUMS` 在解包目录逐项核对；其中路径以 `model-quality/` 开头。

安装步骤与生产数据保护遵循 `INSTALL_CN.md`。候选包内可执行宿主为 `artifacts/cli-proxy-api-quality`，动态库为 `artifacts/model-quality.dylib`，配置仅含明确的示例 `artifacts/config/model-quality.example.yaml`。前端由库嵌入，无需另行发布静态目录。保存正式部署的宿主质量库和插件数据库；升级 UI 不解除暂停、不恢复规则。不要将演示运行目录复制到正式部署。

归档包含插件源码、必要宿主源码和补丁基线、许可证来源、最终 PRODUCT/DESIGN 与操作文档、明确纳入的脱敏测试证据。排除真实配置、凭证、auths、数据库、运行日志、演示状态、临时浏览器数据。当前只验证 macOS arm64 和报告中指定的浏览器；其他系统、浏览器和真实移动设备不据此宣称已验证。没有外部发布，也没有替换用户正式部署。
