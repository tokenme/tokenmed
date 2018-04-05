package redpacket

import (
	//"github.com/davecgh/go-spew/spew"
	"fmt"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
	"github.com/getsentry/raven-go"
	"github.com/gin-gonic/gin"
	"github.com/nlopes/slack"
	"github.com/tokenme/tokenmed/coins/eth"
	ethutils "github.com/tokenme/tokenmed/coins/eth/utils"
	"github.com/tokenme/tokenmed/common"
	. "github.com/tokenme/tokenmed/handler"
	"github.com/tokenme/tokenmed/utils"
	"math/big"
	"net/http"
)

type DepositRequest struct {
	TokenAddress string  `form:"token_address" json:"token_address"`
	TotalTokens  float64 `form:"total_tokens" json:"total_tokens" binding:"required"`
}

func DepositHandler(c *gin.Context) {
	var req DepositRequest
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
	minGasLimit := new(big.Int).SetUint64(Config.RedPacketGasPrice * Config.RedPacketGasLimit)
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
	}

	transactor := eth.TransactorAccount(walletPrivateKey)
	nonce, err := eth.PendingNonce(Service.Geth, c, walletPublicKey)
	if CheckErr(err, c) {
		raven.CaptureError(err, nil)
		return
	}
	var tx *types.Transaction
	gasPrice := new(big.Int).Mul(new(big.Int).SetUint64(Config.RedPacketGasPrice), big.NewInt(params.Shannon))
	if req.TokenAddress != "" {
		transactorOpts := eth.TransactorOptions{
			Nonce:    nonce,
			GasPrice: gasPrice,
			GasLimit: Config.RedPacketGasLimit,
		}
		eth.TransactorUpdate(transactor, transactorOpts, c)
		tokenHandler, err := ethutils.NewToken(token.Address, Service.Geth)
		if CheckErr(err, c) {
			raven.CaptureError(err, nil)
			return
		}
		tx, err = ethutils.Transfer(tokenHandler, transactor, Config.RedPacketIncomeWallet, totalTokens)
		if CheckErr(err, c) {
			raven.CaptureError(err, nil)
			return
		}
	} else {
		transactorOpts := eth.TransactorOptions{
			Nonce:    nonce,
			Value:    totalTokens,
			GasPrice: gasPrice,
			GasLimit: Config.RedPacketGasLimit,
		}
		eth.TransactorUpdate(transactor, transactorOpts, c)
		tx, err = eth.Transfer(transactor, Service.Geth, c, Config.RedPacketIncomeWallet)
		if CheckErr(err, c) {
			raven.CaptureError(err, nil)
			return
		}
	}
	txHash := tx.Hash()
	fundTx := txHash.Hex()
	_, _, err = db.Query(`INSERT INTO tokenme.deposits (tx, user_id, token_address, tokens, status) VALUES ('%s', %d, '%s', %.4f, 0)`, fundTx, user.Id, token.Address, req.TotalTokens)
	if CheckErr(err, c) {
		raven.CaptureError(err, nil)
		return
	}
	_, _, err = db.Query(`INSERT INTO tokenme.user_tx (tx, user_id, from_addr, to_addr, token_address, tokens, eth) VALUES ('%s', %d, '%s', '%s', '%s', %.4f, 0)`, fundTx, user.Id, walletPublicKey, Config.RedPacketIncomeWallet, token.Address, req.TotalTokens)
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
					Title: "Deposit",
					Value: fmt.Sprintf("%.4f", req.TotalTokens),
					Short: true,
				},
			},
		}
		params.Attachments = []slack.Attachment{attachment}
		_, _, err := Service.Slack.PostMessage("G9Y7METUG", "new red packet deposit", params)
		if err != nil {
			raven.CaptureError(err, nil)
		}
	}

	c.JSON(http.StatusOK, APIResponse{Msg: "ok"})
}
