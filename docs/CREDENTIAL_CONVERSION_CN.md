# sub2api → CPA 凭证转换

版本：插件 0.20.0 / UI 12.10.0 / Runtime 2.2.1。仅支持 OpenAI / Codex OAuth。

## 使用

在「账号管理」工具栏点击「转换凭证」，选择 sub2api 导出的 JSON 文件，或粘贴 JSON，点击「开始转换」。核对每条凭证的邮箱、套餐和结果后，可下载 CPA JSON，或勾选有效条目后一键「上传 N 个凭证」。每次最多 100 条、2 MiB；不支持 API Key、Cookie 或 Claude。

支持完整账号导出、`data.accounts` 包装、账号数组、单个带 `credentials` 的账号对象以及单个凭证对象。转换只在当前 CPA 本地完成，不登录、不刷新 token。结果中的凭证可下载为独立 JSON；输入与转换结果只在当前弹窗会话中保留，关闭或断开连接即清空。

上传目标是当前 CPA 的凭证列表。导出内容没有密码和 2FA 时，不会伪造或创建插件账号管理资料。初始禁用状态会保留，其他新凭证上传后启用。已有同身份或同名凭证会被拒绝覆盖；套餐排除策略同样生效。

## 字段依据

参考仓库：`ranxi2001/sub2api`，核对提交 `e687827fcb5c2bbb347dfb1d15f116c0ab0580c9`。依据 `backend/internal/handler/admin/account_data.go` 的导出结构，以及 `backend/internal/service/openai_oauth_service.go` 的 OAuth 字段生成逻辑。源码摘要保存在 `artifacts/credential-conversion-0.20.0/reference/manifest.json`。

| sub2api | CPA Codex JSON |
| --- | --- |
| `platform: openai`、`type: oauth` | `type: codex` |
| `credentials.access_token` | `access_token`，原值保留 |
| `credentials.refresh_token`、`id_token` | 原值保留；缺失不伪造 |
| `chatgpt_account_id` / token 账号声明 | `account_id` |
| `chatgpt_user_id` / token 用户声明 | `user_id` |
| `email` / token 邮箱声明 | `email` |
| `plan_type` / token 套餐声明 | `plan_type` |
| 凭证内部 `expires_at`、`expired` 和 access token `exp` | 取已知最早时间，写为 RFC3339 `expired` |
| 凭证内部 `last_refresh` | 有值且有效时保留 |
| `disabled` / inactive、disabled 状态 | 保留禁用 |

账号对象顶层 `expires_at` 是 sub2api 管理到期时间，不作为 token 到期时间。分组、代理、并发、调度、`extra` 等配置不迁移。

## 校验与持久化

- 拒绝重复 JSON 字段、过深嵌套、错误类型、身份矛盾和缺少稳定账号／用户身份的 token。JWT 只用于结构解析，不代表签名或实时有效性已验证；CPA 真正使用 token 的可用性由上游决定。
- 无 ID token 的有效导出可以转换，access token 中的身份仍必须可核对。缺少有效期或 refresh token 会提示；已经过期且没有 refresh token 的条目不能上传。
- 上传时重新解析输入，重新检查当前套餐策略、同邮箱／同身份未完成的安全任务、已有 CPA 凭证、历史暂停准备检查，以及插件存活状态。
- 写入前持久化身份回执，写后核对物理文件和运行时采用状态。丢失响应或运行时未确认的项显示「待核对」，换请求标识、重开弹窗或重启不会再次写入。
- 上传请求的幂等记录不存放 token；token 只通过官方 `host.auth.save` 写入 CPA 正式凭证文件。转换预览不进入幂等缓存，响应为 `no-store`。
- 官方 SDK 没有跨进程原子 create-only / CAS 接口。插件串行化自身提交并在保存前复查，不能宣称与同时从其他 CPA 管理入口写入之间具备原子互斥。

接口使用官方已有的鉴权插件通道：`POST /v0/management/plugins/model-quality/login/conversion/preview` 和 `/conversion/upload`。宿主源码和镜像未修改。

## 验证

本轮插件完整 Go 测试、登录模块竞态检查、Go vet、34 项前端测试通过。111 项布局检查和转换弹窗的 1440 / 1064 / 620 / 390 / 320px 检查通过，覆盖下载、批量选择、失败条目、忙碌保护、未知响应禁止重复提交、关闭清空输入，以及上传成功后其他工作区刷新失败仍保留成功结果。

Linux arm64 原生包在无外网的空 CPA 容器加载并重启，通过真实官方保存回调上传合成 JWT 凭证，核对运行时启用，以及重启后相同身份没有重复写入。宿主 2,413 个源文件与基线一致，宿主编译及测试命中缓存，WebRTC 测试另在内部网络重新执行通过。日常验证没有使用真实账号、调用模型、执行登录或安全操作。

构建、测试、部署、真实内嵌页面的资源校验与合成 UI 交互记录均在 `artifacts/credential-conversion-0.20.0/`。当前平台安装包仅适用于本次匹配的 CPA 8.0.76 / Linux arm64 构建环境。
