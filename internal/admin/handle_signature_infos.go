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
	"crypto"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/exposure-notifications-server/internal/export/database"
	"github.com/google/exposure-notifications-server/internal/export/model"
	"github.com/google/exposure-notifications-server/internal/project"
)

const defaultDigestMessage = "hello world"

// HandleSignatureInfosSave handles the create/update actions for signature
// infos.
func (s *Server) HandleSignatureInfosSave() func(c *gin.Context) {
	return func(c *gin.Context) {
		var form signatureInfoFormData
		err := c.Bind(&form)
		if err != nil {
			ErrorPage(c, err.Error())
			return
		}

		ctx := c.Request.Context()
		m := TemplateMap{}

		exportDB := database.New(s.env.Database())

		sigID, err := strconv.Atoi(c.Param("id"))
		sigInfo := &model.SignatureInfo{}
		if err != nil {
			ErrorPage(c, "Unable to parse `id` param.")
			return
		}
		if sigID != 0 {
			sigInfo, err = exportDB.GetSignatureInfo(ctx, int64(sigID))
			if err != nil {
				ErrorPage(c, fmt.Sprintf("error processing signature info: %v", err))
				return
			}
		}
		if err := form.PopulateSigInfo(sigInfo); err != nil {
			ErrorPage(c, fmt.Sprintf("error processing signature info: %v", err))
			return
		}

		// Either insert or update.
		updateFn := exportDB.AddSignatureInfo
		if sigID != 0 {
			updateFn = exportDB.UpdateSignatureInfo
		}
		if err := updateFn(ctx, sigInfo); err != nil {
			ErrorPage(c, fmt.Sprintf("Error writing signature info: %v", err))
			return
		}

		m["siginfo"] = sigInfo

		// For new records, sign the magical "hello world" string
		if sigID == 0 {
			signer, err := s.env.GetSignerForKey(ctx, sigInfo.SigningKey)
			if err != nil {
				m.AddErrors(fmt.Sprintf("Failed to get key signer: %s", err))
				c.HTML(http.StatusOK, "siginfo", m)
				return
			}

			digest := sha256.Sum256([]byte(defaultDigestMessage))
			sig, err := signer.Sign(rand.Reader, digest[:], crypto.SHA256)
			if err != nil {
				m.AddErrors(fmt.Sprintf("Failed to digest message: %s", err))
				c.HTML(http.StatusOK, "siginfo", m)
				return
			}

			m["signature"] = base64.StdEncoding.EncodeToString(sig)
		}

		m.AddSuccess(fmt.Sprintf("Updated signture info #%d", sigInfo.ID))
		c.HTML(http.StatusOK, "siginfo", m)
	}
}

// HandleSignatureInfosShow handles the show action for signature infos.
func (s *Server) HandleSignatureInfosShow() func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		m := TemplateMap{}

		sigInfo := &model.SignatureInfo{}
		if IDParam := c.Param("id"); IDParam == "0" {
			m["new"] = true
		} else {
			sigID, err := strconv.ParseInt(IDParam, 10, 64)
			if err != nil {
				ErrorPage(c, "Unable to parse `id` param.")
				return
			}
			exportDB := database.New(s.env.Database())
			sigInfo, err = exportDB.GetSignatureInfo(ctx, sigID)
			if err != nil {
				ErrorPage(c, "error loading signature info.")
				return
			}
		}

		m["siginfo"] = sigInfo
		c.HTML(http.StatusOK, "siginfo", m)
	}
}

type signatureInfoFormData struct {
	SigningKey        string `form:"signing-key"`
	EndDate           string `form:"end-date"`
	EndTime           string `form:"end-time"`
	SigningKeyID      string `form:"signing-key-id"`
	SigningKeyVersion string `form:"signing-key-version"`
}

func (f *signatureInfoFormData) EndTimestamp() (time.Time, error) {
	return CombineDateAndTime(f.EndDate, f.EndTime)
}

func (f *signatureInfoFormData) PopulateSigInfo(si *model.SignatureInfo) error {
	ts, err := f.EndTimestamp()
	if err != nil {
		return err
	}

	si.SigningKey = project.TrimSpaceAndNonPrintable(f.SigningKey)
	si.SigningKeyVersion = project.TrimSpaceAndNonPrintable(f.SigningKeyVersion)
	si.SigningKeyID = f.SigningKeyID
	si.EndTimestamp = ts
	return nil
}
