package promotion

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/mkideal/log"
	"github.com/tokenme/tokenmed/common"
	. "github.com/tokenme/tokenmed/handler"
	"github.com/tokenme/tokenmed/tools/shorturl"
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

	proto, err := common.DecodePromotion([]byte(Config.LinkSalt), req.ProtoKey)
	if CheckErr(err, c) {
		return
	}

	db := Service.Db
	rows, _, err := db.Query(`SELECT t.protocol FROM tokenme.tokens AS t INNER JOIN tokenme.airdrops AS a ON (a.token_address=t.address) WHERE a.id=%d LIMIT 1`, proto.AirdropId)
	if CheckErr(err, c) {
		log.Error(err.Error())
		return
	}
	if Check(len(rows) == 0, "not found", c) {
		return
	}
	protocol := rows[0].Str(0)
	if Check(protocol == "ERC20" && (len(req.Wallet) != 42 || !strings.HasPrefix(req.Wallet, "0x")), "invalid wallet", c) {
		return
	}
	_, _, err = db.Query("DELETE FROM tokenme.codes WHERE wallet='%s' AND airdrop_id=%d AND `status`!=2", db.Escape(req.Wallet), proto.AirdropId)
	if CheckErr(err, c) {
		log.Error(err.Error())
		return
	}
	_, _, err = db.Query("UPDATE tokenme.codes SET wallet='%s', referrer='%s', `status`=1 WHERE id=%d AND `status`=0", db.Escape(req.Wallet), db.Escape(proto.Referrer), req.Code)
	//if strings.Contains(err.Error(), "Duplicate entry") {
	//	err = nil
	//}
	if CheckErr(err, c) {
		log.Error(err.Error())
		return
	}
	promotion, err := getPromotionLink(proto, req.Wallet)
	if CheckErr(err, c) {
		log.Error(err.Error())
		return
	}

	c.JSON(http.StatusOK, promotion)
}

func getPromotionLink(proto common.PromotionProto, wallet string) (promotion common.Promotion, err error) {
	promo := common.PromotionProto{
		Id:        proto.Id,
		UserId:    proto.UserId,
		AirdropId: proto.AirdropId,
		AdzoneId:  proto.AdzoneId,
		ChannelId: proto.ChannelId,
		Referrer:  wallet,
	}

	promoKey, err := common.EncodePromotion([]byte(Config.LinkSalt), promo)
	if err != nil {
		return promotion, err
	}
	link := fmt.Sprintf("%s/promo/%s", Config.BaseUrl, promoKey)
	shortURL, err := shorturl.Sina(link)
	if err == nil && shortURL != "" {
		link = shortURL
	}
	promotion = common.Promotion{
		Link: link,
		Key:  promoKey,
	}
	return promotion, nil
}
