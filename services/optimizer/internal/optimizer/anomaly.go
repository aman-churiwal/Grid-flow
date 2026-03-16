package optimizer

import "time"

type AnomalyEvent struct {
	VehicleID   string
	Type        AnomalyType
	Description string
	Lat         float64
	Lng         float64
	DetectedAt  time.Time
}

type AnomalyType string

const (
	AnomalyZeroSpeed      AnomalyType = "ZERO_SPEED"
	AnomalyOverSpeed      AnomalyType = "OVER_SPEED"
	AnomalyConnectionLost AnomalyType = "CONNECTION_LOST"
	AnomalyRouteDeviation AnomalyType = "ROUTE_DEVIATION"
)
