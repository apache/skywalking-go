package sarama

import (
	"strings"

	"github.com/Shopify/sarama"

	"github.com/apache/skywalking-go/plugins/core/tracing"
)

type producerInterceptor struct {
	brokers []string
}

func (p *producerInterceptor) OnSend(msg *sarama.ProducerMessage) {
	s, err := tracing.CreateExitSpan(
		// operationName
		MQType+"/"+msg.Topic+"/Producer",

		// peer
		strings.Join(p.brokers, ","),

		// injector
		func(k, v string) error {
			h := sarama.RecordHeader{
				Key: []byte(k), Value: []byte(v),
			}
			msg.Headers = append(msg.Headers, h)
			return nil
		},

		// opts
		tracing.WithTag(tracing.TagMQBroker, strings.Join(p.brokers, ",")),
		tracing.WithTag(tracing.TagMQTopic, msg.Topic),
	)

	if err != nil {
		sarama.Logger.Printf("skyWalking create exit span failed: %v", err)
		return
	}

	defer s.End()
	return
}
