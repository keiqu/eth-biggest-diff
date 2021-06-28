package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const apiURL = "https://api.etherscan.io/api"

type Transaction struct {
	From  string   `json:"from"`
	To    string   `json:"to"`
	Value *big.Int `json:"value"`
}

func (t *Transaction) UnmarshalJSON(b []byte) error {
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}

	t.From, _ = m["from"].(string)
	t.To, _ = m["to"].(string)

	str, ok := m["value"].(string)
	if !ok {
		return nil
	}

	str = strings.Replace(str, "0x", "", 1)
	num, ok := new(big.Int).SetString(str, 16)
	if !ok {
		return errors.New("error creating big.Int from string")
	}

	t.Value = num
	return nil
}

type BlockInfo struct {
	Transactions []Transaction `json:"transactions"`
}

type Response struct {
	Status  string          `json:"status"`
	Message string          `json:"message"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result"`
}

type etherscanAPI struct {
	client http.Client
	token  string
}

func (api etherscanAPI) getLastBlockNum() (int64, error) {
	u, err := url.Parse(apiURL)
	if err != nil {
		return 0, err
	}

	q := url.Values{}
	q.Add("module", "proxy")
	q.Add("action", "eth_blockNumber")
	q.Add("apikey", api.token)
	u.RawQuery = q.Encode()

	apiResp, err := api.makeRequest(u.String())
	if err != nil {
		return 0, err
	}

	var hexStr string
	err = json.Unmarshal(apiResp.Result, &hexStr)
	if err != nil {
		return 0, err
	}

	hexStr = strings.Replace(hexStr, "0x", "", 1)
	num, err := strconv.ParseInt(hexStr, 16, 64)
	if err != nil {
		return 0, errors.New("error converting hex to integer")
	}

	return num, nil
}

func (api etherscanAPI) getBlockInfo(blockNum int64) (BlockInfo, error) {
	u, err := url.Parse(apiURL)
	if err != nil {
		return BlockInfo{}, err
	}

	q := url.Values{}
	q.Add("module", "proxy")
	q.Add("action", "eth_getBlockByNumber")
	q.Add("tag", fmt.Sprintf("0x%x", blockNum))
	q.Add("boolean", "true")
	q.Add("apikey", api.token)
	u.RawQuery = q.Encode()

	apiResp, err := api.makeRequest(u.String())
	if err != nil {
		return BlockInfo{}, err
	}

	var blockInfo BlockInfo
	err = json.Unmarshal(apiResp.Result, &blockInfo)
	if err != nil {
		return BlockInfo{}, err
	}

	return blockInfo, nil
}

func (api *etherscanAPI) makeRequest(u string) (Response, error) {
	resp, err := api.client.Get(u)
	if err != nil {
		return Response{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Response{}, fmt.Errorf("status code: %d; status: %s", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Response{}, err
	}

	var apiResp Response
	err = json.Unmarshal(body, &apiResp)
	if err != nil {
		return Response{}, err
	}

	if apiResp.Status != "" {
		return Response{}, fmt.Errorf("api error: status '%s'; message '%s'; result '%s'",
			apiResp.Status, apiResp.Message, apiResp.Result)
	}

	return apiResp, nil
}
