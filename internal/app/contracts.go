package app

// AppError 是可跨绑定返回的安全错误，不包含原始系统错误。
type AppError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
}

type Info struct {
	Name         string `json:"name"`
	Version      string `json:"version"`
	WailsVersion string `json:"wails_version"`
	Platform     string `json:"platform"`
	Architecture string `json:"architecture"`
}

type InfoResult struct {
	OK        bool      `json:"ok"`
	Data      *Info     `json:"data,omitempty"`
	Error     *AppError `json:"error,omitempty"`
	RequestID string    `json:"request_id"`
}

type OperationRequest struct {
	OperationID string `json:"operation_id"`
}

type OperationSnapshot struct {
	OperationID string `json:"operation_id"`
	Status      string `json:"status"`
	Completed   int    `json:"completed"`
	Total       int    `json:"total"`
	Sequence    int    `json:"sequence"`
}

type OperationResult struct {
	OK        bool               `json:"ok"`
	Data      *OperationSnapshot `json:"data,omitempty"`
	Error     *AppError          `json:"error,omitempty"`
	RequestID string             `json:"request_id"`
}

type OperationEvent struct {
	SchemaVersion int               `json:"schema_version"`
	EventID       string            `json:"event_id"`
	OperationID   string            `json:"operation_id"`
	Sequence      int               `json:"sequence"`
	Type          string            `json:"type"`
	OccurredAt    string            `json:"occurred_at"`
	Payload       OperationSnapshot `json:"payload"`
}
