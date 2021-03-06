/*
 * Copyright 2018-2019, EnMasse authors.
 * License: Apache License 2.0 (see the file LICENSE or http://apache.org/licenses/LICENSE-2.0.html).
 */

package iotproject

import (
	"context"

	enmassev1beta1 "github.com/enmasseproject/enmasse/pkg/apis/enmasse/v1beta1"
	iotv1alpha1 "github.com/enmasseproject/enmasse/pkg/apis/iot/v1alpha1"
	userv1beta1 "github.com/enmasseproject/enmasse/pkg/apis/user/v1beta1"
	"github.com/enmasseproject/enmasse/pkg/util"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func (r *ReconcileIoTProject) reconcileManaged(ctx context.Context, request *reconcile.Request, project *iotv1alpha1.IoTProject) (*iotv1alpha1.ExternalDownstreamStrategy, error) {

	log.Info("Reconcile project with managed strategy")

	strategy := project.Spec.DownstreamStrategy.ManagedDownstreamStrategy

	// reconcile address space

	addressSpace := &enmassev1beta1.AddressSpace{
		ObjectMeta: v1.ObjectMeta{Namespace: project.Namespace, Name: strategy.AddressSpaceName},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.client, addressSpace, func(existing runtime.Object) error {
		existingAddressSpace := existing.(*enmassev1beta1.AddressSpace)

		if err := r.ensureOwnerIsSet(project, existingAddressSpace); err != nil {
			return err
		}

		log.Info("Reconcile address space", "AddressSpace", existingAddressSpace)

		return r.reconcileAddressSpace(project, strategy, existingAddressSpace)
	})

	if err != nil {
		log.Error(err, "Failed calling CreateOrUpdate")
		return nil, err
	}

	// create a set of addresses

	err = r.reconcileAddressSet(ctx, project, strategy)

	if err != nil {
		log.Error(err, "Failed to create addresses")
	}

	// create a new user for protocol adapters

	credentials, err := r.reconcileAdapterUser(ctx, project, strategy)
	if err != nil {
		log.Error(err, "failed to create adapter user")
		return nil, err
	}

	// extract endpoint information

	forceTls := true
	return extractEndpointInformation("messaging", iotv1alpha1.Service, "amqps", credentials, addressSpace, &forceTls)
}

func (r *ReconcileIoTProject) reconcileAdapterUser(ctx context.Context, project *iotv1alpha1.IoTProject, strategy *iotv1alpha1.ManagedDownstreamStrategy) (*iotv1alpha1.Credentials, error) {

	adapterUserName := "adapter"
	adapterUser := &userv1beta1.MessagingUser{
		ObjectMeta: v1.ObjectMeta{Namespace: project.Namespace, Name: strategy.AddressSpaceName + "." + adapterUserName},
	}

	credentials := &iotv1alpha1.Credentials{
		Username: adapterUserName,
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.client, adapterUser, func(existing runtime.Object) error {
		existingUser := existing.(*userv1beta1.MessagingUser)

		log.Info("Reconcile messaging user", "MessagingUser", existingUser)

		return r.reconcileAdapterMessagingUser(project, credentials, existingUser)
	})

	return credentials, err
}

func (r *ReconcileIoTProject) reconcileAddress(project *iotv1alpha1.IoTProject, strategy *iotv1alpha1.ManagedDownstreamStrategy, addressName string, plan string, typeName string, existing *enmassev1beta1.Address) error {

	if err := r.ensureControllerOwnerIsSet(project, existing); err != nil {
		return err
	}

	existing.Spec.Address = addressName
	existing.Spec.Plan = plan
	existing.Spec.Type = typeName

	return nil
}

func (r *ReconcileIoTProject) createOrUpdateAddress(ctx context.Context, project *iotv1alpha1.IoTProject, strategy *iotv1alpha1.ManagedDownstreamStrategy, addressBaseName string, plan string, typeName string) error {

	addressName := util.AddressName(project, addressBaseName)
	addressMetaName := util.EncodeAddressSpaceAsMetaName(strategy.AddressSpaceName, addressName)

	log.Info("Creating/updating address", "basename", addressBaseName, "name", addressName, "metaname", addressMetaName)

	address := &enmassev1beta1.Address{
		ObjectMeta: v1.ObjectMeta{Namespace: project.Namespace, Name: addressMetaName},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.client, address, func(existing runtime.Object) error {
		existingAddress := existing.(*enmassev1beta1.Address)

		return r.reconcileAddress(project, strategy, addressName, plan, typeName, existingAddress)
	})

	return err
}

func (r *ReconcileIoTProject) reconcileAddressSet(ctx context.Context, project *iotv1alpha1.IoTProject, strategy *iotv1alpha1.ManagedDownstreamStrategy) error {

	mt := util.MultiTool{}

	mt.Run(func() error {
		return r.createOrUpdateAddress(ctx, project, strategy, "telemetry", "standard-small-anycast", "anycast")
	})
	mt.Run(func() error {
		return r.createOrUpdateAddress(ctx, project, strategy, "event", "standard-small-queue", "queue")
	})
	mt.Run(func() error {
		return r.createOrUpdateAddress(ctx, project, strategy, "control", "standard-small-anycast", "anycast")
	})

	return mt.Error

}

func (r *ReconcileIoTProject) reconcileAddressSpace(project *iotv1alpha1.IoTProject, strategy *iotv1alpha1.ManagedDownstreamStrategy, existing *enmassev1beta1.AddressSpace) error {

	if existing.CreationTimestamp.IsZero() {
		existing.ObjectMeta.Labels = project.Labels
	}

	existing.Spec.Type = "standard"
	existing.Spec.Plan = "standard-unlimited"

	return nil
}

func (r *ReconcileIoTProject) reconcileAdapterMessagingUser(project *iotv1alpha1.IoTProject, credentials *iotv1alpha1.Credentials, existing *userv1beta1.MessagingUser) error {

	if err := r.ensureControllerOwnerIsSet(project, existing); err != nil {
		return err
	}

	username := credentials.Username

	telemetryName := util.AddressName(project, "telemetry")
	eventName := util.AddressName(project, "event")
	commandName := util.AddressName(project, "command")

	existing.Spec.Username = username
	existing.Spec.Authentication = userv1beta1.AuthenticationSpec{
		Type: "password",
	}

	// if the status has endpoint information ...

	if project.Status.DownstreamEndpoint != nil {
		// ... we extract the password and return it
		credentials.Password = project.Status.DownstreamEndpoint.Password
	}
	newPassword := false
	// if the password is not set ...
	if credentials.Password == "" {
		// ... we generate a new one ...
		password, err := util.GeneratePassword(32)
		if err != nil {
			return err
		}
		// ... and return it in the credentials structure
		credentials.Password = password
		newPassword = true
	}

	if newPassword || util.IsNewObject(existing) {
		// only set password when we are creating the object initially
		// or created a new password ... as we cannot detect a change
		existing.Spec.Authentication.Password = []byte(credentials.Password)
	}

	existing.Spec.Authorization = []userv1beta1.AuthorizationSpec{
		{
			Addresses: []string{
				telemetryName,
				telemetryName + "/*",
				eventName,
				eventName + "/*",
				commandName,
				commandName + "/*",
			},
			Operations: []string{
				"send",
				"recv",
			},
		},
	}

	return nil
}
