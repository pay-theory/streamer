package dynamorm

import (
	"github.com/pay-theory/streamer/pkg/models"
)

// Type aliases for backward compatibility
type Connection = models.Connection
type AsyncRequest = models.AsyncRequest
type Subscription = models.Subscription
type RequestStatus = models.RequestStatus

// Status constants
const (
	StatusPending    = models.StatusPending
	StatusProcessing = models.StatusProcessing
	StatusCompleted  = models.StatusCompleted
	StatusFailed     = models.StatusFailed
	StatusCancelled  = models.StatusCancelled
	StatusRetrying   = models.StatusRetrying
)