package jsonx

import (
	"github.com/json-iterator/go"
	"reflect"
)

var _jsonx = jsoniter.Config{
	EscapeHTML:                    true,
	MarshalFloatWith6Digits:       true,
	ObjectFieldMustBeSimpleString: true,
	OnlyTaggedField:               false,
	ValidateJsonRawMessage:        true,
}.Froze()

var (
	Marshal       = _jsonx.Marshal       //jsoniter.ConfigCompatibleWithStandardLibrary.Marshal
	Unmarshal     = _jsonx.Unmarshal     //jsoniter.ConfigCompatibleWithStandardLibrary.Unmarshal
	MarshalIndent = _jsonx.MarshalIndent //jsoniter.ConfigCompatibleWithStandardLibrary.MarshalIndent
	NewEncoder    = _jsonx.NewEncoder    //jsoniter.ConfigCompatibleWithStandardLibrary.NewEncoder
	NewDecoder    = _jsonx.NewDecoder    //jsoniter.ConfigCompatibleWithStandardLibrary.NewDecoder
)

func init() {
	// ▶️ 1) int64 / uint64 → string
	jsoniter.RegisterExtension(&xint64Extension{})
	// ▶️ 2) snake_case → camelCase
	jsoniter.RegisterExtension(&camelCaseExtension{}) // :contentReference[oaicite:1]{index=1}
}

// 필드 정보를 전달하기 위한 구조체
type FieldInfo struct {
	Field        reflect.StructField
	HasStringTag bool
}
