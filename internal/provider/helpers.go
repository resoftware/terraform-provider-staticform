package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// stringList converts a Terraform list of strings into a Go slice. A null or
// unknown list yields a nil slice.
func stringList(ctx context.Context, l types.List) ([]string, diag.Diagnostics) {
	if l.IsNull() || l.IsUnknown() {
		return nil, nil
	}
	var out []string
	d := l.ElementsAs(ctx, &out, false)
	return out, d
}

// toStringList converts a Go slice into a Terraform list of strings.
func toStringList(in []string) types.List {
	if in == nil {
		return types.ListNull(types.StringType)
	}
	elems := make([]types.String, len(in))
	for i, s := range in {
		elems[i] = types.StringValue(s)
	}
	l, _ := types.ListValueFrom(context.Background(), types.StringType, elems)
	return l
}

// stringSet converts a Terraform set of strings into a Go slice. A null or
// unknown set yields a nil slice.
func stringSet(ctx context.Context, s types.Set) ([]string, diag.Diagnostics) {
	if s.IsNull() || s.IsUnknown() {
		return nil, nil
	}
	var out []string
	d := s.ElementsAs(ctx, &out, false)
	return out, d
}

// toStringSet converts a Go slice into a Terraform set of strings.
func toStringSet(in []string) types.Set {
	if in == nil {
		return types.SetNull(types.StringType)
	}
	s, _ := types.SetValueFrom(context.Background(), types.StringType, in)
	return s
}

// stringMap converts a Terraform map of strings into a Go map. A null or unknown
// map yields a nil map.
func stringMap(ctx context.Context, m types.Map) (map[string]string, diag.Diagnostics) {
	if m.IsNull() || m.IsUnknown() {
		return nil, nil
	}
	out := make(map[string]string)
	d := m.ElementsAs(ctx, &out, false)
	return out, d
}

// toStringMap converts a Go map into a Terraform map of strings. An empty map
// yields a null map so it round-trips cleanly against omitempty wire fields.
func toStringMap(in map[string]string) types.Map {
	if len(in) == 0 {
		return types.MapNull(types.StringType)
	}
	m, _ := types.MapValueFrom(context.Background(), types.StringType, in)
	return m
}

// strPtr returns a pointer to s, or nil if s is the empty string.
func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// optStr returns the value of a Terraform string, treating null/unknown as "".
func optStr(s types.String) string {
	if s.IsNull() || s.IsUnknown() {
		return ""
	}
	return s.ValueString()
}

// strFromPtr converts a *string to a Terraform string (null when nil).
func strFromPtr(p *string) types.String {
	if p == nil {
		return types.StringNull()
	}
	return types.StringValue(*p)
}

// int64Ptr returns a pointer to the value, or nil for null/unknown.
func int64Ptr(v types.Int64) *int64 {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	i := v.ValueInt64()
	return &i
}

// int64FromPtr converts a *int64 to a Terraform int64 (null when nil).
func int64FromPtr(p *int64) types.Int64 {
	if p == nil {
		return types.Int64Null()
	}
	return types.Int64Value(*p)
}

// boolPtr returns a pointer to the value, or nil for null/unknown.
func boolPtr(v types.Bool) *bool {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	b := v.ValueBool()
	return &b
}

// boolFromPtr converts a *bool to a Terraform bool (null when nil).
func boolFromPtr(p *bool) types.Bool {
	if p == nil {
		return types.BoolNull()
	}
	return types.BoolValue(*p)
}
