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

// Package v1alpha1 contains API definitions that can be used outside of this
// codebase in their alpha form.
// These APIs are not considered to be stable at HEAD.
package v1alpha1

import (
	"sort"

	"github.com/dgrijalva/jwt-go"
)

const (
	ExposureKeyHMACClaim          = "tekmac"
	HealthAuthorityDataClaim      = "phadata"
	TransmissionRiskOverrideClaim = "trisk"
	KeyVersionClaim               = "keyVersion"
)

// TransmissionRiskVector is an additional set of claims that can be
// included in the verification certificatin for a diagnosis as received
// from a trusted public health authority.
type TransmissionRiskVector []TransmissionRiskOverride

// Compile time check that TranismissionRiskVector implements the sort interface.
var _ sort.Interface = TransmissionRiskVector{}

// TransmissionRiskOverride is an indvidual transmission risk override.
type TransmissionRiskOverride struct {
	TranismissionRisk  int `json:"tr"`
	SinceRollingPeriod int `json:"sinceRollingPeriod"`
}

func (a TransmissionRiskVector) Len() int {
	return len(a)
}

// This sorts the TransmissionRiskVector vector with the largest SinceRollingPeriod
// value first. Descending sort.
func (a TransmissionRiskVector) Less(i, j int) bool {
	return a[i].SinceRollingPeriod > a[j].SinceRollingPeriod
}

func (a TransmissionRiskVector) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}

type VerificationClaims struct {
	PHAClaims         map[string]string      `json:"phadata"`
	TransmissionRisks TransmissionRiskVector `json:"trisk"`
	SignedMAC         string                 `json:"tekmac"`
	KeyVersion        string                 `json:"keyVersion"`
	jwt.StandardClaims
}

func NewVerificationClaims() *VerificationClaims {
	return &VerificationClaims{
		PHAClaims:         make(map[string]string),
		TransmissionRisks: []TransmissionRiskOverride{},
	}
}
