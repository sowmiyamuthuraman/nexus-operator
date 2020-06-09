//     Copyright 2020 Nexus Operator and/or its authors
//
//     This file is part of Nexus Operator.
//
//     Nexus Operator is free software: you can redistribute it and/or modify
//     it under the terms of the GNU General Public License as published by
//     the Free Software Foundation, either version 3 of the License, or
//     (at your option) any later version.
//
//     Nexus Operator is distributed in the hope that it will be useful,
//     but WITHOUT ANY WARRANTY; without even the implied warranty of
//     MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
//     GNU General Public License for more details.
//
//     You should have received a copy of the GNU General Public License
//     along with Nexus Operator.  If not, see <https://www.gnu.org/licenses/>.

package networking

import (
	ctx "context"
	"fmt"
	"github.com/RHsyseng/operator-utils/pkg/resource"
	"github.com/RHsyseng/operator-utils/pkg/resource/compare"
	"github.com/m88i/nexus-operator/pkg/apis/apps/v1alpha1"
	"github.com/m88i/nexus-operator/pkg/cluster/kubernetes"
	"github.com/m88i/nexus-operator/pkg/cluster/openshift"
	"github.com/m88i/nexus-operator/pkg/controller/nexus/resource/infra"
	"github.com/m88i/nexus-operator/pkg/logger"
	routev1 "github.com/openshift/api/route/v1"
	networkingv1beta1 "k8s.io/api/networking/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	discFailureFormat    = "unable to determine if %s are available: %v" // resource type, error
	resUnavailableFormat = "%s are not available in this cluster"        // resource type
	mgrNotInit           = "the manager has not been initialized"
)

var log = logger.GetLogger("networking_manager")

// manager is responsible for creating networking resources, fetching deployed ones and comparing them
type manager struct {
	nexus  *v1alpha1.Nexus
	client client.Client

	routeAvailable, ingressAvailable bool
}

// NewManager creates a networking resources manager
func NewManager(nexus *v1alpha1.Nexus, client client.Client, disc discovery.DiscoveryInterface) (infra.Manager, error) {
	routeAvailable, err := openshift.IsRouteAvailable(disc)
	if err != nil {
		return nil, fmt.Errorf(discFailureFormat, "routes", err)
	}

	ingressAvailable, err := kubernetes.IsIngressAvailable(disc)
	if err != nil {
		return nil, fmt.Errorf(discFailureFormat, "rngresses", err)
	}

	return &manager{
		nexus:            nexus,
		client:           client,
		routeAvailable:   routeAvailable,
		ingressAvailable: ingressAvailable,
	}, nil
}

// GetRequiredResources returns the resources initialized by the manager
func (m *manager) GetRequiredResources() ([]resource.KubernetesResource, error) {
	var resources []resource.KubernetesResource
	if m.nexus.Spec.Networking.Expose {
		switch m.nexus.Spec.Networking.ExposeAs {
		case v1alpha1.RouteExposeType:
			if !m.routeAvailable {
				return nil, fmt.Errorf(resUnavailableFormat, "Routes")
			}

			log.Debugf("Creating Route (%s)", m.nexus.Name)
			route, err := m.createRoute()
			if err != nil {
				log.Errorf("Could not create Route: %v", err)
				return nil, fmt.Errorf("could not create route: %v", err)
			}
			resources = append(resources, route)

		case v1alpha1.IngressExposeType:
			if !m.ingressAvailable {
				return nil, fmt.Errorf(resUnavailableFormat, "ingresses")
			}

			log.Debugf("Creating Ingress (%s)", m.nexus.Name)
			ingress, err := m.createIngress()
			if err != nil {
				log.Errorf("Could not create Ingress: %v", err)
				return nil, fmt.Errorf("could not create ingress: %v", err)
			}
			resources = append(resources, ingress)
		}
	}
	return resources, nil
}

func (m *manager) createRoute() (*routev1.Route, error) {
	builder := newRouteBuilder(m.nexus)
	if m.nexus.Spec.Networking.TLS.Mandatory {
		builder = builder.withRedirect()
	}
	return builder.build()
}

func (m *manager) createIngress() (*networkingv1beta1.Ingress, error) {
	builder := newIngressBuilder(m.nexus)
	if len(m.nexus.Spec.Networking.TLS.SecretName) > 0 {
		builder = builder.withCustomTLS()
	}
	return builder.build()
}

// GetDeployedResources returns the networking resources deployed on the cluster
func (m *manager) GetDeployedResources() ([]resource.KubernetesResource, error) {
	if m.nexus == nil || m.client == nil {
		return nil, fmt.Errorf(mgrNotInit)
	}

	var resources []resource.KubernetesResource
	if m.routeAvailable {
		if route, err := m.getDeployedRoute(); err == nil {
			resources = append(resources, route)
		} else if !errors.IsNotFound(err) {
			log.Errorf("Could not fetch Route (%s): %v", m.nexus.Name, err)
			return nil, fmt.Errorf("could not fetch route (%s): %v", m.nexus.Name, err)
		}
	}
	if m.ingressAvailable {
		if ingress, err := m.getDeployedIngress(); err == nil {
			resources = append(resources, ingress)
		} else if !errors.IsNotFound(err) {
			log.Errorf("Could not fetch Ingress (%s): %v", m.nexus.Name, err)
			return nil, fmt.Errorf("could not fetch ingress (%s): %v", m.nexus.Name, err)
		}
	}
	return resources, nil
}

func (m *manager) getDeployedRoute() (*routev1.Route, error) {
	route := &routev1.Route{}
	key := types.NamespacedName{Namespace: m.nexus.Namespace, Name: m.nexus.Name}
	err := m.client.Get(ctx.TODO(), key, route)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Debugf("There is no deployed Route (%s)", m.nexus.Name)
		}
		return nil, err
	}
	return route, nil
}

func (m *manager) getDeployedIngress() (*networkingv1beta1.Ingress, error) {
	ingress := &networkingv1beta1.Ingress{}
	key := types.NamespacedName{Namespace: m.nexus.Namespace, Name: m.nexus.Name}
	err := m.client.Get(ctx.TODO(), key, ingress)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Debugf("There is no deployed Ingress (%s)", m.nexus.Name)
		}
		return nil, err
	}
	return ingress, nil
}

// GetCustomComparator returns the custom comp function used to compare a networking resource.
// Returns nil if there is none
func (m *manager) GetCustomComparator(t reflect.Type) func(deployed resource.KubernetesResource, requested resource.KubernetesResource) bool {
	if t == reflect.TypeOf(networkingv1beta1.Ingress{}) {
		return ingressEqual
	}
	return nil
}

// GetCustomComparators returns all custom comp functions in a map indexed by the resource type
// Returns nil if there are none
func (m *manager) GetCustomComparators() map[reflect.Type]func(deployed resource.KubernetesResource, requested resource.KubernetesResource) bool {
	ingressType := reflect.TypeOf(networkingv1beta1.Ingress{})
	return map[reflect.Type]func(deployed resource.KubernetesResource, requested resource.KubernetesResource) bool{
		ingressType: ingressEqual,
	}
}

func ingressEqual(deployed resource.KubernetesResource, requested resource.KubernetesResource) bool {
	ingress1 := deployed.(*networkingv1beta1.Ingress)
	ingress2 := requested.(*networkingv1beta1.Ingress)
	var pairs [][2]interface{}
	pairs = append(pairs, [2]interface{}{ingress1.Name, ingress2.Name})
	pairs = append(pairs, [2]interface{}{ingress1.Namespace, ingress2.Namespace})
	pairs = append(pairs, [2]interface{}{ingress1.Spec, ingress2.Spec})

	equal := compare.EqualPairs(pairs)
	if !equal {
		log.Info("Resources are not equal", "deployed", deployed, "requested", requested)
	}
	return equal
}