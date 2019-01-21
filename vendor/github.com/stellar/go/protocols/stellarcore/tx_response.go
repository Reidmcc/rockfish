package stellarcore

const (
	// TXStatusError represents the status value returned by stellar-core when an error occurred from
	// submitting a transaction
	TXStatusError = "ERROR"

	// TXStatusPending represents the status value returned by stellar-core when a transaction has
	// been accepted for processing
	TXStatusPending = "PENDING"

	// TXStatusDuplicate represents the status value returned by stellar-core when a submitted
	// transaction is a duplicate
	TXStatusDuplicate = "DUPLICATE"
)

// TXResponse represents the response returned from a submission request sent to stellar-core's /tx
// endpoint
type TXResponse struct {
	Exception string `json:"exception"`
	Error     string `json:"error"`
	Status    string `json:"status"`
}

// IsException returns true if the response represents an exception response from stellar-core
func (resp *TXResponse) IsException() bool {
	return resp.Exception != ""
}
