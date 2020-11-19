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

package timeutils

import (
	"io/ioutil"
	"sort"
	"strings"
	"unicode"
)

// timezones is a slice of valid timezones.
var timezones []string

// usTimezones is the list of US timezones.
var usTimezones []string

// isAlpha returns true if all characters in the string are letters.
func isAlpha(s string) bool {
	for _, r := range s {
		if !unicode.IsLetter(r) {
			return false
		}
	}
	return true
}

// walkDir walks a timezone directory, adding unique names to the map.
func walkDir(dir, prepend string, zones map[string]string) {
	files, _ := ioutil.ReadDir(dir)
	for _, file := range files {
		if file.Name() != strings.ToUpper(file.Name()[:1])+file.Name()[1:] {
			continue
		}
		if !isAlpha(file.Name()) {
			continue
		}
		name := file.Name()
		if len(prepend) != 0 {
			name = prepend + "/" + name
		}
		if file.IsDir() {
			walkDir(dir+"/"+name, file.Name(), zones)
		} else {
			zones[strings.ToUpper(name)] = name
		}
	}
}

func init() {
	dirs := []string{
		"/usr/share/zoneinfo/",
		"/usr/share/lib/zoneinfo/",
		"/usr/lib/locale/TZ/",
	}

	zones := make(map[string]string)

	for _, dir := range dirs {
		walkDir(dir, "", zones)
	}

	timezones = make([]string, len(zones))
	i := 0
	for _, name := range zones {
		timezones[i] = name
		i++
	}
	sort.Strings(timezones)
	for _, zone := range timezones {
		if strings.HasPrefix(zone, "America") {
			usTimezones = append(usTimezones, zone)
		}
	}
}

// Timezones returns all timezones.
func Timezones() []string {
	return timezones
}

// USTimezones returns the list of US timezones
func USTimezones() []string {
	return usTimezones
}
