package promotion

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/tokenme/tokenmed/coins/eth"
	"github.com/tokenme/tokenmed/common"
	. "github.com/tokenme/tokenmed/handler"
	"github.com/tokenme/tokenmed/utils"
	"net/http"
)

func GetHandler(c *gin.Context) {
	userContext, exists := c.Get("USER")
	if Check(!exists, "need login", c) {
		return
	}
	user := userContext.(common.User)
	promotionId, err := Uint64NonZero(c.Query("id"), "missing promotion id")
	if CheckErr(err, c) {
		return
	}
	db := Service.Db
	query := `SELECT
	p.id ,
	p.adzone_id ,
	p.channel_id ,
	p.airdrop_id ,
	a.title ,
	a.wallet ,
	a.salt ,
	t.address ,
	t.name ,
	t.symbol ,
	t.decimals ,
	a.give_out ,
	a.bonus, 
	a.status ,
	a.balance_status ,
	a.start_date ,
	a.end_date ,
	a.telegram_group ,
	c.name ,
	az.name
FROM
	tokenme.promotions AS p
INNER JOIN tokenme.airdrops AS a ON ( a.id = p.airdrop_id )
INNER JOIN tokenme.tokens AS t ON ( t.address = a.token_address )
INNER JOIN tokenme.channels AS c ON ( c.id = p.channel_id )
INNER JOIN tokenme.adzones AS az ON ( az.id = p.adzone_id )
WHERE
	p.id = %d
AND p.user_id =%d`
	rows, _, err := db.Query(query, promotionId, user.Id)
	if CheckErr(err, c) {
		return
	}
	if Check(len(rows) == 0, "not found", c) {
		return
	}
	row := rows[0]
	wallet := row.Str(5)
	salt := row.Str(6)
	privateKey, _ := utils.AddressDecrypt(wallet, salt, Config.TokenSalt)
	publicKey, _ := eth.AddressFromHexPrivateKey(privateKey)
	airdrop := &common.Airdrop{
		Id:            row.Uint64(3),
		Title:         row.Str(4),
		Wallet:        publicKey,
		WalletPrivKey: privateKey,
		Token: common.Token{
			Address:  row.Str(7),
			Name:     row.Str(8),
			Symbol:   row.Str(9),
			Decimals: row.Uint(10),
		},
		GiveOut:       row.Uint64(11),
		Bonus:         row.Uint(12),
		Status:        row.Uint(13),
		BalanceStatus: row.Uint(14),
		StartDate:     row.ForceLocaltime(15),
		EndDate:       row.ForceLocaltime(16),
		TelegramGroup: row.Str(17),
	}
	airdrop.CheckBalance(Service.Geth, c)
	promotion := common.Promotion{
		Id:          row.Uint64(0),
		AdzoneId:    row.Uint64(1),
		ChannelId:   row.Uint64(2),
		Airdrop:     airdrop,
		ChannelName: row.Str(18),
		AdzoneName:  row.Str(19),
	}
	promo := common.PromotionProto{
		Id:        promotion.Id,
		UserId:    user.Id,
		AirdropId: promotion.Airdrop.Id,
		AdzoneId:  promotion.AdzoneId,
		ChannelId: promotion.ChannelId,
	}
	promoKey, err := common.EncodePromotion([]byte(Config.LinkSalt), promo)
	if CheckErr(err, c) {
		return
	}
	promotion.Key = promoKey
	promotion.Link = fmt.Sprintf("%s/promo/%s", Config.BaseUrl, promoKey)
	c.JSON(http.StatusOK, promotion)
}
