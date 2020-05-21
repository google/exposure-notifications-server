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

package authorizedapp

import (
	"context"
	"errors"

	"github.com/google/exposure-notifications-server/internal/authorizedapp/model"
)

// AppNotFound is the sentinel error returned when AppConfig fails to find an
// app with the given name.
var AppNotFound = errors.New("app not found")

// Provider defines possible AuthorizedApp providers.
type Provider interface {
	// AppConfig returns the application-specific configuration for the given
	// name. An error is returned if the configuration fails to load. An error is
	// returned if no app with the given name is registered in the system.
	AppConfig(ctx context.Context, name string) (*model.AuthorizedApp, error)
}
