package main

import (
	"io/fs"
	"net/http"
	"os"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	dnsv1 "github.com/yourorg/dn-essence/api/v1"
	"github.com/yourorg/dn-essence/internal/api"
	"github.com/yourorg/dn-essence/internal/controller"
	"github.com/yourorg/dn-essence/ui"
)

func main() {
	opts := zap.Options{Development: os.Getenv("DEBUG") == "true"}
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))
	logger := ctrl.Log.WithName("main")

	coreDNSNamespace := getEnv("COREDNS_NAMESPACE", "kube-system")

	scheme := runtime.NewScheme()
	_ = dnsv1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		HealthProbeBindAddress: getEnv("HEALTH_ADDR", ":8081"),
		LeaderElection:         false,
		// Restrict ConfigMap cache to the CoreDNS namespace only —
		// the ServiceAccount has no list permission at cluster scope.
		Cache: cache.Options{
			ByObject: map[client.Object]cache.ByObject{
				&corev1.ConfigMap{}: {
					Namespaces: map[string]cache.Config{
						coreDNSNamespace: {},
					},
				},
			},
		},
	})
	if err != nil {
		logger.Error(err, "failed to create manager")
		os.Exit(1)
	}

	if err := (&controller.DNSRewriteReconciler{
		Client:           mgr.GetClient(),
		CoreDNSNamespace: coreDNSNamespace,
		CoreDNSCMName:    getEnv("COREDNS_CONFIGMAP", "coredns"),
	}).SetupWithManager(mgr); err != nil {
		logger.Error(err, "failed to setup controller")
		os.Exit(1)
	}

	_ = mgr.AddHealthzCheck("healthz", healthz.Ping)
	_ = mgr.AddReadyzCheck("readyz", healthz.Ping)

	// Start the API+UI HTTP server in a separate goroutine.
	go func() {
		mux := http.NewServeMux()

		apiHandler := api.NewHandler(mgr.GetClient())
		apiHandler.Register(mux)

		// Serve UI static files embedded in the binary.
		uiRoot, err := fs.Sub(ui.Files, "dist")
		if err != nil {
			logger.Error(err, "failed to sub ui/dist from embed")
			os.Exit(1)
		}
		mux.Handle("/", http.FileServer(http.FS(uiRoot)))

		addr := getEnv("HTTP_ADDR", ":9090")
		logger.Info("starting HTTP server", "addr", addr)
		if err := http.ListenAndServe(addr, mux); err != nil {
			logger.Error(err, "HTTP server failed")
			os.Exit(1)
		}
	}()

	logger.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		logger.Error(err, "manager exited with error")
		os.Exit(1)
	}
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
