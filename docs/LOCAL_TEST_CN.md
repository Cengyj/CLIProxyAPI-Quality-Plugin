# 本地后台测试实例

> 此文记录的 mock 演示已停止。当前用于自行测试的完整 CPA 部署位于固定端口 8317，请阅读 [LOCAL_CPA_CN.md](LOCAL_CPA_CN.md)。以下地址与进程信息仅保留为历史演示记录。

部署日期：2026-11-14。当前运行 model-quality 0.3.0 与 quality capability v2 补丁宿主，macOS arm64。该记录属于历史测试实例；当前 8317 已部署 0.4.0 / UI 5.1.0 的 OpenAI 登录工作台，见 [LOCAL_CPA_CN.md](LOCAL_CPA_CN.md)。

## 访问

- 插件页面：http://127.0.0.1:57788/v0/resource/plugins/model-quality/index.html
- 管理密钥：`mq-demo-admin-not-a-live-secret`
- 业务测试密钥：`mq-demo-client-not-a-live-key`

以上为仅用于本地离线演示的假密钥。在页面右上角“管理连接”输入管理密钥。密钥仅在页面内存，刷新后重新连接。

已有两个假凭证，均提供 `mq-reasoner` 和 `mq-visual`。选择 `mq-reasoner`、题目“约束盒子”、一个样本，即可点击“开始测试”。模型调用全部进入本机 mock，不消耗真实额度；回答不代表真实模型质量。`mq-visual` 用于查看 SVG 安全预览。

定时检测和自动暂停默认关闭。暂停后禁止全部复测，仅允许管理员手动解除，解除后检测规则仍停止。手动测试不会自动暂停凭证。

## 后台与数据

监督进程和宿主已脱离启动终端运行；关闭页面或终端不会停止。仅监听 127.0.0.1，没有配置开机或登录自动启动。电脑关机、退出登录或手动停止后，需要重新启动。

本次独立目录：

```text
/Users/ceng/Downloads/CLIProxyAPI-Quality-Plugin/artifacts/demo/20261114-030841-6866c6/
```

其中 `mock-config.yaml` 为本次配置，`host.log` 为宿主日志，`plugin-data/` 为插件数据库，`auths/` 为独立凭证及宿主状态目录。没有复制原 CPA 的配置或真实凭证。请勿将真实账号密码或 token 输入 mock 控制接口。

## 查看与停止

```sh
cd /Users/ceng/Downloads/CLIProxyAPI-Quality-Plugin
python3 scripts/demo.py status
python3 scripts/demo.py stop
```

`status` 显示状态记录；实际存活可通过访问页面确认。停止命令通过当前实例的本地控制令牌通知监督进程，不按旧 PID 强杀其他进程。停止保留历史文件。

重新启动：

```sh
cd /Users/ceng/Downloads/CLIProxyAPI-Quality-Plugin
python3 scripts/demo.py start
```

正在运行时 `start` 返回当前实例；停止后再次 `start` 会创建新测试目录和新端口，旧记录保留但不会自动载入。请使用命令新返回的 `page_url`，不要假设一直为 57788。当前实例保持运行时，可随时回来继续同一批测试。

## 模拟异常

需要测试质量规则时，可使用下列命令把本机 mock 切换为明确答错。它只影响这个演示实例；改成 `correct` 可恢复正确输出。其他模式见 DEMO_CN.md。

```sh
cd /Users/ceng/Downloads/CLIProxyAPI-Quality-Plugin
python3 - <<'PY'
import json
from pathlib import Path
from urllib.request import Request, urlopen

state = json.loads(Path('artifacts/demo/state.json').read_text())
request = Request(
    state['upstream_url'] + '/control',
    data=json.dumps({'mode': 'incorrect'}).encode(),
    headers={
        'Content-Type': 'application/json',
        'Authorization': 'Bearer ' + state['control_token'],
    },
)
with urlopen(request, timeout=5) as response:
    print(response.read().decode())
PY
```

模拟答错不会自行创建检测任务或启用自动暂停；规则、全局开关及执行均由管理员明确操作。

## 部署验证

- 宿主及动态库 SHA-256 与既有 0.3.0 验证产物一致。
- 静态页面及四个脚本/样式资源返回 200。
- 未鉴权目标接口返回 401；鉴权后返回两个 mock 目标。
- 打开页面和连接管理接口产生零次模型调用。
- 浏览器实际提交一个测试，完成后回答 `4`、判定正确、解析模型 `mock-reasoner-v1`，mock 仅收到一次请求。
- 验证后定时检测和自动暂停继续关闭，服务保持运行，历史保留这一个部署验证任务。

完整记录：`artifacts/demo/20261114-030841-6866c6/deployment-checks.json`。本次仅做部署与冒烟验证，没有重新执行全部单测；完整产品验收见 TEST_REPORT_CN.md。
