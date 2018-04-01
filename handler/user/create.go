package user

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/nu7hatch/gouuid"
	"github.com/tokenme/tokenmed/coins/eth"
	"github.com/tokenme/tokenmed/common"
	. "github.com/tokenme/tokenmed/handler"
	telegramUtils "github.com/tokenme/tokenmed/tools/telegram"
	"github.com/tokenme/tokenmed/utils"
	"github.com/ziutek/mymysql/mysql"
	"net/http"
	"strings"
)

type CreateRequest struct {
	Mobile      string `form:"mobile" json:"mobile" binding:"required"`
	CountryCode uint   `form:"country_code" json:"country_code" binding:"required"`
	VerifyCode  string `form:"verify_code" json:"verify_code" binding:"required"`
	Password    string `form:"passwd" json:"passwd" binding:"required"`
	RePassword  string `form:"repasswd" json:"repasswd" binding:"required"`
	Telegram    string `form:"telegram" json:"telegram"`
}

func CreateHandler(c *gin.Context) {
	var req CreateRequest
	if CheckErr(c.Bind(&req), c) {
		return
	}
	passwdLength := len(req.Password)
	if Check(passwdLength < 8 || passwdLength > 64, "password length must between 8-32", c) {
		return
	}
	if Check(req.Password != req.RePassword, "repassword!=password", c) {
		return
	}
	token, err := uuid.NewV4()
	if CheckErr(err, c) {
		return
	}
	salt := utils.Sha1(token.String())
	token, err = uuid.NewV4()
	if CheckErr(err, c) {
		return
	}
	activationCode := utils.Sha1(token.String())
	passwd := utils.Sha1(fmt.Sprintf("%s%s%s", salt, req.Password, salt))
	mobile := strings.Replace(req.Mobile, " ", "", 0)
	db := Service.Db
	rows, _, err := db.Query(`SELECT 1 FROM tokenme.auth_verify_codes WHERE country_code=%d AND mobile='%s' AND code='%s' LIMIT 1`, req.CountryCode, db.Escape(mobile), db.Escape(req.VerifyCode))
	if CheckErr(err, c) {
		return
	}
	if Check(len(rows) == 0, "unverified phone number", c) {
		return
	}
	privateKey, _, err := eth.GenerateAccount()
	if CheckErr(err, c) {
		return
	}
	walletSalt, wallet, err := utils.AddressEncrypt(privateKey, Config.TokenSalt)
	if CheckErr(err, c) {
		return
	}
	var telegram common.TelegramUser
	if req.Telegram != "" && telegramUtils.TelegramAuthCheck(req.Telegram, Config.TelegramBotToken) {
		telegram, _ = telegramUtils.ParseTelegramAuth(req.Telegram)
	}
	_, ret, err := db.Query(`INSERT INTO tokenme.users (country_code, mobile, passwd, salt, activation_code, active, telegram_id, telegram_username, telegram_firstname, telegram_lastname, telegram_avatar) VALUES (%d, '%s', '%s', '%s', '%s', 1, %d, '%s', '%s', '%s', '%s')`, req.CountryCode, db.Escape(mobile), db.Escape(passwd), db.Escape(salt), db.Escape(activationCode), telegram.Id, db.Escape(telegram.Username), db.Escape(telegram.Firstname), db.Escape(telegram.Lastname), db.Escape(telegram.Avatar))
	if err != nil && err.(*mysql.Error).Code == mysql.ER_DUP_ENTRY {
		c.JSON(http.StatusOK, APIResponse{Msg: "account already exists"})
		return
	}
	if CheckErr(err, c) {
		return
	}
	userId := ret.InsertId()
	_, _, err = db.Query(`INSERT IGNORE INTO tokenme.user_wallets (user_id, token_type, salt, wallet, name, is_private, is_main) VALUES (%d, 'ETH', '%s', '%s', 'SYS', 1, 1)`, userId, db.Escape(walletSalt), db.Escape(wallet))
	if CheckErr(err, c) {
		return
	}
	if telegram.Id > 0 {
		_, _, err = db.Query(`UPDATE tokenme.red_packet_recipients AS rpr
INNER JOIN tokenme.users AS u ON (u.telegram_id=rpr.telegram_id)
LEFT JOIN tokenme.red_packet_recipients AS rpr2 ON ( 
	rpr2.red_packet_id = rpr.red_packet_id
	AND rpr2.user_id = u.id)
SET rpr.user_id = %d ,
 rpr.country_code = %d ,
 rpr.mobile = '%s'
WHERE
	rpr2.id IS NULL
AND rpr.telegram_id = %d
AND rpr.user_id IS NULL`, userId, req.CountryCode, db.Escape(mobile), telegram.Id)
		if CheckErr(err, c) {
			return
		}
	}
	c.JSON(http.StatusOK, APIResponse{Msg: "ok"})
}
