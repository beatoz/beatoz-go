package web3

import (
	"errors"
	"strconv"
	"strings"

	"github.com/beatoz/beatoz-go/ctrlers/gov/proposal"
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/rpc"
	btztypes "github.com/beatoz/beatoz-go/types"
	btzbytes "github.com/beatoz/beatoz-go/types/bytes"
	"github.com/holiman/uint256"
	tmjson "github.com/tendermint/tendermint/libs/json"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
	tmtypes "github.com/tendermint/tendermint/types"
)

func (bzweb3 *BeatozWeb3) Status() (*coretypes.ResultStatus, error) {
	retStatus := &coretypes.ResultStatus{}

	if req, err := bzweb3.NewRequest("status"); err != nil {
		panic(err)
	} else if resp, err := bzweb3.provider.Call(req); err != nil {
		return nil, err
	} else if resp.Error != nil {
		return nil, errors.New("provider error: " + string(resp.Error))
	} else if err := tmjson.Unmarshal(resp.Result, retStatus); err != nil {
		return nil, err
	}

	return retStatus, nil
}

func (bzweb3 *BeatozWeb3) Genesis() (*tmtypes.GenesisDoc, error) {
	retGen := &coretypes.ResultGenesis{}

	if req, err := bzweb3.NewRequest("genesis"); err != nil {
		panic(err)
	} else if resp, err := bzweb3.provider.Call(req); err != nil {
		return nil, err
	} else if resp.Error != nil {
		return nil, errors.New("provider error: " + string(resp.Error))
	} else if err := tmjson.Unmarshal(resp.Result, retGen); err != nil {
		return nil, err
	}

	return retGen.Genesis, nil
}

// DEPRECATED: Use QueryGovParams instead
func (bzweb3 *BeatozWeb3) GetGovParams() (*ctrlertypes.GovParams, error) {
	return bzweb3.QueryGovParams()
}

func (bzweb3 *BeatozWeb3) QueryGovParams() (*ctrlertypes.GovParams, error) {
	queryResp := &rpc.QueryResult{}

	if req, err := bzweb3.NewRequest("gov_params", strconv.FormatInt(0, 10)); err != nil {
		panic(err)
	} else if resp, err := bzweb3.provider.Call(req); err != nil {
		return nil, err
	} else if resp.Error != nil {
		return nil, errors.New("provider error: " + string(resp.Error))
	} else if err := tmjson.Unmarshal(resp.Result, queryResp); err != nil {
		return nil, err
	}

	govParams := &ctrlertypes.GovParams{}
	if err := tmjson.Unmarshal(queryResp.Value, govParams); err != nil {
		return nil, err
	}
	return govParams, nil

}

// DEPRECATED: Use QueryAccount instead
func (bzweb3 *BeatozWeb3) GetAccount(addr btztypes.Address) (*ctrlertypes.Account, error) {
	return bzweb3.QueryAccount(addr)
}

func (bzweb3 *BeatozWeb3) QueryAccount(addr btztypes.Address) (*ctrlertypes.Account, error) {
	queryResp := &rpc.QueryResult{}

	if req, err := bzweb3.NewRequest("account", addr.String(), strconv.FormatInt(0, 10)); err != nil {
		panic(err)
	} else if resp, err := bzweb3.provider.Call(req); err != nil {
		return nil, err
	} else if resp.Error != nil {
		return nil, errors.New("provider error: " + string(resp.Error))
	} else if err := tmjson.Unmarshal(resp.Result, queryResp); err != nil {
		return nil, err
	}

	_acct := &struct {
		Address btztypes.Address  `json:"address"`
		Name    string            `json:"name,omitempty"`
		Nonce   int64             `json:"nonce,string"`
		Balance string            `json:"balance"`
		Code    btzbytes.HexBytes `json:"code,omitempty"`
		DocURL  string            `json:"docURL,omitempty"`
	}{}

	if err := tmjson.Unmarshal(queryResp.Value, _acct); err != nil {
		return nil, err
	} else {
		var bal *uint256.Int
		if strings.HasPrefix(_acct.Balance, "0x") {
			bal = uint256.MustFromHex(_acct.Balance)
		} else {
			bal = uint256.MustFromDecimal(_acct.Balance)
		}

		return &ctrlertypes.Account{
			Address: _acct.Address,
			Name:    _acct.Name,
			Nonce:   _acct.Nonce,
			Balance: bal,
			Code:    _acct.Code,
			DocURL:  _acct.DocURL,
		}, nil
	}
}

func (bzweb3 *BeatozWeb3) QueryDelegatee(addr btztypes.Address) (*RespQueryDelegatee, error) {
	queryResp := &rpc.QueryResult{}
	dgtee := &RespQueryDelegatee{}

	if req, err := bzweb3.NewRequest("delegatee", addr.String(), strconv.FormatInt(0, 10)); err != nil {
		panic(err)
	} else if resp, err := bzweb3.provider.Call(req); err != nil {
		return nil, err
	} else if resp.Error != nil {
		return nil, errors.New("provider error: " + string(resp.Error))
	} else if err := tmjson.Unmarshal(resp.Result, queryResp); err != nil {
		return nil, err
	} else if err := tmjson.Unmarshal(queryResp.Value, dgtee); err != nil {
		return nil, err
	} else {
		return dgtee, nil
	}
}

func (bzweb3 *BeatozWeb3) QueryStakes(addr btztypes.Address) ([]*RespQueryStake, error) {
	queryResp := &rpc.QueryResult{}
	var stakes []*RespQueryStake
	if req, err := bzweb3.NewRequest("stakes", addr.String(), strconv.FormatInt(0, 10)); err != nil {
		panic(err)
	} else if resp, err := bzweb3.provider.Call(req); err != nil {
		return nil, err
	} else if resp.Error != nil {
		return nil, errors.New("provider error: " + string(resp.Error))
	} else if err := tmjson.Unmarshal(resp.Result, queryResp); err != nil {
		return nil, err
	} else if err := tmjson.Unmarshal(queryResp.Value, &stakes); err != nil {
		return nil, err
	} else {
		return stakes, nil
	}
}

func (bzweb3 *BeatozWeb3) QueryReward(addr btztypes.Address, height int64) (*RespQueryReward, error) {
	queryResp := &rpc.QueryResult{}
	rwd := &RespQueryReward{}
	if req, err := bzweb3.NewRequest("reward", addr.String(), strconv.FormatInt(height, 10)); err != nil {
		panic(err)
	} else if resp, err := bzweb3.provider.Call(req); err != nil {
		return nil, err
	} else if resp.Error != nil {
		return nil, errors.New("provider error: " + string(resp.Error))
	} else if err := tmjson.Unmarshal(resp.Result, queryResp); err != nil {
		return nil, err
	} else if err := tmjson.Unmarshal(queryResp.Value, rwd); err != nil {
		return nil, err
	} else {
		return rwd, nil
	}
}

func (bzweb3 *BeatozWeb3) QueryTotalPower(height int64) (int64, error) {
	queryResp := &rpc.QueryResult{}
	if req, err := bzweb3.NewRequest("stakes/total_power", strconv.FormatInt(height, 10)); err != nil {
		panic(err)
	} else if resp, err := bzweb3.provider.Call(req); err != nil {
		return -1, err
	} else if resp.Error != nil {
		return -1, errors.New("provider error: " + string(resp.Error))
	} else if err := tmjson.Unmarshal(resp.Result, queryResp); err != nil {
		return -1, err
	} else if ret, err := strconv.ParseInt(strings.Trim(string(queryResp.Value), `"`), 10, 64); err != nil {
		return -1, err
	} else {
		return ret, nil
	}
}

func (bzweb3 *BeatozWeb3) QueryValidatorPower(height int64) (int64, error) {
	return bzweb3.QueryVotingPower(height)
}
func (bzweb3 *BeatozWeb3) QueryVotingPower(height int64) (int64, error) {
	queryResp := &rpc.QueryResult{}
	if req, err := bzweb3.NewRequest("stakes/voting_power", strconv.FormatInt(height, 10)); err != nil {
		panic(err)
	} else if resp, err := bzweb3.provider.Call(req); err != nil {
		return -1, err
	} else if resp.Error != nil {
		return -1, errors.New("provider error: " + string(resp.Error))
	} else if err := tmjson.Unmarshal(resp.Result, queryResp); err != nil {
		return -1, err
	} else if ret, err := strconv.ParseInt(strings.Trim(string(queryResp.Value), `"`), 10, 64); err != nil {
		return -1, err
	} else {
		return ret, nil
	}
}

type QueryProposalResult struct {
	Status   string                `json:"status"`
	Proposal *proposal.GovProposal `json:"proposal"`
}

func (bzweb3 *BeatozWeb3) QueryProposal(txhash []byte, height int64) (*QueryProposalResult, error) {
	ret := &QueryProposalResult{}
	queryResp := &rpc.QueryResult{}
	if req, err := bzweb3.NewRequest("proposal", txhash, strconv.FormatInt(height, 10)); err != nil {
		panic(err)
	} else if resp, err := bzweb3.provider.Call(req); err != nil {
		return nil, err
	} else if resp.Error != nil {
		return nil, errors.New("provider error: " + string(resp.Error))
	} else if err := tmjson.Unmarshal(resp.Result, queryResp); err != nil {
		return nil, err
	} else if err := tmjson.Unmarshal(queryResp.Value, ret); err != nil {
		return nil, err
	} else {
		return ret, nil
	}
}

func (bzweb3 *BeatozWeb3) SendTransactionAsync(tx *ctrlertypes.Trx) (*coretypes.ResultBroadcastTx, error) {
	resp, err := bzweb3.sendTransaction(tx, "broadcast_tx_async")
	if err != nil {
		return nil, err
	}

	ret := &coretypes.ResultBroadcastTx{}
	if err := tmjson.Unmarshal(resp.Result, ret); err != nil {
		return nil, err
	}
	return ret, nil
}
func (bzweb3 *BeatozWeb3) SendTransactionSync(tx *ctrlertypes.Trx) (*coretypes.ResultBroadcastTx, error) {
	resp, err := bzweb3.sendTransaction(tx, "broadcast_tx_sync")
	if err != nil {
		return nil, err
	}

	ret := &coretypes.ResultBroadcastTx{}
	if err := tmjson.Unmarshal(resp.Result, ret); err != nil {
		return nil, err
	}
	return ret, nil
}
func (bzweb3 *BeatozWeb3) SendTransactionCommit(tx *ctrlertypes.Trx) (*coretypes.ResultBroadcastTxCommit, error) {
	resp, err := bzweb3.sendTransaction(tx, "broadcast_tx_commit")
	if err != nil {
		return nil, err
	}

	ret := &coretypes.ResultBroadcastTxCommit{}
	if err := tmjson.Unmarshal(resp.Result, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

func (bzweb3 *BeatozWeb3) sendTransaction(tx *ctrlertypes.Trx, method string) (*JSONRpcResp, error) {

	if txbz, err := tx.Encode(); err != nil {
		return nil, err
	} else if req, err := bzweb3.NewRequest(method, txbz); err != nil {
		return nil, err
	} else if resp, err := bzweb3.provider.Call(req); err != nil {
		return nil, err
	} else if resp.Error != nil {
		return nil, errors.New("provider error: " + string(resp.Error))
	} else {
		return resp, nil
	}
}

// DEPRECATED: Use QueryTransaction instead
func (bzweb3 *BeatozWeb3) GetTransaction(txhash []byte) (*TrxResult, error) {
	return bzweb3.QueryTransaction(txhash)
}
func (bzweb3 *BeatozWeb3) QueryTransaction(txhash []byte) (*TrxResult, error) {
	resultTx := &coretypes.ResultTx{}
	txObj := &ctrlertypes.Trx{}

	if req, err := bzweb3.NewRequest("tx", txhash, false); err != nil {
		return nil, err
	} else if resp, err := bzweb3.provider.Call(req); err != nil {
		return nil, err
	} else if resp.Error != nil {
		return nil, errors.New("provider error: " + string(resp.Error))
	} else if err := tmjson.Unmarshal(resp.Result, resultTx); err != nil {
		return nil, err
	} else if err := txObj.Decode(resultTx.Tx); err != nil {
		return nil, err
	} else {
		return &TrxResult{
			Hash:     resultTx.Hash,
			Height:   resultTx.Height,
			Index:    resultTx.Index,
			TxResult: resultTx.TxResult,
			Proof:    resultTx.Proof,
			Tx:       txObj,
		}, nil
	}
}

// DEPRECATED: Use QueryValidators instead
func (bzweb3 *BeatozWeb3) GetValidators(height int64, page, perPage int) (*coretypes.ResultValidators, error) {
	return bzweb3.QueryValidators(height, page, perPage)
}

func (bzweb3 *BeatozWeb3) QueryValidators(height int64, page, perPage int) (*coretypes.ResultValidators, error) {

	retVals := &coretypes.ResultValidators{}

	_height := "0"
	if height > 0 {
		_height = strconv.FormatInt(height, 10)
	}

	if page == 0 {
		page = 1
	}
	_page := strconv.Itoa(page)
	_perPage := strconv.Itoa(perPage)

	if req, err := bzweb3.NewRequest("validators", _height, _page, _perPage); err != nil {
		return nil, err
	} else if resp, err := bzweb3.provider.Call(req); err != nil {
		return nil, err
	} else if resp.Error != nil {
		return nil, errors.New("provider error: " + string(resp.Error))
	} else if err := tmjson.Unmarshal(resp.Result, retVals); err != nil {
		return nil, err
	}
	return retVals, nil
}

func (bzweb3 *BeatozWeb3) VmCall(from, to btztypes.Address, height int64, data []byte) (*ctrlertypes.VMCallResult, error) {
	req, err := bzweb3.NewRequest("vm_call", from, to, strconv.FormatInt(height, 10), data)
	if err != nil {
		return nil, err
	}
	resp, err := bzweb3.provider.Call(req)
	if err != nil {
		return nil, err
	} else if resp.Error != nil {
		return nil, errors.New("provider error: " + string(resp.Error))
	}

	qryResp := &rpc.QueryResult{}
	if err := tmjson.Unmarshal(resp.Result, qryResp); err != nil {
		return nil, err
	}

	if qryResp.Code != 0 {
		return nil, errors.New(qryResp.Log)
	}

	vmRet := &ctrlertypes.VMCallResult{}
	if err := tmjson.Unmarshal(qryResp.Value, vmRet); err != nil {
		return nil, err
	}
	return vmRet, nil
}

func (bzweb3 *BeatozWeb3) VmEstimateGas(from, to btztypes.Address, height int64, data []byte) (*ctrlertypes.VMCallResult, error) {
	req, err := bzweb3.NewRequest("vm_estimate_gas", from, to, strconv.FormatInt(height, 10), data)
	if err != nil {
		return nil, err
	}
	resp, err := bzweb3.provider.Call(req)
	if err != nil {
		return nil, err
	} else if resp.Error != nil {
		return nil, errors.New("provider error: " + string(resp.Error))
	}

	qryResp := &rpc.QueryResult{}
	if err := tmjson.Unmarshal(resp.Result, qryResp); err != nil {
		return nil, err
	}

	if qryResp.Code != 0 {
		return nil, errors.New(qryResp.Log)
	}

	vmRet := &ctrlertypes.VMCallResult{}
	if err := tmjson.Unmarshal(qryResp.Value, vmRet); err != nil {
		return nil, err
	}
	return vmRet, nil
}
