package promotion

import (
	"github.com/gin-gonic/gin"
	. "github.com/tokenme/tokenmed/handler"
	"net/http"
	"strings"
)

type TopNList struct {
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
FROM tokenme.codes AS c
INNER JOIN (
	SELECT referrer, count(*) AS cnt 
	FROM tokenme.airdrop_submissions
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

	ret := []*TopNList{}
	for idx, row := range rows {
		ret = append(ret, &TopNList{
			Wallet: row.Str(0),
			Email:  displayEmail(row.Str(1)),
			Cnt:    row.Int(2),
			Idx:    idx + 1,
		})
	}

	summary := struct {
		TotalCnt        int `json:"total_cnt" codec:"total_cnt"`
		Submissions     int `json:"submissions" codec:"submissions"`
		SelfSubmissions int `json:"self_submissions" codec:"self_submissions"`
	}{}
	query = `SELECT 
	COUNT(1) AS total_cnt,
	SUM(IF(TRIM(IFNULL(asub.referrer, "")) != "" AND asub.wallet != asub.referrer, 1, 0)) AS submissions
FROM tokenme.airdrop_submissions AS asub 
WHERE asub.airdrop_id = %d AND asub.status = 2`
	rows, _, err = db.Query(query, airdropId)
	if CheckErr(err, c) {
		return
	}
	if len(rows) > 0 {
		summary.TotalCnt = rows[0].Int(0)
		summary.Submissions = rows[0].Int(1)
		summary.SelfSubmissions = summary.TotalCnt - summary.Submissions
	}

	c.JSON(http.StatusOK, map[string]interface{}{
		"airdrop_id": airdropId,
		"topn":       ret,
		"summary":    summary,
	})
}

// 邮箱像是前两位及最后一位字符，及@后邮箱域名信息，如：ye****y@163.com
func displayEmail(email string) string {
	if email == "" {
		return ""
	}

	vals := strings.Split(email, "@")
	if len(vals) != 2 || len(vals[0]) < 2 {
		return email
	}

	var pad rune = '*'
	v := []rune(vals[0])
	switch len(v) {
	case 2, 3:
		v[1] = pad
	default:
		for i := 2; i < len(v)-1; i++ {
			v[i] = pad
		}
	}
	return string(v) + "@" + vals[1]
}
