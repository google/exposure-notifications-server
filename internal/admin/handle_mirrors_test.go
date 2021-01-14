// Copyright 2021 Google LLC
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

package admin

import (
	"net/http/httptest"
	"testing"

	"github.com/google/exposure-notifications-server/internal/mirror/model"
)

func TestRenderMirrors(t *testing.T) {
	t.Parallel()

	m := TemplateMap{}
	mirror := &model.Mirror{}
	m["mirror"] = mirror

	recorder := httptest.NewRecorder()
	config := Config{
		TemplatePath: "../../cmd/admin-console/templates",
		TopFile:      "top",
		BotFile:      "bottom",
	}
	err := config.RenderTemplate(recorder, "mirror", m)
	if err != nil {
		t.Fatalf("error rendering template: %v", err)
	}
}
