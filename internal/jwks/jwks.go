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

// Package jwks manages downloading and updating the keys from a JWKS source
// for keys.
package jwks

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/exposure-notifications-server/internal/project"
	hadb "github.com/google/exposure-notifications-server/internal/verification/database"
	"github.com/google/exposure-notifications-server/internal/verification/model"
	"github.com/google/exposure-notifications-server/pkg/cryptorand"
	"github.com/google/exposure-notifications-server/pkg/database"
	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/hashicorp/go-multierror"
	"github.com/rakutentech/jwk-go/jwk"
	"golang.org/x/sync/semaphore"
)

// Manager handles updating all HealthAuthorities if they've specified a JWKS
// URI.
type Manager struct {
	db         *database.DB
	client     *http.Client
	cleanupTTL time.Duration
	maxWorkers uint
}

// NewManager creates a new Manager.
func NewManager(db *database.DB, cleanupTTL, requestTimeout time.Duration, maxWorkers uint) (*Manager, error) {
	if cleanupTTL < 0 {
		cleanupTTL *= -1
	}

	client := &http.Client{
		Timeout: requestTimeout,
	}

	return &Manager{
		db:         db,
		client:     client,
		cleanupTTL: cleanupTTL,
		maxWorkers: maxWorkers,
	}, nil
}

// getKeys reads the keys for a single HealthAuthority from its jwks server.
func (mgr *Manager) getKeys(ctx context.Context, ha *model.HealthAuthority) ([]byte, error) {
	if ha.JwksURI == nil {
		return nil, nil
	}
	jwksURI := *ha.JwksURI
	if len(jwksURI) == 0 {
		return nil, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, jwksURI, nil)
	if err != nil {
		return nil, fmt.Errorf("creating connection: %w", err)
	}

	resp, err := mgr.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("reading connection: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("resp (%v) != %v", resp.StatusCode, http.StatusOK)
	}

	var bytes []byte
	bytes, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading: %w", err)
	}
	return bytes, nil
}

// parseKeys parses the json response, returning the pem encoded public keys,
// and versions.
func parseKeys(data []byte) ([]string, map[string]string, error) {
	if len(data) == 0 {
		return nil, nil, nil
	}

	var jwks []jwk.JWK
	if err := json.Unmarshal(data, &jwks); err != nil {
		return nil, nil, fmt.Errorf("unmarshal error: %w", err)
	}

	keys := make([]string, len(jwks))
	versions := make(map[string]string, len(jwks))
	for i := range jwks {
		spec, err := jwks[i].ParseKeySpec()
		if err != nil {
			return nil, nil, fmt.Errorf("parse error: %w", err)
		}

		var encoded []byte
		encoded, err = x509.MarshalPKIXPublicKey(spec.Key)
		if err != nil {
			return nil, nil, fmt.Errorf("marshal error: %w", err)
		}
		keys[i] = project.TrimSpaceAndNonPrintable(string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: encoded})))
		versions[keys[i]] = spec.KeyID
	}
	return keys, versions, nil
}

// stripKey strips pem signature from a key.
func stripKey(k string) string {
	k = strings.ReplaceAll(k, "-----BEGIN PUBLIC KEY-----", "")
	k = strings.ReplaceAll(k, "-----END PUBLIC KEY-----", "")
	k = strings.ReplaceAll(k, "\n", "")
	k = strings.ReplaceAll(k, "\r", "")
	return k
}

// findKeyMods compares the read keys, and the keys already in the database,
// and returns lists of the deadKey indices, new keys. (note the dead keys are
// sorted.)
//
// NB: We don't depend on the keyID for this, we look at the actual pem strings,
// and compare them.
func findKeyMods(ha *model.HealthAuthority, rxKeys []string) (deadKeys []int, newKeys []string) {
	// Figure out which keys are active, or deleted.
	keySet := make(map[string]int, len(rxKeys))
	for _, key := range rxKeys {
		keySet[stripKey(key)] = -1
	}
	for i, key := range ha.Keys {
		keyStr := stripKey(key.PublicKeyPEM)
		if _, ok := keySet[keyStr]; !ok {
			deadKeys = append(deadKeys, i)
		} else {
			keySet[keyStr] = i
		}
	}
	for _, key := range rxKeys {
		if keySet[stripKey(key)] == -1 {
			newKeys = append(newKeys, key)
		}
	}
	sort.Ints(deadKeys)
	return
}

// updateHA updates HealthAuthority's keys.
func (mgr *Manager) updateHA(ctx context.Context, ha *model.HealthAuthority) error {
	logger := logging.FromContext(ctx).Named("updateHA").
		With("health_authority_name", ha.Name).
		With("health_authority_id", ha.ID)

	if !ha.JWKSEnabled() {
		return nil
	}

	// Create the hadb once to save allocations
	haDB := hadb.New(mgr.db)

	purgeBefore := time.Now().UTC().Add(-1 * mgr.cleanupTTL)
	if count, err := haDB.PurgeHealthAuthorityKeys(ctx, ha, purgeBefore); err != nil {
		logger.Errorw("error purging expired health authority keys", "error", err)
	} else if count > 0 {
		logger.Infow("purged health authority keys", "count", count)
	}

	// Get the keys for the health authority
	keys, err := haDB.GetHealthAuthorityKeys(ctx, ha)
	if err != nil {
		return fmt.Errorf("error getting keys: %w", err)
	}

	ha.Keys = keys

	resp, err := mgr.getKeys(ctx, ha)
	if err != nil {
		return err
	}

	var rxKeys []string
	var versions map[string]string
	rxKeys, versions, err = parseKeys(resp)
	if err != nil {
		return fmt.Errorf("error parsing key: %w", err)
	}

	// Get the modifications we need to make.
	deadKeys, newKeys := findKeyMods(ha, rxKeys)

	// Mark all dead keys.
	// Note, keys aren't removed from the HealthAuthority, there just marked
	// revoked.
	for _, i := range deadKeys {
		hak := ha.Keys[i]
		hak.Revoke()
		if err := haDB.UpdateHealthAuthorityKey(ctx, hak); err != nil {
			return fmt.Errorf("error updating key: %w", err)
		}
	}

	// Create new keys as needed.
	for _, key := range newKeys {
		hak := &model.HealthAuthorityKey{
			AuthorityID:  ha.ID,
			Version:      versions[key],
			From:         time.Now(),
			PublicKeyPEM: project.TrimSpaceAndNonPrintable(key),
		}
		if err := haDB.AddHealthAuthorityKey(ctx, ha, hak); err != nil {
			return fmt.Errorf("error adding key: %w", err)
		}
	}

	logger.Infow("updated jwks",
		"uri", ha.JwksURI,
		"new", len(newKeys),
		"deleted", len(deadKeys))

	return nil
}

// UpdateAll reads the JWKS keys for all HealthAuthorities.
func (mgr *Manager) UpdateAll(ctx context.Context) error {
	logger := logging.FromContext(ctx).Named("UpdateAll")
	logger.Debug("starting jwks update")
	defer logger.Debug("finished jwks update")

	haDB := hadb.New(mgr.db)
	allHealthAuthorities, err := haDB.ListAllHealthAuthoritiesWithoutKeys(ctx)
	if err != nil {
		return fmt.Errorf("failed to query db: %w", err)
	}

	// Pre-filter out health authorities where discovery isn't enabled.
	healthAuthorities := make([]*model.HealthAuthority, 0, len(allHealthAuthorities))
	for _, ha := range allHealthAuthorities {
		if ha.JWKSEnabled() {
			healthAuthorities = append(healthAuthorities, ha)
		}
	}

	//nolint:gosec
	r := rand.New(cryptorand.NewSource())
	r.Shuffle(len(healthAuthorities), func(i, j int) {
		healthAuthorities[i], healthAuthorities[j] = healthAuthorities[j], healthAuthorities[i]
	})

	var merr *multierror.Error
	var merrLock sync.Mutex
	sem := semaphore.NewWeighted(int64(mgr.maxWorkers))
	var wg sync.WaitGroup
	for _, ha := range healthAuthorities {
		// Rate limit the number of concurrent workers with a semaphore.
		if err := sem.Acquire(ctx, 1); err != nil {
			logger.Errorw("failed to acquire semaphore", "error", err)
			merrLock.Lock()
			merr = multierror.Append(merr, fmt.Errorf("failed to processes %v: %w", ha.Name, err))
			merrLock.Unlock()
			break
		}

		wg.Add(1)
		go func(ha *model.HealthAuthority) {
			defer sem.Release(1)
			defer wg.Done()
			err := mgr.updateHA(ctx, ha)
			if err != nil {
				merrLock.Lock()
				merr = multierror.Append(merr, fmt.Errorf("failed to processes %v: %w", ha.Name, err))
				merrLock.Unlock()
			}
		}(ha)
	}
	wg.Wait()

	if err := merr.ErrorOrNil(); err != nil {
		return fmt.Errorf("failed to update all: %w", err)
	}
	return nil
}
