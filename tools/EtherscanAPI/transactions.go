package etherscanAPI

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"
	"strconv"
	"strings"
)

type TxListItem struct {
	BlockNumber       string `json:"blockNumber"`
	TimeStamp         string `json:"timeStamp"`
	Hash              string `json:"hash"`
	Nounce            string `json:"nounce"`
	BlockHash         string `json:"blockHash"`
	TransactionIndex  string `json:"transactionIndex"`
	From              string `json:"from"`
	To                string `json:"to"`
	Value             string `json:"value"`
	Gas               string `json:"gas"`
	GasPrice          string `json:"gasPrice"`
	IsError           string `json:"isError"`
	TxreceiptStatus   string `jsonn:"txreceipt_status"`
	GasUsed           string `json:"gasUsed"`
	CumulativeGasUsed string `json:"cumulativeGasUsed"`
	ContractAddress   string `json:"contractAddress"`
	Input             string `json:"input"`
	Confirmations     string `json:"confirmations"`
	Type              string `json:"type"`
	TraceId           string `json:"traceId"`
	ErrCode           string `json:"errCode"`
}

func (t TxListItem) ToTransaction() Transaction {
	blockNumber, _ := strconv.ParseUint(t.BlockNumber, 10, 64)
	timestamp, _ := strconv.ParseInt(t.TimeStamp, 10, 64)
	nounce, _ := strconv.ParseUint(t.Nounce, 10, 64)
	transactionIndex, _ := strconv.ParseUint(t.TransactionIndex, 10, 64)
	value, _ := strToWei(t.Value)
	gas, _ := strconv.ParseUint(t.Gas, 10, 64)
	gasPrice, _ := strToWei(t.GasPrice)
	isError, _ := strconv.ParseInt(t.IsError, 10, 64)
	txreceiptStatus, _ := strconv.ParseUint(t.TxreceiptStatus, 10, 64)
	gasUsed, _ := strconv.ParseUint(t.GasUsed, 10, 64)
	cumulativeGasUsed, _ := strconv.ParseUint(t.CumulativeGasUsed, 10, 64)
	confirmations, _ := strconv.ParseUint(t.Confirmations, 10, 64)
	traceId, _ := strconv.ParseUint(t.TraceId, 10, 64)
	return Transaction{
		BlockNumber:       blockNumber,
		TimeStamp:         timestamp,
		Hash:              t.Hash,
		Nounce:            nounce,
		BlockHash:         t.BlockHash,
		TransactionIndex:  transactionIndex,
		From:              t.From,
		To:                t.To,
		Value:             value,
		Gas:               gas,
		GasPrice:          gasPrice,
		IsError:           uint(isError),
		TxreceiptStatus:   uint(txreceiptStatus),
		GasUsed:           gasUsed,
		CumulativeGasUsed: cumulativeGasUsed,
		ContractAddress:   t.ContractAddress,
		Input:             t.Input,
		Confirmations:     confirmations,
		Type:              t.Type,
		TraceId:           traceId,
		ErrCode:           t.ErrCode,
	}
}

type Transaction struct {
	BlockNumber       uint64   `json:"blockNumber"`
	TimeStamp         int64    `json:"timeStamp"`
	Hash              string   `json:"hash"`
	Nounce            uint64   `json:"nounce,omitempty"`
	BlockHash         string   `json:"blockHash,omitempty"`
	TransactionIndex  uint64   `json:"transactionIndex,omitempty"`
	From              string   `json:"from"`
	To                string   `json:"to"`
	Value             *big.Int `json:"value"`
	Gas               uint64   `json:"gas"`
	GasPrice          *big.Int `json:"gasPrice"`
	IsError           uint     `json:"isError"`
	TxreceiptStatus   uint     `jsonn:"txreceipt_status,omitempty"`
	GasUsed           uint64   `json:"gasUsed"`
	CumulativeGasUsed uint64   `json:"cumulativeGasUsed,omitempty"`
	ContractAddress   string   `json:"contractAddress"`
	Input             string   `json:"input"`
	Confirmations     uint64   `json:"confirmations,omitempty"`
	Type              string   `json:"type,omitempty"`
	TraceId           uint64   `json:"traceId,omitempty"`
	ErrCode           string   `json:"errCode,omitempty"`
}

// TxListRec - result from Transaction Calls
//   Status  - OK / NOTOK
//   Message - error if Status NOTOK
//   Result  - TxListItem
type TxListRec struct {
	Status  string       `json:"status"`
	Message string       `json:"message"`
	Result  []TxListItem `json:"result"`
}

type ContractExecutionStatusResult struct {
	Status  string                      `json:"status"`
	Message string                      `json:"message"`
	Result  ContractExecutionStatusItem `json:"result"`
}

type ContractExecutionStatusItem struct {
	IsError        string `json:"isError"`
	ErrDescription string `json:"errDescription,omitempty"`
}

func (c ContractExecutionStatusItem) ToContractExecutionStatus() ContractExecutionStatus {
	isError, _ := strconv.ParseUint(c.IsError, 10, 64)
	return ContractExecutionStatus{
		IsError:        uint(isError),
		ErrDescription: c.ErrDescription,
	}
}

type ContractExecutionStatus struct {
	IsError        uint   `json:"isError"`
	ErrDescription string `json:"errDescription,omitempty"`
}

type TransactionReceiptStatusResult struct {
	Status  string                 `json:"status"`
	Message string                 `json:"message"`
	Result  TransactionReceiptItem `json:"result"`
}

type TransactionReceiptItem struct {
	Status string `json:"status"`
}

// [BETA] Check Contract Execution Status (if there was an error during contract execution)
// Note: isError":"0" = Pass , isError":"1" = Error during Contract Execution
// https://api.etherscan.io/api?module=transaction&action=getstatus&txhash=0x15f8e5ea1079d9a0bb04a4c58ae5fe7654b5b2b4463375ff7ffb490aa0032f3a&apikey=YourApiKeyToken
func (a *API) CheckContractExecutionStatus(txHash string) (status ContractExecutionStatus, err error) {
	call := fmt.Sprintf("https://api.etherscan.io/api?module=transaction&action=getstatus&txhash=%s&apikey=%s", txHash, a.apiKey)
	resp, err := http.Get(call)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}
	var tr ContractExecutionStatusResult
	err = json.Unmarshal(body, &tr)
	if err != nil {
		return
	}
	if strings.Compare(tr.Status, "1") != 0 {
		err = errors.New(tr.Message)
		return
	}
	return tr.Result.ToContractExecutionStatus(), nil
}

// [BETA] Check Transaction Receipt Status (Only applicable for Post Byzantium fork transactions)
// Note: status: 0 = Fail, 1 = Pass. Will return null/empty value for pre-byzantium fork
// https://api.etherscan.io/api?module=transaction&action=gettxreceiptstatus&txhash=0x513c1ba0bebf66436b5fed86ab668452b7805593c05073eb2d51d3a52f480a76&apikey=YourApiKeyToken

func (a *API) CheckTransactionReceiptStatus(txHash string) (uint, error) {
	call := fmt.Sprintf("https://api.etherscan.io/api?module=transaction&action=gettxreceiptstatus&txhash=%s&apikey=%s", txHash, a.apiKey)
	resp, err := http.Get(call)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}
	var tr TransactionReceiptStatusResult
	err = json.Unmarshal(body, &tr)
	if err != nil {
		return 0, err
	}
	if strings.Compare(tr.Status, "1") != 0 {
		return 0, errors.New(tr.Message)
	}
	status, err := strconv.ParseUint(tr.Result.Status, 10, 64)
	if err != nil {
		return 0, err
	}
	return uint(status), nil
}
