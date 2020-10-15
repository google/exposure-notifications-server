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
package main

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/exposure-notifications-server/internal/admin"
	"github.com/google/exposure-notifications-server/internal/admin/authorizedapps"
	"github.com/google/exposure-notifications-server/internal/admin/exportimporters"
	"github.com/google/exposure-notifications-server/internal/admin/exports"
	"github.com/google/exposure-notifications-server/internal/admin/healthauthority"
	"github.com/google/exposure-notifications-server/internal/admin/index"
	"github.com/google/exposure-notifications-server/internal/admin/siginfo"
	"github.com/google/exposure-notifications-server/internal/setup"
)

func main() {
	ctx := context.Background()

	var config admin.Config
	env, err := setup.Setup(ctx, &config)
	if err != nil {
		log.Fatalf("setup.Setup: %v", err)
	}
	defer env.Close(ctx)

	router := gin.Default()
	router.SetFuncMap(funcMap())
	router.LoadHTMLGlob(config.TemplatePath + "/*")

	// Landing page.
	indexController := index.New(&config, env)
	router.GET("/", indexController.Execute)

	// Authorized App Handling.
	authAppController := authorizedapps.NewView(&config, env)
	router.GET("/app", authAppController.Execute)
	saveAppController := authorizedapps.NewSave(&config, env)
	router.POST("/app", saveAppController.Execute)

	// HealthAuthority[Key] Handling.
	healthAppController := healthauthority.NewView(&config, env)
	router.GET("/healthauthority/:id", healthAppController.Execute)
	saveHAController := healthauthority.NewSave(&config, env)
	router.POST("/healthauthority/:id", saveHAController.Execute)
	healthAuthorityKeyController := healthauthority.NewKeyController(&config, env)
	router.POST("/healthauthoritykey/:id/:action/:version", healthAuthorityKeyController.Execute)

	// Export Config Handling.
	exportConfigController := exports.NewView(&config, env)
	router.GET("/exports/:id", exportConfigController.Execute)
	saveExportConfigController := exports.NewSave(&config, env)
	router.POST("/exports/:id", saveExportConfigController.Execute)

	// Export importer configuration
	exportImporterController := exportimporters.NewView(&config, env)
	router.GET("/export-importers/:id", exportImporterController.Execute)
	saveExportImporterController := exportimporters.NewSave(&config, env)
	router.POST("/export-importers/:id", saveExportImporterController.Execute)

	// Signature Info.
	sigInfoController := siginfo.NewView(&config, env)
	router.GET("/siginfo/:id", sigInfoController.Execute)
	saveSigInfoController := siginfo.NewSave(&config, env)
	router.POST("/siginfo/:id", saveSigInfoController.Execute)

	log.Printf("listening on http://localhost:" + config.Port)
	if err := router.Run(); err != nil {
		log.Fatal(err)
	}
}

func funcMap() template.FuncMap {
	return map[string]interface{}{
		"deref":        deref,
		"htmlDate":     timestampFormatter("2006-01-02"),
		"htmlTime":     timestampFormatter("15:04"),
		"htmlDatetime": timestampFormatter(time.UnixDate),
	}
}

func timestampFormatter(f string) func(i interface{}) (string, error) {
	return func(i interface{}) (string, error) {
		switch t := i.(type) {
		case nil:
			return "", nil
		case time.Time:
			if t.IsZero() {
				return "", nil
			}
			return t.UTC().Format(f), nil
		case *time.Time:
			if t == nil || t.IsZero() {
				return "", nil
			}
			return t.UTC().Format(f), nil
		case string:
			return t, nil
		default:
			return "", fmt.Errorf("unknown type %v", t)
		}
	}
}

func deref(i interface{}) (string, error) {
	switch t := i.(type) {
	case *string:
		if t == nil {
			return "", nil
		}
		return *t, nil
	case *int:
		if t == nil {
			return "0", nil
		}
		return fmt.Sprintf("%d", *t), nil
	case *int8:
		if t == nil {
			return "0", nil
		}
		return fmt.Sprintf("%d", *t), nil
	case *int16:
		if t == nil {
			return "0", nil
		}
		return fmt.Sprintf("%d", *t), nil
	case *int32:
		if t == nil {
			return "0", nil
		}
		return fmt.Sprintf("%d", *t), nil
	case *int64:
		if t == nil {
			return "0", nil
		}
		return fmt.Sprintf("%d", *t), nil
	case *uint:
		if t == nil {
			return "0", nil
		}
		return fmt.Sprintf("%d", *t), nil
	case *uint8:
		if t == nil {
			return "0", nil
		}
		return fmt.Sprintf("%d", *t), nil
	case *uint16:
		if t == nil {
			return "0", nil
		}
		return fmt.Sprintf("%d", *t), nil
	case *uint32:
		if t == nil {
			return "0", nil
		}
		return fmt.Sprintf("%d", *t), nil
	case *uint64:
		if t == nil {
			return "0", nil
		}
		return fmt.Sprintf("%d", *t), nil
	default:
		return "", fmt.Errorf("unknown type %T: %v", t, t)
	}
}
