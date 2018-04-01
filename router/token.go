package router

import (
	"github.com/gin-gonic/gin"
	"github.com/tokenme/tokenmed/handler/token"
)

func tokenRouter(r *gin.Engine) {
	tokenGroup := r.Group("/token")
	tokenGroup.GET("/get", token.GetHandler)
	tokenGroup.GET("/graph", token.GraphHandler)
	tokenGroup.GET("/market", token.MarketHandler)
}
