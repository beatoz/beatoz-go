package jsonx

import (
	jsoniter "github.com/json-iterator/go"
	"reflect"
	"strconv"
	"strings"
	"unsafe"
)

type int64Extension struct {
	jsoniter.DummyExtension
}

func (e *int64Extension) UpdateStructDescriptor(desc *jsoniter.StructDescriptor) {
	for i := range desc.Fields {
		binding := desc.Fields[i]
		if binding.Field.Type().Kind() == reflect.Int64 {
			jsonTags := binding.Field.Tag().Get("json")
			if jsonTags == "-" {
				continue
			}

			found := false
			if jsonTags != "" {
				parts := strings.Split(jsonTags, ",")
				for _, part := range parts {
					if part == "string" {
						found = true
						break
					}
				}
			}

			if !found {
				binding.Encoder = &int64Encoder{}
				binding.Decoder = &int64Decoder{}
			}
		}
	}
}

type int64Encoder struct{}

func (enc *int64Encoder) Encode(ptr unsafe.Pointer, stream *jsoniter.Stream) {
	encodeInt64(ptr, stream)
}
func (enc *int64Encoder) IsEmpty(ptr unsafe.Pointer) bool {
	return isEmptyInt64(ptr)
}

var _ jsoniter.ValEncoder = (*int64Encoder)(nil)

type int64Decoder struct{}

var _ jsoniter.ValDecoder = (*int64Decoder)(nil)

func (dec *int64Decoder) Decode(ptr unsafe.Pointer, iter *jsoniter.Iterator) {
	decodeInt64(ptr, iter)
}

//
// operation functions

func encodeInt64(ptr unsafe.Pointer, stream *jsoniter.Stream) {
	v := *(*int64)(ptr)

	//currentField := stream.Attachment
	//if currentField != nil {
	//	if sf, ok := currentField.(reflect.StructField); ok {
	//		tagStr := sf.Tag.Get("json")
	//		options := strings.Split(tagStr, ",")
	//
	//		for _, opt := range options[1:] {
	//			if opt == "string" {
	//				stream.WriteInt64(v)
	//				return
	//			}
	//		}
	//	}
	//}

	stream.WriteString(strconv.FormatInt(v, 10))
}
func decodeInt64(ptr unsafe.Pointer, iter *jsoniter.Iterator) {
	switch iter.WhatIsNext() {
	case jsoniter.StringValue:
		s := iter.ReadString()
		if i, err := strconv.ParseInt(s, 10, 64); err == nil {
			*(*int64)(ptr) = i
		} else {
			*(*int64)(ptr) = 0
		}
	case jsoniter.NumberValue:
		*(*int64)(ptr) = iter.ReadInt64()
	default:
		// 다른 타입이면 건너뛰기
		iter.Skip()
	}
}

func isEmptyInt64(ptr unsafe.Pointer) bool {
	return *(*int64)(ptr) == 0
}
