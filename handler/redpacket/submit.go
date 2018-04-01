package redpacket

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/tokenme/tokenmed/common"
	. "github.com/tokenme/tokenmed/handler"
	telegramUtils "github.com/tokenme/tokenmed/tools/telegram"
	"github.com/ziutek/mymysql/mysql"
	"net/http"
)

type SubmitRequest struct {
	RedPacketId uint64 `form:"red_packet_id" json:"red_packet_id" binding:"required"`
	Telegram    string `from:"telegram" json:"telegram"`
}

func SubmitHandler(c *gin.Context) {
	var req SubmitRequest
	if CheckErr(c.Bind(&req), c) {
		return
	}
	db := Service.Db

	var (
		user     common.User
		telegram common.TelegramUser
	)
	userContext, exists := c.Get("USER")
	if exists {
		user = userContext.(common.User)
	}
	if user.Id == 0 && req.Telegram != "" && telegramUtils.TelegramAuthCheck(req.Telegram, Config.TelegramBotToken) {
		telegram, _ = telegramUtils.ParseTelegramAuth(req.Telegram)
		rows, _, err := db.Query(`SELECT id, country_code, mobile FROM tokenme.users WHERE telegram_id=%d LIMIT 1`, telegram.Id)
		if CheckErr(err, c) {
			return
		}
		if len(rows) > 0 {
			row := rows[0]
			user.Id = row.Uint64(0)
			user.CountryCode = row.Uint(1)
			user.Mobile = row.Str(2)
		}
	}

	if Check(user.Id == 0 && telegram.Id == 0, "internal error", c) {
		return
	}
	var (
		ret mysql.Result
		err error
	)
	if user.Id > 0 {
		_, ret, err = db.Query(`UPDATE tokenme.red_packet_recipients SET country_code=%d, mobile='%s', user_id=%d, status=2, submitted_time=NOW() WHERE red_packet_id=%d AND status=0 LIMIT 1`, user.CountryCode, user.Mobile, user.Id, req.RedPacketId)
	} else {
		_, ret, err = db.Query(`UPDATE tokenme.red_packet_recipients SET telegram_id=%d, telegram_username='%s', telegram_firstname='%s', telegram_lastname='%s', telegram_avatar='%s', status=2, submitted_time=NOW() WHERE red_packet_id=%d AND status=0 LIMIT 1`, telegram.Id, db.Escape(telegram.Username), db.Escape(telegram.Firstname), db.Escape(telegram.Lastname), db.Escape(telegram.Avatar), req.RedPacketId)
	}

	if err != nil && err.(*mysql.Error).Code == mysql.ER_DUP_ENTRY {
		c.JSON(http.StatusOK, APIResponse{Msg: "submitted"})
		return
	}
	if CheckErr(err, c) {
		return
	}
	if ret.AffectedRows() == 0 {
		c.JSON(http.StatusOK, APIResponse{Msg: "unlucky"})
		return
	}
	var giveOut float64
	if user.Id > 0 {
		rows, _, err := db.Query(`SELECT give_out FROM tokenme.red_packet_recipients WHERE red_packet_id=%d AND user_id=%d LIMIT 1`, req.RedPacketId, user.Id)
		if CheckErr(err, c) {
			return
		}
		giveOut = rows[0].ForceFloat(0)
	} else {
		rows, _, err := db.Query(`SELECT give_out FROM tokenme.red_packet_recipients WHERE red_packet_id=%d AND telegram_id=%d LIMIT 1`, req.RedPacketId, telegram.Id)
		if CheckErr(err, c) {
			return
		}
		giveOut = rows[0].ForceFloat(0)
	}

	c.JSON(http.StatusOK, APIResponse{Msg: fmt.Sprintf("%.4f", giveOut)})
}
