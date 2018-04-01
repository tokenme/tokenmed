package etherscanAPI

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"
	"strings"
)

type TokenSupplyResult struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Result  string `json:"result"`
}

// Get ERC20-Token TotalSupply by ContractAddress
func (a *API) TokenSupply(addr string) (val *big.Int, err error) {
	call := fmt.Sprintf("http://api.etherscan.io/api?module=stats&action=tokensupply&contractaddress=%s&apikey=%s", addr, a.apiKey)
	resp, err := http.Get(call)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var tr TokenSupplyResult
	err = json.Unmarshal(body, &tr)
	if err != nil {
		return nil, err
	}
	if strings.Compare(tr.Status, "1") != 0 {
		err = errors.New(tr.Message)
		return nil, err
	}
	var ok bool
	val, ok = strToWei(tr.Result)
	if !ok {
		err = errors.New("error understanding " + tr.Result)
	}
	return
}

// Get ERC20-Token Account Balance for TokenContractAddress
func (a *API) TokenAccountBalance(addr string, account string) (ret AccountBalance, err error) {
	call := fmt.Sprintf("http://api.etherscan.io/api?module=account&action=tokenbalance&contractaddress=%s&address=%s&tag=latest&apikey=%s", addr, account, a.apiKey)
	resp, err := http.Get(call)
	if err != nil {
		fmt.Println(err)
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
