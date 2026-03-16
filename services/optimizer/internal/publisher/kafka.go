package publisher

import (
	"context"
	"encoding/json"

	"github.com/aman-churiwal/gridflow-optimizer/internal/optimizer"
	"github.com/aman-churiwal/gridflow-shared/logger"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/segmentio/kafka-go"
)

var publishErrorsTotal = prometheus.NewCounter(prometheus.CounterOpts{
	Name: "anomaly_publish_errors_total",
	Help: "Total number of failed anomaly publishes to Kafka",
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

func (p *KafkaPublisher) Publish(ctx context.Context, event optimizer.AnomalyEvent) error {
	eventJson, err := json.Marshal(event)
	if err != nil {
		p.logger.Error(ctx).Err(err).Msg("Error converting event to JSON")
		return err
	}

	msg := kafka.Message{
		Value: eventJson,
		Key:   []byte(event.VehicleID),
	}

	if err := p.writer.WriteMessages(ctx, msg); err != nil {
		p.logger.Error(ctx).
			Err(err).
			Str("vehicle_id", event.VehicleID).
			Int64("timestamp", event.DetectedAt.Unix()).
			Msg("failed to publish anomaly event to Kafka")
		publishErrorsTotal.Inc()
		return err
	}

	return nil
}

func (p *KafkaPublisher) Close() error {
	return p.writer.Close()
}
