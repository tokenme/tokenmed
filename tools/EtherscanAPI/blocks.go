package etherscanAPI

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
)

// [BETA] Get Block And Uncle Rewards by BlockNo
// https://api.etherscan.io/api?module=block&action=getblockreward&blockno=2165403&apikey=YourApiKeyToken
func (a *API) GetBlockRewords(blockNo uint64) ([]MinedBlock, error) {
	call := fmt.Sprintf("https://api.etherscan.io/api?module=block&action=getblockreward&blockno=%d&apikey=%s", blockNo, a.apiKey)
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
