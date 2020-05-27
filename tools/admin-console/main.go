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
	"log"

	"github.com/gin-gonic/gin"
	"github.com/google/exposure-notifications-server/internal/admin"
	"github.com/google/exposure-notifications-server/internal/admin/authorizedapps"
	"github.com/google/exposure-notifications-server/internal/admin/exports"
	"github.com/google/exposure-notifications-server/internal/admin/index"
	"github.com/google/exposure-notifications-server/internal/admin/siginfo"
	"github.com/google/exposure-notifications-server/internal/setup"
)

func main() {
	ctx := context.Background()

	var config admin.Config
	env, closer, err := setup.Setup(ctx, &config)
	if err != nil {
		log.Fatalf("setup.Setup: %v", err)
	}
	defer closer()

	router := gin.Default()
	router.LoadHTMLGlob(config.TemplatePath + "/*")

	// Landing page.
	indexController := index.New(&config, env)
	router.GET("/", indexController.Execute)

	// Authorized App Handling.
	authAppController := authorizedapps.NewView(&config, env)
	router.GET("/app", authAppController.Execute)
	saveAppController := authorizedapps.NewSave(&config, env)
	router.POST("/app", saveAppController.Execute)

	// Export Config Handling.
	exportConfigController := exports.NewView(&config, env)
	router.GET("/exports/:id", exportConfigController.Execute)
	saveExportConfigController := exports.NewSave(&config, env)
	router.POST("/exports/:id", saveExportConfigController.Execute)

	// Signature Info.
	sigInfoController := siginfo.NewView(&config, env)
	router.GET("/siginfo/:id", sigInfoController.Execute)
	saveSigInfoController := siginfo.NewSave(&config, env)
	router.POST("/siginfo/:id", saveSigInfoController.Execute)

	log.Printf("listening on http://localhost:" + config.Port)
	router.Run()
}
