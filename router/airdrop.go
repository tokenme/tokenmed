package router

import (
	"github.com/gin-gonic/gin"
	"github.com/tokenme/tokenmed/handler/airdrop"
)

func airdropRouter(r *gin.Engine) {
	airdropGroup := r.Group("/airdrop")
	airdropGroup.Use(AuthMiddleware.MiddlewareFunc())
	{
		airdropGroup.POST("/add", airdrop.AddHandler)
		airdropGroup.GET("/list", airdrop.ListHandler)
		airdropGroup.GET("/get", airdrop.GetHandler)
		airdropGroup.GET("/stats", airdrop.StatsHandler)
		airdropGroup.POST("/update", airdrop.UpdateHandler)
		airdropGroup.GET("/publisher/apply", airdrop.PublisherApplyHandler)
	}
}
