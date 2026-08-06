package handler

import (
	"context"

	"github.com/uber-research/rpcshield/go/exampleapps/rpcshielddemo/controller"
)

// Message stands in for an internal Kafka-consumer message type;
// only its use as a method parameter matters for inbound detection.
type Message struct {
	Content string
}

func (h *handler) HandleKafkaConsumerMessage(ctx context.Context, message Message) error {
	return helperHandleKafkaConsumerMessage(ctx, message)
}

// The following is a helper function that should not itself be detected as a
// kafka inbound.
var kafkaCtrl controller.Controller

func helperHandleKafkaConsumerMessage(ctx context.Context, message Message) error {
	if kafkaCtrl != nil {
		_, err := kafkaCtrl.Create(ctx, message.Content)
		if err != nil {
			return err
		}
	}
	return nil
}

// Other than a method on a struct that satisfies the MessageHandlerFunc type,
// we can also attach a method to a function type that satisfies the
// MessageHandlerFunc type.

// MessageHandlerFuncT is a function type that satisfies the
// MessageHandlerFunc interface.
type MessageHandlerFuncT func(ctx context.Context, message Message) error

// HandleKafkaConsumerMessage is a method that satisfies the
// MessageHandlerFunc interface.
func (f MessageHandlerFuncT) HandleKafkaConsumerMessage(ctx context.Context, message Message) error {
	return f(ctx, message)
}

// The following function matches the function type, but we should ignore the
// methods defined in interfaces.

type someInterface interface {
	Handle(ctx context.Context, message Message) error
}
