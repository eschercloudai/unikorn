/*
Copyright 2022 EscherCloud.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package vcluster

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/eschercloudai/unikorn/pkg/util/retry"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	ErrConfigDataMissing          = errors.New("config data not found")
	ErrLoadBalancerIngressMissing = errors.New("ingress address not found")
)

func RESTConfig(ctx context.Context, client client.Client, namespace, name string) (*rest.Config, error) {
	return clientcmd.BuildConfigFromKubeconfigGetter("", KubeConfigGetter(ctx, client, namespace, name))
}

func KubeConfigGetter(ctx context.Context, client client.Client, namespace, name string) clientcmd.KubeconfigGetter {
	return func() (*clientcmdapi.Config, error) {
		return GetConfig(ctx, NewControllerRuntimeGetter(client), namespace, name, false)
	}
}

// ConfigGetter abstracts the fact that we call this code from a controller-runtime
// world, and a kubectl one, each having wildly different client models.
type ConfigGetter interface {
	GetSecret(ctx context.Context, namespace, name string) (*corev1.Secret, error)
	GetService(ctx context.Context, namespace, name string) (*corev1.Service, error)
}

type ControllerRuntimeGetter struct {
	client client.Client
}

func NewControllerRuntimeGetter(client client.Client) *ControllerRuntimeGetter {
	return &ControllerRuntimeGetter{
		client: client,
	}
}

func (g *ControllerRuntimeGetter) GetSecret(ctx context.Context, namespace, name string) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	if err := g.client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, secret); err != nil {
		return nil, err
	}

	return secret, nil
}

func (g *ControllerRuntimeGetter) GetService(ctx context.Context, namespace, name string) (*corev1.Service, error) {
	service := &corev1.Service{}
	if err := g.client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, service); err != nil {
		return nil, err
	}

	return service, nil
}

type KubectlGetter struct {
	client kubernetes.Interface
}

func NewKubectlGetter(client kubernetes.Interface) *KubectlGetter {
	return &KubectlGetter{
		client: client,
	}
}

func (g *KubectlGetter) GetSecret(ctx context.Context, namespace, name string) (*corev1.Secret, error) {
	secret, err := g.client.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return secret, nil
}

func (g *KubectlGetter) GetService(ctx context.Context, namespace, name string) (*corev1.Service, error) {
	service, err := g.client.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return service, nil
}

// GetConfig acknowledges that vcluster configuration is synchronized by a side car, so it
// performs a retry until the provided context expires.  It also acknowledges that load
// balancer services may take a while to get a public IP.
func GetConfig(ctx context.Context, getter ConfigGetter, namespace, name string, external bool) (*clientcmdapi.Config, error) {
	var config *clientcmdapi.Config

	callback := func() error {
		secret, err := getter.GetSecret(ctx, namespace, "vc-"+name)
		if err != nil {
			return err
		}

		// Acquire the kubeconfig and hack it so that the server points to the
		// LoadBalancer endpoint.
		configBytes, ok := secret.Data["config"]
		if !ok {
			return ErrConfigDataMissing
		}

		configStruct, err := clientcmd.NewClientConfigFromBytes(configBytes)
		if err != nil {
			return err
		}

		service, err := getter.GetService(ctx, namespace, name)
		if err != nil {
			return err
		}

		configRaw, err := configStruct.RawConfig()
		if err != nil {
			return err
		}

		host := "https://" + service.Spec.ClusterIP + ":443"

		if external {
			if len(service.Status.LoadBalancer.Ingress) == 0 {
				return ErrLoadBalancerIngressMissing
			}

			host = "https://" + service.Status.LoadBalancer.Ingress[0].IP + ":443"
		}

		configRaw.Clusters["my-vcluster"].Server = host

		config = &configRaw

		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	if err := retry.Forever().DoWithContext(ctx, callback); err != nil {
		return nil, err
	}

	return config, nil
}

// WriteConfig writes a vcluster config to a temporary location.  It returns a path to the config,
// a cleanup callback that should be invoked via a defer in a non nil error.
func WriteConfig(ctx context.Context, getter ConfigGetter, namespace, name string) (string, func(), error) {
	config, err := GetConfig(ctx, getter, namespace, name, true)
	if err != nil {
		return "", nil, err
	}

	tf, err := os.CreateTemp("", "")
	if err != nil {
		return "", nil, err
	}

	tf.Close()

	if err := clientcmd.WriteToFile(*config, tf.Name()); err != nil {
		os.Remove(tf.Name())

		return "", nil, err
	}

	cleanup := func() {
		os.Remove(tf.Name())
	}

	return tf.Name(), cleanup, nil
}

func WriteInClusterConfig(ctx context.Context, getter ConfigGetter, namespace, name string) (string, func(), error) {
	config, err := GetConfig(ctx, getter, namespace, name, false)
	if err != nil {
		return "", nil, err
	}

	tf, err := os.CreateTemp("", "")
	if err != nil {
		return "", nil, err
	}

	tf.Close()

	if err := clientcmd.WriteToFile(*config, tf.Name()); err != nil {
		os.Remove(tf.Name())

		return "", nil, err
	}

	cleanup := func() {
		os.Remove(tf.Name())
	}

	return tf.Name(), cleanup, nil
}
