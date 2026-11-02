package models

import "encoding/json"

type Ret struct {
	Response Response `json:"response"`
}

type Response struct {
	Result   bool            `json:"result"`
	Reason   string          `json:"reason"`
	Vin      string          `json:"vin"`
	Command  string          `json:"command"`
	Response json.RawMessage `json:"response,omitempty"`
	// Stale is set when the data comes from the cache although the car is asleep (allow_stale=true).
	// CacheAgeSeconds is the age of the oldest value in the response.
	Stale           bool `json:"stale,omitempty"`
	CacheAgeSeconds *int `json:"cache_age_seconds,omitempty"`
}
