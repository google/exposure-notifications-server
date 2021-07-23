// Copyright 2020 the Exposure Notifications Server authors
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

package model

import (
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/pkg/errcmp"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/proto"
)

func TestSetJWKS(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input string
		want  *string
	}{
		{
			name:  "empty",
			input: "",
			want:  nil,
		},
		{
			name:  "trim_empty",
			input: "  ",
			want:  nil,
		},
		{
			name:  "valid",
			input: "https://jwks.example.com/",
			want:  proto.String("https://jwks.example.com/"),
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ha := &HealthAuthority{}
			ha.SetJWKS(tc.input)
			if diff := cmp.Diff(tc.want, ha.JwksURI); diff != "" {
				t.Fatalf("mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}

func TestRevoke(t *testing.T) {
	t.Parallel()

	t.Run("existing", func(t *testing.T) {
		t.Parallel()
		hak := &HealthAuthorityKey{}
		hak.From = time.Now().UTC()

		time.Sleep(time.Second)
		hak.Revoke()

		if !hak.Thru.After(hak.From) {
			t.Fatalf("thru (%v) should be set and after from (%v)", hak.Thru, hak.From)
		}
	})

	t.Run("future", func(t *testing.T) {
		t.Parallel()
		hak := &HealthAuthorityKey{}
		hak.From = time.Now().UTC().Add(time.Minute)

		if !hak.IsFuture() {
			t.Fatalf("future dated key not reporting as future")
		}

		hak.Revoke()

		if diff := cmp.Diff(hak.From, hak.Thru); diff != "" {
			t.Fatalf("mismatch (-want, +got):\n%s", diff)
		}
	})
}

func TestIsValid(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	cases := []struct {
		name  string
		from  time.Time
		thru  time.Time
		valid bool
	}{
		{
			name:  "valid now",
			from:  now.Add(-1 * time.Minute),
			thru:  now.Add(1 * time.Minute),
			valid: true,
		},
		{
			name:  "valid no expiration",
			from:  now.Add(-1 * time.Minute),
			thru:  time.Time{},
			valid: true,
		},
		{
			name:  "not valid yet",
			from:  now.Add(1 * time.Minute),
			valid: false,
		},
		{
			name:  "expired",
			from:  now.Add(-2 * time.Minute),
			thru:  now.Add(-1 * time.Second),
			valid: false,
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			hak := HealthAuthorityKey{
				From: tc.from,
				Thru: tc.thru,
			}
			if valid := hak.IsValid(); valid != tc.valid {
				t.Errorf("IsValid: want: %v got: %v", tc.valid, valid)
			}
		})
	}
}

func TestPublicKeyParse(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		pemBlock string
		msg      string
	}{
		{
			name: "valid PEM",
			pemBlock: `-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEA+k9YktDK3UpOhBIy+O17biuwd/g
IBSEEHOdgpAynz0yrHpkWL6vxjNHxRdWcImZxPgL0NVHMdY4TlsL7qaxBQ==
-----END PUBLIC KEY-----`,
		},
		{
			name: "invalid PEM",
			pemBlock: `-----BEGIN PUBLIC KEY-----
totally invalid
-----END PUBLIC KEY-----`,
			msg: "unable to decode PEM block containing PUBLIC KEY",
		},
		{
			name: "wrong key type",
			pemBlock: `-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAvs3MAjWBFJecFLwT4lhd
HxXbn7EaVbx3/JgiXG3Q3PCCxEYQq6SRYp/4qJpZJ2nAW+BoMCxZjTBq8bmby3WT
js5A/G62dLgq5qKRsny6kw2ix3tFXb0I9TsPSUieVmxPgioFF1ytvIU7wKQ07vAZ
HW05DlJJM3E9WhB/ZVKl9NmVp01CcojfhmENPNu65XaAWEMp4txyyX7rU8iPPSsK
QCmoWZQ6r1E1r5+/RumIobbwdYxax3esvC4B3W2jyLFqMJGVBrhWf7tDki/3mCub
NTG3+oqI0Q6a3kPOuAAAupr373j7O1YXrM2KAix966EPwTNlK7YCcJa0m6PKz9DT
6wIDAQAB
-----END PUBLIC KEY-----`,
			msg: "unsupported public key type: *rsa.PublicKey",
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			hak := HealthAuthorityKey{
				PublicKeyPEM: tc.pemBlock,
			}

			k, err := hak.PublicKey()
			errcmp.MustMatch(t, err, tc.msg)
			if err == nil && k == nil {
				t.Errorf("ECDSA public key is unexpectedly nil")
			}
		})
	}
}
