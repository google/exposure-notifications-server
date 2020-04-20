// Package api defines the structures for the infection publishing API.
package api

import (
	"net/http"

	"cambio/pkg/database"
	"cambio/pkg/logging"
	"cambio/pkg/pb"
	"cambio/pkg/storage"
	"time"

	"github.com/golang/protobuf/proto"
)

func HandleExport() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := logging.FromContext(ctx)

		// TODO(guray): determine work required, split into batches, store state, etc
		infections, err := database.GetInfections(ctx)
		if err != nil {
			logger.Errorf("error getting infections: %v", err)
			http.Error(w, "internal processing error", http.StatusInternalServerError)
			return
		}

		logger.Infof("received infections")
		diagnosisKeys := make([]*pb.DiagnosisKeyExport_DiagnosisKey, 0, 20)
		for _, infection := range infections {
			diagnosisKey := pb.DiagnosisKeyExport_DiagnosisKey{
				DiagnosisKey:   infection.DiagnosisKey,
				IntervalNumber: infection.IntervalNumber,
				IntervalCount:  infection.IntervalCount,
			}
			diagnosisKeys = append(diagnosisKeys, &diagnosisKey)
		}
		batch := pb.DiagnosisKeyExport{
			// TODO(guray): real metadata, depending on what batch this is
			StartTimestamp: time.Now().Unix(),
			Region:         "US",
			Keys:           diagnosisKeys,
		}
		data, err := proto.Marshal(&batch)
		if err != nil {
			logger.Errorf("error serializing proto: %v", err)
			http.Error(w, "internal processing error", http.StatusInternalServerError)
		}
		// TODO(guray): sort out naming scheme, cache control, etc
		if err := storage.CreateObject("apollo-public-bucket", "testExport.pb", data); err != nil {
			logger.Errorf("error creating cloud storage object: %v", err)
			http.Error(w, "internal processing error", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}
