package handlers

import (
	"encoding/json"
	"time"
)

// SeedVehicleDataForTest puts a cache entry of the given age into the vehicle_data cache.
func SeedVehicleDataForTest(vin, endpoint, data string, age time.Duration) {
	vehicleDataCacheMux.Lock()
	defer vehicleDataCacheMux.Unlock()
	vehicleDataCache[generateVehicleDataCacheKey(vin, endpoint)] = &vehicleDataCacheEntry{
		data:      json.RawMessage(data),
		timestamp: time.Now().Add(-age),
	}
}
