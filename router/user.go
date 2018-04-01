package router

import (
	"github.com/gin-gonic/gin"
	"github.com/tokenme/tokenmed/handler/user"
)

func userRouter(r *gin.Engine) {
	userGroup := r.Group("/user")
	userGroup.Use(AuthMiddleware.MiddlewareFunc())
	{
		userGroup.GET("/info", user.InfoGetHandler)
		userGroup.GET("/fund", user.FundGetHandler)
		userGroup.POST("/update", user.UpdateHandler)
		userGroup.POST("/wallet/export-private-key", user.GetWalletPrivateKeyHandler)
	}
	r.GET("/user/activate", user.ActivateHandler)
	r.POST("/user/create", user.CreateHandler)
	r.POST("/user/wechat", AuthCheckerFunc(), user.WechatHandler)
	r.POST("/user/reset-password", user.ResetPasswordHandler)
	r.GET("/user/avatar/:key", user.AvatarGetHandler)
}
