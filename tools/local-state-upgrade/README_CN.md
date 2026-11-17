# 本地账号绑定一次性转换工具

此独立工具仅供停止旧 CPA、完成备份后的一次性本地转换；不随插件安装，不启动服务，不调用网络，不修改旧数据库或 CPA 凭证文件。插件 0.14 的生产代码及安装包没有因此修改。

```sh
./local-state-upgrade \
  --source /absolute/path/old/credential-login.db \
  --auth-dir /absolute/path/cpa/auths \
  --destination /absolute/path/plugin-data/accounts/credential-login.db \
  --quality-state /absolute/path/cpa/auths/quality-state.db \
  --quality-db /absolute/path/plugin-data/quality.db
```

目标数据库必须不存在，目标目录必须为 `0700`（不存在时创建），生成数据库为 `0600`。工具拒绝符号链接路径、非本地安全 JSON 文件名、非 schema 2 数据库，以及活动任务、待确认候选、未知提交或未采用结果。旧文件与本地凭证文件均只读打开，转换期间旧数据库保持 bbolt 只读锁。

先核对账号和 vault 的旧 UUID、principal、邮箱与实际 Codex JSON 内两个 token 的身份一致，再把身份版本转换为插件采用的 SHA256。仅变更 `accounts.View.IdentityVersion`、与原绑定精确匹配的已完成任务 `jobs.AdoptedIdentity`、`vault.Identity`。历史任务身份不匹配时保留其原始记录。未知账号绑定或冲突会拒绝整个转换。

完整复制原库所有桶、键值和桶序列，保留 HMAC、幂等记录、设置、密码、2FA、代理及未知字段。Gob 记录使用唯一长度前缀 UUID 的字节替换，只更新该字段和所属消息长度；不会用投影结构重编码和丢弃未知字段。同一记录出现多个相同 UUID 时拒绝。所有未改记录按原字节比较，成功后通过同文件系统 hard link 原子发布目标；不会覆盖已有目标。

成功 stdout 只输出 JSON 数量及源、目标 SHA256。失败 stderr 只输出固定错误标签，不包含邮箱、密码、token、账号 ID 或实际路径。转换失败时不发布目标数据库。报告中的哈希用于核对备份与结果，不是加密。

质量核验的两个参数必须成对使用；本次部署应在停服后传入。工具在目标创建前只读打开两库并持有文件锁至发布完成：宿主的有效暂停 Actions 必须为空，LoginCommits 只允许明确终态 `committed` / `aborted`；插件规则不得有 `action_id` / `blocked_auth_id`，测试运行只允许 `completed` / `failed` / `cancelled` / `interrupted`。发现有效暂停、遗留阻断、活动或不确定任务会拒绝转换，不清理任何限制。核验通过报告包含 `quality_checked:true`；省略两参数时为 false，不能当作已核验。

```sh
go test -race ./...
go vet ./...
go build -trimpath -o local-state-upgrade .
```

这里不转换质量暂停记录、不自动启用账号、不更改旧 runtime 文件夹；这些操作须由部署流程按已批准范围独立执行。成功后仍须核对新插件的账号数量、绑定状态及 CPA 凭证状态；本工具不验证真实 OpenAI 登录。
