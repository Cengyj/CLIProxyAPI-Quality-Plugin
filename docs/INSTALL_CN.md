# 0.3.0 / UI 4.1.0 安装、升级与回滚

本轮安装产物位于独立 `artifacts/candidates/polish-4.1.0-rc1/`。请使用含 `ui-4.1.0` 与构建标识的归档及同目录校验文件；根 `artifacts/model-quality-0.3.0-darwin-arm64.tar.gz` 是保留的旧 UI 3.0.0 历史包。完整校验与解包实测步骤见 [CANDIDATE_RELEASE_CN.md](CANDIDATE_RELEASE_CN.md)。本任务只生成本地候选，不替换正式部署。

## 从 UI 4.0.0 升级

后端版本仍为 0.3.0，界面随重新编译的动态库升级为 4.1.0。停止旧实例、备份宿主和插件数据后替换候选包中的动态库；需要完整宿主时使用同包补丁宿主。保留两个质量数据库，不清理暂停、不恢复规则。重启后检查页面源码 `build-info` 中的 UI 4.1.0，再核对宿主限制。无需单独覆盖静态目录。

## 必要宿主与平台

本插件需要带补丁的 CPA 宿主，不是对原始8.0.67归档即插即用。最低要求为 **quality capability v2**：独立v8插件路由、实例级operation/cancel、持久质量暂停、最终条件证据、禁止复测与仅鉴权管理请求可人工释放。插件检查能力，不把旧宿主当作支持新语义。

实际验证平台和本轮测试结果见 TEST_REPORT_CN.md。既有平台为macOS Darwin arm64、Go1.27.1，两个module的go directive为1.26.10。其他平台需用对应C/Go工具链重新构建并实测；macOS dylib不能直接用于Linux/Windows。

项目尚未发布GitHub仓库，metadata来源字段明确为 `local-workspace:CLIProxyAPI-Quality-Plugin`。

## 构建与源码补丁

```sh
./scripts/build.sh
./scripts/verify.sh
```

分别测试插件和宿主module，执行宿主必需的test-output编译检查，生成动态库、C头文件和 `artifacts/cli-proxy-api-quality`。前端构建/测试见UI_CN.md，完整离线演示见DEMO_CN.md。

若应用到同一归档的干净源码副本：

```sh
./scripts/apply_host_patch.sh /absolute/path/to/clean-source-copy
```

脚本逐文件校验baseline，先git apply --check再应用。原归档无Git元数据；本地补丁基线提交不是上游提交。不要对原始只读参考、生产运行目录或其他版本直接应用。源码与所需资源白名单不包含生产配置、凭证、日志、数据库或程序。

## 新安装

1. 停止旧宿主，备份配置、auth-dir与插件data_dir。
2. 安装补丁宿主，将动态库放在独立plugins目录，文件名保持model-quality.dylib（对应平台扩展名）。plugin ID来自文件名。
3. 合并 `artifacts/config/model-quality.example.yaml` 的plugins段。生产使用绝对路径；插件数据目录独立、可写且限制权限。
4. standalone正常管理鉴权启动，保持 scheduling_enabled和automatic_pause关闭。不存在自动恢复开关。
5. 进入 `/v0/resource/plugins/model-quality/index.html`，输入管理密钥；它仅保存在页面内存。业务API全走鉴权v8，遵守remote-access。
6. 先用离线目标验证，再显式启用规则/全局开关。保存规则或预设不等于执行。

Home质量动作不支持。不要通过插件绕过Home控制权。

## 从0.2.0升级

宿主最低能力继续为 v2，插件库升级为0.3.0；停止旧实例后替换动态库，保留原插件quality.db和宿主quality-state.db。schema2继续使用，已有规则、历史与暂停归属不变。新增前端资源随动态库嵌入，不需要单独部署。升级不会恢复停止的规则或解除任何凭证暂停。

## 从0.1.0升级

新语义是“暂停后不再检测，仅人工解除”，并非仅调整默认值。升级前关闭旧宿主定时/自动动作、停接新检测、取消并等待旧任务收尾；停止宿主后同时备份宿主quality-state.db和插件quality.db，再替换宿主与动态库。

插件schema1迁移到schema2保留历史、预算、动作归属和审计。旧automatic_recovery、pass_threshold、pass_count、retest_seconds不再起作用；未完成任务标记interrupted，不补发。已经暂停及待确认动作进入等待人工处理状态，相关检测规则停止。旧配置中的恢复true不会触发模型调用或自动释放；新API不能重新启用这些已移除参数。

完成升级后核对活动暂停和规则状态；不要为验证升级而对暂停凭证发起复测。活动暂停的正确验证是：业务与评测请求都受阻，管理页面可查看归属并人工释放。

新宿主还会拒绝旧quality_retest和自动恢复回调，避免误加载旧动态库获得绕过权限。旧插件不能直接读取schema2数据库；不要通过删库或降级schema字段绕过检查。

## 人工处理与卸载

质量暂停会停止该凭证所有模型的新业务与评测执行，也会停止把它作为control/judge的相关检测。规则保持awaiting_release，不再自动复测。

管理员预览动作后人工CAS释放。插件会先停止相关规则、清空连续计数并使旧任务版本失效；释放成功后规则仍停止，没有剩余质量限制时为paused，否则继续awaiting_release。需要继续检测时，单独恢复规则，服务端重新检查所有依赖和剩余限制。

人工释放只删除匹配的质量限制，不清除人工禁用、原生冷却或其他owner。插件quiesce、shutdown、卸载和进程重启保留宿主暂停。插件不可用时仍可使用：

- GET `/v8/management/credentials/quality-pauses`
- POST `/v8/management/credentials/quality-pauses/release`

POST提交GET返回的完整绑定（auth_id、owner_plugin、rule_id/revision、action_id/revision、identity_version等）。这是鉴权管理员处理孤立/已替换/已删除目标动作的入口，不依赖插件加载。处理后仍检查剩余限制。

## 回滚保护

不识别质量库的旧宿主可能放行暂停凭证；旧0.1.0插件也包含已废弃复测/自动恢复逻辑。不能只替换二进制、删除数据库或修改schema编号完成回滚。

1. 停入口业务流量，关闭调度/自动暂停，暂停规则，取消并等待所有评测操作。
2. 保持新宿主运行，核对每个活动暂停。需要继续隔离的凭证先通过原生持久管理操作人工禁用并验证来源行为；不能精确持久禁用的类型继续使用新宿主，或不携带这些凭证回滚。
3. 管理员明确确认后逐项CAS释放已妥善处理的质量限制，不盲目批量释放。
4. 执行只读保护脚本：

```sh
export CPA_MANAGEMENT_URL=http://127.0.0.1:8317
# Set CPA_MANAGEMENT_KEY securely in this process environment.
python3 scripts/rollback_guard.py
```

有暂停或查询失败时脚本拒绝继续。零暂停且原生隔离已确认后，停止宿主并保留两个数据库备份，再切换版本。回滚旧宿主时保持model-quality禁用；不要恢复旧自动复测行为。若确需旧插件/数据库备份，必须另行审核其旧语义和备份以来的动作差异，不能直接覆盖当前权威库。

## 身份维护边界

普通token刷新不作为身份替换；已观察删除重建与材料更改使旧证据失效。宿主停机期间完全相同材料的文件删除重建不可从现有API识别。此类维护先显式解决旧归属，再通过受管理流程导入，不能依赖旧结果证明新身份。详见DESIGN_CN.md。
