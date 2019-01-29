/*
 * Copyright 2018, EnMasse authors.
 * License: Apache License 2.0 (see the file LICENSE or http://apache.org/licenses/LICENSE-2.0.html).
 */

// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	v1beta1 "github.com/enmasseproject/enmasse/pkg/client/clientset/versioned/typed/enmasse/v1beta1"
	rest "k8s.io/client-go/rest"
	testing "k8s.io/client-go/testing"
)

type FakeEnmasseV1beta1 struct {
	*testing.Fake
}

func (c *FakeEnmasseV1beta1) Addresses(namespace string) v1beta1.AddressInterface {
	return &FakeAddresses{c, namespace}
}

func (c *FakeEnmasseV1beta1) AddressSpaces(namespace string) v1beta1.AddressSpaceInterface {
	return &FakeAddressSpaces{c, namespace}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeEnmasseV1beta1) RESTClient() rest.Interface {
	var ret *rest.RESTClient
	return ret
}
