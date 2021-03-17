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

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Controller is the interfactor for controllers that can be pluggied into Gin
// for the admin console portion of this project.
type Controller interface {
	Execute(g *gin.Context)
}

func ErrorPage(c *gin.Context, messages ...string) {
	log.Printf("error: %v", messages)
	c.HTML(http.StatusInternalServerError, "error", gin.H{"error": messages})
	c.Abort()
}
