package airdrop

import (
	//"github.com/davecgh/go-spew/spew"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/tokenme/tokenmed/common"
	. "github.com/tokenme/tokenmed/handler"
	"net/http"
	"strings"
)

type UpdateRequest struct {
	Id       uint64 `form:"id" json:"id" binding:"required"`
	GasPrice uint64 `form:"gas_price" json:"gas_price"`
	GasLimit uint64 `form:"gas_limit" json:"gas_limit"`
	GiveOut  uint64 `form:"give_out" json:"give_out"`
	Status   uint   `form:"status" json:"status"`
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
	if Check(user.IsPublisher == 0, "invalid permission", c) {
		return
	}
	var updateFields []string
	if req.GasPrice > 0 {
		updateFields = append(updateFields, fmt.Sprintf("gas_price=%d", req.GasPrice))
	}
	if req.GasLimit > 0 {
		updateFields = append(updateFields, fmt.Sprintf("gas_limit=%d", req.GasLimit))
	}
	if req.GiveOut > 0 {
		updateFields = append(updateFields, fmt.Sprintf("give_out=%d", req.GiveOut))
	}
	updateFields = append(updateFields, fmt.Sprintf("status=%d", req.Status))
	if len(updateFields) == 0 {
		c.JSON(http.StatusOK, APIResponse{Msg: "ok"})
		return
	}
	db := Service.Db
	_, _, err := db.Query(`UPDATE tokenme.airdrops SET %s WHERE id=%d AND user_id=%d LIMIT 1`, strings.Join(updateFields, ","), req.Id, user.Id)
	if CheckErr(err, c) {
		return
	}
	c.JSON(http.StatusOK, APIResponse{Msg: "ok"})
}
