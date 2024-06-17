// Copyright (c) Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project
// Licensed under the Apache License 2.0

package rendering

import (
	v1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/kustomize/api/resource"

	"github.com/stolostron/multicluster-observability-operator/operators/multiclusterobservability/pkg/config"
	rendererutil "github.com/stolostron/multicluster-observability-operator/operators/pkg/rendering"
	"github.com/stolostron/multicluster-observability-operator/operators/pkg/util"
)

func (r *MCORenderer) newGranfanaRenderer() {
	r.renderGrafanaFns = map[string]rendererutil.RenderFn{
		"Deployment":            r.renderGrafanaDeployments,
		"Service":               r.renderer.RenderNamespace,
		"ServiceAccount":        r.renderer.RenderNamespace,
		"ConfigMap":             r.renderer.RenderNamespace,
		"ClusterRole":           r.renderer.RenderClusterRole,
		"ClusterRoleBinding":    r.renderer.RenderClusterRoleBinding,
		"Secret":                r.renderer.RenderNamespace,
		"Role":                  r.renderer.RenderNamespace,
		"RoleBinding":           r.renderer.RenderNamespace,
		"Ingress":               r.renderer.RenderNamespace,
		"PersistentVolumeClaim": r.renderer.RenderNamespace,
	}
}

func (r *MCORenderer) renderGrafanaDeployments(res *resource.Resource,
	namespace string, labels map[string]string) (*unstructured.Unstructured, error) {
	u, err := r.renderer.RenderDeployments(res, namespace, labels)
	if err != nil {
		return nil, err
	}

	obj := util.GetK8sObj(u.GetKind())
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, obj)
	if err != nil {
		return nil, err
	}
	dep := obj.(*v1.Deployment)
	dep.Name = config.GetOperandName(config.Grafana)
	dep.Spec.Replicas = config.GetReplicas(config.Grafana, r.cr.Spec.InstanceSize, r.cr.Spec.AdvancedConfig)

	spec := &dep.Spec.Template.Spec
	imagePullPolicy := config.GetImagePullPolicy(r.cr.Spec)

	spec.Containers[0].Image = config.DefaultImgRepository + "/" + config.GrafanaImgKey +
		":" + config.DefaultImgTagSuffix
	found, image := config.ReplaceImage(r.cr.Annotations, spec.Containers[0].Image, config.GrafanaImgKey)
	if found {
		spec.Containers[0].Image = image
	}
	spec.Containers[0].ImagePullPolicy = imagePullPolicy
	spec.Containers[0].Resources = config.GetResources(config.Grafana, r.cr.Spec.InstanceSize, r.cr.Spec.AdvancedConfig)

	spec.Containers[1].Image = config.DefaultImgRepository + "/" + config.GrafanaDashboardLoaderName +
		":" + config.DefaultImgTagSuffix
	found, image = config.ReplaceImage(r.cr.Annotations, spec.Containers[1].Image,
		config.GrafanaDashboardLoaderKey)
	if found {
		spec.Containers[1].Image = image
	}
	spec.Containers[1].ImagePullPolicy = imagePullPolicy

	found, image = config.ReplaceImage(nil, config.OauthProxyImgRepo,
		config.OauthProxyKey)
	if found {
		spec.Containers[2].Image = image
	}
	spec.Containers[2].ImagePullPolicy = imagePullPolicy

	unstructuredObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}

	return &unstructured.Unstructured{Object: unstructuredObj}, nil
}

func (r *MCORenderer) renderGrafanaTemplates(templates []*resource.Resource,
	namespace string, labels map[string]string) ([]*unstructured.Unstructured, error) {
	uobjs := []*unstructured.Unstructured{}
	for _, template := range templates {
		render, ok := r.renderGrafanaFns[template.GetKind()]
		if !ok {
			m, err := template.Map()
			if err != nil {
				return []*unstructured.Unstructured{}, err
			}
			uobjs = append(uobjs, &unstructured.Unstructured{Object: m})
			continue
		}
		uobj, err := render(template.DeepCopy(), namespace, labels)
		if err != nil {
			return []*unstructured.Unstructured{}, err
		}
		if uobj == nil {
			continue
		}
		uobjs = append(uobjs, uobj)

	}

	return uobjs, nil
}
