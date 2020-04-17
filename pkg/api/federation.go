package api

import (
	"cambio/pkg/database"
	"cambio/pkg/logging"
	"cambio/pkg/pb"
	"context"
	"fmt"
	"time"
)

type diagKeyList []*pb.DiagnosisKey
type diagKeys map[pb.DiagnosisStatus]diagKeyList
type collator map[string]diagKeys
type fetchIterator func(context.Context, database.FetchInfectionsCriteria) (database.InfectionIterator, error)

// NewFederationServer builds a new FederationServer.
func NewFederationServer(timeout time.Duration) pb.FederationServer {
	return &federationServer{timeout: timeout}
}

type federationServer struct {
	timeout time.Duration
}

// Fetch implements the FederationServer Fetch endpoint.
func (s *federationServer) Fetch(ctx context.Context, req *pb.FederationFetchRequest) (*pb.FederationFetchResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	return s.fetch(ctx, req, database.IterateInfections)
}

func (s *federationServer) fetch(ctx context.Context, req *pb.FederationFetchRequest, itFunc fetchIterator) (*pb.FederationFetchResponse, error) {
	logger := logging.FromContext(ctx)

	criteria := database.FetchInfectionsCriteria{
		IncludeRegions: req.RegionIdentifiers,
		SinceTimestamp: time.Unix(req.LastFetchResponseKeyTimestamp, 0),
		// UntilTimestamp: time.Time{}, // TODO(jasonco): do we limit how recent the data can be? Possibly at least the size of the windowing.
		LastCursor: req.FetchToken,
	}

	// Filter excluded countries in memory, using a map for efficiency.
	excluded := map[string]struct{}{}
	for _, region := range req.ExcludeRegionIdentifiers {
		excluded[region] = struct{}{}
	}

	it, err := itFunc(ctx, criteria)
	if err != nil {
		return nil, fmt.Errorf("querying infections (criteria: %#v): %v", criteria, err)
	}

	c := collator{}
	var maxTimestamp int64

	for {
		select {
		case <-ctx.Done():
			// Err() may be context.Canceled due to test code.
			if err := ctx.Err(); err != context.DeadlineExceeded && err != context.Canceled {
				return nil, fmt.Errorf("context error: %v", err)
			}
			logger.Infof("Fetch request reached time out, returning partial response.")
			cursor, err := it.Cursor()
			if err != nil {
				return nil, fmt.Errorf("generating cursor: %v", err)
			}
			return &pb.FederationFetchResponse{
				Response:        convertResponse(c),
				PartialResponse: true,
				// TODO(jasonco): Should this value be included for a partial response? (I.e., a partial response might truncate a window,
				// so starting a query with the timestamp (but not the NextFetchToken) will likely miss the end of that truncated window.)
				FetchResponseKeyTimestamp: maxTimestamp,
				NextFetchToken:            cursor,
			}, nil
		default:
		}

		inf, done, err := it.Next()
		if err != nil {
			return nil, fmt.Errorf("iterating results: %v", err)
		}
		if done {
			break
		}
		if inf == nil {
			continue
		}

		// Skip excluded countries.
		if _, isExcluded := excluded[inf.Country]; isExcluded {
			continue
		}

		if _, exists := c[inf.Country]; !exists {
			c[inf.Country] = make(diagKeys)
		}

		// TODO(jasonco): add DiagnosisStatus
		status := pb.DiagnosisStatus_positive_verified
		if _, exists := c[inf.Country][status]; !exists {
			c[inf.Country][status] = nil
		}

		// TODO(jasonco): decrypt key
		k := pb.DiagnosisKey{DiagnosisKey: inf.DiagnosisKey, Timestamp: inf.KeyDay.Unix()}
		c[inf.Country][status] = append(c[inf.Country][status], &k)
		if k.Timestamp > maxTimestamp {
			maxTimestamp = k.Timestamp
		}
	}

	return &pb.FederationFetchResponse{
		Response:                  convertResponse(c),
		FetchResponseKeyTimestamp: maxTimestamp,
	}, nil
}

func convertResponse(c collator) []*pb.ContactTracingResponse {
	r := []*pb.ContactTracingResponse{}
	for region := range c {
		for status := range c[region] {
			keys := c[region][status]
			rec := pb.ContactTracingResponse{
				DiagnosisStatus:  status, // TODO(jasonco) convert?
				RegionIdentifier: region, // TODO(jasonco) convert?
			}
			rec.DiagnosisKeys = append(rec.DiagnosisKeys, keys...)
			r = append(r, &rec)
		}
	}
	return r
}
