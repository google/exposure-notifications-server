package exports

import (
	"reflect"
	"testing"
)

func TestSplitRegions(t *testing.T) {
	tests := []struct {
		r string
		e []string
	}{
		{"", []string{}},
		{"  test  ", []string{"test"}},
		{"test\n\rfoo", []string{"foo", "test"}},
		{"test\n\rfoo bar\n\r", []string{"foo bar", "test"}},
		{"test\n\rfoo bar\n\r  ", []string{"foo bar", "test"}},
		{"test\nfoo\n", []string{"foo", "test"}},
	}

	for i, test := range tests {
		if res := splitRegions(test.r); !reflect.DeepEqual(res, test.e) {
			t.Errorf("[%d] splitRegions(%v) = %v, expected %v", i, test.r, res, test.e)
		}
	}
}
