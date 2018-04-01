package promotion

import (
	"github.com/gin-gonic/gin"
	. "github.com/tokenme/tokenmed/handler"
	"github.com/tokenme/tokenmed/utils/token"
	"net/http"
	"strings"
)

type SubmitRequest struct {
	Wallet string      `json:"wallet,omitempty"`
	Code   token.Token `json:"verify_code,omitempty"`
}

func SubmitHandler(c *gin.Context) {
	var req SubmitRequest
	if CheckErr(c.Bind(&req), c) {
		return
	}
	if Check(len(req.Wallet) != 42 || !strings.HasPrefix(req.Wallet, "0x"), "invalid wallet", c) {
		return
	}
	db := Service.Db
	_, _, err := db.Query(`UPDATE tokenme.codes SET wallet='%s', status=1 WHERE id=%d AND status=0`, db.Escape(req.Wallet), req.Code)
	if CheckErr(err, c) {
		return
	}

	c.JSON(http.StatusOK, APIResponse{Msg: "ok"})
}
