package web3

import (
	"errors"
	"github.com/beatoz/beatoz-go/ctrlers/gov/proposal"
	"github.com/beatoz/beatoz-go/ctrlers/stake"
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	bzweb3types "github.com/beatoz/beatoz-go/libs/web3/types"
	"github.com/beatoz/beatoz-go/rpc"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/holiman/uint256"
	tmjson "github.com/tendermint/tendermint/libs/json"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
	"strconv"
	"strings"
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

func (bzweb3 *BeatozWeb3) Genesis() (*coretypes.ResultGenesis, error) {
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

	return retGen, nil
}

func (bzweb3 *BeatozWeb3) GetGovParams() (*ctrlertypes.GovParams, error) {
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
func (bzweb3 *BeatozWeb3) GetAccount(addr types.Address) (*ctrlertypes.Account, error) {
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
		Address types.Address  `json:"address"`
		Name    string         `json:"name,omitempty"`
		Nonce   uint64         `json:"nonce,string"`
		Balance string         `json:"balance"`
		Code    bytes.HexBytes `json:"code,omitempty"`
		DocURL  string         `json:"docURL,omitempty"`
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

func (bzweb3 *BeatozWeb3) GetDelegatee(addr types.Address) (*stake.Delegatee, error) {
	queryResp := &rpc.QueryResult{}
	delegatee := stake.NewDelegatee(nil, nil)

	if req, err := bzweb3.NewRequest("delegatee", addr.String(), strconv.FormatInt(0, 10)); err != nil {
		panic(err)
	} else if resp, err := bzweb3.provider.Call(req); err != nil {
		return nil, err
	} else if resp.Error != nil {
		return nil, errors.New("provider error: " + string(resp.Error))
	} else if err := tmjson.Unmarshal(resp.Result, queryResp); err != nil {
		return nil, err
	} else if err := tmjson.Unmarshal(queryResp.Value, delegatee); err != nil {
		return nil, err
	} else {
		return delegatee, nil
	}
}

func (bzweb3 *BeatozWeb3) GetStakes(addr types.Address) ([]*stake.Stake, error) {
	queryResp := &rpc.QueryResult{}
	var stakes []*stake.Stake
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

func (bzweb3 *BeatozWeb3) QueryReward(addr types.Address, height int64) (*stake.Reward, error) {
	queryResp := &rpc.QueryResult{}
	rwd := stake.NewReward(addr)
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
	} else if ret, err := strconv.ParseInt(string(queryResp.Value), 10, 64); err != nil {
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
	} else if ret, err := strconv.ParseInt(string(queryResp.Value), 10, 64); err != nil {
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

func (bzweb3 *BeatozWeb3) sendTransaction(tx *ctrlertypes.Trx, method string) (*bzweb3types.JSONRpcResp, error) {

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

func (bzweb3 *BeatozWeb3) GetTransaction(txhash []byte) (*bzweb3types.TrxResult, error) {
	txRet := &bzweb3types.TrxResult{
		ResultTx: &coretypes.ResultTx{},
		TrxObj:   &ctrlertypes.Trx{},
	}

	if req, err := bzweb3.NewRequest("tx", txhash, false); err != nil {
		return nil, err
	} else if resp, err := bzweb3.provider.Call(req); err != nil {
		return nil, err
	} else if resp.Error != nil {
		return nil, errors.New("provider error: " + string(resp.Error))
	} else if err := tmjson.Unmarshal(resp.Result, txRet.ResultTx); err != nil {
		return nil, err
	} else if err := txRet.TrxObj.Decode(txRet.ResultTx.Tx); err != nil {
		return nil, err
	} else {
		return txRet, nil
	}
}

func (bzweb3 *BeatozWeb3) GetValidators(height int64, page, perPage int) (*coretypes.ResultValidators, error) {
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

func (bzweb3 *BeatozWeb3) VmCall(from, to types.Address, height int64, data []byte) (*ctrlertypes.VMCallResult, error) {
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

func (bzweb3 *BeatozWeb3) VmEstimateGas(from, to types.Address, height int64, data []byte) (*ctrlertypes.VMCallResult, error) {
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
