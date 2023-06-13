package sarama

import (
	"strings"

	"github.com/Shopify/sarama"

	"github.com/apache/skywalking-go/plugins/core/tracing"
)

type consumerInterceptor struct {
	brokers []string
}

func (c *consumerInterceptor) OnConsume(msg *sarama.ConsumerMessage) {
	s, err := tracing.CreateEntrySpan(
		// operationName
		MQType+"/"+msg.Topic+"/Consumer",

		// extractor
		func(k string) (string, error) {
			// find SkyWalking header in msg.Headers
			for _, h := range msg.Headers {
				if string(h.Key) == k {
					return string(h.Value), nil
				}
			}
			return "", nil
		},
		// opts
		tracing.WithTag(tracing.TagMQBroker, strings.Join(c.brokers, ",")),
		tracing.WithTag(tracing.TagMQTopic, msg.Topic),
	)

	if err != nil {
		sarama.Logger.Printf("skyWalking create entry span failed: %v", err)
		return
	}

	defer s.End()
	return
}
