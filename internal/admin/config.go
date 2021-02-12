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

// Package admin provides a small admin UI. Requires connection to the database
// and permissions to access whatever else you might need to access.
package admin

import (
	"fmt"
	"html/template"
	"log"
	"net/http"

	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/setup"
	"github.com/google/exposure-notifications-server/internal/storage"
	"github.com/google/exposure-notifications-server/pkg/keys"
	"github.com/google/exposure-notifications-server/pkg/secrets"
)

var (
	_ setup.BlobstoreConfigProvider     = (*Config)(nil)
	_ setup.DatabaseConfigProvider      = (*Config)(nil)
	_ setup.KeyManagerConfigProvider    = (*Config)(nil)
	_ setup.SecretManagerConfigProvider = (*Config)(nil)
)

type Config struct {
	Database      database.Config
	KeyManager    keys.Config
	SecretManager secrets.Config
	Storage       storage.Config

	Port         string `env:"PORT, default=8080"`
	TemplatePath string `env:"TEMPLATE_DIR, default=./cmd/admin-console/templates"`
	TopFile      string `env:"TOP_FILE, default=top"`
	BotFile      string `env:"BOTTOM_FILE, default=bottom"`
}

func (c *Config) DatabaseConfig() *database.Config {
	return &c.Database
}

func (c *Config) KeyManagerConfig() *keys.Config {
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

	t, err := template.
		New(tmpl).
		Funcs(TemplateFuncMap).
		ParseFiles(files...)
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
