package token

import (
	"fmt"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/gin-gonic/gin"
	cmc "github.com/miguelmota/go-coinmarketcap"
	"github.com/tokenme/tokenmed/coins/eth"
	. "github.com/tokenme/tokenmed/handler"
	"math"
	"math/big"
	"net/http"
	"strings"
)

func MarketHandler(c *gin.Context) {
	q := c.Query("q")
	var (
		coinId   string
		price    float64
		decimals *big.Int
	)
	if q == "ETH" {
		coinId = "ethereum"
	} else {
		q = strings.ToLower(q)
		db := Service.Db
		rows, _, err := db.Query(`SELECT name, price, decimals FROM tokenme.tokens WHERE address='%s' LIMIT 1`, q)
		if CheckErr(err, c) {
			return
		}
		if len(rows) == 0 {
			c.JSON(http.StatusOK, APIError{Code: NOTFOUND_ERROR, Msg: "not found"})
			return
		}

		row := rows[0]
		price = row.ForceFloat(1)
		if row.Int(2) > 0 {
			decimals = big.NewInt(int64(math.Pow10(row.Int(2))))
		} else {
			decimals = big.NewInt(0)
		}

		coinId = row.Str(0)
		coinId = strings.ToLower(coinId)
		coinId = strings.Replace(coinId, " ", "-", 0)
	}

	coin, _ := cmc.GetCoinData(coinId)
	if coin.ID == "" && price > 0 {
		coin = cmc.Coin{
			PriceUSD: price,
		}
		tokenCaller, err := eth.NewStandardTokenCaller(ethcommon.HexToAddress(q), Service.Geth)
		if CheckErr(err, c) {
			return
		}
		totalSupply, err := tokenCaller.TotalSupply(nil)
		if CheckErr(err, c) {
			return
		}
		if decimals.Cmp(big.NewInt(0)) == 0 {
			coin.TotalSupply = float64(totalSupply.Uint64())
		} else {
			coin.TotalSupply = float64(new(big.Int).Div(totalSupply, decimals).Uint64())
		}

		coin.MarketCapUSD = coin.TotalSupply * coin.PriceUSD
	} else {
		redisMasterConn := Service.Redis.Master.Get()
		defer redisMasterConn.Close()
		redisMasterConn.Do("SETEX", fmt.Sprintf("coinprice-%s", strings.ToLower(q)), 60*60, coin.PriceUSD)
	}
	c.JSON(http.StatusOK, coin)
	return
}
