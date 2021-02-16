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

package keys

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"strings"
	"testing"
)

func TestParseECDSAPublicKey_DecodeError(t *testing.T) {
	t.Parallel()

	_, err := ParseECDSAPublicKey("foo")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestParseECDSAPublicKey_WrongKeyType(t *testing.T) {
	t.Parallel()

	pk, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		t.Fatal(err)
	}

	x509EncodedPub := x509.MarshalPKCS1PublicKey(&pk.PublicKey)
	pemEncodedPub := pem.EncodeToMemory(&pem.Block{Type: "RSA PUBLIC KEY", Bytes: x509EncodedPub})
	pemPublicKey := string(pemEncodedPub)

	_, err = ParseECDSAPublicKey(pemPublicKey)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if want := "x509.ParsePKIXPublicKey"; !strings.Contains(err.Error(), want) {
		t.Fatalf("wrong error, want: %q got: %q", want, err.Error())
	}
}
