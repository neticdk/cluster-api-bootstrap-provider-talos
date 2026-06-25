// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package controllers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes/scheme"
	capiv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	bsutil "sigs.k8s.io/cluster-api/bootstrap/util"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	bootstrapv1beta1 "github.com/siderolabs/cluster-api-bootstrap-provider-talos/api/v1beta1"
)

func TestRenderTemplate(t *testing.T) {
	ctxCtx := TemplateContext{
		Vars: map[string]string{
			"foo": "bar",
			"dns": "1.1.1.1",
		},
	}
	ctxCtx.Machine.Name = "test-machine"

	tests := []struct {
		name    string
		tmpl    string
		want    string
		wantErr bool
	}{
		{
			name: "Render Var",
			tmpl: "hostname: {{ .Vars.foo }}",
			want: "hostname: bar",
		},
		{
			name: "Render Machine Name",
			tmpl: "machine: {{ .Machine.Name }}",
			want: "machine: test-machine",
		},
		{
			name: "Render mixed",
			tmpl: "host: {{ .Machine.Name }}, dns: {{ .Vars.dns }}",
			want: "host: test-machine, dns: 1.1.1.1",
		},
		{
			name:    "Invalid template syntax",
			tmpl:    "host: {{ .Machine.Name",
			wantErr: true,
		},
		{
			name:    "Missing key error",
			tmpl:    "dns: {{ .Vars.not_existent }}",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := renderTemplate(tt.name, tt.tmpl, ctxCtx)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, res)
			}
		})
	}
}

func TestResolveVariables(t *testing.T) {
	s := scheme.Scheme
	_ = bootstrapv1beta1.AddToScheme(s)
	_ = corev1.AddToScheme(s)

	t.Run("Successfully resolve all variables", func(t *testing.T) {
		config := &bootstrapv1beta1.TalosConfig{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      "my-config",
			},
			Spec: bootstrapv1beta1.TalosConfigSpec{
				Variables: []bootstrapv1beta1.Variable{
					{
						Name: "hostname",
						ValueFrom: bootstrapv1beta1.VariableSource{
							SecretKeyRef: &bootstrapv1beta1.SecretKeySelector{
								Name: "{{ .Machine.Name }}-metadata",
								Key:  "hostname",
							},
						},
					},
					{
						Name: "global_dns",
						ValueFrom: bootstrapv1beta1.VariableSource{
							SecretKeyRef: &bootstrapv1beta1.SecretKeySelector{
								Name: "global-config",
								Key:  "dns",
							},
						},
					},
				},
			},
		}

		machineSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      "node-0-metadata",
			},
			Data: map[string][]byte{
				"hostname": []byte("node-zero"),
			},
		}

		globalSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      "global-config",
			},
			Data: map[string][]byte{
				"dns": []byte("8.8.8.8"),
			},
		}

		fakeClient := fake.NewClientBuilder().WithScheme(s).WithObjects(machineSecret, globalSecret).Build()
		r := &TalosConfigReconciler{
			Client: fakeClient,
			Log:    ctrl.Log,
		}

		machineObj := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "cluster.x-k8s.io/v1beta2",
				"kind":       "Machine",
				"metadata": map[string]interface{}{
					"name":      "node-0",
					"namespace": "default",
				},
			},
		}
		owner := &bsutil.ConfigOwner{
			Unstructured: machineObj,
		}

		scope := &TalosConfigScope{
			Config:      config,
			ConfigOwner: owner,
		}

		vars, err := r.resolveVariables(context.Background(), scope)
		require.NoError(t, err)
		assert.Equal(t, "node-zero", vars["hostname"])
		assert.Equal(t, "8.8.8.8", vars["global_dns"])
	})

	t.Run("Return transient requeueError on missing Secret", func(t *testing.T) {
		config := &bootstrapv1beta1.TalosConfig{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      "my-config",
			},
			Spec: bootstrapv1beta1.TalosConfigSpec{
				Variables: []bootstrapv1beta1.Variable{
					{
						Name: "hostname",
						ValueFrom: bootstrapv1beta1.VariableSource{
							SecretKeyRef: &bootstrapv1beta1.SecretKeySelector{
								Name: "missing-metadata",
								Key:  "hostname",
							},
						},
					},
				},
			},
		}

		fakeClient := fake.NewClientBuilder().WithScheme(s).Build()
		r := &TalosConfigReconciler{
			Client: fakeClient,
			Log:    ctrl.Log,
		}

		machineObj := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "cluster.x-k8s.io/v1beta2",
				"kind":       "Machine",
				"metadata": map[string]interface{}{
					"name":      "node-0",
					"namespace": "default",
				},
			},
		}
		owner := &bsutil.ConfigOwner{
			Unstructured: machineObj,
		}

		scope := &TalosConfigScope{
			Config:      config,
			ConfigOwner: owner,
		}

		_, err := r.resolveVariables(context.Background(), scope)
		require.Error(t, err)

		var reqErr *requeueError
		require.ErrorAs(t, err, &reqErr)
		assert.Contains(t, err.Error(), "waiting for metadata secret missing-metadata")
	})

	t.Run("Return error on missing key in Secret", func(t *testing.T) {
		config := &bootstrapv1beta1.TalosConfig{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      "my-config",
			},
			Spec: bootstrapv1beta1.TalosConfigSpec{
				Variables: []bootstrapv1beta1.Variable{
					{
						Name: "hostname",
						ValueFrom: bootstrapv1beta1.VariableSource{
							SecretKeyRef: &bootstrapv1beta1.SecretKeySelector{
								Name: "some-metadata",
								Key:  "missing-key",
							},
						},
					},
				},
			},
		}

		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      "some-metadata",
			},
			Data: map[string][]byte{
				"other-key": []byte("value"),
			},
		}

		fakeClient := fake.NewClientBuilder().WithScheme(s).WithObjects(secret).Build()
		r := &TalosConfigReconciler{
			Client: fakeClient,
			Log:    ctrl.Log,
		}

		machineObj := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "cluster.x-k8s.io/v1beta2",
				"kind":       "Machine",
				"metadata": map[string]interface{}{
					"name":      "node-0",
					"namespace": "default",
				},
			},
		}
		owner := &bsutil.ConfigOwner{
			Unstructured: machineObj,
		}

		scope := &TalosConfigScope{
			Config:      config,
			ConfigOwner: owner,
		}

		_, err := r.resolveVariables(context.Background(), scope)
		require.Error(t, err)
		assert.NotErrorIs(t, err, &requeueError{})
		assert.Contains(t, err.Error(), "key missing-key not found in secret some-metadata")
	})
}

func TestReconcileGenerateWithTemplates(t *testing.T) {
	s := scheme.Scheme
	_ = bootstrapv1beta1.AddToScheme(s)
	_ = corev1.AddToScheme(s)
	_ = capiv1.AddToScheme(s)

	t.Run("Evaluate templates and write rendered bootstrap secret", func(t *testing.T) {
		config := &bootstrapv1beta1.TalosConfig{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      "my-config",
			},
			Spec: bootstrapv1beta1.TalosConfigSpec{
				GenerateType: "none",
				Data:         "version: v1alpha1\nmachine:\n  network:\n    hostname: {{ .Machine.Name }}\n  time:\n    servers:\n      - {{ .Vars.dns }}",
				Variables: []bootstrapv1beta1.Variable{
					{
						Name: "dns",
						ValueFrom: bootstrapv1beta1.VariableSource{
							SecretKeyRef: &bootstrapv1beta1.SecretKeySelector{
								Name: "global-config",
								Key:  "dns",
							},
						},
					},
				},
				StrategicPatches: []string{
					"machine:\n  time:\n    servers: [{{ .Vars.dns }}]",
				},
				ConfigPatches: []bootstrapv1beta1.ConfigPatches{
					{
						Op:   "add",
						Path: "/machine/install",
						Value: apiextensions.JSON{
							Raw: []byte(`{"disk": "/dev/sda"}`),
						},
					},
				},
			},
		}

		globalSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      "global-config",
			},
			Data: map[string][]byte{
				"dns": []byte("1.1.1.1"),
			},
		}

		fakeClient := fake.NewClientBuilder().WithScheme(s).WithObjects(globalSecret).Build()
		r := &TalosConfigReconciler{
			Client: fakeClient,
			Log:    ctrl.Log,
		}

		cluster := &capiv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      "my-cluster",
			},
		}

		machineObj := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "cluster.x-k8s.io/v1beta2",
				"kind":       "Machine",
				"metadata": map[string]interface{}{
					"name":      "node-0",
					"namespace": "default",
				},
			},
		}
		owner := &bsutil.ConfigOwner{
			Unstructured: machineObj,
		}

		scope := &TalosConfigScope{
			Config:      config,
			ConfigOwner: owner,
			Cluster:     cluster,
		}

		err := r.reconcileGenerate(context.Background(), scope)
		require.NoError(t, err)

		// The reconciler should have written the rendered bootstrap data to a secret named "node-0-bootstrap-data"
		bootstrapSecret := &corev1.Secret{}
		secretKey := client.ObjectKey{Namespace: "default", Name: "node-0-bootstrap-data"}
		err = fakeClient.Get(context.Background(), secretKey, bootstrapSecret)
		require.NoError(t, err)

		bootstrapData := string(bootstrapSecret.Data["value"])
		assert.Contains(t, bootstrapData, "hostname: node-0")
		// Assert strategic patch and initial data evaluation succeeded
		assert.Contains(t, bootstrapData, "- 1.1.1.1")
		// Assert json patch applied
		assert.Contains(t, bootstrapData, "disk: /dev/sda")

		// Assert that the original Specs were NOT modified (prevents writing rendered values back to k8s api)
		assert.Equal(t, "version: v1alpha1\nmachine:\n  network:\n    hostname: {{ .Machine.Name }}\n  time:\n    servers:\n      - {{ .Vars.dns }}", config.Spec.Data)
		assert.Equal(t, "machine:\n  time:\n    servers: [{{ .Vars.dns }}]", config.Spec.StrategicPatches[0])
	})
}
