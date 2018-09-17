package promotion

import (
	"github.com/gin-gonic/gin"
	. "github.com/tokenme/tokenmed/handler"
	"net/http"
)

type TopNResponse struct {
	Wallet string `json:"wallet" codec:"wallet"`
	Email  string `json:"email" codec:"email"`
	Cnt    int    `json:"cnt" codec:"cnt"`
	Idx    int    `json:"idx" codec:"idx"`
}

func TopNHandler(c *gin.Context) {
	n, err := Uint64NonZero(c.Query("n"), "missing top n")
	if CheckErr(err, c) {
		return
	}
	if n > 500 {
		n = 500
	}

	airdropId, err := Uint64NonZero(c.Query("airdrop_id"), "missing airdrop id")
	if CheckErr(err, c) {
		return
	}

	db := Service.Db
	query := `SELECT DISTINCT 
		c.wallet, c.email, IFNULL(s.cnt, 0) AS total_cnt
FROM codes AS c
INNER JOIN (
	SELECT referrer, count(*) AS cnt 
	FROM airdrop_submissions
  WHERE airdrop_id = %d
	GROUP BY referrer
  ORDER BY cnt DESC, referrer
  LIMIT %d
) AS s ON s.referrer = c.wallet
WHERE
	c.airdrop_id = %d
  AND c.email IS NOT NULL
ORDER BY total_cnt DESC, c.email`
	rows, _, err := db.Query(query, airdropId, n, airdropId)
	if CheckErr(err, c) {
		return
	}

	ret := []*TopNResponse{}
	for idx, row := range rows {
		ret = append(ret, &TopNResponse{
			Wallet: row.Str(0),
			Email:  row.Str(1),
			Cnt:    row.Int(2),
			Idx:    idx + 1,
		})
	}

	c.JSON(http.StatusOK, ret)
}
