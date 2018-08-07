package auth

import (
	"github.com/getsentry/raven-go"
	"github.com/gin-gonic/gin"
	. "github.com/tokenme/tokenmed/handler"
    "github.com/silenceper/wechat"
	wechatTool "github.com/tokenme/tokenmed/tools/wechat"
    "net/http"
    "net/url"
    "fmt"
)

func WechatMpGetCodeHandler(c *gin.Context) {
    redis := wechatTool.NewRedis(Service.Redis.Master)
    wechatConfig := &wechat.Config{
        AppID:          Config.WXMPAppId,
        AppSecret:      Config.WXMPSecret,
        Token:          Config.WXMPToken,
        EncodingAESKey: Config.WXMPEncodingAESKey,
        Cache:          redis,
    }
    wc := wechat.NewWechat(wechatConfig)
    oauth := wc.GetOauth()
    err := oauth.Redirect(c.Writer, c.Request, fmt.Sprintf("https://%s/auth/wechat-mp/get-user-info", c.Request.Host), "snsapi_userinfo", "")
	if CheckErr(err, c) {
		raven.CaptureError(err, nil)
		return
	}
}

func WechatMpGetUserInfoHandler(c *gin.Context) {
    code := c.Query("code")
    if code != "" {
        c.Redirect(http.StatusFound, fmt.Sprintf("https://%s/rp.html#/guide?code=%s", c.Request.Host, code))
    }
}

func WechatMpGetJs(c *gin.Context) {
    pageUrl := c.Query("url")
    if pageUrl != "" {
        decodeUrl, err := url.QueryUnescape(pageUrl)
        if CheckErr(err, c) {
            raven.CaptureError(err, nil)
            return
        }
        redis := wechatTool.NewRedis(Service.Redis.Master)
        wechatConfig := &wechat.Config{
            AppID:          Config.WXMPAppId,
            AppSecret:      Config.WXMPSecret,
            Token:          Config.WXMPToken,
            EncodingAESKey: Config.WXMPEncodingAESKey,
            Cache:          redis,
        }
        wc := wechat.NewWechat(wechatConfig)
        js := wc.GetJs()
        cfg, err := js.GetConfig(decodeUrl)
        if CheckErr(err, c) {
             raven.CaptureError(err, nil)
             return
        }
        c.JSON(http.StatusOK, cfg)
    }
}
