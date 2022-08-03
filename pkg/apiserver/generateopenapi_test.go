package apiserver

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-openapi/jsonreference"
	generatedopenapi "github.com/mgoltzsche/kubemate/pkg/generated/openapi"
	"github.com/stretchr/testify/require"
	"k8s.io/kube-openapi/pkg/common"
	"k8s.io/kube-openapi/pkg/validation/spec"
	"sigs.k8s.io/yaml"
)

func TestGenerateOpenAPI(t *testing.T) {
	defs := generatedopenapi.GetOpenAPIDefinitions(ref)
	swagger := spec.Swagger{
		SwaggerProps: spec.SwaggerProps{
			Swagger: "2.0",
			Info: &spec.Info{
				InfoProps: spec.InfoProps{
					Title:   "Kubemate",
					Version: "unversioned",
				},
			},
			Paths: &spec.Paths{
				Paths: map[string]spec.PathItem{},
			},
			Definitions: map[string]spec.Schema{},
		},
	}
	typeNames := []string{
		"github.com/mgoltzsche/kubemate/pkg/apis/apps/v1alpha1.App",
		"github.com/mgoltzsche/kubemate/pkg/apis/devices/v1.Device",
		"github.com/mgoltzsche/kubemate/pkg/apis/devices/v1.DeviceToken",
		"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1.CustomResourceDefinition",
		"k8s.io/api/networking/v1.Ingress",
	}
	for _, typeName := range typeNames {
		err := addType(defs, typeName, &swagger)
		require.NoError(t, err)
	}
	b, err := yaml.Marshal(&swagger)
	require.NoError(t, err)
	err = ioutil.WriteFile(filepath.Join("..", "..", "openapi.yaml"), b, 0644)
	require.NoError(t, err)
}

func addType(typeDefs map[string]common.OpenAPIDefinition, typeName string, swagger *spec.Swagger) error {
	typeDef, ok := typeDefs[typeName]
	if !ok {
		return fmt.Errorf("add openapi type to schema: type %q not found", typeName)
	}
	swagger.Definitions[oapiTypeName(typeName)] = typeDef.Schema
	for _, dep := range typeDef.Dependencies {
		_, ok = swagger.Definitions[oapiTypeName(dep)]
		if !ok {
			err := addType(typeDefs, dep, swagger)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func ref(path string) spec.Ref {
	return spec.Ref{
		Ref: jsonreference.MustCreateRef(fmt.Sprintf("#/definitions/%s", oapiTypeName(path))),
	}
}

func oapiTypeName(name string) string {
	pathSegments := strings.Split(name, "/")
	hostSegments := strings.Split(pathSegments[0], ".")
	reversedHostSegments := make([]string, len(hostSegments))
	for i := 0; i < len(hostSegments); i++ {
		reversedHostSegments[i] = hostSegments[len(hostSegments)-1-i]
	}
	pathSegments[0] = strings.Join(reversedHostSegments, ".")
	name = strings.Join(pathSegments, "/")
	return strings.ReplaceAll(name, "/", ".")
}
