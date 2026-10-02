package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestBoolFromResponse(t *testing.T) {
	tru, fls := true, false
	cases := []struct {
		name     string
		resp     *bool
		prior    types.Bool
		fallback types.Bool
		want     types.Bool
	}{
		{"returned value wins over prior", &fls, types.BoolValue(true), types.BoolNull(), types.BoolValue(false)},
		{"returned value resolves unknown", &tru, types.BoolUnknown(), types.BoolNull(), types.BoolValue(true)},
		{"absent keeps known prior", nil, types.BoolValue(false), types.BoolNull(), types.BoolValue(false)},
		{"absent resolves unknown to fallback", nil, types.BoolUnknown(), types.BoolNull(), types.BoolNull()},
		{"absent resolves null to fallback", nil, types.BoolNull(), types.BoolValue(false), types.BoolValue(false)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := boolFromResponse(c.resp, c.prior, c.fallback); !got.Equal(c.want) {
				t.Errorf("got %s, want %s", got, c.want)
			}
		})
	}
}

func TestListFromResponse(t *testing.T) {
	empty := []string{}
	some := []string{"email", "phone"}
	emptyList := toStringList([]string{})
	cases := []struct {
		name  string
		resp  *[]string
		prior types.List
		want  types.List
	}{
		{"absent keeps known prior", nil, toStringList([]string{"email"}), toStringList([]string{"email"})},
		{"absent keeps null prior", nil, types.ListNull(types.StringType), types.ListNull(types.StringType)},
		{"absent resolves unknown to null", nil, types.ListUnknown(types.StringType), types.ListNull(types.StringType)},
		{"empty response with null prior is null", &empty, types.ListNull(types.StringType), types.ListNull(types.StringType)},
		{"empty response with unknown prior is null", &empty, types.ListUnknown(types.StringType), types.ListNull(types.StringType)},
		{"empty response keeps explicit empty prior", &empty, emptyList, emptyList},
		{"empty response replaces non-empty prior", &empty, toStringList([]string{"email"}), types.ListNull(types.StringType)},
		{"returned values win", &some, types.ListNull(types.StringType), toStringList(some)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := listFromResponse(c.resp, c.prior); !got.Equal(c.want) {
				t.Errorf("got %s, want %s", got, c.want)
			}
		})
	}
}

func TestExplicitList(t *testing.T) {
	if got := explicitList(nil); got == nil || len(*got) != 0 {
		t.Errorf("nil input: got %v, want pointer to empty slice", got)
	}
	if got := explicitList([]string{"a"}); got == nil || len(*got) != 1 || (*got)[0] != "a" {
		t.Errorf("non-empty input: got %v", got)
	}
}
