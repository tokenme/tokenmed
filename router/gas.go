package router

import (
	"github.com/gin-gonic/gin"
	"github.com/tokenme/tokenmed/handler/gas"
)

func gasRouter(r *gin.Engine) {
	gasGroup := r.Group("/gas")
	gasGroup.GET("/suggest-price", gas.SuggestPriceHandler)
}
