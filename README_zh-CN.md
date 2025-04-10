# CronJob 包

[简体中文](README_zh-CN.md) | [English](README.md)

[![Go Reference](https://pkg.go.dev/badge/github.com/hy-shine/cronjob-go.svg)](https://pkg.go.dev/github.com/hy-shine/cronjob-go) [![Go Report Card](https://goreportcard.com/badge/github.com/hy-shine/cronjob-go)](https://goreportcard.com/report/github.com/hy-shine/cronjob-go) [![LICENSE](https://img.shields.io/github/license/hy-shine/cronjob-go)](https://github.com/hy-shine/cronjob-go/blob/master/LICENSE)

一个围绕 [robfig/cron/v3](https://github.com/robfig/cron) 库的线程安全包装器，用于通过唯一标识符管理计划任务。

## 特性

*   **任务管理:** 使用唯一的字符串 ID 添加、更新、移除和查询任务。
*   **线程安全:** 所有操作都通过 `sync.RWMutex` 保证并发安全。
*   **优雅生命周期:** 优雅地启动和停止调度器。`Stop()` 会等待正在运行的任务完成。
*   **灵活调度:** 支持标准的 cron 规范和可选的秒级精度。
*   **时区支持:** 配置任务在特定时区运行。
*   **全面的错误处理:** 为常见问题预定义了错误（例如 `ErrJobNotFound`, `ErrJobIdAlreadyExists`）。
*   **可配置日志:** 使用 `cronlib.Logger` 与您现有的日志基础设施集成。
*   **灵活重试机制:** 内建支持重试失败的任务，具有可配置的策略（固定等待时间或指数增长）。
*   **跳过并发运行:** 可选配置，防止任务在其前一个实例仍在运行时启动。
*   **批量操作:** 原子性地添加多个任务 (`AddBatch`)。
*   **任务查询:** 获取任务详情 (`Get`) 并列出所有任务 ID (`Jobs`, `Len`)。
*   **清空所有任务:** 移除所有已调度的任务 (`Clear`)。

## 安装

```bash
go get github.com/hy-shine/cronjob-go
```

## 快速开始

```go
package main

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/hy-shine/cronjob-go"
)

func main() {
	cron, err := cronjob.New(
		cronjob.WithCronSeconds(),
		cronjob.WithRetry(3, 1*time.Second), // 失败的任务重试 3 次，延迟 1 秒
	)
	if err != nil {
		logger.Error("创建 cron 调度器失败", "error", err)
		os.Exit(1)
	}

	err = cron.Add("job1", "*/5 * * * * *", func() error {
		logger.Info("运行 job1")
		return nil
	})
	if err != nil {
		logger.Error("添加 job1 失败", "error", err)
	}

	err = cron.Add("job2", "@every 10s", func() error {
		logger.Info("运行 job2")
		return nil
	})
	if err != nil {
		logger.Error("添加 job2 失败", "error", err)
	}

	cron.Start()
	logger.Info("Cron 调度器已启动")

	defer cron.Stop()

	select {}
}
```

## API 参考

### `CronJober` 接口

```go
type CronJober interface {
	// Add 调度一个新任务。如果 ID 已被占用，则返回 ErrJobIdAlreadyExists。
	Add(jobId, spec string, f func() error) error

	// AddBatch 原子性地调度多个任务。如果出现任何错误则回滚。
	AddBatch(jobs []BatchFunc) error

	// Upsert 更新现有任务，如果不存在则添加。
	Upsert(jobId, spec string, f func() error) error

	// Get 检索任务的详细信息（spec、内部 ID）。
	Get(jobId string) (info JobInfo, ok bool)

	// Jobs 返回所有已注册任务 ID 的切片。
	Jobs() []string

	// Len 返回已注册任务的数量。
	Len() int

	// Remove 通过 ID 删除一个或多个任务。如果任何 ID 无效，则返回 ErrJobNotFound。
	Remove(jobId ...string) error

	// Clear 从调度器中移除所有任务。
	Clear()

	// Start 在新的 goroutine 中启动调度器。
	Start()

	// Stop 优雅地关闭调度器，等待正在运行的任务。
	Stop()
}

// BatchFunc 表示用于批量添加的任务。
type BatchFunc struct {
	JobId string
	Spec  string
	Func  func() error
}

// JobInfo 包含有关已调度任务的详细信息。
type JobInfo struct {
	JobId   string
	Spec    string
	// ...
}
```

### 配置选项 (`cronjob.Option`)

将这些选项传递给 `cronjob.New()`：

*   `cronjob.WithCronSeconds()`: 在 cron 规范中启用秒字段（例如 `* * * * * *`）。
*   `cronjob.WithSkipIfJobRunning()`: 如果任务的前一个实例仍在运行，则阻止其运行。
*   `cronjob.WithLogger(logger cronlib.Logger)`: 设置自定义日志记录器（必须实现 `cronlib.Logger` 接口）。建议在生产环境中使用。
*   `cronjob.WithLocation(loc *time.Location)`: 设置用于解释计划的时区（默认为 `time.Local`）。
*   `cronjob.WithRetry(retry uint, wait time.Duration)`: 配置常规重试（固定的 `wait` 持续时间）。`retry` 是初始失败*之后*的尝试次数。
*   `cronjob.WithRetryBackoff(retry uint, initialWait, maxWait time.Duration)`: 配置指数退避重试。等待时间从 `initialWait` 开始，每次加倍（带有抖动），最多不超过 `maxWait`。

### 预定义错误

*   `cronjob.ErrJobNotFound`: 指定的任务 ID 不存在。
*   `cronjob.ErrJobIdEmpty`: 提供了空字符串作为任务 ID。
*   `cronjob.ErrSpecEmpty`: 提供了空字符串作为 cron 规范。
*   `cronjob.ErrJobIdAlreadyExists`: 尝试添加一个 ID 已被使用的任务。

## 高级用法

### 批量操作

在单个原子操作中添加多个任务。如果任何任务验证或添加失败，整个批次将回滚。

```go
jobs := []cronjob.BatchFunc{
	{JobId: "batchJob1", Spec: "0 0 * * *", Func: func() error { fmt.Println("批量任务 1"); return nil }},
	{JobId: "batchJob2", Spec: "@hourly", Func: func() error { fmt.Println("批量任务 2"); return nil }},
}
err := cron.AddBatch(jobs)
if err != nil {
	panic(err)
}
```

### 重试机制

配置失败任务的重试方式。

**常规重试:**

```go
cron, _ := cronjob.New(cronjob.WithRetry(5, 10*time.Second))
```

**指数退避重试:**

```go
cron, _ := cronjob.New(cronjob.WithRetryBackoff(5, 1*time.Second, 60*time.Second))
```

### 更新任务 (`Upsert`)

修改现有任务的计划或函数，如果不存在则添加它。

```go
// 将 job1 更改为每 10 秒运行一次
err := cron.Upsert("job1", "*/10 * * * * *", func() error {
	logger.Info("运行更新后的 job1")
	return nil
})
```

## 最佳实践

1.  **优雅关闭:** 在应用程序退出之前始终调用 `cron.Stop()`。
2.  **有意义的任务 ID:** 使用描述性且唯一的 ID，以便于识别、记录和调试。
3.  **日志记录:** 使用 `WithLogger` 集成生产级日志记录器（如 `slog`, `zap`, `logrus`）。
4.  **时区:** 如果您的任务需要根据服务器默认时区以外的特定时区运行，请使用 `WithLocation`。
5.  **重试策略:** 根据任务的幂等性和潜在故障的性质选择 `WithRetry` 或 `WithRetryBackoff`。

## 贡献

欢迎贡献！请随时提交问题或拉取请求。

## 许可证

本项目采用 MIT 许可证 - 详情请参阅 [LICENSE](LICENSE) 文件。