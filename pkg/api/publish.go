// Package api defines the structures for the infection publishing API.
package api

import (
	"net/http"
	"time"

	"cambio/pkg/database"
	"cambio/pkg/encryption"
	"cambio/pkg/logging"
	"cambio/pkg/model"
	"cambio/pkg/verification"
)

func HandlePublish() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := logging.FromContext(ctx)

		var data model.Publish
		err, code := unmarshal(w, r, &data)
		if err != nil {
			logger.Errorf("error unmarhsaling API call, code: %v: %v", code, err)
			// Log but don't return internal decode error message reason.
			http.Error(w, "bad API request", code)
			return
		}

		err = verification.VerifySafetyNet(ctx)
		if err != nil {
			logger.Errorf("unable to verify safetynet payload: %v", err)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		batchTime := time.Now()
		infections, err := model.TransformPublish(&data, batchTime)
		if err != nil {
			logger.Errorf("error transforming publish data: %v", err)
			http.Error(w, "bad API request", http.StatusBadRequest)
			return
		}

		infections, err = encryption.EncryptDiagnosisKeys(ctx, infections)
		if err != nil {
			logger.Errorf("error during diagnosis key encryption: %v", err)
			http.Error(w, "internal processing error", http.StatusInternalServerError)
			return
		}

		err = database.InsertInfections(ctx, infections)
		if err != nil {
			logger.Errorf("error writing infection record: %v", err)
			http.Error(w, "internal processing error", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}
