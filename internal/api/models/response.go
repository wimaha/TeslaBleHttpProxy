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
}
