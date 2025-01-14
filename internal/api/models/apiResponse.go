package models

import (
	"context"
	"encoding/json"
	"sync"
)

type ApiResponse struct {
	Wait     *sync.WaitGroup
	Result   bool
	Error    string
	Response json.RawMessage
	Ctx      context.Context
}
