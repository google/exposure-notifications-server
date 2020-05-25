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
	"github.com/google/exposure-notifications-server/internal/setup"
)

var _ setup.DBConfigProvider = (*Config)(nil)

type Config struct {
	Port         string `envconfig:"PORT" default:"8080"`
	TemplatePath string `envconfig:"TEMPLATE_DIR" default:"./tools/admin-console/templates"`
	Database     *database.Config
	TopFile      string `envconfig:"TOP_FILE" default:"top"`
	BotFile      string `envconfig:"BOTTOM_FILE" default:"bottom"`
}

// DB returns the configuration for the databse.
func (c *Config) DB() *database.Config {
	return c.Database
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
