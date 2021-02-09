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
	"context"
	"io/ioutil"
	"os"
	"testing"
)

// TestKeyManager creates a new key manager suitable for use in tests.
func TestKeyManager(tb testing.TB) KeyManager {
	tb.Helper()

	ctx := context.Background()

	tmpdir, err := ioutil.TempDir("", "")
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() {
		if err := os.RemoveAll(tmpdir); err != nil {
			tb.Fatal(err)
		}
	})

	kms, err := NewFilesystem(ctx, &Config{
		FilesystemRoot: tmpdir,
	})
	if err != nil {
		tb.Fatal(err)
	}

	return kms
}

// TestEncryptionKey creates a new encryption key and key version in the given
// key manager. If the provided key manager does not support managing encryption
// keys, it calls t.Fatal.
func TestEncryptionKey(tb testing.TB, kms KeyManager) string {
	tb.Helper()

	typ, ok := kms.(EncryptionKeyManager)
	if !ok {
		tb.Fatal("kms cannot manage encryption keys")
	}

	ctx := context.Background()
	parent, err := typ.CreateEncryptionKey(ctx, "parent", "key")
	if err != nil {
		tb.Fatal(err)
	}
	if _, err := typ.CreateKeyVersion(ctx, parent); err != nil {
		tb.Fatal(err)
	}

	return parent
}

// TestSigningKey creates a new signing key and key version in the given key
// manager. If the provided key manager does not support managing signing keys,
// it calls t.Fatal.
func TestSigningKey(tb testing.TB, kms KeyManager) string {
	tb.Helper()

	typ, ok := kms.(SigningKeyManager)
	if !ok {
		tb.Fatal("kms cannot manage signing keys")
	}

	ctx := context.Background()
	parent, err := typ.CreateSigningKey(ctx, "parent", "key")
	if err != nil {
		tb.Fatal(err)
	}
	if _, err := typ.CreateKeyVersion(ctx, parent); err != nil {
		tb.Fatal(err)
	}

	return parent
}
