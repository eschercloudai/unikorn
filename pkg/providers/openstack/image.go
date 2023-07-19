/*
Copyright 2022-2023 EscherCloud.

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

package openstack

import (
	"context"
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/base64"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/imageservice/v2/images"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"

	"github.com/eschercloudai/unikorn/pkg/constants"
)

// ImageClient wraps the generic client because gophercloud is unsafe.
type ImageClient struct {
	client *gophercloud.ServiceClient
}

// NewImageClient provides a simple one-liner to start computing.
func NewImageClient(provider Provider) (*ImageClient, error) {
	providerClient, err := provider.Client()
	if err != nil {
		return nil, err
	}

	client, err := openstack.NewImageServiceV2(providerClient, gophercloud.EndpointOpts{})
	if err != nil {
		return nil, err
	}

	c := &ImageClient{
		client: client,
	}

	return c, nil
}

// verifyImage asserts the image is trustworthy for use with our goodselves.
func verifyImage(image *images.Image, key *ecdsa.PublicKey) bool {
	if image.Properties == nil {
		return false
	}

	// Legacy behaviour (for unikornctl only) that accepts images if
	// they have Kubernetes and Nvidia driver versions.
	if key == nil {
		if value, ok := image.Properties["k8s"]; !ok || value == "" {
			return false
		}

		if value, ok := image.Properties["gpu"]; !ok || value == "" {
			return false
		}

		return true
	}

	// These will be digitally signed by Baski when created, so we only trust
	// those images.
	signatureRaw, ok := image.Properties["digest"]
	if !ok {
		return false
	}

	signatureB64, ok := signatureRaw.(string)
	if !ok {
		return false
	}

	signature, err := base64.StdEncoding.DecodeString(signatureB64)
	if err != nil {
		return false
	}

	hash := sha256.Sum256([]byte(image.ID))

	return ecdsa.VerifyASN1(key, hash[:], signature)
}

// Images returns a list of images.
func (c *ImageClient) Images(ctx context.Context, key *ecdsa.PublicKey) ([]images.Image, error) {
	tracer := otel.GetTracerProvider().Tracer(constants.Application)

	_, span := tracer.Start(ctx, "/imageservice/v2/images", trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	page, err := images.List(c.client, &images.ListOpts{}).AllPages()
	if err != nil {
		return nil, err
	}

	result, err := images.ExtractImages(page)
	if err != nil {
		return nil, err
	}

	// Filter out images that aren't compatible.
	filtered := []images.Image{}

	for i := range result {
		image := result[i]

		if image.Status != "active" {
			continue
		}

		if !verifyImage(&image, key) {
			continue
		}

		filtered = append(filtered, image)
	}

	return filtered, nil
}
