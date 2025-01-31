package references

import (
	networking_v1alpha3 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	security_v1beta "istio.io/client-go/pkg/apis/security/v1beta1"

	"github.com/kiali/kiali/kubernetes"
	"github.com/kiali/kiali/models"
)

type ServiceEntryReferences struct {
	Namespace             string
	Namespaces            models.Namespaces
	ServiceEntries        []networking_v1alpha3.ServiceEntry
	Sidecars              []networking_v1alpha3.Sidecar
	AuthorizationPolicies []security_v1beta.AuthorizationPolicy
	DestinationRules      []networking_v1alpha3.DestinationRule
	RegistryServices      []*kubernetes.RegistryService
}

func (n ServiceEntryReferences) References() models.IstioReferencesMap {
	result := models.IstioReferencesMap{}

	for _, se := range n.ServiceEntries {
		key := models.IstioReferenceKey{Namespace: se.Namespace, Name: se.Name, ObjectType: models.ObjectTypeSingular[kubernetes.ServiceEntries]}
		references := &models.IstioReferences{}
		references.ObjectReferences = append(references.ObjectReferences, n.getConfigReferences(se)...)
		references.ServiceReferences = append(references.ServiceReferences, n.getServiceReferences(se)...)
		result.MergeReferencesMap(models.IstioReferencesMap{key: references})
	}

	return result

}

func (n ServiceEntryReferences) getConfigReferences(se networking_v1alpha3.ServiceEntry) []models.IstioReference {
	result := make([]models.IstioReference, 0)
	for _, dr := range n.DestinationRules {
		fqdn := kubernetes.GetHost(dr.Spec.Host, dr.Namespace, dr.ClusterName, n.Namespaces.GetNames())
		if !fqdn.IsWildcard() {
			for _, seHost := range se.Spec.Hosts {
				if seHost == fqdn.String() {
					result = append(result, models.IstioReference{Name: dr.Name, Namespace: dr.Namespace, ObjectType: models.ObjectTypeSingular[kubernetes.DestinationRules]})
					continue
				}
			}
		}
	}
	for _, sc := range n.Sidecars {
		for _, ei := range sc.Spec.Egress {
			if ei == nil {
				continue
			}
			if len(ei.Hosts) > 0 {
				for _, h := range ei.Hosts {
					hostNs, dnsName, _ := getHostComponents(h)
					if hostNs == "*" || hostNs == "~" || hostNs == "." || dnsName == "*" {
						continue
					}
					fqdn := kubernetes.ParseHost(dnsName, hostNs, sc.ClusterName)

					if se.Namespace != hostNs {
						continue
					}
					for _, seHost := range se.Spec.Hosts {
						if seHost == fqdn.String() {
							result = append(result, models.IstioReference{Name: sc.Name, Namespace: sc.Namespace, ObjectType: models.ObjectTypeSingular[kubernetes.Sidecars]})
							break
						}
					}
				}
			}
		}
	}
	result = append(result, n.getAuthPoliciesReferences(se)...)
	return result
}

func (n ServiceEntryReferences) getAuthPoliciesReferences(se networking_v1alpha3.ServiceEntry) []models.IstioReference {
	result := make([]models.IstioReference, 0)
	for _, ap := range n.AuthorizationPolicies {
		namespace, clusterName := ap.Namespace, ap.ClusterName
		for _, rule := range ap.Spec.Rules {
			if rule == nil {
				continue
			}
			if len(rule.To) > 0 {
				for _, t := range rule.To {
					if t == nil || t.Operation == nil || len(t.Operation.Hosts) == 0 {
						continue
					}
					for _, h := range t.Operation.Hosts {
						fqdn := kubernetes.GetHost(h, namespace, clusterName, n.Namespaces.GetNames())
						if !fqdn.IsWildcard() {
							for _, seHost := range se.Spec.Hosts {
								if seHost == fqdn.String() {
									result = append(result, models.IstioReference{Name: ap.Name, Namespace: ap.Namespace, ObjectType: models.ObjectTypeSingular[kubernetes.AuthorizationPolicies]})
									continue
								}
							}
						}
					}
				}
			}
		}
	}
	return result
}

func (n ServiceEntryReferences) getServiceReferences(se networking_v1alpha3.ServiceEntry) []models.ServiceReference {
	result := make([]models.ServiceReference, 0)
	keys := make(map[string]bool)
	allServices := make([]models.ServiceReference, 0)
	for _, seHost := range se.Spec.Hosts {
		for _, rStatus := range n.RegistryServices {
			if kubernetes.FilterByRegistryService(se.Namespace, seHost, rStatus) {
				allServices = append(allServices, models.ServiceReference{Name: rStatus.Hostname, Namespace: rStatus.IstioService.Attributes.Namespace})
			}
		}
	}
	// filter unique references
	for _, s := range allServices {
		if !keys[s.Name+"."+s.Namespace] {
			result = append(result, s)
			keys[s.Name+"."+s.Namespace] = true
		}
	}
	return result
}
