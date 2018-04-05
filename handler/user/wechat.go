package user

import (
	"fmt"
	"github.com/getsentry/raven-go"
	"github.com/gin-gonic/gin"
	"github.com/nu7hatch/gouuid"
	"github.com/tokenme/tokenmed/coins/eth"
	. "github.com/tokenme/tokenmed/handler"
	"github.com/tokenme/tokenmed/tools/wechat"
	"github.com/tokenme/tokenmed/utils"
	"net/http"
	"strconv"
	"strings"
)

type WechatRequest struct {
	OpenId        string `form:"openId", json:"openId"`
	Nick          string `form:"nickName", json:"nickName"`
	Gender        uint   `form:"gender", json:"gender"`
	City          string `form:"city" json:"city"`
	Province      string `form:"province" json:"province"`
	Country       string `form:"country" json:"country"`
	Avatar        string `form:"avatarUrl" json:"avatarUrl"`
	Language      string `form:"language" json:"language"`
	EncryptedData string `from:"encryptedData" json:"encryptedData" binding:"required"`
	Iv            string `from:"iv" json:"iv" binding:"required"`
}

func WechatHandler(c *gin.Context) {
	var req WechatRequest
	if CheckErr(c.Bind(&req), c) {
		return
	}
	wechatKeyContext, exists := c.Get("WX_SESSION_KEY")
	if Check(!exists, "missing wx_session_key", c) {
		return
	}
	wechatKey := strings.TrimPrefix(wechatKeyContext.(string), "#wechat#")
	db := Service.Db
	rows, _, err := db.Query(`SELECT open_id, session_key FROM tokenme.wx_oauth WHERE k='%s' LIMIT 1`, db.Escape(wechatKey))
	if CheckErr(err, c) {
		raven.CaptureError(err, nil)
		return
	}
	if Check(len(rows) == 0, "unauthorized", c) {
		return
	}
	req.OpenId = rows[0].Str(0)
	sessionKey := rows[0].Str(1)
	wechatPhone, err := wechat.Decrypt(sessionKey, req.Iv, req.EncryptedData)
	if CheckErr(err, c) {
		raven.CaptureError(err, nil)
		return
	}
	countryCode, err := strconv.ParseUint(wechatPhone.CountryCode, 10, 64)
	if CheckErr(err, c) {
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
	initPassword, err := uuid.NewV4()
	if CheckErr(err, c) {
		return
	}
	activationCode := utils.Sha1(token.String())
	passwd := utils.Sha1(fmt.Sprintf("%s%s%s", salt, initPassword, salt))

	privateKey, _, err := eth.GenerateAccount()
	if CheckErr(err, c) {
		raven.CaptureError(err, nil)
		return
	}
	walletSalt, wallet, err := utils.AddressEncrypt(privateKey, Config.TokenSalt)
	if CheckErr(err, c) {
		raven.CaptureError(err, nil)
		return
	}
	_, ret, err := db.Query(`INSERT INTO tokenme.users (country_code, mobile, passwd, salt, activation_code, active, wx_openid, wx_nick, wx_avatar, wx_gender, wx_city, wx_province, wx_country, wx_language) VALUES (%d, '%s', '%s', '%s', '%s', 1, '%s', '%s', '%s', %d, '%s', '%s', '%s', '%s') ON DUPLICATE KEY UPDATE wx_openid=VALUES(wx_openid), wx_nick=VALUES(wx_nick), wx_avatar=VALUES(wx_avatar), wx_gender=VALUES(wx_gender), wx_city=VALUES(wx_city), wx_province=VALUES(wx_province), wx_country=VALUES(wx_country), wx_language=VALUES(wx_language)`, countryCode, db.Escape(wechatPhone.PurePhoneNumber), db.Escape(passwd), db.Escape(salt), db.Escape(activationCode), db.Escape(req.OpenId), db.Escape(req.Nick), db.Escape(req.Avatar), req.Gender, db.Escape(req.City), db.Escape(req.Province), db.Escape(req.Country), db.Escape(req.Language))
	if CheckErr(err, c) {
		raven.CaptureError(err, nil)
		return
	}
	userId := ret.InsertId()
	if userId > 0 {
		rows, _, err := db.Query(`SELECT 1 FROM tokenme.user_wallets WHERE user_id=%d AND token_type='ETH' LIMIT 1`, userId)
		if CheckErr(err, c) {
			raven.CaptureError(err, nil)
			return
		}
		if len(rows) == 0 {
			_, _, err = db.Query(`INSERT INTO tokenme.user_wallets (user_id, token_type, salt, wallet, name, is_private, is_main) VALUES (%d, 'ETH', '%s', '%s', 'SYS', 1, 1)`, userId, db.Escape(walletSalt), db.Escape(wallet))
			if CheckErr(err, c) {
				raven.CaptureError(err, nil)
				return
			}
		}
	}
	c.JSON(http.StatusOK, APIResponse{Msg: "ok"})
}
