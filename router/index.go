package router

import (
	"github.com/danielkov/gin-helmet"
	//"github.com/gin-contrib/static"
	"github.com/dvwright/xss-mw"
	"github.com/gin-gonic/gin"
	"github.com/tokenme/tokenmed/router/static"
)

func NewRouter(uiPath string) *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.Use(helmet.Default())
	xssMdlwr := &xss.XssMw{
		FieldsToSkip: []string{"password", "start_date", "end_date", "token"},
		BmPolicy:     "UGCPolicy",
	}
	r.Use(xssMdlwr.RemoveXss())
	r.Use(static.Serve("/", static.LocalFile(uiPath, 0, true)))
	authRouter(r)
	userRouter(r)
	channelRouter(r)
	adzoneRouter(r)
	airdropRouter(r)
	redPacketRouter(r)
	tokenRouter(r)
	gasRouter(r)
	promotionRouter(r)
	geoIPRouter(r)
	return r
}
