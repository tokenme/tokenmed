package etherscanAPI

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// Get Ether Balance for a single Address
// https://api.etherscan.io/api?module=account&action=balance&address=0xde0b295669a9fd93d5f28d9ec85e40f4cb697bae&tag=latest&apikey=YourApiKeyToken
type AccountBalanceResult struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Result  string `json:"result"`
}

type MultiAccountBalance struct {
	Status  string           `json:"status"`
	Message string           `json:"message"`
	Result  []accountBalance `json:"result"`
}

type accountBalance struct {
	Account string `json:"accounnt"`
	Balance string `json:"balance"`
}

func (a accountBalance) ToAccountBalance() (account AccountBalance, err error) {
	balance, ok := strToWei(a.Balance)
	if !ok {
		err = errors.New("error in number " + a.Balance)
		return
	}
	return AccountBalance{Account: a.Account, Balance: balance}, nil
}

type AccountBalance struct {
	Account string   `json:"accounnt"`
	Balance *big.Int `json:"balance"`
}

type GetTransactionsRequest struct {
	Address    string `json:"address"`
	StartBlock uint64 `json:"start_block,omitempty"`
	EndBlock   uint64 `json:"end_block,omitempty"`
	Page       uint   `json:"page,omitempty"`
	Offset     uint   `json:"offset,omitempty"`
	Sort       string `json:"sort,omitempty"`
}

type GetMinedBlocksRequest struct {
	Address   string `json:"address"`
	BlockType string `json:"type"`
	Page      uint   `json:"page,omitempty"`
	Offset    uint   `json:"offset,omitempty"`
}

type MinedBlockRec struct {
	Status  string           `json:"status"`
	Message string           `json:"message"`
	Result  []MinedBlockItem `json:"result"`
}

type MinedBlockItem struct {
	BlockNumber string           `json:"blockNumber"`
	TimeStamp   string           `json:"timeStamp"`
	BlockReward string           `json:"blockReward"`
	BlockMiner  string           `json:"blockMiner,omitempty"`
	Uncles      []BlockUncleItem `json:"uncles,omitempty"`
}

type BlockUncleItem struct {
	Miner         string `json:"miner"`
	UnclePosition string `json:"unclePosition"`
	BlockReward   string `json:"blockreward"`
}

func (b BlockUncleItem) ToBlockUncle() BlockUncle {
	unclePosition, _ := strconv.ParseUint(b.UnclePosition, 10, 64)
	blockReward, _ := strToWei(b.BlockReward)
	return BlockUncle{
		Miner:         b.Miner,
		UnclePosition: unclePosition,
		BlockReward:   blockReward,
	}
}

type BlockUncle struct {
	Miner         string   `json:"miner"`
	UnclePosition uint64   `json:"unclePosition"`
	BlockReward   *big.Int `json:"blockreward"`
}

func (m MinedBlockItem) ToMinedBlock() MinedBlock {
	blockNumber, _ := strconv.ParseUint(m.BlockNumber, 10, 64)
	timestamp, _ := strconv.ParseInt(m.TimeStamp, 10, 64)
	blockReward, _ := strToWei(m.BlockReward)
	var uncles []BlockUncle
	for _, b := range m.Uncles {
		uncles = append(uncles, b.ToBlockUncle())
	}
	return MinedBlock{
		BlockNumber: blockNumber,
		TimeStamp:   timestamp,
		BlockReward: blockReward,
		BlockMiner:  m.BlockMiner,
		Uncles:      uncles,
	}
}

type MinedBlock struct {
	BlockNumber uint64       `json:"blockNumber"`
	TimeStamp   int64        `json:"timeStamp"`
	BlockReward *big.Int     `json:"blockReward"`
	BlockMiner  string       `json:"blockMiner,omitempty"`
	Uncles      []BlockUncle `json:"uncles,omitempty"`
}

func (a *API) GetEtherBalance(addr string) (ret AccountBalance, err error) {
	call := fmt.Sprintf("http://api.etherscan.io/api?module=account&action=balance&address=%s&tag=latest&apikey=%s", addr, a.apiKey)
	resp, err := http.Get(call)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}
	var tr AccountBalanceResult
	err = json.Unmarshal(body, &tr)
	if err != nil {
		return
	}
	if strings.Compare(tr.Status, "1") != 0 {
		err = errors.New(tr.Message)
		return
	}
	balance, ok := strToWei(tr.Result)
	if !ok {
		err = errors.New("error in number " + tr.Result)
		return
	}
	return AccountBalance{Account: addr, Balance: balance}, nil
}

// Get Ether Balance for multiple Addresses in a single call
// https://api.etherscan.io/api?module=account&action=balancemulti&address=0xddbd2b932c763ba5b1b7ae3b362eac3e8d40121a,0x63a9975ba31b0b9626b34300f7f627147df1f526,0x198ef1ec325a96cc354c7266a038be8b5c558f67&tag=latest&apikey=YourApiKeyToken
func (a *API) GetMultiEtherBalances(addr []string) ([]AccountBalance, error) {
	addresses := strings.Join(addr, ",")
	call := fmt.Sprintf("http://api.etherscan.io/api?module=account&action=balancemulti&address=%s&tag=latest&apikey=%s", addresses, a.apiKey)
	resp, err := http.Get(call)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var tr MultiAccountBalance
	err = json.Unmarshal(body, &tr)
	if err != nil {
		return nil, err
	}
	if strings.Compare(tr.Status, "1") != 0 {
		return nil, errors.New(tr.Message)
	}
	var accounts []AccountBalance
	for _, a := range tr.Result {
		balance, err := a.ToAccountBalance()
		if err != nil {
			continue
		}
		accounts = append(accounts, balance)
	}
	return accounts, nil
}

// Get a list of 'Normal' Transactions By Address
// [Optional Parameters] startblock: starting blockNo to retrieve results, endblock: ending blockNo to retrieve results
// ([BETA] Returned 'isError' values: 0=No Error, 1=Got Error)
// (Returns up to a maximum of the last 10000 transactions only)
// https://api.etherscan.io/api?module=account&action=txlist&address=0xddbd2b932c763ba5b1b7ae3b362eac3e8d40121a&startblock=0&endblock=99999999&page=1&offset=10&sort=asc&apikey=YourApiKeyToken
func (a *API) GetNormalTransactions(req GetTransactionsRequest) ([]Transaction, error) {
	values := url.Values{}
	values.Set("module", "account")
	values.Set("action", "txlist")
	values.Set("address", req.Address)
	values.Set("sort", "asc")
	values.Set("page", "1")
	values.Set("offset", "1000")
	values.Set("apikey", a.apiKey)
	if req.Sort != "" {
		values.Set("sort", req.Sort)
	}
	if req.Page > 0 {
		values.Set("page", fmt.Sprintf("%d", req.Page))
	}
	if req.Offset > 0 {
		values.Set("offset", fmt.Sprintf("%d", req.Offset))
	}
	if req.StartBlock > 0 {
		values.Set("startblock", strconv.FormatUint(req.StartBlock, 10))
	}
	if req.EndBlock > 0 {
		values.Set("endblock", strconv.FormatUint(req.EndBlock, 10))
	}
	call := fmt.Sprintf("https://api.etherscan.io/api?%s", values.Encode())
	resp, err := http.Get(call)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var tr TxListRec
	err = json.Unmarshal(body, &tr)
	if err != nil {
		return nil, err
	}
	if strings.Compare(tr.Status, "1") != 0 {
		return nil, errors.New(tr.Message)
	}
	var transactions []Transaction
	for _, tx := range tr.Result {
		transactions = append(transactions, tx.ToTransaction())
	}
	return transactions, nil
}

// Get a list of 'Internal' Transactions by Address
// [Optional Parameters] startblock: starting blockNo to retrieve results, endblock: ending blockNo to retrieve results
// ([BETA] Returned 'isError' values: 0=No Error, 1=Got Error)
// (Returns up to a maximum of the last 10000 transactions only)
// https://api.etherscan.io/api?module=account&action=txlistinternal&address=0x2c1ba59d6f58433fb1eaee7d20b26ed83bda51a3&startblock=0&endblock=2702578&page=1&offset=10&sort=asc&apikey=YourApiKeyToken
func (a *API) GetInternalTransactions(req GetTransactionsRequest) ([]Transaction, error) {
	values := url.Values{}
	values.Set("module", "account")
	values.Set("action", "txlistinternal")
	values.Set("address", req.Address)
	values.Set("sort", "asc")
	values.Set("page", "1")
	values.Set("offset", "1000")
	values.Set("apikey", a.apiKey)
	if req.Sort != "" {
		values.Set("sort", req.Sort)
	}
	if req.Page > 0 {
		values.Set("page", fmt.Sprintf("%d", req.Page))
	}
	if req.Offset > 0 {
		values.Set("offset", fmt.Sprintf("%d", req.Offset))
	}
	if req.StartBlock > 0 {
		values.Set("startblock", strconv.FormatUint(req.StartBlock, 10))
	}
	if req.EndBlock > 0 {
		values.Set("endblock", strconv.FormatUint(req.EndBlock, 10))
	}
	call := fmt.Sprintf("https://api.etherscan.io/api?%s", values.Encode())
	resp, err := http.Get(call)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var tr TxListRec
	err = json.Unmarshal(body, &tr)
	if err != nil {
		return nil, err
	}
	if strings.Compare(tr.Status, "1") != 0 {
		return nil, errors.New(tr.Message)
	}
	var transactions []Transaction
	for _, tx := range tr.Result {
		transactions = append(transactions, tx.ToTransaction())
	}
	return transactions, nil
}

// Get "Internal Transactions" by Transaction Hash
// (Returned 'isError' values: 0=Ok, 1=Rejected/Cancelled)
// (Returns up to a maximum of the last 10000 transactions only)
// https://api.etherscan.io/api?module=account&action=txlistinternal&txhash=0x40eb908387324f2b575b4879cd9d7188f69c8fc9d87c901b9e2daaea4b442170&apikey=YourApiKeyToken
func (a *API) GetInternalTransactionsByHash(txHash string) ([]Transaction, error) {
	call := fmt.Sprintf("https://api.etherscan.io/api?module=account&action=txlistinternal&txhash=%s&apikey=%s", txHash, a.apiKey)
	resp, err := http.Get(call)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var tr TxListRec
	err = json.Unmarshal(body, &tr)
	if err != nil {
		return nil, err
	}
	if strings.Compare(tr.Status, "1") != 0 {
		return nil, errors.New(tr.Message)
	}
	var transactions []Transaction
	for _, tx := range tr.Result {
		transactions = append(transactions, tx.ToTransaction())
	}
	return transactions, nil
}

// Get list of Blocks Mined by Address
// (To get paginated results use page=<page number> and offset=<max records to return>)
// ** type = blocks (full blocks only) or uncles (uncle blocks only)
// https://api.etherscan.io/api?module=account&action=txlistinternal&txhash=0x40eb908387324f2b575b4879cd9d7188f69c8fc9d87c901b9e2daaea4b442170&apikey=YourApiKeyToken
func (a *API) GetMinedBlocks(req GetMinedBlocksRequest) ([]MinedBlock, error) {
	blockType := "blocks"
	if req.BlockType != "" {
		blockType = req.BlockType
	}
	call := fmt.Sprintf("https://api.etherscan.io/api?module=account&action=getminedblocks&address=%s&blocktype=%s&page=%d&offset=%d&apikey=%s", req.Address, blockType, req.Page, req.Offset, a.apiKey)
	resp, err := http.Get(call)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var tr MinedBlockRec
	err = json.Unmarshal(body, &tr)
	if err != nil {
		return nil, err
	}
	if strings.Compare(tr.Status, "1") != 0 {
		return nil, errors.New(tr.Message)
	}
	var blocks []MinedBlock
	for _, b := range tr.Result {
		blocks = append(blocks, b.ToMinedBlock())
	}
	return blocks, nil
}
