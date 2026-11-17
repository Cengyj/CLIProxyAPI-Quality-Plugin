# UI 4.3.0 宿主对齐本地候选

本轮候选标识为 `host-alignment-4.3.0-rc1`，插件后端 **0.3.1**、UI **4.3.0**、quality capability v2、schema 2。版本分别表示后端与界面，不要求相同。仅生成本地候选，不替换用户正在运行的 CPA，不对外发布。旧 4.1 候选与 4.2 报告保留为历史证据，不能证明本轮改造。

## 验证对象与门槛

当前证据目录：`artifacts/host-alignment-4.3.0/verification/`。`record_verification.py` 实际执行每条命令并记录退出码、输入源码集合与 SHA-256、日志 SHA-256、构建输出/报告 SHA-256。检查期间源码变化、缺少声明的输出、旧报告与当前源码不匹配均不能通过。已有同名记录不得覆盖；失败应保留并以新的证据目录复测。

打包器要求：宿主 fresh 全量测试、选定包 race、必需编译与宿主构建、只读基线和干净副本补丁复现；插件独立 module 测试/race/vet、C shared library 构建和原生生命周期；题目测试纯逻辑、实际原生 API、fake API 浏览器、实际原生浏览器与固定宿主管理前端浏览器验证。后者分别绑定最终五项 UI 源码与截图，原生报告还绑定实际宿主/动态库。配色、轮询、长输出、弹窗几何证据由执行该检查的命令绑定。

八组同数据、同主题、同视口的前后截图位于 `baseline/paired-clean/` 与 `verification/`，前图逐文件绑定冻结的 UI 4.2.0 源码。回答截图定位到实际回答区域。`comparison-desktop.png`、`comparison-mobile.png` 及 `comparison-provenance.json` 绑定比较图及所用原始截图。当前宿主矩阵包含独立与内嵌、三主题、窄 iframe、侧栏两状态和 200% 缩放；具体已执行范围与限制以报告为准。检测器意见及人工判断见 [UI_HOST_ALIGNMENT_REPORT_CN.md](UI_HOST_ALIGNMENT_REPORT_CN.md)，机械检测不是无障碍认证。

## 重建与记录

```sh
sh scripts/build_ui.sh
node scripts/test_question_test.mjs
python3 scripts/record_verification.py \
  --output artifacts/host-alignment-4.3.0/verification \
  --name plugin-tests --scope plugin --cwd plugin -- go test -count=1 ./...
```

动态库构建记录示例：

```sh
python3 scripts/record_verification.py \
  --output artifacts/host-alignment-4.3.0/verification \
  --name plugin-build --scope plugin --cwd plugin \
  --artifact artifacts/model-quality.dylib --artifact artifacts/model-quality.h \
  -- go build -buildmode=c-shared -o ../artifacts/model-quality.dylib ./cmd/model-quality
```

浏览器检查使用 `UI_VERIFY_OUTPUT` 指向本轮证据目录；原生检查使用独立 `MQ_DEMO_DIR`，不接管既有演示。当前题目 API 检查可指定 `--report` 和 `--demo-dir`，二者必须在 `artifacts/` 内。全部模型请求仅发往本机 fake upstream。

## 候选与解包冒烟

所有门槛与最终源码匹配后：

```sh
python3 scripts/package_release.py \
  --build-id host-alignment-4.3.0-rc1 \
  --evidence artifacts/host-alignment-4.3.0/verification
python3 scripts/smoke_candidate.py \
  artifacts/candidates/host-alignment-4.3.0-rc1/model-quality-0.3.1-ui-4.3.0-host-alignment-4.3.0-rc1-darwin-arm64.tar.gz \
  --report artifacts/candidates/host-alignment-4.3.0-rc1/INSTALL_SMOKE.json
```

候选目录同时包含 `release-manifest.json`、`validated-build.json`、`PAYLOAD_SHA256SUMS`、`SHA256SUMS`。解包冒烟用包内实际宿主、动态库与脚本启动临时独立安装：逐文件校验载荷，利用本机已有 Go module 缓存并关闭网络依赖下载，从包内源码重新构建动态库（依赖未 vendor，不代表干净机器可离线构建），校验五项资源返回字节，验证鉴权、当前单模型多回答/编辑自定义/HTML 题目、兼容确定性规则、三轮答错暂停、暂停时拒绝和人工解除后规则继续停止。报告绑定归档 SHA-256，位于归档外以避免循环哈希。它只停止并删除本次临时安装。

## 安装载荷与边界

在候选目录运行 `shasum -a 256 -c SHA256SUMS`；解包后用外部 `PAYLOAD_SHA256SUMS` 在解包目录核对（路径以 `model-quality/` 开头）。安装操作见 [INSTALL_CN.md](INSTALL_CN.md)。宿主为 `artifacts/cli-proxy-api-quality`，动态库为 `artifacts/model-quality.dylib`。UI 由动态库内嵌，无需发布额外静态目录。保留生产宿主质量库和插件数据库，升级不解除暂停、不恢复规则。

归档只纳入白名单源码（包括 `.mjs`、内嵌题目 `.txt` 和 LGPL/GPL 许可文本）、必要构建产物、示例配置、来源许可证、设计和操作文档、最终脱敏验证证据。排除真实配置/密钥/凭证、数据库、运行日志、临时浏览器数据和演示状态，初步截图与失败尝试也不作为发行证据打包。仅验证 macOS arm64；浏览器版本、模拟设备与其他限制写入本轮报告，未验证的平台和真实移动设备不据此宣称支持。
