package admin

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Controller is the interfactor for controllers that can be pluggied into Gin
// for the admin console portion of this project.
type Controller interface {
	Execute(g *gin.Context)
}

func ErrorPage(c *gin.Context, messages ...string) {
	c.HTML(http.StatusOK, "error", gin.H{"error": messages})
}
