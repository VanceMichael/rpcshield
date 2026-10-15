# 让中断的 WPA 分析收敛后再退出

> **Workspace:** /Users/vance/project/mineProject/swe/09101505/project-01

---

**User**

平台工程师在耗时的 wpa 扫描中按 Ctrl-C，或库调用方取消传给 Driver.Run 的 context 后，已启动的分析不应继续消耗 CPU、等待模型请求，也不能拿已完成的一部分结果覆盖上一轮可信报告。请让 SIGINT/SIGTERM 与调用方取消贯穿目标装载、RTA、并行 call tree、路径逐 hop analyzer 和 LLM 请求；停止接收新目标和工作单元，唤醒并等待所有已启动 worker 收敛后，以可识别的取消错误返回。多 target 只完成前缀时不得把它计作整项成功，取消过程中生成的 result.json、CSV、HTML 临时内容不能替换既有完整产物；本次创建的 lifted GOPATH 要在使用者退出后清理，KeepGoPathAfterRun 仍保留目录。取消发生在 graph worker 阻塞和 LLM 请求进行中都应能确定复现并恢复；未取消运行的 verdict、metrics、报告格式和并行度选项保持兼容，非取消错误仍沿用现有错误结果诊断。

**Assistant**

我先回顾相关经验，然后探查代码库结构。

toolName: view_folder

status: success

/Users/vance/project/mineProject/swe/09101505/project-01

这是一个复杂的多模块任务，我先建立任务清单，然后深入阅读核心代码。

toolName: todo_write

status: success

Todos updated: 7 items

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/driver.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/cmd/rpcshield/main.go

继续阅读 wpa mode、graph builder、genai 客户端等关键文件。

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/cmd/rpcshield/wpamode/wpa.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/graph/graph.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/graph/rtaeval.go

toolName: view_folder

status: success

/Users/vance/project/mineProject/swe/09101505/project-01/go/rta

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/genai/genai.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/driver.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/rta/lib.go

关键发现：`Driver.Run` 接收 ctx 但完全未使用，LLM 调用用 `context.Background()`。继续看 RTA worker 池与并行 call tree。

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/rta/utils/workerpool.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/rta/utils/pmap.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/rta/prta_kumo_nonblocking/impl.go

现在查看并行 call tree、bazel GOPATH lift、reporting 写入逻辑。

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/graph/cct.go

继续探查 bazel lift、reporting 写入、Runner 执行命令的方式。

toolName: view_folder

status: success

/Users/vance/project/mineProject/swe/09101505/project-01/go/bazel

toolName: search_by_regex

status: success

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/reporting/csv.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/reporting/html.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/bazel/bazel.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/common/utils/utils.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/bazel/inferfaces.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/bazel/rename.go

toolName: search_by_regex

status: success

查看测试约定、loader、jsonmode 入口与 go 版本。

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/driver_test.go

toolName: search_by_regex

status: success

toolName: search_by_regex

status: success

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/cmd/rpcshield/jsonmode/jsonmode.go

toolName: search_by_regex

status: success

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/driver_test.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/rtabench.go

toolName: search_by_regex

status: success

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/driver_test.go

toolName: search_by_regex

status: success

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/rta/srta/impl.go

架构已摸清。查看 graph 测试与 rtatest 的夹具加载方式，然后开始实现。

toolName: search_by_regex

status: success

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/rta/rtatest/helper.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/graph/graph_test.go

方案已经清晰。更新任务清单，开始实现。

toolName: todo_write

status: success

Todos updated: 10 items

## 1. RTA 层：context 选项

toolName: edit_file_search_replace

status: failed

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/rta/lib.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/rta/lib.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/rta/lib.go

changes: undefined

## 2. WorkerPool 取消能力

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/rta/utils/workerpool.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/rta/utils/workerpool.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/rta/utils/workerpool.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/rta/utils/workerpool.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/rta/utils/workerpool.go

changes: undefined

## 3. 四个并行 RTA flavor 接入取消

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/rta/prta_kumo_nonblocking/impl.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/rta/prta_kumo_nonblocking/impl.go

changes: undefined

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/rta/prta_kumo_nonblocking/impl.go

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/rta/prta_kumo_nonblocking/impl.go

changes: undefined

现在同样改造其余三个并行 flavor，先看它们的 Analyze 结构。

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/rta/prta_kumo/impl.go

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/rta/prta_kumo/impl.go

changes: undefined

prta_kumo 的 processNodesInBatches 在 Wait 后复用池，需要看它如何使用。

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/rta/prta_kumo/impl.go

toolName: search_by_regex

status: success

prta_kumo 的 dedup 阶段复用同一池且依赖信号量，需要特殊处理。重写该段：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/rta/prta_kumo/impl.go

changes: undefined

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/rta/prta_kumo/impl.go

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/rta/prta_kumo/impl.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/rta/prta_kumo/impl.go

changes: undefined

toolName: edit_file_search_replace

status: failed

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/rta/prta_kumo/impl.go

changes: undefined

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/rta/prta_kumo/impl.go

逐段小编辑：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/rta/prta_kumo/impl.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/rta/prta_kumo/impl.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/rta/prta_kumo/impl.go

changes: undefined

现在改造 prta_naive 和 prta_nonblocking：

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/rta/prta_naive/impl.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/rta/prta_nonblocking/impl.go

toolName: search_by_regex

status: success

toolName: search_by_regex

status: success

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/rta/prta_naive/impl.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/rta/prta_nonblocking/impl.go

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/rta/prta_naive/impl.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/rta/prta_nonblocking/impl.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/rta/prta_naive/impl.go

changes: undefined

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/rta/prta_naive/impl.go

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/rta/prta_naive/impl.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/rta/prta_nonblocking/impl.go

changes: undefined

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/rta/prta_nonblocking/impl.go

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/rta/prta_nonblocking/impl.go

changes: undefined

## 4. 四个串行 flavor 加入 worklist 取消检查

toolName: search_by_regex

status: success

toolName: search_by_regex

status: success

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/rta/srta_kumo/impl.go

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/rta/srta_kumo/impl.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/rta/srta/impl.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/rta/srta/impl.go

changes: undefined

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/rta/srta_opt/impl.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/rta/srta_struct/impl.go

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/rta/srta_opt/impl.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/rta/srta_struct/impl.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/rta/srta_opt/impl.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/rta/srta_struct/impl.go

changes: undefined

RTA 层完成，先编译验证：

toolName: Shell

status: success

RTA 层编译通过。更新进度，开始 graph 层。

toolName: todo_write

status: success

Todos updated: 10 items

## 5. graph 层取消传播

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/graph/graph.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/graph/graph.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/graph/graph.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/graph/graph.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/graph/graph.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/graph/graph.go

changes: undefined

现在改 rtaeval.go 的 runSingleFlavor 与 sweep：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/graph/rtaeval.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/graph/rtaeval.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/graph/rtaeval.go

changes: undefined

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/graph/rtaeval.go

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/graph/rtaeval.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/graph/rtaeval.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/graph/rtaeval.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/graph/rtaeval.go

changes: undefined

## 6. 并行 call tree（cct.go）取消传播

toolName: search_by_regex

status: success

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/graph/cct.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/graph/cct.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/graph/cct.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/graph/cct.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/graph/cct.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/graph/cct.go

changes: undefined

toolName: Shell

status: success

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/graph/cct.go

changes: undefined

toolName: Shell

status: success

graph 层完成。接下来目标装载：命令执行 ctx 化。

toolName: todo_write

status: success

Todos updated: 10 items

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/common/utils/utils.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/common/utils/utils.go

changes: undefined

## 8. bazel LiftGoPath ctx 化（可选接口，保持 mock 兼容）

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/bazel/inferfaces.go

changes: undefined

toolName: search_by_regex

status: success

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/bazel/bazel.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/bazel/bazel.go

changes: undefined

toolName: edit_file_search_replace

status: failed

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/bazel/bazel.go

changes: undefined

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/bazel/bazel.go

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/bazel/bazel.go

changes: undefined

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/bazel/serverlessfix.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/bazel/importcomment.go

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/bazel/serverlessfix.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/bazel/importcomment.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/bazel/importcomment.go

changes: undefined

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/bazel/cgo.go

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/bazel/cgo.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/bazel/cgo.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/bazel/cgo.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/bazel/cgo.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/bazel/cgo.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/bazel/cgo.go

changes: undefined

toolName: Shell

status: success

目标装载层编译通过。接下来 reporting 原子写。

toolName: todo_write

status: success

Todos updated: 9 items

## 9. reporting 原子写入（temp + rename）

toolName: edit_file_search_replace

status: failed

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/reporting/csv.go

changes: undefined

分小块编辑 csv.go：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/reporting/csv.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/reporting/csv.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/reporting/html.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/reporting/html.go

changes: undefined

## 10. driver 核心改造

先加可识别的取消错误：

toolName: edit_file_search_replace

status: failed

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/driver.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/driver.go

changes: undefined

toolName: edit_file_search_replace

status: failed

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/driver.go

changes: undefined

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/driver.go

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/driver.go

changes: undefined

现在改造目标循环。查看当前行号：

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/driver.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/driver.go

逐段改造 Run：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/driver.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/driver.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/driver.go

changes: undefined

新增 driver 的 lift/cleanup 辅助方法：

toolName: edit_file_search_replace

status: failed

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/driver.go

changes: undefined

toolName: search_by_regex

status: success

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/driver.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/driver.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/driver.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/driver.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/driver.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/driver.go

changes: undefined

## 11. 逐 hop analyzer 与 LLM 请求 ctx 化

toolName: edit_file_search_replace

status: failed

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/driver.go

changes: undefined

toolName: search_by_regex

status: success

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/driver.go

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/driver.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/driver.go

changes: undefined

Now analyzeEdge：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/driver.go

changes: undefined

toolName: edit_file_search_replace

status: failed

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/driver.go

changes: undefined

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/driver.go

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/driver.go

changes: undefined

toolName: edit_file_search_replace

status: failed

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/driver.go

changes: undefined

toolName: search_by_regex

status: success

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/driver.go

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/driver.go

changes: undefined

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/driver.go

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/driver.go

changes: undefined

更新 rtabench.go：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/rtabench.go

changes: undefined

toolName: Shell

status: success

driver 编译通过。现在信号与 CLI 层。

toolName: todo_write

status: success

Todos updated: 9 items

## 12. 信号处理与模式入口 ctx

toolName: Write

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/cmd/rpcshield/signals.go

changes: undefined

toolName: Write

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/cmd/rpcshield/signals.go

changes: undefined

现在更新 main.go：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/cmd/rpcshield/main.go

changes: undefined

修正 import 块：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/cmd/rpcshield/main.go

changes: undefined

toolName: edit_file_search_replace

status: failed

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/cmd/rpcshield/main.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/cmd/rpcshield/main.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/cmd/rpcshield/main.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/cmd/rpcshield/main.go

changes: undefined

Now wpamode.Run signature:

toolName: edit_file_search_replace

status: failed

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/cmd/rpcshield/wpamode/wpa.go

changes: undefined

toolName: edit_file_search_replace

status: failed

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/cmd/rpcshield/wpamode/wpa.go

changes: undefined

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/cmd/rpcshield/wpamode/wpa.go

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/cmd/rpcshield/wpamode/wpa.go

changes: undefined

toolName: edit_file_search_replace

status: failed

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/cmd/rpcshield/wpamode/wpa.go

changes: undefined

toolName: search_by_regex

status: success

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/cmd/rpcshield/wpamode/wpa.go

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/cmd/rpcshield/wpamode/wpa.go

changes: undefined

jsonmode.Run 接收 ctx：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/cmd/rpcshield/jsonmode/jsonmode.go

changes: undefined

toolName: Shell

status: success

全量编译通过。更新既有测试调用点：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/driver_test.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/driver_test.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/driver_test.go

changes: undefined

toolName: search_by_regex

status: success

rtaeval_test.go 需适配新签名：

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/graph/rtaeval_test.go

toolName: Shell

status: success

toolName: search_by_regex

status: success

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/graph/rtaeval_test.go

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/graph/rtaeval_test.go

changes: undefined

toolName: Shell

status: success

toolName: Shell

status: success

toolName: Shell

status: success

toolName: Shell

status: success

发现一个收敛性缺陷：worker 直接退出会让已入队工作单元滞留导致 Wait 永久阻塞。修正设计——worker 不直接退出，靠 Cancel 广播唤醒排空队列，最终由 Stop 收尾：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/rta/utils/workerpool.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/rta/utils/workerpool.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/rta/utils/workerpool.go

changes: undefined

toolName: edit_file_search_replace

status: failed

filePath: /Users/vance/project/mineProject/swe/091505/project-01/go/rta/utils/workerpool.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/rta/utils/workerpool.go

changes: undefined

现在编写取消复现测试。先是 WorkerPool 层（确定性复现 worker 阻塞/在途收敛/拒绝新单元）：

toolName: Write

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/rta/utils/workerpool_cancel_test.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/rta/utils/workerpool_cancel_test.go

changes: undefined

toolName: Shell

status: success

toolName: Shell

status: success

### RTA flavor 层取消测试

toolName: search_by_regex

status: success

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/rta/prta_kumo_nonblocking/impl_test.go

toolName: Write

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/rta/prta_kumo_nonblocking/impl_cancel_test.go

changes: undefined

toolName: Write

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/rta/srta/impl_cancel_test.go

changes: undefined

toolName: Shell

status: success

toolName: Shell

status: success

toolName: Shell

status: success

夹具依赖 Bazel 产物，本地 plain-go 跑不了既有 rta 测试。改用内存合成 SSA 程序，让取消测试无外部夹具依赖：

toolName: Write

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/rta/prta_kumo_nonblocking/impl_cancel_test.go

changes: undefined

toolName: Write

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/rta/srta/impl_cancel_test.go

changes: undefined

toolName: Shell

status: success

toolName: Shell

status: success

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/rta/srta/impl_cancel_test.go

changes: undefined

toolName: edit_file_search_replace

status: failed

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/rta/prta_kumo_nonblocking/impl_cancel_test.go

changes: undefined

toolName: Shell

status: success

toolName: Shell

status: success

合成程序的跨函数调用需要 syntax 关联，改为空函数（取消测试不依赖调用内容）：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/rta/prta_kumo_nonblocking/impl_cancel_test.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/rta/srta/impl_cancel_test.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/rta/prta_kumo_nonblocking/impl_cancel_test.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/rta/srta/impl_cancel_test.go

changes: undefined

toolName: Shell

status: success

toolName: Shell

status: success

graph 测试同样依赖 Bazel 夹具，按既有约定写（CI/Bazel 下运行）：

toolName: Write

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/graph/cancellation_test.go

changes: undefined

toolName: Shell

status: success

driver 包测试依赖 Bazel 生成的 mock。尝试本地用 mockgen 临时生成以运行验证（随后删除生成物）：

toolName: Shell

status: success

toolName: Shell

status: success

toolName: Shell

status: success

toolName: Shell

status: success

toolName: Shell

status: success

toolName: Shell

status: success

toolName: Shell

status: success

需要临时嵌套 module 让 `mock/...` 导入路径可解析（验证后删除）：

toolName: Shell

status: success

toolName: Shell

status: success

toolName: Shell

status: success

toolName: Shell

status: success

toolName: Shell

status: success

toolName: Shell

status: success

本地测试通道就绪。现在编写无夹具依赖的 driver 取消测试。先把 detached rm 抽成可测函数：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/driver.go

changes: undefined

toolName: todo_write

status: success

Todos updated: 9 items

toolName: Write

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/cancel_test.go

changes: undefined

重写整个测试文件（修正 PackageSource 类型与全部用例）：

toolName: Write

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/cancel_test.go

changes: undefined

toolName: Write

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/cancel_test.go

changes: undefined

toolName: Shell

status: success

toolName: search_by_regex

status: success

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/cancel_test.go

changes: undefined

toolName: Shell

status: success

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/cancel_test.go

toolName: Shell

status: success

toolName: Shell

status: success

问题定位：阶段错误被通用包装，缺少 ErrCanceled。修正各加载分支的错误归类：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/driver.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/driver.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/driver.go

changes: undefined

toolName: Shell

status: success

toolName: Shell

status: success

信号测试：

toolName: Write

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/cmd/rpcshield/signals_test.go

changes: undefined

toolName: Shell

status: success

`stop()` 需要幂等（测试调用两次，实际 defer 也可能重复）：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/cmd/rpcshield/signals.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/cmd/rpcshield/signals.go

changes: undefined

toolName: Shell

status: success

补充 reporting 原子写的兼容性测试：

toolName: search_by_regex

status: success

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/reporting/html_test.go

toolName: Write

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/reporting/atomic_write_test.go

changes: undefined

toolName: Write

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/reporting/atomic_write_test.go

changes: undefined

toolName: Shell

status: success

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/reporting/atomic_write_test.go

toolName: Shell

status: success

toolName: Shell

status: success

toolName: Shell

status: success

少了一个闭合括号（WriteFile 与 NoError 各需一个）：

toolName: Shell

status: success

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/reporting/atomic_write_test.go

toolName: Shell

status: success

更新既有测试对错误消息的断言：

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/reporting/html_test.go

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/reporting/html_test.go

changes: undefined

toolName: Shell

status: success

全部测试跑一遍，区分夹具缺失（既有）与真实回归：

toolName: todo_write

status: success

Todos updated: 2 items

toolName: Shell

status: success

补生成 output mock，然后汇总失败包分类：

toolName: Shell

status: success

toolName: Shell

status: success

toolName: Shell

status: success

toolName: Shell

status: success

toolName: Shell

status: success

确认所有既有失败测试均依赖同一 Bazel 夹具：

toolName: Shell

status: success

toolName: Shell

status: success

toolName: Shell

status: success

用 git worktree 建立 HEAD 基线环境，对比失败集合以确认无回归：

toolName: Shell

status: success

toolName: Shell

status: success

toolName: Shell

status: success

toolName: Shell

status: success

toolName: Shell

status: success

toolName: Shell

status: success

toolName: Shell

status: success

toolName: Shell

status: success

toolName: Shell

status: success

差异全部为既有 Bazel 夹具限制。跑可运行包的完整测试与 race 验证：

toolName: Shell

status: success

toolName: Shell

status: success

toolName: Shell

status: success

toolName: Shell

status: success

toolName: Shell

status: success

清理前先检查 Bazel BUILD 是否需要登记新测试文件：

toolName: Shell

status: success

BUILD 用显式列表，需要登记新文件。逐一查看：

toolName: Shell

status: success

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/rta/utils/BUILD.bazel

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/rta/srta/BUILD.bazel

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/rta/prta_kumo_nonblocking/BUILD.bazel

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/reporting/BUILD.bazel

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/BUILD.bazel

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/cmd/rpcshield/BUILD.bazel

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/graph/BUILD.bazel

changes: undefined

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/cmd/rpcshield/BUILD.bazel

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/cmd/rpcshield/BUILD.bazel

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/cmd/rpcshield/BUILD.bazel

changes: undefined

toolName: Shell

status: success

toolName: Shell

status: success

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/rta/srta/BUILD.bazel

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/rta/prta_kumo_nonblocking/BUILD.bazel

changes: undefined

清理本地临时构建产物（go.work、生成的 mock、worktree）：

toolName: Shell

status: success

toolName: Shell

status: success

toolName: Shell

status: success

toolName: Shell

status: success

最终全量构建与无夹具依赖测试复跑：

toolName: Shell

status: success

补充两处：genai 边界检查与 wpamode 取消后跳过 LDA 外部上报副作用：

toolName: edit_file_search_replace

status: failed

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/genai/genai.go

changes: undefined

toolName: edit_file_search_replace

status: failed

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/genai/genai.go

changes: undefined

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/genai/genai.go

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/genai/genai.go

changes: undefined

toolName: edit_file_search_replace

status: failed

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/cmd/rpcshield/wpamode/wpa.go

changes: undefined

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/cmd/rpcshield/wpamode/wpa.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/cmd/rpcshield/wpamode/wpa.go

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/cmd/rpcshield/wpamode/wpa.go

changes: undefined

toolName: Shell

status: success

toolName: Shell

status: success

toolName: Shell

status: success

toolName: Shell

status: success

toolName: Shell

status: success

toolName: Shell

status: success

toolName: Shell

status: success

最后通读关键 diff 做一致性检查：

toolName: Shell

status: success

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/09101505/project-01/go/driver.go

toolName: todo_write

status: success

Todos updated: 2 items

toolName: Shell

status: success

全部改造与验证已完成。总结如下：

## 取消传播链路（SIGINT/SIGTERM 与调用方 context 贯通）

- 信号层：新增 [signals.go](file:///Users/vance/project/mineProject/swe/09101505/project-01/go/cmd/rpcshield/signals.go)，首个 SIGINT/SIGTERM 取消 context，第二个信号恢复默认行为并强杀；[main.go](file:///Users/vance/project/mineProject/swe/09101505/project-01/go/cmd/rpcshield/main.go) 将 ctx 传入 wpa/json 模式；`wpamode.Run`/`jsonmode.Run` 改为接收 ctx。
- 目标装载：新增 `utils.ExecCommandContext`（[utils.go](file:///Users/vance/project/mineProject/swe/09101505/project-01/go/common/utils/utils.go)）和可选接口 `bazel.ContextGoPathWrapper`（[inferfaces.go](file:///Users/vance/project/mineProject/swe/09101505/project-01/go/bazel/inferfaces.go)，不破坏 gomock），`LiftGoPathContext` 将 ctx 贯穿 buildozer / `bazel build` / cquery / cgo / chmod / cp（[bazel.go](file:///Users/vance/project/mineProject/swe/09101505/project-01/go/bazel/bazel.go)、cgo.go、serverlessfix.go、importcomment.go），取消即杀死在途 bazel 子进程。
- RTA：`AnalyzeConfig` 新增 `WithContext`（[lib.go](file:///Users/vance/project/mineProject/swe/09101505/project-01/go/rta/lib.go)）；8 个 flavor 全部接入（4 个并行 flavor 的 worker pool 拒绝新工作单元，4 个串行 flavor 的 worklist 边界退出；dedup 批次停止提交并收敛）。
- WorkerPool：[workerpool.go](file:///Users/vance/project/mineProject/swe/09101505/project-01/go/rta/utils/workerpool.go) 新增 `Cancel()`——Push 取消后变 no-op、广播唤醒所有阻塞在 sleep select 的 worker、已入队单元被排空、Wait 必然归零，Stop 再让 worker 退出。
- 并行 call tree：[cct.go](file:///Users/vance/project/mineProject/swe/09101505/project-01/go/graph/cct.go) 不再启动新 source worker，节点递归/边遍历/SCCP sink 可行性检查全部响应 ctx，WaitGroup 收敛后返回包装 `context.Canceled` 的错误。
- 逐 hop analyzer 与 LLM：[driver.go](file:///Users/vance/project/mineProject/swe/09101505/project-01/go/driver.go) 的 `analyzeCallPaths`/`analyzeEdge`/`runAnalyzers`/`runLLMAnalyzer` 全部接收 ctx；LLM 不再用 `context.Background()`，在途 HTTP 请求随取消中止（[genai.go](file:///Users/vance/project/mineProject/swe/09101505/project-01/go/genai/genai.go)）；取消错误向上传播而非降级为 FailUnknownWithError 边诊断。

## 可识别错误与“前缀不算成功”

- 新增 `rpcshield.ErrCanceled`，双重包装 `fmt.Errorf("%w: %w", ErrCanceled, context.Canceled)`，`errors.Is` 两者均可命中；非取消错误沿用既有诊断。
- 目标循环每轮开头检查 ctx，取消后不再接收新 target；顶层 defer 兜底：即便某阶段在取消后返回 nil，也不会出现成功 verdict（status 强制为 `error`）。

## 产物保护（不覆盖可信报告）

- 取消时 result.json **完全不写**；正常路径与 [csv.go](file:///Users/vance/project/mineProject/swe/09101505/project-01/go/reporting/csv.go)、[html.go](file:///Users/vance/project/mineProject/swe/09101505/project-01/go/reporting/html.go) 均改为同目录 temp + `os.Rename` 原子提交，失败/中断绝不截断既有完整文件，且无临时文件残留。
- 取消时跳过 CSV/HTML 写入（既有 `err != nil` 守卫）与 LDA 外部上报副作用。

## GOPATH 清理

- 清理在 lift 返回时即注册（成功/失败/取消均覆盖，取消时返回部分树路径），保留原有两条策略：`testing` 不动、`KeepGoPathAfterRun` 保留；其余情况 detached `rm -rf`（[driver.go scheduleGoPathCleanup](file:///Users/vance/project/mineProject/swe/09101505/project-01/go/driver.go#L1809-L1831)）。

## 可确定性复现的测试

- Worker 阻塞 + 在途 + 排队单元的唤醒/收敛/拒收：[workerpool_cancel_test.go](file:///Users/vance/project/mineProject/swe/09101505/project-01/go/rta/utils/workerpool_cancel_test.go)（含全员睡眠场景）。
- RTA flavor 预取消返回 nil（合成 SSA 程序，无外部夹具）：srta 与 prta_kumo_nonblocking 的 `impl_cancel_test.go`。
- LLM 请求在途取消、目标装载在途取消、预取消不接收任何 target、可信 result.json/CSV/HTML 哨兵文件保持字节不变、KeepGoPathAfterRun 保留、detached rm 实际回收：[cancel_test.go](file:///Users/vance/project/mineProject/swe/09101505/project-01/go/cancel_test.go)。
- 信号：自送 SIGINT 验证 ctx 取消（[signals_test.go](file:///Users/vance/project/mineProject/swe/09101505/project-01/go/cmd/rpcshield/signals_test.go)）。
- 原子替换：[atomic_write_test.go](file:///Users/vance/project/mineProject/swe/09101505/project-01/go/reporting/atomic_write_test.go)；graph 取消接线：[cancellation_test.go](file:///Users/vance/project/mineProject/swe/09101505/project-01/go/graph/cancellation_test.go)。
- 所有新增测试在 `-race -count=3` 下稳定通过；新测试文件与新源文件均已登记到对应 BUILD.bazel。

## 兼容性

未取消时 verdict、metrics、报告格式、并行度选项、非取消错误诊断行为保持不变；`GoPathWrapper`/Builder/Flavor 接口零破坏（context 能力走可选接口与配置项，nil context 默认 `Background`）。

验证说明：本机无 Bazel，依赖 `rpcshielddemo-anal` go_path 夹具与 runfiles 的既有测试（TestCorrectness、TestE2E 等）在 plain `go test` 下本来就无法运行；我通过 HEAD worktree 基线对比确认分支未引入新的失败类别（新增夹具型 graph 测试在 Bazel CI 下会随夹具运行）。全量 `go build ./...` 通过。
