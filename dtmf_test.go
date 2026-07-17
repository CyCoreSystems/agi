package agi

import (
	"errors"
	"testing"
)

func TestDtmfDigit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		resp *Response
		want string
		err  bool
	}{
		{
			name: "stream file digit 6 with endpos",
			resp: &Response{Result: 54, ResultString: "54", Value: "endpos=21760"},
			want: "6",
		},
		{
			name: "stream file digit 8",
			resp: &Response{Result: 56, ResultString: "56", Value: "endpos=34720"},
			want: "8",
		},
		{
			name: "no digit result=0",
			resp: &Response{Result: 0, ResultString: "0", Value: "endpos=80017"},
			want: "",
		},
		{
			name: "hash",
			resp: &Response{Result: 35, ResultString: "35"},
			want: "#",
		},
		{
			name: "error",
			resp: &Response{Error: errors.New("hangup")},
			err:  true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := dtmfDigit(tt.resp)
			if tt.err {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %q want %q", got, tt.want)
			}
		})
	}
}
