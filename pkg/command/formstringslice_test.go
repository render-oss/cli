package command

import (
	"reflect"
	"testing"
)

type formStringSliceProbe struct {
	Values []string `cli:"values"`
}

func TestStringSliceFormRoundTripPreservesLiteralElements(t *testing.T) {
	cases := []struct {
		name string
		in   []string
	}{
		{name: "literal comma", in: []string{"LEFT,RIGHT"}},
		{name: "comma plus another element", in: []string{"LEFT,RIGHT", "other"}},
		{name: "embedded quotes", in: []string{`say "hello"`, "other"}},
		{name: "unicode comma", in: []string{"左，右", "other"}},
		{name: "single empty", in: []string{""}},
		{name: "crlf", in: []string{"line1\r\nline2", "other"}},
		{name: "plain multi term", in: []string{"left", "right"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			encoded := stringArrayToString(tc.in)
			got, err := arrayFromString(encoded)
			if err != nil {
				t.Fatalf("decode %q: %v", encoded, err)
			}
			if !reflect.DeepEqual(got, tc.in) {
				t.Fatalf("decoded %q = %#v, want %#v", encoded, got, tc.in)
			}

			probe := &formStringSliceProbe{Values: tc.in}
			values := FormValuesFromStruct(probe)
			var decoded formStringSliceProbe
			if err := StructFromFormValues(values, &decoded); err != nil {
				t.Fatalf("StructFromFormValues: %v", err)
			}
			if !reflect.DeepEqual(decoded.Values, tc.in) {
				t.Fatalf("struct round trip = %#v, want %#v", decoded.Values, tc.in)
			}
		})
	}
}

func TestArrayFromStringKeepsEditableCommaSeparatedInput(t *testing.T) {
	got, err := arrayFromString("left,right")
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	want := []string{"left", "right"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("arrayFromString = %#v, want %#v", got, want)
	}
}

func TestArrayFromStringRejectsMalformedQuotedInput(t *testing.T) {
	if _, err := arrayFromString(`"unterminated`); err == nil {
		t.Fatal("expected unterminated quote error")
	}
	if _, err := arrayFromString(`"left"right`); err == nil {
		t.Fatal("expected data-after-quote error")
	}
}
