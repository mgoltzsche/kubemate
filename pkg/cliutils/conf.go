package cliutils

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// WriteTempConfigFile substitutes placeholder within the given config template and writes the result into a temporary file
func WriteTempConfigFile(name, confTpl string, args ...string) (string, bool, error) {
	conf := strings.NewReplacer(args...).Replace(confTpl)
	h := sha256.New()
	_, _ = h.Write([]byte(conf))
	confHash := hex.EncodeToString(h.Sum(nil))
	file := filepath.Join(os.TempDir(), fmt.Sprintf("kubemate_%s_%s.conf", name, confHash[:12]))
	if _, err := os.Stat(file); os.IsNotExist(err) {
		err := os.WriteFile(file, []byte(conf), 0600)
		if err != nil {
			return "", false, fmt.Errorf("write %s config: %w", name, err)
		}
		return file, true, nil
	}
	return file, false, nil
}
