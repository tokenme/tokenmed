package airdrop

import (
	"context"
	"fmt"
	"github.com/ethereum/go-ethereum/params"
	"github.com/mkideal/log"
	"github.com/tokenme/tokenmed/coins/eth"
	ethutils "github.com/tokenme/tokenmed/coins/eth/utils"
	"github.com/tokenme/tokenmed/common"
	"github.com/tokenme/tokenmed/utils"
	"math/big"
	"strings"
	"sync"
	"time"
)

type AllawanceChecker struct {
	service *common.Service
	config  common.Config
	exitCh  chan struct{}
}

func NewAllowanceChecker(service *common.Service, config common.Config) *AllawanceChecker {
	return &AllawanceChecker{
		service: service,
		config:  config,
		exitCh:  make(chan struct{}, 1),
	}
}

func (this *AllawanceChecker) Start() {
	log.Info("AllawanceChecker Start")
	ctx, cancel := context.WithCancel(context.Background())
	go this.CheckLoop(ctx)
	<-this.exitCh
	cancel()
}

func (this *AllawanceChecker) Stop() {
	close(this.exitCh)
	log.Info("AllawanceChecker Stopped")
}

func (this *AllawanceChecker) CheckLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			this.Check(ctx)
		}
		time.Sleep(2 * time.Minute)
	}
}

func (this *AllawanceChecker) Check(ctx context.Context) {
	db := this.service.Db
	query := `SELECT id, wallet, salt, token_address, approve_tx, dealer_contract, approve_tx_status FROM tokenme.airdrops WHERE balance_status=0 AND dealer_tx_status=2 AND end_date>=DATE(NOW()) AND (approve_tx_status=1 OR allowance_checked<DATE_SUB(NOW(), INTERVAL 1 HOUR)) AND id> %d ORDER BY id DESC LIMIT 1000`
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
		if len(rows) == 0 {
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
				Id:              row.Uint64(0),
				Wallet:          publicKey,
				WalletPrivKey:   privateKey,
				Token:           common.Token{Address: row.Str(3)},
				ApproveTx:       row.Str(4),
				DealerContract:  row.Str(5),
				ApproveTxStatus: row.Uint(6),
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

		var val []string
		for _, a := range airdrops {
			val = append(val, fmt.Sprintf("(%d, %d)", a.Id, a.BalanceStatus))
		}
		if len(val) > 0 {
			_, _, err = db.Query(`INSERT INTO tokenme.airdrops (id, balance_status) VALUES %s ON DUPLICATE KEY UPDATE balance_status=VALUES(balance_status)`, strings.Join(val, ","))
			if err != nil {
				log.Error(err.Error())
			}
		}
		for _, airdrop := range airdrops {
			this.CheckApprove(airdrop, ctx)
		}
		if endId == startId {
			break
		}
		startId = endId
	}
}

func (this *AllawanceChecker) CheckApprove(airdrop *common.Airdrop, ctx context.Context) error {
	token, err := ethutils.NewStandardToken(airdrop.Token.Address, this.service.Geth)
	if err != nil {
		log.Error(err.Error())
		return err
	}
	allowance, err := ethutils.StandardTokenAllowance(token, airdrop.Wallet, airdrop.DealerContract)
	if err != nil {
		log.Error(err.Error())
		return err
	}
	db := this.service.Db
	tokenBalance, err := ethutils.StandardTokenBalanceOf(token, airdrop.Wallet)
	if allowance.Cmp(tokenBalance) == -1 {
		if airdrop.ApproveTxStatus == 1 {
			receipt, err := ethutils.TransactionReceipt(this.service.Geth, ctx, airdrop.ApproveTx)
			if err != nil {
				log.Error(err.Error())
				return err
			}
			if receipt == nil {
				log.Info("Approve Tx:%s, AirdropId:%d isPending", airdrop.ApproveTx, airdrop.Id)
				return nil
			}
			var status uint = 3
			if receipt.Status == 1 {
				status = 2
			}
			_, _, err = db.Query(`UPDATE tokenme.airdrops SET allowance=%d, approve_tx_status=%d, allowance_checked=NOW() WHERE id=%d`, allowance.Uint64(), status, airdrop.Id)
			if err != nil {
				log.Error(err.Error())
				return err
			}
			log.Info("Approve Tx:%s, AirdropId:%d Done", airdrop.ApproveTx, airdrop.Id)
			return nil
		}
		transactor := eth.TransactorAccount(airdrop.WalletPrivKey)
		nonce, err := eth.PendingNonce(this.service.Geth, ctx, airdrop.Wallet)
		if err != nil {
			log.Error(err.Error())
			return err
		}
		transactorOpts := eth.TransactorOptions{
			Nonce:    nonce,
			GasPrice: big.NewInt(this.config.DealerContractCreateGasPrice * params.Shannon),
			GasLimit: this.config.DealerContractCreateGasLimit,
		}
		eth.TransactorUpdate(transactor, transactorOpts, ctx)
		if airdrop.ApproveTx != "" {
			_, isPending, err := ethutils.TransactionByHash(this.service.Geth, ctx, airdrop.ApproveTx)
			if err != nil {
				log.Error(err.Error())
				return err
			}
			if isPending {
				log.Info("Approve Tx:%s, AirdropId:%d isPending", airdrop.ApproveTx, airdrop.Id)
				return nil
			}
		}
		approveTx, err := ethutils.StandardTokenApprove(token, transactor, airdrop.DealerContract, tokenBalance)
		if err != nil {
			log.Error(err.Error())
			return err
		}
		approvalTxHash := approveTx.Hash()
		_, _, err = db.Query(`UPDATE tokenme.airdrops SET allowance=%d, approve_tx_status=1, approve_tx=1, allowance_checked=NOW() WHERE id=%d`, tokenBalance.Uint64(), approvalTxHash.Hex(), airdrop.Id)
		if err != nil {
			log.Error(err.Error())
			return err
		}
		log.Info("Approve TX:%s, AirdropId:%d, OldAllowance:%d, Amounnt:%d", approvalTxHash.Hex(), airdrop.Id, allowance.Uint64(), tokenBalance.Uint64())
	} else {
		_, _, err = db.Query(`UPDATE tokenme.airdrops SET allowance=%d, approve_tx_status=2, allowance_checked=NOW() WHERE id=%d`, allowance.Uint64(), airdrop.Id)
		if err != nil {
			log.Error(err.Error())
			return err
		}
		log.Info("Update Allowance:%d, AirdropId:%d", allowance.Uint64(), airdrop.Id)
	}
	return nil
}