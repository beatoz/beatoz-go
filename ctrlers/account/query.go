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

	item, xerr := immuLedger.Get(btztypes.LedgerKeyAccount(req.Data))
	if xerr != nil {
		item = btztypes.NewAccount(req.Data)
	}
	if item == nil {
		return nil, xerrors.ErrQuery.Wrap(xerrors.ErrNotFoundAccount)
	}

	// NOTE
	// `Account::Balance`, which type is *uint256.Int, is marshaled to hex-string.
	// To marshal this value to decimal format...
	acct, _ := item.(*btztypes.Account)
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
