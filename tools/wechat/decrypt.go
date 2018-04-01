package wechat

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"github.com/tokenme/tokenmed/common"
	"github.com/tokenme/tokenmed/utils"
)

func Decrypt(sessionKey string, ivText string, cryptoText string) (phone common.WechatUser, err error) {
	aesKey, err := base64.StdEncoding.DecodeString(sessionKey)
	if err != nil {
		return phone, err
	}
	ciphertext, err := base64.StdEncoding.DecodeString(cryptoText)
	if err != nil {
		return phone, err
	}
	iv, err := base64.StdEncoding.DecodeString(ivText)
	if err != nil {
		return phone, err
	}
	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return phone, err
	}
	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(ciphertext, ciphertext)
	ciphertext, err = utils.PKCS7UnPadding(ciphertext, block.BlockSize())
	if err != nil {
		return phone, err
	}
	err = json.Unmarshal(ciphertext, &phone)
	if err != nil {
		return
	}
	return phone, nil
}
