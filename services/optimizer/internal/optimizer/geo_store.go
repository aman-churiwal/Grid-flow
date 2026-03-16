package optimizer

import (
	"context"
	"time"

	"github.com/aman-churiwal/gridflow-shared/logger"
	"github.com/redis/go-redis/v9"
)

const redisGeoKey = "geo:vehicles"
const vehicleTTLKey = "vehicle:last_seen:"

type VehicleLocation struct {
	VehicleID string
	Lat       float64
	Lng       float64
}

type GeoStore struct {
	redisClient *redis.Client
	logger      *logger.Logger
	radius      int
}

func NewGeoStore(client *redis.Client, logger *logger.Logger, radius int) *GeoStore {
	return &GeoStore{
		redisClient: client,
		logger:      logger,
		radius:      radius,
	}
}

func (g *GeoStore) UpsertVehicle(ctx context.Context, vehicleID string, longitude, latitude float64) error {
	geoLocation := &redis.GeoLocation{
		Name:      vehicleID,
		Longitude: longitude,
		Latitude:  latitude,
	}

	if err := g.redisClient.GeoAdd(ctx, redisGeoKey, geoLocation).Err(); err != nil {
		return err
	}

	err := g.redisClient.Set(ctx, vehicleTTLKey+vehicleID, "", 30*time.Second).Err()

	return err
}

func (g *GeoStore) FindNearby(ctx context.Context, vehicleID string, longitude, latitude float64) []VehicleLocation {
	geoSearchQuery := redis.GeoSearchQuery{
		Longitude:  longitude,
		Latitude:   latitude,
		Radius:     float64(g.radius),
		RadiusUnit: "km",
		Sort:       "ASC",
	}

	result := g.redisClient.GeoSearchLocation(ctx, redisGeoKey, &redis.GeoSearchLocationQuery{
		GeoSearchQuery: geoSearchQuery,
		WithCoord:      true,
	}).Val()

	var locations []VehicleLocation
	for _, v := range result {
		if v.Name == vehicleID {
			// Skipping self
			continue
		}
		locations = append(locations, VehicleLocation{
			VehicleID: v.Name,
			Lat:       v.Latitude,
			Lng:       v.Longitude,
		})
	}

	return locations
}

func (g *GeoStore) StartPruner(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				geoQuery := &redis.GeoSearchQuery{
					Longitude:  0,
					Latitude:   0,
					Radius:     20000,
					RadiusUnit: "km",
				}

				vehicles := g.redisClient.GeoSearch(ctx, redisGeoKey, geoQuery).Val()

				pruned := 0
				for _, v := range vehicles {
					ok := g.redisClient.Exists(ctx, vehicleTTLKey+v).Val()

					if ok == 0 {
						if err := g.redisClient.ZRem(ctx, redisGeoKey, v).Err(); err != nil {
							g.logger.Error(ctx).Err(err).Str("vehicle_id", v).Msg("failed to remove vehicle from geo index")
							continue
						}
						pruned++
					}
				}

				if pruned > 0 {
					g.logger.Info(ctx).Int("pruned", pruned).Msg("stale vehicles pruned")
				}
			}
		}
	}()
}
