package user

import (
	"fmt"
	//"github.com/davecgh/go-spew/spew"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/garyburd/redigo/redis"
	"github.com/gin-gonic/gin"
	cmc "github.com/miguelmota/go-coinmarketcap"
	"github.com/mkideal/log"
	"github.com/tokenme/tokenmed/coins/eth"
	ethutils "github.com/tokenme/tokenmed/coins/eth/utils"
	"github.com/tokenme/tokenmed/common"
	. "github.com/tokenme/tokenmed/handler"
	"github.com/tokenme/tokenmed/tools/ethplorer-api"
	"github.com/tokenme/tokenmed/utils"
	"math/big"
	"net/http"
	"strings"
	"sync"
)

func FundGetHandler(c *gin.Context) {
	userContext, exists := c.Get("USER")
	if Check(!exists, "need login", c) {
		return
	}
	user := userContext.(common.User)

	walletId, _ := Uint64Value(c.Query("wallet_id"), 0)
	var where string
	if walletId > 0 {
		where = fmt.Sprintf(" AND uw.id=%d", walletId)
	} else {
		where = " AND uw.is_main=1"
	}

	db := Service.Db

	query := `SELECT
	DISTINCT (t.address),
	t.name,
	t.symbol,
	t.decimals,
	t.price,
	t.logo,
	uw.id,
	uw.wallet,
	uw.salt,
	uw.name,
	uw.is_main,
	uw.is_private
FROM tokenme.promotions AS p
INNER JOIN tokenme.user_wallets AS uw ON (uw.user_id=p.user_id AND uw.token_type='ETH')
INNER JOIN tokenme.airdrops AS a ON (a.id=p.airdrop_id)
INNER JOIN tokenme.tokens AS t ON (t.address=a.token_address)
WHERE p.user_id=%d%s
UNION
	SELECT
	DISTINCT (t.address),
	t.name,
	t.symbol,
	t.decimals,
	t.price,
	t.logo,
	uw.id,
	uw.wallet,
	uw.salt,
	uw.name,
	uw.is_main,
	uw.is_private
FROM tokenme.red_packet_recipients AS rpr
INNER JOIN tokenme.user_wallets AS uw ON (uw.user_id=rpr.user_id AND uw.token_type='ETH')
INNER JOIN tokenme.red_packets AS rp ON (rp.id=rpr.red_packet_id)
INNER JOIN tokenme.tokens AS t ON (t.address=rp.token_address)
WHERE rpr.user_id=%d%s`
	rows, _, err := db.Query(query, user.Id, where, user.Id, where)
	if CheckErr(err, c) {
		return
	}

	var (
		funds      []common.UserFund
		userWallet common.UserWallet
		cashOnly   = c.Query("cash_only") == "true"
		walletOnly = c.Query("wallet_only") == "true"
	)

	if len(rows) == 0 {
		query := `SELECT
			uw.id,
			uw.wallet,
			uw.salt,
			uw.name,
			uw.is_main,
			uw.is_private
		FROM tokenme.user_wallets AS uw
		WHERE uw.user_id=%d%s`
		db := Service.Db
		rows, _, err := db.Query(query, user.Id, where)
		if CheckErr(err, c) {
			return
		}
		row := rows[0]
		walletId := row.Uint64(0)
		wallet := row.Str(1)
		salt := row.Str(2)
		walletName := row.Str(3)
		isMain := row.Uint(4)
		isPrivate := row.Uint(5)
		var (
			privateKey string
			publicKey  string
		)
		if isPrivate == 1 {
			privateKey, _ = utils.AddressDecrypt(wallet, salt, Config.TokenSalt)
			publicKey, _ = eth.AddressFromHexPrivateKey(privateKey)
		} else {
			publicKey = wallet
		}

		userWallet = common.UserWallet{
			Id:            walletId,
			Name:          walletName,
			Wallet:        publicKey,
			IsMain:        isMain,
			IsPrivate:     isPrivate,
			DepositWallet: Config.RedPacketIncomeWallet,
		}
	} else {
		row := rows[0]
		wId := row.Uint64(6)
		wallet := row.Str(7)
		salt := row.Str(8)
		walletName := row.Str(9)
		isMain := row.Uint(10)
		isPrivate := row.Uint(11)
		var (
			privateKey string
			publicKey  string
		)
		if isPrivate == 1 {
			privateKey, _ = utils.AddressDecrypt(wallet, salt, Config.TokenSalt)
			publicKey, _ = eth.AddressFromHexPrivateKey(privateKey)
		} else {
			publicKey = wallet
		}

		userWallet = common.UserWallet{
			Id:            wId,
			Name:          walletName,
			Wallet:        publicKey,
			IsMain:        isMain,
			IsPrivate:     isPrivate,
			DepositWallet: Config.RedPacketIncomeWallet,
		}
	}

	var (
		redPacketIncome      = make(map[string]*big.Int)
		redPacketOutcomeLeft = make(map[string]*big.Int)
		redPacketCashOutput  = make(map[string]*big.Int)
	)

	if userWallet.IsMain == 1 && !walletOnly {
		rows, _, err := db.Query(`SELECT
	rp.token_address ,
	SUM(rpr.give_out) AS income,
	IF(ISNULL(t.address), 18, t.decimals) AS decimals
FROM
	tokenme.red_packet_recipients AS rpr
INNER JOIN tokenme.red_packets AS rp ON ( rp.id = rpr.red_packet_id )
LEFT JOIN tokenme.tokens AS t ON ( t.address = rp.token_address )
WHERE
	rpr.user_id = %d
AND rpr.status = 2
GROUP BY
	rp.token_address`, user.Id)
		if CheckErr(err, c) {
			log.Error(err.Error())
			return
		}
		for _, row := range rows {
			giveOut := row.ForceFloat(1)
			decimals := row.Int(2)
			var value uint64
			if decimals >= 4 {
				value = uint64(giveOut * float64(utils.Pow40.Uint64()))
				redPacketIncome[row.Str(0)] = new(big.Int).Mul(new(big.Int).SetUint64(value), utils.Pow10(decimals))
				redPacketIncome[row.Str(0)] = new(big.Int).Div(redPacketIncome[row.Str(0)], utils.Pow40)
			} else {
				value = uint64(giveOut)
				redPacketIncome[row.Str(0)] = new(big.Int).Mul(new(big.Int).SetUint64(value), utils.Pow10(decimals))
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
GROUP BY
	rp.token_address`, user.Id)
		if CheckErr(err, c) {
			log.Error(err.Error())
			return
		}
		for _, row := range rows {
			addr := row.Str(0)
			outcomeLeft := row.ForceFloat(1)
			cashOutput := row.ForceFloat(2)
			decimals := row.Int(3)
			if decimals >= 4 {
				leftVal := uint64(outcomeLeft * float64(utils.Pow40.Uint64()))
				cashVal := uint64(cashOutput * float64(utils.Pow40.Uint64()))
				redPacketOutcomeLeft[addr] = new(big.Int).Mul(new(big.Int).SetUint64(leftVal), utils.Pow10(decimals))
				redPacketOutcomeLeft[addr] = new(big.Int).Div(redPacketOutcomeLeft[addr], utils.Pow40)
				redPacketCashOutput[addr] = new(big.Int).Mul(new(big.Int).SetUint64(cashVal), utils.Pow10(decimals))
				redPacketCashOutput[addr] = new(big.Int).Div(redPacketCashOutput[addr], utils.Pow40)
			} else {
				leftVal := uint64(outcomeLeft)
				cashVal := uint64(cashOutput)
				redPacketOutcomeLeft[addr] = new(big.Int).Mul(new(big.Int).SetUint64(leftVal), utils.Pow10(decimals))
				redPacketCashOutput[addr] = new(big.Int).Mul(new(big.Int).SetUint64(cashVal), utils.Pow10(decimals))
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
AND rpr.status!=0
AND rp.expire_time < NOW()
GROUP BY
	rp.token_address`, user.Id)
		if CheckErr(err, c) {
			log.Error(err.Error())
			return
		}
		for _, row := range rows {
			addr := row.Str(0)
			val := row.ForceFloat(1)
			decimals := row.Int(2)
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
			if outcomeLeft, found := redPacketOutcomeLeft[addr]; found {
				redPacketOutcomeLeft[addr] = new(big.Int).Sub(outcomeLeft, expiredOutcome)
			}
		}
	}
	userWallet.RedPacketMinGas = Config.RedPacketGasPrice * Config.RedPacketGasLimit
	ethBalance := big.NewInt(0)

	if !cashOnly {
		ethBalance, err = Service.Geth.BalanceAt(c, ethcommon.HexToAddress(userWallet.Wallet), nil)
		if CheckErr(err, c) {
			return
		}
	}
	ethToken := common.Token{
		Name:     "ETH",
		Symbol:   "Ether",
		Decimals: 18,
		Price:    &ethplorer.TokenPrice{Currency: "USD"},
		Logo:     1,
	}
	ethToken.LogoAddress = ethToken.GetLogoAddress(Config.CDNUrl)

	fund := common.UserFund{
		UserId: user.Id,
		Token:  ethToken,
		Amount: ethBalance,
		Cash:   big.NewInt(0),
	}
	if income, found := redPacketIncome[""]; found {
		fund.Cash = income
	}
	if outcomeLeft, found := redPacketOutcomeLeft[""]; found {
		fund.Cash = new(big.Int).Add(fund.Cash, outcomeLeft)
	}

	if cashOutput, found := redPacketCashOutput[""]; found {
		if fund.Cash.Cmp(cashOutput) == -1 {
			fund.Cash = big.NewInt(0)
		} else {
			fund.Cash = new(big.Int).Sub(fund.Cash, cashOutput)
		}
	}

	userWallet.RedPacketEnoughGas = fund.Amount.Cmp(big.NewInt(int64(userWallet.RedPacketMinGas))) != -1

	redisMasterConn := Service.Redis.Master.Get()
	defer redisMasterConn.Close()

	if fund.Amount.Cmp(big.NewInt(0)) == 1 || fund.Cash.Cmp(big.NewInt(0)) == 1 {
		coinPrice, err := redis.Float64(redisMasterConn.Do("GET", "coinprice-eth"))
		if err != nil || coinPrice == 0 {
			coinPrice, err := cmc.GetCoinPriceUSD("ethereum")
			if err == nil {
				fund.Token.Price.Rate = coinPrice
				redisMasterConn.Do("SETEX", "coinprice-eth", 60*60, coinPrice)
			}
		} else {
			fund.Token.Price.Rate = coinPrice
		}
	}
	funds = append(funds, fund)

	var (
		tokens        []common.Token
		tokenPriceMap = make(map[string]float64)
	)
	for _, row := range rows {
		if row.Str(0) == "" {
			continue
		}
		token := common.Token{
			Address:  row.Str(0),
			Name:     row.Str(1),
			Symbol:   row.Str(2),
			Decimals: row.Uint(3),
			Logo:     row.Uint(5),
		}
		token.LogoAddress = token.GetLogoAddress(Config.CDNUrl)
		tokens = append(tokens, token)
		tokenPriceMap[token.Address] = row.ForceFloat(4)
	}

	ethplorerClient := ethplorer.NewClient(Config.EthplorerKey)
	addressInfo, err := ethplorerClient.GetAddressInfo(userWallet.Wallet, "")
	if err == nil {
		var (
			tokenQueryList []string
			tokenAddrs     []string
		)
		for _, t := range addressInfo.Tokens {
			if _, found := tokenPriceMap[t.Token.Address]; found {
				continue
			}
			tokenQueryList = append(tokenQueryList, fmt.Sprintf("'%s'", db.Escape(t.Token.Address)))
		}
		if len(tokenQueryList) > 0 {
			rows, _, err := db.Query(`SELECT address, name, symbol, decimals, price, logo, FROM tokenme.tokens WHERE address IN (%s)`, strings.Join(tokenQueryList, ","))
			if CheckErr(err, c) {
				return
			}
			for _, row := range rows {
				addr := row.Str(0)
				if _, found := tokenPriceMap[addr]; found {
					continue
				}
				token := common.Token{
					Address:  row.Str(0),
					Name:     row.Str(1),
					Symbol:   row.Str(2),
					Decimals: row.Uint(3),
					Logo:     row.Uint(5),
				}
				token.LogoAddress = token.GetLogoAddress(Config.CDNUrl)
				tokens = append(tokens, token)
				tokenPriceMap[addr] = row.ForceFloat(4)
			}
			for _, t := range addressInfo.Tokens {
				if _, found := tokenPriceMap[t.Token.Address]; found {
					continue
				}
				tokenAddrs = append(tokenAddrs, t.Token.Address)
			}
		}
		if len(tokenAddrs) > 0 {
			var newTokens []common.Token
			for _, addr := range tokenAddrs {
				tokenCaller, err := eth.NewTokenCaller(ethcommon.HexToAddress(addr), Service.Geth)
				if CheckErr(err, c) {
					continue
				}
				tokenSymbol, err := tokenCaller.Symbol(nil)
				if CheckErr(err, c) {
					continue
				}
				tokenDecimals, err := tokenCaller.Decimals(nil)
				if CheckErr(err, c) {
					continue
				}
				tokenName, err := tokenCaller.Name(nil)
				if CheckErr(err, c) {
					continue
				}
				token := common.Token{
					Address:  addr,
					Name:     tokenName,
					Symbol:   tokenSymbol,
					Decimals: uint(tokenDecimals),
					Protocol: "ERC20",
				}
				newTokens = append(newTokens, token)
			}
			if len(newTokens) > 0 {
				var vals []string
				for _, t := range newTokens {
					vals = append(vals, fmt.Sprintf("('%s', '%s', '%s', %d, '%s')", db.Escape(t.Address), db.Escape(t.Name), db.Escape(t.Symbol), t.Decimals, t.Protocol))
				}
				_, _, err = db.Query(`INSERT IGNORE INTO tokenme.tokens (address, name, symbol, decimals, protocol) VALUES %s`, strings.Join(vals, ","))
				if err != nil {
					log.Error(err.Error())
				}
				tokens = append(tokens, newTokens...)
			}
		}
	}
	var wg sync.WaitGroup
	var (
		mgetKeys []interface{}
		msetKeys = make(map[string]float64)
	)
	for _, token := range tokens {
		if price, found := tokenPriceMap[token.Address]; !found || price == 0 {
			mgetKeys = append(mgetKeys, fmt.Sprintf("coinprice-%s", token.Address))
		}
	}
	if len(mgetKeys) > 0 {
		prices, err := redis.Float64s(redisMasterConn.Do("MGET", mgetKeys...))
		if err != nil {
			log.Error(err.Error())
		} else {
			for idx, price := range prices {
				tokenPriceMap[mgetKeys[idx].(string)] = price
			}
		}
	}
	for _, token := range tokens {
		wg.Add(1)
		go func(token common.Token) {
			defer wg.Done()
			if price, found := tokenPriceMap[token.Address]; found && price > 0 {
				token.Price = &ethplorer.TokenPrice{Rate: price, Currency: "USD"}
			} else {
				var coinId = token.Name
				coinId = strings.ToLower(coinId)
				coinId = strings.Replace(coinId, " ", "-", 0)
				coinPrice, err := cmc.GetCoinPriceUSD(coinId)
				if err == nil && coinPrice != 0 {
					token.Price = &ethplorer.TokenPrice{Rate: coinPrice, Currency: "USD"}
					msetKeys[token.Address] = coinPrice
				}
			}
			tokenBalance := big.NewInt(0)
			if !cashOnly {
				tokenHandler, err := ethutils.NewStandardToken(token.Address, Service.Geth)
				if err != nil {
					log.Error(err.Error())
					return
				}
				tokenBalance, err = ethutils.StandardTokenBalanceOf(tokenHandler, userWallet.Wallet)
				if err != nil {
					log.Error(err.Error())
					return
				}
			}
			fund := common.UserFund{
				UserId: user.Id,
				Token:  token,
				Amount: tokenBalance,
			}
			if income, found := redPacketIncome[token.Address]; found {
				fund.Cash = income
			}
			if outcomeLeft, found := redPacketOutcomeLeft[token.Address]; found {
				fund.Cash = new(big.Int).Add(fund.Cash, outcomeLeft)
			}

			if cashOutput, found := redPacketCashOutput[token.Address]; found {
				if fund.Cash.Cmp(cashOutput) == -1 {
					fund.Cash = big.NewInt(0)
				} else {
					fund.Cash = new(big.Int).Sub(fund.Cash, cashOutput)
				}
			}

			funds = append(funds, fund)
		}(token)
	}
	wg.Wait()
	if len(msetKeys) > 0 {
		err := redisMasterConn.Send("MULTI")
		if err != nil {
			log.Error(err.Error())
		} else {
			for addr, price := range msetKeys {
				err = redisMasterConn.Send("SETEX", fmt.Sprintf("coinprice-%s", addr), 60*60, price)
				if err != nil {
					log.Error(err.Error())
				}
			}
			redisMasterConn.Do("EXEC")
		}
	}
	userWallet.Funds = funds
	c.JSON(http.StatusOK, userWallet)
}
