package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockagentruntime"
	bedrocktypes "github.com/aws/aws-sdk-go-v2/service/bedrockagentruntime/types"
	"github.com/pay-theory/streamer/pkg/streamer"
)

type KnowledgeBaseHandler struct {
	bedrockClient   *bedrockagentruntime.Client
	payTheoryKBID   string
	commerceHubKBID string
}

func NewKnowledgeBaseHandler() *KnowledgeBaseHandler {
	// Initialize AWS config
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		panic(fmt.Sprintf("Failed to load AWS config: %v", err))
	}

	return &KnowledgeBaseHandler{
		bedrockClient:   bedrockagentruntime.NewFromConfig(cfg),
		payTheoryKBID:   "L7CDRZEDIR",
		commerceHubKBID: "CK3EOAVB7Z",
	}
}

func (h *KnowledgeBaseHandler) EstimatedDuration() time.Duration {
	return 15 * time.Second // Will trigger async processing
}

func (h *KnowledgeBaseHandler) Validate(req *streamer.Request) error {
	if req.Payload == nil {
		return fmt.Errorf("payload is required")
	}

	var params KnowledgeBaseParams
	if err := json.Unmarshal(req.Payload, &params); err != nil {
		return fmt.Errorf("invalid payload format: %w", err)
	}

	if params.Query == "" {
		return fmt.Errorf("query is required")
	}

	if params.KnowledgeBase != "" && params.KnowledgeBase != "paytheory" && params.KnowledgeBase != "commercehub" {
		return fmt.Errorf("invalid knowledge_base: must be 'paytheory' or 'commercehub'")
	}

	return nil
}

func (h *KnowledgeBaseHandler) Process(ctx context.Context, req *streamer.Request) (*streamer.Result, error) {
	return nil, fmt.Errorf("use ProcessWithProgress for async handlers")
}

func (h *KnowledgeBaseHandler) ProcessWithProgress(
	ctx context.Context,
	req *streamer.Request,
	reporter streamer.ProgressReporter,
) (*streamer.Result, error) {

	var params KnowledgeBaseParams
	if err := json.Unmarshal(req.Payload, &params); err != nil {
		return nil, fmt.Errorf("failed to parse payload: %w", err)
	}

	// Default to PayTheory KB if not specified
	if params.KnowledgeBase == "" {
		params.KnowledgeBase = "paytheory"
	}

	// Select appropriate KB ID
	var kbID string
	switch params.KnowledgeBase {
	case "paytheory":
		kbID = h.payTheoryKBID
	case "commercehub":
		kbID = h.commerceHubKBID
	default:
		return nil, fmt.Errorf("unknown knowledge base: %s", params.KnowledgeBase)
	}

	// Set metadata
	reporter.SetMetadata("knowledge_base", params.KnowledgeBase)
	reporter.SetMetadata("query_length", fmt.Sprintf("%d", len(params.Query)))

	// Step 1: Initialize
	reporter.Report(10, fmt.Sprintf("Querying %s knowledge base", params.KnowledgeBase))

	// Step 2: Execute retrieve query
	reporter.Report(30, "Executing knowledge base query...")

	results, err := h.executeRetrieve(ctx, kbID, params, reporter)
	if err != nil {
		return nil, fmt.Errorf("failed to query knowledge base: %w", err)
	}

	// Step 3: Process results
	reporter.Report(80, "Processing query results...")

	processedResults := h.processResults(results, params)

	reporter.Report(100, "Knowledge base query complete!")

	return &streamer.Result{
		RequestID: req.ID,
		Success:   true,
		Data: map[string]interface{}{
			"query":          params.Query,
			"knowledge_base": params.KnowledgeBase,
			"documents":      processedResults,
			"result_count":   len(processedResults),
			"timestamp":      time.Now().Format(time.RFC3339),
			"action":         "knowledge_query",
		},
		Metadata: map[string]string{
			"knowledge_base": params.KnowledgeBase,
			"query_type":     "retrieve",
		},
	}, nil
}

func (h *KnowledgeBaseHandler) executeRetrieve(ctx context.Context, kbID string, params KnowledgeBaseParams, reporter streamer.ProgressReporter) ([]bedrocktypes.KnowledgeBaseRetrievalResult, error) {
	maxResults := int32(5)
	if params.MaxResults > 0 {
		maxResults = params.MaxResults
	}

	input := &bedrockagentruntime.RetrieveInput{
		KnowledgeBaseId: aws.String(kbID),
		RetrievalQuery: &bedrocktypes.KnowledgeBaseQuery{
			Text: aws.String(params.Query),
		},
		RetrievalConfiguration: &bedrocktypes.KnowledgeBaseRetrievalConfiguration{
			VectorSearchConfiguration: &bedrocktypes.KnowledgeBaseVectorSearchConfiguration{
				NumberOfResults: aws.Int32(maxResults),
			},
		},
	}

	reporter.Report(50, "Calling Bedrock Retrieve API...")

	output, err := h.bedrockClient.Retrieve(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("bedrock retrieve failed: %w", err)
	}

	reporter.Report(70, fmt.Sprintf("Retrieved %d documents", len(output.RetrievalResults)))

	return output.RetrievalResults, nil
}

func (h *KnowledgeBaseHandler) processResults(results []bedrocktypes.KnowledgeBaseRetrievalResult, params KnowledgeBaseParams) []map[string]interface{} {
	documents := make([]map[string]interface{}, 0, len(results))

	for _, result := range results {
		doc := map[string]interface{}{
			"score": 0.0,
			"text":  "",
		}

		if result.Score != nil {
			doc["score"] = *result.Score
		}

		if result.Content != nil && result.Content.Text != nil {
			doc["text"] = aws.ToString(result.Content.Text)
		}

		if result.Location != nil && result.Location.S3Location != nil {
			doc["source"] = aws.ToString(result.Location.S3Location.Uri)
		}

		// Apply confidence filtering if specified
		if params.MinConfidence > 0 {
			if score, ok := doc["score"].(float64); ok && score >= params.MinConfidence {
				documents = append(documents, doc)
			}
		} else {
			documents = append(documents, doc)
		}
	}

	return documents
}

// Types
type KnowledgeBaseParams struct {
	Query         string  `json:"query"`
	KnowledgeBase string  `json:"knowledge_base,omitempty"`
	MaxResults    int32   `json:"max_results,omitempty"`
	MinConfidence float64 `json:"min_confidence,omitempty"`
}