package jsonx

import (
	jsoniter "github.com/json-iterator/go"
	"strings"
	"unicode"
	"unicode/utf8"
)

type camelCaseExtension struct {
	jsoniter.DummyExtension
}

func (e *camelCaseExtension) UpdateStructDescriptor(desc *jsoniter.StructDescriptor) {
	for i := range desc.Fields {
		binding := desc.Fields[i]

		// name from json tag
		tagName, ok := jsonTagName(binding.Field.Name(), binding.Field.Tag().Get("json"))
		if !ok {
			continue
		}

		// snake_case를 camelCase로 변환
		if camelName := camelName(tagName); camelName != tagName {
			binding.ToNames = []string{camelName}
			binding.FromNames = []string{camelName, tagName} // 둘 다 지원
		}
	}
}

func JSONName(fieldName, jsonTag string) (string, bool) {
	tagName, ok := jsonTagName(fieldName, jsonTag)
	if !ok {
		return "", false
	}
	return camelName(tagName), true
}

func jsonTagName(fieldName, jsonTag string) (string, bool) {
	if jsonTag == "-" {
		return "", false
	}
	if parts := strings.Split(jsonTag, ","); parts[0] != "" {
		return parts[0], true
	}
	return fieldName, true
}

func camelName(tagName string) string {
	if strings.Contains(tagName, "_") || isFirstCharUpper(tagName) {
		return toLowerFirstCamel(tagName)
	}
	return tagName
}

func toLowerFirstCamel(s string) string {
	if strings.Contains(s, "_") {
		// snake_case 처리
		parts := strings.Split(s, "_")
		out := ""
		for _, p := range parts {
			if p == "" {
				continue
			}
			if out == "" {
				out += strings.ToLower(p[:1]) + p[1:]
				continue
			}
			out += strings.ToUpper(p[:1]) + p[1:]
		}
		return out
	}
	if s == "" {
		return s
	}
	// PascalCase → lowerCamelCase
	return strings.ToLower(s[:1]) + s[1:]
}

func toCamelCase(s string) string {
	parts := strings.Split(s, "_")
	if len(parts) == 1 {
		return s
	}

	out := parts[0]
	for _, part := range parts[1:] {
		if len(part) > 0 {
			out += strings.ToUpper(part[:1]) + part[1:]
		}
	}
	return out
}

func isFirstCharUpper(s string) bool {
	if s == "" {
		return false
	}
	r, _ := utf8.DecodeRuneInString(s)
	return unicode.IsUpper(r)
}
