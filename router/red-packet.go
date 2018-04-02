package router

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/tokenme/tokenmed/handler/redpacket"
	"gopkg.in/dgrijalva/jwt-go.v3"
	"net/http"
)

func AuthCheckerFunc() gin.HandlerFunc {
	if err := AuthMiddleware.MiddlewareInit(); err != nil {
		return func(c *gin.Context) {
			c.Next()
			return
		}
	}

	return func(c *gin.Context) {
		token, err := AuthMiddleware.ParseToken(c)

		if err != nil {
			c.Next()
			return
		}

		claims := token.Claims.(jwt.MapClaims)

		id := AuthMiddleware.IdentityHandler(claims)
		c.Set("JWT_PAYLOAD", claims)
		c.Set("userID", id)

		if !AuthMiddleware.Authorizator(id, c) {
			c.Next()
			return
		}

		c.Next()

		return
	}
}

func redPacketRouter(r *gin.Engine) {
	redPacketGroup := r.Group("/red-packet")
	redPacketGroup.Use(AuthMiddleware.MiddlewareFunc())
	{
		redPacketGroup.POST("/add", redpacket.AddHandler)
		redPacketGroup.GET("/list", redpacket.ListHandler)
		redPacketGroup.GET("/get", redpacket.GetHandler)
		redPacketGroup.POST("/deposit", redpacket.DepositHandler)
	}
	redPacketShowGroup := r.Group("/red-packet/show")
	redPacketShowGroup.Use(AuthCheckerFunc())
	redPacketShowGroup.GET("/:key", redpacket.ShowHandler)

	redPacketSubmitGroup := r.Group("/red-packet")
	redPacketSubmitGroup.Use(AuthCheckerFunc())
	redPacketSubmitGroup.POST("/submit", redpacket.SubmitHandler)

	r.GET("/rp/:key", func(c *gin.Context) {
		key := c.Param("key")
		c.Redirect(http.StatusFound, fmt.Sprintf("/rp.html#/%s", key))
	})
}
