package user

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/tokenme/tokenmed/common"
	. "github.com/tokenme/tokenmed/handler"
	"github.com/tokenme/tokenmed/utils"
	"net/http"
)

type GetWallerPrivateKeyRequest struct {
	WalletId uint64 `form:"wallet_id" json:"wallet_id" binding:"required"`
	Passwd   string `form:"passwd" json:"passwd" binding:"required"`
}

func GetWalletPrivateKeyHandler(c *gin.Context) {
	var req GetWallerPrivateKeyRequest
	if CheckErr(c.Bind(&req), c) {
		return
	}
	userContext, exists := c.Get("USER")
	if Check(!exists, "need login", c) {
		return
	}
	user := userContext.(common.User)

	db := Service.Db
	query := `SELECT
	uw.wallet,
	uw.salt,
	uw.is_private,
	uw.passwd,
	u.passwd,
	u.salt
FROM tokenme.user_wallets AS uw
INNER JOIN tokenme.users AS u ON (u.id=uw.user_id)
WHERE uw.user_id=%d AND uw.id=%d`
	rows, _, err := db.Query(query, user.Id, req.WalletId)
	if CheckErr(err, c) {
		return
	}

	row := rows[0]
	wallet := row.Str(0)
	salt := row.Str(1)
	isPrivate := row.Uint(2)
	password := row.Str(3)
	userPassword := row.Str(4)
	userSalt := row.Str(5)
	if Check(isPrivate != 1, "no privte key provided", c) {
		return
	}
	var validPassword bool
	if password == "" {
		passwdSha1 := utils.Sha1(fmt.Sprintf("%s%s%s", userSalt, req.Passwd, userSalt))
		validPassword = passwdSha1 == userPassword
	} else {
		passwdSha1 := utils.Sha1(fmt.Sprintf("%s%s%s", salt, req.Passwd, salt))
		validPassword = passwdSha1 == password
	}
	if Check(!validPassword, "invalid password", c) {
		return
	}
	privateKey, err := utils.AddressDecrypt(wallet, salt, Config.TokenSalt)
	if CheckErr(err, c) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"priv": privateKey})
}
