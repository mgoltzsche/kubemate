package apiserver

import (
	"context"

	deviceapi "github.com/mgoltzsche/k3spi/pkg/apis/devices/v1"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
)

const adminUser = "admin"

type deviceAuthorizer struct{}

func (deviceAuthorizer) Authorize(ctx context.Context, a authorizer.Attributes) (authorized authorizer.Decision, reason string, err error) {
	if a.GetUser().GetName() == adminUser {
		return authorizer.DecisionAllow, "", nil
	}
	if a.GetUser().GetName() == "anonymous" {
		if a.GetAPIGroup() == deviceapi.GroupVersion.Group && a.GetResource() == "devices" {
			if a.GetVerb() == "get" || a.GetVerb() == "list" {
				return authorizer.DecisionAllow, "", nil
			}
		}
		return authorizer.DecisionDeny, "anonymous user is allowed to read device infos but nothing else", nil
	}
	if a.GetAPIGroup() != deviceapi.GroupVersion.Group {
		return authorizer.DecisionAllow, "", nil
	}
	if a.GetResource() == "devices" && (a.GetVerb() == "get" || a.GetVerb() == "list") {
		return authorizer.DecisionAllow, "", nil
	}
	return authorizer.DecisionDeny, "unauthorized", nil
}

func (deviceAuthorizer) RulesFor(user user.Info, namespace string) ([]authorizer.ResourceRuleInfo, []authorizer.NonResourceRuleInfo, bool, error) {
	return []authorizer.ResourceRuleInfo{
			&authorizer.DefaultResourceRuleInfo{
				Verbs:     []string{"get", "list"},
				APIGroups: []string{deviceapi.GroupVersion.Group},
				Resources: []string{"devices"},
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
