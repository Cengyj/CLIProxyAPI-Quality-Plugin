# 补丁基线与应用

本目录的 model-quality.patch 是源码补丁，包含新增 SDK/宿主实现与回归测试，不包含 auth、配置密钥、数据库、动态库或运行日志。

基线为用户给定 `/Users/ceng/Downloads/CLIProxyAPI-8.0.67` 归档。该目录不存在 Git 元数据；8.0.67 是目录标签，源码 buildinfo.Version 默认 dev。baseline.json 保存所有 2,303 个允许复制的源码/必要构建文件 SHA-256。工作副本的首次 Git 提交仅用于生成 patch，不是上游提交。

在干净的同一归档源码副本上执行：

```sh
/path/to/CLIProxyAPI-Quality-Plugin/scripts/apply_host_patch.sh /path/to/clean-source-copy
```

脚本校验全部基线并先运行 git apply --check。应用不要求目标目录存在 .git。不要直接在原始只读参考目录操作，不适用于不同版本。应用后遵守 AGENTS.md，运行 gofmt、全量测试、必需 test-output 编译检查；插件是另一 module，必须单独测试和 c-shared 构建。

`python3 scripts/verify_patch.py` 已在临时源码副本应用本补丁，并逐字节确认结果与已验证的宿主工作副本相同，日志在 artifacts/verification/patch-reproduction.log。

补丁内容：独立持久 quality authority；身份/动作 CAS、禁止复测和自动恢复、作用域受限的人工释放；调度/重试/最终 HTTP/WS 准入；插件 v8 精确路由；实例级操作取消；最终 payload/配置/上游终止证据；可信用量来源；独立人工释放管理入口及测试。既有 v0 管理端点没有新增业务或修改分发行为。

另外修复了基线验证中复现的两处测试问题及一个退出同步问题，详见 docs/TEST_REPORT_CN.md。未升级 upstream、未修改独立 translator 架构。

回滚旧宿主前请阅读 docs/INSTALL_CN.md。旧宿主忽略权威暂停库；不能删除数据库后回滚或卸载插件来实现恢复。
