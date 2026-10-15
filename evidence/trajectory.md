# 让服务注册数据按一致代际换新

> **Workspace:** /Users/vance/project/mineProject/swe/09101505/project-02

---

**User**

可靠性工程师为 WPA 刷新服务注册数据时，如果 AllServices 已经写入缓存，而随后的 AllNetworkGroups 获取或 Fx 扫描失败，下一次 auto 运行不应拿新服务层级去解释旧的客户端归属。请把 services、network groups 以及由两者生成的 Fx client mapping 作为一个可识别的 generation：refresh 先在非活动位置取得并校验整组数据，全部成功后才切换活动代；任一获取、扫描、编码、同步或发布错误都继续使用上一份完整 generation。auto 读到缺件、内容损坏或 manifest 不一致的候选时，有旧代就明确记录回退，没有可恢复数据才指出具体组件并失败，文件修复后可再次 refresh。多个进程同时刷新同一 workspace 时，读者只能看到完整旧代或完整新代，不能把不同写入者的 services 与 mapping 配对。显式 services/Fx 缓存路径保持只读语义，旧版 grail_all_services.json 和 fx_mappings.json 可以安全接管；定制 ServiceRegistryProvider 不强制使用默认磁盘布局。结果或运行 metrics 应标明本次 registry generation、各组件来源和回退状态。keep-inbound、生产流量与 tier override 仍按现有独立参数处理，不被纳入或写回服务注册 generation。

**Assistant**

我先了解一下现有代码结构，再制定实施方案。

toolName: view_folder

status: success

/Users/vance/project/mineProject/swe/09101505/project-02

这是一个 Go 代码库，关键逻辑集中在 `finder/servicemapping`、`grail` 和 `cmd/rpcshield`。让我先阅读核心文件。

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-02/go/finder/servicemapping/mapping.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-02/go/grail/grail.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-02/go/cmd/rpcshield/common/cache.go

继续阅读 WPA 模式入口、fx_scanner 以及相关配置。

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-02/go/cmd/rpcshield/wpamode/wpa.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-02/go/finder/servicemapping/fx_scanner.go

现在查看 `serviceregistry` 包，这是服务注册数据的核心提供层。

toolName: file_search

status: success

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-02/go/serviceregistry/serviceregistry.go

现在查看 main.go 中缓存参数如何接线，以及 registry 包是否有测试、其他引用点。

toolName: search_by_regex

status: success

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-02/go/cmd/rpcshield/main.go

查看测试约定、BUILD 文件和 Go 版本，确认可用依赖。

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-02/go/finder/servicemapping/mapping_test.go

toolName: Shell

status: success

查看 BUILD 文件、CI 配置和 wpa 的测试情况，了解构建与测试约定。

toolName: Shell

status: success

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-02/go/cmd/rpcshield/BUILD.bazel

toolName: file_search

status: success

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-02/go/finder/servicemapping/config_resolver.go

toolName: Shell

status: success

环境确认：本地用 `go test`，CI 用 Bazel（需同步更新 BUILD 文件）。检查剩余引用点后制定计划。

toolName: search_by_regex

status: success

查看 fx_scanner_test.go 的夹具构造方式，以便测试中复现成功扫描。

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-02/go/finder/servicemapping/fx_scanner_test.go

代码结构和需求已经梳理清楚。这是一个涉及多文件的可靠性改造，我先建立任务清单。

toolName: todo_write

status: success

Todos updated: 7 items

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-02/go/grail/BUILD.bazel

现在开始实现核心的 generation store。

toolName: Write

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-02/go/serviceregistry/generation.go

changes: undefined

清理 listGenerationIDs 中残留的无用代码，并补上 sha256 辅助函数。

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-02/go/serviceregistry/generation.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-02/go/serviceregistry/generation.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-02/go/serviceregistry/generation.go

changes: undefined

文件行数看起来异常，检查是否有重复内容。

toolName: Shell

status: success

修正几处设计问题：简化 loadFromDir 签名、移除多余的 loadBest 变体、修复空 network groups 的 Present 判定。

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-02/go/serviceregistry/generation.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-02/go/serviceregistry/generation.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-02/go/serviceregistry/generation.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-02/go/serviceregistry/generation.go

changes: undefined

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-02/go/serviceregistry/generation.go

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-02/go/serviceregistry/generation.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-02/go/serviceregistry/generation.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-02/go/serviceregistry/generation.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-02/go/serviceregistry/generation.go

changes: undefined

现在创建跨进程刷新锁（平台拆分）。

toolName: Write

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-02/go/serviceregistry/lock_unix.go

changes: undefined

toolName: Write

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-02/go/serviceregistry/lock_other.go

changes: undefined

现在重写 `serviceregistry.go`：加入 meta 类型，Provide 分流 managed generation 与显式只读路径。

toolName: Write

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-02/go/serviceregistry/serviceregistry.go

changes: undefined

让 generation.go 的 validateBundle 复用独立校验函数，并给 fx_scanner 增加致命错误哨兵。

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-02/go/serviceregistry/generation.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-02/go/finder/servicemapping/fx_scanner.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-02/go/finder/servicemapping/fx_scanner.go

changes: undefined

现在更新 wpa.go：日志与运行 metrics 标注 generation、组件来源和回退状态。

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-02/go/cmd/rpcshield/wpamode/wpa.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-02/go/cmd/rpcshield/wpamode/wpa.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-02/go/cmd/rpcshield/wpamode/wpa.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-02/go/cmd/rpcshield/wpamode/wpa.go

changes: undefined

补上 `registryMetricValues` 辅助函数。

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-02/go/cmd/rpcshield/wpamode/wpa.go

changes: undefined

先编译检查已有改动。

toolName: Shell

status: success

toolName: edit_file_search_replace

status: failed

filePath: /Users/vance/project/mineProject/swe/09101505/project-02/go/lock_unix.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-02/go/serviceregistry/generation.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-02/go/serviceregistry/lock_unix.go

changes: undefined

toolName: Shell

status: success

编译和 vet 通过。更新进度，编写测试。

toolName: todo_write

status: success

Todos updated: 7 items

toolName: Write

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-02/go/serviceregistry/serviceregistry_test.go

changes: undefined

toolName: Shell

status: success

有几个测试失败，需要逐一排查。先单独复现 store 并发发布的问题。

toolName: Shell

status: success

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-02/go/serviceregistry/generation.go

代码逻辑看似正确，先做一个最小复现：串行调用 stageAndPublish 也失败吗？

toolName: Shell

status: success

串行通过，问题只在并发。加一个两 writer 的复现并打印生命周期。

toolName: Shell

status: success

两 writer 通过，差异在 reader。加入并发 reader 复现。

toolName: Shell

status: success

toolName: StopCommand

status: success

toolName: Shell

status: success

toolName: Shell

status: success

找到问题根因了：

1. `loadBest` 的回退扫描与并发 prune 竞争时缺少"重读 active pointer"的最后尝试；
2. auto 分支丢弃了 `loadBest` 返回的 fellBack/reason；
3. refresh 前的基线检查被 legacy 读穿透短路，导致不会先落盘接管；
4. 两个测试自身的断言/清理问题。

先修复 generation.go 的 loadBest 与 refresh 基线逻辑。

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-02/go/serviceregistry/generation.go

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-02/go/serviceregistry/generation.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-02/go/serviceregistry/generation.go

changes: undefined

现在重构 loadBest：增加 store-only 视图和 pointer 重读重试。

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-02/go/serviceregistry/generation.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-02/go/serviceregistry/generation.go

changes: undefined

修复两个并发测试的断言问题。

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-02/go/serviceregistry/serviceregistry_test.go

changes: undefined

简化这段测试逻辑，去掉多余的哨兵。

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-02/go/serviceregistry/serviceregistry_test.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-02/go/serviceregistry/serviceregistry_test.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-02/go/serviceregistry/serviceregistry_test.go

changes: undefined

重新运行整个包的测试。

toolName: Shell

status: success

根因是 reader 的 500 次纯内存循环在 writer 首次落盘前就跑完了。改成基于时间窗口轮询，并确保 writer 被正确回收。

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-02/go/serviceregistry/serviceregistry_test.go

changes: undefined

toolName: Shell

status: success

全部通过。用 race detector 多跑几次，并运行相关包的完整测试。

toolName: Shell

status: success

全部通过（含 race）。现在更新 Bazel BUILD 文件。

toolName: Write

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-02/go/serviceregistry/BUILD.bazel

changes: undefined

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-02/go/cmd/rpcshield/wpamode/BUILD.bazel

检查 fx_scanner 的 BUILD 无需更新后，做全量验证，包括交叉编译（windows/linux 锁文件分支）。

toolName: Shell

status: success

toolName: Shell

status: success

Windows 的编译失败在既有的 `common/utils`（与本次改动无关，该仓库本就不支持 Windows）；Linux 交叉编译通过。运行全量测试。

toolName: Shell

status: success

唯一失败的 `ssaanal` 看起来与本次改动无关（它向上查找名为 "rpcshield" 的目录，而当前工作目录是 `project-02`）。确认一下：

toolName: Shell

status: success

`ssaanal` 失败是环境性问题（测试硬编码查找名为 "rpcshield" 的 GOPATH 目录），与改动无关。最后做格式检查与 lint 配置确认。

toolName: Shell

status: success

toolName: Shell

status: success

复查并简化 toRegistry 中冗余的 else 分支。

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-02/go/serviceregistry/generation.go

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-02/go/serviceregistry/generation.go

changes: undefined

补一个 wpa metrics 的小测试（metrics 标注是明确需求），并更新其 BUILD 文件。

toolName: Write

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-02/go/cmd/rpcshield/wpamode/wpa_test.go

changes: undefined

toolName: Write

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-02/go/cmd/rpcshield/wpamode/BUILD.bazel

changes: undefined

toolName: Shell

status: success

全部通过。做一处 errcheck 防御性修正（syncDir 里的 deferred Close）。

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-02/go/serviceregistry/generation.go

changes: undefined

toolName: todo_write

status: success

Todos updated: 7 items

toolName: Shell

status: success

最终全量构建验证。

toolName: Shell

status: success

toolName: todo_write

status: success

Todos updated: 7 items

改造完成。以下是方案与实现说明。

## 核心设计：registry generation 原子协议

把 `services`、`network groups`、由二者生成的 `Fx client mapping` 封装为一个不可分割的 **generation**，落盘布局：

```
<workspace>/build/rpcshield/
  grail_all_services.json        # 旧版单文件缓存（只读接管源，永不修改/删除）
  fx_mappings.json
  registry/
    refresh.lock                 # 跨进程 flock，刷新串行化
    active.json                  # 活动代指针（rename 原子翻转）
    generations/gen-<时间戳>-<随机>/
      manifest.json              # 各组件 size/sha256/来源
      services.json
      network_groups.json
      fx_mappings.json
```

**刷新流程**（[generation.go](file:///Users/vance/project/mineProject/swe/09101505/project-02/go/serviceregistry/generation.go)）：加锁 → 无存量代时先把旧版缓存导入为基线代 → 在唯一 `.stage-*` 暂存目录编码全部组件（先入内存 buffer，编码失败不触盘）→ fsync + 目录 fsync → 读回校验（sha256、大小、JSON 严格解码、跨组件一致性：mapping 引用的 service 必须在同一 services 内）→ 目录就位 → **一次原子 rename 翻转 `active.json`**。任何获取/扫描/编码/同步/发布错误都不动指针，下一次 auto 必然读到完整旧代。

**读取回退**：活动代缺件/损坏/manifest 摘要不符 → 依次尝试次新代 → 重读指针（捕获并发刷新刚发布的新代，防止与 prune 竞争）→ 旧版文件读穿透。命中非活动候选时 `Meta.FellBack=true` 并给出具体原因；无任何可恢复数据时错误信息点名具体组件（如 `component "fx_mappings"`）及各候选路径；修复后再 refresh 即自愈。

## 需求逐项对应

- **刷新中途失败不串代**：AllServices 成功但 AllNetworkGroups/Fx 扫描失败时活动指针不变，auto 继续用旧代，并有 `TestProvideManaged_RefreshFailureAfterServicesKeepsPreviousGeneration` 复现验证。
- **扫描错误分级**：[fx_scanner.go](file:///Users/vance/project/mineProject/swe/09101505/project-02/go/finder/servicemapping/fx_scanner.go#L26-L32) 新增哨兵 `ErrFxScanFailed`（workspace walk 失败=致命，中止刷新）；包加载/构造函数无法解析等历来容忍的诊断只计数入 manifest，mapping 照常生效。
- **多进程隔离**：唯一暂存目录 + flock + 指针原子 rename + 加载时哈希配对校验，读者只能看到完整旧代或完整新代，不可能跨写入者配对（并发测试含 6 writer/8 reader 及 2 writer×500 次轮询配对断言，`-race` 通过）。
- **显式路径只读**：任一 `-use-cached-*-json` 传文件路径即走只读外部装配，绝不写入该文件（混合模式下另一组件沿用旧单文件缓存），来源/摘要如实记录；定制 `ServiceRegistryProvider` 完全不接触磁盘布局。
- **metrics**：wpa 日志与运行指标新增 `registry_generation`、`registry_services_source`、`registry_network_groups_source`、`registry_fx_mappings_source`、`registry_fallback`、`registry_fallback_reason`（[wpa.go](file:///Users/vance/project/mineProject/swe/09101505/project-02/go/cmd/rpcshield/wpamode/wpa.go#L528-L554)）。
- **边界保持**：keep-inbound、生产流量记录、tier override 仍走各自独立参数，不进入也不写回 generation（tier override 在 Provide 之后施加于内存切片）。
- **一个有意的行为收紧**：只要两选项都在 auto/refresh 管理模式，任一为 `refresh` 都会整体重建三件套（避免用新 groups 配旧 services）；旧版缓存在首次刷新前先导入为基线，首次刷新失败也能回退。

## 变更文件

- 新增：[generation.go](file:///Users/vance/project/mineProject/swe/09101505/project-02/go/serviceregistry/generation.go)、[lock_unix.go](file:///Users/vance/project/mineProject/swe/09101505/project-02/go/serviceregistry/lock_unix.go)/[lock_other.go](file:///Users/vance/project/mineProject/swe/09101505/project-02/go/serviceregistry/lock_other.go)、[serviceregistry_test.go](file:///Users/vance/project/mineProject/swe/09101505/project-02/go/serviceregistry/serviceregistry_test.go)（15 个场景）、[wpa_test.go](file:///Users/vance/project/mineProject/swe/09101505/project-02/go/cmd/rpcshield/wpamode/wpa_test.go)
- 修改：[serviceregistry.go](file:///Users/vance/project/mineProject/swe/09101505/project-02/go/serviceregistry/serviceregistry.go)（`RegistryMeta`/`ComponentProvenance`、Provide 分流）、[fx_scanner.go](file:///Users/vance/project/mineProject/swe/09101505/project-02/go/finder/servicemapping/fx_scanner.go)、[wpa.go](file:///Users/vance/project/mineProject/swe/09101505/project-02/go/cmd/rpcshield/wpamode/wpa.go) 及两个 BUILD.bazel

验证：`go build ./...`、`go vet`、受影响包全量测试及 `-race` 多轮均通过；唯一失败的 `staticanalyzer/ssaanal` 是既有环境问题（测试硬编码查找名为 rpcshield 的 GOPATH 目录），与本次改动无关。
