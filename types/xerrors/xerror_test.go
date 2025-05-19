package xerrors

import (
	"errors"
	"fmt"
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_Wrap(t *testing.T) {
	err := errors.New("base error")
	xerr0 := NewOrdinary("first xerror").Wrap(err)
	xerr1 := NewOrdinary("second xerror").Wrap(xerr0)

	//second xerror
	//	first xerror
	//	base error
	fmt.Println(xerr1)

	xerr0 = NewOrdinary("first xerror").Wrapf("initial error: %s", err.Error())
	xerr1 = NewOrdinary("second xerror").Wrap(xerr0)

	//second xerror
	//	first xerror
	//	initial error: base error
	fmt.Println(xerr1)
}

func Test_Contains(t *testing.T) {
	err := errors.New("base error")
	xerr0 := NewOrdinary("first xerror").Wrap(err)
	xerr1 := NewOrdinary("second xerror").Wrap(xerr0)
	xerrNotContained := NewOrdinary("third xerror").Wrap(err)

	require.True(t, xerr1.Contains(xerr0))
	require.False(t, xerr1.Contains(xerrNotContained))
}

func Test_Equal(t *testing.T) {
	xerr := ErrNotFoundResult
	require.Equal(t, ErrNotFoundResult, xerr)
	require.True(t, ErrNotFoundResult == xerr)
	require.False(t, ErrNotFoundResult != xerr)
	require.True(t, errors.Is(xerr, ErrNotFoundResult))
	require.True(t, xerr.Equal(ErrNotFoundResult))

	xerr = ErrNotFoundProposal
	require.Equal(t, ErrNotFoundProposal, xerr)
	require.True(t, ErrNotFoundProposal == xerr)
	require.False(t, ErrNotFoundProposal != xerr)
	require.True(t, errors.Is(xerr, ErrNotFoundProposal))
	require.True(t, xerr.Equal(ErrNotFoundProposal))

	xerr = ErrInvalidGas
	require.Equal(t, ErrInvalidGas, xerr)
	require.True(t, ErrInvalidGas == xerr)
	require.False(t, ErrInvalidGas != xerr)
	require.True(t, errors.Is(xerr, ErrInvalidGas))
	require.True(t, xerr.Equal(ErrInvalidGas))
	require.True(t, xerr.Contains(ErrInvalidGas))
	require.True(t, xerr.Contains(ErrInvalidTrx))
	require.False(t, xerr.Contains(ErrInvalidGasPrice))
}
