# model-quality 0.3.0 原生插件

独立Go `/v8` module与C ABI插件，提供“手动智商测试”和“质量运维”两个页面。质量暂停影响整个凭证的所有模型。

**当前规则：确认异常并暂停凭证后，自动复测和手动复测全部禁止，只能由管理员人工解除。解除后检测规则仍停止，须单独恢复规则才会再次检测。** 默认失败门槛仍为连续三轮所有选定模型明确失败；手动测试不自动处置。

本交付需要带quality capability v2补丁的宿主，不是原版8.0.67即插即用。默认只观察；定时检测与自动暂停分别启用，不存在自动恢复开关。

## 阅读入口

- [本次架构复核与逻辑调整](docs/ARCHITECTURE_REVIEW_CN.md)
- [本轮架构与代码质量改进](docs/REFACTOR_CN.md)
- [版本变更与0.1.0迁移](docs/CHANGELOG_CN.md)
- [安装、升级与回滚](docs/INSTALL_CN.md)
- [设计与能力边界](docs/DESIGN_CN.md)
- [API和存储](docs/API_CN.md)
- [宿主v2扩展](docs/HOST_EXTENSION_CN.md)
- [测试报告](docs/TEST_REPORT_CN.md)
- [离线完整演示](docs/DEMO_CN.md) · [前端验证](docs/UI_CN.md)
- [源码参考与许可](docs/SOURCE_AUDIT_CN.md)

源码在plugin/；可编辑宿主在workspaces/CLIProxyAPI/；固定sub2api参考在workspaces/sub2api/；源码补丁与准确基线在host-patches/；构建产物、日志和截图在artifacts/。

```sh
./scripts/build.sh
./scripts/verify.sh
python3 scripts/demo.py start
python3 scripts/demo.py status
python3 scripts/demo.py stop
```

演示只使用loopback fake upstream、临时假密钥和独立数据。打开页面、连接、保存规则或预设不调用模型。演示可使用界面查看暂停及人工释放；已暂停凭证不能用于复测。

升级到schema2保留历史与暂停归属，旧复测/恢复字段不再生效。卸载插件不释放宿主暂停。回滚旧宿主前必须按安装说明处理限制并通过rollback_guard，不能删除数据库。

自动证据当前覆盖原生OpenAI-compatible chat/completions；Home动作不支持。平台和最新验收以TEST_REPORT_CN.md为准。完全相同材料在宿主停机期间删除重建的身份识别限制见DESIGN_CN.md。

原始 `/Users/ceng/Downloads/CLIProxyAPI-8.0.67` 保持只读；baseline.json记录2,303个文件SHA-256，补丁应用可复现。
