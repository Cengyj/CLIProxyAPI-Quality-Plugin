> 历史补丁宿主版本记录。当前官方兼容版请阅读 [官方单插件方案](OFFICIAL_PLUGIN_CN.md) 和 [安装说明](INSTALL_CN.md)。

# 账号、密钥与运行数据的持久化

> UI 8.9.0 更新：临时登录入口及表单已移除，账号列表不再显示登录出口列。代理继续通过账号资料、批次导入和“代理与巡检”配置。下面涉及旧临时登录流程的内容仅保留为历史说明，已有保存资料不会因移除界面而删除。

OpenAI 账号与临时登录资料直接以明文持久化在本机，不需要“启用加密保存”，不生成或依赖存储加密密钥。当前项目尚未上线，本轮按首次使用的明文数据格式实现，不提供历史加密数据迁移。数据由宿主统一保存；刷新页面、关闭浏览器或重启 CPA 不会删除已保存的账号和巡检设置。

本文根据当前源码、8317 配置中的路径字段及运行进程打开的文件核对。没有读取或记录真实密码、密钥、token 或数据库内容。部署结果以本轮安装记录为准。

## 1. 实际保存位置

当前安装根目录：

```text
/Users/ceng/Downloads/CLIProxyAPI-Quality-Plugin/artifacts/local-cpa/
├── config.yaml
├── access-info.json
├── kv.db
├── plugin-data/
│   └── quality.db
├── auths/
│   ├── <CPA 凭证名>.json
│   ├── quality-state.db
│   └── .credential-login/
│       ├── credential-login.db
│       └── runtime/                  # 本地登录运行包，不是账号数据库
└── backups/
```

| 数据 | 当前文件 | 持久化方式及用途 |
| --- | --- | --- |
| OpenAI 账号邮箱、密码、2FA 长期密钥 | `auths/.credential-login/credential-login.db` 的 `accounts` 桶 | schema 2 使用 Gob 编码保存完整明文 `Credentials`，不进行 AES 加密；可在账号资料页读取和修改 |
| 账号备注、CPA 绑定、自动授权许可、状态和时间 | 同库 `accounts` 桶 | 与账号资料、revision、稳定身份、失败次数、已消费异常证据和下次尝试时间一并保存 |
| 全局巡检开关、自动重登开关、分钟间隔 | 同库 `meta` 桶的 `account-settings` | 显式保存后生效；重启读取相同设置，默认关闭、15 分钟 |
| 临时登录中选择“保存资料”的邮箱、密码、2FA | 同库 `vault` 桶 | 与 CPA 凭证 ID、稳定身份及 revision 绑定的明文记录；和账号目录是两套记录，不会自动互相导入 |
| 登录任务、结果、错误码、幂等记录 | 同库 `jobs`、`idempotency`、`meta` | 保存任务状态与请求摘要；不把完整登录请求、密码、回调地址、PKCE 或 OAuth 候选 token 复制到任务历史 |
| OAuth `access_token`、`refresh_token`、`id_token` | `auths/<CPA 凭证名>.json` | 经候选确认并提交后，由宿主保存为正常的 Codex OAuth JSON；这些文件原本就不是 AES 加密存储 |
| 质量暂停归属、稳定身份、OAuth 提交收据 | `auths/quality-state.db` | 宿主权威状态；与账号登录数据库分开，保证暂停和未决提交不会因插件重启丢失 |
| 题目测试、回答、检测规则、设置、预设、审计与用量 | `plugin-data/quality.db` | 插件独立 bbolt 数据库，记录 JSON；不是 OpenAI 账号资料库 |
| CPA 通用 KV 状态 | `kv.db` | 宿主通用 bbolt，当前 `server.kv-store.path` 明确指向安装根目录的此文件；不是 `auths/kv.db` |
| 管理接口密钥的校验值 | `config.yaml` 的 `management.secret-key` | CPA 原生 bcrypt 单向校验哈希，不是可解密密文 |
| 管理密钥原值、业务 API Key 原值 | `access-info.json` | 本机安装辅助文件，保存原值；启动脚本用 `management_key` 执行就绪检查 |
| 业务 API Key 配置 | `config.yaml` 的访问配置 | CPA 业务鉴权配置，与 OpenAI 账号密码和 2FA 无关；不因本轮账号存储改造改变 |

bbolt 是嵌入式数据库，Gob 是 Go 的二进制序列化格式。文件不是文本格式，不代表内容被加密；schema 2 的账号密码和 2FA 长期密钥可以由对应数据库读取代码直接还原，不需要额外密钥。

当前上述配置及数据库文件权限为 `0600`；登录资料目录要求 `0700`。文件权限仍保留，用于限定本机操作系统账户的读取范围，不属于加密。

## 2. 保存与读取的完整流程

### 导入账号

1. 页面按每行 `邮箱----密码----2FA长期密钥` 解析输入。
2. 通过受管理鉴权保护的 `POST /v8/management/credential-logins/accounts/import` 发送给宿主。
3. 宿主校验邮箱、密码、TOTP 格式、重复账号和数量限制。
4. 宿主在一个 bbolt 写事务中保存整批账号；任一条失败则整批不写入。
5. 写入成功后返回账号状态，页面清空导入框。导入动作不发起 OpenAI 登录。
6. 再次打开页面，`GET /accounts` 从宿主获取列表；点击“账号资料”时，`GET /accounts/detail?id=...` 获取完整邮箱、密码和 2FA。

账号备注、自动化开关和密码更新均使用账号 `revision`，避免旧页面覆盖其他操作的新数据。保存账号不依赖页面继续打开。

### 手动登录与 OAuth 凭证

账号目录中的保存资料用于生成 2FA 登录请求。临时登录若未勾选保存，只在本次宿主任务中使用；若勾选保存，在授权提交成功后写入 `vault` 桶。

登录中的授权 URL、PKCE verifier、OAuth state、一次性回调和未提交的候选 token 保留在宿主会话内存；本地浏览器运行包使用独立非持久化浏览器上下文，不复用用户 Chrome 个人资料，不导出 Cookie 或 storageState。任务结束或取消后关闭该上下文。

首次授权及普通手动重登仍需要核对候选并点击确认。提交先把凭证文件写入临时文件，执行落盘同步，再原子发布，并记录提交收据和采用结果。新凭证默认禁用。重新授权更新 token 字段，同时保留已有人工禁用、质量暂停、冷却和其他限制。

此登录提交路径只支持 standalone 本地文件型 Codex OAuth 凭证；远端 PostgreSQL/Git/对象存储不能因为存在本地缓存目录就当作已支持。账号资料库也不会自动同步到这些远端存储。

### 巡检和自动重登

账号与设置保存在宿主中，宿主读取 CPA 已记录的认证状态。浏览器关闭后仍可按保存的间隔巡检；宿主停止时任务自然停止，宿主重新启动后会恢复已有数据库的调度。

“定时巡检”“全局自动重新授权”“该账号允许自动授权”是独立保存的条件。修改未保存的页面草稿不会改变服务端。只有已绑定账号、明确认证失效和其他执行条件同时满足才会自动重登；普通网络错误、额度不足、人工禁用、质量暂停或冷却都不会变成自动恢复授权。

## 3. 不同“密钥”的区别

- **2FA 长期密钥**：用于本机计算六位 TOTP 动态码。与邮箱和密码一起明文保存；数据库不需要保存每次生成的短期动态码。
- **OAuth token**：模型调用使用的授权材料，在 CPA 凭证 JSON 文件中持久化；不等于 OpenAI 登录密码。
- **管理密钥**：用于连接 CPA 管理接口。本机 `access-info.json` 保存原值，CPA 配置保存 bcrypt 校验哈希。bcrypt 不可解密，也不是“加密保存账号”，因此保留原生鉴权实现。
- **HMAC key**：登录数据库 `meta/hmac` 中的 32 字节随机值，用于稳定生成账号去重、限流和幂等请求指纹。HMAC 不是账号加密，不能用它解出密码；它与数据库一起保留，不能每次重启重新生成。

## 4. 浏览器究竟保存什么

UI 8.3.1 起，插件成功连接后将自己的管理密钥明文写入当前源的 `localStorage`，键名为 `model-quality:management-key:/v8/management/plugins/model-quality`。浏览器按协议、域名和端口隔离该值，插件键名另外限定管理接口路径。刷新、重新打开或浏览器后退恢复页面时读取它并恢复管理连接；请求仍由 `ManagementClient` 通过 Authorization header 发送。没有加密或混淆，也不写入 `sessionStorage`、Cookie、URL 或导出文件。

主动点击“断开”或管理接口返回 401 会清除已保存的密钥。临时网络故障、服务错误不会删除已保存值；自动重连只读取管理数据，不提交模型测试或登录任务。浏览器阻止本地存储时会提示，连接降级为仅当前页面内存可用，刷新后需要重新输入。

账号密码、2FA、OAuth 回调和未保存的导入内容不写入浏览器存储。切换登录视图、关闭账号编辑器、断开连接或遇到 401 会清空当前显示的秘密输入，并阻止旧异步响应重新填回。

账号数据的持久化发生在宿主数据库中。清除浏览器站点数据会删除浏览器保存的管理密钥，但不会删除宿主账号；重新连接后列表仍在。未经保存的导入文本、临时输入和设置草稿只是页面内存，刷新会丢失。

CPA 外层管理后台自己的“记住密码”是另一套行为：固定参考前端把登录状态写入浏览器 `localStorage` 的 `cli-proxy-auth`，仅勾选记住时包含管理密钥。其 `enc::v2::` 格式使用 XOR/Base64 的可逆混淆，源码已明确它不是加密安全边界。该父页面不是本插件 UI，本轮没有改动它；插件也不会读取父页面的登录存储。

## 5. 重启、更新和删除的结果

| 操作 | 数据结果 |
| --- | --- |
| 刷新插件或关闭浏览器 | 已保存账号、凭证和设置仍在；浏览器保留已成功连接的管理密钥，下次打开自动重连；未保存草稿消失 |
| 点击插件“断开”或管理接口返回 401 | 清除浏览器保存的插件管理密钥和当前页面管理数据，不删除宿主账号与任务 |
| 清除浏览器站点数据 | 删除浏览器保存的管理密钥；宿主账号、凭证与设置保持不变 |
| 重启 CPA，沿用相同数据路径 | 重新读取账号、设置、规则、凭证和暂停状态；不会自动重放中断的手动登录 |
| 替换插件共享库 | 数据文件不随动态库替换而删除；当前宿主按 schema 2 读取登录资料 |
| 删除账号目录中的账号 | 删除该账号记录及自动化许可；不删除对应 CPA OAuth 凭证，不撤销远端授权，不自动删除单独的临时登录保存记录 |
| 删除临时登录的已保存资料 | 删除 `vault` 条目；CPA 凭证、账号目录及远端授权保持 |
| 更改数据目录或只复制二进制 | 新位置不会自动找到旧数据，需要保留或迁移原数据库和配置路径 |

重启时，未到提交阶段的活动任务标为 `interrupted`，不会恢复旧 PKCE 或回调继续登录。已经进入提交阶段的任务根据宿主 `quality-state.db` 中的收据对账；无法确定结果时保留 `commit_unknown` 或 `saved_not_adopted`，不会把旧 token 写回或无条件重复登录。

登录终态任务与已完成幂等记录通常保留 30 天；未决提交受到保护，不按普通历史清除。账号、巡检设置和有效质量暂停不会因这项历史保留期自动删除。bbolt 删除属于逻辑删除，旧页面可能保留在数据库空闲页或既有备份中，不等于磁盘安全擦除。

## 6. 首次使用与开发库重置

登录数据库直接使用 schema 2。首次打开时，宿主创建本机数据库、账号和临时保存资料桶，以及用于幂等和去重的 HMAC 元数据。明文保存默认可用，没有加密初始化步骤或兼容初始化接口。

当前实现不支持旧 schema 的开发数据库，也不执行解密或格式迁移。检测到其他 schema 时返回 `storage_unavailable`，不会擅自覆盖或清空已有数据库。

UI 8.1.0 改为明文存储的那次部署，先确认开发环境中的账号、登录任务及已保存登录资料均为空，再正常停服并备份原数据文件，之后重新初始化登录数据库。当前本机已经有账号，后续更新保留此数据库，不再按空库处理。UI 8.4.0 的代理字段保存在同一 schema 的账号与设置记录中，旧记录缺省为空，沿用代理继承行为，不执行数据迁移或清空。

## 7. 备份与恢复应包括哪些内容

本机完整数据备份至少包括：

1. `config.yaml` 与 `access-info.json`，保留管理连接和当前绝对路径配置。
2. 整个 `auths/`，包含全部 OAuth JSON、`quality-state.db` 和 `.credential-login/credential-login.db`；有原生冷却文件时也一并保留。
3. 整个 `plugin-data/`，保留插件测试、规则、历史与设置。
4. 当前实际使用的 `kv.db`。
5. 与数据库兼容的宿主二进制、插件共享库和登录运行包，或能重建这些版本的源码及构建记录。

最直接的本机迁移单位是整个 `artifacts/local-cpa/`，但其中包含运行文件、日志和历史备份。精简备份可以按上述清单保存。不要仅备份 `model-quality.dylib`，也不要仅备份账号数据库后丢掉 OAuth 凭证和质量暂停状态。

复制数据库前先使用 `sh scripts/local_cpa.sh stop` 正常停服，等进程退出、数据库关闭后复制同一个时间点的文件。恢复时也先停服，再恢复成套文件、权限和匹配版本；保留原路径时无需修改配置，换目录时要更新 `oauth.auth-dir`、`plugins.configs.model-quality.data_dir`、`server.kv-store.path`、插件目录及本机启动配置。最后使用 `sh scripts/local_cpa.sh start` 启动并核对账号数量、设置、凭证列表和暂停归属。

当前更新先检查活动任务，停服并备份数据库、插件数据、配置、KV、宿主、插件和登录运行包，再替换构建产物。保留已有账号、登录任务、代理及巡检设置，不重新初始化数据库。备份保留在独立目录中；更新前后核对业务状态摘要。

## 8. 主要源码依据

- `workspaces/CLIProxyAPI/internal/authlogin/store.go`：数据库、schema、HMAC、幂等记录和历史保留。
- `workspaces/CLIProxyAPI/internal/authlogin/accounts.go`：账号资料、绑定、巡检设置和自动化状态。
- `workspaces/CLIProxyAPI/internal/authlogin/vault.go`：临时保存资料、revision 和身份校验。
- `workspaces/CLIProxyAPI/internal/authlogin/service.go`：登录会话内存与重启对账。
- `workspaces/CLIProxyAPI/internal/api/handlers/management/credential_logins.go`：资料目录和宿主重启后恢复巡检。
- `workspaces/CLIProxyAPI/sdk/cliproxy/auth/credential_login.go`、`credential_login_journal.go`、`quality.go`：OAuth 原子提交、收据和暂停权威存储。
- `plugin/quality/store.go`、`engine.go`：插件业务数据库和恢复。
- `plugin/ui/app.js`、`client.js`、`login.js`：管理密钥持久化与恢复、页面内存、鉴权和清除行为。
- `scripts/local_cpa_session.py`：管理密钥就绪检查与当前会话启动方式。

## 9. 本机验证结果

2026-11-16 13:47:03（北京时间）已更新本机 CPA 8317 为插件 0.7.0 / UI 8.1.0。更新前核对为 0 个账号、0 个登录任务、未启用过保存资料；备份后重新创建空的 schema 2 登录开发库，没有执行数据迁移。宿主返回 `storage_mode=plaintext`，无需初始化即可保存，未生成存储加密密钥。其他配置、凭证、规则、暂停、历史、巡检设置和运行包摘要保持一致。

16 份最终验证记录通过，含宿主全套测试/vet、相关 race、插件测试/race/共享库构建、原生生命周期和 CPA 内嵌 UI。账号 UI 26 项、登录 UI 22 项通过；明文保存、重启读取、身份约束和 CAS 均使用离线假账号验证，没有调用真实模型或执行真实 OpenAI 登录。

本轮备份：`artifacts/local-cpa/backups/plaintext-storage-8.1.0-20261116-134700`。证据：`artifacts/plaintext-storage-8.1.0/verification-summary.json`、`deployment-checks.json`、`installed-browser-checks.json` 和 `verification-final/`。
