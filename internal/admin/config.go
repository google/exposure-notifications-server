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

// This tool provides a small admin UI. Requires connection to the database
// and permissions to access whatever else you might need to access.
package admin

import (
	"fmt"
	"html/template"
	"log"
	"net/http"

	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/secrets"
	"github.com/google/exposure-notifications-server/internal/setup"
	"github.com/google/exposure-notifications-server/internal/signing"
	"github.com/google/exposure-notifications-server/internal/storage"
)

var _ setup.BlobstoreConfigProvider = (*Config)(nil)
var _ setup.DatabaseConfigProvider = (*Config)(nil)
var _ setup.KeyManagerConfigProvider = (*Config)(nil)
var _ setup.SecretManagerConfigProvider = (*Config)(nil)

type Config struct {
	Database      database.Config
	KeyManager    signing.Config
	SecretManager secrets.Config
	Storage       storage.Config

	Port         string `env:"PORT, default=8080"`
	TemplatePath string `env:"TEMPLATE_DIR, default=./tools/admin-console/templates"`
	TopFile      string `env:"TOP_FILE, default=top"`
	BotFile      string `env:"BOTTOM_FILE, default=bottom"`
}

func (c *Config) DatabaseConfig() *database.Config {
	return &c.Database
}

func (c *Config) KeyManagerConfig() *signing.Config {
	return &c.KeyManager
}

func (c *Config) SecretManagerConfig() *secrets.Config {
	return &c.SecretManager
}

func (c *Config) BlobstoreConfig() *storage.Config {
	return &c.Storage
}

func (c *Config) RenderTemplate(w http.ResponseWriter, tmpl string, p TemplateMap) error {
	files := []string{
		fmt.Sprintf("%s/%s.html", c.TemplatePath, c.TopFile),
		fmt.Sprintf("%s/%s.html", c.TemplatePath, tmpl),
		fmt.Sprintf("%s/%s.html", c.TemplatePath, c.BotFile),
	}

	t, err := template.ParseFiles(files...)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, err)
		log.Printf("ERROR: %v", err)
		return err
	}
	if err := t.ExecuteTemplate(w, tmpl, p); err != nil {
		message := fmt.Sprintf("error rendering template: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, message)
		log.Printf("ERROR: %v", err)
		return fmt.Errorf("error rendering template: %w", err)
	}

	return nil
}

// TestConfigDefaults returns a configuration populated with the default values.
// It should only be used for testing.
func TestConfigDefaults() *Config {
	return &Config{
		Database:      *database.TestConfigDefaults(),
		KeyManager:    *signing.TestConfigDefaults(),
		SecretManager: *secrets.TestConfigDefaults(),
		Storage:       *storage.TestConfigDefaults(),

		Port:         "8080",
		TemplatePath: "./tools/admin-console/templates",
		TopFile:      "top",
		BotFile:      "bottom",
	}
}

// TestConfigValued returns a configuration populated with values that match
// TestConfigValues() It should only be used for testing.
func TestConfigValued() *Config {
	return &Config{
		Database:      *database.TestConfigValued(),
		KeyManager:    *signing.TestConfigValued(),
		SecretManager: *secrets.TestConfigValued(),
		Storage:       *storage.TestConfigValued(),

		Port:         "5555",
		TemplatePath: "/tmp",
		TopFile:      "top2",
		BotFile:      "bottom2",
	}
}

// TestConfigValues returns a list of configuration that corresponds to
// TestConfigValued. It should only be used for testing.
func TestConfigValues() map[string]string {
	m := map[string]string{
		"PORT":         "5555",
		"TEMPLATE_DIR": "/tmp",
		"TOP_FILE":     "top2",
		"BOTTOM_FILE":  "bottom2",
	}

	embedded := []map[string]string{
		database.TestConfigValues(),
		signing.TestConfigValues(),
		secrets.TestConfigValues(),
		storage.TestConfigValues(),
	}
	for _, c := range embedded {
		for k, v := range c {
			m[k] = v
		}
	}

	return m
}

// TestConfigOverridden returns a configuration with non-default values set. It
// should only be used for testing.
func TestConfigOverridden() *Config {
	return &Config{
		Database:      *database.TestConfigOverridden(),
		KeyManager:    *signing.TestConfigOverridden(),
		SecretManager: *secrets.TestConfigOverridden(),
		Storage:       *storage.TestConfigOverridden(),

		Port:         "4444",
		TemplatePath: "/var",
		TopFile:      "top3",
		BotFile:      "bottom3",
	}
}
