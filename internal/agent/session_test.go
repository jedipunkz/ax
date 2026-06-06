package agent

import (
	"errors"
	"testing"
)

func TestStripLeadingDoubleDash(t *testing.T) {
	cases := []struct {
		in   []string
		want []string
	}{
		{nil, nil},
		{[]string{}, []string{}},
		{[]string{"--", "x"}, []string{"x"}},
		{[]string{"x", "--"}, []string{"x", "--"}},
		{[]string{"x"}, []string{"x"}},
	}
	for _, tc := range cases {
		got := stripLeadingDoubleDash(tc.in)
		if len(got) != len(tc.want) {
			t.Errorf("stripLeadingDoubleDash(%v) length = %d, want %d", tc.in, len(got), len(tc.want))
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("stripLeadingDoubleDash(%v)[%d] = %q, want %q", tc.in, i, got[i], tc.want[i])
			}
		}
	}
}

func TestClassifyExitNil(t *testing.T) {
	code, signaled := classifyExit(nil)
	if code != 0 || signaled {
		t.Errorf("classifyExit(nil) = (%d,%v), want (0,false)", code, signaled)
	}
}

func TestClassifyExitGenericError(t *testing.T) {
	code, signaled := classifyExit(errors.New("boom"))
	if code != 1 || signaled {
		t.Errorf("classifyExit(generic) = (%d,%v), want (1,false)", code, signaled)
	}
}
