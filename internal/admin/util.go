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

package admin

type TemplateMap map[string]interface{}

func (t TemplateMap) AddTitle(title string) {
	t["title"] = title
}

func (t TemplateMap) AddJumbotron(headline, subheader string) {
	t["jumbotron"] = headline
	if subheader != "" {
		t["jumbotronsub"] = subheader
	}
}

func (t TemplateMap) AddSubNav(name string) {
	t["subnav"] = name
}

func (t TemplateMap) AddErrors(errors ...string) {
	t["error"] = errors
}

func (t TemplateMap) AddSuccess(success ...string) {
	t["success"] = success
}
