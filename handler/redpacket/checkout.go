package redpacket

import (
	//"github.com/davecgh/go-spew/spew"
	"fmt"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
	"github.com/getsentry/raven-go"
	"github.com/gin-gonic/gin"
	//"github.com/mkideal/log"
	"github.com/nlopes/slack"
	"github.com/tokenme/tokenmed/coins/eth"
	ethutils "github.com/tokenme/tokenmed/coins/eth/utils"
	"github.com/tokenme/tokenmed/common"
	. "github.com/tokenme/tokenmed/handler"
	"github.com/tokenme/tokenmed/utils"
	"math/big"
	"net/http"
)

type CheckoutRequest struct {
	TokenAddress string  `form:"token_address" json:"token_address"`
	TotalTokens  float64 `form:"total_tokens" json:"total_tokens" binding:"required"`
	GasPrice     uint64  `form:"gas_price" json:"gas_price"`
	Wallet       string  `form:"wallet" json:"wallet"`
}

func CheckoutHandler(c *gin.Context) {
	var req CheckoutRequest
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
			raven.CaptureError(err, nil)
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
	var (
		totalTokens        *big.Int
		totalTokensForSave *big.Int
	)
	if token.Decimals >= 4 {
		totalTokensForSave = new(big.Int).SetUint64(uint64(req.TotalTokens * float64(utils.Pow40.Uint64())))
		totalTokens = new(big.Int).Mul(totalTokensForSave, utils.Pow10(int(token.Decimals)))
		totalTokens = new(big.Int).Div(totalTokens, utils.Pow40)
	} else {
		totalTokensForSave = new(big.Int).SetUint64(uint64(req.TotalTokens))
		totalTokens = new(big.Int).Mul(totalTokensForSave, utils.Pow10(int(token.Decimals)))
	}

	if req.GasPrice == 0 {
		req.GasPrice = Config.RedPacketGasPrice
	}

	query := `SELECT
            uw.wallet,
            uw.salt,
            uw.is_private
        FROM tokenme.user_wallets AS uw
        WHERE uw.user_id=%d AND is_main=1`
	rows, _, err := db.Query(query, user.Id)
	if CheckErr(err, c) {
		raven.CaptureError(err, nil)
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

	ethBalance, err := eth.BalanceOf(Service.Geth, c, walletPublicKey)
	if CheckErr(err, c) {
		raven.CaptureError(err, nil)
		return
	}

	if req.Wallet == "" {
		tokenBalance, err := getTokenCashBalance(token.Address, int(token.Decimals), user.Id)
		if CheckErr(err, c) {
			return
		}

		if tokenBalance.Cmp(big.NewInt(0)) != 1 || tokenBalance.Cmp(totalTokens) == -1 {
			c.JSON(http.StatusOK, APIError{Code: 503, Msg: fmt.Sprintf("%.4f", req.TotalTokens)})
			return
		}
		minETHGWei := new(big.Int).SetUint64(Config.CheckoutFee)
		minETHWei := new(big.Int).Mul(minETHGWei, big.NewInt(params.Shannon))
		if ethBalance.Cmp(big.NewInt(0)) != 1 || ethBalance.Cmp(minETHWei) == -1 {
			c.JSON(http.StatusOK, APIError{Code: 502, Msg: fmt.Sprintf("%d", minETHGWei.Uint64())})
			return
		}

		outputPrivateKey, _ := utils.AddressDecrypt(Config.RedPacketOutputWallet, Config.OutputSalt, Config.OutputKey)
		outputPublicKey, _ := eth.AddressFromHexPrivateKey(outputPrivateKey)

		_, err = checkoutTransfer(c, walletPrivateKey, walletPublicKey, "", minETHWei, Config.RedPacketIncomeWallet)
		if CheckErr(err, c) {
			return
		}
		tx, err := checkoutTransfer(c, outputPrivateKey, outputPublicKey, token.Address, totalTokens, walletPublicKey)
		if CheckErr(err, c) {
			return
		}
		txHash := tx.Hash()
		fundTx := txHash.Hex()
		_, _, err = db.Query(`INSERT INTO tokenme.checkouts (tx, user_id, token_address, tokens, status) VALUES ('%s', %d, '%s', %.4f, 0)`, fundTx, user.Id, token.Address, req.TotalTokens)
		if CheckErr(err, c) {
			raven.CaptureError(err, nil)
			return
		}
		_, _, err = db.Query(`INSERT INTO tokenme.user_tx (tx, user_id, from_addr, to_addr, token_address, tokens, eth) VALUES ('%s', %d, '%s', '%s', '%s', %.4f, 0)`, fundTx, user.Id, Config.RedPacketIncomeWallet, walletPublicKey, token.Address, req.TotalTokens)
		if CheckErr(err, c) {
			raven.CaptureError(err, nil)
			return
		}
		if Service.Slack != nil {
			params := slack.PostMessageParameters{}
			attachment := slack.Attachment{
				Color:      "#bd503a",
				AuthorName: user.ShowName,
				AuthorIcon: user.Avatar,
				Fields: []slack.AttachmentField{
					slack.AttachmentField{
						Title: "Token",
						Value: token.Name,
						Short: true,
					},
					slack.AttachmentField{
						Title: "Checkout",
						Value: fmt.Sprintf("%.4f", req.TotalTokens),
						Short: true,
					},
				},
			}
			params.Attachments = []slack.Attachment{attachment}
			_, _, err := Service.Slack.PostMessage("G9Y7METUG", "new red packet checkout", params)
			if err != nil {
				raven.CaptureError(err, nil)
			}
		}

		c.JSON(http.StatusOK, APIResponse{Msg: "ok"})
		return
	}

	if req.TokenAddress != "" {
		tokenBalance, err := ethutils.BalanceOfToken(Service.Geth, token.Address, walletPublicKey)
		if CheckErr(err, c) {
			raven.CaptureError(err, nil)
			return
		}
		if tokenBalance.Cmp(big.NewInt(0)) != 1 || tokenBalance.Cmp(totalTokens) == -1 {
			c.JSON(http.StatusOK, APIError{Code: 503, Msg: fmt.Sprintf("%.4f", req.TotalTokens)})
			return
		}
	} else {
		minGasLimit := new(big.Int).Mul(new(big.Int).SetUint64(req.GasPrice), new(big.Int).SetUint64(Config.RedPacketGasLimit))
		var minETHGwei *big.Int
		if req.TokenAddress == "" {
			totalTokensGwei := new(big.Int).Div(totalTokens, big.NewInt(params.Shannon))
			minETHGwei = new(big.Int).Add(minGasLimit, totalTokensGwei)
		} else {
			minETHGwei = minGasLimit
		}
		minETHWei := new(big.Int).Mul(minETHGwei, big.NewInt(params.Shannon))
		if ethBalance.Cmp(big.NewInt(0)) != 1 || ethBalance.Cmp(minETHWei) == -1 {
			c.JSON(http.StatusOK, APIError{Code: 502, Msg: fmt.Sprintf("%d", minETHGwei.Uint64())})
			return
		}
	}
	_, err = checkoutTransfer(c, walletPrivateKey, walletPublicKey, token.Address, totalTokens, req.Wallet)
	if CheckErr(err, c) {
		return
	}
	c.JSON(http.StatusOK, APIResponse{Msg: "ok"})
}

func checkoutTransfer(c *gin.Context, privKey string, pubKey string, tokenAddress string, totalTokens *big.Int, toAddress string) (tx *types.Transaction, err error) {
	transactor := eth.TransactorAccount(privKey)
	nonce, err := eth.Nonce(c, Service.Geth, Service.Redis.Master, pubKey, "main")
	if err != nil {
		raven.CaptureError(err, nil)
		return nil, err
	}
	gasPrice := new(big.Int).Mul(new(big.Int).SetUint64(Config.RedPacketGasPrice), big.NewInt(params.Shannon))
	if tokenAddress != "" {
		transactorOpts := eth.TransactorOptions{
			Nonce:    nonce,
			GasPrice: gasPrice,
			GasLimit: Config.RedPacketGasLimit,
		}
		eth.TransactorUpdate(transactor, transactorOpts, c)
		tokenHandler, err := ethutils.NewToken(tokenAddress, Service.Geth)
		if err != nil {
			raven.CaptureError(err, nil)
			return nil, err
		}
		tx, err = ethutils.Transfer(tokenHandler, transactor, toAddress, totalTokens)
		if err != nil {
			raven.CaptureError(err, nil)
			return nil, err
		}
	} else {
		transactorOpts := eth.TransactorOptions{
			Nonce:    nonce,
			Value:    totalTokens,
			GasPrice: gasPrice,
			GasLimit: Config.RedPacketGasLimit,
		}
		eth.TransactorUpdate(transactor, transactorOpts, c)
		tx, err = eth.Transfer(transactor, Service.Geth, c, toAddress)
		if err != nil {
			raven.CaptureError(err, nil)
			return nil, err
		}
	}
	return tx, nil
}
func getTokenCashBalance(tokenAddress string, decimals int, userId uint64) (*big.Int, error) {
	var (
		redPacketIncome      *big.Int
		redPacketOutcomeLeft *big.Int
		redPacketCashOutput  *big.Int
	)

	db := Service.Db
	rows, _, err := db.Query(`SELECT
	SUM(rpr.give_out) AS income
FROM
	tokenme.red_packet_recipients AS rpr
INNER JOIN tokenme.red_packets AS rp ON ( rp.id = rpr.red_packet_id )
WHERE
	rpr.user_id = %d
AND rp.token_address = '%s'
AND rpr.status = 2`, userId, db.Escape(tokenAddress))
	if err != nil {
		raven.CaptureError(err, nil)
		return nil, err
	}
	if len(rows) > 0 {
		row := rows[0]
		giveOut := row.ForceFloat(0)
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
	SUM(d.tokens) AS income
FROM
	tokenme.deposits AS d
WHERE
	d.user_id = %d
AND d.token_address = '%s'
AND d.status = 1`, userId, db.Escape(tokenAddress))
	if err != nil {
		raven.CaptureError(err, nil)
		return nil, err
	}
	if len(rows) > 0 {
		row := rows[0]
		deposit := row.ForceFloat(0)
		var (
			value        uint64
			depositValue *big.Int
		)
		if decimals >= 4 {
			value = uint64(deposit * float64(utils.Pow40.Uint64()))
			depositValue = new(big.Int).Mul(new(big.Int).SetUint64(value), utils.Pow10(decimals))
			depositValue = new(big.Int).Div(depositValue, utils.Pow40)
		} else {
			value = uint64(deposit)
			depositValue = new(big.Int).Mul(new(big.Int).SetUint64(value), utils.Pow10(decimals))
		}
		if redPacketIncome != nil {
			redPacketIncome = new(big.Int).Add(redPacketIncome, depositValue)
		} else {
			redPacketIncome = depositValue
		}
	}

	rows, _, err = db.Query(`SELECT
	SUM(IF(rp.expire_time < NOW(), rp.total_tokens, 0)) AS expired_outcome,
	SUM(IF(rp.fund_tx_status = 0, rp.total_tokens, 0)) AS cash_output
FROM
	tokenme.red_packets AS rp
WHERE
	rp.user_id =%d
AND rp.token_address = '%s'`, userId, db.Escape(tokenAddress))
	if err != nil {
		raven.CaptureError(err, nil)
		return nil, err
	}
	if len(rows) > 0 {
		row := rows[0]
		outcomeLeft := row.ForceFloat(0)
		cashOutput := row.ForceFloat(1)
		var (
			leftValue *big.Int
			cashValue *big.Int
		)
		if decimals >= 4 {
			leftVal := uint64(outcomeLeft * float64(utils.Pow40.Uint64()))
			cashVal := uint64(cashOutput * float64(utils.Pow40.Uint64()))
			leftValue = new(big.Int).Mul(new(big.Int).SetUint64(leftVal), utils.Pow10(decimals))
			leftValue = new(big.Int).Div(leftValue, utils.Pow40)
			cashValue = new(big.Int).Mul(new(big.Int).SetUint64(cashVal), utils.Pow10(decimals))
			cashValue = new(big.Int).Div(cashValue, utils.Pow40)
		} else {
			leftVal := uint64(outcomeLeft)
			cashVal := uint64(cashOutput)
			leftValue = new(big.Int).Mul(new(big.Int).SetUint64(leftVal), utils.Pow10(decimals))
			cashValue = new(big.Int).Mul(new(big.Int).SetUint64(cashVal), utils.Pow10(decimals))
		}
		redPacketOutcomeLeft = leftValue
		redPacketCashOutput = cashValue
	}

	rows, _, err = db.Query(`SELECT
	SUM(ck.tokens) AS checkout
FROM
	tokenme.checkouts AS ck
WHERE
	ck.user_id = %d
AND ck.token_address = '%s'
AND ck.status IN (0, 1)`, userId, db.Escape(tokenAddress))
	if err != nil {
		raven.CaptureError(err, nil)
		return nil, err
	}
	if len(rows) > 0 {
		row := rows[0]
		checkout := row.ForceFloat(0)
		var (
			value         uint64
			checkoutValue *big.Int
		)
		if decimals >= 4 {
			value = uint64(checkout * float64(utils.Pow40.Uint64()))
			checkoutValue = new(big.Int).Mul(new(big.Int).SetUint64(value), utils.Pow10(decimals))
			checkoutValue = new(big.Int).Div(checkoutValue, utils.Pow40)
		} else {
			value = uint64(checkout)
			checkoutValue = new(big.Int).Mul(new(big.Int).SetUint64(value), utils.Pow10(decimals))
		}
		if redPacketCashOutput != nil {
			redPacketCashOutput = new(big.Int).Add(redPacketCashOutput, checkoutValue)
		} else {
			redPacketCashOutput = checkoutValue
		}
	}

	rows, _, err = db.Query(`SELECT
	SUM(IFNULL(rpr.give_out, 0)) AS expired_giveout
FROM
	tokenme.red_packet_recipients AS rpr
INNER JOIN tokenme.red_packets AS rp ON ( rp.id = rpr.red_packet_id )
WHERE
	rp.user_id=%d
AND rp.token_address = '%s'
AND rpr.status!=0
AND rp.expire_time < NOW()`, userId, db.Escape(tokenAddress))
	if err != nil {
		raven.CaptureError(err, nil)
		return nil, err
	}
	if len(rows) > 0 {
		row := rows[0]
		val := row.ForceFloat(0)
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
		if redPacketOutcomeLeft != nil {
			redPacketOutcomeLeft = new(big.Int).Sub(redPacketOutcomeLeft, expiredOutcome)
		}
	}

	cash := big.NewInt(0)
	if redPacketIncome != nil {
		cash = redPacketIncome
	}
	if redPacketOutcomeLeft != nil {
		cash = new(big.Int).Add(cash, redPacketOutcomeLeft)
	}
	if redPacketCashOutput != nil {
		if cash.Cmp(redPacketCashOutput) == -1 {
			cash = big.NewInt(0)
		} else {
			cash = new(big.Int).Sub(cash, redPacketCashOutput)
		}
	}
	return cash, nil
}
