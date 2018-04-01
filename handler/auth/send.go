package auth

import (
	"github.com/gin-gonic/gin"
	. "github.com/tokenme/tokenmed/handler"
	"github.com/tokenme/tokenmed/utils/twilio"
	"net/http"
	"strings"
)

type SendRequest struct {
	Mobile  string `form:"mobile" json:"mobile" binding:"required"`
	Country uint   `form:"country" json:"country" binding:"required"`
}

func SendHandler(c *gin.Context) {
	var req SendRequest
	if CheckErr(c.Bind(&req), c) {
		return
	}
	mobile := strings.Replace(req.Mobile, " ", "", 0)
	ret, err := twilio.AuthSend(Config.TwilioToken, mobile, req.Country)
	if CheckErr(err, c) {
		return
	}
	if Check(!ret.Success, ret.Message, c) {
		return
	}
	c.JSON(http.StatusOK, APIResponse{Msg: "ok"})
}
