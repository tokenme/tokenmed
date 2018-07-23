package redpacket

import (
	"context"
	"github.com/mkideal/log"
	ethutils "github.com/tokenme/tokenmed/coins/eth/utils"
	"github.com/tokenme/tokenmed/common"
	"time"
)

type CheckoutChecker struct {
	service *common.Service
	config  common.Config
	exitCh  chan struct{}
}

func NewCheckoutChecker(service *common.Service, config common.Config) *CheckoutChecker {
	return &CheckoutChecker{
		service: service,
		config:  config,
		exitCh:  make(chan struct{}, 1),
	}
}

func (this *CheckoutChecker) Start() {
	log.Info("CheckoutChecker Start")
	ctx, cancel := context.WithCancel(context.Background())
	go this.CheckLoop(ctx)
	<-this.exitCh
	cancel()
}

func (this *CheckoutChecker) Stop() {
	close(this.exitCh)
	log.Info("CheckoutChecker Stopped")
}

func (this *CheckoutChecker) CheckLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			this.Check(ctx)
		}
		time.Sleep(30 * time.Second)
	}
}

func (this *CheckoutChecker) Check(ctx context.Context) {
	db := this.service.Db
	query := `SELECT
	tx
FROM tokenme.checkouts
WHERE status=0 AND tx > '%s'
ORDER BY tx ASC
LIMIT 1000`
	var (
		startTx string
		endTx   string
	)
	for {
		endTx = startTx
		rows, _, err := db.Query(query, startTx)
		if err != nil {
			log.Error(err.Error())
			break
		}
		for _, row := range rows {
			tx := row.Str(0)
			this.CheckStatus(ctx, tx)
			endTx = tx
		}
		if endTx == startTx {
			break
		}
		startTx = endTx
	}
}

func (this *CheckoutChecker) CheckStatus(ctx context.Context, tx string) error {
	receipt, err := ethutils.TransactionReceipt(this.service.Geth, ctx, tx)
	if err != nil {
		log.Error(err.Error())
		return err
	}
	if receipt == nil {
		log.Info("Deposit Tx:%s, isPending", tx)
		return nil
	}
	log.Info("Deposit Tx:%s, status:%d", tx, receipt.Status)
	var (
		txStatus uint = 2
	)
	if receipt.Status == 1 {
		txStatus = 1
	}
	db := this.service.Db
	_, _, err = db.Query(`UPDATE tokenme.checkouts SET status=%d WHERE tx='%s'`, txStatus, tx)
	if err != nil {
		log.Error(err.Error())
		return err
	}
	return nil
}
