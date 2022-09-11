package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTruncateName(t *testing.T) {
	for _, c := range []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty name",
			input:    "",
			expected: "-e3b0c44298f",
		},
		{
			name:     "short names",
			input:    "name",
			expected: "name-82a3537ff0d",
		},
		{
			name:     "max name length",
			input:    "loooooooooooooooooooooooooooooooooooooooong-name",
			expected: "loooooooooooooooooooooooooooooooooooooooong-name-3f7b872fd4a",
		},
		{
			name:     "name too long",
			input:    "looooooooooooooooooooooooooooooooooooooooong-name",
			expected: "looooooooooooooooooooooooooooooooooooooooong-name-79d19e934b9",
		},

		{
			name:     "invalid name",
			input:    "invalid_name",
			expected: "invalid-name-f123fd210ff",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			a := TruncateName(c.input, MaxResourceNameLength)
			assert.Equal(t, c.expected, a)
			require.Truef(t, len(a) <= 63, "len(name) < 63, len(name) == %d", len(a))
		})
	}
}
