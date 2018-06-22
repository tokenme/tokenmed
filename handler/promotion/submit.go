package promotion

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/tokenme/tokenmed/common"
	. "github.com/tokenme/tokenmed/handler"
	"github.com/tokenme/tokenmed/utils/token"
	"net/http"
	"strings"
)

type SubmitRequest struct {
	Wallet   string      `json:"wallet,omitempty"`
	Code     token.Token `json:"verify_code,omitempty"`
	ProtoKey string      `json:"proto,omitempty"`
}

func SubmitHandler(c *gin.Context) {
	var req SubmitRequest
	if CheckErr(c.Bind(&req), c) {
		return
	}
	if Check(len(req.Wallet) != 42 || !strings.HasPrefix(req.Wallet, "0x"), "invalid wallet", c) {
		return
	}

	proto, err := common.DecodePromotion([]byte(Config.LinkSalt), req.ProtoKey)
	if CheckErr(err, c) {
		return
	}

	db := Service.Db
	_, _, err = db.Query(`UPDATE tokenme.codes SET wallet='%s', referrer='%s', status=1 WHERE id=%d AND status=0`, db.Escape(req.Wallet), db.Escape(proto.Referrer), req.Code)
	if CheckErr(err, c) {
		return
	}

	promo := common.PromotionProto{
		Id:        proto.Id,
		UserId:    proto.UserId,
		AirdropId: proto.AirdropId,
		AdzoneId:  proto.AdzoneId,
		ChannelId: proto.ChannelId,
		Referrer:  req.Wallet,
	}

	promoKey, err := common.EncodePromotion([]byte(Config.LinkSalt), promo)
	if CheckErr(err, c) {
		return
	}
	promotion := common.Promotion{
		Link: fmt.Sprintf("%s/promo/%s", Config.BaseUrl, promoKey),
		Key:  promoKey,
	}

	c.JSON(http.StatusOK, promotion)
}
