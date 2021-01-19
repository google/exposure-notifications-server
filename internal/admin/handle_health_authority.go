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
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/exposure-notifications-server/internal/project"
	"github.com/google/exposure-notifications-server/internal/verification/database"
	"github.com/google/exposure-notifications-server/internal/verification/model"
)

// HandleHealthAuthoritySave handles the create/update actions for health
// authorities.
func (s *Server) HandleHealthAuthoritySave() func(c *gin.Context) {
	return func(c *gin.Context) {
		var form healthAuthorityFormData
		err := c.Bind(&form)
		if err != nil {
			ErrorPage(c, err.Error())
			return
		}

		ctx := c.Request.Context()
		m := TemplateMap{}

		haDB := database.New(s.env.Database())
		healthAuthority := &model.HealthAuthority{}
		haID, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			ErrorPage(c, "Unable to parse `id` param")
			return
		}
		if haID != 0 {
			healthAuthority, err = haDB.GetHealthAuthorityByID(ctx, haID)
			if err != nil {
				ErrorPage(c, fmt.Sprintf("error processing health authority: %v", err))
				return
			}
		}
		form.PopulateHealthAuthority(healthAuthority)

		// Decide if update or insert.
		updateFn := haDB.AddHealthAuthority
		if haID != 0 {
			updateFn = haDB.UpdateHealthAuthority
		}
		if err := updateFn(ctx, healthAuthority); err != nil {
			ErrorPage(c, fmt.Sprintf("Error writing health authority: %v", err))
			return
		}

		m.AddSuccess(fmt.Sprintf("Updated Health Authority '%v'", healthAuthority.Issuer))
		m["ha"] = healthAuthority
		m["hak"] = &model.HealthAuthorityKey{From: time.Now()} // For create form.
		c.HTML(http.StatusOK, "healthauthority", m)
	}
}

// HandleHealthAuthorityShow handles the show action for health authorities.
func (s *Server) HandleHealthAuthorityShow() func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		m := TemplateMap{}

		healthAuthority := &model.HealthAuthority{
			EnableStatsAPI: true, // default enabled.
		}
		if IDParam := c.Param("id"); IDParam == "0" {
			m["new"] = true
		} else {
			haID, err := strconv.ParseInt(IDParam, 10, 64)
			if err != nil {
				ErrorPage(c, "Unable to parse `id` param.")
				return
			}

			haDB := database.New(s.env.Database())
			healthAuthority, err = haDB.GetHealthAuthorityByID(ctx, haID)
			if err != nil {
				ErrorPage(c, fmt.Sprintf("Unable to find requested health authority: %v. Error: %v", haID, err))
				return
			}
		}
		m["ha"] = healthAuthority
		m["hak"] = &model.HealthAuthorityKey{From: time.Now()} // For create form.
		c.HTML(http.StatusOK, "healthauthority", m)
	}
}

// HandleHealthAuthorityKeys handles the keys action for health authorities.
func (s *Server) HandleHealthAuthorityKeys() func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		haDB := database.New(s.env.Database())
		haID, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			ErrorPage(c, "Unable to parse `id` param")
			return
		}
		healthAuthority, err := haDB.GetHealthAuthorityByID(ctx, haID)
		if err != nil {
			ErrorPage(c, fmt.Sprintf("error processing health authority: %v", err))
			return
		}

		if action := c.Param("action"); action == "create" {
			var form keyhealthAuthorityFormData
			err := c.Bind(&form)
			if err != nil {
				ErrorPage(c, err.Error())
				return
			}

			var hak model.HealthAuthorityKey
			err = form.PopulateHealthAuthorityKey(&hak)
			if err != nil {
				ErrorPage(c, fmt.Sprintf("Error parsing new health authority key: %v", err))
				return
			}
			err = haDB.AddHealthAuthorityKey(ctx, healthAuthority, &hak)
			if err != nil {
				ErrorPage(c, fmt.Sprintf("Error saving health authority key: %v", err))
				return
			}

		} else if action == "revoke" || action == "reinstate" || action == "activate" {
			version := c.Param("version")

			// find the key.
			var hak *model.HealthAuthorityKey
			for _, key := range healthAuthority.Keys {
				if key.Version == version {
					hak = key
					break
				}
			}
			if hak == nil {
				ErrorPage(c, "Invalid key specified")
				return
			}

			if action == "activate" {
				if hak.IsFuture() {
					hak.From = time.Now()
				}
			} else if action == "revoke" {
				hak.Revoke()
			} else {
				hak.Thru = time.Time{}
			}

			log.Printf("HAK %+v", hak)

			err = haDB.UpdateHealthAuthorityKey(ctx, hak)
			if err != nil {
				ErrorPage(c, fmt.Sprintf("Error saving health authority key: %v", err))
				return
			}

		} else {
			ErrorPage(c, "invalid action")
			return
		}
		c.Redirect(http.StatusFound, fmt.Sprintf("/healthauthority/%v", healthAuthority.ID))
	}
}

type healthAuthorityFormData struct {
	Issuer         string `form:"issuer"`
	Audience       string `form:"audience"`
	Name           string `form:"name"`
	EnableStatsAPI bool   `form:"enable-stats-api"`
	JwksURI        string `form:"jwks-uri"`
}

func (f *healthAuthorityFormData) PopulateHealthAuthority(ha *model.HealthAuthority) {
	ha.Issuer = f.Issuer
	ha.Audience = f.Audience
	ha.Name = f.Name
	ha.EnableStatsAPI = f.EnableStatsAPI
	ha.SetJWKS(f.JwksURI)
}

type keyhealthAuthorityFormData struct {
	Version  string `form:"version"`
	PEMBlock string `form:"public-key-pem"`
	FromDate string `form:"from-date"`
	FromTime string `form:"from-time"`
	ThruDate string `form:"thru-date"`
	ThruTime string `form:"thru-time"`
}

func (f *keyhealthAuthorityFormData) FromTimestamp() (time.Time, error) {
	return CombineDateAndTime(f.FromDate, f.FromTime)
}

func (f *keyhealthAuthorityFormData) ThruTimestamp() (time.Time, error) {
	return CombineDateAndTime(f.ThruDate, f.ThruTime)
}

func (f *keyhealthAuthorityFormData) PopulateHealthAuthorityKey(hak *model.HealthAuthorityKey) error {
	fTime, err := f.FromTimestamp()
	if err != nil {
		return err
	}
	tTime, err := f.ThruTimestamp()
	if err != nil {
		return err
	}
	hak.Version = project.TrimSpaceAndNonPrintable(f.Version)
	hak.From = fTime
	hak.Thru = tTime
	hak.PublicKeyPEM = project.TrimSpaceAndNonPrintable(f.PEMBlock)

	_, err = hak.PublicKey()
	return err
}
