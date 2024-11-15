package account

import (
	btztypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	tmjson "github.com/tendermint/tendermint/libs/json"
)

func (ctrler *AcctCtrler) Query(req abcitypes.RequestQuery) ([]byte, xerrors.XError) {
	immuLedger, xerr := ctrler.acctState.ImitableLedgerAt(req.Height)
	if xerr != nil {
		return nil, xerrors.ErrQuery.Wrap(xerr)
	}

	var acct *btztypes.Account
	item, xerr := immuLedger.Get(req.Data)
	if xerr != nil {
		acct = btztypes.NewAccount(req.Data)
	} else {
		acct = item.(*btztypes.Account)
	}

	//acct, xerr := ctrler.acctState.Get(req.Data, false)
	//if xerr != nil {
	//	acct = btztypes.NewAccount(req.Data)
	//}

	// NOTE
	// `Account::Balance`, which type is *uint256.Int, is marshaled to hex-string.
	// To marshal this value to decimal format...
	_acct := &struct {
		Address types.Address  `json:"address"`
		Name    string         `json:"name,omitempty"`
		Nonce   uint64         `json:"nonce,string"`
		Balance string         `json:"balance"`
		Code    bytes.HexBytes `json:"code,omitempty"`
		DocURL  string         `json:"docURL,omitempty"`
	}{
		Address: acct.Address,
		Name:    acct.Name,
		Nonce:   acct.Nonce,
		Balance: acct.Balance.Dec(),
		Code:    acct.Code,
		DocURL:  acct.DocURL,
	}
	if raw, err := tmjson.Marshal(_acct); err != nil {
		return nil, xerrors.ErrQuery.Wrap(err)
	} else {
		return raw, nil
	}
}
