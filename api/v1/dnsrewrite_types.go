package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DNSRewriteSpec defines the desired state of a DNS rewrite rule.
type DNSRewriteSpec struct {
	// Host is the public FQDN to rewrite (e.g. api.example.com).
	Host string `json:"host"`
	// Target is the in-cluster Kubernetes FQDN (e.g. svc.namespace.svc.cluster.local).
	Target string `json:"target"`
	// Enabled controls whether this rule is active in CoreDNS.
	Enabled bool `json:"enabled"`
}

// DNSRewriteStatus reflects the observed state after reconciliation.
type DNSRewriteStatus struct {
	// Applied is true when the rule has been successfully written to CoreDNS.
	Applied bool `json:"applied"`
	// Error holds the last reconciliation error message, if any.
	Error string `json:"error,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="Host",type=string,JSONPath=`.spec.host`
// +kubebuilder:printcolumn:name="Target",type=string,JSONPath=`.spec.target`
// +kubebuilder:printcolumn:name="Enabled",type=boolean,JSONPath=`.spec.enabled`
// +kubebuilder:printcolumn:name="Applied",type=boolean,JSONPath=`.status.applied`

// DNSRewrite is the Schema for the dnsrewrites API.
type DNSRewrite struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DNSRewriteSpec   `json:"spec,omitempty"`
	Status DNSRewriteStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DNSRewriteList contains a list of DNSRewrite.
type DNSRewriteList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DNSRewrite `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DNSRewrite{}, &DNSRewriteList{})
}
