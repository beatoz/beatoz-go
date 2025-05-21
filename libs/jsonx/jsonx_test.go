package jsonx

import (
	"encoding/json"
	"testing"
)

func TestMarshal(t *testing.T) {
	type TestSubStruct struct {
		SubField      string `json:"sub_field"`
		AnotherSubInt int64  `json:"another_sub_int"`
		UntaggedField string
	}

	type TestStruct struct {
		StringField   string        `json:"string_field"`
		IntField      int           `json:"int_field"`
		Int64Field    int64         `json:"int64_field"`
		Int64FieldStr int64         `json:"int64_field_str,string"`
		BoolField     bool          `json:"bool_field"`
		NestedField   TestSubStruct `json:"nested_field"`
		UntaggedInt   int64
	}

	test := TestStruct{
		StringField:   "hello",
		IntField:      42,
		Int64Field:    9223372036854775807, // 최대 int64 값
		Int64FieldStr: 9223372036854775807,
		BoolField:     true,
		NestedField: TestSubStruct{
			SubField:      "nested",
			AnotherSubInt: 9223372036854775806, // 최대 int64 값 - 1
			UntaggedField: "태그 없는 필드",
		},
		UntaggedInt: 123456789,
	}

	// Use jsonx.Marshal
	result, err := Marshal(test)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	// Verify by deserializing the JSON string into a map.
	var actual map[string]interface{}
	if err := json.Unmarshal(result, &actual); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	// Check whether field names are converted to camelCase.
	if _, exists := actual["stringField"]; !exists {
		t.Error("Expected 'stringField' in result")
	}

	if _, exists := actual["int64Field"]; !exists {
		t.Error("Expected 'int64Field' in result")
	}

	if _, exists := actual["int64FieldStr"]; !exists {
		t.Error("Expected 'int64FieldStr' in result")
	}

	// Check whether int64 values are encoded as strings.
	int64Value, ok := actual["int64Field"].(string)
	if !ok {
		t.Error("Expected int64Field to be encoded as string")
	} else if int64Value != "9223372036854775807" {
		t.Errorf("Expected int64Field to be '9223372036854775807', got '%s'", int64Value)
	}

	int64FieldStr, ok := actual["int64FieldStr"].(string)
	if !ok {
		t.Error("Expected int64FieldStr to be encoded as string")
	} else if int64FieldStr != "9223372036854775807" {
		t.Errorf("Expected int64FieldStr to be '9223372036854775807', got '%s'", int64FieldStr)
	}

	// no json tag
	untaggedInt, ok := actual["untaggedInt"].(string)
	if !ok {
		t.Error("Expected untaggedInt to be encoded as string")
	} else if untaggedInt != "123456789" {
		t.Errorf("Expected untaggedInt to be '123456789', got '%s'", untaggedInt)
	}

	// Check whether the field names of nested structs are converted to camelCase.
	nestedField, ok := actual["nestedField"].(map[string]interface{})
	if !ok {
		t.Error("Expected nestedField to be a map")
	} else {
		if _, exists := nestedField["subField"]; !exists {
			t.Error("Expected 'subField' in nested field")
		}

		//Check whether nested int64 fields are also encoded as strings.
		subInt64, ok := nestedField["anotherSubInt"].(string)
		if !ok {
			t.Error("Expected nested anotherSubInt to be encoded as string")
		} else if subInt64 != "9223372036854775806" {
			t.Errorf("Expected nested anotherSubInt to be '9223372036854775806', got '%s'", subInt64)
		}
	}
}

func TestMarshalIndent(t *testing.T) {
	type TestStruct struct {
		StringField string `json:"string_field"`
		IntField    int64  `json:"int_field"`
	}

	test := TestStruct{
		StringField: "hello",
		IntField:    123456789,
	}

	result, err := MarshalIndent(test, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent failed: %v", err)
	}

	expected := `{
  "stringField": "hello",
  "intField": "123456789"
}`

	if string(result) != expected {
		t.Errorf("Expected:\n%s\n\nGot:\n%s", expected, string(result))
	}
}

func TestUnmarshal(t *testing.T) {
	jsonData := []byte(`{
		"stringField": "hello",
		"intField": 42,
		"int64Field": "9223372036854775807",
		"nestedField": {
			"subField": "nested",
			"anotherSubInt": "9223372036854775806"
		}
	}`)

	type TestSubStruct struct {
		SubField      string `json:"sub_field"`
		AnotherSubInt int64  `json:"another_sub_int"`
	}

	type TestStruct struct {
		StringField string        `json:"string_field"`
		IntField    int           `json:"int_field"`
		Int64Field  int64         `json:"int64_field"`
		NestedField TestSubStruct `json:"nested_field"`
	}

	var result TestStruct
	if err := Unmarshal(jsonData, &result); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	// Check whether the value is correctly deserialized.
	if result.StringField != "hello" {
		t.Errorf("Expected StringField to be 'hello', got '%s'", result.StringField)
	}

	if result.IntField != 42 {
		t.Errorf("Expected IntField to be 42, got %d", result.IntField)
	}

	if result.Int64Field != 9223372036854775807 {
		t.Errorf("Expected Int64Field to be 9223372036854775807, got %d", result.Int64Field)
	}

	if result.NestedField.SubField != "nested" {
		t.Errorf("Expected NestedField.SubField to be 'nested', got '%s'", result.NestedField.SubField)
	}

	if result.NestedField.AnotherSubInt != 9223372036854775806 {
		t.Errorf("Expected NestedField.AnotherSubInt to be 9223372036854775806, got %d", result.NestedField.AnotherSubInt)
	}
}

// Test whether numeric int64 fields can also be handled.
func TestUnmarshalNumberedInt64(t *testing.T) {
	jsonData := []byte(`{
		"stringField": "hello",
		"int64Field": 9223372036854775807
	}`)

	type TestStruct struct {
		StringField string `json:"string_field"`
		Int64Field  int64  `json:"int64_field"`
	}

	var result TestStruct
	if err := Unmarshal(jsonData, &result); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if result.Int64Field != 9223372036854775807 {
		t.Errorf("Expected Int64Field to be 9223372036854775807, got %d", result.Int64Field)
	}
}

// Test whether JSON data in snake_case can also be handled.
func TestUnmarshalSnakeCaseInput(t *testing.T) {
	jsonData := []byte(`{
		"string_field": "hello",
		"int64_field": "9223372036854775807"
	}`)

	type TestStruct struct {
		StringField string `json:"string_field"`
		Int64Field  int64  `json:"int64_field"`
	}

	var result TestStruct
	if err := Unmarshal(jsonData, &result); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if result.StringField != "hello" {
		t.Errorf("Expected StringField to be 'hello', got '%s'", result.StringField)
	}

	if result.Int64Field != 9223372036854775807 {
		t.Errorf("Expected Int64Field to be 9223372036854775807, got %d", result.Int64Field)
	}
}
