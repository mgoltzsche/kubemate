package storage

import (
	"fmt"
	"testing"

	"github.com/mgoltzsche/kubemate/pkg/resource/fake"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func verifyStore(t *testing.T, testee func() func() Interface) {
	key := "fakekey"
	t.Run("create", func(t *testing.T) {
		s := testee()
		r := createFakeResource(t, s(), key)
		a := &fake.FakeResource{}
		err := s().Get(key, a)
		require.NoError(t, err, "Get()")
		r.SetCreationTimestamp(a.GetCreationTimestamp()) // TODO: fix
		require.Equal(t, r, a)
	})
	t.Run("get non-existing should return not found error", func(t *testing.T) {
		s := testee()
		createFakeResource(t, s(), key)
		a := &fake.FakeResource{}
		err := s().Get(key+"-unknown", a)
		require.Error(t, err, "Get()")
		require.True(t, errors.IsNotFound(err), "IsNotFound(err)")
	})
	t.Run("update", func(t *testing.T) {
		s := testee()
		newValue := "changed value"
		store := s()
		r := createFakeResource(t, store, key)
		a := &fake.FakeResource{}
		err := store.Update(key, a, func() error {
			a.Spec.ValueA = newValue
			return nil
		})
		r.Spec.ValueA = newValue
		r.ResourceVersion = "2"
		a = &fake.FakeResource{}
		err = store.Get(key, a)
		require.NoError(t, err, "Get()")
		r.SetCreationTimestamp(a.GetCreationTimestamp()) // TODO: fix
		require.Equal(t, r, a)

		// with reopened store after update, to ensure persistent and in-memory state is consistent.
		a = &fake.FakeResource{}
		err = store.Update(key, a, func() error {
			a.Spec.ValueA = newValue + "-x"
			return nil
		})
		r.Spec.ValueA = newValue + "-x"
		a = &fake.FakeResource{}
		err = s().Get(key, a)
		require.NoError(t, err, "Get()")
		r.ResourceVersion = a.ResourceVersion            // since file store starts to count from the beginning as opposed to in-memory store
		r.SetCreationTimestamp(a.GetCreationTimestamp()) // TODO: fix
		require.Equal(t, r, a)
	})
	t.Run("delete", func(t *testing.T) {
		s := testee()
		store := s()
		r := createFakeResource(t, store, key)
		err := store.Delete(key, r, func() error { return nil })
		require.NoError(t, err, "Delete()")
		a := &fake.FakeResource{}
		err = store.Get(key, a)
		require.Error(t, err, "Get()")
		require.Contains(t, err.Error(), "not found", "error message")

		// with reopened store after deletion
		store = s()
		r = createFakeResource(t, store, key)
		err = store.Delete(key, r, func() error { return nil })
		require.NoError(t, err, "Delete()")
		a = &fake.FakeResource{}
		err = s().Get(key, a)
		require.Error(t, err, "Get()")
		require.Contains(t, err.Error(), "not found", "error message")
		require.True(t, errors.IsNotFound(err), "IsNotFound(err)")
	})
	t.Run("delete should return error", func(t *testing.T) {
		s := testee()
		r := createFakeResource(t, s(), key)
		err := s().Delete(key, r, func() error { return fmt.Errorf("fake error") })
		require.Error(t, err, "Delete()")
		require.Contains(t, err.Error(), "fake error")
	})
	t.Run("list", func(t *testing.T) {
		s := testee()
		r1 := createFakeResource(t, s(), "fakekey1")
		r2 := createFakeResource(t, s(), "fakekey2")
		r1.SetCreationTimestamp(metav1.Time{})
		r2.SetCreationTimestamp(metav1.Time{})
		a := &fake.FakeResourceList{}
		err := s().List(a)
		require.NoError(t, err, "List()")
		r := &fake.FakeResourceList{
			Items: []fake.FakeResource{*r1, *r2},
		}
		r.Kind = "List"
		r.APIVersion = "v1"
		r.ResourceVersion = "2"
		for i := range a.Items {
			r := a.Items[i]
			r.SetCreationTimestamp(metav1.Time{}) // TODO: preserve creation date
			a.Items[i] = r
		}
		require.Equal(t, r, a)
	})
}

func createFakeResource(t *testing.T, testee Interface, key string) *fake.FakeResource {
	r := &fake.FakeResource{}
	r.Name = "fake-resource"
	r.Spec.ValueA = "fake value"
	err := testee.Create(key, r)
	require.Equal(t, "FakeResource", r.Kind, "kind after resource created")
	require.Equal(t, "fakegroup/v1alpha1", r.APIVersion, "apiVersion after resource created")
	require.NotEmpty(t, r.ResourceVersion, "resourceVersion after resource created")
	require.NoError(t, err, "Create()")
	return r
}
