package txsub

import (
	"context"
	"net/http"
	"time"

	"github.com/stellar/go/clients/stellarcore"
	proto "github.com/stellar/go/protocols/stellarcore"
	"github.com/stellar/go/support/errors"
)

// NewDefaultSubmitter returns a new, simple Submitter implementation
// that submits directly to the stellar-core at `url` using the http client
// `h`.
func NewDefaultSubmitter(h *http.Client, url string) Submitter {
	return &submitter{
		StellarCore: &stellarcore.Client{
			HTTP: h,
			URL:  url,
		},
	}
}

// submitter is the default implementation for the Submitter interface.  It
// submits directly to the configured stellar-core instance using the
// configured http client.
type submitter struct {
	StellarCore *stellarcore.Client
}

// Submit sends the provided envelope to stellar-core and parses the response into
// a SubmissionResult
func (sub *submitter) Submit(ctx context.Context, env string) (result SubmissionResult) {
	start := time.Now()
	defer func() { result.Duration = time.Since(start) }()

	cresp, err := sub.StellarCore.SubmitTransaction(ctx, env)
	if err != nil {
		result.Err = errors.Wrap(err, "failed to submit")
		return
	}

	// interpet response
	if cresp.IsException() {
		result.Err = errors.Errorf("stellar-core exception: %s", cresp.Exception)
		return
	}

	switch cresp.Status {
	case proto.TXStatusError:
		result.Err = &FailedTransactionError{cresp.Error}
	case proto.TXStatusPending, proto.TXStatusDuplicate:
		//noop.  A nil Err indicates success
	default:
		result.Err = errors.Errorf("Unrecognized stellar-core status response: %s", cresp.Status)
	}

	return
}
