package functions

import (
	"context"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"cloud.google.com/go/storage"
)

type PubSubMessage struct {
	Data []byte `json:"data"`
}

func SchedulerPubSubFunction(ctx context.Context, m PubSubMessage) error {
	payload := string(m.Data)
	log.Printf("Payload: %s", payload)
	createStorageFile("apollo-public-bucket", "testObject.txt", payload)
	return nil
}

func createStorageFile(bucket, objectName, contents string) error {
	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("storage.NewClient: %v", err)
	}
	defer client.Close()
	ctx, cancel := context.WithTimeout(ctx, time.Second*50)
	defer cancel()
	wc := client.Bucket(bucket).Object(objectName).NewWriter(ctx)
	r := strings.NewReader(contents)
	if _, err = io.Copy(wc, r); err != nil {
		return fmt.Errorf("io.Copy: %v", err)
	}
	if err := wc.Close(); err != nil {
		return fmt.Errorf("Writer.Close: %v", err)
	}
	return nil
}

