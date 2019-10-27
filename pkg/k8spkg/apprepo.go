package k8spkg

import (
	"context"
	"strings"

	"github.com/mgoltzsche/k8spkg/pkg/client"
	"github.com/mgoltzsche/k8spkg/pkg/resource"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var (
	CrdAPIGroup   = "k8spkg.mgoltzsche.github.com"
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

type AppEvent struct {
	App *App
	Err error
}

type AppRepo struct {
	client client.K8sClient
}

func NewAppRepo(client client.K8sClient) *AppRepo {
	return &AppRepo{client}
}

func (m *AppRepo) GetAll(ctx context.Context, namespace string) <-chan AppEvent {
	evts := make(chan AppEvent)
	go func() {
		var err error
		var a *App
		kinds := []string{strings.ToLower(CrdKind) + "." + CrdAPIGroup}
		for evt := range m.client.Get(ctx, kinds, namespace, nil) {
			if evt.Error == nil {
				if a, err = appFromResource(evt.Resource); err != nil {
					evts <- AppEvent{Err: err}
				} else {
					evts <- AppEvent{App: a}
				}
			} else {
				evts <- AppEvent{Err: evt.Error}
			}
		}
		close(evts)
	}()
	return evts
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
	return errors.Wrapf(err, "put app resource %s:%s", app.Namespace, app.Name)
}

func (m *AppRepo) Delete(ctx context.Context, app *App) (err error) {
	err = m.client.Delete(ctx, app.Namespace, []resource.K8sResourceRef{resourceFromApp(app)})
	return errors.Wrapf(err, "delete app resource %s:%s", app.Namespace, app.Name)
}

func appFromResource(obj *resource.K8sResource) (a *App, err error) {
	var (
		resOk, refOk, apiVersionOk, kindOk, nsOk, nameOk bool
		refMap                                           map[string]interface{}
		apiVersion, kind, name, namespace                string
	)
	rawRefs, resOk, err := unstructured.NestedSlice(obj.Raw(), "spec", "resources")
	if err != nil {
		return
	}
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
			}
			if !refOk || !apiVersionOk || !kindOk || !nameOk || !nsOk ||
				apiVersion == "" || kind == "" || name == "" {
				if err == nil {
					err = errors.Errorf("invalid resource ref %#v", rawRef)
				}
				continue
			}
			resources = append(resources,
				resource.ResourceRef(apiVersion, kind, namespace, name))
		}
	} else {
		err = errors.Errorf("app spec does not specify resources: %#v", obj.Raw())
	}
	err = errors.WithMessagef(err, "read app resource %s", obj.Name())
	return &App{Name: obj.Name(), Namespace: obj.Namespace(), Resources: resources}, err
}

func resourceFromApp(app *App) (r *resource.K8sResource) {
	ref := resource.ResourceRef(CrdAPIGroup+"/"+CrdAPIVersion, CrdKind, app.Namespace, app.Name)
	res := make([]interface{}, len(app.Resources))
	for i, r := range app.Resources {
		res[i] = map[string]interface{}{
			"apiVersion": r.APIVersion(),
			"kind":       r.Kind(),
			"name":       r.Name(),
			"namespace":  r.Namespace(),
		}
	}
	return resource.Resource(ref, map[string]interface{}{"spec": map[string]interface{}{"resources": res}})
}
