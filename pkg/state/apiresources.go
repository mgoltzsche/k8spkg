package state

import (
	"bytes"
	"os/exec"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

type ApiResourceTypes struct {
	namespaced []string
	cluster    []string
}

func (r *ApiResourceTypes) Namespaced() (typeNames []string, err error) {
	if r.namespaced == nil {
		r.namespaced, err = loadApiResourceTypeNames(true)
	}
	return r.namespaced, err
}

func (r *ApiResourceTypes) Cluster() (typeNames []string, err error) {
	if r.cluster == nil {
		r.cluster, err = loadApiResourceTypeNames(false)
	}
	return r.cluster, err
}

func (r *ApiResourceTypes) All() (typeNames []string, err error) {
	typeNames, err = r.Cluster()
	if err != nil {
		return
	}
	namespaced, err := r.Namespaced()
	typeNames = append(typeNames, namespaced...)
	return
}

func loadApiResourceTypeNames(namespacedOnly bool) (typeNames []string, err error) {
	var stdout, stderr bytes.Buffer
	c := exec.Command("kubectl", "api-resources", "--verbs", "delete", "--namespaced="+strconv.FormatBool(namespacedOnly), "-o", "name")
	c.Stdout = &stdout
	c.Stderr = &stderr
	err = c.Run()
	if err != nil {
		err = errors.Errorf("%+v: %s, stderr: %s", c.Args, err, strings.TrimSuffix(stderr.String(), "\n"))
	} else {
		typeNames = strings.Split(stdout.String(), "\n")
		typeNames = typeNames[:len(typeNames)-1]
	}
	return
}
