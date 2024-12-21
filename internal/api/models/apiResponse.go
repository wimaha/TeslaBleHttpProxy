package models

import (
	"encoding/json"
	"sync"
)

type ApiResponse struct {
	Wait     *sync.WaitGroup
	Result   bool
	Error    string
	Response json.RawMessage
}
