package main

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestBlock_Lock(t *testing.T) {
	b := MakeBlock()

	tests := []struct {
		name   string
		action func() bool
		want   bool
	}{
		{
			name: "check that lock free at start",
			action: func() bool {
				return b.IsFree()
			},
			want: true,
		},
		{
			name: "lock",
			action: func() bool {
				return b.Lock()
			},
			want: true,
		},
		{
			name: "try lock second time",
			action: func() bool {
				return b.Lock()
			},
			want: false,
		},
		{
			name: "check is lock free or not",
			action: func() bool {
				return b.IsFree()
			},
			want: false,
		},
		{
			name: "unlock",
			action: func() bool {
				return b.Unlock()
			},
			want: true,
		},
		{
			name: "check is lock free or not after unlock",
			action: func() bool {
				return b.IsFree()
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				res := tt.action()

				require.Equal(t, tt.want, res)
			},
		)
	}
}
