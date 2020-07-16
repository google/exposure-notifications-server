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

// Package database is a database interface to publish.
package database

import (
	"context"
	"database/sql"
	"encoding/base64"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/logging"
	"github.com/google/exposure-notifications-server/internal/publish/model"
	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1alpha1"
	"github.com/google/exposure-notifications-server/pkg/base64util"

	pgx "github.com/jackc/pgx/v4"
)

const (
	// InsertExposuresBatchSize is the maximum number of exposures that can be inserted at once.
	InsertExposuresBatchSize = 500
)

type PublishDB struct {
	db *database.DB
}

func New(db *database.DB) *PublishDB {
	return &PublishDB{
		db: db,
	}
}

// IterateExposuresCriteria is criteria to iterate exposures.
type IterateExposuresCriteria struct {
	IncludeRegions []string
	ExcludeRegions []string
	SinceTimestamp time.Time
	UntilTimestamp time.Time
	LastCursor     string
	RevisedKeys    bool // If true, only revised keys that match will be selected.

	// OnlyLocalProvenance indicates that only exposures with LocalProvenance=true will be returned.
	OnlyLocalProvenance bool
}

// IterateExposures calls f on each Exposure in the database that matches the
// given criteria. If f returns an error, the iteration stops, and the returned
// error will match f's error with errors.Is.
//
// If an error occurs during the query, IterateExposures will return a non-empty
// string along with a non-nil error. That string, when passed as
// criteria.LastCursor in a subsequent call to IterateExposures, will continue
// the iteration at the failed row. If IterateExposures returns a nil error,
// the first return value will be the empty string.
func (db *PublishDB) IterateExposures(ctx context.Context, criteria IterateExposuresCriteria, f func(*model.Exposure) error) (cur string, err error) {
	conn, err := db.db.Pool.Acquire(ctx)
	if err != nil {
		return "", fmt.Errorf("acquiring connection: %v", err)
	}
	defer conn.Release()
	offset := 0
	if criteria.LastCursor != "" {
		offsetStr, err := decodeCursor(criteria.LastCursor)
		if err != nil {
			return "", fmt.Errorf("decoding cursor: %v", err)
		}
		offset, err = strconv.Atoi(offsetStr)
		if err != nil {
			return "", fmt.Errorf("decoding cursor %v", err)
		}
	}

	query, args, err := generateExposureQuery(criteria)
	if err != nil {
		return "", fmt.Errorf("generating where: %v", err)
	}
	logging.FromContext(ctx).Debugf("Query: %s", query)
	logging.FromContext(ctx).Debugf("Args: %v", args)

	// TODO: this is a pretty weak cursor solution, but not too bad since we'll
	// typically have queries ahead of the cleanup and before the current
	// ingestion window, and those should be stable.
	cursor := func() string { return encodeCursor(strconv.Itoa(offset)) }

	rows, err := conn.Query(ctx, query, args...)
	if err != nil {
		return cursor(), err
	}
	defer rows.Close()
	for rows.Next() {
		if err := ctx.Err(); err != nil {
			return cursor(), err
		}
		var (
			m          model.Exposure
			encodedKey string
			syncID     *int64
		)
		if err := rows.Scan(&encodedKey, &m.TransmissionRisk, &m.AppPackageName, &m.Regions, &m.IntervalNumber,
			&m.IntervalCount, &m.CreatedAt, &m.LocalProvenance, &syncID, &m.HealthAuthorityID, &m.ReportType,
			&m.DaysSinceSymptomOnset, &m.RevisedReportType, &m.RevisedAt, &m.RevisedDaysSinceSymptomOnset); err != nil {
			return cursor(), err
		}
		var err error
		m.ExposureKey, err = decodeExposureKey(encodedKey)
		if err != nil {
			return cursor(), err
		}
		if syncID != nil {
			m.FederationSyncID = *syncID
		}
		if err := f(&m); err != nil {
			return cursor(), err
		}
		offset++
	}
	if err := rows.Err(); err != nil {
		return cursor(), err
	}
	return "", nil
}

func generateExposureQuery(criteria IterateExposuresCriteria) (string, []interface{}, error) {
	var args []interface{}
	q := `
		SELECT
			exposure_key, transmission_risk, LOWER(app_package_name), regions, interval_number, interval_count,
			created_at, local_provenance, sync_id, health_authority_id, report_type,
			days_since_symptom_onset, revised_report_type, revised_at, revised_days_since_symptom_onset
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

	timeField := "created_at"
	if criteria.RevisedKeys {
		q += " AND revised_at IS NOT NULL"
		timeField = "revised_at"
	}

	// It is important for StartTimestamp to be inclusive (as opposed to exclusive). When the exposure keys are
	// published, they are truncated to a time boundary (e.g., time.Hour). Even though the exposure keys might arrive
	// during a current open export batch window, the exposure keys are truncated to the start of that window,
	// which would make them fall into the _previous_ (already processed) batch if StartTimestamp is exclusive
	// (in the case where the publish window and the export period align).
	if !criteria.SinceTimestamp.IsZero() {
		args = append(args, criteria.SinceTimestamp)
		q += fmt.Sprintf(" AND %s >= $%d", timeField, len(args))
	}

	if !criteria.UntilTimestamp.IsZero() {
		args = append(args, criteria.UntilTimestamp)
		q += fmt.Sprintf(" AND %s < $%d", timeField, len(args))
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

// ReadExposures will read an existing set of exposures from the database.
// This is necessary in case a key needs to be revised.
// In the return map, the key is the base64 of the ExposureKey.
// The keys are read for update in a provided transaction.
func (db *PublishDB) ReadExposures(ctx context.Context, tx pgx.Tx, b64keys []string) (map[string]*model.Exposure, error) {
	query := `
		SELECT
			exposure_key, transmission_risk, app_package_name, regions,
			interval_number, interval_count, created_at, local_provenance, sync_id,
			health_authority_id, report_type, days_since_symptom_onset,
			revised_report_type, revised_at, revised_days_since_symptom_onset,
			revised_transmission_risk
		FROM
			Exposure
		WHERE exposure_key = ANY($1)
		FOR UPDATE`
	rows, err := tx.Query(ctx, query, b64keys)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]*model.Exposure)
	for rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("iterating rows: %w", err)
		}

		var encodedKey string
		var syncID sql.NullInt64

		var exposure model.Exposure
		if err := rows.Scan(
			&encodedKey, &exposure.TransmissionRisk, &exposure.AppPackageName,
			&exposure.Regions, &exposure.IntervalNumber, &exposure.IntervalCount,
			&exposure.CreatedAt, &exposure.LocalProvenance, &syncID,
			&exposure.HealthAuthorityID, &exposure.ReportType, &exposure.DaysSinceSymptomOnset,
			&exposure.RevisedReportType, &exposure.RevisedAt, &exposure.RevisedDaysSinceSymptomOnset,
			&exposure.RevisedTransmissionRisk,
		); err != nil {
			return nil, err
		}

		// Base64 decode the exposure key
		exposure.ExposureKey, err = decodeExposureKey(encodedKey)
		if err != nil {
			return nil, err
		}
		// Optionally set all of the nullable columns.
		if syncID.Valid {
			exposure.FederationSyncID = syncID.Int64
		}

		result[exposure.ExposureKeyBase64()] = &exposure
	}

	return result, nil
}

func prepareInsertExposure(ctx context.Context, tx pgx.Tx) (string, error) {
	const stmtName = "insert exposures"
	_, err := tx.Prepare(ctx, stmtName, `
		INSERT INTO
			Exposure
				(exposure_key, transmission_risk, app_package_name, regions, interval_number, interval_count,
				 created_at, local_provenance, sync_id, health_authority_id, report_type, days_since_symptom_onset)
		VALUES
			($1, $2, LOWER($3), $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (exposure_key) DO NOTHING
	`)
	return stmtName, err
}

func executeInsertExposure(ctx context.Context, tx pgx.Tx, stmtName string, exp *model.Exposure) error {
	var syncID *int64
	if exp.FederationSyncID != 0 {
		syncID = &exp.FederationSyncID
	}
	_, err := tx.Exec(ctx, stmtName, encodeExposureKey(exp.ExposureKey), exp.TransmissionRisk,
		exp.AppPackageName, exp.Regions, exp.IntervalNumber, exp.IntervalCount,
		exp.CreatedAt, exp.LocalProvenance, syncID,
		exp.HealthAuthorityID, exp.ReportType, exp.DaysSinceSymptomOnset)
	if err != nil {
		return fmt.Errorf("inserting exposure: %v", err)
	}
	return nil
}

func prepareReviseExposure(ctx context.Context, tx pgx.Tx) (string, error) {
	const stmtName = "update exposures"
	_, err := tx.Prepare(ctx, stmtName, `
		UPDATE
			Exposure
		SET
			health_authority_id = $1, revised_report_type = $2, revised_at = $3,
			revised_days_since_symptom_onset = $4, revised_transmission_risk = $5
		WHERE
			exposure_key = $6 AND revised_at IS NULL
		`)
	return stmtName, err
}

func executeReviseExposure(ctx context.Context, tx pgx.Tx, stmtName string, exp *model.Exposure) error {
	result, err := tx.Exec(ctx, stmtName,
		exp.HealthAuthorityID, exp.RevisedReportType, exp.RevisedAt,
		exp.RevisedDaysSinceSymptomOnset, exp.RevisedTransmissionRisk,
		encodeExposureKey(exp.ExposureKey))
	if err != nil {
		return fmt.Errorf("revising exposure: %v", err)
	}
	if result.RowsAffected() != 1 {
		return fmt.Errorf("invalid key revision request")
	}
	return nil
}

// InsertAndReviseExposures transactionally revises and inserts a set of keys as necessary.
func (db *PublishDB) InsertAndReviseExposures(ctx context.Context, incoming []*model.Exposure) (int, error) {
	updated := 0
	err := db.db.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
		log.Printf("\n\nincoming: %#v\n\n", incoming)

		// Read any existing TEKs FOR UPDATE in this transaction.
		b64keys := make([]string, len(incoming))
		for i, e := range incoming {
			b64keys[i] = e.ExposureKeyBase64()
		}

		log.Printf("\n\nb64keys: %#v\n\n", b64keys)

		existing, err := db.ReadExposures(ctx, tx, b64keys)
		if err != nil {
			return fmt.Errorf("unable to check for existing records")
		}

		log.Printf("\n\nexisting: %#v\n\n", existing)

		// Run through the merge logic.
		exposures, err := model.ReviseKeys(ctx, existing, incoming)
		if err != nil {
			return fmt.Errorf("unable to revise keys: %w", err)
		}

		log.Printf("\n\nexposures: %#v\n\n", exposures)

		// Prepare the insert and update statements.
		insertStmt, err := prepareInsertExposure(ctx, tx)
		if err != nil {
			return fmt.Errorf("preparing insert statement: %v", err)
		}
		updateStmt, err := prepareReviseExposure(ctx, tx)
		if err != nil {
			return fmt.Errorf("preparing update statement: %v", err)
		}

		log.Printf("\n\ninsertStmt: %v\n\n", insertStmt)
		log.Printf("\n\nupdateStmt: %v\n\n", updateStmt)

		for _, exp := range exposures {
			if exp.RevisedAt == nil {
				log.Printf("\n\nrevistedAt nil\n\n")
				if exp.ReportType == verifyapi.ReportTypeNegative {
					continue
				}
				if err := executeInsertExposure(ctx, tx, insertStmt, exp); err != nil {
					return err
				}
				updated++
			} else {
				log.Printf("\n\nrevistedAt not nil\n\n")
				if err := executeReviseExposure(ctx, tx, updateStmt, exp); err != nil {
					return err
				}
				updated++
			}
		}
		return nil
	})
	if err != nil {
		updated = 0
	}
	return updated, err
}

// DeleteExposure deletes exposure
func (db *PublishDB) DeleteExposure(ctx context.Context, exposureKey []byte) (int64, error) {
	var count int64
	// ReadCommitted is sufficient here because we are dealing with historical, immutable rows.
	err := db.db.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
		result, err := tx.Exec(ctx, `
			DELETE FROM
				Exposure
			WHERE
				exposure_key = $1
			`, encodeExposureKey(exposureKey))
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

// DeleteExposuresBefore deletes exposures created before "before" date. Returns the number of records deleted.
func (db *PublishDB) DeleteExposuresBefore(ctx context.Context, before time.Time) (int64, error) {
	var count int64
	// ReadCommitted is sufficient here because we are dealing with historical, immutable rows.
	err := db.db.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
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
	b, err := base64util.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("decoding cursor: %v", err)
	}
	return string(b), nil
}

func encodeExposureKey(b []byte) string {
	return base64.StdEncoding.EncodeToString(b)
}

func decodeExposureKey(encoded string) ([]byte, error) {
	return base64util.DecodeString(encoded)
}
