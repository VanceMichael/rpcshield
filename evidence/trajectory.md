# 让 analyzer verdict 可安全跨进程复用

> **Workspace:** /Users/vance/project/mineProject/swe/09101505/project-03

---

**User**

在同一源码版本上反复扫描大型服务的使用者，希望第二个进程能复用上一轮已经完成的 caller/callsite verdict，而不是因 SSA 指针全部重建再次执行 SCCP、SSA、AST 或 LLM。请为启用 Cache analyzer 的 CLI 和库调用提供可选的持久缓存位置；未配置时继续采用现有进程内行为。持久 key 必须由跨进程稳定的函数与调用点身份、被分析代码版本、工具或 schema 版本以及会影响 verdict 的 analyzer 配置共同决定，源码、调用位置、analyzer 顺序或相关选项变化均应产生 miss。版本化记录要保留 failness 和真正的 OriginalSource，只有 analyzer 链完整得到的可复用结论才能提交；error、panic、FailUnknownWithError、缺失来源或未完成输出不能在后续运行中成为命中。多个进程写同一缓存时不能互相覆盖有效记录，并以协调写入、合并、安全替换和版本校验保证中途失败后仍可读取旧数据。缓存损坏、schema 不支持或目录不可写只能让本次分析退回正常 analyzer 链并给出可观察诊断，不能改变最终 verdict；文件修复或权限恢复后能够重新建立缓存。请在结果 metrics 中区分进程内命中、持久命中、miss、失效或拒绝记录以及写入降级，并用跨 Driver/进程复用、并发写、源码或配置失效、损坏与只读目录场景验证。

**Assistant**

我先了解现有代码库的结构，再制定实现方案。

toolName: view_folder

status: success

/Users/vance/project/mineProject/swe/09101505/project-03

这是一个 Go 静态分析项目。让我先阅读关键文件，理解现有的 analyzer 链、Driver 和缓存机制。

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-03/go/driver.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-03/go/cmd/rpcshield/common/cache.go

现在查看 cacheanalyzer 包和 analyzer 链的核心代码。

toolName: file_search

status: success

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-03/go/staticanalyzer/cacheanalyzer/cacheanalyzer.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-03/go/staticanalyzer/cacheanalyzer/cacheanalyzer_test.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-03/go/constant/constant.go

让我继续阅读 driver.go 中 analyzeEdge 的完整实现，了解 cache analyzer 的使用方式。

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-03/go/driver.go

理解了核心分析链。现在查看 CLI 入口和 output 包结构。

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-03/go/cmd/rpcshield/main.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-03/go/cmd/rpcshield/wpamode/wpa.go

toolName: search_by_regex

status: success

toolName: search_by_regex

status: success

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-03/go/driver_test.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-03/go/driver_test.go

我已理解整体架构。让我再查看几个关键文件：BUILD 配置、go.mod 依赖、以及库调用方示例。

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-03/go/staticanalyzer/cacheanalyzer/BUILD.bazel

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-03/go/go.mod

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-03/go/pipeline_evaluation_scripts/cmd/oss_pipeline_bench/main.go

现在查看测试辅助工具（如何加载 SSA 程序）和 BUILD 依赖命名约定。

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-03/go/staticanalyzer/ssaanal/simple_test.go

toolName: file_search

status: success

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-03/go/staticanalyzer/function/function_test.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-03/go/rpcshieldtest/testhelper.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-03/go/loader/loader.go

toolName: file_search

status: success

toolName: search_by_regex

status: success

现在信息基本足够。建立任务清单并开始实施。

toolName: todo_write

status: success

Todos updated: 8 items

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-03/go/rta/rtatest/helper.go

toolName: Shell

status: success

开始实现。先确认本地是否有 bazel 生成的测试夹具目录，并查看相关 BUILD 文件。

toolName: Shell

status: success

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-03/go/BUILD.bazel

检查 `function.Run` 和 LLM 返回的 AnalyzerOutput 是否设置了 OriginalSource（这决定缓存提交门控）。

toolName: search_by_regex

status: success

toolName: search_by_regex

status: success

所有 analyzer 都会正确设置 OriginalSource。现在开始写代码。先重写 cacheanalyzer.go：

toolName: Write

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-03/go/staticanalyzer/cacheanalyzer/cacheanalyzer.go

changes: undefined

现在实现跨进程稳定身份 identity.go：

toolName: Write

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-03/go/staticanalyzer/cacheanalyzer/identity.go

changes: undefined

修正 identity.go 中的类型引用错误和多余 import：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-03/go/staticanalyzer/cacheanalyzer/identity.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-03/go/staticanalyzer/cacheanalyzer/identity.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-03/go/staticanalyzer/cacheanalyzer/identity.go

changes: undefined

现在实现持久化存储 persistent.go：

toolName: Write

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-03/go/staticanalyzer/cacheanalyzer/persistent.go

changes: undefined

修正 persistent.go 末尾的占位代码和 imports：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-03/go/staticanalyzer/cacheanalyzer/persistent.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-03/go/staticanalyzer/cacheanalyzer/persistent.go

changes: undefined

现在创建平台文件锁与目录 fsync：

toolName: Write

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-03/go/staticanalyzer/cacheanalyzer/lock_unix.go

changes: undefined

toolName: Write

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-03/go/staticanalyzer/cacheanalyzer/lock_windows.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-03/go/staticanalyzer/cacheanalyzer/lock_windows.go

changes: undefined

先编译 cacheanalyzer 包检查语法：

toolName: Shell

status: success

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-03/go/staticanalyzer/cacheanalyzer/identity.go

changes: undefined

toolName: Shell

status: success

库编译通过。更新现有测试签名，然后接线 Driver：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-03/go/staticanalyzer/cacheanalyzer/cacheanalyzer_test.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-03/go/staticanalyzer/cacheanalyzer/cacheanalyzer_test.go

changes: undefined

现在接线 Driver。先更新 driver.go 的选项、构造器和指标常量：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-03/go/driver.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-03/go/driver.go

changes: undefined

添加 metrics 常量：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-03/go/driver.go

changes: undefined

在 Run 开头加入 flush 与按运行统计的缓存指标：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-03/go/driver.go

changes: undefined

修正 defer 顺序（flush 必须先于指标快照），并用内置 max：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-03/go/driver.go

changes: undefined

现在修改 runAnalyzers 的缓存查找与提交门控：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-03/go/driver.go

changes: undefined

为了确保 panic 期间绝不提交记录，在提交 defer 中显式拦截并重新抛出 panic（由外层 recover 统一转成 error）：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-03/go/driver.go

changes: undefined

编译整个模块验证：

toolName: Shell

status: success

整体编译通过。现在接线 CLI：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-03/go/cmd/rpcshield/main.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-03/go/cmd/rpcshield/main.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-03/go/cmd/rpcshield/wpamode/wpa.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-03/go/cmd/rpcshield/wpamode/wpa.go

changes: undefined

toolName: Shell

status: success

更新 cacheanalyzer 的 BUILD.bazel：

toolName: Write

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-03/go/staticanalyzer/cacheanalyzer/BUILD.bazel

changes: undefined

现在编写完整的持久缓存测试（跨进程、并发写、失效、损坏、只读等场景）：

toolName: Write

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-03/go/staticanalyzer/cacheanalyzer/persistent_test.go

changes: undefined

修正测试文件中的若干问题（TestMain、helper、失效用例预期、权限错误处理）：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-03/go/staticanalyzer/cacheanalyzer/persistent_test.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-03/go/staticanalyzer/cacheanalyzer/persistent_test.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-03/go/staticanalyzer/cacheanalyzer/persistent_test.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-03/go/staticanalyzer/cacheanalyzer/persistent_test.go

changes: undefined

现在让 flush 在遇到权限类错误时立即进入写降级（只读目录场景），而非保留 pending 重试：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-03/go/staticanalyzer/cacheanalyzer/persistent.go

changes: undefined

toolName: Shell

status: success

运行 cacheanalyzer 测试：

toolName: Shell

status: success

两个测试失败：一个是测试预期问题（空 flush 不建文件），另一个是改动 body 后列号也变了（StableID 不同，不触发剪枝）。修正 fixture 和断言：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-03/go/staticanalyzer/cacheanalyzer/persistent_test.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-03/go/staticanalyzer/cacheanalyzer/persistent_test.go

changes: undefined

toolName: Shell

status: success

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-03/go/staticanalyzer/cacheanalyzer/persistent_test.go

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-03/go/staticanalyzer/cacheanalyzer/persistent_test.go

changes: undefined

toolName: Shell

status: success

全部通过。用 -race 验证并发安全：

toolName: Shell

status: success

cacheanalyzer 包全部通过（含 -race）。现在编写 Driver 级集成测试。先确认 bazel 夹具是否可用：

toolName: todo_write

status: success

Todos updated: 8 items

toolName: Shell

status: success

本地无 bazel 夹具，Driver 集成测试按现有 TestDriver_RunOnPackages 的模式编写（本地编译验证，bazel CI 运行）：

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-03/go/driver_test.go

toolName: Shell

status: success

toolName: Shell

status: success

清理未使用变量并编译驱动测试：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-03/go/driver_test.go

changes: undefined

toolName: Shell

status: success

mock 包是 bazel 生成物。确认这是既有情况，与本次改动无关：

toolName: Shell

status: success

代码审查发现一个改进点：未启用 Cache analyzer 时不应初始化持久层。补上门控：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-03/go/driver.go

changes: undefined

格式化并重新验证：

toolName: Shell

status: success

toolName: Shell

status: success

toolName: Shell

status: success

通读 persistent.go 做最终审查：

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-03/go/staticanalyzer/cacheanalyzer/persistent.go

发现 salvageCorrupt 在失败路径会双重计数 loadFailures，修正为每个终止分支只计一次：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-03/go/staticanalyzer/cacheanalyzer/persistent.go

changes: undefined

toolName: Shell

status: success

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-03/go/staticanalyzer/cacheanalyzer/persistent.go

同步清理进程内已被取代的旧版本记录，保持内存与磁盘一致：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-03/go/staticanalyzer/cacheanalyzer/persistent.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-03/go/staticanalyzer/cacheanalyzer/persistent.go

changes: undefined

再完整检查 identity.go：

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-03/go/staticanalyzer/cacheanalyzer/identity.go

运行多次以排除并发测试偶发问题：

toolName: Shell

status: success

验证 CLI 帮助输出并检查 mock 生成方式（用于本地编译 driver 测试）：

toolName: search_by_regex

status: success

toolName: Shell

status: success

本地生成 bazel mock 仅为编译校验 driver 测试（不提交）。查看各 mock 源：

toolName: Shell

status: success

toolName: Shell

status: success

toolName: Shell

status: success

mock 是 bazel 路径风格，临时配置 go.mod replace 仅做本地编译校验（之后还原）：

toolName: Shell

status: success

驱动测试**编译通过**（类型全部正确），仅因缺少 bazel 生成的 GOPATH 夹具在运行期失败——与既有的 TestDriver_Run 等测试在本地的限制相同。先用 `-run='^$'` 完成完整编译校验，然后还原临时 mock 与 go.mod：

toolName: Shell

status: success

编译校验完成。清理临时 mock 与 go.mod 改动：

toolName: Shell

status: success

验证 CLI flag 实际注册：

toolName: Shell

status: success

最终全面校验：格式化、编译、vet、测试。

toolName: Shell

status: success

检查 wpamode BUILD 如何处理带平台标签的文件（lock_unix/lock_windows 同理）：

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-03/go/cmd/rpcshield/wpamode/BUILD.bazel

BUILD 约定一致（显式 srcs + 构建标签由 rules_go 过滤）。最后再跑一次 -race 确认最新改动：

toolName: Shell

status: success

toolName: todo_write

status: success

Todos updated: 8 items

已完成。为 Cache analyzer 增加了可选的跨进程持久 verdict 缓存，未配置时行为与现状完全一致（不触碰文件系统）。

## 实现要点

**1. 可选持久化（CLI 与库调用）**
- CLI：`rpcshield wpa -persistent-cache-path /path/verdicts.json`（仅在启用 Cache analyzer 时生效），见 [main.go](file:///Users/vance/project/mineProject/swe/09101505/project-03/go/cmd/rpcshield/main.go#L120-L120) 与 [wpa.go](file:///Users/vance/project/mineProject/swe/09101505/project-03/go/cmd/rpcshield/wpamode/wpa.go#L84-L86)
- 库：`DriverOptions.PersistentCachePath`（[driver.go](file:///Users/vance/project/mineProject/swe/09101505/project-03/go/driver.go#L171-L182)），构造逻辑见 [newDriverCacheAnalyzer](file:///Users/vance/project/mineProject/swe/09101505/project-03/go/driver.go#L209-L221)；`NewCacheAnalyzer(opts...)` 保持无参时纯进程内行为。

**2. 跨进程稳定 key（[identity.go](file:///Users/vance/project/mineProject/swe/09101505/project-03/go/staticanalyzer/cacheanalyzer/identity.go)）**
- 身份：包导入路径、`f.String()`（含接收者/funclit 序号）、归一化文件（剥离 GOPATH/bazel 前缀）、行列号、调用原文、调用签名、静态被调者——全部来自源码位置，不依赖 SSA 指针。
- key = SHA256(schema 版本 `v1`、tool 版本、稳定身份、GitSHA+caller 函数字节 hash、analyzer 配置指纹)。源码、调用位置、analyzer 集合/顺序、`NilOtherSourcesAfterSrc`、`KnownErrorConstructorFuncs`、GitSHA 任一变化即 miss（`VerdictConfig.fingerprint` 对构造函数列表排序，顺序无关）。

**3. 只提交完整链结论（[IsReusable](file:///Users/vance/project/mineProject/swe/09101505/project-03/go/staticanalyzer/cacheanalyzer/cacheanalyzer.go#L98-L120) + [runAnalyzers 提交门控](file:///Users/vance/project/mineProject/swe/09101505/project-03/go/driver.go#L1524-L1540)）**：error、panic（内层 recover 后重抛）、nil 输出、`FailUnknownWithError`、Unknown/Cache 来源一律不提交；记录保留 failness 与真正的 OriginalSource（Predefined/SCCP/SSA/AST/LLM），加载/合并时逐条再校验。

**4. 多进程安全（[persistent.go](file:///Users/vance/project/mineProject/swe/09101505/project-03/go/staticanalyzer/cacheanalyzer/persistent.go)）**：flock 协调（崩溃由内核自动释放），锁内重读最新文件→校验→并集合并（同函数+调用点的旧指纹记录被新版本取代，无关有效记录绝不丢失）→同目录临时文件 + fsync + 原子 rename + 目录 fsync；并发读验证无半写文件。

**5. 故障只降级、不改 verdict**：损坏文件在锁内改名 `.corrupt-<时间戳>` 保留证据并重建；未知/更新 schema 或更新 tool 版本则完全不覆写；目录不可写（权限错误确定性放弃、锁超时保留 pending 待 run 末重试）。均通过 zap 日志 + 指标可观察，权限恢复后自动重建。

**6. Metrics（每次 Run 的增量，Driver 复用时按差值统计）**：`analyzer_cache_inproc_hit_count`、`analyzer_cache_persistent_hit_count`、`analyzer_cache_miss_count`、`analyzer_cache_invalid_record_count`、`analyzer_cache_write_degraded_count`、`analyzer_cache_load_failure_count`。

## 验证
- [persistent_test.go](file:///Users/vance/project/mineProject/swe/09101505/project-03/go/staticanalyzer/cacheanalyzer/persistent_test.go)：真实子进程（父写/子读）跨进程命中、独立实例复用、源码/调用位置/GitSHA/analyzer 顺序与选项失效、StableID 剪枝、拒绝记录、损坏抢救、脏记录隔离、新 schema/new tool 保留不覆写、只读目录降级与恢复、8 writer 并发合并无撕裂读、阈值 flush；`go test -count=3 -race` 全部通过。
- [driver_test.go](file:///Users/vance/project/mineProject/swe/09101505/project-03/go/driver_test.go#L1124-L1233)：冷/热两次 Run 对比持久命中且逐边 verdict 与 FailnessMetrics 完全一致、热运行边来源为 `Cache_*`、SHA/配置变化归零命中、损坏抢救与只读目录 verdict 不变（本地用临时生成的 mock 完成编译校验；运行期依赖 bazel 的 `rpcshielddemo-anal` 夹具，与既有 Driver 测试相同）。
- `go build ./...`、`go vet`、gofmt 均通过；BUILD.bazel 已更新。

一处行为微调：cache miss 不再把内部的 "not found in cache" 误当作 analyzer 链 error 传播给边诊断（miss 后显式 `err = nil`）。
