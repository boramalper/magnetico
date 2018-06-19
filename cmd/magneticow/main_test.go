package main

import (
	"fmt"
	"testing"
)

type schemaStruct struct {
	PString *string `schema:"pstring"`
	PUint64 *uint64 `schema:"puint64"`
	PBool   *bool   `schema:"pbool"`

	String string `schema:"string"`
	Uint64 uint64 `schema:"uint64"`
	Bool   bool   `schema:"bool"`
}

type schemaRStruct struct {
	Uint64 uint64 `schema:"ruint64,required"`  // https://github.com/gorilla/schema/pull/68
}

// TestSchemaUnsuppliedNil tests that unsupplied values yield nil.
func TestSchemaUnsuppliedNil(t *testing.T) {
	ss := new(schemaStruct)
	if err := decoder.Decode(ss, make(map[string][]string)); err != nil {
		t.Error("decoding error", err.Error())
	}

	if ss.PString != nil { t.Error("PString is not nil") }
	if ss.PUint64 != nil { t.Error("PUint64 is not nil") }
	if ss.PBool   != nil { t.Error("PBool is not nil") }
}

// TestSchemaInvalidUint64 tests that an invalid uint64 value yields nil.
func TestSchemaInvalidUint64(t *testing.T) {
	dict := make(map[string][]string)
	dict["puint64"] = []string{"-1"}

	ss := new(schemaStruct)
	err := decoder.Decode(ss, dict)
	if err == nil { t.Error("err is nil") }
}

// TestSchemaInvalidBool tests that an invalid bool value yields nil.
func TestSchemaInvalidBool(t *testing.T) {
	dict := make(map[string][]string)
	dict["pbool"] = []string{"yyy"}

	ss := new(schemaStruct)
	err := decoder.Decode(ss, dict)
	if err == nil { t.Error("err is nil") }
}

// TestSchemaOverflow tests that integer values greater than the maximum value a field can store
// leads to decoding errors, rather than silent overflows.
func TestSchemaOverflow(t *testing.T) {
	dict := make(map[string][]string)
	dict["puint64"] = []string{"18446744073709551616"}  // 18,446,744,073,709,551,615 + 1

	ss := new(schemaStruct)
	err := decoder.Decode(ss, dict)
	if err == nil { t.Error("err is nil") }
}

// TestSchemaEmptyString tests that empty string yields nil.
func TestSchemaEmptyString(t *testing.T) {
	dict := make(map[string][]string)
	dict["pstring"] = []string{""}

	ss := new(schemaStruct)
	if err := decoder.Decode(ss, make(map[string][]string)); err != nil {
		t.Error("decoding error", err.Error())
	}

	if ss.PString != nil { t.Error("PString is not nil") }
}

// TestSchemaDefault tests if unsupplied values defaults to "zero" and doesn't err
func TestSchemaDefault(t *testing.T) {
	ss := new(schemaStruct)
	if err := decoder.Decode(ss, make(map[string][]string)); err != nil {
		t.Error("decoding error", err.Error())
	}

	if ss.String != ""    { t.Error("String is not empty") }
	if ss.Uint64 != 0     { t.Error("Uint64 is not 0") }
	if ss.Bool   != false { t.Error("Bool is not false") }
}

func TestSchemaRequired(t *testing.T) {
	rs := new(schemaRStruct)
	err := decoder.Decode(rs, make(map[string][]string))
	if err == nil { t.Error("err is nil") }
	fmt.Printf(err.Error())
}