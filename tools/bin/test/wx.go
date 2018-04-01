package main

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"github.com/davecgh/go-spew/spew"
	"github.com/tokenme/tokenmed/common"
	"github.com/tokenme/tokenmed/utils"
	"log"
)

func main() {
	sessionKey := "tiihtNczf5v6AKRyjwEUhQ=="
	iv := "r7BXXKkLb8qrSNn05n0qiA=="
	encryptedData := "CiyLU1Aw2KjvrjMdj8YKliAjtP4gsMZMQmRzooG2xrDcvSnxIMXFufNstNGTyaGS9uT5geRa0W4oTOb1WT7fJlAC+oNPdbB+3hVbJSRgv+4lGOETKUQz6OYStslQ142dNCuabNPGBzlooOmB231qMM85d2/fV6ChevvXvQP8Hkue1poOFtnEtpyxVLW1zAo6/1Xx1COxFvrc2d7UL/lmHInNlxuacJXwu0fjpXfz/YqYzBIBzD6WUfTIF9GRHpOn/Hz7saL8xz+W//FRAUid1OksQaQx4CMs8LOddcQhULW4ucetDf96JcR3g0gfRK4PC7E/r7Z6xNrXd2UIeorGj5Ef7b1pJAYB6Y5anaHqZ9J6nKEBvB4DnNLIVWSgARns/8wR2SiRS7MNACwTyrGvt9ts8p12PKFdlqYTopNHR1Vf7XjfhQlVsAJdNiKdYmYVoKlaRv85IfVunYzO0IKXsyl7JCUjCpoG20f0a04COwfneQAGGwd5oa+T8yO5hzuyDb/XcxxmK01EpqOyuxINew=="
	wechatPhone, err := wechatDecrypt(sessionKey, iv, encryptedData)
	if err != nil {
		log.Fatalln(err)
		return
	}
	spew.Dump(wechatPhone)
}

func wechatDecrypt(sessionKey string, ivText string, cryptoText string) (phone common.WechatUser, err error) {
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
