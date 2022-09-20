package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
)

const MaxResourceNameLength = 63

var invalidNameChars = regexp.MustCompile("[^a-z0-9-]+")

func TruncateName(name string, maxLen int) string {
	h := sha256.New()
	_, _ = h.Write([]byte(name))
	hx := hex.EncodeToString(h.Sum(nil))
	name = invalidNameChars.ReplaceAllString(strings.ToLower(name), "-")
	name = strings.Trim(name, "-")
	if len(name) > maxLen {
		name = name[:maxLen]
	}
	hashLen := 11
	if len(name) > maxLen-(hashLen+1) {
		name = name[:maxLen-(hashLen+1)]
	}
	return fmt.Sprintf("%s-%s", strings.Trim(name, "-"), hx[:hashLen])
}
