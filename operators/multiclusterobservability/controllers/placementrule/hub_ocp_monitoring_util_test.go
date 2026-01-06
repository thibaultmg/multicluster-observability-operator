// Copyright (c) Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project
// Licensed under the Apache License 2.0

package placementrule

import (
	"context"
	"testing"

	"github.com/ghodss/yaml"
	cmomanifests "github.com/openshift/cluster-monitoring-operator/pkg/manifests"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	operatorconfig "github.com/stolostron/multicluster-observability-operator/operators/pkg/config"
)

func TestRevertHubClusterMonitoringConfig(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	hubInfo := &operatorconfig.HubInfo{
		AlertmanagerEndpoint: "https://alertmanager-host.com",
		HubClusterDomain:     "test-hub",
	}
	hubInfoYaml, _ := yaml.Marshal(hubInfo)
	hubInfoSecret = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      operatorconfig.HubInfoSecretName,
			Namespace: promNamespace,
		},
		Data: map[string][]byte{
			operatorconfig.HubInfoSecretKey: hubInfoYaml,
		},
	}

	tests := []struct {
		name            string
		existingConfig  *cmomanifests.ClusterMonitoringConfiguration
		managedFields   []metav1.ManagedFieldsEntry
		expectedConfigs int
	}{
				{
					name: "Remove matching by host AND legacy prefix",
					existingConfig: &cmomanifests.ClusterMonitoringConfiguration{
						PrometheusK8sConfig: &cmomanifests.PrometheusK8sConfig{
							AlertmanagerConfigs: []cmomanifests.AdditionalAlertmanagerConfig{
								{
									StaticConfigs: []string{"alertmanager-host.com"},
									TLSConfig: cmomanifests.TLSConfig{
										CA: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: hubAmRouterCASecretName + "-legacy-hash",
											},
										},
									},
								},
								{
									StaticConfigs: []string{"other-host.com"},
								},
							},
						},
					},
					managedFields: []metav1.ManagedFieldsEntry{{Manager: endpointMonitoringOperatorMgr}},
					expectedConfigs: 1,
				},
				{
					name: "Don't remove if only host matches",
					existingConfig: &cmomanifests.ClusterMonitoringConfiguration{
						PrometheusK8sConfig: &cmomanifests.PrometheusK8sConfig{
							AlertmanagerConfigs: []cmomanifests.AdditionalAlertmanagerConfig{
								{
									StaticConfigs: []string{"alertmanager-host.com"},
									TLSConfig: cmomanifests.TLSConfig{
										CA: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "other-secret",
											},
										},
									},
								},
							},
						},
					},
					managedFields: []metav1.ManagedFieldsEntry{{Manager: endpointMonitoringOperatorMgr}},
					expectedConfigs: 1,
				},
				{
					name: "Don't remove if only prefix matches",
					existingConfig: &cmomanifests.ClusterMonitoringConfiguration{
						PrometheusK8sConfig: &cmomanifests.PrometheusK8sConfig{
							AlertmanagerConfigs: []cmomanifests.AdditionalAlertmanagerConfig{
								{
									StaticConfigs: []string{"other-host.com"},
									TLSConfig: cmomanifests.TLSConfig{
										CA: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: hubAmRouterCASecretName + "-legacy-hash",
											},
										},
									},
								},
							},
						},
					},
					managedFields: []metav1.ManagedFieldsEntry{{Manager: endpointMonitoringOperatorMgr}},
					expectedConfigs: 1,
				},
		{
			name: "Don't touch if not managed",
			existingConfig: &cmomanifests.ClusterMonitoringConfiguration{
				PrometheusK8sConfig: &cmomanifests.PrometheusK8sConfig{
					AlertmanagerConfigs: []cmomanifests.AdditionalAlertmanagerConfig{
						{
							StaticConfigs: []string{"alertmanager-host.com"},
						},
					},
				},
			},
			managedFields:   []metav1.ManagedFieldsEntry{{Manager: "other-manager"}},
			expectedConfigs: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configYaml, _ := yaml.Marshal(tt.existingConfig)
			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:          clusterMonitoringConfigName,
					Namespace:     promNamespace,
					ManagedFields: tt.managedFields,
				},
				Data: map[string]string{
					clusterMonitoringConfigDataKey: string(configYaml),
				},
			}

			client := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(cm).Build()
			err := RevertHubClusterMonitoringConfig(ctx, client)
			assert.NoError(t, err)

			updatedCm := &corev1.ConfigMap{}
			err = client.Get(ctx, types.NamespacedName{Name: clusterMonitoringConfigName, Namespace: promNamespace}, updatedCm)
			if err != nil {
				assert.Equal(t, 0, tt.expectedConfigs, "ConfigMap should have been deleted")
				return
			}
			updatedConfig := &cmomanifests.ClusterMonitoringConfiguration{}
			_ = yaml.Unmarshal([]byte(updatedCm.Data[clusterMonitoringConfigDataKey]), updatedConfig)

			if updatedConfig.PrometheusK8sConfig == nil || updatedConfig.PrometheusK8sConfig.AlertmanagerConfigs == nil {
				assert.Equal(t, 0, tt.expectedConfigs)
			} else {
				assert.Equal(t, tt.expectedConfigs, len(updatedConfig.PrometheusK8sConfig.AlertmanagerConfigs))
			}
		})
	}
}
