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

var _ setup.DatabaseConfigProvider = (*Config)(nil)
var _ setup.KeyManagerConfigProvider = (*Config)(nil)
var _ setup.SecretManagerConfigProvider = (*Config)(nil)
var _ setup.BlobstoreConfigProvider = (*Config)(nil)

type Config struct {
	Database      *database.Config
	KeyManager    *signing.Config
	SecretManager *secrets.Config
	Storage       *storage.Config

	Port         string `envconfig:"PORT" default:"8080"`
	TemplatePath string `envconfig:"TEMPLATE_DIR" default:"./tools/admin-console/templates"`
	TopFile      string `envconfig:"TOP_FILE" default:"top"`
	BotFile      string `envconfig:"BOTTOM_FILE" default:"bottom"`
}

func (c *Config) DatabaseConfig() *database.Config {
	return c.Database
}

func (c *Config) KeyManagerConfig() *signing.Config {
	return c.KeyManager
}

func (c *Config) SecretManagerConfig() *secrets.Config {
	return c.SecretManager
}

func (c *Config) BlobstoreConfig() *storage.Config {
	return c.Storage
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
		w.Write([]byte(err.Error()))
		log.Printf("ERROR: %v", err)
		return err
	}
	if err := t.ExecuteTemplate(w, tmpl, p); err != nil {
		message := fmt.Sprintf("error rendering template: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(message))
		log.Printf("ERROR: %v", err)
		return fmt.Errorf("error rendering template: %w", err)
	}

	return nil
}
