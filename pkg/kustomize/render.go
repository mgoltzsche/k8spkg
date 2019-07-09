// +build !kubectl_kustomize

package kustomize

import (
	"sigs.k8s.io/kustomize/k8sdeps/kunstruct"
	transformimpl "sigs.k8s.io/kustomize/k8sdeps/transformer"
	"sigs.k8s.io/kustomize/k8sdeps/validator"
	"sigs.k8s.io/kustomize/pkg/fs"
	"sigs.k8s.io/kustomize/pkg/loader"
	"sigs.k8s.io/kustomize/pkg/plugins"
	"sigs.k8s.io/kustomize/pkg/resmap"
	"sigs.k8s.io/kustomize/pkg/resource"
	"sigs.k8s.io/kustomize/pkg/target"
)

func Render(o RenderOptions) (err error) {
	fSys := fs.MakeRealFS()
	uf := kunstruct.NewKunstructuredFactoryImpl()
	rf := resmap.NewFactory(resource.NewFactory(uf))
	v := validator.NewKustValidator()
	ptf := transformimpl.NewFactoryImpl()
	pl := plugins.NewLoader(plugins.DefaultPluginConfig(), rf)

	loadRestrictor := loader.RestrictionRootOnly
	if o.Unrestricted {
		loadRestrictor = loader.RestrictionNone
	}
	ldr, err := loader.NewLoader(loadRestrictor, v, o.Source, fSys)
	if err != nil {
		return
	}
	defer ldr.Cleanup()
	kt, err := target.NewKustTarget(ldr, rf, ptf, pl)
	if err != nil {
		return
	}
	m, err := kt.MakeCustomizedResMap()
	if err != nil {
		return
	}
	b, err := m.AsYaml()
	if err != nil {
		return
	}
	_, err = o.Out.Write(b)
	return
}
