# 本机 8317 官方 CPA 升级结果

2026-11-17 02:33（Asia/Shanghai）已按用户明确确认完成更新。当前运行：

- 未修改 CPA 8.0.67 原始源码构建的官方宿主。
- model-quality 插件 0.14.0 / UI 9.1.0。
- 插件内嵌 Go＋Sobek 2.0.1；账号登录成功后自动保存到 CPA 并启用，进度与结果留在列表。

## 数据保留

2 个账号、9 条登录记录及全部日志、1 份 CPA 凭证均保留。账号密码、2FA、代理、备注、绑定对象、自动化许可和巡检设置的前后摘要一致，CPA 凭证文件逐字节摘要一致，配置文件未改变。巡检和自动重登仍关闭，原间隔 15 分钟。

停服后确认没有宿主质量暂停、未决凭证提交、插件阻断规则或活动测试，再运行独立转换工具。源 schema 2 账号库只读打开；新库只改动 1 个账号的身份版本和 1 条对应已提交任务的身份版本。HMAC、幂等记录、设置、其他历史和未知 Gob 字段保持原始内容。没有 vault 记录需要转换，没有重新登录、上传或启用既有凭证。

旧登录运行包包含辅助 JSON，不能继续放在官方递归扫描的 auth-dir 内。旧 `auths/.credential-login/` 整目录已可逆移入停服备份，原始数据库未修改。

## 当前路径与备份

```text
artifacts/local-cpa/
  bin/cli-proxy-api                         # Unmodified official source build
  plugins/model-quality.dylib              # Verified 0.14.0 native plugin
  plugin-data/quality.db                   # Preserved tests, rules and history
  plugin-data/accounts/credential-login.db # Plugin-owned accounts and jobs
  plugin-data/accounts/login-receipts.db   # New submission receipts
  plugin-data/login-runtime/               # Embedded Go/Sobek 2.0.1
  auths/                                  # Original CPA credentials retained
```

停服备份目录：

```text
artifacts/local-cpa/backups/official-oneclick-0.14.0-20261117-023337
```

备份包括原宿主、插件、配置、私有访问文件、KV、插件数据库、凭证文件，以及移出的整个旧登录目录。目录位于本机私有运行区，不能加入发布包。若需回滚，应先停止服务并核对升级后新增账号/任务，不能直接用旧库覆盖新数据。

## 验证

- 官方宿主二进制 SHA256：`00de93170cbe1b85c7f4a1cfd20fd1a104ebe6454aee4670040f2a4cf3a18703`。
- 插件 SHA256：`51f80a7e22c89bd7763b8e16497b501969a657da7ea3229de3f51b68513ddf53`。
- 管理接口返回 `automatic_commit=true`、`commit_mode=account_auto_commit`，原生运行时为 Go＋Sobek 2.0.1。
- 六份已安装静态资源与源码摘要一致，页面为 UI 9.1.0。
- 原始参考与官方构建副本的 2,303 个基线文件均保持不变。
- 核对只使用管理 GET；本次更新没有发起真实 2FA 登录或模型调用。

证据位于 `artifacts/login-oneclick-0.14.0/deployment/`：`preflight.json`、`conversion.json`、`after.json`、`deployment.json`、`installed-ui.json`。独立转换工具通过离线 race/vet，部署脚本的失败回滚使用离线临时文件验证通过。

## 使用

继续打开 http://127.0.0.1:8317/management.html ，刷新插件页面后使用“OpenAI 登录”。由于插件接口存储键从 v8 变为官方 v0，升级后首次连接可能需要在插件“管理连接”重新输入原管理密钥一次；成功后会保存，后续刷新自动恢复。管理密钥本身没有更改。

`sh scripts/local_cpa.sh status|start|stop|restart` 继续可用，启动就绪检查已支持官方 v0 插件路由。当前仍采用原会话进程启动方式，系统重启后的启动方式没有变更。
