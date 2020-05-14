package ios

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dgrijalva/jwt-go"
)

func TestValidateDeviceToken(t *testing.T) {
	teamID := "ABCDE1FGHI"
	keyID := "1BC2D3EFG4"
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	srv := testServer(t, teamID, keyID, &privateKey.PublicKey)
	endpoint = srv.URL

	cases := []struct {
		name        string
		deviceToken string
		teamID      string
		keyID       string
		privateKey  *ecdsa.PrivateKey
		err         bool
	}{
		{
			name:        "valid",
			deviceToken: "TOTALLY_VALID",
			teamID:      teamID,
			keyID:       keyID,
			privateKey:  privateKey,
			err:         false,
		},
		{
			name:        "invalid device token",
			deviceToken: "NOPE",
			teamID:      teamID,
			keyID:       keyID,
			privateKey:  privateKey,
			err:         true,
		},
		{
			name:        "bad team ID",
			deviceToken: "TOTALLY_VALID",
			teamID:      teamID + "ABC",
			keyID:       keyID,
			privateKey:  privateKey,
			err:         true,
		},
		{
			name:        "bad key ID",
			deviceToken: "TOTALLY_VALID",
			teamID:      teamID,
			keyID:       keyID + "ABC",
			privateKey:  privateKey,
			err:         true,
		},
		{
			name:        "bad signature",
			deviceToken: "TOTALLY_VALID",
			teamID:      teamID,
			keyID:       keyID,
			privateKey: func() *ecdsa.PrivateKey {
				k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
				if err != nil {
					t.Fatal(err)
				}
				return k
			}(),
			err: true,
		},
		{
			name:        "no team id",
			deviceToken: "TOTALLY_VALID",
			teamID:      "",
			keyID:       keyID,
			privateKey:  privateKey,
			err:         true,
		},
		{
			name:        "no key id",
			deviceToken: "TOTALLY_VALID",
			teamID:      teamID,
			keyID:       "",
			privateKey:  privateKey,
			err:         true,
		},
		{
			name:        "no private key",
			deviceToken: "TOTALLY_VALID",
			teamID:      teamID,
			keyID:       keyID,
			privateKey:  nil,
			err:         true,
		},
	}

	for _, c := range cases {
		c := c

		ctx := context.Background()
		err := ValidateDeviceToken(ctx, c.deviceToken, &VerifyOpts{
			TeamID:     c.teamID,
			KeyID:      c.keyID,
			PrivateKey: c.privateKey,
		})
		if (err != nil) != c.err {
			t.Fatalf("%s: expected no error, got: %v", c.name, err)
		}
	}
}

func testServer(tb testing.TB, teamID, keyID string, pubkey *ecdsa.PublicKey) *httptest.Server {
	tb.Helper()

	handler := func(w http.ResponseWriter, r *http.Request) {
		rawToken := strings.Split(r.Header.Get("Authorization"), " ")[1]

		jwtToken, err := jwt.Parse(rawToken, func(tok *jwt.Token) (interface{}, error) {
			return pubkey, nil
		})
		if err != nil {
			http.Error(w, "Bad Authorization Token", 400)
			return
		}

		if !jwtToken.Valid {
			http.Error(w, "Bad Authorization Token", 400)
			return
		}

		mapClaims := jwtToken.Claims.(jwt.MapClaims)
		if !mapClaims.VerifyIssuer(teamID, true) {
			http.Error(w, "Bad Authorization Token", 400)
			return
		}

		if jwtToken.Header["kid"] != keyID {
			http.Error(w, "Bad Authorization Token", 400)
			return
		}

		var i validateRequest
		if err := json.NewDecoder(r.Body).Decode(&i); err != nil {
			tb.Fatal(err)
		}

		if i.DeviceToken != "TOTALLY_VALID" {
			http.Error(w, "Bad Device Token", 400)
			return
		}

		w.WriteHeader(200)
	}

	server := httptest.NewServer(http.HandlerFunc(handler))
	tb.Cleanup(func() { server.Close() })
	return server
}

func TestParsePrivateKey(t *testing.T) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	derKey, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatal(err)
	}

	pemBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: derKey,
	})

	cases := []struct {
		name string
		pem  string
		err  bool
	}{
		{
			name: "valid",
			pem:  string(pemBytes),
		},
	}

	for _, c := range cases {
		c := c

		key, err := ParsePrivateKey(c.pem)
		if (err != nil) != c.err {
			t.Fatalf("%s: expected no error, got: %v", c.name, err)
		}

		_ = key
	}
}
