package common

import (
	"fmt"
	"github.com/tokenme/tokenmed/utils"
	"math/big"
)

type User struct {
	Id          uint64        `json:"id,omitempty"`
	CountryCode uint          `json:"country_code,omitempty"`
	Mobile      string        `json:"mobile,omitempty"`
	Email       string        `json:"email,omitempty"`
	Name        string        `json:"realname,omitempty"`
	Telegram    *TelegramUser `json:"telegram,omitempty"`
	Wechat      *WechatUser   `json:"wechat,omitempty"`
	ShowName    string        `json:"showname,omitempty"`
	Avatar      string        `json:"avatar,omitempty"`
	Salt        string        `json:"-"`
	Password    string        `json:"-"`
	IsAdmin     uint          `json:"is_admin,omitempty"`
	IsPublisher uint          `json:"is_publisher,omitempty"`
}

type TelegramUser struct {
	Id        int64  `json:"id"`
	Username  string `json:"username"`
	Firstname string `json:"firstname"`
	Lastname  string `json:"lastname"`
	Avatar    string `json:"avatar"`
}

type WechatUser struct {
	UnionId         string          `json:"unionId,omitempty"`
	OpenId          string          `json:"openId,omitempty"`
	Nick            string          `json:"nickName,omitempty"`
	Gender          uint            `json:"gender,omitempty"`
	City            string          `json:"city,omitempty"`
	Province        string          `json:"province,omitempty"`
	Country         string          `json:"country,omitempty"`
	Avatar          string          `json:"avatarUrl,omitempty"`
	Language        string          `json:"language,omitempty"`
	PhoneNumber     string          `json:"phoneNumber,omitempty"`
	PurePhoneNumber string          `json:"purePhoneNumber,omitempty"`
	CountryCode     string          `json:"countryCode,omitempty"`
	Watermark       WechatWatermark `json:"watermark,omitempty"`
}

type WechatWatermark struct {
	AppId     string `json:"appid,omitempty"`
	Timestamp int64  `json:"timestamp,omitempty"`
}

type WechatOAuth struct {
	UnionId    string `json:"unionid,omitempty"`
	OpenId     string `json:"openid,omitempty"`
	SessionKey string `json:"session_key,omitempty"`
}

func (this User) GetShowName() string {
	if this.Name != "" {
		return this.Name
	}
	if this.Wechat != nil && this.Wechat.Nick != "" {
		return this.Wechat.Nick
	}
	if this.Telegram != nil && (this.Telegram.Firstname != "" && this.Telegram.Lastname != "" || this.Telegram.Username != "") {
		if this.Telegram.Username != "" {
			return this.Telegram.Username
		}
		return fmt.Sprintf("%s %s", this.Telegram.Firstname, this.Telegram.Lastname)
	}
	if this.Email != "" {
		return this.Email
	}
	return fmt.Sprintf("+%d%s", this.CountryCode, this.Mobile)
}

func (this User) GetAvatar(cdn string) string {
	if this.Wechat != nil && this.Wechat.Avatar != "" {
		return this.Wechat.Avatar
	}
	if this.Telegram != nil && this.Telegram.Avatar != "" {
		return this.Telegram.Avatar
	}
	key := utils.Md5(fmt.Sprintf("+%d%s", this.CountryCode, this.Mobile))
	return fmt.Sprintf("%suser/avatar/%s", cdn, key)
}

type UserWallet struct {
	Id                 uint64     `json:"id"`
	UserId             uint64     `json:"user_id"`
	Wallet             string     `json:"wallet"`
	PrivateKey         string     `json:"-"`
	Name               string     `json:"name"`
	IsMain             uint       `json:"is_main"`
	IsPrivate          uint       `json:"is_private"`
	DepositWallet      string     `json:"deposit_wallet"`
	RedPacketMinGas    uint64     `json:"rp_min_gas"` // Gwei
	RedPacketEnoughGas bool       `json:"rp_enough_gas"`
	Funds              []UserFund `json:"funds"`
}

type UserFund struct {
	UserId uint64   `json:"user_id"`
	Token  Token    `json:"token"`
	Amount *big.Int `json:"amount"`
	Cash   *big.Int `json:"cash"`
}
