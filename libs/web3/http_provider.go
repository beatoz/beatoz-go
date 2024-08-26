package web3

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/beatoz/beatoz-go/libs/web3/types"
	"io/ioutil"
	"net/http"
	"sync"
)

type HttpProvider struct {
	url string
	//httpClient *http.Client

	mtx sync.RWMutex
}

func NewHttpProvider(url string, opts ...func(*HttpProvider)) *HttpProvider {
	ret := &HttpProvider{
		url: url,
		//httpClient: &http.Client{
		//	//Timeout: time.Second * time.Duration(10), // for [connect ~ request ~ response] time
		//	Transport: &http.Transport{
		//		DisableKeepAlives: false,
		//		IdleConnTimeout:   time.Minute,
		//		MaxConnsPerHost:   100,
		//	},
		//},
	}

	for _, cb := range opts {
		cb(ret)
	}
	return ret
}

func (client *HttpProvider) Call(req *types.JSONRpcReq) (*types.JSONRpcResp, error) {
	client.mtx.Lock()
	defer client.mtx.Unlock()

	reqbz, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpBody := bytes.NewBuffer(reqbz)
	httpResp, err := http.Post(client.url, "application/json", httpBody)
	if err != nil {
		return nil, err
	}

	defer func() {
		httpResp.Body.Close()
	}()

	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Bad HTTP Response: %v", httpResp.Status)
	}

	respBody, err := ioutil.ReadAll(httpResp.Body)
	if err != nil {
		return nil, err
	}

	res := &types.JSONRpcResp{}
	if err = json.Unmarshal(respBody, res); err != nil {
		return nil, err
	}
	return res, nil
}
