package jsonx

import (
	"github.com/json-iterator/go"
	"reflect"
)

var _jsonx = jsoniter.Config{
	IndentionStep:          2,
	EscapeHTML:             true,
	SortMapKeys:            true,
	ValidateJsonRawMessage: true,
}.Froze()

//var _jsonx = jsoniter.ConfigCompatibleWithStandardLibrary

var (
	Marshal       = _jsonx.Marshal       //jsoniter.ConfigCompatibleWithStandardLibrary.Marshal
	Unmarshal     = _jsonx.Unmarshal     //jsoniter.ConfigCompatibleWithStandardLibrary.Unmarshal
	MarshalIndent = _jsonx.MarshalIndent //jsoniter.ConfigCompatibleWithStandardLibrary.MarshalIndent
	NewEncoder    = _jsonx.NewEncoder    //jsoniter.ConfigCompatibleWithStandardLibrary.NewEncoder
	NewDecoder    = _jsonx.NewDecoder    //jsoniter.ConfigCompatibleWithStandardLibrary.NewDecoder
)

func init() {
	// ▶️ 1) int64 / uint64 → string
	jsoniter.RegisterExtension(newIntegerExtension(reflect.Int64, reflect.Uint64))
	// ▶️ 2) snake_case → camelCase
	jsoniter.RegisterExtension(&camelCaseExtension{})
}
