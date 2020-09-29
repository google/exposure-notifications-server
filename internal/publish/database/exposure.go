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
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/pb"
	"github.com/google/exposure-notifications-server/internal/publish/model"
	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1alpha1"
	"github.com/google/exposure-notifications-server/pkg/base64util"
	"github.com/google/exposure-notifications-server/pkg/logging"

	pgx "github.com/jackc/pgx/v4"
)

const (
	// InsertExposuresBatchSize is the maximum number of exposures that can be inserted at once.
	InsertExposuresBatchSize = 500
)

var (
	// ErrExistingKeyNotInToken is returned when attempting to present an exposure that already exists, but
	// isn't in the provided revision token.
	ErrExistingKeyNotInToken = errors.New("sent existing exposure key that is not in revision token")

	// ErrNoRevisionToken is returned when presenting exposures that already exists, but no revision
	// token was presented.
	ErrNoRevisionToken = errors.New("sent existing exposures but no revision token present")

	// ErrRevisionTokenMetadataMismatch is returned when a revision token has the correct TEK in it,
	// but the new request is attempting to change the metadata of the key (intervalNumber/Count)
	ErrRevisionTokenMetadataMismatch = errors.New("changing exposure key metadata is not allowed")

	// ErrIncomingMetadataMismatch is returned when incoming data has a known TEK
	// in it, but the new request is attempting to change the metadata of the key
	// (intervalNumber/Count).
	ErrIncomingMetadataMismatch = errors.New("incoming exposure key metadata does not match expected values")
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
	IncludeRegions   []string
	IncludeTravelers bool // Include records in the IncludeRegions OR travalers
	OnlyTravelers    bool // Only includes records marked as travelers.
	ExcludeRegions   []string
	SinceTimestamp   time.Time
	UntilTimestamp   time.Time
	LastCursor       string
	OnlyRevisedKeys  bool // If true, only revised keys that match will be selected.

	// OnlyLocalProvenance indicates that only exposures with LocalProvenance=true will be returned.
	OnlyLocalProvenance bool

	// If limit is > 0, a limit query will be set on the database query.
	Limit uint32
}

type IteratorFunction func(*model.Exposure) error

// IterateExposures calls f on each Exposure in the database that matches the
// given criteria. If f returns an error, the iteration stops, and the returned
// error will match f's error with errors.Is.
//
// If an error occurs during the query, IterateExposures will return a non-empty
// string along with a non-nil error. That string, when passed as
// criteria.LastCursor in a subsequent call to IterateExposures, will continue
// the iteration at the failed row. If IterateExposures returns a nil error,
// the first return value will be the empty string.
func (db *PublishDB) IterateExposures(ctx context.Context, criteria IterateExposuresCriteria, f IteratorFunction) (cur string, err error) {
	offset := 0
	if criteria.LastCursor != "" {
		offsetStr, err := decodeCursor(criteria.LastCursor)
		if err != nil {
			return "", fmt.Errorf("decoding cursor: %w", err)
		}
		offset, err = strconv.Atoi(offsetStr)
		if err != nil {
			return "", fmt.Errorf("decoding cursor %w", err)
		}
	}

	query, args, err := generateExposureQuery(criteria)
	if err != nil {
		return "", fmt.Errorf("generating where: %v", err)
	}

	logging.FromContext(ctx).Debugw("iterator query", "query", query, "args", args)

	// TODO: this is a pretty weak cursor solution, but not too bad since we'll
	// typically have queries ahead of the cleanup and before the current
	// ingestion window, and those should be stable.
	cursor := func() string { return encodeCursor(strconv.Itoa(offset)) }

	if err := db.db.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, query, args...)
		if err != nil {
			return fmt.Errorf("failed to list: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			if err := rows.Err(); err != nil {
				return fmt.Errorf("failed to iterate: %w", err)
			}

			if err := ctx.Err(); err != nil {
				return err
			}

			var m model.Exposure
			var encodedKey string
			var syncID *int64
			var queryID *string
			if err := rows.Scan(&encodedKey, &m.TransmissionRisk, &m.AppPackageName, &m.Regions, &m.Traveler,
				&m.IntervalNumber, &m.IntervalCount, &m.CreatedAt, &m.LocalProvenance, &syncID, &queryID, &m.HealthAuthorityID,
				&m.ReportType, &m.DaysSinceSymptomOnset, &m.RevisedReportType, &m.RevisedAt, &m.RevisedDaysSinceSymptomOnset); err != nil {
				return fmt.Errorf("failed to parse: %w", err)
			}

			var err error
			m.ExposureKey, err = decodeExposureKey(encodedKey)
			if err != nil {
				return fmt.Errorf("failed to decode key: %w", err)
			}
			if syncID != nil {
				m.FederationSyncID = *syncID
			}
			if queryID != nil {
				m.FederationQueryID = *queryID
			}
			if err := f(&m); err != nil {
				return err
			}
			offset++
		}

		return nil
	}); err != nil {
		return cursor(), fmt.Errorf("iterate exposures: %w", err)
	}

	return "", nil
}

func generateExposureQuery(criteria IterateExposuresCriteria) (string, []interface{}, error) {
	var args []interface{}
	q := `
		SELECT
			exposure_key, transmission_risk, LOWER(app_package_name), regions, traveler,
			interval_number, interval_count,
			created_at, local_provenance, sync_id, sync_query_id, health_authority_id, report_type,
			days_since_symptom_onset, revised_report_type, revised_at, revised_days_since_symptom_onset
		FROM
			Exposure
		WHERE 1=1
	`

	if len(criteria.IncludeRegions) == 1 {
		if criteria.IncludeTravelers {
			// If the query has include ragions and include travelers set - we want the union of the specified regions and
			// all "traveler" keys that this server knows about.
			args = append(args, criteria.IncludeRegions)
			args = append(args, true)
			q += fmt.Sprintf(" AND ((regions && $%d) OR traveler = $%d)", len(args)-1, len(args)) // Operation "&&" means "array overlaps / intersects"
		} else {
			args = append(args, criteria.IncludeRegions)
			q += fmt.Sprintf(" AND (regions && $%d)", len(args)) // Operation "&&" means "array overlaps / intersects"
		}
	}

	if len(criteria.ExcludeRegions) == 1 {
		args = append(args, criteria.ExcludeRegions)
		q += fmt.Sprintf(" AND NOT (regions && $%d)", len(args)) // Operation "&&" means "array overlaps / intersects"
	}

	timeField := "created_at"
	if criteria.OnlyRevisedKeys {
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

	if criteria.OnlyTravelers {
		args = append(args, true)
		q += fmt.Sprintf(" AND traveler = $%d", len(args))
	}

	if criteria.OnlyRevisedKeys {
		q += " ORDER BY revised_at"
	} else {
		q += " ORDER BY created_at"
	}

	if criteria.LastCursor != "" {
		decoded, err := decodeCursor(criteria.LastCursor)
		if err != nil {
			return "", nil, err
		}
		args = append(args, decoded)
		q += fmt.Sprintf(" OFFSET $%d", len(args))
	}

	if criteria.Limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", criteria.Limit)
	}

	q = strings.ReplaceAll(q, "\n", " ")

	return q, args, nil
}

// ReadExposures will read an existing set of exposures from the database.
// This is necessary in case a key needs to be revised.
// In the return map, the key is the base64 of the ExposureKey.
// The keys are read for update in a provided transaction.
func (db *PublishDB) ReadExposures(ctx context.Context, tx pgx.Tx, b64keys []string) (map[string]*model.Exposure, error) {
	exposures := make(map[string]*model.Exposure)

	if err := db.db.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT
				exposure_key, transmission_risk, app_package_name, regions, traveler,
				interval_number, interval_count, created_at, local_provenance, sync_id,
				health_authority_id, report_type, days_since_symptom_onset,
				revised_report_type, revised_at, revised_days_since_symptom_onset,
				revised_transmission_risk
			FROM
				Exposure
			WHERE exposure_key = ANY($1)
			FOR UPDATE
		`, b64keys)
		if err != nil {
			return fmt.Errorf("failed to list: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			if err := rows.Err(); err != nil {
				return fmt.Errorf("failed to iterate: %w", err)
			}

			var encodedKey string
			var syncID sql.NullInt64

			var exposure model.Exposure
			if err := rows.Scan(
				&encodedKey, &exposure.TransmissionRisk, &exposure.AppPackageName,
				&exposure.Regions, &exposure.Traveler, &exposure.IntervalNumber, &exposure.IntervalCount,
				&exposure.CreatedAt, &exposure.LocalProvenance, &syncID,
				&exposure.HealthAuthorityID, &exposure.ReportType, &exposure.DaysSinceSymptomOnset,
				&exposure.RevisedReportType, &exposure.RevisedAt, &exposure.RevisedDaysSinceSymptomOnset,
				&exposure.RevisedTransmissionRisk,
			); err != nil {
				return fmt.Errorf("failed to parse: %w", err)
			}

			// Base64 decode the exposure key
			exposure.ExposureKey, err = decodeExposureKey(encodedKey)
			if err != nil {
				return fmt.Errorf("failed to decode key: %w", err)
			}
			// Optionally set all of the nullable columns.
			if syncID.Valid {
				exposure.FederationSyncID = syncID.Int64
			}

			exposures[exposure.ExposureKeyBase64()] = &exposure
		}

		return nil
	}); err != nil {
		return nil, fmt.Errorf("read exposures: %w", err)
	}

	return exposures, nil
}

func prepareInsertExposure(ctx context.Context, tx pgx.Tx) (string, error) {
	const stmtName = "insert exposures"
	_, err := tx.Prepare(ctx, stmtName, `
		INSERT INTO
			Exposure
				(exposure_key, transmission_risk, app_package_name, regions, traveler, interval_number, interval_count,
				 created_at, local_provenance, sync_id, sync_query_id, health_authority_id, report_type, days_since_symptom_onset)
		VALUES
			($1, $2, LOWER($3), $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		ON CONFLICT (exposure_key) DO NOTHING
	`)
	return stmtName, err
}

func executeInsertExposure(ctx context.Context, tx pgx.Tx, stmtName string, exp *model.Exposure) error {
	var syncID *int64
	var queryID *string
	if exp.FederationSyncID != 0 {
		syncID = &exp.FederationSyncID
	}
	if exp.FederationQueryID != "" {
		queryID = &exp.FederationQueryID
	}

	_, err := tx.Exec(ctx, stmtName, encodeExposureKey(exp.ExposureKey), exp.TransmissionRisk,
		exp.AppPackageName, exp.Regions, exp.Traveler, exp.IntervalNumber, exp.IntervalCount,
		exp.CreatedAt, exp.LocalProvenance, syncID, queryID,
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

// InsertAndReviseExposuresRequest is used as input to InsertAndReviseExposures.
type InsertAndReviseExposuresRequest struct {
	Incoming []*model.Exposure
	Token    *pb.RevisionTokenData

	// RequireToken requires that the request supply a revision token to re-upload
	// existing keys.
	RequireToken bool

	// AllowPartialRevisions allows revising a subset of exposures if other
	// exposures are included that are not part of the revision token. This exists
	// to support roaming scenarios. This is only used if RequireToken is true.
	AllowPartialRevisions bool

	// The following operations are for federation.

	// If true, if a key is determined to be a revsion, it is skipped.
	SkipRevions bool
	// If true, only revisions will be processed.
	OnlyRevisions bool
	// Require matching Sync QueryID only allows revisions if they originated from the same query ID.
	RequireQueryID bool
}

// InsertAndReviseExposuresResponse is the response from an
// InsertAndReviseExposures call.
type InsertAndReviseExposuresResponse struct {
	// Inserted is the number of new exposures that were inserted into the
	// database.
	Inserted uint64

	// Revised is the number of exposures that matched an existing TEK and were
	// subsequently revised.
	Revised uint64

	// Dropped is the number of exposures that were not inserted or updated. This
	// could be because they weren't present in the revision token, etc.
	Dropped uint64

	// Exposures is the actual exposures that were inserted or updated in this
	// call.
	Exposures []*model.Exposure
}

// InsertAndReviseExposures transactionally revises and inserts a set of keys as
// necessary.
func (db *PublishDB) InsertAndReviseExposures(ctx context.Context, req *InsertAndReviseExposuresRequest) (*InsertAndReviseExposuresResponse, error) {
	logger := logging.FromContext(ctx).Named("InsertAndReviseExposures")

	if req == nil {
		return nil, fmt.Errorf("missing request")
	}
	if req.SkipRevions && req.OnlyRevisions {
		return nil, fmt.Errorf("configuration paradox: skipRevisions and onlyRevisions are both set to true")
	}

	// Maintain a record of the number of exposures inserted and updated, and a
	// record of the exposures that were actually inserted/updated after merge
	// logic.
	var resp InsertAndReviseExposuresResponse

	if err := db.db.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
		// Build the base64-encoded list of keys - this is needed so we can lookup
		// the keys in the database. Also build a lookup map by key for validation
		// later.
		b64keys := make([]string, len(req.Incoming))
		incomingMap := make(map[string]*model.Exposure, len(req.Incoming))
		for i, v := range req.Incoming {
			b64keys[i] = v.ExposureKeyBase64()
			incomingMap[v.ExposureKeyBase64()] = v
		}

		// Lookup the keys in the database and build a lookup map for validation
		// later.
		existing, err := db.ReadExposures(ctx, tx, b64keys)
		if err != nil {
			return fmt.Errorf("unable to check for existing records: %w", err)
		}
		existingMap := make(map[string]*model.Exposure, len(existing))
		for _, v := range existing {
			existingMap[v.ExposureKeyBase64()] = v
		}

		// For federation - if we ONLY want to process revisions.
		if req.OnlyRevisions {
			for k := range incomingMap {
				if _, ok := existingMap[k]; !ok {
					delete(incomingMap, k)
				}
			}
		}

		// See if the revision token is relevant. We only need to check it if keys
		// are being revised.
		if len(existing) > 0 {
			// Check if a revision token is required.
			if req.Token == nil {
				logger.Errorw("attempted to revise keys, but revision token is missing")
				if req.RequireToken {
					return ErrNoRevisionToken
				}
			}

			// Build a map of allowed revisions for validation and comparison.
			allowedRevisions := make(map[string]*pb.RevisableKey)
			if req.Token != nil {
				// Special handling for the allow bypass scenario where no token was presented.
				for _, v := range req.Token.RevisableKeys {
					b := base64.StdEncoding.EncodeToString(v.TemporaryExposureKey)
					allowedRevisions[b] = v
				}
			}

			// Check that any existing exposures are present in the token.
			for k, ex := range existing {
				// For federation, if a key is rquested for insert.
				if req.SkipRevions {
					logger.Warnw("skipping key: would be revised but revision disabled for request")
					delete(incomingMap, k)
					continue
				}

				// For federation. If the exposure is inbound on the same query, it is allowed.
				if req.RequireQueryID {
					if in, ok := incomingMap[k]; ok {
						if in.FederationQueryID != ex.FederationQueryID {
							logger.Warnw("key revision attempted on federated key with wrong origin", "queryID", ex.FederationQueryID, "proposedQueryID", in.FederationQueryID)
							delete(incomingMap, k)
							continue
						}
					}
				}

				// Check the incoming values first.
				if in, ok := incomingMap[k]; ok {
					if ex.IntervalNumber != in.IntervalNumber || ex.IntervalCount != in.IntervalCount {
						logger.Errorw("incoming metadata mismatch",
							"existing_count", ex.IntervalCount,
							"existing_number", ex.IntervalNumber,
							"incoming_count", in.IntervalCount,
							"incoming_number", in.IntervalNumber)
						if req.RequireToken {
							return ErrIncomingMetadataMismatch
						}
					}
				}

				// Now check against allowed revisions.
				if rk, ok := allowedRevisions[k]; ok {
					if ex.IntervalNumber != rk.IntervalNumber || ex.IntervalCount != rk.IntervalCount {
						logger.Errorw("token metadata mismatch",
							"existing_count", ex.IntervalCount,
							"existing_number", ex.IntervalNumber,
							"incoming_count", rk.IntervalCount,
							"incoming_number", rk.IntervalNumber)
						if req.RequireToken {
							return ErrRevisionTokenMetadataMismatch
						}
					}
				} else {
					// The user sent an existing key for which they do not have a revision
					// token. There's a plausible scenario with roaming where this user
					// has changed apps in the middle of a roaming period, and thus this
					// could be legitimate.
					//
					// Suppose a user was in California, using the California app. They
					// feel ill, do a video call and get a "likely" diagnosis. They decide
					// to drive to Montana to quarantine, downloading the Montana app.
					// Once in Montanan, their symptoms worsen and they get a clinical
					// diagnosis as "positive", marking as such in the app. Since the
					// Montana app does not have a revision token for the keys that were
					// previously uploaded when the user was in California, some of the
					// keys cannot be revised.
					if req.RequireToken {
						if req.AllowPartialRevisions {
							logger.Warnw("skipping key: not in revision token, but " +
								"partial revision is permitted")
							delete(incomingMap, k)
						} else {
							logger.Errorw("cannot revise key: not in revision token")
							return ErrExistingKeyNotInToken
						}
					}
				}
			}
		}

		// Calculate the number of dropped responses.
		resp.Dropped = uint64(len(req.Incoming) - len(incomingMap))

		// If we got this far, the revision token is valid for this request, not
		// required, or bypassed. It's possible that keys were given, but none of
		// those keys matched the revision token keys and partial responses were
		// allowed. In that case, stop here.
		if len(incomingMap) == 0 {
			return nil
		}

		// Build the true incoming list from the map. It's possible items were removed
		// from the map if they did not exist in the revision token and partial
		// responses were allowed.
		incoming := make([]*model.Exposure, 0, len(incomingMap))
		for _, v := range incomingMap {
			incoming = append(incoming, v)
		}

		// Run through the merge logic.
		exposures, err := model.ReviseKeys(ctx, existing, incoming)
		if err != nil {
			return fmt.Errorf("unable to revise keys: %w", err)
		}
		resp.Exposures = exposures

		// Prepare the insert and update statements.
		insertStmt, err := prepareInsertExposure(ctx, tx)
		if err != nil {
			return fmt.Errorf("preparing insert statement: %v", err)
		}
		updateStmt, err := prepareReviseExposure(ctx, tx)
		if err != nil {
			return fmt.Errorf("preparing update statement: %v", err)
		}
		for _, exp := range exposures {
			if exp.RevisedAt == nil {
				if exp.ReportType == verifyapi.ReportTypeNegative {
					continue
				}
				if err := executeInsertExposure(ctx, tx, insertStmt, exp); err != nil {
					return err
				}
				resp.Inserted++
			} else {
				if err := executeReviseExposure(ctx, tx, updateStmt, exp); err != nil {
					return err
				}
				resp.Revised++
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return &resp, nil
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
