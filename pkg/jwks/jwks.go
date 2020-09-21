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
	"io/ioutil"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/exposure-notifications-server/internal/database"
	hadb "github.com/google/exposure-notifications-server/internal/verification/database"
	"github.com/google/exposure-notifications-server/internal/verification/model"
	"github.com/rakutentech/jwk-go/jwk"
	"go.uber.org/zap"
)

type Config struct {
	Database database.Config
}

func (c *Config) DatabaseConfig() *database.Config {
	return &c.Database
}

func (c *Config) JWKSManagerConfig() *Config {
	return c
}

// Manager handles updating all HealthAuthorities if they've specified a JWKS
// URI.
type Manager struct {
	ctxt   context.Context
	logger *zap.SugaredLogger
	db     *database.DB
}

// getKeys reads the keys for a single HealthAuthority from its jwks server.
func (mgr Manager) getKeys(ha *model.HealthAuthority) ([]byte, error) {
	if len(ha.JwksURI) == 0 {
		return nil, nil
	}

	reqCtxt, done := context.WithTimeout(mgr.ctxt, 5*time.Second)
	defer done()
	req, err := http.NewRequestWithContext(reqCtxt, "GET", ha.JwksURI, nil)
	if err != nil {
		return nil, fmt.Errorf("creating connection: %w", err)
	}

	var resp *http.Response
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("reading connection: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("resp (%v) != %v", resp.StatusCode, http.StatusOK)
	}

	var bytes []byte
	bytes, err = ioutil.ReadAll(resp.Body)
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
		keys[i] = string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: encoded}))
		versions[keys[i]] = spec.KeyID
	}
	return keys, versions, nil
}

// stripKey strips pem signature from a key.
func stripKey(k string) string {
	k = strings.ReplaceAll(k, "-----BEGIN PUBLIC KEY-----", "")
	k = strings.ReplaceAll(k, "-----END PUBLIC KEY-----", "")
	k = strings.ReplaceAll(k, "\n", "")
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
func (mgr *Manager) updateHA(haDB *hadb.HealthAuthorityDB, ha *model.HealthAuthority) error {
	if len(ha.JwksURI) == 0 {
		mgr.logger.Infof("Skipping JWKS, HealthAuthority: %q No URI specified.", ha.Name)
		return nil
	}

	resp, err := mgr.getKeys(ha)
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
		if err := haDB.UpdateHealthAuthorityKey(mgr.ctxt, hak); err != nil {
			return fmt.Errorf("error updating key: %w", err)
		}
	}

	// Create new keys as needed.
	for _, key := range newKeys {
		hak := &model.HealthAuthorityKey{
			AuthorityID:  ha.ID,
			Version:      versions[key],
			From:         time.Now(),
			PublicKeyPEM: key,
		}
		if err := haDB.AddHealthAuthorityKey(mgr.ctxt, ha, hak); err != nil {
			return fmt.Errorf("error adding key: %w", err)
		}
		ha.Keys = append(ha.Keys, hak)
	}

	// And save the HealthAuthority.
	haDB.UpdateHealthAuthority(mgr.ctxt, ha)

	mgr.logger.Infof("Updated JWKS HealthAuthority: %q URI (%v): %d new, %d deleted", ha.Name, ha.JwksURI, len(newKeys), len(deadKeys))

	return nil
}

// UpdateAll reads the JWKS keys for all HealthAuthorities.
func (mgr Manager) UpdateAll() {
	mgr.logger.Info("Starting JWKS update")

	haDB := hadb.New(mgr.db)
	healthAuthorities, err := haDB.ListAllHealthAuthoritiesWithoutKeys(mgr.ctxt)
	if err != nil {
		mgr.logger.Errorf("error querying db %v", err)
		return
	}

	var wg sync.WaitGroup
	for _, ha := range healthAuthorities {
		wg.Add(1)
		go func(ha *model.HealthAuthority) {
			defer wg.Done()
			err := mgr.updateHA(haDB, ha)
			if err != nil {
				mgr.logger.Errorf("error running JWKS HealthAuthority: %q error: %v", ha.Name, err)
				return
			}
		}(ha)
	}
	wg.Wait()

	mgr.logger.Info("Done JWKS update")
}

// NewFromEnv creates a new JWKS configuration manager.
func NewFromEnv(ctxt context.Context, logger *zap.SugaredLogger, db *database.DB, cfg *Config) (Manager, error) {
	return Manager{
		ctxt:   ctxt,
		logger: logger,
		db:     db,
	}, nil
}
