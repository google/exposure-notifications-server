// Copyright 2020 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package database

import (
	"context"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/exposure-notifications-server/internal/model"

	pgx "github.com/jackc/pgx/v4"
)

const (
	// InsertExposuresBatchSize is the maximum number of exposures that can be inserted at once.
	InsertExposuresBatchSize = 500
)

// ExposureIterator iterates over a set of exposures.
type ExposureIterator interface {
	// Next returns an exposure and a flag indicating if the iterator is done (the exposure will be nil when done==true).
	Next() (exposure *model.Exposure, done bool, err error)
	// Cursor returns a string that can be passed as LastCursor in FetchExposuresCriteria when generating another iterator.
	Cursor() (string, error)
	// Close should be called when done iterating.
	Close() error
}

// IterateExposuresCriteria is criteria to iterate exposures.
type IterateExposuresCriteria struct {
	IncludeRegions []string
	ExcludeRegions []string
	SinceTimestamp time.Time
	UntilTimestamp time.Time
	LastCursor     string

	// OnlyLocalProvenance indicates that only exposures with LocalProvenance=true will be returned.
	OnlyLocalProvenance bool
}

// IterateExposures returns an iterator for exposures meeting the criteria. Must call iterator's Close() method when done.
func (db *DB) IterateExposures(ctx context.Context, criteria IterateExposuresCriteria) (ExposureIterator, error) {
	conn, err := db.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquiring connection: %v", err)
	}
	// We don't defer Release() here because the iterator's Close() method will do it.

	offset := 0
	if criteria.LastCursor != "" {
		offsetStr, err := decodeCursor(criteria.LastCursor)
		if err != nil {
			return nil, fmt.Errorf("decoding cursor: %v", err)
		}
		offset, err = strconv.Atoi(offsetStr)
		if err != nil {
			return nil, fmt.Errorf("decoding cursor %v", err)
		}
	}

	query, args, err := generateExposureQuery(criteria)
	if err != nil {
		return nil, fmt.Errorf("generating where: %v", err)
	}

	rows, err := conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	return &postgresExposureIterator{iter: newRowIterator(conn, rows), offset: offset}, nil
}

type postgresExposureIterator struct {
	iter   *rowIterator
	offset int
}

func (i *postgresExposureIterator) Next() (*model.Exposure, bool, error) {
	if done, err := i.iter.next(); done {
		return nil, done, err
	}

	var (
		m          model.Exposure
		encodedKey string
		syncID     *int64
	)
	if err := i.iter.rows.Scan(&encodedKey, &m.TransmissionRisk, &m.AppPackageName, &m.Regions, &m.IntervalNumber,
		&m.IntervalCount, &m.CreatedAt, &m.LocalProvenance, &m.VerificationAuthorityName, &syncID); err != nil {
		return nil, false, err
	}

	var err error
	m.ExposureKey, err = decodeExposureKey(encodedKey)
	if err != nil {
		return nil, false, err
	}
	if syncID != nil {
		m.FederationSyncID = *syncID
	}
	i.offset++

	return &m, false, nil
}

func (i *postgresExposureIterator) Cursor() (string, error) {
	// TODO: this is a pretty weak cursor solution, but not too bad since we'll typcially have queries ahead of the wipeout
	// and before the current ingestion window, and those should be stable.
	return encodeCursor(strconv.Itoa(i.offset)), nil
}

func (i *postgresExposureIterator) Close() error {
	return i.iter.close()
}

func generateExposureQuery(criteria IterateExposuresCriteria) (string, []interface{}, error) {
	var args []interface{}
	q := `
		SELECT
			exposure_key, transmission_risk, app_package_name, regions, interval_number, interval_count,
			created_at, local_provenance, verification_authority_name, sync_id
		FROM
			Exposure
		WHERE 1=1
	`

	if len(criteria.IncludeRegions) == 1 {
		args = append(args, criteria.IncludeRegions)
		q += fmt.Sprintf(" AND (regions && $%d)", len(args)) // Operation "&&" means "array overlaps / intersects"
	}

	if len(criteria.ExcludeRegions) == 1 {
		args = append(args, criteria.ExcludeRegions)
		q += fmt.Sprintf(" AND NOT (regions && $%d)", len(args)) // Operation "&&" means "array overlaps / intersects"
	}

	if !criteria.SinceTimestamp.IsZero() {
		args = append(args, criteria.SinceTimestamp)
		q += fmt.Sprintf(" AND created_at > $%d", len(args))
	}

	if !criteria.UntilTimestamp.IsZero() {
		args = append(args, criteria.UntilTimestamp)
		q += fmt.Sprintf(" AND created_at <= $%d", len(args))
	}

	if criteria.OnlyLocalProvenance {
		args = append(args, true)
		q += fmt.Sprintf(" AND local_provenance = $%d", len(args))
	}

	q += " ORDER BY created_at"

	if criteria.LastCursor != "" {
		decoded, err := decodeCursor(criteria.LastCursor)
		if err != nil {
			return "", nil, err
		}
		args = append(args, decoded)
		q += fmt.Sprintf(" OFFSET $%d", len(args))
	}
	q = strings.ReplaceAll(q, "\n", " ")

	return q, args, nil
}

// InsertExposures inserts a set of exposures.
func (db *DB) InsertExposures(ctx context.Context, exposures []*model.Exposure) error {
	return db.inTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
		const stmtName = "insert exposures"
		_, err := tx.Prepare(ctx, stmtName, `
			INSERT INTO
				Exposure
			    (exposure_key, transmission_risk, app_package_name, regions, interval_number, interval_count,
			     created_at, local_provenance, verification_authority_name, sync_id)
			VALUES
			  ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			ON CONFLICT (exposure_key) DO NOTHING
		`)
		if err != nil {
			return fmt.Errorf("preparing insert statment: %v", err)
		}

		for _, inf := range exposures {
			var syncID *int64
			if inf.FederationSyncID != 0 {
				syncID = &inf.FederationSyncID
			}
			_, err := tx.Exec(ctx, stmtName, encodeExposureKey(inf.ExposureKey), inf.TransmissionRisk, inf.AppPackageName, inf.Regions, inf.IntervalNumber, inf.IntervalCount,
				inf.CreatedAt, inf.LocalProvenance, inf.VerificationAuthorityName, syncID)
			if err != nil {
				return fmt.Errorf("inserting exposure: %v", err)
			}
		}
		return nil
	})
}

// DeleteExposures deletes exposures created before "before" date. Returns the number of records deleted.
func (db *DB) DeleteExposures(ctx context.Context, before time.Time) (int64, error) {
	var count int64
	// ReadCommitted is sufficient here because we are dealing with historical, immutable rows.
	err := db.inTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
		result, err := tx.Exec(ctx, `
			DELETE FROM
				Exposure
			WHERE
				created_at < $1
			`, before)
		if err != nil {
			return fmt.Errorf("deleting exposures: %v", err)
		}
		count = result.RowsAffected()
		return nil
	})
	if err != nil {
		return 0, err
	}
	return count, nil
}

func encodeCursor(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}

func decodeCursor(encoded string) (string, error) {
	b, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("decoding cursor: %v", err)
	}
	return string(b), nil
}

func encodeExposureKey(b []byte) string {
	return base64.StdEncoding.EncodeToString(b)
}

func decodeExposureKey(encoded string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(encoded)
}
