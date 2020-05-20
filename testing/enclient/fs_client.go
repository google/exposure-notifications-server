package enclient

import (
	"context"
	"fmt"
	"sort"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
)

type CloudStorage struct {
	client *storage.Client
}

func NewCloudStorage(ctx context.Context) (*CloudStorage, error) {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("storage.NewClient: %w", err)
	}
	return &CloudStorage{client}, nil
}

func (gcs *CloudStorage) ListBucket(ctx context.Context, bucket string, filter func(attr *storage.ObjectAttrs) bool) []storage.ObjectAttrs {
	query := &storage.Query{
		Prefix: "exposureKeyExport-US/",
	}
	it := gcs.client.Bucket(bucket).Objects(ctx, query)

	result := make([]storage.ObjectAttrs, 0)
	for {
		objAttrs, err := it.Next()
		if err == iterator.Done {
			break
		}

		if objAttrs != nil && filter(objAttrs) {
			result = append(result, *objAttrs)
		}
	}

	sort.SliceStable(result, func(i, j int) bool {
		return result[j].Created.Before(result[i].Created)
	})

	return result
}
