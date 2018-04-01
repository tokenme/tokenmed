package router

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/tokenme/tokenmed/handler/promotion"
	"net/http"
)

func promotionRouter(r *gin.Engine) {
	promotionGroup := r.Group("/promotion")
	promotionGroup.Use(AuthMiddleware.MiddlewareFunc())
	{
		promotionGroup.POST("/add", promotion.AddHandler)
		promotionGroup.GET("/list", promotion.ListHandler)
		promotionGroup.GET("/get", promotion.GetHandler)
		promotionGroup.GET("/stats", promotion.StatsHandler)
	}
	r.GET("/promotion/wallet", promotion.NewWalletHandler)
	r.GET("/promotion/show/:key", promotion.ShowHandler)
	r.POST("/promotion/submit", promotion.SubmitHandler)
	r.GET("/promo/:key", func(c *gin.Context) {
		key := c.Param("key")
		c.Redirect(http.StatusFound, fmt.Sprintf("/promo.html#/%s", key))
	})
}
