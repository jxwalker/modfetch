package main

import (
	"math"
	"testing"
)

func TestValidateGiBOverride(t *testing.T) {
	tests := []struct {
		name    string
		value   float64
		wantErr bool
	}{
		{name: "unset", value: 0},
		{name: "fractional", value: 0.5},
		{name: "max", value: maxHardwareOverrideGiB},
		{name: "negative", value: -1, wantErr: true},
		{name: "too large", value: maxHardwareOverrideGiB + 1, wantErr: true},
		{name: "nan", value: math.NaN(), wantErr: true},
		{name: "inf", value: math.Inf(1), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateGiBOverride("--ram-gb", tt.value)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateGiBOverride(%g) err = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}
