// +build !kubectl_kustomize

package kustomize

import (
	"sigs.k8s.io/kustomize/v3/k8sdeps/kunstruct"
	transformimpl "sigs.k8s.io/kustomize/v3/k8sdeps/transformer"
	"sigs.k8s.io/kustomize/v3/k8sdeps/validator"
	"sigs.k8s.io/kustomize/v3/pkg/fs"
	"sigs.k8s.io/kustomize/v3/pkg/loader"
	"sigs.k8s.io/kustomize/v3/pkg/plugins"
	"sigs.k8s.io/kustomize/v3/pkg/resmap"
	"sigs.k8s.io/kustomize/v3/pkg/resource"
	"sigs.k8s.io/kustomize/v3/pkg/target"
	"sigs.k8s.io/kustomize/v3/pkg/types"
)

func Render(o RenderOptions) (err error) {
	fSys := fs.MakeRealFS()
	uf := kunstruct.NewKunstructuredFactoryImpl()
	ptf := transformimpl.NewFactoryImpl()
	rf := resmap.NewFactory(resource.NewFactory(uf), ptf)
	v := validator.NewKustValidator()
	var pluginCfg *types.PluginConfig
	if o.EnableAlphaPlugins {
		pluginCfg = plugins.ActivePluginConfig()
	} else {
		pluginCfg = plugins.DefaultPluginConfig()
	}
	pl := plugins.NewLoader(pluginCfg, rf)

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
