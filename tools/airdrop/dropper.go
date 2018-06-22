package airdrop

import (
	"context"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/mkideal/log"
	"github.com/tokenme/tokenmed/coins/eth"
	ethutils "github.com/tokenme/tokenmed/coins/eth/utils"
	"github.com/tokenme/tokenmed/common"
	"github.com/tokenme/tokenmed/utils"
	"math/big"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Airdropper struct {
	service *common.Service
	config  common.Config
	exitCh  chan struct{}
}

func NewAirdropper(service *common.Service, config common.Config) *Airdropper {
	return &Airdropper{
		service: service,
		config:  config,
		exitCh:  make(chan struct{}, 1),
	}
}

func (this *Airdropper) Start() {
	log.Info("Airdropper Start")
	ctx, cancel := context.WithCancel(context.Background())
	go this.DropLoop(ctx)
	<-this.exitCh
	cancel()
}

func (this *Airdropper) Stop() {
	close(this.exitCh)
	log.Info("Airdropper Stopped")
}

func (this *Airdropper) DropLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			this.Drop(ctx)
		}
		time.Sleep(10 * time.Second)
	}
}

func (this *Airdropper) Drop(ctx context.Context) {
	db := this.service.Db
	query := `SELECT a.id, a.wallet, a.salt, a.token_address, a.gas_price, a.gas_limit, a.bonus, a.commission_fee, a.give_out, t.decimals, a.dealer_contract FROM tokenme.airdrops AS a INNER JOIN tokenme.tokens AS t ON (t.address = a.token_address) WHERE a.balance_status=0 AND a.approve_tx_status=2 AND a.dealer_tx_status=2 AND EXISTS (SELECT 1 FROM tokenme.airdrop_submissions AS ass WHERE ass.status=0 AND ass.airdrop_id=a.id LIMIT 1) AND NOT EXISTS (SELECT 1 FROM tokenme.airdrop_submissions AS ass WHERE ass.status=1 AND ass.airdrop_id=a.id LIMIT 1) AND a.id> %d AND a.end_date<=DATE(NOW()) ORDER BY a.id DESC LIMIT 1000`
	var (
		startId uint64
		endId   uint64
	)
	for {
		endId = startId
		rows, _, err := db.Query(query, startId)
		if err != nil {
			log.Error(err.Error())
			break
		}
		var airdrops []*common.Airdrop
		var wg sync.WaitGroup
		for _, row := range rows {
			wallet := row.Str(1)
			salt := row.Str(2)
			privateKey, _ := utils.AddressDecrypt(wallet, salt, this.config.TokenSalt)
			publicKey, _ := eth.AddressFromHexPrivateKey(privateKey)
			airdrop := &common.Airdrop{
				Id:             row.Uint64(0),
				Wallet:         publicKey,
				WalletPrivKey:  privateKey,
				Token:          common.Token{Address: row.Str(3), Decimals: row.Uint(9)},
				GasPrice:       row.Uint64(4),
				GasLimit:       row.Uint64(5),
				Bonus:          row.Uint(6),
				CommissionFee:  row.Uint64(7),
				GiveOut:        row.Uint64(8),
				DealerContract: row.Str(10),
			}
			endId = airdrop.Id
			wg.Add(1)
			go func(airdrop *common.Airdrop, c context.Context) {
				defer wg.Done()
				airdrop.CheckBalance(this.service.Geth, c)
			}(airdrop, ctx)
			airdrops = append(airdrops, airdrop)
		}
		wg.Wait()
		for _, airdrop := range airdrops {
			this.DropAirdrop(ctx, airdrop)
		}
		if endId == startId {
			break
		}
		startId = endId
	}
}

func (this *Airdropper) DropAirdrop(ctx context.Context, airdrop *common.Airdrop) {
	db := this.service.Db
	rows, _, err := db.Query(`SELECT COUNT(*) AS num FROM tokenme.airdrop_submissions WHERE status!=2 AND airdrop_id=%d`, airdrop.Id)
	if err != nil {
		log.Error(err.Error())
		return
	}
	totalSubmissions := rows[0].Int64(0)
	if totalSubmissions == 0 {
		return
	}
	gasNeed, tokenNeed, enoughGas, enoughToken := airdrop.EnoughBudgetForSubmissions(totalSubmissions)
	if !enoughGas {
		log.Error("Not enough gas, need:%d, left:%d", gasNeed.Uint64(), airdrop.GasBalance.Uint64())
		return
	}
	if !enoughToken {
		log.Error("Not enough token, need:%d, left:%d", tokenNeed.Uint64(), airdrop.TokenBalance.Uint64())
		return
	}
	token, err := ethutils.NewStandardToken(airdrop.Token.Address, this.service.Geth)
	if err != nil {
		log.Error(err.Error())
		return
	}
	allowance, err := ethutils.StandardTokenAllowance(token, airdrop.Wallet, airdrop.DealerContract)
	if err != nil {
		log.Error(err.Error())
		return
	}
	if allowance.Cmp(tokenNeed) == -1 {
		db.Query("UPDATE tokenme.airdrops SET allowance=0, approve_tx_status=0 WHERE id=%d", airdrop.Id)
		log.Error("Not enough allowance, need:%d, left:%d, contract:%s", tokenNeed.Uint64(), allowance.Uint64(), airdrop.DealerContract)
		return
	}

	query := `SELECT ass.id, ass.promotion_id, ass.adzone_id, ass.channel_id, ass.promoter_id, ass.wallet, ass.referrer, u.wallet, u.salt FROM tokenme.airdrop_submissions AS ass INNER JOIN tokenme.user_wallets AS u ON (u.user_id=ass.promoter_id AND u.token_type='ETH' AND u.is_main=1) WHERE ass.status=0 AND ass.airdrop_id=%d ORDER BY id DESC LIMIT 100`
	var submissions []*common.AirdropSubmission
	rows, _, err = db.Query(query, airdrop.Id)
	if err != nil {
		log.Error(err.Error())
		return
	}
	for _, row := range rows {
		wallet := row.Str(7)
		salt := row.Str(8)
		privateKey, _ := utils.AddressDecrypt(wallet, salt, this.config.TokenSalt)
		publicKey, _ := eth.AddressFromHexPrivateKey(privateKey)
		submissionWallet := row.Str(5)
		referrer := row.Str(6)
		if referrer == submissionWallet || referrer == publicKey {
			referrer = ""
		}
		submission := &common.AirdropSubmission{
			Id:      row.Uint64(0),
			Airdrop: airdrop,
			Proto: common.PromotionProto{
				Id:        row.Uint64(1),
				AdzoneId:  row.Uint64(2),
				ChannelId: row.Uint64(3),
				UserId:    row.Uint64(4),
				Referrer:  referrer,
			},
			Wallet:         submissionWallet,
			PromoterWallet: publicKey,
		}
		submissions = append(submissions, submission)
	}
	this.DropAirdropChunk(ctx, airdrop, submissions)
}

func (this *Airdropper) DropAirdropChunk(ctx context.Context, airdrop *common.Airdrop, submissions []*common.AirdropSubmission) {
	totalSubmissions := int64(len(submissions))
	if totalSubmissions == 0 {
		return
	}
	log.Info("Airdroping %d submissions for airdrop:%d", totalSubmissions, airdrop.Id)
	nonce, err := eth.PendingNonce(this.service.Geth, ctx, airdrop.Wallet)
	if err != nil {
		log.Error(err.Error())
		return
	}
	transactor := eth.TransactorAccount(airdrop.WalletPrivKey)

	transactorOpts := eth.TransactorOptions{
		Nonce:    nonce,
		Value:    airdrop.CommissionFeeToWei(),
		GasPrice: airdrop.GasPriceToWei(),
		GasLimit: airdrop.GasLimit,
	}
	eth.TransactorUpdate(transactor, transactorOpts, ctx)
	var (
		recipientsMap = make(map[string]*big.Int)
		tokenAmounts  []*big.Int
		recipients    []ethcommon.Address
		promoWallet   = submissions[0].PromoterWallet
	)
	for _, submission := range submissions {
		recipientsMap[submission.Wallet] = airdrop.TokenGiveOut()
		if submission.Proto.Referrer != "" {
			if _, found := recipientsMap[submission.Proto.Referrer]; found {
				recipientsMap[submission.Proto.Referrer] = new(big.Int).Add(recipientsMap[submission.Proto.Referrer], airdrop.TokenBonus())
			} else {
				recipientsMap[submission.Proto.Referrer] = airdrop.TokenBonus()
			}
		} else if promoWallet != submission.Wallet {
			if _, found := recipientsMap[promoWallet]; found {
				recipientsMap[promoWallet] = new(big.Int).Add(recipientsMap[promoWallet], airdrop.TokenBonus())
			} else {
				recipientsMap[promoWallet] = airdrop.TokenBonus()
			}
		}

	}
	for addr, amount := range recipientsMap {
		recipients = append(recipients, ethcommon.HexToAddress(addr))
		tokenAmounts = append(tokenAmounts, amount)
	}
	multiSenderContract, err := ethutils.NewMultiSendERC20Dealer(airdrop.DealerContract, this.service.Geth)
	if err != nil {
		log.Error(err.Error())
		return
	}
	tx, err := ethutils.MultiSendERC20DealerTransfer(multiSenderContract, transactor, airdrop.Token.Address, airdrop.Wallet, this.config.DealerIncomeWallet, airdrop.CommissionFeeToWei(), recipients, tokenAmounts)
	if err != nil {
		log.Error(err.Error())
		return
	}
	txHash := tx.Hash()
	db := this.service.Db
	_, _, err = db.Query(`INSERT IGNORE INTO tokenme.airdrop_tx (tx, airdrop_id) VALUES ('%s', %d)`, txHash.Hex(), airdrop.Id)
	if err != nil {
		log.Error(err.Error())
		return
	}
	var ids []string
	for _, submission := range submissions {
		ids = append(ids, strconv.FormatUint(submission.Id, 10))
	}
	_, _, err = db.Query(`UPDATE tokenme.airdrop_submissions SET status=1, tx='%s' WHERE airdrop_id=%d AND id IN (%s)`, db.Escape(txHash.Hex()), airdrop.Id, strings.Join(ids, ","))
	if err != nil {
		log.Error(err.Error())
		return
	}
}
