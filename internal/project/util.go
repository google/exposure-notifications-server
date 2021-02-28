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

package project

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
)

// devMode indicates whether the project is running in development mode.
var devMode, _ = strconv.ParseBool(os.Getenv("DEV_MODE"))

// DevMode indicates whether the project is running in development mode.
func DevMode() bool {
	return devMode
}

// root is the path to this file parent module.
var root = func() string {
	_, self, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(self), "..", "..")
}()

// Root returns the filepath to the root of this project.
func Root(more ...string) string {
	if len(more) == 0 {
		return root
	}

	parts := make([]string, 0, len(more)+1)
	parts = append(parts, root)
	parts = append(parts, more...)
	return filepath.Join(parts...)
}
