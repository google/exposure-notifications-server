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

package metricsware

import (
	"context"

	"github.com/google/exposure-notifications-server/internal/metrics/rotate"
	"go.opencensus.io/stats"
)

func (m Middleware) RecordRevisionKeyCreation(ctx context.Context) {
	(*m.exporter).WriteInt("revision-keys-created", true, 1)
	stats.Record(ctx, rotate.RevisionKeysCreated.M(1))
}

func (m Middleware) RecordRevisionKeyDeletion(ctx context.Context, deletions int) {
	(*m.exporter).WriteInt("revision-keys-deleted", true, deletions)
	stats.Record(ctx, rotate.RevisionKeysDeleted.M(int64(deletions)))
}
