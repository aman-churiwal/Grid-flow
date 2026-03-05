package publisher

import (
	"context"
	"encoding/json"

	"github.com/aman-churiwal/gridflow-shared/logger"
	"github.com/aman-churiwal/gridflow-shared/proto/gen"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/segmentio/kafka-go"
)

var publishErrorsTotal = prometheus.NewCounter(prometheus.CounterOpts{
	Name: "telemetry_publish_errors_total",
	Help: "Total number of failed telemetry publishes to Kafka",
})

func init() {
	prometheus.MustRegister(publishErrorsTotal)
}

type KafkaPublisher struct {
	writer *kafka.Writer
	logger *logger.Logger
}

func NewKafkaPublisher(brokers []string, topic string, logger *logger.Logger) *KafkaPublisher {
	writer := &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Topic:        topic,
		RequiredAcks: kafka.RequireAll,
		Async:        false,
		Balancer:     &kafka.Hash{},
	}

	return &KafkaPublisher{
		writer: writer,
		logger: logger,
	}
}

func (p *KafkaPublisher) Publish(ctx context.Context, ping *gen.VehiclePing) error {
	vehiclePingJSON, err := json.Marshal(ping)
	if err != nil {
		p.logger.Error(ctx).Err(err).Msg("Error converting ping to JSON")
		return err
	}

	msg := kafka.Message{
		Value: vehiclePingJSON,
		Key:   []byte(ping.VehicleId),
	}

	if err := p.writer.WriteMessages(ctx, msg); err != nil {
		p.logger.Error(ctx).
			Err(err).
			Str("vehicle_id", ping.VehicleId).
			Int64("timestamp", ping.Timestamp).
			Msg("failed to publish ping to Kafka")
		publishErrorsTotal.Inc()
		return err
	}

	return nil
}

func (p *KafkaPublisher) Close() error {
	return p.writer.Close()
}
