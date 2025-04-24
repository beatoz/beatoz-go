package xerrors

import (
	"errors"
	"fmt"
	abcitypes "github.com/tendermint/tendermint/abci/types"
)

const (
	ErrCodeSuccess uint32 = abcitypes.CodeTypeOK + iota
	ErrCodeOrdinary
	ErrCodeInitChain
	ErrCodeCheckTx
	ErrCodeBeginBlock
	ErrCodeDeliverTx
	ErrCodeEndBlock
	ErrCodeCommit
	ErrCodeNotFoundAccount
	ErrCodeInvalidAccountType
	ErrCodeInvalidTrx
	ErrCodeNotFoundTx
	ErrCodeNotFoundDelegatee
	ErrCodeNotFoundStake
	ErrCodeNotFoundProposal
	ErrCodeNotFoundVoter
	ErrCodeNotFoundReward
	ErrCodeUpdatableStakeRatio
)

const (
	ErrCodeQuery uint32 = 1000 + iota
	ErrCodeInvalidQueryPath
	ErrCodeInvalidQueryParams
	ErrCodeNotFoundResult
	ErrLast
)

var (
	ErrCommon     = New(ErrCodeOrdinary, "beatoz error")
	ErrOverFlow   = New(ErrCodeOrdinary, "overflow")
	ErrInitChain  = New(ErrCodeInitChain, "InitChain failed")
	ErrCheckTx    = New(ErrCodeCheckTx, "CheckTx failed")
	ErrBeginBlock = New(ErrCodeBeginBlock, "BeginBlock failed")
	ErrDeliverTx  = New(ErrCodeDeliverTx, "DeliverTx failed")
	ErrEndBlock   = New(ErrCodeEndBlock, "EndBlock failed")
	ErrCommit     = New(ErrCodeCommit, "Commit failed")
	ErrQuery      = New(ErrCodeQuery, "query failed")

	ErrNotFoundAccount         = New(ErrCodeNotFoundAccount, "not found account")
	ErrInvalidAccountType      = New(ErrCodeInvalidAccountType, "invalid account type")
	ErrInvalidTrx              = New(ErrCodeInvalidTrx, "invalid transaction")
	ErrNegGas                  = ErrInvalidTrx.Wrap(NewOrdinary("negative gas"))
	ErrInvalidGas              = ErrInvalidTrx.Wrap(NewOrdinary("invalid gas"))
	ErrInvalidGasPrice         = ErrInvalidTrx.Wrap(NewOrdinary("invalid gas price"))
	ErrInvalidAddress          = ErrInvalidTrx.Wrap(NewOrdinary("invalid address"))
	ErrInvalidNonce            = ErrInvalidTrx.Wrap(NewOrdinary("invalid nonce"))
	ErrInvalidAmount           = ErrInvalidTrx.Wrap(NewOrdinary("invalid amount"))
	ErrInsufficientFund        = ErrInvalidTrx.Wrap(NewOrdinary("insufficient fund"))
	ErrInvalidTrxType          = ErrInvalidTrx.Wrap(NewOrdinary("wrong transaction type"))
	ErrInvalidTrxPayloadType   = ErrInvalidTrx.Wrap(NewOrdinary("wrong transaction payload type"))
	ErrInvalidTrxPayloadParams = ErrInvalidTrx.Wrap(NewOrdinary("invalid params of transaction payload"))
	ErrInvalidTrxSig           = ErrInvalidTrx.Wrap(NewOrdinary("invalid signature"))
	ErrNotFoundTx              = New(ErrCodeNotFoundTx, "not found tx")
	ErrNotFoundDelegatee       = New(ErrCodeNotFoundDelegatee, "not found delegatee")
	ErrNotFoundStake           = New(ErrCodeNotFoundStake, "not found stake")
	ErrNotFoundProposal        = New(ErrCodeNotFoundProposal, "not found proposal")
	ErrNotFoundVoter           = New(ErrCodeNotFoundVoter, "not found voter")
	ErrNotFoundReward          = New(ErrCodeNotFoundReward, "not found reward")
	ErrUpdatableStakeRatio     = New(ErrCodeUpdatableStakeRatio, "exceeded updatable stake ratio")

	ErrInvalidQueryPath   = New(ErrCodeInvalidQueryPath, "invalid query path")
	ErrInvalidQueryParams = New(ErrCodeInvalidQueryParams, "invalid query parameters")

	ErrNotFoundResult = New(ErrCodeNotFoundResult, "not found result")

	// new style errors
	ErrUnknownTrxType        = NewOrdinary("unknown transaction type")
	ErrUnknownTrxPayloadType = NewOrdinary("unknown transaction payload type")
	ErrNoRight               = NewOrdinary("no right")
	ErrNotVotingPeriod       = NewOrdinary("not voting period")
	ErrDuplicatedKey         = NewOrdinary("already existed key")
)

type XError interface {
	Code() uint32
	Cause() error
	Error() string
	Msg() string
	Wrap(error) XError
	Wrapf(string, ...any) XError
	Contains(XError) bool
	Equal(XError) bool
}

type xerror struct {
	code  uint32
	msg   string
	cause error
}

func New(code uint32, msg string) XError {
	return &xerror{
		code: code,
		msg:  msg,
	}
}

func NewOrdinary(msg string) XError {
	return &xerror{
		code: ErrCodeOrdinary,
		msg:  msg,
	}
}

func From(err error) XError {
	if err == nil {
		return nil
	}
	return NewOrdinary(err.Error())
}

func Wrap(err error, msg string) XError {
	return &xerror{
		code:  ErrCodeOrdinary,
		msg:   msg,
		cause: err,
	}
}

func (xerr *xerror) Code() uint32 {
	type hascode interface {
		Cause() error
	}

	return xerr.code
}

func (xerr *xerror) Error() string {
	msg := xerr.msg

	if xerr.cause != nil {
		msg += "\n\t" + xerr.cause.Error()
	}

	return msg

}

func (xerr *xerror) Msg() string {
	return xerr.msg
}

func (xerr *xerror) Cause() error {
	return xerr.cause
}

func (xerr *xerror) Wrap(err error) XError {
	if xerr.cause != nil {
		if cerr, ok := xerr.cause.(*xerror); ok {
			return &xerror{
				code:  xerr.code,
				msg:   xerr.msg,
				cause: cerr.Wrap(err),
			}
		}
	}
	return &xerror{
		code:  xerr.code,
		msg:   xerr.msg,
		cause: err,
	}
}

func (xerr *xerror) Wrapf(format string, args ...any) XError {
	return xerr.Wrap(New(ErrCodeOrdinary, fmt.Sprintf(format, args...)))
}

func (xerr *xerror) Contains(other XError) bool {
	if xerr.code == other.Code() && xerr.msg == other.Msg() {
		return true
	} else if xerr.cause != nil {
		if _xerr, ok := xerr.cause.(*xerror); ok {
			return _xerr.Contains(other)
		} else {
			return errors.Is(xerr.cause, other)
		}
	}
	return false
}

func (xerr *xerror) Equal(other XError) bool {
	return xerr.code == other.Code()
}

//func (xerr *xerror) DeepEqual(other XError) bool {
//	return xerr.code == other.Code() && xerr.msg == other.Error() && errors.Is(xerr.cause, other.Cause())
//}
