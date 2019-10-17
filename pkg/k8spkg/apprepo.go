package k8spkg

import (
	"context"
	"strings"

	"github.com/mgoltzsche/k8spkg/pkg/client"
	"github.com/mgoltzsche/k8spkg/pkg/resource"
	"github.com/pkg/errors"
)

var (
	CrdAPIGroup   = "mgoltzsche.github.com"
	CrdAPIVersion = "v1alpha1"
	CrdKind       = "Application"
)

type appNotFoundError struct {
	error
}

func IsAppNotFound(err error) bool {
	_, ok := err.(appNotFoundError)
	return ok
}

type AppRepo struct {
	client client.K8sClient
}

func NewAppRepo(client client.K8sClient) *AppRepo {
	return &AppRepo{client}
}

func (m *AppRepo) GetAll(ctx context.Context, namespace string) (apps []*App, err error) {
	var e error
	var a *App
	kinds := []string{strings.ToLower(CrdKind) + "." + CrdAPIGroup}
	l, err := m.client.Get(ctx, kinds, namespace, nil)
	apps = make([]*App, 0, len(l))
	for _, res := range l {
		if a, e = appFromResource(res); e != nil && err == nil {
			err = e
		}
		apps = append(apps, a)
	}
	return
}

func (m *AppRepo) Get(ctx context.Context, namespace, name string) (app *App, err error) {
	kind := strings.ToLower(CrdKind) + "." + CrdAPIGroup
	res, err := m.client.GetResource(ctx, kind, namespace, name)
	if err != nil {
		return
	}
	return appFromResource(res)
}

func (m *AppRepo) Put(ctx context.Context, app *App) (err error) {
	// TODO: do optimistic locking and merge resources
	appRes := resourceFromApp(app)
	_, err = m.client.Apply(ctx, app.Namespace, []*resource.K8sResource{appRes}, false, nil)
	return errors.Wrapf(err, "put app crd %s:%s", app.Namespace, app.Name)
}

func (m *AppRepo) Delete(ctx context.Context, app *App) (err error) {
	err = m.client.Delete(ctx, app.Namespace, []resource.K8sResourceRef{resourceFromApp(app)})
	return errors.Wrapf(err, "delete app crd %s:%s", app.Namespace, app.Name)
}

func appFromResource(obj *resource.K8sResource) (a *App, err error) {
	var (
		resOk, refOk, apiVersionOk, kindOk, nsOk, nameOk bool
		refMap                                           map[string]interface{}
		refMap2                                          map[interface{}]interface{}
		apiGroup, apiVersion, kind, name, namespace      string
	)
	spec := obj.Spec()
	rawRefs, resOk := spec["resources"].([]interface{})
	resources := make([]resource.K8sResourceRef, 0, len(rawRefs))
	if resOk && len(rawRefs) > 0 {
		for _, rawRef := range rawRefs {
			if refMap, refOk = rawRef.(map[string]interface{}); refOk {
				apiVersion, apiVersionOk = refMap["apiVersion"].(string)
				kind, kindOk = refMap["kind"].(string)
				name, nameOk = refMap["name"].(string)
				nsRaw := refMap["namespace"]
				if nsOk = nsRaw == nil; nsOk {
					namespace = ""
				} else {
					namespace, nsOk = nsRaw.(string)
				}
			} else if refMap2, refOk = rawRef.(map[interface{}]interface{}); refOk {
				// additional case due to variant behaviour of yaml unmarshaller
				apiVersion, apiVersionOk = refMap2["apiVersion"].(string)
				kind, kindOk = refMap2["kind"].(string)
				name, nameOk = refMap2["name"].(string)
				nsRaw := refMap["namespace"]
				if nsOk = nsRaw == nil; nsOk {
					namespace = ""
				} else {
					namespace, nsOk = nsRaw.(string)
				}
			}
			apiGroup = ""
			gv := strings.SplitN(apiVersion, "/", 2)
			if len(gv) == 2 {
				apiGroup = gv[0]
				apiVersion = gv[1]
			}
			if !refOk || !apiVersionOk || !kindOk || !nameOk || !nsOk ||
				apiVersion == "" || kind == "" || name == "" {
				if err == nil {
					err = errors.Errorf("invalid resource ref %#v", rawRef)
				}
				continue
			}
			resources = append(resources,
				resource.ResourceRef(apiGroup, apiVersion, kind, namespace, name))
		}
	} else {
		err = errors.Errorf("app spec does not specify resources: %#v", spec)
	}
	err = errors.WithMessagef(err, "read app crd resource %s", obj.Name())
	return &App{Name: obj.Name(), Namespace: obj.Namespace(), Resources: resources}, err
}

func resourceFromApp(app *App) (r *resource.K8sResource) {
	ref := resource.ResourceRef(CrdAPIGroup, CrdAPIVersion, CrdKind, app.Namespace, app.Name)
	res := make([]interface{}, len(app.Resources))
	for i, r := range app.Resources {
		apiVersion := r.APIVersion()
		apiGroup := r.APIGroup()
		if apiGroup != "" {
			apiVersion = apiGroup + "/" + apiVersion
		}
		res[i] = map[string]interface{}{
			"apiVersion": apiVersion,
			"kind":       r.Kind(),
			"name":       r.Name(),
			"namespace":  r.Namespace(),
		}
	}
	return resource.Resource(ref, map[string]interface{}{"spec": map[string]interface{}{"resources": res}})
}
