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
	Err      error // the error behind Error, for errors.Is
	Response json.RawMessage
	Ctx      context.Context
}
