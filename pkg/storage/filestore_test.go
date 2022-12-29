package storage

import (
	"os"
	"testing"

	"github.com/mgoltzsche/kubemate/pkg/resource/fake"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestFileStore(t *testing.T) {
	tmpDirs := []string{}
	defer func() {
		for _, dir := range tmpDirs {
			_ = os.RemoveAll(dir)
		}
	}()
	verifyStore(t, func() func() Interface {
		scheme := runtime.NewScheme()
		err := fake.AddToScheme(scheme)
		require.NoError(t, err)
		tmpDir, err := os.MkdirTemp("", "kubemate-storetest-")
		require.NoError(t, err)
		tmpDirs = append(tmpDirs, tmpDir)
		return func() Interface {
			testee, err := FileStore(tmpDir, &fake.FakeResource{}, scheme)
			require.NoError(t, err, "FileStore()")
			return testee
		}
	})
}
