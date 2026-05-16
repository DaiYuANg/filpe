package bytesrange_test

import (
	"errors"
	"testing"

	"github.com/lyonbrown4d/maxio/internal/bytesrange"
)

func TestParseValidByteRange(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		header  string
		size    int64
		start   int64
		end     int64
		partial bool
	}{
		{name: "full", size: 10, start: 0, end: 9},
		{name: "bounded", header: "bytes=2-5", size: 10, start: 2, end: 5, partial: true},
		{name: "open ended", header: "bytes=7-", size: 10, start: 7, end: 9, partial: true},
		{name: "suffix", header: "bytes=-4", size: 10, start: 6, end: 9, partial: true},
		{name: "clamped", header: "bytes=7-20", size: 10, start: 7, end: 9, partial: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			spec, err := bytesrange.Parse(tt.header, tt.size)
			if err != nil {
				t.Fatalf("parse range: %v", err)
			}
			assertSpec(t, spec, tt.start, tt.end, tt.partial)
		})
	}
}

func TestParseInvalidByteRange(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		header string
		size   int64
	}{
		{name: "multi range rejected", header: "bytes=0-1,3-4", size: 10},
		{name: "unsatisfied", header: "bytes=10-11", size: 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := bytesrange.Parse(tt.header, tt.size)
			if !errors.Is(err, bytesrange.ErrInvalidRange) {
				t.Fatalf("error = %v, want ErrInvalidRange", err)
			}
		})
	}
}

func assertSpec(t *testing.T, spec bytesrange.Spec, start, end int64, partial bool) {
	t.Helper()
	if spec.Start != start || spec.End != end || spec.Partial != partial {
		t.Fatalf("spec = %+v, want start=%d end=%d partial=%v", spec, start, end, partial)
	}
}
