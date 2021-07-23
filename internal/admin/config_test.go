// Copyright 2021 the Exposure Notifications Server authors
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
	"bytes"
	"strings"
	"testing"
)

func testRenderTemplate(tb testing.TB, name string, p TemplateMap) string {
	tb.Helper()

	cfg := &Config{}
	tmpl, err := cfg.TemplateRenderer()
	if err != nil {
		tb.Fatalf("failed to get template renderer: %v", err)
	}

	var b bytes.Buffer
	if err := tmpl.ExecuteTemplate(&b, name, p); err != nil {
		tb.Fatalf("failed to execute template: %v", err)
	}
	return b.String()
}

func TestConfig_DatabaseConfig(t *testing.T) {
	t.Parallel()

	cfg := &Config{}

	if dbConfig := cfg.DatabaseConfig(); dbConfig == nil {
		t.Errorf("expected DatabaseConfig to not be nil")
	}
}

func TestConfig_KeyManagerConfig(t *testing.T) {
	t.Parallel()

	cfg := &Config{}

	if kmConfig := cfg.KeyManagerConfig(); kmConfig == nil {
		t.Errorf("expected KeyManagerConfig to not be nil")
	}
}

func TestConfig_SecretManagerConfig(t *testing.T) {
	t.Parallel()

	cfg := &Config{}

	if smConfig := cfg.SecretManagerConfig(); smConfig == nil {
		t.Errorf("expected SecretManagerConfig to not be nil")
	}
}

func TestConfig_BlobstoreConfig(t *testing.T) {
	t.Parallel()

	cfg := &Config{}

	if bsConfig := cfg.BlobstoreConfig(); bsConfig == nil {
		t.Errorf("expected BlobstoreConfig to not be nil")
	}
}

func TestConfig_TemplateRenderer(t *testing.T) {
	t.Parallel()

	cfg := &Config{}
	tmpl, err := cfg.TemplateRenderer()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := tmpl.DefinedTemplates(), "top"; !strings.Contains(got, want) {
		t.Errorf("expected %q to contain %q", got, want)
	}
}
