package airdrop

import (
	"context"
	"errors"
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

type DealerContractDeployer struct {
	service *common.Service
	config  common.Config
	exitCh  chan struct{}
}

func NewDealerContractDeployer(service *common.Service, config common.Config) *DealerContractDeployer {
	return &DealerContractDeployer{
		service: service,
		config:  config,
		exitCh:  make(chan struct{}, 1),
	}
}

func (this *DealerContractDeployer) Start() {
	log.Info("DealerContractDeployer Start")
	ctx, cancel := context.WithCancel(context.Background())
	go this.DeployLoop(ctx)
	<-this.exitCh
	cancel()
}

func (this *DealerContractDeployer) Stop() {
	close(this.exitCh)
	log.Info("DealerContractDeployer Stopped")
}

func (this *DealerContractDeployer) DeployLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			this.Deploy(ctx)
		}
		time.Sleep(2 * time.Minute)
	}
}

func (this *DealerContractDeployer) Deploy(ctx context.Context) {
	db := this.service.Db
	query := `SELECT id, wallet, salt, dealer_tx, dealer_tx_status FROM tokenme.airdrops WHERE balance_status=0 AND dealer_tx_status<2 AND end_date>=DATE(NOW()) AND id> %d ORDER BY id DESC LIMIT 1000`
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
				Id:             row.Uint64(0),
				Wallet:         publicKey,
				WalletPrivKey:  privateKey,
				DealerTx:       row.Str(3),
				DealerTxStatus: row.Uint(4),
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
			this.DeployAirdrop(airdrop, ctx)
		}
		if endId == startId {
			break
		}
		startId = endId
	}
}

func (this *DealerContractDeployer) DeployAirdrop(airdrop *common.Airdrop, ctx context.Context) error {
	gasNeed := new(big.Int).Mul(big.NewInt(5*params.Shannon), big.NewInt(210000))
	if airdrop.GasBalance.Cmp(gasNeed) == -1 {
		err := errors.New("not enough gas")
		log.Error(err.Error())
		return err
	}
	db := this.service.Db
	if airdrop.DealerTxStatus == 0 {
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
		contractAddress, tx, _, err := ethutils.DeployMultiSendERC20Dealer(transactor, this.service.Geth)
		if err != nil {
			log.Error(err.Error())
			return err
		}
		txHash := tx.Hash()
		_, _, err = db.Query(`UPDATE tokenme.airdrops SET dealer_contract='%s', dealer_tx='%s', dealer_tx_status=1 WHERE id=%d`, contractAddress.Hex(), txHash.Hex(), airdrop.Id)
		if err != nil {
			log.Error(err.Error())
			return err
		}
		log.Info("Contract:%s Tx:%s, Airdrop:%d Created", contractAddress.Hex(), txHash.Hex(), airdrop.Id)
		return nil
	}
	receipt, err := ethutils.TransactionReceipt(this.service.Geth, ctx, airdrop.DealerTx)
	if err != nil {
		log.Error(err.Error())
		return err
	}
	if receipt == nil {
		log.Info("Contract Tx:%s, AirdropId:%d isPending", airdrop.DealerTx, airdrop.Id)
		return nil
	}
	var status uint = 3
	if receipt.Status == 1 {
		status = 2
	}
	_, _, err = db.Query(`UPDATE tokenme.airdrops SET dealer_tx_status=%d WHERE id=%d`, status, airdrop.Id)
	if err != nil {
		log.Error(err.Error())
		return err
	}
	return nil
}
