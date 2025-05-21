package jsonx

import (
	"reflect"
	"strconv"
	"strings"
	"unsafe"

	jsoniter "github.com/json-iterator/go"
)

type integerExtension struct {
	jsoniter.DummyExtension
	targets []reflect.Kind
}

func newIntegerExtension(targets ...reflect.Kind) *integerExtension {
	return &integerExtension{
		targets: targets,
	}
}

func (e *integerExtension) UpdateStructDescriptor(desc *jsoniter.StructDescriptor) {
	for i := range desc.Fields {
		binding := desc.Fields[i]

		fieldKind := binding.Field.Type().Kind()
		for _, target := range e.targets {
			if fieldKind == target {
				jsonTags := binding.Field.Tag().Get("json")
				if jsonTags == "-" {
					continue
				}

				if jsonTags != "" {
					parts := strings.Split(jsonTags, ",")
					found := false
					for _, part := range parts {
						if part == "string" {
							found = true
							break
						}
					}
					if found {
						break
					}
				}

				codec := newIntegerCodec(binding.Field.Type().Kind())
				binding.Encoder = codec
				binding.Decoder = codec
				break
			}
		}
	}
}

type integerCodec struct {
	targetType reflect.Kind
}

func newIntegerCodec(targetType reflect.Kind) *integerCodec {
	return &integerCodec{
		targetType: targetType,
	}
}

func (enc *integerCodec) IsEmpty(ptr unsafe.Pointer) bool {
	switch enc.targetType {
	case reflect.Int:
		return isEmpty[int](ptr)
	case reflect.Int8:
		return isEmpty[int8](ptr)
	case reflect.Int16:
		return isEmpty[int16](ptr)
	case reflect.Int32:
		return isEmpty[int32](ptr)
	case reflect.Int64:
		return isEmpty[int64](ptr)
	case reflect.Uint:
		return isEmpty[uint](ptr)
	case reflect.Uint8:
		return isEmpty[uint8](ptr)
	case reflect.Uint16:
		return isEmpty[uint16](ptr)
	case reflect.Uint32:
		return isEmpty[uint32](ptr)
	case reflect.Uint64:
		return isEmpty[uint64](ptr)
	default:
		return false
	}
}
func (enc *integerCodec) Encode(ptr unsafe.Pointer, stream *jsoniter.Stream) {
	switch enc.targetType {
	case reflect.Int:
		encodeSigned[int](ptr, stream)
	case reflect.Int8:
		encodeSigned[int8](ptr, stream)
	case reflect.Int16:
		encodeSigned[int16](ptr, stream)
	case reflect.Int32:
		encodeSigned[int32](ptr, stream)
	case reflect.Int64:
		encodeSigned[int64](ptr, stream)
	case reflect.Uint:
		encodeUnsigned[uint](ptr, stream)
	case reflect.Uint8:
		encodeUnsigned[uint8](ptr, stream)
	case reflect.Uint16:
		encodeUnsigned[uint16](ptr, stream)
	case reflect.Uint32:
		encodeUnsigned[uint32](ptr, stream)
	case reflect.Uint64:
		encodeUnsigned[uint64](ptr, stream)
	default:
		return
	}
}

func (enc *integerCodec) Decode(ptr unsafe.Pointer, iter *jsoniter.Iterator) {
	switch enc.targetType {
	case reflect.Int:
		decodeSigned[int](ptr, iter)
	case reflect.Int8:
		decodeSigned[int8](ptr, iter)
	case reflect.Int16:
		decodeSigned[int16](ptr, iter)
	case reflect.Int32:
		decodeSigned[int32](ptr, iter)
	case reflect.Int64:
		decodeSigned[int64](ptr, iter)
	case reflect.Uint:
		decodeUnsigned[uint](ptr, iter)
	case reflect.Uint8:
		decodeUnsigned[uint8](ptr, iter)
	case reflect.Uint16:
		decodeUnsigned[uint16](ptr, iter)
	case reflect.Uint32:
		decodeUnsigned[uint32](ptr, iter)
	case reflect.Uint64:
		decodeUnsigned[uint64](ptr, iter)
	default:
		return
	}
}

var _ jsoniter.ValEncoder = (*integerCodec)(nil)
var _ jsoniter.ValDecoder = (*integerCodec)(nil)

func encodeSigned[T ~int | ~int8 | ~int16 | ~int32 | ~int64](
	ptr unsafe.Pointer,
	stream *jsoniter.Stream,
) {
	v := *(*T)(ptr)                                     // ✅ 정확한 타입으로 읽기
	stream.WriteString(strconv.FormatInt(int64(v), 10)) // ✅ 안전한 확장
}

func encodeUnsigned[T ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64](
	ptr unsafe.Pointer,
	stream *jsoniter.Stream,
) {
	v := *(*T)(ptr)
	stream.WriteString(strconv.FormatUint(uint64(v), 10))
}

func decodeSigned[T ~int | ~int8 | ~int16 | ~int32 | ~int64](
	ptr unsafe.Pointer,
	iter *jsoniter.Iterator,
) {
	switch iter.WhatIsNext() {
	case jsoniter.StringValue:
		s := iter.ReadString()
		if i, err := strconv.ParseInt(s, 10, 64); err == nil {
			*(*T)(ptr) = T(i)
		} else {
			*(*T)(ptr) = 0
		}
	case jsoniter.NumberValue:
		*(*T)(ptr) = T(iter.ReadInt64())
	default:
		iter.Skip()
	}
}

func decodeUnsigned[T ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64](
	ptr unsafe.Pointer,
	iter *jsoniter.Iterator,
) {
	switch iter.WhatIsNext() {
	case jsoniter.StringValue:
		s := iter.ReadString()
		if i, err := strconv.ParseUint(s, 10, 64); err == nil {
			*(*T)(ptr) = T(i)
		} else {
			*(*T)(ptr) = 0
		}
	case jsoniter.NumberValue:
		*(*T)(ptr) = T(iter.ReadUint64())
	default:
		iter.Skip()
	}
}

func isEmpty[T ~int | ~int8 | ~int16 | ~int32 | ~int64 |
	~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64](
	ptr unsafe.Pointer,
) bool {
	return *(*T)(ptr) == 0
}
