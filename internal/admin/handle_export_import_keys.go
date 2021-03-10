// Copyright 2021 Google LLC
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

package admin

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/exposure-notifications-server/internal/exportimport/database"
	"github.com/google/exposure-notifications-server/internal/exportimport/model"
	"github.com/google/exposure-notifications-server/internal/project"
)

type exportImportKeyForm struct {
	KeyID     string `form:"keyid"`
	Version   string `form:"version"`
	PublicKey string `form:"public-key-pem"`

	// FromDate and FromTime are combined into FromTimestamp.
	FromDate string `form:"from-date"`
	FromTime string `form:"from-time"`

	// ThruDate and ThruTime are combined into ThruTimestamp.
	ThruDate string `form:"thru-date"`
	ThruTime string `form:"thru-time"`
}

func (f *exportImportKeyForm) FromTimestamp() (time.Time, error) {
	return CombineDateAndTime(f.FromDate, f.FromTime)
}

func (f *exportImportKeyForm) ThruTimestamp() (time.Time, error) {
	return CombineDateAndTime(f.ThruDate, f.ThruTime)
}

func (f *exportImportKeyForm) PopulateImportFilePublicKey(exportImportID int64, key *model.ImportFilePublicKey) error {
	key.ExportImportID = exportImportID
	key.KeyID = f.KeyID
	key.KeyVersion = f.Version
	key.PublicKeyPEM = strings.ReplaceAll(project.TrimSpaceAndNonPrintable(f.PublicKey), "\r", "")

	fTime, err := f.FromTimestamp()
	if err != nil {
		return fmt.Errorf("invalid from time: %w", err)
	}
	if fTime.IsZero() {
		fTime = time.Now().UTC().Add(-1 * time.Minute)
	}
	key.From = fTime

	tTime, err := f.ThruTimestamp()
	if err != nil {
		return fmt.Errorf("invalid thru time: %w", err)
	}
	if !tTime.IsZero() {
		key.Thru = &tTime
	}

	return nil
}

func (s *Server) HandleExportImportKeys() func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		db := database.New(s.env.Database())
		exportImportID, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			ErrorPage(c, "Unable to parse `id` param")
			return
		}
		exportImport, err := db.GetConfig(ctx, exportImportID)
		if err != nil {
			ErrorPage(c, fmt.Sprintf("error reading export-import config: %v", err))
			return
		}

		if action := c.Param("action"); action == "create" {
			var form exportImportKeyForm
			if err := c.Bind(&form); err != nil {
				ErrorPage(c, err.Error())
				return
			}

			var key model.ImportFilePublicKey
			if err := form.PopulateImportFilePublicKey(exportImport.ID, &key); err != nil {
				ErrorPage(c, fmt.Sprintf("Error parsing new import file public key: %v", err))
				return
			}

			if err := db.AddImportFilePublicKey(ctx, &key); err != nil {
				ErrorPage(c, fmt.Sprintf("error saving new public key: %v", err))
				return
			}
		} else if action == "revoke" || action == "reinstate" || action == "activate" {
			existingKeys, err := db.AllPublicKeys(ctx, exportImport)
			if err != nil {
				ErrorPage(c, fmt.Sprintf("Unable to load existing public keys: %v", err))
				return
			}

			// find the key.
			var importFileKey *model.ImportFilePublicKey
			for _, key := range existingKeys {
				if key.KeyID == c.Param("keyid") {
					importFileKey = key
					break
				}
			}
			if importFileKey == nil {
				ErrorPage(c, "Invalid key specified")
				return
			}

			if action == "activate" {
				if importFileKey.Future() {
					importFileKey.From = time.Now()
				}
			} else if action == "revoke" {
				importFileKey.Revoke()
			} else {
				importFileKey.Thru = nil
			}

			err = db.SavePublicKeyTimestamps(ctx, importFileKey)
			if err != nil {
				ErrorPage(c, fmt.Sprintf("Error manipulating public key: %v", err))
				return
			}
		} else {
			ErrorPage(c, "invalid action")
			return
		}
		c.Redirect(http.StatusFound, fmt.Sprintf("/export-importers/%d", exportImport.ID))
	}
}
