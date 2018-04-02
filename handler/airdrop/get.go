package airdrop

import (
	"github.com/gin-gonic/gin"
	"github.com/tokenme/tokenmed/coins/eth"
	"github.com/tokenme/tokenmed/common"
	. "github.com/tokenme/tokenmed/handler"
	"github.com/tokenme/tokenmed/utils"
	"net/http"
)

func GetHandler(c *gin.Context) {
	_, exists := c.Get("USER")
	if Check(!exists, "need login", c) {
		return
	}
	airdropId, err := Uint64NonZero(c.Query("id"), "missing airdrop id")
	if CheckErr(err, c) {
		return
	}
	db := Service.Db
	rows, _, err := db.Query(`SELECT a.id, a.user_id, a.title, a.wallet, a.salt, t.address, t.name, t.symbol, t.decimals, a.gas_price, a.gas_limit, a.commission_fee, a.give_out, a.bonus, a.status, a.balance_status, a.start_date, a.end_date, a.telegram_group, a.inserted, a.updated FROM tokenme.airdrops AS a INNER JOIN tokenme.tokens AS t ON (t.address=a.token_address) WHERE a.id=%d`, airdropId)
	if CheckErr(err, c) {
		return
	}
	var airdrop *common.Airdrop
	if len(rows) > 0 {
		row := rows[0]
		wallet := row.Str(3)
		salt := row.Str(4)
		privateKey, _ := utils.AddressDecrypt(wallet, salt, Config.TokenSalt)
		publicKey, _ := eth.AddressFromHexPrivateKey(privateKey)
		airdrop = &common.Airdrop{
			Id:            row.Uint64(0),
			User:          common.User{Id: row.Uint64(1)},
			Title:         row.Str(2),
			Wallet:        publicKey,
			WalletPrivKey: privateKey,
			Token: common.Token{
				Address:  row.Str(5),
				Name:     row.Str(6),
				Symbol:   row.Str(7),
				Decimals: row.Uint(8),
			},
			GasPrice:      row.Uint64(9),
			GasLimit:      row.Uint64(10),
			CommissionFee: row.Uint64(11),
			GiveOut:       row.Uint64(12),
			Bonus:         row.Uint(13),
			Status:        row.Uint(14),
			BalanceStatus: row.Uint(15),
			StartDate:     row.ForceLocaltime(16),
			EndDate:       row.ForceLocaltime(17),
			TelegramGroup: row.Str(18),
			Inserted:      row.ForceLocaltime(19),
			Updated:       row.ForceLocaltime(20),
			TelegramBot:   Config.TelegramBotName,
		}
		airdrop.CheckBalance(Service.Geth, c)
		_, _, err = db.Query(`UPDATE tokenme.airdrops SET balance_status=%d WHERE id=%d`, airdrop.BalanceStatus, airdrop.Id)
		if CheckErr(err, c) {
			return
		}
	}
	c.JSON(http.StatusOK, airdrop)
}