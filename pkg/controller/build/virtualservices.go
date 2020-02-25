package build

import (
	"fmt"
	"reflect"

	"github.com/kolonialno/test-environment-manager/pkg/internal"

	"github.com/kolonialno/test-environment-manager/pkg/apis/networking/v1alpha3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// nolint: gocyclo
func (br *buildReconciler) reconcileViritualServices() error {
	buildName := fmt.Sprintf(
		"%s-%s-%d",
		br.build.Spec.Git.Owner,
		br.build.Spec.Git.Repository,
		br.build.Spec.Git.PullRequestNumber,
	)

	buildURL := internal.GenerateBuildURL(
		br.build.Spec.Git.Owner,
		br.build.Spec.Git.Repository,
		br.build.Spec.Git.PullRequestNumber,
		options.ClusterDomain,
	)

	logger := br.logger.WithField("virtualservice", buildName)

	httpRoutes := []v1alpha3.HTTPRoute{}

	// Add remote terminal route
	terminalDestination := v1alpha3.Destination{
		Host: fmt.Sprintf(
			"%s.%s.svc.cluster.local",
			options.StatusServiceName, options.Namespace,
		),
		Port: v1alpha3.PortSelector{
			Number: options.StatusServicePort,
		},
	}
	httpRoutes = append(httpRoutes, v1alpha3.HTTPRoute{
		Match: []v1alpha3.HTTPMatchRequest{
			{
				URI: v1alpha3.StringMatch{
					Prefix: "/term/",
				},
			},
		},
		Destination: []v1alpha3.DestinationWeight{
			{
				Destination: terminalDestination,
			},
		},
		WebsocketUpgrade: true,
	})

	// Add user defined routes
	for _, route := range br.environment.Spec.Routing {
		serviceName := fmt.Sprintf("%s-container", route.ContainerName)
		namespaceName := fmt.Sprintf("%s%s", options.BuildPrefix, buildName)

		// Lookup servcice endpoints - direct traffic to the status page if no endpoints are available
		var activeEndpoints bool
		{
			endpoints := &corev1.Endpoints{}
			err := br.r.Get(br.ctx, types.NamespacedName{Name: serviceName, Namespace: namespaceName}, endpoints)
			if err != nil {
				return err
			}

			// Try to lookup matching endpont addresses
			for _, subset := range endpoints.Subsets {
				for _, port := range subset.Ports {
					if int64(port.Port) == route.Port {
						activeEndpoints = len(subset.Addresses) != 0
					}
				}
			}
		}

		var destination v1alpha3.Destination
		if activeEndpoints {
			destination = v1alpha3.Destination{
				Host: fmt.Sprintf(
					"%s.%s.svc.cluster.local",
					serviceName, namespaceName,
				),
				Port: v1alpha3.PortSelector{
					Number: route.Port,
				},
			}
		} else {
			destination = v1alpha3.Destination{
				Host: fmt.Sprintf(
					"%s.%s.svc.cluster.local",
					options.StatusServiceName, options.Namespace,
				),
				Port: v1alpha3.PortSelector{
					Number: options.StatusServicePort,
				},
			}
		}

		httpRoutes = append(httpRoutes, v1alpha3.HTTPRoute{
			Match: []v1alpha3.HTTPMatchRequest{
				{
					URI: v1alpha3.StringMatch{
						Prefix: route.URLPrefix,
					},
				},
			},
			Destination: []v1alpha3.DestinationWeight{
				{
					Destination: destination,
				},
			},
			WebsocketUpgrade: true,
		})
	}

	// Add redirects
	for _, redirect := range br.environment.Spec.Redirects {
		httpRoutes = append(httpRoutes, v1alpha3.HTTPRoute{
			Match: []v1alpha3.HTTPMatchRequest{
				{
					URI: v1alpha3.StringMatch{
						Prefix: redirect.URLPrefix,
					},
				},
			},
			Redirect: &v1alpha3.HTTPRedirect{
				URI: redirect.Destination,
			},
		})
	}

	if len(httpRoutes) == 0 {
		logger.Warn("skipping virtualservice without routes")
		return nil
	}

	deploy := &v1alpha3.VirtualService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      buildName,
			Namespace: options.IstioNamespace,
		},
		Spec: v1alpha3.VirtualServiceSpec{
			Gateways: []string{options.IstioGateway},
			Hosts:    []string{buildURL},
			HTTP:     httpRoutes,
		},
	}
	if err := controllerutil.SetControllerReference(br.build, deploy, br.r.scheme); err != nil {
		return err
	}

	found := &v1alpha3.VirtualService{}
	err := br.r.Get(br.ctx, types.NamespacedName{Name: deploy.Name, Namespace: deploy.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		logger.Info("creating viritualservice")
		return br.r.Create(br.ctx, deploy)
	} else if err != nil {
		return err
	}

	// Use DeepEqual, we do a lot of endpoint maipulation because of the status server.
	if !reflect.DeepEqual(deploy.Spec, found.Spec) {
		found.Spec = deploy.Spec
		logger.Info("updating viritualservice")
		return br.r.Update(br.ctx, found)
	}

	return nil
}
