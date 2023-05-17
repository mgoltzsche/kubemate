package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/go-openapi/jsonreference"
	generatedopenapi "github.com/mgoltzsche/kubemate/pkg/generated/openapi"
	"k8s.io/kube-openapi/pkg/common"
	"k8s.io/kube-openapi/pkg/validation/spec"
	"sigs.k8s.io/yaml"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "expects exactly 1 argument: the OpenAPI destination file path")
		os.Exit(1)
	}
	if err := writeOpenAPIFile(os.Args[1]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func writeOpenAPIFile(file string) error {
	if file == "" {
		return fmt.Errorf("provided OpenAPI destination file path is empty")
	}
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
		"github.com/mgoltzsche/kubemate/pkg/apis/apps/v1alpha1.AppConfigSchema",
		"github.com/mgoltzsche/kubemate/pkg/apis/devices/v1alpha1.NetworkInterface",
		"github.com/mgoltzsche/kubemate/pkg/apis/devices/v1alpha1.Device",
		"github.com/mgoltzsche/kubemate/pkg/apis/devices/v1alpha1.DeviceDiscovery",
		"github.com/mgoltzsche/kubemate/pkg/apis/devices/v1alpha1.DeviceToken",
		"github.com/mgoltzsche/kubemate/pkg/apis/devices/v1alpha1.WifiNetwork",
		"github.com/mgoltzsche/kubemate/pkg/apis/devices/v1alpha1.WifiPassword",
		"github.com/mgoltzsche/kubemate/pkg/apis/devices/v1alpha1.Certificate",
		"github.com/mgoltzsche/kubemate/pkg/apis/devices/v1alpha1.UserAccount",
		"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1.CustomResourceDefinition",
		"k8s.io/api/networking/v1.Ingress",
		"k8s.io/api/core/v1.Secret",
	}
	for _, typeName := range typeNames {
		err := addType(defs, typeName, &swagger)
		if err != nil {
			return err
		}
	}
	b, err := yaml.Marshal(&swagger)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(file, b, 0644)
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
