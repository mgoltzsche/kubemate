package apiserver

import (
	"context"
	"fmt"

	deviceapi "github.com/mgoltzsche/kubemate/pkg/apis/devices/v1alpha1"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
)

const (
	adminGroup = "admin"
)

var readOnlyVerbs = []string{"get", "list", "watch"}

type deviceAuthorizer struct{}

func (deviceAuthorizer) Authorize(ctx context.Context, a authorizer.Attributes) (authorized authorizer.Decision, reason string, err error) {
	if isNotDeviceAPIGroup := a.GetAPIGroup() != deviceapi.GroupVersion.Group; isNotDeviceAPIGroup {
		// Delegate authorization to proxied apiserver.
		return authorizer.DecisionAllow, "", nil
	}

	if isAdmin := contains(a.GetUser().GetGroups(), adminGroup); isAdmin {
		return authorizer.DecisionAllow, "", nil // admin can do anything.
	}
	isAnonymous := a.GetUser().GetName() == user.Anonymous
	isDeviceAPI := a.GetAPIGroup() == deviceapi.GroupVersion.Group && a.GetResource() == "devicediscovery"
	if isAnonymous && isDeviceAPI && contains(readOnlyVerbs, a.GetVerb()) {
		// Let anonymous users read nothing but the available devices.
		return authorizer.DecisionAllow, "", nil
	}
	return authorizer.DecisionDeny, fmt.Sprintf("you must login to use this device. to manage the device, you need to be a member of the %s group", adminGroup), nil
}

func (deviceAuthorizer) RulesFor(user user.Info, namespace string) ([]authorizer.ResourceRuleInfo, []authorizer.NonResourceRuleInfo, bool, error) {
	return []authorizer.ResourceRuleInfo{
			&authorizer.DefaultResourceRuleInfo{
				Verbs:     []string{"get", "list", "watch"},
				APIGroups: []string{deviceapi.GroupVersion.Group},
				Resources: []string{"devicediscovery"},
			},
		}, []authorizer.NonResourceRuleInfo{
			&authorizer.DefaultNonResourceRuleInfo{
				Verbs:           []string{"get", "list"},
				NonResourceURLs: []string{"*"},
			},
		}, false, nil
}

func NewDeviceAuthorizer() *deviceAuthorizer {
	return new(deviceAuthorizer)
}

func contains(l []string, item string) bool {
	for _, s := range l {
		if s == item {
			return true
		}
	}
	return false
}
