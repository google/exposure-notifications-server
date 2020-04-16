// Package api defines the structures for the infection publishing API.
package api

import (
	"net/http"

	"cambio/pkg/database"
	"cambio/pkg/encryption"
	"cambio/pkg/logging"
	"cambio/pkg/storage"
)

func HandleGenerateBatch() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := logging.FromContext(ctx)

		infections, err := database.GetInfections(ctx)
		if err != nil {
			logger.Errorf("error getting infections: %v", err)
			// TODO(guray): cloud scheduler able to retry on non-200s?
			http.Error(w, "internal processing error", http.StatusInternalServerError)
			return
		}

		logger.Infof("received infections")
		encryption.DecryptDiagnosisKeys(ctx, infections)
		// TODO(guray): adjust final format (see tools/fake_client_data.go)
		body := "diagnosisKey\n"
		for _, infection := range infections {
			body = body + string(infection.DiagnosisKey) + "\n"
		}
		// TODO(guray): sort out naming scheme, cache control, etc
		err = storage.CreateObject("apollo-public-bucket", "testBatch.txt", body)
		if err != nil {
			logger.Errorf("error creating cloud storage object: %v", err)
			http.Error(w, "internal processing error", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}
