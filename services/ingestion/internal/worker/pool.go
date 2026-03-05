package worker

import (
	"context"
	"sync"

	"github.com/aman-churiwal/gridflow-shared/logger"
	"github.com/aman-churiwal/gridflow-shared/proto/gen"
)

type Pool struct {
	workers   int
	pings     <-chan *gen.VehiclePing
	wg        *sync.WaitGroup
	logger    *logger.Logger
	publisher IPublisher
}

type IPublisher interface {
	Publish(ctx context.Context, ping *gen.VehiclePing) error
}

func NewPool(workers int, pings <-chan *gen.VehiclePing, logger *logger.Logger, publisher IPublisher) *Pool {
	return &Pool{
		workers:   workers,
		pings:     pings,
		wg:        &sync.WaitGroup{},
		logger:    logger,
		publisher: publisher,
	}
}

func (p *Pool) Start(ctx context.Context) {
	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.run(ctx, i)
	}
}

func (p *Pool) Wait() {
	p.wg.Wait()
}

func (p *Pool) run(ctx context.Context, workerID int) {
	defer p.wg.Done()
	for {
		select {
		case <-ctx.Done():
			// Drain remaining pings before exit
			for {
				select {
				case ping, ok := <-p.pings:
					if !ok {
						return
					}
					p.process(ping, workerID)
				default:
					return
				}
			}
		case ping, ok := <-p.pings:
			if !ok {
				// Channel closed - shut down this worker
				return
			}
			p.process(ping, workerID)
		}
	}
}

func (p *Pool) process(ping *gen.VehiclePing, workerID int) {
	p.logger.Info(context.Background()).
		Int("worker_id", workerID).
		Str("vehicle_id", ping.VehicleId).
		Float64("lat", ping.Lat).
		Float64("lng", ping.Lng).
		Int64("timestamp", ping.Timestamp).
		Msg("processing ping")

	if err := p.publisher.Publish(context.Background(), ping); err != nil {
		p.logger.Error(context.Background()).Err(err).
			Str("vehicle_id", ping.VehicleId).
			Msg("failed to publish ping")
	}
}
