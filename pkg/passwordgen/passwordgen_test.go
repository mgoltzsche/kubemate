package passwordgen

import (
	"fmt"
	"testing"

	//"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateWord(t *testing.T) {
	words := map[string]struct{}{}
	for i := 0; i < 100; i++ {
		wordLen := 12
		word, err := generateWord(wordLen)
		require.NoError(t, err)
		fmt.Println(word)
		require.Equal(t, wordLen, len(word), "word length")
		words[word] = struct{}{}
	}
	require.Equal(t, 100, len(words), "should not generate duplicate word")
}

func TestGenerateMemorablePassword(t *testing.T) {
	passwords := map[string]struct{}{}
	duplicates := []string{}
	iterations := -1
	for i := 0; i < 10000; i++ {
		for j := 0; j < 5; j++ {
			pwlen := 12 + j
			password, err := generateMemorablePassword(pwlen)
			require.NoError(t, err)
			fmt.Println(password)
			require.Equal(t, pwlen, len(password), "password length")
			//require.Truef(t, len(password) >= pwlen && len(password) <= pwlen+3, "password length actual (%d) <> expected (%d)", len(password), pwlen)
			_, exists := passwords[password]
			if exists {
				duplicates = append(duplicates, password)
				if iterations == -1 {
					iterations = i * (j + 1)
				}
			}
			passwords[password] = struct{}{}
		}
	}
	require.Equalf(t, []string{}, duplicates, "duplicate password (%d/%d) after %d iterations", len(duplicates), len(passwords), iterations)
}
