package consumer

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/aman-churiwal/gridflow-shared/logger"
	"github.com/aman-churiwal/gridflow-shared/proto/gen"
	"github.com/segmentio/kafka-go"
)

type Consumer struct {
	reader           *kafka.Reader
	optimizerChannel chan<- Message
	logger           *logger.Logger
}

type Message struct {
	Ping gen.VehiclePing
	Ack  func()
}

func NewConsumerGroup(reader *kafka.Reader, msgs chan<- Message, appLogger *logger.Logger) *Consumer {
	return &Consumer{
		reader:           reader,
		optimizerChannel: msgs,
		logger:           appLogger,
	}
}

func (c *Consumer) Start(ctx context.Context) {
	go func() {
		for {
			m, err := c.reader.FetchMessage(ctx)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					return
				}

				c.logger.Error(ctx).Err(err).Msg("error reading message")
				continue
			}

			var ping gen.VehiclePing
			if err := json.Unmarshal(m.Value, &ping); err != nil {
				c.logger.Error(ctx).Err(err).
					Bytes("message", m.Value).
					Int64("offset", m.Offset).
					Msg("error unmarshalling message")

				_ = c.reader.CommitMessages(ctx, m)
				continue
			}

			c.logger.Info(ctx).
				Str("vehicle_id", ping.VehicleId).
				Str("topic", m.Topic).
				Msg("message received")

			done := make(chan struct{})
			newMessage := Message{
				Ping: ping,
				Ack: func() {
					if err := c.reader.CommitMessages(ctx, m); err != nil {
						c.logger.Error(ctx).Err(err).Msg("error committing message")
					}
					close(done)
				},
			}
			select {
			case c.optimizerChannel <- newMessage:
			case <-ctx.Done():
				return
			}

			select {
			case <-done:
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (c *Consumer) Close() error {
	if err := c.reader.Close(); err != nil {
		c.logger.Error(context.Background()).Err(err).Msg("Unable to close Kafka reader")

		return err
	}

	return nil
}
