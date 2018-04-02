package user

import (
	//"github.com/davecgh/go-spew/spew"
	"fmt"
	"github.com/getsentry/raven-go"
	"github.com/gin-gonic/gin"
	"github.com/tokenme/tokenmed/common"
	. "github.com/tokenme/tokenmed/handler"
	"net/http"
	"strings"
)

type UpdateRequest struct {
	Realname string `form:"realname" json:"realname"`
	Email    string `form:"email" json:"email"`
}

func UpdateHandler(c *gin.Context) {
	var req UpdateRequest
	if CheckErr(c.Bind(&req), c) {
		return
	}
	userContext, exists := c.Get("USER")
	if Check(!exists, "need login", c) {
		return
	}
	user := userContext.(common.User)
	db := Service.Db

	var updateFields []string
	if req.Realname != "" {
		updateFields = append(updateFields, fmt.Sprintf("realname='%s'", db.Escape(req.Realname)))
	}
	if req.Email != "" {
		updateFields = append(updateFields, fmt.Sprintf("email='%s'", db.Escape(req.Email)))
	}
	if len(updateFields) == 0 {
		c.JSON(http.StatusOK, APIResponse{Msg: "ok"})
		return
	}

	_, _, err := db.Query(`UPDATE tokenme.users SET %s WHERE id=%d LIMIT 1`, strings.Join(updateFields, ","), user.Id)
	if CheckErr(err, c) {
		raven.CaptureError(err, nil)
		return
	}
	c.JSON(http.StatusOK, APIResponse{Msg: "ok"})
}
