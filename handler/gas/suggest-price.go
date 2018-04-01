package gas

import (
	"github.com/gin-gonic/gin"
	. "github.com/tokenme/tokenmed/handler"
	"math/big"
	"net/http"
)

func SuggestPriceHandler(c *gin.Context) {
	geth := Service.Geth
	if geth == nil {
		c.JSON(http.StatusOK, APIError{Code: NOTFOUND_ERROR, Msg: "not found"})
		return
	}
	price, err := geth.SuggestGasPrice(c)
	if CheckErr(err, c) {
		return
	}
	c.JSON(http.StatusOK, map[string]*big.Int{"price": price})
}
