package handlers

import (
	"encoding/json"
	"time"
)

// SetLimitsForTest shortens the stale-data limit and the wake-up retry interval for a test and
// returns a function that restores them.
func SetLimitsForTest(stale, wakeRetry time.Duration) (restore func()) {
	oldStale, oldRetry := staleMaxAge, wakeRetryInterval
	staleMaxAge, wakeRetryInterval = stale, wakeRetry
	return func() { staleMaxAge, wakeRetryInterval = oldStale, oldRetry }
}

// SeedVehicleDataForTest puts a cache entry of the given age into the vehicle_data cache.
func SeedVehicleDataForTest(vin, endpoint, data string, age time.Duration) {
	vehicleDataCacheMux.Lock()
	defer vehicleDataCacheMux.Unlock()
	vehicleDataCache[generateVehicleDataCacheKey(vin, endpoint)] = &vehicleDataCacheEntry{
		data:      json.RawMessage(data),
		timestamp: time.Now().Add(-age),
	}
}
