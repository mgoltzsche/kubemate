package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

const MaxResourceNameLength = 63

var (
	invalidNameChars = regexp.MustCompile("[^a-z0-9]+")
)

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

func GenerateObjectName(o interface{}, prefix string) (string, error) {
	prefix = invalidNameChars.ReplaceAllString(strings.ToLower(prefix), "-")
	prefix = strings.TrimLeft(prefix, "-")
	b, err := json.Marshal(o)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	_, _ = h.Write(b)
	hx := hex.EncodeToString(h.Sum(nil))[:7]
	if maxLen := 63 - len(hx); len(prefix) > maxLen {
		prefix = prefix[:maxLen]
	}
	return fmt.Sprintf("%s%s", prefix, hx), nil
}
