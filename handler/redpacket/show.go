package redpacket

import (
	"fmt"
	"github.com/garyburd/redigo/redis"
	"github.com/getsentry/raven-go"
	"github.com/gin-gonic/gin"
	cmc "github.com/miguelmota/go-coinmarketcap"
	"github.com/tokenme/tokenmed/common"
	. "github.com/tokenme/tokenmed/handler"
	"github.com/tokenme/tokenmed/tools/ethplorer-api"
	telegramUtils "github.com/tokenme/tokenmed/tools/telegram"
	"github.com/tokenme/tokenmed/utils"
	"math/big"
	"net/http"
	"strings"
	"time"
)

func ShowHandler(c *gin.Context) {
	cryptedKey := c.Param("key")
	if Check(cryptedKey == "", "missing key", c) {
		return
	}
	redPacketId, err := common.DecodeRedPacketLink([]byte(Config.LinkSalt), cryptedKey)
	if CheckErr(err, c) {
		return
	}

	db := Service.Db
	rows, _, err := db.Query(`SELECT a.id, a.user_id, a.message, FLOOR(a.total_tokens * 10000), IFNULL(t.address, ''), IFNULL(t.name, 'ETH'), IFNULL(t.symbol, 'Ether'), IFNULL(t.decimals, 18), a.recipients, a.expire_time, a.inserted, a.updated, IF(a.expire_time<NOW(), 6, a.status), u.id, u.country_code, u.mobile, u.realname, u.email, u.telegram_id, u.telegram_username, u.telegram_firstname, u.telegram_lastname, u.telegram_avatar, IFNULL(t.logo, 1), IFNULL(t.price, 0), u.wx_nick, u.wx_avatar FROM tokenme.red_packets AS a INNER JOIN tokenme.users AS u ON (u.id=a.user_id) LEFT JOIN tokenme.tokens AS t ON (t.address=a.token_address) WHERE a.status > 0 AND a.id=%d`, redPacketId)
	if CheckErr(err, c) {
		raven.CaptureError(err, nil)
		return
	}
	if Check(len(rows) == 0, "missing red packet", c) {
		return
	}
	var rp *common.RedPacket
	row := rows[0]
	redPacketUser := common.User{
		Id:          row.Uint64(13),
		CountryCode: row.Uint(14),
		Mobile:      row.Str(15),
		Name:        row.Str(16),
		Email:       row.Str(17),
		Telegram: &common.TelegramUser{
			Id:        row.Int64(18),
			Username:  row.Str(19),
			Firstname: row.Str(20),
			Lastname:  row.Str(21),
			Avatar:    row.Str(22),
		},
	}
	wxNick := row.Str(25)
	if wxNick != "" {
		wechat := &common.WechatUser{
			Nick:   wxNick,
			Avatar: row.Str(26),
		}
		redPacketUser.Wechat = wechat
	}
	redPacketUser.ShowName = redPacketUser.GetShowName()
	redPacketUser.Avatar = redPacketUser.GetAvatar(Config.CDNUrl)
	decimals := row.Int(7)
	totalTokens := new(big.Int).Mul(new(big.Int).SetUint64(row.Uint64(3)), utils.Pow10(decimals-4))
	rp = &common.RedPacket{
		Id:          row.Uint64(0),
		User:        common.User{Id: redPacketUser.Id, ShowName: redPacketUser.ShowName, Avatar: redPacketUser.Avatar},
		Message:     row.Str(2),
		TotalTokens: totalTokens,
		Recipients:  row.Uint(8),
		ExpireTime:  row.ForceLocaltime(9),
		Inserted:    row.ForceLocaltime(10),
		Updated:     row.ForceLocaltime(11),
		Status:      row.Uint(12),
	}
	linkKey, _ := common.EncodeRedPacketLink([]byte(Config.LinkSalt), rp.Id)
	rp.Link = fmt.Sprintf("%s%s", Config.RedPacketShareLink, linkKey)
	rp.ShortUrl = rp.GetShortUrl(Service)

	if rp.ExpireTime.Before(time.Now()) {
		rp.Status = common.RedPacketStatusExpired
	}
	token := common.Token{
		Address:  row.Str(4),
		Name:     row.Str(5),
		Symbol:   row.Str(6),
		Decimals: uint(decimals),
		Logo:     row.Uint(23),
	}
	token.LogoAddress = token.GetLogoAddress(Config.CDNUrl)
	tokenPrice := row.ForceFloat(24)
	redisMasterConn := Service.Redis.Master.Get()
	defer redisMasterConn.Close()
	if token.Name == "ETH" {
		coinPrice, err := redis.Float64(redisMasterConn.Do("GET", "coinprice-eth"))
		if err != nil || coinPrice == 0 {
			coinPrice, err := cmc.GetCoinPriceUSD("ethereum")
			if err == nil {
				token.Price = &ethplorer.TokenPrice{Currency: "USD", Rate: coinPrice}
				redisMasterConn.Do("SETEX", "coinprice-eth", 60*60, coinPrice)
			}
		} else {
			token.Price = &ethplorer.TokenPrice{Currency: "USD", Rate: coinPrice}
		}
	} else if tokenPrice > 0 {
		token.Price = &ethplorer.TokenPrice{Rate: tokenPrice, Currency: "USD"}
	} else {
		redisKey := fmt.Sprintf("coinprice-%s", token.Address)
		coinPrice, err := redis.Float64(redisMasterConn.Do("GET", redisKey))
		if err != nil || coinPrice == 0 {
			var coinId = token.Name
			coinId = strings.ToLower(coinId)
			coinId = strings.Replace(coinId, " ", "-", 0)
			coinPrice, err = cmc.GetCoinPriceUSD(coinId)
			if err == nil && coinPrice != 0 {
				token.Price = &ethplorer.TokenPrice{Rate: coinPrice, Currency: "USD"}
				redisMasterConn.Do("SETEX", redisKey, 60*60, coinPrice)
			}
		} else {
			token.Price = &ethplorer.TokenPrice{Rate: coinPrice, Currency: "USD"}
		}
	}
	rp.Token = token

	userContext, exists := c.Get("USER")
	var userId uint64
	if exists {
		user := userContext.(common.User)
		userId = user.Id
	}
	var telegram common.TelegramUser
	telegramStr := c.Query("telegram")
	if telegramStr != "" && telegramUtils.TelegramAuthCheck(telegramStr, Config.TelegramBotToken) {
		telegram, _ = telegramUtils.ParseTelegramAuth(telegramStr)
	}
	recipients, err := getRedPacketRecipients(rp.Id, rp.Token.Decimals, userId, telegram.Id)
	if CheckErr(err, c) {
		return
	}
	if len(recipients) >= int(rp.Recipients) {
		rp.SubmittedRecipients = recipients
		if rp.Status == common.RedPacketStatusOk {
			rp.Status = common.RedPacketStatusAllTaken
			_, _, err := db.Query(`UPDATE tokenme.red_packets SET status=2 WHERE id=%d`, rp.Id)
			if CheckErr(err, c) {
				raven.CaptureError(err, nil)
				return
			}
		}
	} else if len(recipients) > 0 && rp.Status == common.RedPacketStatusExpired {
		rp.SubmittedRecipients = recipients
	} else if (userId > 0 || telegram.Id > 0) && len(recipients) > 0 {
		for _, rpr := range recipients {
			if rpr.User.Id == userId || rpr.User.Telegram != nil && rpr.User.Telegram.Id == telegram.Id {
				rp.SubmittedRecipients = recipients
				break
			}
		}
	}
	c.JSON(http.StatusOK, rp)
}

func getRedPacketRecipients(packetId uint64, decimals uint, userId uint64, telegramId int64) ([]common.RedPacketRecipient, error) {
	db := Service.Db
	rows, _, err := db.Query(`SELECT 
				rpr.id,
				FLOOR(rpr.give_out * 10000),
				IF(ISNULL(t.address), 18, t.decimals), 
				IFNULL(t.address, ''),
				IFNULL(u.id, 0), 
				IFNULL(u.country_code, 0),
				IFNULL(u.mobile, 0),
				IFNULL(u.realname, ''),
				IFNULL(u.email, ''),
				IFNULL(u.telegram_id, IFNULL(rpr.telegram_id, 0)),
				IFNULL(u.telegram_username, IFNULL(rpr.telegram_username, '')),
				IFNULL(u.telegram_firstname, IFNULL(rpr.telegram_firstname, '')),
				IFNULL(u.telegram_lastname, IFNULL(rpr.telegram_lastname, '')),
				IFNULL(u.telegram_avatar, IFNULL(rpr.telegram_avatar, '')),
				IFNULL(u.wx_nick, ''),
				IFNULL(u.wx_avatar, ''),
				rp.user_id,
				rpr.status,
				rpr.submitted_time 
			FROM tokenme.red_packet_recipients AS rpr 
			INNER JOIN tokenme.red_packets AS rp ON (rp.id=rpr.red_packet_id) 
			LEFT JOIN users AS u ON (u.id=rpr.user_id) 
			LEFT JOIN tokenme.tokens AS t ON (t.address = rp.token_address)
			WHERE rpr.red_packet_id=%d AND (rpr.user_id>0 OR rpr.telegram_id>0) ORDER BY rpr.submitted_time DESC`, packetId)
	if err != nil {
		raven.CaptureError(err, nil)
		return nil, err
	}
	var recipients []common.RedPacketRecipient
	for _, row := range rows {
		rpUserId := row.Uint64(16)
		user := common.User{
			Id:          row.Uint64(4),
			CountryCode: row.Uint(5),
			Mobile:      row.Str(6),
			Name:        row.Str(7),
			Email:       row.Str(8),
			Telegram: &common.TelegramUser{
				Id:        row.Int64(9),
				Username:  row.Str(10),
				Firstname: row.Str(11),
				Lastname:  row.Str(12),
				Avatar:    row.Str(13),
			},
		}
		wxNick := row.Str(14)
		if wxNick != "" {
			wechat := &common.WechatUser{
				Nick:   wxNick,
				Avatar: row.Str(15),
			}
			user.Wechat = wechat
		}
		user.Avatar = user.GetAvatar(Config.CDNUrl)
		if userId != user.Id && userId != rpUserId && user.Mobile != "" {
			user.Mobile = utils.HideMobile(user.Mobile)
			user.Id = 0
		}
		user.ShowName = user.GetShowName()
		user.Name = ""
		user.Email = ""
		if user.Telegram.Id != 0 {
			user.CountryCode = 0
			user.Mobile = ""
		} else {
			user.Telegram = nil
		}
		val := row.Uint64(1)
		decimals := row.Int(2)
		giveOut := new(big.Int).Mul(new(big.Int).SetUint64(val), utils.Pow10(decimals-4))
		recipients = append(recipients, common.RedPacketRecipient{
			Id:            row.Uint64(0),
			GiveOut:       giveOut,
			User:          user,
			Status:        row.Uint(17),
			Decimals:      uint(decimals),
			SubmittedTime: row.ForceLocaltime(18),
		})
	}
	return recipients, nil
}
