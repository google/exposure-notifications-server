package verification

import (
	"cambio/pkg/logging"
	"context"
)

func VerifySafetyNet(ctx context.Context) error {
	logger := logging.FromContext(ctx)
	logger.Error("SAFETYNET VERIFICATION NOT IMPLEMENTED.")
	// TODO - implement the verification of the payload
	return nil
}
