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

package keys

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/exposure-notifications-server/internal/project"
)

func TestNewFilesystem(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)

	t.Run("root_empty", func(t *testing.T) {
		t.Parallel()

		if _, err := NewFilesystem(ctx, &Config{
			FilesystemRoot: "",
		}); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("root_relative", func(t *testing.T) {
		t.Parallel()

		t.Cleanup(func() {
			if err := os.RemoveAll("tmp1"); err != nil {
				t.Fatal(err)
			}
		})

		fs, err := NewFilesystem(ctx, &Config{
			FilesystemRoot: "tmp1",
		})
		if err != nil {
			t.Fatal(err)
		}
		fst, ok := fs.(SigningKeyManager)
		if !ok {
			t.Fatal("not SigningKeyManager")
		}

		if _, err := fst.CreateSigningKey(ctx, "foo", "bar"); err != nil {
			t.Fatal(err)
		}

		if _, err := os.Stat("tmp1/foo/bar"); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("root_absolute", func(t *testing.T) {
		t.Parallel()

		tmp, err := ioutil.TempDir("", "")
		if err != nil {
			t.Fatal(err)
		}

		t.Cleanup(func() {
			if err := os.RemoveAll(tmp); err != nil {
				t.Fatal(err)
			}
		})

		fs, err := NewFilesystem(ctx, &Config{
			FilesystemRoot: tmp,
		})
		if err != nil {
			t.Fatal(err)
		}
		fst, ok := fs.(SigningKeyManager)
		if !ok {
			t.Fatal("not SigningKeyManager")
		}

		if _, err := fst.CreateSigningKey(ctx, "foo", "bar"); err != nil {
			t.Fatal(err)
		}

		if _, err := os.Stat(tmp + "/foo/bar"); err != nil {
			t.Fatal(err)
		}
	})
}

func TestFilesystem_NewSigner(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		keyID string
		setup func(string) error
		err   string
	}{
		{
			name:  "error_key_not_exist",
			keyID: "totally_not_valid",
			err:   "failed to read signing key",
		},
		{
			name:  "error_key_not_ecdsa",
			keyID: "banana",
			setup: func(dir string) error {
				pth := filepath.Join(dir, "banana")
				return ioutil.WriteFile(pth, []byte("dafd"), 0o600)
			},
			err: "failed to parse signing key",
		},
		{
			name:  "happy",
			keyID: "apple",
			setup: func(dir string) error {
				pk, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
				if err != nil {
					return err
				}
				b, err := x509.MarshalECPrivateKey(pk)
				if err != nil {
					return err
				}

				pth := filepath.Join(dir, "apple")
				return ioutil.WriteFile(pth, b, 0o600)
			},
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := project.TestContext(t)

			dir, err := ioutil.TempDir("", "")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { os.RemoveAll(dir) })

			if tc.setup != nil {
				if err := tc.setup(dir); err != nil {
					t.Fatal(err)
				}
			}

			fs, err := NewFilesystem(ctx, &Config{
				FilesystemRoot: dir,
			})
			if err != nil {
				t.Fatal(err)
			}

			if _, err := fs.NewSigner(ctx, tc.keyID); err != nil {
				if tc.err == "" {
					t.Fatal(err)
				}

				if !strings.Contains(err.Error(), tc.err) {
					t.Fatalf("expected %#v to contain %#v", err.Error(), tc.err)
				}
			}
		})
	}
}

func TestFilesystem_EncryptDecrypt(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		keyID     string
		plaintext []byte
		aad       []byte
		setup     func(*Filesystem) error
		err       string
	}{
		{
			name:  "error_key_not_exist",
			keyID: "totally_not_valid",
			err:   "failed to list keys",
		},
		{
			name:  "no_key_versions",
			keyID: "banana",
			setup: func(fs *Filesystem) error {
				dir := filepath.Join(fs.root, "banana")
				return os.MkdirAll(dir, 0o700)
			},
			err: "no key versions",
		},
		{
			name:      "happy",
			keyID:     "apple",
			plaintext: []byte("bacon"),
			setup: func(fs *Filesystem) error {
				ctx := project.TestContext(t)
				id, err := fs.CreateEncryptionKey(ctx, "", "apple")
				if err != nil {
					return err
				}
				if _, err := fs.CreateKeyVersion(ctx, id); err != nil {
					return err
				}
				return nil
			},
		},
		{
			name:      "multi",
			keyID:     "apple",
			plaintext: []byte("bacon"),
			setup: func(fs *Filesystem) error {
				ctx := project.TestContext(t)
				id, err := fs.CreateEncryptionKey(ctx, "", "apple")
				if err != nil {
					return err
				}

				for i := 0; i < 3; i++ {
					if _, err := fs.CreateKeyVersion(ctx, id); err != nil {
						return err
					}
				}
				return nil
			},
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := project.TestContext(t)

			dir, err := ioutil.TempDir("", "")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { os.RemoveAll(dir) })

			fs, err := NewFilesystem(ctx, &Config{
				FilesystemRoot: dir,
			})
			if err != nil {
				t.Fatal(err)
			}
			fst, ok := fs.(*Filesystem)
			if !ok {
				t.Fatal("not Filesystem")
			}

			if tc.setup != nil {
				if err := tc.setup(fst); err != nil {
					t.Fatal(err)
				}
			}

			ciphertext, err := fs.Encrypt(ctx, tc.keyID, tc.plaintext, tc.aad)
			if err != nil {
				if tc.err == "" {
					t.Fatal(err)
				}

				if !strings.Contains(err.Error(), tc.err) {
					t.Fatalf("expected %#v to contain %#v", err.Error(), tc.err)
				}
			}

			if len(ciphertext) > 0 {
				// Create another key version - this will ensure our ciphertext -> key
				// version mapping works.
				for i := 0; i < 3; i++ {
					if _, err := fst.CreateKeyVersion(ctx, tc.keyID); err != nil {
						t.Fatal(err)
					}
				}

				plaintext, err := fs.Decrypt(ctx, tc.keyID, ciphertext, tc.aad)
				if err != nil {
					t.Fatal(err)
				}

				if got, want := plaintext, tc.plaintext; !bytes.Equal(got, want) {
					t.Errorf("expected %#v to be %#v", got, want)
				}
			}
		})
	}
}

func TestFilesystem_SigningKeyVersions(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		keyID string
		setup func(*Filesystem) error
		exp   int
		err   string
	}{
		{
			name:  "error_key_not_exist",
			keyID: "totally_not_valid",
			err:   "failed to open metadata",
		},
		{
			name:  "happy",
			keyID: "apple",
			setup: func(fs *Filesystem) error {
				ctx := project.TestContext(t)
				id, err := fs.CreateSigningKey(ctx, "", "apple")
				if err != nil {
					return err
				}
				if _, err := fs.CreateKeyVersion(ctx, id); err != nil {
					return err
				}
				return nil
			},
			exp: 1,
		},
		{
			name:  "multi",
			keyID: "apple",
			setup: func(fs *Filesystem) error {
				ctx := project.TestContext(t)
				id, err := fs.CreateSigningKey(ctx, "", "apple")
				if err != nil {
					return err
				}

				for i := 0; i < 3; i++ {
					if _, err := fs.CreateKeyVersion(ctx, id); err != nil {
						return err
					}
				}
				return nil
			},
			exp: 3,
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := project.TestContext(t)

			dir, err := ioutil.TempDir("", "")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { os.RemoveAll(dir) })

			fs, err := NewFilesystem(ctx, &Config{
				FilesystemRoot: dir,
			})
			if err != nil {
				t.Fatal(err)
			}
			fst, ok := fs.(*Filesystem)
			if !ok {
				t.Fatal("not Filesystem")
			}

			if tc.setup != nil {
				if err := tc.setup(fst); err != nil {
					t.Fatal(err)
				}
			}

			versions, err := fst.SigningKeyVersions(ctx, tc.keyID)
			if err != nil {
				if tc.err == "" {
					t.Fatal(err)
				}

				if !strings.Contains(err.Error(), tc.err) {
					t.Fatalf("expected %#v to contain %#v", err.Error(), tc.err)
				}
			}

			if got, want := len(versions), tc.exp; got != want {
				t.Errorf("expected %d version to be %d", got, want)
			}
		})
	}
}
