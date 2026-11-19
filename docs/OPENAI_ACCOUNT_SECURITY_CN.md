> 2026-11-18：重置行为与现行 toSub2 对齐，详见 [对齐说明](RESET_2FA_REFERENCE_ALIGNMENT_CN.md)。以下历史部署和版本说明不代表当前版本。

# 重置 2FA 与退出所有设备

插件 0.15.0 的账号管理页增加「重置 2FA」和「退出所有设备」。操作位于账号行的更多菜单中，沿用原账号列表、任务进度和内联日志，不需要跳转到另一个登录页面。CPA 仍为未修改的官方版本，安全操作由插件内嵌的 Go＋Sobek 2.1.0 引擎完成。

## 操作行为

**重置 2FA** 会重新登录当前账号，核实网页登录身份和唯一有效旧 TOTP 因子，移除旧因子，再登记、激活并保存新密钥。确认框可同时选择退出所有设备；可选择成功后重新授权，自动保存到 CPA 并启用凭证。新密钥确认后会更新账号资料，原验证器中的旧密钥不再适用。

**退出所有设备** 是单独的账号操作，不重置 2FA。它建立本次网页登录会话并核实身份后，调用退出接口。选择重新授权时，退出完成后建立新的授权流程，自动保存并启用 CPA 凭证。取消重新授权选项时，只执行安全操作，不额外上传 CPA 凭证。

两项操作都需要在确认框明确提交。打开页面、查看资料、刷新、后台巡检和升级插件不会执行重置或退出操作。后台自动重登也不会自行重置 2FA 或退出其他设备。

## 参考实现与接口顺序

参考项目为 `/Users/ceng/Downloads/toSub2-go-port`，重点依据：

- `docs/pure-go-runtime/totp-reset.md`：当前流程、未知结果和本地完成边界。
- `internal/protocol/totp_web_authorization.go`、`totp_reset.go`：网页登录身份核验、请求头和退出响应判断。
- `internal/protocol/totp_reset_state.go`、`totp_reset_continue.go`：移除、登记、激活的请求顺序及不可重放边界。

安全操作使用本次普通网页登录取得的会话，核对邮箱、账号 ID 和访问令牌上下文；不能直接把 CPA 的 Codex OAuth token 当成 ChatGPT 网页安全会话。每次新操作使用新的内存 CookieJar，不恢复或保存跨任务 Cookie 会话，不启动浏览器自动化。

重置的主要接口为：

1. `GET /api/auth/session` 核实当前会话身份。
2. `GET /backend-api/accounts/mfa_info` 核实唯一有效旧 TOTP 因子。
3. `POST /backend-api/accounts/mfa/user/disable_in_house` 移除已核实的旧因子。
4. `POST /backend-api/accounts/mfa/enroll` 登记新因子，保留新密钥、登记会话及因子绑定。
5. `POST /backend-api/accounts/mfa/user/activate_enrollment` 激活新因子。
6. 按选择调用 `POST /backend-api/accounts/logout_all`，随后重新授权并上传 CPA。

完整业务成功回执允许直接推进下一步；不会仅凭 HTTP 200 或空对象判断因子已激活。移除或激活响应不明确时，仅允许在本次会话中有限读取状态核实，不能重发修改请求。登记返回的必要材料缺失时停止，不能再次登记猜测结果。

退出接口沿用参考项目兼容行为：完整收到无挑战、HTML 或明确错误的 2xx 响应可以成功，空响应、`null`、数组不因形式本身被拒绝。超时、断连、响应读取失败或 5xx 保持未知结果，不自动重复退出。

## 持久化和中断处理

账号密码、2FA 密钥和代理继续按用户要求明文保存。当前凭据位于插件数据目录的 `accounts/credential-login.db`；每次安全操作另有 `accounts/security/<job_id>.json` 私有操作记录。目录和文件分别采用 `0700`、`0600` 权限。这些操作记录不位于 CPA 的凭证扫描目录。

引擎在发送有副作用的步骤前先原子保存操作意图，并进行持久化和读回核对。新密钥、登记会话与因子信息在激活前保存；只有本次新因子的完整激活证据成立后，才将新密钥提升为账号当前凭据。插件同时更新已有的同账号登录资料副本，避免继续使用旧 2FA。后续退出或重新授权失败不会撤销已经确认的新密钥。

| 显示结果 | 含义与后续处理 |
| --- | --- |
| 操作完成 | 所选安全步骤已经确认；未选择重新授权时到此结束。 |
| 已保存并启用 | 后续 OAuth 已保存到 CPA，凭证身份与启用状态核对通过。 |
| 新 2FA 已保存，后续步骤未完成 | 新密钥已生效并保留，退出或重新授权等后续步骤没有完整完成。 |
| 安全操作待核对 | 可能已发送修改，但缺少完整结果。保留记录与收到的新密钥，阻止重复安全操作、普通登录和自动重登。 |

进程重启只对账本地证据，不重发远端修改。未知操作继续阻止账号修改和删除，以免覆盖唯一恢复材料。账号资料中的「待核对资料」可以显示已经收到的新密钥；未确认激活的密钥仅作为恢复材料，不会当作有效登录凭据。

「完成本地保存」只接受已经确认激活的新因子和对应密钥，在本地完成剩余保存。它不会连接 OpenAI、重新激活、退出设备、重新授权或上传，也不能把未知激活结果变为成功。

## 插件管理接口

以下路由属于插件，由官方 CPA 的 management 鉴权保护；公共资源 GET 只提供静态页面。

`POST /v0/management/plugins/model-quality/login/accounts/security`

```json
{
  "id": "account-id",
  "revision": 3,
  "action": "reset_totp",
  "logout_all_devices": false,
  "reauthorize": true
}
```

`action` 仅允许 `reset_totp` 或 `logout_all`。独立退出操作使用 `logout_all_devices:false`。所有写请求都需唯一的 `Idempotency-Key`，重复提交同一意图返回同一任务，不重复发起操作。两个布尔字段均需明确给出。

`POST /v0/management/plugins/model-quality/login/accounts/security/complete`

```json
{
  "id": "account-id",
  "revision": 4,
  "job_id": "security-job-id"
}
```

任务返回 `action`、`security_status`、`totp_reset`、`devices_logged_out`、`reauthorize`、`logout_all_devices`。账号的 `security_pending` 表示仍有未解决的安全操作。鉴权账号详情在需要时附带 `security_recovery`；普通任务日志不包含密码、新旧密钥或会话令牌。

## 验证范围

实现验证使用合成账号、本地假上游和临时数据目录，没有对真实账号执行重置、退出设备或登录。插件回归覆盖接口幂等、互斥、确认新密钥后的失败保留、未知结果隔离、重启不重放、本地完成零网络，以及旧任务不得覆盖后续新密钥。引擎的请求顺序、身份和因子校验、持久化先于修改、未知结果不重发另有离线测试。

通过本地验证不代表当前线上 OpenAI challenge、账号状态或代理出口已经完成真实验收；官方要求的额外身份验证仍会作为实际错误或需交互结果呈现。

## 本机交付

8317 已于 2026-11-17T03:02:37.802278+08:00 更新为插件 0.15.0 / UI 9.2.0 / Go＋Sobek 2.1.0。官方 CPA 程序、配置和原凭证文件未改动，2 个账号、9 条历史任务及日志保留。停服备份：`artifacts/local-cpa/backups/account-security-0.15.0-20261117-030235`。没有执行真实账号安全操作。

已通过 74 项 UI 检查、13 项插件安全回归（含坏证据子用例）、插件与引擎 race/vet、官方安装/重启、回滚模拟及已安装 6 个 UI 资源摘要核对。证据见 [验证汇总](../artifacts/account-security-0.15.0/verification-summary.json)。

升级或回滚前需结束当前任务并处理未知安全结果。包含 `security_pending` 或未知修改证据的实例不能回滚到不认识这些状态的旧插件，也不能删除操作记录来解除隔离。
