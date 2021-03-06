/*
 * Copyright 2017-2018, EnMasse authors.
 * License: Apache License 2.0 (see the file LICENSE or http://apache.org/licenses/LICENSE-2.0.html).
 */
package io.enmasse.controller;

import io.enmasse.address.model.*;
import io.enmasse.admin.model.v1.InfraConfig;
import io.enmasse.config.AnnotationKeys;
import io.enmasse.controller.common.Kubernetes;
import io.enmasse.controller.common.KubernetesHelper;
import io.enmasse.k8s.api.SchemaProvider;
import io.enmasse.user.api.UserApi;
import io.fabric8.kubernetes.api.model.HasMetadata;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import static io.enmasse.controller.InfraConfigs.parseCurrentInfraConfig;

import java.util.HashSet;
import java.util.List;
import java.util.Optional;
import java.util.Set;
import java.util.stream.Collectors;

public class StatusController implements Controller {
    private static final Logger log = LoggerFactory.getLogger(StatusController.class.getName());
    private final Kubernetes kubernetes;
    private final SchemaProvider schemaProvider;
    private final InfraResourceFactory infraResourceFactory;
    private final UserApi userApi;

    public StatusController(Kubernetes kubernetes, SchemaProvider schemaProvider, InfraResourceFactory infraResourceFactory, UserApi userApi) {
        this.kubernetes = kubernetes;
        this.schemaProvider = schemaProvider;
        this.infraResourceFactory = infraResourceFactory;
        this.userApi = userApi;
    }

    @Override
    public AddressSpace handle(AddressSpace addressSpace) {
        checkComponentsReady(addressSpace);
        checkAuthServiceReady(addressSpace);
        return addressSpace;
    }

    private InfraConfig getInfraConfig(AddressSpace addressSpace) {
        AddressSpaceResolver addressSpaceResolver = new AddressSpaceResolver(schemaProvider.getSchema());
        return addressSpaceResolver.getInfraConfig(addressSpace.getSpec().getType(), addressSpace.getSpec().getPlan());
    }

    private void checkComponentsReady(AddressSpace addressSpace) {
        try {
            InfraConfig infraConfig = Optional.ofNullable(parseCurrentInfraConfig(schemaProvider.getSchema(), addressSpace)).orElseGet(() -> getInfraConfig(addressSpace));
            List<HasMetadata> requiredResources = infraResourceFactory.createInfraResources(addressSpace, infraConfig);

            checkDeploymentsReady(addressSpace, requiredResources);
            checkStatefulSetsReady(addressSpace, requiredResources);
        } catch (Exception e) {
            String msg = String.format("Error checking for ready components: %s", e.getMessage());
            log.warn(msg, e);
            addressSpace.getStatus().setReady(false);
            addressSpace.getStatus().appendMessage(msg);
        }
    }

    private void checkStatefulSetsReady(AddressSpace addressSpace, List<HasMetadata> requiredResources) {
        Set<String> readyStatefulSets = kubernetes.getReadyStatefulSets(addressSpace).stream()
                .map(statefulSet -> statefulSet.getMetadata().getName())
                .collect(Collectors.toSet());


        Set<String> requiredStatefulSets = requiredResources.stream()
                .filter(KubernetesHelper::isStatefulSet)
                .map(item -> item.getMetadata().getName())
                .collect(Collectors.toSet());

        boolean isReady = readyStatefulSets.containsAll(requiredStatefulSets);
        if (!isReady) {
            Set<String> missing = new HashSet<>(requiredStatefulSets);
            missing.removeAll(readyStatefulSets);
            addressSpace.getStatus().setReady(false);
            addressSpace.getStatus().appendMessage("The following stateful sets are not ready: " + missing);
        }
    }

    private void checkDeploymentsReady(AddressSpace addressSpace, List<HasMetadata> requiredResources) {
        Set<String> readyDeployments = kubernetes.getReadyDeployments(addressSpace).stream()
                .map(deployment -> deployment.getMetadata().getName())
                .collect(Collectors.toSet());


        Set<String> requiredDeployments = requiredResources.stream()
                .filter(KubernetesHelper::isDeployment)
                .map(item -> item.getMetadata().getName())
                .collect(Collectors.toSet());

        boolean isReady = readyDeployments.containsAll(requiredDeployments);
        if (!isReady) {
            Set<String> missing = new HashSet<>(requiredDeployments);
            missing.removeAll(readyDeployments);
            addressSpace.getStatus().setReady(false);
            addressSpace.getStatus().appendMessage("The following deployments are not ready: " + missing);
        }
    }

    private void checkAuthServiceReady(AddressSpace addressSpace) {
        if (AuthenticationServiceType.STANDARD.equals(addressSpace.getSpec().getAuthenticationService().getType())) {
            try {
                boolean isReady = userApi.realmExists(addressSpace.getAnnotation(AnnotationKeys.REALM_NAME));
                if (!isReady) {
                    addressSpace.getStatus().setReady(false);
                    addressSpace.getStatus().appendMessage("Standard authentication service is not configured with realm " + addressSpace.getAnnotation(AnnotationKeys.REALM_NAME));
                }
            } catch (Exception e) {
                String msg = String.format("Error checking authentication service status: %s", e.getMessage());
                log.warn(msg);
                addressSpace.getStatus().setReady(false);
                addressSpace.getStatus().appendMessage(msg);
            }
        }
    }

    @Override
    public String toString() {
        return "StatusController";
    }
}
