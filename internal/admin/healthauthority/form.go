// Copyright 2020 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, softwar
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package healthauthority is part of the admin system.
package healthauthority

import (
	"strings"
	"time"

	"github.com/google/exposure-notifications-server/internal/admin"
	"github.com/google/exposure-notifications-server/internal/verification/model"
)

type formData struct {
	Issuer   string `form:"issuer"`
	Audience string `form:"audience"`
	Name     string `form:"name"`
	JwksURI  string `form:"jwks-uri"`
}

func (f *formData) PopulateHealthAuthority(ha *model.HealthAuthority) {
	ha.Issuer = f.Issuer
	ha.Audience = f.Audience
	ha.Name = f.Name
	ha.SetJWKS(f.JwksURI)
}

type keyFormData struct {
	Version  string `form:"version"`
	PEMBlock string `form:"public-key-pem"`
	FromDate string `form:"from-date"`
	FromTime string `form:"from-time"`
	ThruDate string `form:"Thru-date"`
	ThruTime string `form:"thru-time"`
}

func (f *keyFormData) FromTimestamp() (time.Time, error) {
	return admin.CombineDateAndTime(f.FromDate, f.FromTime)
}

func (f *keyFormData) ThruTimestamp() (time.Time, error) {
	return admin.CombineDateAndTime(f.ThruDate, f.ThruTime)
}

func (f *keyFormData) PopulateHealthAuthorityKey(hak *model.HealthAuthorityKey) error {
	fTime, err := f.FromTimestamp()
	if err != nil {
		return err
	}
	tTime, err := f.ThruTimestamp()
	if err != nil {
		return err
	}
	hak.Version = strings.TrimSpace(f.Version)
	hak.From = fTime
	hak.Thru = tTime
	hak.PublicKeyPEM = strings.TrimSpace(f.PEMBlock)

	_, err = hak.PublicKey()
	if err != nil {
		return err
	}

	return nil
}
