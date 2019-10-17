package k8spkg

import (
	"github.com/mgoltzsche/k8spkg/pkg/resource"
)

type App struct {
	Name      string
	Namespace string
	Resources resource.K8sResourceRefList
}

type AppResourceRef struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Name       string `yaml:"name"`
	Namespace  string `yaml:"namespace"`
}
