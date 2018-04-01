package common

import (
	"context"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/params"
	"github.com/tokenme/tokenmed/coins/eth"
	"math"
	"math/big"
	"time"
)

type AirdropStatus = uint

const (
	AirdropStatusStop     AirdropStatus = 0
	AirdropStatusStart    AirdropStatus = 1
	AirdropStatusNotStart AirdropStatus = 2
	AirdropStatusFinished AirdropStatus = 3
)

type AirdropBalanceStatus = uint

const (
	AirdropBalanceStatusOk      AirdropBalanceStatus = 0
	AirdropBalanceStatusNoGas   AirdropBalanceStatus = 1
	AirdropBalanceStatusNoToken AirdropBalanceStatus = 2
	AirdropBalanceStatusEmpty   AirdropBalanceStatus = 3
)

type Airdrop struct {
	Id               uint64               `json:"id"`
	User             User                 `json:"user"`
	Title            string               `json:"title"`
	Token            Token                `json:"token"`
	WalletPrivKey    string               `json:"-"`
	Wallet           string               `json:"wallet"`
	GasPrice         uint64               `json:"gas_price"`
	GasLimit         uint64               `json:"gas_limit"`
	CommissionFee    uint64               `json:"commission_fee"`
	GiveOut          uint64               `json:"give_out"`
	Bonus            uint                 `json:"bonus"`
	TelegramGroup    string               `json:"telegram_group"`
	GasBalance       *big.Int             `json:"gas_balance"`
	GasBalanceGwei   *big.Int             `json:"gas_balance_gwei"`
	TokenBalance     *big.Int             `json:"token_balance"`
	Status           AirdropStatus        `json:"status"`
	BalanceStatus    AirdropBalanceStatus `json:"balance_status"`
	DealerContract   string               `json:"-"`
	DealerTx         string               `json:"-"`
	DealerTxStatus   uint                 `json:"-"`
	Allowance        *big.Int             `json:"-"`
	AllowanceChecked time.Time            `json:"-"`
	ApproveTx        string               `json:"-"`
	ApproveTxStatus  uint                 `json:"-"`
	StartDate        time.Time            `json:"start_date"`
	EndDate          time.Time            `json:"end_date"`
	Inserted         time.Time            `json:"inserted"`
	Updated          time.Time            `json:"updated"`
	TelegramBot      string               `json:"telegram_bot"`
}

type AirdropStats struct {
	Pv            uint64    `json:"pv"`
	Submissions   uint64    `json:"submissions"`
	Transactions  uint64    `json:"transactions"`
	GiveOut       uint64    `json:"give_out"`
	Bonus         uint64    `json:"bonus"`
	CommissionFee uint64    `json:"commission_fee"`
	Decimals      uint      `json:"decimals"`
	RecordOn      time.Time `json:"record_on"`
}

type AirdropStatsWithSummary struct {
	Summary AirdropStats   `json:"summary"`
	Stats   []AirdropStats `json:"stats"`
}

func (this *Airdrop) CheckBalance(geth *ethclient.Client, ctx context.Context) (AirdropBalanceStatus, error) {
	gasBalance, err := this.GetGasBalance(geth, ctx)
	if err != nil {
		return AirdropBalanceStatusEmpty, err
	}
	tokenBalance, err := this.GetTokenBalance(geth)
	if err != nil {
		return AirdropBalanceStatusEmpty, err
	}
	if gasBalance.Uint64() == 0 && tokenBalance.Uint64() == 0 {
		this.BalanceStatus = AirdropBalanceStatusEmpty
	} else if gasBalance.Uint64() == 0 {
		this.BalanceStatus = AirdropBalanceStatusNoGas
	} else if tokenBalance.Uint64() == 0 {
		this.BalanceStatus = AirdropBalanceStatusNoToken
	} else {
		this.BalanceStatus = AirdropBalanceStatusOk
	}
	return this.BalanceStatus, nil
}

func (this *Airdrop) GetTokenBalance(geth *ethclient.Client) (balance *big.Int, err error) {
	token, err := eth.NewStandardTokenCaller(common.HexToAddress(this.Token.Address), geth)
	if err != nil {
		return nil, err
	}
	balance, err = token.BalanceOf(nil, common.HexToAddress(this.Wallet))
	if err != nil {
		return nil, err
	}
	this.TokenBalance = balance
	return balance, nil
}

func (this *Airdrop) GetGasBalance(geth *ethclient.Client, ctx context.Context) (balance *big.Int, err error) {
	balance, err = geth.BalanceAt(ctx, common.HexToAddress(this.Wallet), nil)
	if err != nil {
		return nil, err
	}
	this.GasBalance = balance
	this.GasBalanceGwei = new(big.Int).Div(balance, big.NewInt(params.Shannon))
	return balance, nil
}

func (this *Airdrop) TokenBonus() *big.Int {
	if this.Token.Decimals == 0 {
		return new(big.Int).SetUint64(this.GiveOut * uint64(this.Bonus) / 100)
	}
	return new(big.Int).SetUint64(this.GiveOut * uint64(this.Bonus) * uint64(math.Pow10(int(this.Token.Decimals))) / 100)
}

func (this *Airdrop) TotalTokenBonus(num int64) *big.Int {
	return new(big.Int).Mul(this.TokenBonus(), big.NewInt(num))
}

func (this *Airdrop) TotalGiveOut(num int64) *big.Int {
	return new(big.Int).Mul(this.TokenGiveOut(), big.NewInt(num))
}

func (this *Airdrop) TotalGiveOutDecimals(num int64) *big.Int {
	if this.Token.Decimals == 0 {
		return this.TotalGiveOut(num)
	}
	return new(big.Int).Div(this.TotalGiveOut(num), new(big.Int).SetUint64(uint64(math.Pow10(int(this.Token.Decimals)))))
}

func (this *Airdrop) TokenGiveOut() *big.Int {
	if this.Token.Decimals == 0 {
		return new(big.Int).SetUint64(this.GiveOut)
	}
	return new(big.Int).SetUint64(this.GiveOut * uint64(math.Pow10(int(this.Token.Decimals))))
}

func (this *Airdrop) CommissionFeeToWei() *big.Int {
	return new(big.Int).SetUint64(this.CommissionFee * params.Shannon)
}

func (this *Airdrop) TotalCommissionFee(num int64) *big.Int {
	return new(big.Int).Mul(this.CommissionFeeToWei(), big.NewInt(num))
}

func (this *Airdrop) TotalCommissionFeeGwei(num int64) *big.Int {
	return new(big.Int).Div(this.TotalCommissionFee(num), big.NewInt(params.Shannon))
}

func (this *Airdrop) GasPriceToWei() *big.Int {
	return new(big.Int).SetUint64(this.GasPrice * params.Shannon)
}

func (this *Airdrop) MaxGasFee() *big.Int {
	return new(big.Int).SetUint64(this.GasPrice * this.GasLimit * params.Shannon)
}

func (this *Airdrop) EnoughBudgetForSubmissions(num int64) (gasNeed *big.Int, tokenNeed *big.Int, enoughGas bool, enoughToken bool) {
	pendingSubmissions := big.NewInt(num)
	totalBonus := this.TotalTokenBonus(num)
	totalGiveOut := new(big.Int).Mul(this.TokenGiveOut(), pendingSubmissions)
	totalCommissionFee := new(big.Int).Mul(this.CommissionFeeToWei(), pendingSubmissions)
	gasNeed = new(big.Int).Add(totalCommissionFee, this.MaxGasFee())
	tokenNeed = new(big.Int).Add(totalBonus, totalGiveOut)
	if this.GasBalance.Cmp(gasNeed) != -1 {
		enoughGas = true
	}
	if this.TokenBalance.Cmp(tokenNeed) != -1 {
		enoughToken = true
	}
	return
}
