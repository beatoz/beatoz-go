package jsonx

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_Underscore(t *testing.T) {
	r := toLowerFirstCamel("_test_underscore")
	require.Equal(t, "testUnderscore", r)
}
