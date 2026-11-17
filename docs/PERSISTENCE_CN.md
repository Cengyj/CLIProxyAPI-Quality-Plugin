# 官方插件账号与密钥持久化

插件 0.15.0 将账号、代理、登录任务和巡检设置保存到**插件自己的数据目录**。按照用户要求使用明文，不需要加密密钥、加密初始化或数据迁移。关闭浏览器不会丢失已保存内容；CPA 重启后从同一目录恢复。

## 保存位置

通过 CPA 配置 `plugins.configs.model-quality.data_dir` 指定目录。默认是 CPA 工作目录下 `model-quality-data`；推荐使用固定绝对路径。

```text
<plugin-data-dir>/
  quality.db
  accounts/
    credential-login.db
    login-receipts.db
  login-runtime/
    2.1.0-<os>-<arch>-<archive-sha12>/
      manifest.json
      bin/login-runtime
      来源与许可材料
```

| 内容 | 保存位置与格式 |
| --- | --- |
| 邮箱、密码、2FA 长期密钥、账号备注、代理、CPA 绑定和账号许可 | `accounts/credential-login.db` 的 `accounts` 桶，明文 Gob 数据 |
| 全局代理、巡检开关/间隔、自动重登开关 | 同库 `meta` 中的账号设置 |
| 登录任务、日志、幂等记录、历史保存资料 | 同库 `jobs`、`idempotency`、`vault` 等桶 |
| 凭证提交收据和不确定结果 | `accounts/login-receipts.db`，不用于反复重放 token 写入 |
| 手动测试、回答、质量观察规则、设置、审计 | `quality.db`，bbolt 中保存 JSON |
| 正式 OAuth token | 账号登录成功后自动通过官方 `host.auth.save` 写入 CPA 自己的凭证目录 |
| CPA 管理密钥 | 成功连接后存入当前浏览器的 `localStorage`，不写入插件数据库 |
| Go＋Sobek 程序 | `login-runtime/`；没有账号密码和 token |

bbolt/Gob 是存储及序列化格式，不是加密。账号和 2FA 可由程序直接读取。账号目录要求 `0700`、数据库 `0600`；权限控制并非加密。CPA 原有管理密钥 bcrypt 校验保持官方行为，不涉及插件账号加密。

HMAC 元数据用于去重、幂等和限流指纹，不是存储加密密钥；与数据库一同保留。账号明文、OAuth token 和 CPA 管理密钥是三类独立数据，删除其中一种不等于删除其他内容。

## 保存与登录

导入 `账号----密码----2FA长期密钥` 后，通过受鉴权的插件 `/login/accounts/import` 保存到账号库。导入本身不发起登录。编辑账号带 revision，旧页面不能覆盖已有新版本。

从列表发起 2FA 后，插件在 Go 原生进程中执行登录。密码与 TOTP 种子来自账号库；PKCE、OAuth state、会话 Cookie 和尚未提交的候选 token 留在活动会话内存，任务结束时清理，不保存浏览器资料。候选不能跨重启继续确认。

从账号列表发起的一次登录包含成功后的保存和启用授权，无需再确认候选；显式手工 OAuth 与历史候选仍使用原提交操作。保存前重新核对身份、原 JSON 和禁用状态；保存前持久化未决收据，保存后核对物理文件内容及运行时记录的身份、auth_index 和启用状态；官方不公开运行时 token，不能据此独立证明新 token 已在内存执行器生效。官方没有跨进程原子 CAS，结果不确定时保留 `commit_unknown` / `saved_not_adopted`，不自动重复写入。账号登录的新凭证直接启用；已有目标在启动时冻结身份和策略，用户手动登录可启用原本禁用的目标，但之后发生的策略变化会阻止写入。后台巡检不对人工禁用目标发起修复。非 token 策略保持原值。

巡检与自动登录设置保存在服务端。即使浏览器关闭，已开启的巡检仍随 CPA 运行；CPA 停止期间不会执行。已授权的重登成功后自动保存到绑定的 CPA 凭证，提交前复查全局和账号许可；关闭自动化或目标策略变化会阻止保存。

## 浏览器中的管理密钥

键名为：

```text
model-quality:management-key:/v0/management/plugins/model-quality
```

浏览器按协议、域名和端口隔离这个值。成功连接后明文保存；刷新或重新打开会读取它并恢复连接。请求使用 Authorization header，不把密钥放入 URL、Cookie 或导出文件。账号密码、2FA 和未保存导入文本不存入浏览器 localStorage。

主动断开或该密钥返回 401 时清除；临时网络错误保留。浏览器禁用本地存储时会提示并降级为本页面内存连接。清除浏览器站点数据只会清掉浏览器密钥，不删除插件账号库。官方 CPA 外层管理页面有自己的登录存储，插件不读取它。

## 重启、升级和备份

| 操作 | 结果 |
| --- | --- |
| 刷新/关闭页面 | 已保存账号和设置保留，未保存草稿丢失 |
| CPA 重启且 `data_dir` 不变 | 重新读取持久化账号与设置；中断任务不自动重放 |
| 替换插件动态库 | 数据库保留；新版本引擎按摘要释放到自己的目录 |
| 删除插件账号 | 不自动删除对应 CPA 凭证或撤销远端授权 |
| 改变 `data_dir` | 读取新目录，不会自动发现旧库 |
| 卸载插件 | 不自动删除账号库、CPA 凭证或历史记录 |

备份前正常停止 CPA，成套备份 CPA 配置、整个插件 `data_dir` 和 CPA 凭证目录；保留相应插件版本与构建摘要。不要只备份动态库。恢复时保持原路径或同步修改配置，恢复 `0700`/`0600` 权限，再启动核对账号和设置。

未决提交收据需随账号库一起保留，不能删除收据来解除不确定状态。数据库逻辑删除不是磁盘安全擦除，旧备份中可能仍有原内容。

## 本机实例已完成的升级

2026-11-17 02:33（北京时间），用户确认后，8317 已从补丁宿主切换到官方 CPA 和插件 0.14.0。现在实际账号库是 `artifacts/local-cpa/plugin-data/accounts/credential-login.db`；提交收据在同目录 `login-receipts.db`，新 Go＋Sobek 引擎释放到 `plugin-data/login-runtime/`。

旧 `auths/.credential-login/` 整目录已移至停服备份 `artifacts/local-cpa/backups/official-oneclick-0.14.0-20261117-023337/auths/.credential-login/`，因此不再被官方凭证加载器扫描。源账号库保持原字节，转换工具在新库中仅更改 1 个账号绑定和 1 条对应已提交任务的身份版本，其他记录及未知字段保留。前后账号凭据、代理设置、历史日志和 CPA 凭证内容摘要全部一致。

这是经用户批准的一次性本机操作；插件安装包没有自动迁移或清空逻辑。[升级报告](LOCAL_OFFICIAL_UPGRADE_CN.md) 记录了步骤和备份，[旧持久化记录](PERSISTENCE_PATCHED_HOST_HISTORY_CN.md) 仅供历史参考。

源码依据：`plugin/cmd/model-quality/application.go`、`plugin/login/store.go`、`plugin/login/accounts.go`、`plugin/login/account/host.go`、`plugin/login/service.go`、`plugin/loginbundle/` 和 `plugin/ui/app.js`。

## 账号安全操作记录（0.15.0）

重置 2FA、退出所有设备使用 `accounts/security/<job_id>.json` 保存本次操作意图、因子绑定和激活证据。新密钥在发送激活请求前明文持久化；确认生效后同步账号库和已有资料副本。操作记录不保存可恢复登录 Cookie，未知结果不自动重发。该目录必须和账号库一起备份；详细字段及本地完成边界见 [账号安全](OPENAI_ACCOUNT_SECURITY_CN.md)。
