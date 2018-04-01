package airdrop

import (
	"github.com/gin-gonic/gin"
	"github.com/tokenme/tokenmed/common"
	. "github.com/tokenme/tokenmed/handler"
	"github.com/ziutek/mymysql/mysql"
	"net/http"
)

func PublisherApplyHandler(c *gin.Context) {
	userContext, exists := c.Get("USER")
	if Check(!exists, "need login", c) {
		return
	}
	user := userContext.(common.User)
	db := Service.Db
	_, _, err := db.Query(`INSERT IGNORE INTO tokenme.airdrop_applications (user_id) VALUES (%d)`, user.Id)
	if Check(err != nil && err.(*mysql.Error).Code == mysql.ER_DUP_ENTRY, "Already applied", c) {
		return
	}
	if CheckErr(err, c) {
		return
	}
	c.JSON(http.StatusOK, APIResponse{Msg: "ok"})
}
