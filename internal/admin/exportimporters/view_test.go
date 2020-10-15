// Copyright 2020 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, softwar
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package exportimporters

import (
	"net/http/httptest"
	"testing"

	"github.com/google/exposure-notifications-server/internal/admin"
	exportimportmodel "github.com/google/exposure-notifications-server/internal/exportimport/model"
)

func TestRenderSignatureInfo(t *testing.T) {
	// Hello developer!
	// If this test fails, it's likely that you changed something in
	//  internal/authorizedapp/model/
	// And whatever you changed is used in the
	//  tools/admin-console/templates/siginfo.html
	// That is what caused the test failure.
	m := admin.TemplateMap{}
	model := new(exportimportmodel.ExportImport)
	m["model"] = model

	recorder := httptest.NewRecorder()
	config := admin.Config{
		TemplatePath: "../../../tools/admin-console/templates",
		TopFile:      "top",
		BotFile:      "bottom",
	}
	err := config.RenderTemplate(recorder, "export-importer", m)
	if err != nil {
		t.Fatalf("error rendering template: %v", err)
	}
}
