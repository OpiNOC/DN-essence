package controller

import (
	"context"
	"fmt"
	"sort"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	dnsv1 "github.com/yourorg/dn-essence/api/v1"
	"github.com/yourorg/dn-essence/internal/coredns"
)

const (
	defaultCoreDNSNamespace = "kube-system"
	defaultCoreDNSCMName    = "coredns"
	coreDNSDataKey          = "Corefile"
)

// DNSRewriteReconciler reconciles DNSRewrite objects.
type DNSRewriteReconciler struct {
	client.Client
	CoreDNSNamespace string
	CoreDNSCMName    string
}

func (r *DNSRewriteReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("reconciling", "trigger", req.NamespacedName)

	// 1. List all DNSRewrite resources cluster-wide.
	var list dnsv1.DNSRewriteList
	if err := r.List(ctx, &list); err != nil {
		return ctrl.Result{}, fmt.Errorf("list DNSRewrites: %w", err)
	}

	// 2. Build sorted rewrite lines from enabled rules.
	var lines []string
	for _, item := range list.Items {
		if item.Spec.Enabled {
			lines = append(lines, fmt.Sprintf(
				"rewrite name %s %s", item.Spec.Host, item.Spec.Target,
			))
		}
	}
	sort.Strings(lines) // deterministic output

	// 3. Fetch ConfigMap.
	cmKey := types.NamespacedName{
		Namespace: r.coreDNSNamespace(),
		Name:      r.coreDNSCMName(),
	}
	var cm corev1.ConfigMap
	if err := r.Get(ctx, cmKey, &cm); err != nil {
		return ctrl.Result{}, fmt.Errorf("get ConfigMap %s: %w", cmKey, err)
	}

	// 4. Patch only if needed (idempotency).
	currentData := cm.Data[coreDNSDataKey]
	if coredns.IsUpToDate(currentData, lines) {
		logger.Info("ConfigMap already up-to-date, nothing to do")
		return ctrl.Result{}, r.updateAllStatuses(ctx, list.Items, true, "")
	}

	// 5. Apply managed block.
	newData, err := coredns.ApplyManagedBlock(currentData, lines)
	if err != nil {
		_ = r.updateAllStatuses(ctx, list.Items, false, err.Error())
		return ctrl.Result{}, fmt.Errorf("build managed block: %w", err)
	}

	// 6. Patch ConfigMap.
	patch := client.MergeFrom(cm.DeepCopy())
	if cm.Data == nil {
		cm.Data = map[string]string{}
	}
	cm.Data[coreDNSDataKey] = newData
	if err := r.Patch(ctx, &cm, patch); err != nil {
		_ = r.updateAllStatuses(ctx, list.Items, false, err.Error())
		return ctrl.Result{}, fmt.Errorf("patch ConfigMap: %w", err)
	}

	logger.Info("ConfigMap patched", "rules", len(lines))
	return ctrl.Result{}, r.updateAllStatuses(ctx, list.Items, true, "")
}

// updateAllStatuses sets .status on every DNSRewrite in the list.
func (r *DNSRewriteReconciler) updateAllStatuses(ctx context.Context, items []dnsv1.DNSRewrite, applied bool, errMsg string) error {
	for i := range items {
		items[i].Status.Applied = applied
		items[i].Status.Error = errMsg
		if err := r.Status().Update(ctx, &items[i]); err != nil {
			return err
		}
	}
	return nil
}

func (r *DNSRewriteReconciler) coreDNSNamespace() string {
	if r.CoreDNSNamespace != "" {
		return r.CoreDNSNamespace
	}
	return defaultCoreDNSNamespace
}

func (r *DNSRewriteReconciler) coreDNSCMName() string {
	if r.CoreDNSCMName != "" {
		return r.CoreDNSCMName
	}
	return defaultCoreDNSCMName
}

// SetupWithManager registers the controller with the manager.
func (r *DNSRewriteReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dnsv1.DNSRewrite{}).
		Complete(r)
}
