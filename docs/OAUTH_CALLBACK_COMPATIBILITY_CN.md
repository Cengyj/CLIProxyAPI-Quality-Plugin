# OAuth 回调兼容修复

版本：插件 0.29.4 / UI 13.6.4 / Go＋Sobek 2.2.1。

## 现象与定位

用户提供的失败任务 `df8f6c41-515a-4a4f-a488-ef673399dbbc` 已完成密码验证、TOTP 验证、网页会话核对及 OAuth 会话选择。运行时随后发出 `callback` 进度，插件在数毫秒后报 `invalid_callback`，尚未进入授权码交换。

Go＋Sobek 的 `flow.callback` 只在回调地址、state 和 code 均通过校验后发出 `callback` 进度。插件接着经过 `runtimeResponse` 和 `Service.Callback` 两道校验，原先两道规则不一致：前者允许额外查询参数；后者只允许 `state/code/scope/session_state/error/error_description` 六个字段。

参考项目 `/Users/ceng/Downloads/toSub2-go-port-2026.11.17/internal/protocol/oauth.go` 的 `ParseOAuthCallback` 不拒绝可选提供方字段。其 `oauth_callback_boundary_test.go` 明确要求 `provider_field=retained` 的合法回调通过。参考源码哈希见 `artifacts/oauth-callback-0.29.4/reference.json`。

修复前回归已复现：同一合法回调添加可选字段后，运行时响应通过，插件二次校验失败，假账号完整登录也以 `invalid_callback` 停止。这与用户日志的失败位置吻合。历史日志未保存回调原文，无法据此确定该次实际包含的是 `iss` 或其他扩展字段；这些字段仅作为离线复现样例，不能当作真实响应证据。

## 修复边界

两个插件校验入口共用 `validateCallbackFor`，兼容不参与授权的未知查询参数。它们不会影响身份、凭证内容、交换地址或保存策略，仅 code 会传给原有 OAuth 交换逻辑。

继续要求回调协议、主机、端口和路径精确一致；拒绝用户信息、fragment、编码路径、畸形查询参数、错误/缺失 state、关键参数重复、无效 code、超过长度限制的响应。`error` 与 `code` 混合或空 error 会拒绝；正常授权拒绝按 `interaction_required` 处理。

本次没有更改登录引擎、密码/TOTP 流程、工作区选择、PKCE、令牌交换、凭证身份检查、启用策略或 CPA 宿主源码。历史失败任务不自动重放，使用新版本需要手动创建新的登录任务。

## 可诊断性

新增固定原因码并保留到任务和日志：

| 原因码 | 显示内容 |
| --- | --- |
| `callback_url_invalid` | 地址与本次授权不一致或格式无效 |
| `callback_parameters_invalid` | 参数格式无效或关键参数重复 |
| `callback_state_mismatch` | state 与本次授权不匹配 |
| `callback_code_invalid` | 缺少有效授权码 |
| `callback_authorization_denied` | 授权服务拒绝本次授权 |

移除原有“请复制完整回调地址”的通用错误说明，适配自动 2FA 登录。诊断只包含固定分类，不增加回调原文、授权码、state 或敏感参数日志。

## 验证

修复前失败日志见 `regression-before.log`；修复后插件全量测试、登录包 race、Linux 原生构建测试/vet、35 项前端测试通过。回归包含扩展字段、23 类异常回调、完整假账号的一次交换/保存/启用、重复回调阻止、失败诊断持久化和日志脱敏。

界面沿用现有布局，111 项布局与历史/设置/资料交互检查通过；隔离 CPA 验证原生加载与重启，真实 CPA 管理壳中的页面验证使用合成业务数据。未执行真实账号登录或模型调用。

[验证汇总](../artifacts/oauth-callback-0.29.4/verification-summary.json) · [修复前复现](../artifacts/oauth-callback-0.29.4/regression-before.log) · [插件测试](../artifacts/oauth-callback-0.29.4/plugin-tests.log) · [部署记录](../artifacts/oauth-callback-0.29.4/deployment.json)
