package rest

import (
	"context"
	"fmt"

	"golang.org/x/crypto/bcrypt"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	registryrest "k8s.io/apiserver/pkg/registry/rest"

	deviceapi "github.com/mgoltzsche/kubemate/pkg/apis/devices/v1alpha1"
	"github.com/mgoltzsche/kubemate/pkg/storage"
)

type userAccountREST struct {
	*REST
}

func NewUserAccountREST(dir string, scheme *runtime.Scheme) (*userAccountREST, error) {
	store, err := storage.FileStore(dir, &deviceapi.UserAccount{}, scheme)
	if err != nil {
		return nil, err
	}
	r := &userAccountREST{
		REST: NewREST(&deviceapi.UserAccount{}, store),
	}
	return r, nil
}

func (r *userAccountREST) Delete(ctx context.Context, key string, deleteValidation registryrest.ValidateObjectFunc, options *metav1.DeleteOptions) (runtime.Object, bool, error) {
	if key == deviceapi.AdminUserAccount {
		a := deviceapi.UserAccount{}
		err := fmt.Errorf("refusing to delete admin user account")
		return nil, false, errors.NewForbidden(a.GetGroupVersionResource().GroupResource(), deviceapi.AdminUserAccount, err)
	}
	return r.REST.Delete(ctx, key, deleteValidation, options)
}

func (r *userAccountREST) Create(ctx context.Context, obj runtime.Object, createValidation registryrest.ValidateObjectFunc, options *metav1.CreateOptions) (runtime.Object, error) {
	a, ok := obj.(*deviceapi.UserAccount)
	if !ok {
		return nil, fmt.Errorf("create user account: provided object is not of type UserAccount but %T", obj)
	}
	err := r.bcryptAccountPassword(a)
	if err != nil {
		return nil, err
	}
	return r.REST.Create(ctx, obj, createValidation, options)
}

func (r *userAccountREST) Update(ctx context.Context, key string, objInfo registryrest.UpdatedObjectInfo, createValidation registryrest.ValidateObjectFunc, updateValidation registryrest.ValidateObjectUpdateFunc, forceAllowCreate bool, options *metav1.UpdateOptions) (runtime.Object, bool, error) {
	updateValidation = func(ctx context.Context, updatedObj, oldObj runtime.Object) error {
		err := updateValidation(ctx, updatedObj, oldObj)
		if err != nil {
			return err
		}
		a, ok := updatedObj.(*deviceapi.UserAccount)
		if !ok {
			return fmt.Errorf("update user account: provided object is not of type UserAccount but %T", updatedObj)
		}
		return r.bcryptAccountPassword(a)
	}
	return r.REST.Update(ctx, key, objInfo, createValidation, updateValidation, forceAllowCreate, options)
}

func (r *userAccountREST) bcryptAccountPassword(a *deviceapi.UserAccount) error {
	if a.Data.PasswordString != "" {
		b, err := bcrypt.GenerateFromPassword([]byte(a.Data.PasswordString), 14)
		if err != nil {
			return err
		}
		a.Data.Password = string(b)
		a.Data.PasswordString = ""
	}
	return nil
}
