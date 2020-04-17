package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"cambio/pkg/database"
	"cambio/pkg/encryption"
	"cambio/pkg/logging"
	"cambio/pkg/model"

	"cloud.google.com/go/datastore"
)

// Scans the database and writes descrypted diagnosis keys to the terminal.
// This is to verify the crypto operations.
func main() {
	var numKeys = flag.Int("num", 1, "number of keys to scan -num=1")
	flag.Parse()

	ctx := context.Background()
	logger := logging.FromContext(ctx)

	if err := database.Initialize(); err != nil {
		logger.Fatalf("database.Initialize: %v", err)
	}
	if err := encryption.InitDiagnosisKeys(); err != nil {
		logger.Fatalf("encryption.InitDiagnosisKeys: %v", err)
	}

	client := database.Connection()
	if client == nil {
		logger.Fatalf("database.Connection error")
	}

	var infections []model.Infection
	q := datastore.NewQuery("infection").Order("- createdAt").Limit(*numKeys)
	if _, err := client.GetAll(ctx, q, &infections); err != nil {
		logger.Fatalf("unable to query datbase: %v", err)
	}

	err := encryption.DecryptDiagnosisKeys(ctx, infections)
	if err != nil {
		logger.Fatalf("encryption.DecryptDiagnosisKeys: %v", err)
	}

	var stdout = os.Stdout
	for _, inf := range infections {
		fmt.Fprintf(stdout, "%v | %v\n", inf.K, inf.DiagnosisKey)
	}
}
