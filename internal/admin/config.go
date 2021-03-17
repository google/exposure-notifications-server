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
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"

	"github.com/google/exposure-notifications-server/internal/setup"
	"github.com/google/exposure-notifications-server/internal/storage"
	"github.com/google/exposure-notifications-server/pkg/database"
	"github.com/google/exposure-notifications-server/pkg/keys"
	"github.com/google/exposure-notifications-server/pkg/secrets"
)

var (
	//go:embed templates/*
	templatesFS embed.FS

	//go:embed assets/*
	assetsFolderFS embed.FS
	assetsFS, _    = fs.Sub(assetsFolderFS, "assets")
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

	Port string `env:"PORT, default=8080"`
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

func (c *Config) TemplateRenderer() (*template.Template, error) {
	tmpl, err := template.New("").
		Option("missingkey=zero").
		Funcs(TemplateFuncMap).
		ParseFS(templatesFS, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("failed to parse templates from fs: %w", err)
	}
	return tmpl, nil
}

func (c *Config) RenderTemplate(w http.ResponseWriter, name string, p TemplateMap) error {
	tmpl, err := c.TemplateRenderer()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	}

	if err := tmpl.ExecuteTemplate(w, name, p); err != nil {
		err = fmt.Errorf("failed to render template %q: %w", name, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	}

	w.WriteHeader(http.StatusOK)
	return nil
}
