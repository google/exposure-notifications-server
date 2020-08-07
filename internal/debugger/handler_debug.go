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

package debugger

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	authorizedappdatabase "github.com/google/exposure-notifications-server/internal/authorizedapp/database"
	authorizedappmodel "github.com/google/exposure-notifications-server/internal/authorizedapp/model"
	exportdatabase "github.com/google/exposure-notifications-server/internal/export/database"
	exportmodel "github.com/google/exposure-notifications-server/internal/export/model"
	"github.com/google/exposure-notifications-server/internal/logging"
	"github.com/kelseyhightower/run"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/iam/v1"
)

func (s *Server) handleDebug(ctx context.Context) http.HandlerFunc {
	db := s.env.Database()
	authorizedappDB := authorizedappdatabase.New(db)
	exportDB := exportdatabase.New(db)
	logger := logging.FromContext(ctx)

	services := []string{
		"cleanup-export",
		"cleanup-exposure",
		"export",
		"exposure",
		"federationin",
		"federationout",
		"generate",
		"key-rotation",
	}

	type response struct {
		ServiceEnvironment map[string]map[string]string

		AuthorizedApps []*authorizedappmodel.AuthorizedApp

		ExportConfigs    []*exportmodel.ExportConfig
		ExportBatchEnds  map[int64]*time.Time
		ExportBatchFiles map[int64][]string

		SignatureInfos []*exportmodel.SignatureInfo
	}

	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		errCh := make(chan error, 1)

		var wg sync.WaitGroup
		var ml sync.Mutex

		resp := &response{
			ServiceEnvironment: make(map[string]map[string]string),
			ExportBatchEnds:    make(map[int64]*time.Time),
			ExportBatchFiles:   make(map[int64][]string),
		}

		for _, service := range services {
			service := service

			queue(&wg, errCh, func() error {
				env, err := cloudRunEnv(ctx, service)
				if err != nil {
					return fmt.Errorf("failed to lookup environment for %s: %w", service, err)
				}
				ml.Lock()
				resp.ServiceEnvironment[service] = env
				ml.Unlock()
				return nil
			})
		}

		queue(&wg, errCh, func() error {
			authorizedApps, err := authorizedappDB.ListAuthorizedApps(ctx)
			if err != nil {
				return fmt.Errorf("failed to lookup authorized app configs: %w", err)
			}
			resp.AuthorizedApps = authorizedApps
			return nil
		})

		queue(&wg, errCh, func() error {
			exportConfigs, err := exportDB.GetAllExportConfigs(ctx)
			if err != nil {
				return fmt.Errorf("failed to get all export configs: %w", err)
			}
			resp.ExportConfigs = exportConfigs

			queue(&wg, errCh, func() error {
				for _, ec := range exportConfigs {
					exportBatchFiles, err := exportDB.LookupExportFiles(ctx, ec.ConfigID, 4*time.Hour)
					if err != nil {
						return fmt.Errorf("failed to lookup export files: %w", err)
					}
					resp.ExportBatchFiles[ec.ConfigID] = exportBatchFiles
				}
				return nil
			})
			return nil
		})

		queue(&wg, errCh, func() error {
			exportBatchEnds, err := exportDB.ListLatestExportBatchEnds(ctx)
			if err != nil {
				return fmt.Errorf("failed to get latest export batch ends: %w", err)
			}
			resp.ExportBatchEnds = exportBatchEnds
			return nil
		})

		queue(&wg, errCh, func() error {
			signatureInfos, err := exportDB.ListAllSigntureInfos(ctx)
			if err != nil {
				return fmt.Errorf("failed to list signature infos: %w", err)
			}
			resp.SignatureInfos = signatureInfos
			return nil
		})

		// Wait for all the queues to be done or for an error to occur.
		wgDone := make(chan struct{})
		go func() {
			wg.Wait()
			close(wgDone)
		}()

		select {
		case <-wgDone:
		case err := <-errCh:
			logger.Error(err)
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, http.StatusText(http.StatusInternalServerError))
			return
		}

		// Marshall the json.
		b, err := json.MarshalIndent(resp, "", "  ")
		if err != nil {
			logger.Errorf("failed to marshal json: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, http.StatusText(http.StatusInternalServerError))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, "%s", b)
	}
}

// queue creates a job, adding it to the given wg. Errors are pushed onto the
// provided errCh unless it's closed.
func queue(wg *sync.WaitGroup, errCh chan<- error, f func() error) {
	wg.Add(1)

	go func() {
		defer wg.Done()

		if err := f(); err != nil {
			select {
			case errCh <- err:
			default:
			}
		}
	}()
}

func cloudRunEnv(ctx context.Context, name string) (map[string]string, error) {
	client, err := google.DefaultClient(ctx, iam.CloudPlatformScope)
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}
	client.Timeout = 5 * time.Second

	// Assume the target service is the same project (run caches this).
	project, err := run.ProjectID()
	if err != nil {
		return nil, fmt.Errorf("failed to get project: %w", err)
	}

	// Assume the target service is the same region (run caches this).
	region, err := run.Region()
	if err != nil {
		return nil, fmt.Errorf("failed to get region: %w", err)
	}

	// Lookup service to get revision
	serviceURL := fmt.Sprintf("https://%s-run.googleapis.com/apis/serving.knative.dev/v1/namespaces/%s/services/%s",
		region, project, name)
	serviceResp, err := client.Get(serviceURL)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup service: %w", err)
	}
	defer serviceResp.Body.Close()

	if serviceResp.StatusCode != 200 {
		b, _ := ioutil.ReadAll(serviceResp.Body)
		return nil, fmt.Errorf("failed to lookup service: %d: %s", serviceResp.StatusCode, b)
	}

	var s cloudRunService
	if err := json.NewDecoder(serviceResp.Body).Decode(&s); err != nil {
		return nil, fmt.Errorf("failed to parse service as json: %w", err)
	}

	// Lookup revision to get environment.
	revisionURL := fmt.Sprintf("https://%s-run.googleapis.com/apis/serving.knative.dev/v1/namespaces/%s/revisions/%s",
		region, project, s.Status.Revision)
	revisionResp, err := client.Get(revisionURL)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup revision: %w", err)
	}
	defer revisionResp.Body.Close()

	if revisionResp.StatusCode != 200 {
		b, _ := ioutil.ReadAll(revisionResp.Body)
		return nil, fmt.Errorf("failed to lookup revision: %d: %s", revisionResp.StatusCode, b)
	}

	var r cloudRunRevision
	if err := json.NewDecoder(revisionResp.Body).Decode(&r); err != nil {
		return nil, fmt.Errorf("failed to parse revision as json: %w", err)
	}

	if len(r.Spec.Containers) == 0 {
		return nil, fmt.Errorf("no containers: %#v", r)
	}
	container := r.Spec.Containers[0]

	envvars := make(map[string]string, len(container.Env))
	for _, env := range container.Env {
		envvars[env.Name] = env.Value
	}
	return envvars, nil
}

type cloudRunService struct {
	Status struct {
		Revision string `json:"latestReadyRevisionName"`
	} `json:"status"`
}

type cloudRunRevision struct {
	Spec struct {
		Containers []struct {
			Env []struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			} `json:"env"`
		} `json:"containers"`
	} `json:"spec"`
}
