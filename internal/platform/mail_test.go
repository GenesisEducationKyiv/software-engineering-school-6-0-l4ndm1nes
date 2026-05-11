package platform

import (
	"errors"
	"testing"
)

func TestIsTransientMailError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "permanent", err: errors.New("550 mailbox not found"), want: false},
		{name: "auth failure", err: errors.New("535 authentication failed"), want: false},
		{name: "dial", err: errors.New("dial tcp: i/o timeout"), want: true},
		{name: "timeout", err: errors.New("write timeout exceeded"), want: true},
		{name: "EOF", err: errors.New("unexpected EOF"), want: true},
		{name: "connection refused", err: errors.New("connection refused"), want: true},
		{name: "broken pipe", err: errors.New("write: broken pipe"), want: true},
		{name: "temporary", err: errors.New("smtp temporary failure"), want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsTransientMailError(tc.err); got != tc.want {
				t.Errorf("IsTransientMailError(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}
