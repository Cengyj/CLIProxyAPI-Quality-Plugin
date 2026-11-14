# 离线原生宿主演示

此演示实际启动带补丁的 CPA 可执行程序，并从动态库加载 model-quality。模型请求发往本机 fake OpenAI 上游，不使用真实模型账户或额度。

## 启动与停止

先完成构建：

```sh
sh scripts/build.sh
python3 scripts/demo.py start
```

`start` 返回 JSON，其中 `page_url` 是两个页面的入口，`admin_key` 是仅用于本演示的固定假管理密钥。复制 `page_url` 到浏览器，在“管理连接”内输入该假密钥。它不代表生产凭证，也不能用于任何外部服务。

查询、停止：

```sh
python3 scripts/demo.py status
python3 scripts/demo.py stop
```

演示宿主和 fake 上游都仅监听 `127.0.0.1` 的临时端口。每次启动创建新的 `artifacts/demo/<UTC 时间>-<随机后缀>/`，内含独立 `auths/`、宿主权威暂停数据库、插件数据库、原生库副本、mock 配置及宿主日志。不会加载 `~/.auths` 或用户的常规凭证目录，不从原 CPA 目录复制配置、密钥或运行数据。子进程环境仅保留必要系统变量，避免继承生产存储、代理或云密钥环境变量。

关闭浏览器不会停止宿主。`stop` 使用带单次随机控制令牌的本机控制请求，由演示监督进程终止自己的子进程，并关闭 fake 上游；不会根据可能已复用的旧 PID 强制结束其他进程。停止后保留该次 mock 数据，便于审查日志和暂停归属。不要把删除数据库作为生产回滚或解除暂停手段。

## 演示内容

两个配置型假凭证提供 `mq-reasoner` 与 `mq-visual`，映射到 `mock-reasoner-v1` 和 `mock-visual-v1`。

- `logic-boxes` 返回严格整数答案；随机有界约束题由 fake 上游本地计算，不访问外部服务。
- SVG 题返回可安全预览的内联图形。
- 流式响应包含心跳、多块内容、finish_reason、usage 和上游完成标记。
- 本机 `/control` 支持 `correct`、`incorrect`、`empty`、`truncated`、`401`、`429`、`hold` 模式，仅用于离线验证。

默认定时检测和自动暂停开关关闭。自动恢复已移除，暂停后仅人工解除。打开页面和连接管理 API 不调用模型。请先在“手动评测”中选择假凭证、模型及题目，显式点击开始测试。

## 实际宿主浏览器验证

```sh
PLAYWRIGHT_MODULE=/path/to/node_modules/playwright \
CHROME_PATH=/path/to/chrome \
node scripts/verify_native_ui.cjs
```

验证脚本自行启动演示，并在 `finally` 中停止。它使用实际宿主、实际 C ABI 动态库、实际后台 engine、实际管理鉴权和实际浏览器 UI；只有远端模型服务替换为本机 fake upstream。

检查路径包括：未鉴权 v8 请求拒绝；静态页/连接无模型调用；浏览器创建固定凭证、多样本、流式、模型别名和 reasoning 测试；保存规则无调用；三轮明确答错后持久化凭证暂停；暂停时目标、规则、对照和判题均在入队与预算消耗前被拒绝；禁止所有暂停中复测；浏览器预览和人工 CAS 解除自身暂停后，所有相关规则仍保持停止；只有管理员另行恢复规则才可开始新的检测。

2026-11-13 在 macOS arm64、Chrome 157.0.8103.78 完成上述 9 项原生浏览器检查。fake upstream 共收到 6 次显式调用：2 个手动样本、3 轮失败检测，以及人工解除并显式恢复规则后的 1 次新检测；暂停期间为 0 次调用，任务数量和调用预算均不增长。演示进程已在验证结束后停止。截图已检查手动多样本、等待人工解除和解除预览三个页面状态。

最新结果以 `artifacts/verification/native-ui-results.json` 为准，失败时写 `native-ui-failure.txt`。实际宿主截图为 `native-manual-page.png`、`native-operations-page.png`、`native-release-preview.png`。fake 管理 API 的前端测试结果在 `ui-results.json`，两者分别记录，不混淆验证范围。

## 与已有演示并行验证

4.1.0 的验收使用 `MQ_DEMO_DIR` 指定独立状态目录，避免接管已经运行的演示。启动、查询和停止必须使用相同目录，例如：

```sh
MQ_DEMO_DIR="$PWD/artifacts/polish-preview" python3 scripts/demo.py start
MQ_DEMO_DIR="$PWD/artifacts/polish-preview" python3 scripts/demo.py status
MQ_DEMO_DIR="$PWD/artifacts/polish-preview" python3 scripts/demo.py stop
```

这里的状态目录只用于离线假数据，不是安装载荷；使用根构建产物前先核对候选版本。原生浏览器验收和候选解包验收自行使用独立目录并在结束时停止。本文上方 2026-11-13 的九项记录为历史证据，本轮报告位于 `artifacts/polish-4.1.0/verification-final2/native-ui-results.json`。
