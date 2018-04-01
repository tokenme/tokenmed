package redpacket

import (
	//"github.com/davecgh/go-spew/spew"
	"fmt"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
	"github.com/gin-gonic/gin"
	"github.com/mkideal/log"
	"github.com/tokenme/tokenmed/coins/eth"
	ethutils "github.com/tokenme/tokenmed/coins/eth/utils"
	"github.com/tokenme/tokenmed/common"
	. "github.com/tokenme/tokenmed/handler"
	"github.com/tokenme/tokenmed/tools/redpacket"
	"github.com/tokenme/tokenmed/utils"
	"math/big"
	"net/http"
	"strings"
	"time"
)

type AddRequest struct {
	Message      string  `form:"message" json:"message"`
	TokenAddress string  `form:"token_address" json:"token_address"`
	Recipients   uint    `form:"recipients" json:"recipients" binding:"required"`
	TotalTokens  float64 `form:"total_tokens" json:"total_tokens" binding:"required"`
	WalletId     uint64  `form:"wallet_id" json:"wallet_id" binding:"required"`
}

func AddHandler(c *gin.Context) {
	var req AddRequest
	if CheckErr(c.Bind(&req), c) {
		return
	}
	userContext, exists := c.Get("USER")
	if Check(!exists, "need login", c) {
		return
	}
	user := userContext.(common.User)
	var token common.Token
	db := Service.Db
	if req.TokenAddress != "" {
		rows, _, err := db.Query(`SELECT address, name, symbol, decimals, protocol FROM tokenme.tokens WHERE address='%s' LIMIT 1`, db.Escape(req.TokenAddress))
		if CheckErr(err, c) {
			return
		}
		if Check(len(rows) == 0, "missing token", c) {
			return
		}
		token = common.Token{
			Address:  rows[0].Str(0),
			Name:     rows[0].Str(1),
			Symbol:   rows[0].Str(2),
			Decimals: rows[0].Uint(3),
			Protocol: rows[0].Str(4),
		}
	} else {
		token = common.Token{
			Address:  "",
			Name:     "ETH",
			Symbol:   "Ether",
			Decimals: 18,
			Protocol: "ETH",
		}
	}
	if Check(token.Decimals >= 4 && req.TotalTokens < 0.001 || token.Decimals < 4 && req.TotalTokens < 10, "not enough tokens", c) {
		return
	}
	var totalTokens *big.Int
	if token.Decimals >= 4 {
		totalTokens = new(big.Int).Mul(new(big.Int).SetUint64(uint64(req.TotalTokens)), utils.Pow40)
	} else {
		totalTokens = new(big.Int).SetUint64(uint64(req.TotalTokens))
	}
	now := time.Now()
	rp := common.RedPacket{
		User:       common.User{Id: user.Id, Email: user.Email},
		Message:    req.Message,
		Token:      token,
		GasPrice:   Config.RedPacketGasPrice,
		GasLimit:   Config.RedPacketGasLimit,
		Recipients: req.Recipients,
		ExpireTime: now.AddDate(0, 0, 3),
		Inserted:   now,
		Updated:    now,
	}

	if Check(req.WalletId == 0, "missing wallet", c) {
		return
	}
	var (
		redPacketIncome      = big.NewInt(0)
		redPacketOutcomeLeft = big.NewInt(0)
		redPacketCashOutput  = big.NewInt(0)
		tokenCash            *big.Int
	)

	{
		rows, _, err := db.Query(`SELECT
	rp.token_address ,
	SUM(rpr.give_out),
	IF(ISNULL(t.address), 18, t.decimals)
FROM
	tokenme.red_packet_recipients AS rpr
INNER JOIN tokenme.red_packets AS rp ON ( rp.id = rpr.red_packet_id )
LEFT JOIN tokenme.tokens AS t ON ( t.address = rp.token_address )
WHERE
	rpr.user_id = %d
AND rpr.status = 2
AND rp.token_address='%s' GROUP BY rp.token_address`, user.Id, req.TokenAddress)
		if CheckErr(err, c) {
			return
		}
		if len(rows) > 0 {
			giveOut := rows[0].ForceFloat(1)
			decimals := rows[0].Int(2)
			var value uint64
			if decimals >= 4 {
				value = uint64(giveOut * float64(utils.Pow40.Uint64()))
				redPacketIncome = new(big.Int).Mul(new(big.Int).SetUint64(value), utils.Pow10(decimals))
				redPacketIncome = new(big.Int).Div(redPacketIncome, utils.Pow40)
			} else {
				value = uint64(giveOut)
				redPacketIncome = new(big.Int).Mul(new(big.Int).SetUint64(value), utils.Pow10(decimals))
			}
		}

		rows, _, err = db.Query(`SELECT
	rp.token_address ,
	SUM(rp.total_tokens) - SUM(IF(rp.fund_tx_status = 0 AND rp.expire_time >= NOW(), rp.total_tokens, 0)) AS unexpired_outcome_left,
	SUM(IF(rp.fund_tx_status = 0, rp.total_tokens, 0)) AS cash_output,
	IF(ISNULL(t.address), 18, t.decimals) AS decimals
FROM
	tokenme.red_packets AS rp
LEFT JOIN tokenme.tokens AS t ON ( t.address = rp.token_address )
WHERE
	rp.user_id =%d
AND rp.token_address = '%s'
GROUP BY
	rp.token_address`, user.Id, req.TokenAddress)
		if CheckErr(err, c) {
			return
		}
		if len(rows) > 0 {
			outcomeLeft := rows[0].ForceFloat(1)
			cashOutput := rows[0].ForceFloat(2)
			decimals := rows[0].Int(3)
			if decimals >= 4 {
				leftVal := uint64(outcomeLeft * float64(utils.Pow40.Uint64()))
				cashVal := uint64(cashOutput * float64(utils.Pow40.Uint64()))
				redPacketOutcomeLeft = new(big.Int).Mul(new(big.Int).SetUint64(leftVal), utils.Pow10(decimals))
				redPacketOutcomeLeft = new(big.Int).Div(redPacketOutcomeLeft, utils.Pow40)
				redPacketCashOutput = new(big.Int).Mul(new(big.Int).SetUint64(cashVal), utils.Pow10(decimals))
				redPacketCashOutput = new(big.Int).Div(redPacketCashOutput, utils.Pow40)
			} else {
				leftVal := uint64(outcomeLeft)
				cashVal := uint64(cashOutput)
				redPacketOutcomeLeft = new(big.Int).Mul(new(big.Int).SetUint64(leftVal), utils.Pow10(decimals))
				redPacketCashOutput = new(big.Int).Mul(new(big.Int).SetUint64(cashVal), utils.Pow10(decimals))
			}
		}

		rows, _, err = db.Query(`SELECT
	rp.token_address,
	SUM(IFNULL(rpr.give_out, 0)) AS expired_outcome,
	IF(ISNULL(t.address), 18, t.decimals) AS decimals
FROM
	tokenme.red_packet_recipients AS rpr
INNER JOIN tokenme.red_packets AS rp ON ( rp.id = rpr.red_packet_id )
LEFT JOIN tokenme.tokens AS t ON ( t.address = rp.token_address )
WHERE
	rp.user_id=%d
AND rp.token_address = '%s'
AND rpr.status!=0
AND rp.expire_time < NOW()
GROUP BY
	rp.token_address`, user.Id, req.TokenAddress)
		if CheckErr(err, c) {
			return
		}
		if len(rows) > 0 {
			val := rows[0].ForceFloat(1)
			decimals := rows[0].Int(2)
			var (
				expiredOutcome *big.Int
				value          uint64
			)
			if decimals >= 4 {
				value = uint64(val * float64(utils.Pow40.Uint64()))
				expiredOutcome = new(big.Int).Mul(new(big.Int).SetUint64(value), utils.Pow10(decimals))
				expiredOutcome = new(big.Int).Div(expiredOutcome, utils.Pow40)
			} else {
				value = uint64(val)
				expiredOutcome = new(big.Int).Mul(new(big.Int).SetUint64(value), utils.Pow10(decimals))
			}
			redPacketOutcomeLeft = new(big.Int).Sub(redPacketOutcomeLeft, expiredOutcome)
		}
		tokenCash = new(big.Int).Add(redPacketIncome, redPacketOutcomeLeft)
		if tokenCash.Cmp(redPacketCashOutput) == -1 {
			tokenCash = big.NewInt(0)
		} else {
			tokenCash = new(big.Int).Sub(tokenCash, redPacketCashOutput)
		}
	}

	query := `SELECT
            uw.wallet,
            uw.salt,
            uw.is_private
        FROM tokenme.user_wallets AS uw
        WHERE uw.user_id=%d AND id=%d`
	rows, _, err := db.Query(query, user.Id, req.WalletId)
	if CheckErr(err, c) {
		return
	}
	row := rows[0]
	walletAddress := row.Str(0)
	walletSalt := row.Str(1)
	isPrivate := row.Uint(2)
	if Check(isPrivate != 1, "this wallet can't create red packet", c) {
		return
	}
	walletPrivateKey, _ := utils.AddressDecrypt(walletAddress, walletSalt, Config.TokenSalt)
	walletPublicKey, _ := eth.AddressFromHexPrivateKey(walletPrivateKey)
	if tokenCash.Cmp(totalTokens) == -1 { // NOT ENOUGH CASH need to use Wallet balannce
		ethBalance, err := eth.BalanceOf(Service.Geth, c, walletPublicKey)
		if CheckErr(err, c) {
			return
		}
		minGasLimit := new(big.Int).SetUint64(rp.GasPrice * rp.GasLimit)
		var minETHGwei *big.Int
		if req.TokenAddress == "" {
			minETHGwei = new(big.Int).Add(minGasLimit, totalTokens)
		} else {
			minETHGwei = minGasLimit
		}
		minETHWei := new(big.Int).Mul(minETHGwei, big.NewInt(params.Shannon))
		if ethBalance.Cmp(minETHWei) == -1 {
			c.JSON(http.StatusOK, APIError{Code: 502, Msg: fmt.Sprintf("%d", minETHGwei.Uint64())})
			return
		}
		if req.TokenAddress != "" {
			tokenBalance, err := ethutils.BalanceOfToken(Service.Geth, rp.Token.Address, walletPublicKey)
			if CheckErr(err, c) {
				return
			}
			var tokenValue *big.Int
			if rp.Token.Decimals >= 4 {
				tokenValue = new(big.Int).Mul(totalTokens, utils.Pow10(int(rp.Token.Decimals)))
				tokenValue = new(big.Int).Div(tokenValue, utils.Pow40)
			} else {
				tokenValue = new(big.Int).Mul(totalTokens, utils.Pow10(int(rp.Token.Decimals)))
			}
			if tokenBalance.Cmp(tokenValue) == -1 {
				c.JSON(http.StatusOK, APIError{Code: 503, Msg: fmt.Sprintf("%d", rp.TotalTokens)})
				return
			}
		}

		transactor := eth.TransactorAccount(walletPrivateKey)
		nonce, err := eth.PendingNonce(Service.Geth, c, walletPublicKey)
		if CheckErr(err, c) {
			return
		}
		var tx *types.Transaction
		gasPrice := new(big.Int).Mul(new(big.Int).SetUint64(rp.GasPrice), big.NewInt(params.Shannon))
		if req.TokenAddress != "" {
			transactorOpts := eth.TransactorOptions{
				Nonce:    nonce,
				GasPrice: gasPrice,
				GasLimit: rp.GasLimit,
			}
			eth.TransactorUpdate(transactor, transactorOpts, c)
			tokenHandler, err := ethutils.NewToken(rp.Token.Address, Service.Geth)
			if CheckErr(err, c) {
				return
			}
			tx, err = ethutils.Transfer(tokenHandler, transactor, Config.RedPacketIncomeWallet, totalTokens)
			if CheckErr(err, c) {
				return
			}
		} else {
			value := new(big.Int).Mul(totalTokens, big.NewInt(params.Shannon))
			transactorOpts := eth.TransactorOptions{
				Nonce:    nonce,
				Value:    value,
				GasPrice: gasPrice,
				GasLimit: rp.GasLimit,
			}
			eth.TransactorUpdate(transactor, transactorOpts, c)
			tx, err = eth.Transfer(transactor, Service.Geth, c, Config.RedPacketIncomeWallet)
			if CheckErr(err, c) {
				return
			}
		}
		txHash := tx.Hash()
		rp.FundTx = txHash.Hex()
		rp.FundTxStatus = 1
		_, _, err = db.Query(`INSERT IGNORE INTO tokenme.user_tx (tx, user_id, from_addr, to_addr, token_address, tokens, eth) VALUES ('%s', %d, '%s', '%s', '%s', %d, 0)`, rp.FundTx, user.Id, walletPublicKey, Config.RedPacketIncomeWallet, rp.Token.Address, totalTokens.Uint64())
		if CheckErr(err, c) {
			return
		}
	}

	_, ret, err := db.Query(`INSERT INTO tokenme.red_packets (user_id, message, token_address, total_tokens, gas_price, gas_limit, recipients, fund_tx, fund_tx_status, expire_time, status) VALUES (%d, '%s', '%s', %.4f, %d, %d, %d, '%s', %d, '%s', 1)`, user.Id, db.Escape(rp.Message), db.Escape(rp.Token.Address), req.TotalTokens, rp.GasPrice, rp.GasLimit, rp.Recipients, rp.FundTx, rp.FundTxStatus, db.Escape(rp.ExpireTime.Format("2006-01-02 15:04:05")))
	if CheckErr(err, c) {
		return
	}
	rp.Id = ret.InsertId()
	err = prepareRedPacketRecipients(rp.Id, uint64(rp.Recipients), totalTokens.Uint64())
	if CheckErr(err, c) {
		return
	}
	c.JSON(http.StatusOK, rp)
}

func prepareRedPacketRecipients(packetId uint64, recipients uint64, tokens uint64) error {
	if tokens == 0 || recipients == 0 {
		return nil
	}
	db := Service.Db
	rows, _, err := db.Query(`SELECT 1 FROM tokenme.red_packet_recipients WHERE red_packet_id=%d LIMIT 1`, packetId)
	if err != nil {
		log.Error(err.Error())
		return err
	}
	if len(rows) > 0 {
		return nil
	}
	packs := redpacket.Generate(tokens, recipients, 1)
	var val []string
	for _, p := range packs {
		val = append(val, fmt.Sprintf("(%d, %.4f)", packetId, float64(p)/10000))
	}
	_, _, err = db.Query(`INSERT INTO tokenme.red_packet_recipients (red_packet_id, give_out) VALUES %s`, strings.Join(val, ","))
	if err != nil {
		return err
	}
	_, _, err = db.Query(`UPDATE tokenme.red_packets SET status=1, expire_time=DATE_ADD(NOW(), INTERVAL 3 DAY) WHERE id=%d`, packetId)
	if err != nil {
		return err
	}
	return nil
}
