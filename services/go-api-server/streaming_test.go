package main

import "testing"

func TestNormalizeBitrate(t *testing.T) {
	cases := []struct {
		input    string
		fallback string
		want     string
	}{
		{"320", "192k", "320k"},
		{"  320K  ", "192k", "320k"},
		{"128k", "192k", "128k"},
		{"0", "192k", "192k"},
		{"abc", "192k", "192k"},
		{"", "192k", "192k"},
	}
	for _, c := range cases {
		got := normalizeBitrate(c.input, c.fallback)
		if got != c.want {
			t.Errorf("normalizeBitrate(%q, %q) = %q, want %q", c.input, c.fallback, got, c.want)
		}
	}
}

func TestVolumeGainDb(t *testing.T) {
	loud := 0.0
	cases := []struct {
		loudness  *float64
		normalize bool
		want      float64
	}{
		{nil, true, 0},
		{&loud, true, 0},
		{ptrFloat64(-8.0), true, -6.0},
		{ptrFloat64(-50.0), true, maxGainDb},
		{ptrFloat64(10.0), true, minGainDb},
		{ptrFloat64(-8.0), false, 0},
	}
	for _, c := range cases {
		got := volumeGainDb(c.loudness, c.normalize)
		if got != c.want {
			t.Errorf("volumeGainDb(%v, %v) = %v, want %v", c.loudness, c.normalize, got, c.want)
		}
	}
}

func ptrFloat64(f float64) *float64 {
	return &f
}
