package server

import (
	"errors"
	"io"
	"time"

	"github.com/aman-churiwal/gridflow-ingestion/internal/ratelimit"
	"github.com/aman-churiwal/gridflow-ingestion/internal/session"
	"github.com/aman-churiwal/gridflow-shared/logger"
	"github.com/aman-churiwal/gridflow-shared/proto/gen"
)

type IngestionServer struct {
	gen.UnimplementedIngestionServiceServer
	sessions    *session.Store
	logger      *logger.Logger
	pings       chan *gen.VehiclePing
	rateLimiter ratelimit.IRateLimiter
}

func NewIngestionServer(sessions *session.Store, logger *logger.Logger, rateLimiter ratelimit.IRateLimiter) *IngestionServer {
	return &IngestionServer{
		sessions:    sessions,
		logger:      logger,
		pings:       make(chan *gen.VehiclePing, 100),
		rateLimiter: rateLimiter,
	}
}

func (s *IngestionServer) Pings() <-chan *gen.VehiclePing {
	return s.pings
}

func (s *IngestionServer) StreamTelemetry(stream gen.IngestionService_StreamTelemetryServer) error {
	firstPing, err := stream.Recv()
	if err != nil {
		return err
	}

	if err := validatePing(firstPing); err != nil {
		_ = stream.Send(&gen.TelemetryAck{
			VehicleId:  firstPing.VehicleId,
			ReceivedAt: time.Now().Unix(),
			Status:     gen.TelemetryStatus_INVALID,
		})
		return nil
	}

	allowed, err := s.rateLimiter.Allow(stream.Context(), firstPing.VehicleId)
	if err != nil {
		s.logger.Warn(stream.Context()).Err(err).
			Msg("rate limiter error, allowing ping")
		allowed = true
	}
	if !allowed {
		s.logger.Warn(stream.Context()).
			Str("vehicle_id", firstPing.VehicleId).
			Int64("timestamp", firstPing.Timestamp).
			Msg("rate limit hit")

		_ = stream.Send(&gen.TelemetryAck{
			VehicleId:  firstPing.VehicleId,
			ReceivedAt: time.Now().Unix(),
			Status:     gen.TelemetryStatus_RATE_LIMITED,
		})
		return nil
	}
	vehicleID := firstPing.VehicleId

	sess := &session.Session{
		VehicleID: vehicleID,
		Stream:    stream,
	}
	s.sessions.Add(vehicleID, sess)
	defer s.sessions.Remove(vehicleID)

	s.logger.Info(stream.Context()).
		Str("vehicle_id", vehicleID).
		Float64("lat", firstPing.Lat).
		Float64("lng", firstPing.Lng).
		Msg("First ping received")

	select {
	case s.pings <- firstPing:
		_ = stream.Send(&gen.TelemetryAck{
			VehicleId:  vehicleID,
			ReceivedAt: time.Now().Unix(),
			Status:     gen.TelemetryStatus_OK,
		})
	default:
		_ = stream.Send(&gen.TelemetryAck{
			VehicleId:  vehicleID,
			ReceivedAt: time.Now().Unix(),
			Status:     gen.TelemetryStatus_RATE_LIMITED,
		})
	}

	for {
		ping, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			// Client closed the stream cleanly
			return nil
		}

		if err != nil {
			// Client disconnected or network error
			return err
		}

		if err := validatePing(ping); err != nil {
			_ = stream.Send(&gen.TelemetryAck{
				VehicleId:  ping.VehicleId,
				ReceivedAt: time.Now().Unix(),
				Status:     gen.TelemetryStatus_INVALID,
			})
			continue
		}

		allowed, err := s.rateLimiter.Allow(stream.Context(), ping.VehicleId)
		if err != nil {
			s.logger.Warn(stream.Context()).Err(err).
				Msg("rate limiter error, allowing ping")
			allowed = true
		}

		if !allowed {
			s.logger.Warn(stream.Context()).
				Str("vehicle_id", ping.VehicleId).
				Int64("timestamp", ping.Timestamp).
				Msg("rate limit exceeded")

			_ = stream.Send(&gen.TelemetryAck{
				VehicleId:  ping.VehicleId,
				ReceivedAt: time.Now().Unix(),
				Status:     gen.TelemetryStatus_RATE_LIMITED,
			})
			continue
		}

		select {
		case s.pings <- ping:
			_ = stream.Send(&gen.TelemetryAck{
				VehicleId:  vehicleID,
				ReceivedAt: time.Now().Unix(),
				Status:     gen.TelemetryStatus_OK,
			})
		default:
			// Channel full — drop and notify
			_ = stream.Send(&gen.TelemetryAck{
				VehicleId:  vehicleID,
				ReceivedAt: time.Now().Unix(),
				Status:     gen.TelemetryStatus_RATE_LIMITED,
			})
		}

		s.logger.Info(stream.Context()).
			Float64("lat", ping.Lat).
			Float64("lng", ping.Lng).
			Str("vehicle_id", vehicleID).
			Msg("ping received")
	}
}

func validatePing(ping *gen.VehiclePing) error {
	if ping.VehicleId == "" {
		return errors.New("vehicle_id is required")
	}
	if ping.Lat < -90 || ping.Lat > 90 {
		return errors.New("lat must be between -90 and 90")
	}
	if ping.Lng < -180 || ping.Lng > 180 {
		return errors.New("lng must be between -180 and 180")
	}
	if ping.Timestamp > time.Now().Unix() {
		return errors.New("timestamp must not be in the future")
	}

	return nil
}
