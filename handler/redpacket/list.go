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
	"sync"
)

const DEFAULT_PAGE_SIZE uint64 = 10

func ListHandler(c *gin.Context) {
	userContext, exists := c.Get("USER")
	if Check(!exists, "need login", c) {
		return
	}
	user := userContext.(common.User)
	page, _ := Uint64Value(c.Query("page"), 1)
	if page == 0 {
		page = 1
	}
	pageSize, _ := Uint64Value(c.Query("page_size"), DEFAULT_PAGE_SIZE)
	if pageSize == 0 {
		pageSize = DEFAULT_PAGE_SIZE
	}
	offset := (page - 1) * pageSize

	db := Service.Db
	rows, _, err := db.Query(`SELECT a.id, a.user_id, a.message, FLOOR(a.total_tokens * 10000), IFNULL(t.address, ''), IFNULL(t.name, 'Ethereum'), IFNULL(t.symbol, 'Ether'), IFNULL(t.decimals, 18), a.recipients, a.expire_time, a.inserted, a.updated, IF(a.expire_time<NOW(), 6, a.status), a.fund_tx, a.fund_tx_status, IFNULL(t.logo, 1) FROM tokenme.red_packets AS a LEFT JOIN tokenme.tokens AS t ON (t.address=a.token_address) WHERE a.user_id=%d ORDER BY a.id DESC LIMIT %d, %d`, user.Id, offset, pageSize)
	if CheckErr(err, c) {
		raven.CaptureError(err, nil)
		return
	}
	var redPackets []*common.RedPacket
	var wg sync.WaitGroup
	for _, row := range rows {
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
		rp := &common.RedPacket{
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
		wg.Add(1)
		go func(rp *common.RedPacket) {
			defer wg.Done()
			if rp.FundTxStatus == 1 {
				receipt, err := ethutils.TransactionReceipt(Service.Geth, c, rp.FundTx)
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
				linkKey, _ := common.EncodeRedPacketLink([]byte(Config.LinkSalt), rp.Id)
				rp.Link = fmt.Sprintf("%s%s", Config.RedPacketShareLink, linkKey)
			}
		}(rp)
		redPackets = append(redPackets, rp)
	}
	wg.Wait()
	c.JSON(http.StatusOK, redPackets)
}
