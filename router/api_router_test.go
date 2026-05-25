package router

import (
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSetApiRouterRegistersWithoutPathConflicts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()

	SetApiRouter(engine)
}
