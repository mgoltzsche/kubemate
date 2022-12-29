package storage

import (
	"testing"

	"github.com/mgoltzsche/kubemate/pkg/resource/fake"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestInMemoryStore(t *testing.T) {
	verifyStore(t, func() func() Interface {
		scheme := runtime.NewScheme()
		err := fake.AddToScheme(scheme)
		require.NoError(t, err)
		testee := InMemory(scheme)
		return func() Interface {
			return testee
		}
	})
}
