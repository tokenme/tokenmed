package user

import (
	"github.com/gin-gonic/gin"
	. "github.com/tokenme/tokenmed/handler"
	"net/http"
)

func ActivateHandler(c *gin.Context) {
	email := c.Query("email")
	if Check(email == "", "missing email", c) {
		return
	}

	activationCode := c.Query("activation_code")
	if Check(activationCode == "", "missing activation_code", c) {
		return
	}

	db := Service.Db
	_, ret, err := db.Query(`UPDATE tokenme.users SET active = 1 WHERE email='%s' AND activation_code='%s' AND active = 0 AND created >= DATE_SUB(NOW(), INTERVAL 2 HOUR)`, db.Escape(email), db.Escape(activationCode))
	if CheckErr(err, c) {
		return
	}
	if ret.AffectedRows() == 0 {
		c.JSON(http.StatusOK, APIError{Code: BADREQUEST_ERROR, Msg: "Wrong email or activation code expired"})
	}
	c.JSON(http.StatusOK, APIResponse{Msg: "ok"})
	return
}
