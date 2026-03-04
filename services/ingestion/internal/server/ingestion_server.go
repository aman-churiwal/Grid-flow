package server

import (
	"errors"
	"io"

	"github.com/aman-churiwal/gridflow-ingestion/internal/session"
	"github.com/aman-churiwal/gridflow-shared/logger"
	"github.com/aman-churiwal/gridflow-shared/proto/gen"
)

type IngestionServer struct {
	gen.UnimplementedIngestionServiceServer
	sessions *session.Store
	logger   *logger.Logger
}

func NewIngestionServer(sessions *session.Store, logger *logger.Logger) *IngestionServer {
	return &IngestionServer{sessions: sessions, logger: logger}
}

func (s *IngestionServer) StreamTelemetry(stream gen.IngestionService_StreamTelemetryServer) error {
	firstPing, err := stream.Recv()
	if err != nil {
		return err
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

		s.logger.Info(stream.Context()).
			Float64("lat", ping.Lat).
			Float64("lng", ping.Lng).
			Str("vehicle_id", vehicleID).
			Msg("ping received")
	}
}
