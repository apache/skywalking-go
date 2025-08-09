package main

import (
	"fmt"
	"time"

	"github.com/apache/skywalking-go/plugins/core/reporter"
	common "skywalking.apache.org/repo/goapi/collect/common/v3"
)

func main() {
	// 创建ProfileManager
	profileManager := reporter.NewProfileManager()

	// 模拟添加profile任务
	args := []*common.KeyStringValuePair{
		{Key: "TaskId", Value: "test-task-001"},
		{Key: "EndpointName", Value: "/test-endpoint"},
		{Key: "Duration", Value: "1"}, // 1分钟
		{Key: "MinDurationThreshold", Value: "100"}, // 100ms
		{Key: "DumpPeriod", Value: "1000"}, // 1秒
		{Key: "MaxSamplingCount", Value: "100"},
		{Key: "StartTime", Value: fmt.Sprintf("%d", time.Now().UnixMilli())},
		{Key: "CreateTime", Value: fmt.Sprintf("%d", time.Now().UnixMilli())},
		{Key: "SerialNumber", Value: "test-serial-001"},
	}

	profileManager.AddProfileTask(args)

	fmt.Println("已添加profile任务")

	// 开始profiling
	traceSegmentID := "test-trace-segment-001"
	err := profileManager.StartProfiling(traceSegmentID)
	if err != nil {
		fmt.Printf("启动profiling失败: %v\n", err)
		return
	}

	fmt.Println("开始profiling...")

	// 添加一些span ID
	profileManager.AddSpanId(1)
	profileManager.AddSpanId(2)
	profileManager.AddSpanId(3)

	// 检查一些时间值
	profileManager.CheckTimeIfEnough(1, 150) // 超过阈值
	profileManager.CheckTimeIfEnough(2, 200) // 超过阈值
	profileManager.CheckTimeIfEnough(3, 50)  // 低于阈值

	// 等待一段时间
	fmt.Println("等待profile数据生成...")
	time.Sleep(3 * time.Second)

	// 结束profiling
	profileManager.EndProfiling()

	fmt.Println("profiling结束")

	// 等待结果
	fmt.Println("等待profile结果...")
	select {
	case result := <-profileManager.ReportResults:
		fmt.Printf("收到profile结果:\n")
		fmt.Printf("  TaskID: %s\n", result.TaskID)
		fmt.Printf("  TraceSegmentID: %s\n", result.TraceSegmentID)
		fmt.Printf("  SpanIDs: %v\n", result.SpanIDs)
		fmt.Printf("  Payload chunks: %d\n", len(result.Payload))
		for i, chunk := range result.Payload {
			fmt.Printf("    Chunk %d size: %d bytes\n", i, len(chunk))
		}
	case <-time.After(5 * time.Second):
		fmt.Println("等待profile结果超时")
	}

	fmt.Println("测试完成")
} 