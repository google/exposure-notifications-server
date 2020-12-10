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

package main

import (
	"path/filepath"
	"sort"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclparse"
)

const (
	alertTF = "../../terraform/alerting/alerts.tf"

	// Ignore files not starting with an upper case letter.
	playbooksGlob = "./alerts/[A-Z]*.md"
)

var alertPolicySchema = hcl.BodySchema{
	Attributes: []hcl.AttributeSchema{
		{Name: "display_name", Required: true},
	},
}

type resource struct {
	Type      string   `hcl:"type,label"`
	Name      string   `hcl:"name,label"`
	Remaining hcl.Body `hcl:",remain"`
}

type config struct {
	Resources []*resource `hcl:"resource,block"`
	Remaining hcl.Body    `hcl:",remain"`
}

func allPlaybookNames(t *testing.T) []string {
	files, err := filepath.Glob(playbooksGlob)
	if err != nil {
		t.Fatalf("failed to find all playbooks: %s", err)
	}
	var ret []string
	for _, x := range files {
		basename := filepath.Base(x)
		ret = append(ret, basename[:len(basename)-len(".md")])
	}
	return ret
}

func allAlertNames(t *testing.T) []string {
	hcl, diag := hclparse.NewParser().ParseHCLFile(alertTF)
	if diag.HasErrors() {
		t.Fatalf("failed to parse alerts.tf: %v", diag)
	}
	var c config
	diag = gohcl.DecodeBody(hcl.Body, nil, &c)
	if diag.HasErrors() {
		t.Fatalf("failed to decode body: %v", diag)
	}
	var ret []string
	for _, res := range c.Resources {
		if res.Type != "google_monitoring_alert_policy" {
			continue
		}
		content, _, diag := res.Remaining.PartialContent(&alertPolicySchema)
		if diag.HasErrors() {
			t.Fatalf("failed to get attributes from block body: %v", diag)
		}
		v, diag := content.Attributes["display_name"].Expr.Value(nil)
		if diag.HasErrors() {
			t.Fatalf("failed to evaluate value of display_name in an alert policy block: %v", diag)
		}
		ret = append(ret, v.AsString())
	}
	return ret
}

func setDiff(xs, ys []string) []string {
	s1 := make(map[string]bool)
	s2 := make(map[string]bool)
	for _, s := range xs {
		s1[s] = true
	}
	for _, s := range ys {
		s2[s] = true
	}
	for k := range s1 {
		if s2[k] {
			delete(s1, k)
		}
	}
	var ret []string
	for k := range s1 {
		ret = append(ret, k)
	}
	sort.Strings(ret)
	return ret
}

// Test to ensure every playbook has a corresponding alert, and every alert has
// a corresponding laybook.
func TestPlaybooks(t *testing.T) {
	playbooks := allPlaybookNames(t)
	alerts := allAlertNames(t)
	for _, x := range setDiff(playbooks, alerts) {
		t.Errorf("Missing alert for playbook %q.", x)
	}
	for _, x := range setDiff(alerts, playbooks) {
		t.Errorf("Missing playbook for alert %q.", x)
	}
}
