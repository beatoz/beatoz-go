package rpc

import "fmt"

// ethRPCError carries an Ethereum JSON-RPC error code. go-ethereum type-asserts
// rpc.Error / rpc.DataError directly (no errors.As), so return it unwrapped.
type ethRPCError struct {
	code    int
	message string
	data    interface{}
}

func (e *ethRPCError) Error() string          { return e.message }
func (e *ethRPCError) ErrorCode() int         { return e.code }
func (e *ethRPCError) ErrorData() interface{} { return e.data }

const ethErrInvalidParams = -32602

func ethInvalidParams(format string, args ...interface{}) error {
	return &ethRPCError{code: ethErrInvalidParams, message: fmt.Sprintf(format, args...)}
}
