package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strconv"

	"golang.org/x/time/rate"
)

var errAPITimeout = errors.New("too many requests to api")

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

	v, ok := m["from"].(string)
	if !ok {
		return errors.New("error asserting type of 'from' to string")
	}
	t.From = v

	// 'to' can be null if transaction creates smart contract
	t.To, _ = m["to"].(string)

	str, ok := m["value"].(string)
	if !ok {
		return nil
	}

	num, ok := new(big.Int).SetString(str, 0)
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
	client      *http.Client
	rateLimiter *rate.Limiter
	token       string
}

func (api *etherscanAPI) getLastBlockNum() (uint64, error) {
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

	num, err := strconv.ParseUint(hexStr, 0, 64)
	if err != nil {
		return 0, errors.New("error converting hex to integer")
	}

	return num, nil
}

func (api *etherscanAPI) getBlockInfo(blockNum uint64) (BlockInfo, error) {
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
	ctx := context.Background()
	err := api.rateLimiter.Wait(ctx)
	if err != nil {
		return Response{}, err
	}

	resp, err := api.client.Get(u)
	if err != nil {
		return Response{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Response{}, fmt.Errorf("error: status code %d; status '%s'", resp.StatusCode, resp.Status)
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
		return Response{}, errAPITimeout
	}

	return apiResp, nil
}
