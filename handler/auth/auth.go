package auth

import (
	"encoding/json"
	"fmt"
	//"github.com/davecgh/go-spew/spew"
	"github.com/getsentry/raven-go"
	"github.com/gin-gonic/gin"
	"github.com/mkideal/log"
	"github.com/nu7hatch/gouuid"
	"github.com/tokenme/tokenmed/common"
	. "github.com/tokenme/tokenmed/handler"
	"github.com/tokenme/tokenmed/middlewares/jwt"
	telegramUtils "github.com/tokenme/tokenmed/tools/telegram"
	"github.com/tokenme/tokenmed/utils"
	"github.com/ziutek/mymysql/mysql"
	"github.com/tokenme/tokenmed/coins/eth"
	wechatTool "github.com/tokenme/tokenmed/tools/wechat"
    "github.com/silenceper/wechat"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
)

const WX_AUTH_GATEWAY = "https://api.weixin.qq.com/sns/jscode2session?appid=%s&secret=%s&js_code=%s&grant_type=authorization_code"

var AuthenticatorFunc = func(loginInfo jwt.Login, c *gin.Context) (string, bool) {
	db := Service.Db
	var where string
    if loginInfo.WechatMPCode != "" {
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
        resToken, err := oauth.GetUserAccessToken(loginInfo.WechatMPCode)
        if err != nil {
            raven.CaptureError(err, nil)
            log.Error(err.Error())
			return loginInfo.WechatMPCode, false
        }
        userInfo, err := oauth.GetUserInfo(resToken.AccessToken, resToken.OpenID)
        if err != nil {
            raven.CaptureError(err, nil)
            log.Error(err.Error())
			return loginInfo.WechatMPCode, false
        }
		token, err := uuid.NewV4()
		if err != nil {
            raven.CaptureError(err, nil)
			log.Error(err.Error())
			return loginInfo.WechatMPCode, false
		}
		sessionKey := utils.Sha1(fmt.Sprintf("%s%s%s", token, resToken.OpenID, resToken.AccessToken))
		_, _, err = db.Query(`INSERT INTO tokenme.wx_oauth (union_id, appkey, open_id, k, session_key) VALUES ('%s', '%s', '%s', '%s', '%s') ON DUPLICATE KEY UPDATE union_id=VALUES(union_id), k=VALUES(k), session_key=VALUES(session_key)`, db.Escape(userInfo.Unionid), db.Escape(Config.WXMPAppId), db.Escape(userInfo.OpenID), db.Escape(sessionKey), db.Escape(resToken.AccessToken))
		if err != nil {
			raven.CaptureError(err, nil)
			log.Error(err.Error())
			return loginInfo.WechatMPCode, false
		}

	    token, err = uuid.NewV4()
        if err != nil {
			raven.CaptureError(err, nil)
			log.Error(err.Error())
			return loginInfo.WechatMPCode, false
        }
        salt := utils.Sha1(token.String())
        token, err = uuid.NewV4()
        if err != nil {
			raven.CaptureError(err, nil)
			log.Error(err.Error())
			return loginInfo.WechatMPCode, false
        }
        initPassword, err := uuid.NewV4()
        if err != nil {
			raven.CaptureError(err, nil)
			log.Error(err.Error())
			return loginInfo.WechatMPCode, false
        }
        activationCode := utils.Sha1(token.String())
        passwd := utils.Sha1(fmt.Sprintf("%s%s%s", salt, initPassword, salt))

        privateKey, _, err := eth.GenerateAccount()
        if err != nil {
			raven.CaptureError(err, nil)
			log.Error(err.Error())
			return loginInfo.WechatMPCode, false
        }
        walletSalt, wallet, err := utils.AddressEncrypt(privateKey, Config.TokenSalt)
        if err != nil {
			raven.CaptureError(err, nil)
			log.Error(err.Error())
			return loginInfo.WechatMPCode, false
        }

        _, ret, err := db.Query(`INSERT INTO tokenme.users (passwd, salt, activation_code, active, wx_unionid, wx_openid, wx_nick, wx_avatar, wx_gender, wx_city, wx_province, wx_country) VALUES ('%s', '%s', '%s', 1, '%s', '%s', '%s', '%s', %d, '%s', '%s', '%s') ON DUPLICATE KEY UPDATE wx_unionid=VALUES(wx_unionid), wx_openid=VALUES(wx_openid), wx_nick=VALUES(wx_nick), wx_avatar=VALUES(wx_avatar), wx_gender=VALUES(wx_gender), wx_city=VALUES(wx_city), wx_province=VALUES(wx_province), wx_country=VALUES(wx_country)`, db.Escape(passwd), db.Escape(salt), db.Escape(activationCode), db.Escape(userInfo.Unionid), db.Escape(userInfo.OpenID), db.Escape(userInfo.Nickname), db.Escape(userInfo.HeadImgURL), userInfo.Sex, db.Escape(userInfo.City), db.Escape(userInfo.Province), db.Escape(userInfo.Country))
        if err != nil {
			raven.CaptureError(err, nil)
			log.Error(err.Error())
			return loginInfo.WechatMPCode, false
        }
        userId := ret.InsertId()
        if userId > 0 {
            rows, _, err := db.Query(`SELECT 1 FROM tokenme.user_wallets WHERE user_id=%d AND token_type='ETH' LIMIT 1`, userId)
            if err != nil {
                raven.CaptureError(err, nil)
                log.Error(err.Error())
                return loginInfo.WechatMPCode, false
            }
            if len(rows) == 0 {
                _, _, err = db.Query(`INSERT INTO tokenme.user_wallets (user_id, token_type, salt, wallet, name, is_private, is_main) VALUES (%d, 'ETH', '%s', '%s', 'SYS', 1, 1)`, userId, db.Escape(walletSalt), db.Escape(wallet))
                if err != nil {
                    raven.CaptureError(err, nil)
                    log.Error(err.Error())
                    return loginInfo.WechatMPCode, false
                }
            }
        }

		return fmt.Sprintf("#wechat#%s", sessionKey), true
    } else if loginInfo.Wechat != "" {
		resp, err := http.Get(fmt.Sprintf(WX_AUTH_GATEWAY, Config.WXAppId, Config.WXSecret, loginInfo.Wechat))
		if err != nil {
			raven.CaptureError(err, nil)
			return loginInfo.Wechat, false
		}
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			raven.CaptureError(err, nil)
			return loginInfo.Wechat, false
		}
		var oauth common.WechatOAuth
		err = json.Unmarshal(body, &oauth)
		if err != nil {
			raven.CaptureError(err, nil)
			return loginInfo.Wechat, false
		}
		if oauth.OpenId == "" || oauth.SessionKey == "" {
			return loginInfo.Wechat, false
		}
        log.Info("%#v", oauth)
        log.Info(loginInfo.WechatIv)
        log.Info(loginInfo.WechatEncryptedData)

        var wechatUser common.WechatUser
        if loginInfo.WechatIv != "" && loginInfo.WechatEncryptedData != "" {
            wechatUser, err = wechatTool.Decrypt(oauth.SessionKey, loginInfo.WechatIv, loginInfo.WechatEncryptedData)
            if err != nil {
                raven.CaptureError(err, nil)
                return loginInfo.Wechat, false
            }
        } else if wechatUser.UnionId == "" {
            return loginInfo.Wechat, false
        }
        log.Info("%#v", wechatUser)

		token, err := uuid.NewV4()
		if err != nil {
            raven.CaptureError(err, nil)
			log.Error(err.Error())
			return loginInfo.Wechat, false
		}
		sessionKey := utils.Sha1(fmt.Sprintf("%s%s%s", token, wechatUser.OpenId, oauth.SessionKey))
		_, _, err = db.Query(`INSERT INTO tokenme.wx_oauth (union_id, appkey, open_id, k, session_key) VALUES ('%s', '%s', '%s', '%s', '%s') ON DUPLICATE KEY UPDATE union_id=VALUES(union_id), k=VALUES(k), session_key=VALUES(session_key)`, db.Escape(wechatUser.UnionId), db.Escape(Config.WXAppId), db.Escape(oauth.OpenId), db.Escape(sessionKey), db.Escape(oauth.SessionKey))
		if err != nil {
			raven.CaptureError(err, nil)
			log.Error(err.Error())
			return loginInfo.Wechat, false
		}
        if loginInfo.WechatIv == "" || loginInfo.WechatEncryptedData == "" {
		    return fmt.Sprintf("#wechat#%s", sessionKey), true
        }

	    token, err = uuid.NewV4()
        if err != nil {
			raven.CaptureError(err, nil)
			log.Error(err.Error())
			return loginInfo.Wechat, false
        }
        salt := utils.Sha1(token.String())
        token, err = uuid.NewV4()
        if err != nil {
			raven.CaptureError(err, nil)
			log.Error(err.Error())
			return loginInfo.Wechat, false
        }
        initPassword, err := uuid.NewV4()
        if err != nil {
			raven.CaptureError(err, nil)
			log.Error(err.Error())
			return loginInfo.Wechat, false
        }
        activationCode := utils.Sha1(token.String())
        passwd := utils.Sha1(fmt.Sprintf("%s%s%s", salt, initPassword, salt))

        privateKey, _, err := eth.GenerateAccount()
        if err != nil {
			raven.CaptureError(err, nil)
			log.Error(err.Error())
			return loginInfo.Wechat, false
        }
        walletSalt, wallet, err := utils.AddressEncrypt(privateKey, Config.TokenSalt)
        if err != nil {
			raven.CaptureError(err, nil)
			log.Error(err.Error())
			return loginInfo.Wechat, false
        }

        _, ret, err := db.Query(`INSERT INTO tokenme.users (passwd, salt, activation_code, active, wx_unionid, wx_openid, wx_nick, wx_avatar, wx_gender, wx_city, wx_province, wx_country) VALUES ('%s', '%s', '%s', 1, '%s', '%s', '%s', '%s', %d, '%s', '%s', '%s') ON DUPLICATE KEY UPDATE wx_unionid=VALUES(wx_unionid), wx_openid=VALUES(wx_openid), wx_nick=VALUES(wx_nick), wx_avatar=VALUES(wx_avatar), wx_gender=VALUES(wx_gender), wx_city=VALUES(wx_city), wx_province=VALUES(wx_province), wx_country=VALUES(wx_country)`, db.Escape(passwd), db.Escape(salt), db.Escape(activationCode), db.Escape(wechatUser.UnionId), db.Escape(wechatUser.OpenId), db.Escape(wechatUser.Nick), db.Escape(wechatUser.Avatar), wechatUser.Gender, db.Escape(wechatUser.City), db.Escape(wechatUser.Province), db.Escape(wechatUser.Country))
        if err != nil {
			raven.CaptureError(err, nil)
			log.Error(err.Error())
			return loginInfo.Wechat, false
        }
        userId := ret.InsertId()
        if userId > 0 {
            rows, _, err := db.Query(`SELECT 1 FROM tokenme.user_wallets WHERE user_id=%d AND token_type='ETH' LIMIT 1`, userId)
            if err != nil {
                raven.CaptureError(err, nil)
                log.Error(err.Error())
                return loginInfo.Wechat, false
            }
            if len(rows) == 0 {
                _, _, err = db.Query(`INSERT INTO tokenme.user_wallets (user_id, token_type, salt, wallet, name, is_private, is_main) VALUES (%d, 'ETH', '%s', '%s', 'SYS', 1, 1)`, userId, db.Escape(walletSalt), db.Escape(wallet))
                if err != nil {
                    raven.CaptureError(err, nil)
                    log.Error(err.Error())
                    return loginInfo.Wechat, false
                }
            }
        }

		return fmt.Sprintf("#wechat#%s", sessionKey), true
        /*
		resp, err := http.Get(fmt.Sprintf(WX_AUTH_GATEWAY, Config.WXAppId, Config.WXSecret, loginInfo.Wechat))
		if err != nil {
			raven.CaptureError(err, nil)
			return loginInfo.Wechat, false
		}
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			raven.CaptureError(err, nil)
			return loginInfo.Wechat, false
		}
		var oauth common.WechatOAuth
        log.Info("%s", body)
		err = json.Unmarshal(body, &oauth)
		if err != nil {
			raven.CaptureError(err, nil)
			return loginInfo.Wechat, false
		}
        log.Info("%#v", oauth)
		if oauth.OpenId == "" || oauth.SessionKey == "" {
			return loginInfo.Wechat, false
		}
		token, err := uuid.NewV4()
		if err != nil {
			log.Error(err.Error())
			return loginInfo.Wechat, false
		}
		sessionKey := utils.Sha1(fmt.Sprintf("%s%s%s", token, oauth.OpenId, oauth.SessionKey))
		_, _, err = db.Query(`INSERT INTO tokenme.wx_oauth (union_id, appkey, open_id, k, session_key) VALUES ('%s', '%s', '%s', '%s', '%s') ON DUPLICATE KEY UPDATE k=VALUES(k), session_key=VALUES(session_key), union_id=VALUES(union_id), appkey=VALUES(appkey)`, db.Escape(oauth.UnionId), db.Escape(Config.WXAppId), db.Escape(oauth.OpenId), db.Escape(sessionKey), db.Escape(oauth.SessionKey))
		if err != nil {
			raven.CaptureError(err, nil)
			return loginInfo.Wechat, false
		}
		return fmt.Sprintf("#wechat#%s", sessionKey), true
        */
	} else if loginInfo.Telegram != "" {
		if !telegramUtils.TelegramAuthCheck(loginInfo.Telegram, Config.TelegramBotToken) {
			log.Error("Wrong checksum")
			return loginInfo.Username, false
		}
		telegram, err := telegramUtils.ParseTelegramAuth(loginInfo.Telegram)
		if err != nil {
			raven.CaptureError(err, nil)
			return loginInfo.Username, false
		}
		where = fmt.Sprintf("telegram_id=%d", telegram.Id)
	} else if loginInfo.Username != "" && loginInfo.Password != "" {
		arr := strings.Split(loginInfo.Username, ".")
		if len(arr) != 2 {
			return loginInfo.Username, false
		}
		countryCode, err := strconv.ParseUint(arr[0], 10, 64)
		if err != nil || countryCode == 0 {
			return loginInfo.Username, false
		}
		mobile := arr[1]
		where = fmt.Sprintf("country_code=%d AND mobile='%s'", countryCode, db.Escape(mobile))
	} else {
		return loginInfo.Username, false
	}
	query := `SELECT 
                id, 
                country_code,
                mobile,
                email, 
                realname,
                salt, 
                passwd,
                is_admin,
                is_publisher,
                telegram_id,
                telegram_username,
                telegram_firstname,
                telegram_lastname,
                telegram_avatar
            FROM tokenme.users
            WHERE %s
            AND active = 1
            LIMIT 1`
	rows, _, err := db.Query(query, where)
	if err != nil || len(rows) == 0 {
		return loginInfo.Username, false
	}
	row := rows[0]
	user := common.User{
		Id:          row.Uint64(0),
		CountryCode: row.Uint(1),
		Mobile:      row.Str(2),
		Email:       row.Str(3),
		Name:        row.Str(4),
		Salt:        row.Str(5),
		Password:    row.Str(6),
		IsAdmin:     row.Uint(7),
		IsPublisher: row.Uint(8),
	}
	telegramId := row.Int64(9)
	if telegramId > 0 {
		telegram := &common.TelegramUser{
			Id:        telegramId,
			Username:  row.Str(10),
			Firstname: row.Str(11),
			Lastname:  row.Str(12),
			Avatar:    row.Str(13),
		}
		user.Telegram = telegram
	}
	user.ShowName = user.GetShowName()
	user.Avatar = user.GetAvatar(Config.CDNUrl)
	c.Set("USER", user)
	passwdSha1 := utils.Sha1(fmt.Sprintf("%s%s%s", user.Salt, loginInfo.Password, user.Salt))
	return fmt.Sprintf("%d.%s", user.CountryCode, user.Mobile), passwdSha1 == user.Password || loginInfo.Telegram != ""
}

var AuthorizatorFunc = func(username string, c *gin.Context) bool {
	db := Service.Db
	var row mysql.Row
	if strings.HasPrefix(username, "#wechat#") {
		sessionKey := strings.TrimPrefix(username, "#wechat#")
		c.Set("WX_SESSION_KEY", sessionKey)
		query := `SELECT  
                u.id, 
                u.country_code, 
                u.mobile,
                u.email,
                u.realname,
                u.salt, 
                u.passwd,
                u.is_admin,
                u.is_publisher,
                u.telegram_id,
                u.telegram_username,
                u.telegram_firstname,
                u.telegram_lastname,
                u.telegram_avatar,
                u.wx_nick,
                u.wx_avatar
            FROM tokenme.users AS u
            INNER JOIN tokenme.wx_oauth AS wx ON ((u.wx_unionid IS NOT NULL AND wx.union_id IS NOT NULL AND wx.union_id = u.wx_unionid) OR (u.wx_unionid IS NULL AND wx.open_id = u.wx_openid))
            WHERE 
                wx.k='%s'
            AND u.active = 1
            LIMIT 1`
		rows, _, err := db.Query(query, db.Escape(sessionKey))
		if err != nil || len(rows) == 0 {
			if err != nil {
				raven.CaptureError(err, nil)
			}
			return false
		}
		row = rows[0]
	} else {
		query := `SELECT  
                id, 
                country_code, 
                mobile,
                email,
                realname,
                salt, 
                passwd,
                is_admin,
                is_publisher,
                telegram_id,
                telegram_username,
                telegram_firstname,
                telegram_lastname,
                telegram_avatar,
                wx_nick,
                wx_avatar
            FROM tokenme.users
            WHERE 
                country_code=%d
            AND mobile='%s'
            AND active = 1
            LIMIT 1`
		arr := strings.Split(username, ".")
		if len(arr) != 2 {
			return false
		}
		countryCode, err := strconv.ParseUint(arr[0], 10, 64)
		if err != nil || countryCode == 0 {
			return false
		}
		mobile := arr[1]
		rows, _, err := db.Query(query, countryCode, db.Escape(mobile))
		if err != nil || len(rows) == 0 {
			if err != nil {
				raven.CaptureError(err, nil)
			}
			return false
		}
		row = rows[0]
	}

	user := common.User{
		Id:          row.Uint64(0),
		CountryCode: row.Uint(1),
		Mobile:      row.Str(2),
		Email:       row.Str(3),
		Name:        row.Str(4),
		Salt:        row.Str(5),
		Password:    row.Str(6),
		IsAdmin:     row.Uint(7),
		IsPublisher: row.Uint(8),
	}
	telegramId := row.Int64(9)
	if telegramId > 0 {
		telegram := &common.TelegramUser{
			Id:        telegramId,
			Username:  row.Str(10),
			Firstname: row.Str(11),
			Lastname:  row.Str(12),
			Avatar:    row.Str(13),
		}
		user.Telegram = telegram
	}
	wxNick := row.Str(14)
	if wxNick != "" {
		wx := &common.WechatUser{
			Nick:   wxNick,
			Avatar: row.Str(15),
		}
		user.Wechat = wx
	}
	user.ShowName = user.GetShowName()
	user.Avatar = user.GetAvatar(Config.CDNUrl)
	c.Set("USER", user)
	return true
}
