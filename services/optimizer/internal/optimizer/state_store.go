package optimizer

import (
	"sync"
	"time"

	"github.com/aman-churiwal/gridflow-shared/proto/gen"
)

type VehicleState struct {
	LastPings           []*gen.VehiclePing // ring buffer of last N pings (N=3 for now)
	LastSeen            time.Time
	LastRoute           [2]*gen.VehiclePing // last two pings used as a route proxy
	ConnectionLostFired bool
}

type StateStore struct {
	VehiclesState map[string]*VehicleState
	mu            sync.RWMutex
}
