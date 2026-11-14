# model-quality 0.3.1 原生插件

独立 Go `/v8` module 与 C ABI 插件，提供“智商测试”和“质量运维”两个入口。

**智商测试只做题目测试。** 选择凭证、一道题和一个模型，设置思考强度与 1–8 份回答，点击开始后在同一页面查看回答。提供糖果逻辑测试、鹈鹕骑自行车（HTML）及自定义题目；题面可以编辑，改动后按自定义题冻结，不沿用内置答案。手动测试不自动评分，支持停止、复制、HTML 安全预览、下载结果和测试记录，不包含探针或 ModelTrace。

质量运维保留独立的检测规则、调度、判题与凭证暂停管理。质量暂停影响整个凭证的所有模型；手动题目测试结果不用于自动处置。

**当前规则：确认异常并暂停凭证后，自动复测和手动复测全部禁止，只能由管理员人工解除。解除后检测规则仍停止，须单独恢复规则才会再次检测。** 默认失败门槛仍为连续三轮所有选定模型明确失败；手动测试不自动处置。

本交付需要带quality capability v2补丁的宿主，不是原版8.0.67即插即用。默认只观察；定时检测与自动暂停分别启用，不存在自动恢复开关。

当前源码为 **插件 0.3.1 / UI 4.2.0**，使用方式见 [页面说明](docs/UI_CN.md)。本轮验证材料归档于 `artifacts/question-test-4.2.0/`，以实际生成的日志、报告和截图为准。[4.1.0 界面报告](docs/UI_POLISH_REPORT_CN.md) 和 [4.1.0 候选包说明](docs/CANDIDATE_RELEASE_CN.md) 保留为历史记录；它们以及旧安装包不能证明当前源码已经验收。

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

演示只使用 loopback fake upstream、临时假密钥和独立数据。打开页面、连接、编辑题面和保存规则不调用模型；必须显式开始测试或执行规则。演示可查看暂停及人工释放；已暂停凭证不能用于复测。旧版预设 API 与持久化记录保留兼容，UI 4.2.0 的智商测试页不再提供预设、对照或独立判题配置。

升级到schema2保留历史与暂停归属，旧复测/恢复字段不再生效。卸载插件不释放宿主暂停。回滚旧宿主前必须按安装说明处理限制并通过rollback_guard，不能删除数据库。

自动证据当前覆盖原生 OpenAI-compatible chat/completions；Home 动作不支持。历史测试范围见 TEST_REPORT_CN.md，UI 4.2.0 的实际平台和验收以 `artifacts/question-test-4.2.0/` 本轮记录为准。完全相同材料在宿主停机期间删除重建的身份识别限制见 DESIGN_CN.md。

原始 `/Users/ceng/Downloads/CLIProxyAPI-8.0.67` 保持只读；baseline.json记录2,303个文件SHA-256，补丁应用可复现。
