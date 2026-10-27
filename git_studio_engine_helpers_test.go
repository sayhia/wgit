package main

import "testing"

// TestDemangleSwift pins the behavior of demangleSwift, the Swift branch of
// DemangleSymbol used by minidump stack parsing (parseNativeStackFrames ->
// DemangleSymbol -> demangleSwift -> resolveAndBlameFrame).
//
// Swift mangled identifiers are length-prefixed: the decimal prefix counts
// runes (not bytes) and is followed by exactly that many identifier runes.
// The parser must consume each identifier wholesale so that digits embedded
// in a name ("iOS18SDK") do not start a new prefix, and must treat a prefix
// that is zero, overflows, or overruns the input as noise to skip.
func TestDemangleSwift(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			// Foundation.Date struct; previously collapsed into one blob because
			// the regex capture class allowed digits.
			name:  "realistic_foundation_date_struct",
			input: "$s10Foundation4DateV",
			want:  "Foundation.Date",
		},
		{
			// Module 9 "MyAppName", class 7 "MyClass", method 5 "runyy" (y/F thunks dropped).
			name:  "realistic_app_class_method",
			input: "$s9MyAppName7MyClassC5runyyF",
			want:  "MyAppName.MyClass.runyy",
		},
		{
			// Encountered as: _$s9MyAppName9ViewModelC6handle4withySS_tF.
			name:  "realistic_underscore_prefixed_viewmodel",
			input: "_$s9MyAppName9ViewModelC6handle4withySS_tF",
			want:  "MyAppName.ViewModel.handle.with",
		},
		{
			// Library symbol for a struct nominal type (So = ObjC type ref).
			name:  "realistic_stdlib_objc_struct",
			input: "$sSo7CGPointV",
			want:  "CGPoint",
		},
		{
			// Digits inside an identifier belong to it: the 8-rune prefix
			// consumes "iOS18SDK" as one part.
			name:  "embedded_digits_preserved_by_length_consumption",
			input: "$s8iOS18SDK5Thing",
			want:  "iOS18SDK.Thing",
		},
		{
			name:  "empty_string_returns_input",
			input: "",
			want:  "",
		},
		{
			name:  "plain_text_without_numeric_prefix",
			input: "main.swift",
			want:  "main.swift",
		},
		{
			name:  "mangling_prefix_without_lengths_returns_input",
			input: "$sFoo",
			want:  "$sFoo",
		},
		{
			// The trailing digit run overruns the input, so it is skipped.
			name:  "trailing_digits_without_following_ident",
			input: "utf8",
			want:  "utf8",
		},
		{
			// "C" is a class marker and is dropped.
			name:  "filters_class_marker_C",
			input: "$s3foo1C3bar",
			want:  "foo.bar",
		},
		{
			// "V" is a struct marker and is dropped.
			name:  "filters_struct_marker_V",
			input: "$s3foo1V3bar",
			want:  "foo.bar",
		},
		{
			// "SiyF" is a synthesized thunk marker and is dropped.
			name:  "filters_synthesized_thunk_SiyF",
			input: "$s3foo4SiyF3bar",
			want:  "foo.bar",
		},
		{
			// Single-character parts are dropped.
			name:  "filters_single_char_part",
			input: "$s3foo1a3bar",
			want:  "foo.bar",
		},
		{
			// When every part is filtered the original input is returned.
			name:  "all_parts_filtered_returns_input",
			input: "$s1a1V1C4SiyF",
			want:  "$s1a1V1C4SiyF",
		},
		{
			name:  "underscores_and_case_preserved",
			input: "$s7Foo_Bar",
			want:  "Foo_Bar",
		},
		{
			name:  "multiple_segments_joined_with_dot",
			input: "$s3foo3bar6barBaz",
			want:  "foo.bar.barBaz",
		},
		{
			// Plain text containing digits is not a mangled symbol.
			name:  "plain_text_with_digits_returns_input",
			input: "abc123def",
			want:  "abc123def",
		},
		{
			// Length prefix longer than the remaining runes: no panic, input kept.
			name:  "truncated_identifier_returns_input",
			input: "$s10Foo",
			want:  "$s10Foo",
		},
		{
			name:  "zero_length_prefix_returns_input",
			input: "$s0",
			want:  "$s0",
		},
		{
			// Zero prefixes are skipped, scanning resumes after the digit run.
			name:  "zero_length_prefix_skipped_then_ident_kept",
			input: "$s0Foo3bar",
			want:  "bar",
		},
		{
			// strconv.Atoi overflow: skipped instead of panicking.
			name:  "overflowing_length_prefix_returns_input",
			input: "$s999999999999999999999FooV",
			want:  "$s999999999999999999999FooV",
		},
		{
			// Length prefix counts runes, not bytes: a byte slice would split
			// the multibyte module name.
			name:  "unicode_module_consumed_by_rune_count",
			input: "$s3网络库6ClientC",
			want:  "网络库.Client",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := demangleSwift(tt.input); got != tt.want {
				t.Fatalf("demangleSwift(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestIsJSONCollectionEmpty pins the current semantics used by
// parseBundleManifestJSON to distinguish "no recognizable entries" from an
// intentionally empty collection.
func TestIsJSONCollectionEmpty(t *testing.T) {
	tests := []struct {
		name string
		raw  interface{}
		want bool
	}{
		{name: "nil_interface", raw: nil, want: false},
		{name: "empty_map", raw: map[string]interface{}{}, want: true},
		{name: "non_empty_map", raw: map[string]interface{}{"a": 1}, want: false},
		{name: "empty_slice", raw: []interface{}{}, want: true},
		{name: "non_empty_slice", raw: []interface{}{1}, want: false},
		{name: "string", raw: "text", want: false},
		{name: "int", raw: 42, want: false},
		{name: "bool", raw: true, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isJSONCollectionEmpty(tt.raw); got != tt.want {
				t.Fatalf("isJSONCollectionEmpty(%#v) = %v, want %v", tt.raw, got, tt.want)
			}
		})
	}
}
