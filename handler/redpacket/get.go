package redpacket

import (
	"fmt"
	"github.com/getsentry/raven-go"
	"github.com/gin-gonic/gin"
	ethutils "github.com/tokenme/tokenmed/coins/eth/utils"
	"github.com/tokenme/tokenmed/common"
	. "github.com/tokenme/tokenmed/handler"
	"github.com/tokenme/tokenmed/utils"
	"math/big"
	"net/http"
)

func GetHandler(c *gin.Context) {
	userContext, exists := c.Get("USER")
	if Check(!exists, "need login", c) {
		return
	}
	user := userContext.(common.User)

	redPacketId, err := Uint64NonZero(c.Query("id"), "missing repacket id")
	if CheckErr(err, c) {
		return
	}
	db := Service.Db
	rows, _, err := db.Query(`SELECT a.id, a.user_id, a.message, FLOOR(a.total_tokens * 10000), IFNULL(t.address, ''), IFNULL(t.name, 'Ethereum'), IFNULL(t.symbol, 'Ether'), IFNULL(t.decimals, 18), a.recipients, a.expire_time, a.inserted, a.updated, IF(a.expire_time<NOW(), 6, a.status), a.fund_tx, a.fund_tx_status, IFNULL(t.logo, 1) FROM tokenme.red_packets AS a LEFT JOIN tokenme.tokens AS t ON (t.address=a.token_address) WHERE a.id=%d AND a.user_id=%d`, redPacketId, user.Id)
	if CheckErr(err, c) {
		raven.CaptureError(err, nil)
		return
	}
	var rp *common.RedPacket
	if len(rows) > 0 {
		row := rows[0]
		decimals := row.Uint(7)
		token := common.Token{
			Address:  row.Str(4),
			Name:     row.Str(5),
			Symbol:   row.Str(6),
			Decimals: decimals,
			Logo:     row.Uint(15),
		}
		token.LogoAddress = token.GetLogoAddress(Config.CDNUrl)
		totalTokens := new(big.Int).Mul(new(big.Int).SetUint64(row.Uint64(3)), utils.Pow10(int(decimals)-4))
		rp = &common.RedPacket{
			Id:           row.Uint64(0),
			User:         common.User{Id: row.Uint64(1)},
			Message:      row.Str(2),
			TotalTokens:  totalTokens,
			Token:        token,
			Recipients:   row.Uint(8),
			ExpireTime:   row.ForceLocaltime(9),
			Inserted:     row.ForceLocaltime(10),
			Updated:      row.ForceLocaltime(11),
			Status:       row.Uint(12),
			FundTx:       row.Str(13),
			FundTxStatus: row.Uint(14),
		}
		rp.SubmittedRecipients, err = getRedPacketRecipients(rp.Id, rp.Token.Decimals, user.Id, 0)
		if CheckErr(err, c) {
			return
		}
		if rp.FundTxStatus == 1 {
			receipt, _ := ethutils.TransactionReceipt(Service.Geth, c, rp.FundTx)
			if receipt != nil {
				var status uint = 3
				if receipt.Status == 1 {
					status = 2
				}
				_, _, err = db.Query(`UPDATE tokenme.red_packets SET fund_tx_status=%d WHERE id=%d`, status, rp.Id)
				if CheckErr(err, c) {
					raven.CaptureError(err, nil)
					return
				}
				rp.FundTxStatus = status
			}
		}
		if rp.FundTxStatus == 2 || rp.FundTxStatus == 0 {
            var shareLink string
            if IsWeixinBrowser(c) {
                shareLink = Config.RedPacketWechatShareLink
            } else {
                shareLink = Config.RedPacketShareLink
            }
			rp.HashKey, _ = common.EncodeRedPacketLink([]byte(Config.LinkSalt), rp.Id)
			rp.Link = fmt.Sprintf("%s%s", shareLink, rp.HashKey)
			rp.ShortUrl = rp.GetShortUrl(Service)
		}
	}
	c.JSON(http.StatusOK, rp)
}
