package common

import (
	"fmt"
	"github.com/garyburd/redigo/redis"
	"github.com/tokenme/tokenmed/tools/shorturl"
	"github.com/tokenme/tokenmed/utils"
	"math/big"
	"time"
)

type RedPacketStatus = uint

const (
	RedPacketStatusNoBalance  RedPacketStatus = 0
	RedPacketStatusOk         RedPacketStatus = 1
	RedPacketStatusAllTaken   RedPacketStatus = 2
	RedPacketStatusTransfered RedPacketStatus = 3
	RedPacketStatusSuccess    RedPacketStatus = 4
	RedPacketStatusFailed     RedPacketStatus = 5
	RedPacketStatusExpired    RedPacketStatus = 6
)

type RedPacketRecipient struct {
	Id            uint64    `json:"id"`
	User          User      `json:"user"`
	Status        uint      `json:"status"`
	GiveOut       *big.Int  `json:"give_out"`
	Decimals      uint      `json:"decimals"`
	SubmittedTime time.Time `json:"submitted_time"`
}

type RedPacket struct {
	Id                  uint64               `json:"id"`
	User                User                 `json:"user"`
	Message             string               `json:"message"`
	Token               Token                `json:"token"`
	TotalTokens         *big.Int             `json:"total_tokens"`
	GasPrice            uint64               `json:"gas_price"`
	GasLimit            uint64               `json:"gas_limit"`
	Recipients          uint                 `json:"recipients"`
	Status              uint                 `json:"status"`
	FundTx              string               `json:"fund_tx,omitempty"`
	FundTxStatus        uint                 `json:"fund_tx_status"`
	ExpireTime          time.Time            `json:"expire_time"`
	Inserted            time.Time            `json:"inserted"`
	Updated             time.Time            `json:"updated"`
	Link                string               `json:"link"`
	ShortUrl            string               `json:"short_url"`
	SubmittedRecipients []RedPacketRecipient `json:"submitted_recipients,omitempty"`
}

func EncodeRedPacketLink(key []byte, id uint64) (string, error) {
	buf := utils.Uint64ToByte(id)
	return utils.AESEncryptBytes(key, buf)
}

func DecodeRedPacketLink(key []byte, cryptoText string) (uint64, error) {
	data, err := utils.AESDecryptBytes(key, cryptoText)
	if err != nil {
		return 0, err
	}
	return utils.ByteToUint64(data), nil
}

func (this RedPacket) GetShortUrl(service *Service) string {
	redisMasterConn := service.Redis.Master.Get()
	defer redisMasterConn.Close()
	shortURL, err := redis.String(redisMasterConn.Do("GET", fmt.Sprintf("rp-shorturl-%d", this.Id)))
	if err != nil {
		shortURL, err = shorturl.Sina(this.Link)
		if err == nil {
			redisMasterConn.Do("SETEX", fmt.Sprintf("rp-shorturl-%d", this.Id), 60*60*24*2, shortURL)
		}
	}
	return shortURL
}
