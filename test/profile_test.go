package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/apache/skywalking-go/plugins/core/operator"
	"github.com/apache/skywalking-go/plugins/core/reporter"
	"github.com/apache/skywalking-go/plugins/core/reporter/grpc"
)

func main() {
	// 创建logger
	logger := &operator.DefaultLogOperator{}

	// 创建连接管理器
	connManager := reporter.NewConnectionManager()

	// 创建CDS管理器
	cdsManager := reporter.NewCDSManager()

	// 创建gRPC reporter
	reporter, err := grpc.NewGRPCReporter(
		logger,
		"localhost:11800", // OAP服务器地址
		30*time.Second,    // checkInterval
		10*time.Second,    // profileFetchIntervalVal
		connManager,
		cdsManager,
	)
	if err != nil {
		log.Fatalf("Failed to create reporter: %v", err)
	}

	// 创建entity
	entity := &reporter.Entity{
		ServiceName:         "test-service",
		ServiceInstanceName: "test-instance",
		Props: map[string]string{
			"language": "go",
		},
	}

	// 启动reporter
	reporter.Boot(entity, nil)

	// 模拟profile任务
	fmt.Println("开始模拟profile任务...")

	// 模拟开始profiling
	reporter.Profiling("test-trace-id", "/test-endpoint")

	// 模拟添加span ID
	reporter.AddSpanIdToProfile(1)
	reporter.AddSpanIdToProfile(2)
	reporter.AddSpanIdToProfile(3)

	// 模拟检查profile值
	reporter.CheckProfileValue(1, time.Now().UnixMilli(), time.Now().Add(100*time.Millisecond).UnixMilli())
	reporter.CheckProfileValue(2, time.Now().UnixMilli(), time.Now().Add(200*time.Millisecond).UnixMilli())
	reporter.CheckProfileValue(3, time.Now().UnixMilli(), time.Now().Add(50*time.Millisecond).UnixMilli())

	// 等待一段时间让profile数据生成
	fmt.Println("等待profile数据生成...")
	time.Sleep(5 * time.Second)

	// 结束profiling
	reporter.EndProfiling()

	// 等待一段时间让数据发送
	fmt.Println("等待数据发送...")
	time.Sleep(10 * time.Second)

	// 关闭reporter
	reporter.Close()

	fmt.Println("测试完成")
} 