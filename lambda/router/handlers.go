package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/pay-theory/streamer/pkg/streamer"
)

// registerHandlers registers all production handlers
func registerHandlers(router *streamer.DefaultRouter) error {
	// Register simple handlers
	if err := router.Handle("echo", streamer.NewEchoHandler()); err != nil {
		return fmt.Errorf("failed to register echo handler: %w", err)
	}

	if err := router.Handle("health", NewHealthHandler()); err != nil {
		return fmt.Errorf("failed to register health handler: %w", err)
	}

	// Register async handlers
	if err := router.Handle("generate_report", NewReportHandler()); err != nil {
		return fmt.Errorf("failed to register report handler: %w", err)
	}

	if err := router.Handle("process_data", NewDataProcessingHandler()); err != nil {
		return fmt.Errorf("failed to register data processing handler: %w", err)
	}

	if err := router.Handle("bulk_operation", NewBulkHandler()); err != nil {
		return fmt.Errorf("failed to register bulk handler: %w", err)
	}

	logger.Printf("Registered %d handlers", 5)
	return nil
}

// HealthHandler returns system health status
type HealthHandler struct {
	estimatedDuration time.Duration
}

func NewHealthHandler() *HealthHandler {
	return &HealthHandler{
		estimatedDuration: 10 * time.Millisecond,
	}
}

func (h *HealthHandler) EstimatedDuration() time.Duration {
	return h.estimatedDuration
}

func (h *HealthHandler) Validate(req *streamer.Request) error {
	return nil // No validation needed for health check
}

func (h *HealthHandler) Process(ctx context.Context, req *streamer.Request) (*streamer.Result, error) {
	return &streamer.Result{
		RequestID: req.ID,
		Success:   true,
		Data: map[string]interface{}{
			"status":    "healthy",
			"timestamp": time.Now().Unix(),
			"version":   "1.0.0",
		},
	}, nil
}

// ReportHandler generates reports (async)
type ReportHandler struct {
	estimatedDuration time.Duration
}

// ReportParams defines the structure for report generation requests
type ReportParams struct {
	StartDate     time.Time `json:"start_date"`
	EndDate       time.Time `json:"end_date"`
	Format        string    `json:"format"` // pdf, csv, excel
	IncludeCharts bool      `json:"include_charts"`
	ReportType    string    `json:"report_type"` // monthly, quarterly, annual
}

func NewReportHandler() *ReportHandler {
	return &ReportHandler{
		estimatedDuration: 2 * time.Minute, // Async processing
	}
}

func (h *ReportHandler) EstimatedDuration() time.Duration {
	return h.estimatedDuration
}

func (h *ReportHandler) Validate(req *streamer.Request) error {
	if req.Payload == nil {
		return errors.New("payload is required")
	}

	var params ReportParams
	if err := json.Unmarshal(req.Payload, &params); err != nil {
		return fmt.Errorf("invalid payload format: %w", err)
	}

	// Validate dates
	if params.StartDate.IsZero() || params.EndDate.IsZero() {
		return errors.New("start_date and end_date are required")
	}

	if params.StartDate.After(params.EndDate) {
		return errors.New("start_date must be before end_date")
	}

	// Validate format
	validFormats := map[string]bool{"pdf": true, "csv": true, "excel": true}
	if !validFormats[params.Format] {
		return fmt.Errorf("invalid format: %s (must be pdf, csv, or excel)", params.Format)
	}

	// Validate report type
	validTypes := map[string]bool{"monthly": true, "quarterly": true, "annual": true}
	if !validTypes[params.ReportType] {
		return fmt.Errorf("invalid report_type: %s", params.ReportType)
	}

	return nil
}

func (h *ReportHandler) Process(ctx context.Context, req *streamer.Request) (*streamer.Result, error) {
	// This will be called by async processor, not here
	return nil, errors.New("use ProcessWithProgress for async handlers")
}

// ProcessWithProgress implements async processing with progress updates
func (h *ReportHandler) ProcessWithProgress(
	ctx context.Context,
	req *streamer.Request,
	reporter streamer.ProgressReporter,
) (*streamer.Result, error) {
	// This will be implemented in the async processor
	// For now, return a placeholder
	return nil, errors.New("async processing not yet implemented")
}

// DataProcessingHandler processes large data sets (async)
type DataProcessingHandler struct {
	estimatedDuration time.Duration
}

type DataProcessingParams struct {
	DatasetID    string   `json:"dataset_id"`
	Operations   []string `json:"operations"`    // filter, transform, aggregate
	OutputFormat string   `json:"output_format"` // json, parquet, csv
}

func NewDataProcessingHandler() *DataProcessingHandler {
	return &DataProcessingHandler{
		estimatedDuration: 5 * time.Minute, // Async processing
	}
}

func (h *DataProcessingHandler) EstimatedDuration() time.Duration {
	return h.estimatedDuration
}

func (h *DataProcessingHandler) Validate(req *streamer.Request) error {
	if req.Payload == nil {
		return errors.New("payload is required")
	}

	var params DataProcessingParams
	if err := json.Unmarshal(req.Payload, &params); err != nil {
		return fmt.Errorf("invalid payload format: %w", err)
	}

	if params.DatasetID == "" {
		return errors.New("dataset_id is required")
	}

	if len(params.Operations) == 0 {
		return errors.New("at least one operation is required")
	}

	// Validate operations
	validOps := map[string]bool{
		"filter": true, "transform": true, "aggregate": true,
		"sort": true, "deduplicate": true,
	}
	for _, op := range params.Operations {
		if !validOps[op] {
			return fmt.Errorf("invalid operation: %s", op)
		}
	}

	// Validate output format
	validFormats := map[string]bool{"json": true, "parquet": true, "csv": true}
	if !validFormats[params.OutputFormat] {
		return fmt.Errorf("invalid output_format: %s", params.OutputFormat)
	}

	return nil
}

func (h *DataProcessingHandler) Process(ctx context.Context, req *streamer.Request) (*streamer.Result, error) {
	return nil, errors.New("use ProcessWithProgress for async handlers")
}

// BulkHandler handles bulk operations (async)
type BulkHandler struct {
	estimatedDuration time.Duration
}

type BulkOperationParams struct {
	OperationType string                   `json:"operation_type"` // create, update, delete
	EntityType    string                   `json:"entity_type"`    // user, product, order
	Items         []map[string]interface{} `json:"items"`
	BatchSize     int                      `json:"batch_size"`
}

func NewBulkHandler() *BulkHandler {
	return &BulkHandler{
		estimatedDuration: 10 * time.Minute, // Async processing
	}
}

func (h *BulkHandler) EstimatedDuration() time.Duration {
	return h.estimatedDuration
}

func (h *BulkHandler) Validate(req *streamer.Request) error {
	if req.Payload == nil {
		return errors.New("payload is required")
	}

	var params BulkOperationParams
	if err := json.Unmarshal(req.Payload, &params); err != nil {
		return fmt.Errorf("invalid payload format: %w", err)
	}

	// Validate operation type
	validOps := map[string]bool{"create": true, "update": true, "delete": true}
	if !validOps[params.OperationType] {
		return fmt.Errorf("invalid operation_type: %s", params.OperationType)
	}

	// Validate entity type
	validEntities := map[string]bool{"user": true, "product": true, "order": true}
	if !validEntities[params.EntityType] {
		return fmt.Errorf("invalid entity_type: %s", params.EntityType)
	}

	// Validate items
	if len(params.Items) == 0 {
		return errors.New("items cannot be empty")
	}

	if len(params.Items) > 10000 {
		return errors.New("too many items (max 10000)")
	}

	// Validate batch size
	if params.BatchSize <= 0 {
		params.BatchSize = 25 // Default
	} else if params.BatchSize > 100 {
		return errors.New("batch_size too large (max 100)")
	}

	return nil
}

func (h *BulkHandler) Process(ctx context.Context, req *streamer.Request) (*streamer.Result, error) {
	return nil, errors.New("use ProcessWithProgress for async handlers")
}
