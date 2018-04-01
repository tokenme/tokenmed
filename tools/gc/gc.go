package gc

import (
	"github.com/mkideal/log"
	"github.com/tokenme/tokenmed/common"
	"time"
)

const (
	VerifyCodeGCHours      int = 2
	AuthVerifyCodesGCHours int = 3
)

type Handler struct {
	Service *common.Service
	Config  common.Config
	exitCh  chan struct{}
}

func New(service *common.Service, config common.Config) *Handler {
	return &Handler{
		Service: service,
		Config:  config,
		exitCh:  make(chan struct{}, 1),
	}
}

func (this *Handler) Start() {
	log.Info("GC Start")
	hourlyTicker := time.NewTicker(1 * time.Hour)
	for {
		select {
		case <-hourlyTicker.C:
			this.verifyCodesRecycle()
			this.authVerifyCodesRecycle()
			this.airdropTXRecycle()
		case <-this.exitCh:
			hourlyTicker.Stop()
			return
		}
	}
}

func (this *Handler) Stop() {
	close(this.exitCh)
	log.Info("GC Stopped")
}

func (this *Handler) verifyCodesRecycle() error {
	db := this.Service.Db
	_, _, err := db.Query(`DELETE FROM tokenme.codes WHERE status!=2 AND updated<DATE_SUB(NOW(), INTERVAL %d HOUR)`, VerifyCodeGCHours)
	return err
}

func (this *Handler) authVerifyCodesRecycle() error {
	db := this.Service.Db
	_, _, err := db.Query(`DELETE FROM tokenme.auth_verify_codes WHERE inserted<DATE_SUB(NOW(), INTERVAL %d HOUR)`, AuthVerifyCodesGCHours)
	return err
}

func (this *Handler) airdropTXRecycle() error {
	db := this.Service.Db
	query := `DELETE
FROM
	tokenme.airdrop_tx
WHERE
	airdrop_tx.status = 1
AND EXISTS ( SELECT
	1
FROM
	tokenme.airdrop_submissions AS ass
WHERE
	ass.tx = airdrop_tx.tx
AND ass.status = 2
LIMIT 1 )
OR airdrop_tx.status = 0
AND NOT EXISTS ( SELECT
	1
FROM
	tokenme.airdrop_submissions AS ass
WHERE
	ass.tx = airdrop_tx.tx
LIMIT 1 )`
	_, _, err := db.Query(query)
	return err
}
