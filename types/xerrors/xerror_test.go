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
