// Copyright 2021 Google LLC
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
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/exposure-notifications-server/internal/project"
	"github.com/google/exposure-notifications-server/internal/serverenv"
	"github.com/google/exposure-notifications-server/pkg/database"
	"github.com/google/exposure-notifications-server/pkg/keys"
)

var testDatabaseInstance *database.TestInstance

func TestMain(m *testing.M) {
	testDatabaseInstance = database.MustTestInstance()
	defer testDatabaseInstance.MustClose()

	m.Run()
}

func newTestServer(t testing.TB) (*serverenv.ServerEnv, *Server) {
	t.Helper()

	ctx := project.TestContext(t)
	testDB, _ := testDatabaseInstance.NewDatabase(t)

	_, self, _, ok := runtime.Caller(1)
	if !ok {
		t.Fatal("failed to get caller")
	}

	localDir := filepath.Join(filepath.Dir(self), "../../local/keys")
	keyConfig := &keys.Config{
		FilesystemRoot: localDir,
	}
	keys, err := keys.NewFilesystem(ctx, keyConfig)
	if err != nil {
		t.Fatalf("unable to init key manager: %v", err)
	}

	env := serverenv.New(ctx,
		serverenv.WithDatabase(testDB),
		serverenv.WithKeyManager(keys))

	config := &Config{}

	server, err := NewServer(config, env)
	if err != nil {
		t.Fatalf("error creating test server: %v", err)
	}

	return env, server
}

// Reflectively serialize the fields in f into form
// fields on the https request, r.
func serializeForm(i interface{}) (url.Values, error) {
	if i == nil {
		return url.Values{}, nil
	}

	v := reflect.ValueOf(i)
	if v.Kind() != reflect.Ptr {
		return nil, fmt.Errorf("provided interface is not a pointer")
	}

	if v.IsNil() {
		return url.Values{}, nil
	}

	e := v.Elem()
	if e.Kind() != reflect.Struct {
		return nil, fmt.Errorf("provided interface is not a struct")
	}

	t := e.Type()

	form := url.Values{}
	for i := 0; i < t.NumField(); i++ {
		ef := e.Field(i)
		tf := t.Field(i)
		tag := tf.Tag.Get("form")

		if ef.Kind() == reflect.Slice || ef.Kind() == reflect.Array {
			for i := 0; i < ef.Len(); i++ {
				form.Add(tag, fmt.Sprintf("%v", ef.Index(i)))
			}
		} else {
			form.Add(tag, fmt.Sprintf("%v", ef))
		}
	}
	return form, nil
}

func newHTTPServer(t testing.TB, method string, path string, handler gin.HandlerFunc) *httptest.Server {
	t.Helper()

	tmpl, err := template.New("").
		Option("missingkey=zero").
		Funcs(TemplateFuncMap).
		ParseFS(templatesFS, "templates/*.html")
	if err != nil {
		t.Fatalf("failed to parse templates from fs: %v", err)
	}

	r := gin.Default()
	r.SetFuncMap(TemplateFuncMap)
	r.SetHTMLTemplate(tmpl)
	switch method {
	case http.MethodGet:
		r.GET(path, handler)
	case http.MethodPost:
		r.POST(path, handler)
	default:
		t.Fatalf("unsupported http method: %v", method)
	}

	return httptest.NewServer(r)
}

func mustFindStrings(t testing.TB, resp *http.Response, want ...string) {
	t.Helper()
	if len(want) == 0 {
		t.Error("not checking for any strings, error in test?")
	}

	defer resp.Body.Close()
	bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("unable to read response: %v", err)
	}

	result := string(bytes)

	for _, wants := range want {
		if !strings.Contains(result, wants) {
			t.Errorf("result missing expected string: %v, got: %v", wants, result)
		}
	}
}

func intPtr(i int) *int {
	if i == 0 {
		return nil
	}
	return &i
}
