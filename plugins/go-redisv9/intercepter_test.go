package goredisv9

import (
	"context"
	"github.com/apache/skywalking-go/plugins/core"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func init() {
	core.ResetTracingContext()
}

func TestInvoke(t *testing.T) {
	defer core.ResetTracingContext()

	interceptor := &GoRedisInterceptor{}
	clusterClient := redis.NewClusterClient(&redis.ClusterOptions{
		Addrs:          []string{"localhost:6379", "localhost:7379"},
		DialTimeout:    time.Second * 10,
		PoolSize:       10,
		RouteByLatency: true,
	})
	err := interceptor.AfterInvoke(nil, clusterClient)
	assert.Nil(t, err, "failed to invoke AfterInvoke")
	status, err := clusterClient.Ping(context.Background()).Result()
	if status != "PONG" {
		t.Fatalf("err: %v", err)
	}
	clusterClient.Get(context.Background(), "test").Val()
}
