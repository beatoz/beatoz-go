package rpc

import (
	tmrpctypes "github.com/tendermint/tendermint/rpc/jsonrpc/types"
	"regexp"
	"strings"
)

var hex40Reg = regexp.MustCompile(`(?i)[a-f0-9]{40,}`)

func parseHeight(heightPtr *int64) int64 {
	if heightPtr == nil {
		return 0
	}
	if *heightPtr < 0 {
		return 0
	}
	return *heightPtr
}

func parsePath(ctx *tmrpctypes.Context) string {
	if ctx.JSONReq != nil {
		return ctx.JSONReq.Method
	}
	if ctx.HTTPReq != nil {
		return strings.TrimSuffix(strings.TrimPrefix(ctx.HTTPReq.URL.Path, "/"), "/")
	}
	return ""
}

func hexToUpper(s string) string {
	return hex40Reg.ReplaceAllStringFunc(s, strings.ToUpper)
}
