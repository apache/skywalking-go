package sarama

import (
	"fmt"

	"github.com/Shopify/sarama"

	"github.com/apache/skywalking-go/plugins/core/operator"
)

type AsyncProducerInterceptor struct {
}

type ConsumerInterceptor struct {
}

// BeforeInvoke would be called before the target method invocation.
func (p *AsyncProducerInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	client, ok := invocation.Args()[0].(sarama.Client)
	if !ok {
		return fmt.Errorf("sarama :skyWalking cannot create producer interceptor for client not match Client interface: %T", client)
	}
	conf := client.Config()
	var brokers []string
	for _, s := range client.Brokers() {
		brokers = append(brokers, s.Addr())
	}
	conf.Producer.Interceptors = append(conf.Producer.Interceptors, &producerInterceptor{
		brokers: brokers,
	})
	err := conf.Validate()
	if err != nil {
		return fmt.Errorf("sarama :skyWalking validate producer interceptor config failed: %v", err)
	}
	return nil
}

// AfterInvoke would be called after the target method invocation.
func (p *AsyncProducerInterceptor) AfterInvoke(invocation operator.Invocation, result ...interface{}) error {
	return nil
}

// BeforeInvoke would be called before the target method invocation.
func (c *ConsumerInterceptor) BeforeInvoke(invocation operator.Invocation) error {
	client, ok := invocation.Args()[0].(sarama.Client)
	if !ok {
		return fmt.Errorf("sarama :skyWalking cannot create consumer interceptor for client not match Client interface: %T", client)
	}
	conf := client.Config()
	var brokers []string
	for _, s := range client.Brokers() {
		brokers = append(brokers, s.Addr())
	}
	conf.Consumer.Interceptors = append(conf.Consumer.Interceptors, &consumerInterceptor{
		brokers: brokers,
	})
	err := conf.Validate()
	if err != nil {
		return fmt.Errorf("sarama :skyWalking validate consumer interceptor config failed: %v", err)
	}
	return nil
}

// AfterInvoke would be called after the target method invocation.
func (c *ConsumerInterceptor) AfterInvoke(invocation operator.Invocation, result ...interface{}) error {
	return nil
}
