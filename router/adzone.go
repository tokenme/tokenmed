package router

import (
	"github.com/gin-gonic/gin"
	"github.com/tokenme/tokenmed/handler/adzone"
)

func adzoneRouter(r *gin.Engine) {
	adzoneGroup := r.Group("/adzone")
	adzoneGroup.Use(AuthMiddleware.MiddlewareFunc())
	{
		adzoneGroup.GET("/list", adzone.ListGetHandler)
		adzoneGroup.POST("/add", adzone.AddHandler)
	}
}
