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

	"github.com/eschercloudai/unikorn/pkg/util/retry"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

var (
	ErrConfigDataMissing          = errors.New("config data not found")
	ErrLoadBalancerIngressMissing = errors.New("ingress address not found")
)

func RESTClient(c context.Context, client kubernetes.Interface, namespace, name string) (*rest.Config, error) {
	return clientcmd.BuildConfigFromKubeconfigGetter("", KubeConfigGetter(c, client, namespace, name))
}

func KubeConfigGetter(c context.Context, client kubernetes.Interface, namespace, name string) clientcmd.KubeconfigGetter {
	return func() (*clientcmdapi.Config, error) {
		return GetConfig(c, client, namespace, name)
	}
}

// GetConfig acknowledges that vcluster configuration is synchronized by a side car, so it
// performs a retry until the provided context expires.  It also acknowledges that load
// balancer services may take a while to get a public IP.
func GetConfig(c context.Context, client kubernetes.Interface, namespace, name string) (*clientcmdapi.Config, error) {
	var config *clientcmdapi.Config

	callback := func() error {
		secret, err := client.CoreV1().Secrets(namespace).Get(c, "vc-"+name, metav1.GetOptions{})
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

		service, err := client.CoreV1().Services(namespace).Get(c, name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		configRaw, err := configStruct.RawConfig()
		if err != nil {
			return err
		}

		if len(service.Status.LoadBalancer.Ingress) == 0 {
			return ErrLoadBalancerIngressMissing
		}

		configRaw.Clusters["my-vcluster"].Server = "https://" + service.Status.LoadBalancer.Ingress[0].IP + ":443"

		config = &configRaw

		return nil
	}

	if err := retry.Forever().DoWithContext(c, callback); err != nil {
		return nil, err
	}

	return config, nil
}

// WriteConfig writes a vcluster config to a temporary location.  It returns a path to the config,
// a cleanup callback that should be invoked via a defer in a non nil error.
func WriteConfig(c context.Context, client kubernetes.Interface, namespace, name string) (string, func(), error) {
	config, err := GetConfig(c, client, namespace, name)
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
