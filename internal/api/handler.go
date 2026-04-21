// Package api exposes a simple REST interface for managing DNSRewrite CRDs.
// It does NOT touch CoreDNS directly — that is the controller's responsibility.
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dnsv1 "github.com/yourorg/dn-essence/api/v1"
)

// Handler is the HTTP handler for the DN-essence REST API.
type Handler struct {
	client client.Client
}

// NewHandler creates a new Handler using the provided controller-runtime client.
func NewHandler(c client.Client) *Handler {
	return &Handler{client: c}
}

// Register mounts all API routes on mux under /api/.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/api/rewrites", h.handleRewrites)
	mux.HandleFunc("/api/rewrites/", h.handleRewriteByName)
}

// rewriteRequest is the JSON body for create/update.
type rewriteRequest struct {
	Name    string `json:"name"`
	Host    string `json:"host"`
	Target  string `json:"target"`
	Enabled bool   `json:"enabled"`
}

// rewriteResponse is the JSON body returned to the client.
type rewriteResponse struct {
	Name    string `json:"name"`
	Host    string `json:"host"`
	Target  string `json:"target"`
	Enabled bool   `json:"enabled"`
	Applied bool   `json:"applied"`
	Error   string `json:"error,omitempty"`
}

func toResponse(r dnsv1.DNSRewrite) rewriteResponse {
	return rewriteResponse{
		Name:    r.Name,
		Host:    r.Spec.Host,
		Target:  r.Spec.Target,
		Enabled: r.Spec.Enabled,
		Applied: r.Status.Applied,
		Error:   r.Status.Error,
	}
}

// handleRewrites handles GET /api/rewrites and POST /api/rewrites.
func (h *Handler) handleRewrites(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listRewrites(w, r)
	case http.MethodPost:
		h.createRewrite(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleRewriteByName handles PUT /api/rewrites/{name} and DELETE /api/rewrites/{name}.
func (h *Handler) handleRewriteByName(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/api/rewrites/")
	if name == "" {
		http.Error(w, "missing name", http.StatusBadRequest)
		return
	}
	switch r.Method {
	case http.MethodPut:
		h.updateRewrite(w, r, name)
	case http.MethodDelete:
		h.deleteRewrite(w, r, name)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) listRewrites(w http.ResponseWriter, r *http.Request) {
	var list dnsv1.DNSRewriteList
	if err := h.client.List(context.Background(), &list); err != nil {
		jsonError(w, "failed to list rewrites: "+err.Error(), http.StatusInternalServerError)
		return
	}
	resp := make([]rewriteResponse, 0, len(list.Items))
	for _, item := range list.Items {
		resp = append(resp, toResponse(item))
	}
	jsonOK(w, resp)
}

func (h *Handler) createRewrite(w http.ResponseWriter, r *http.Request) {
	var req rewriteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid body: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.Name == "" || req.Host == "" || req.Target == "" {
		jsonError(w, "name, host, and target are required", http.StatusBadRequest)
		return
	}

	obj := &dnsv1.DNSRewrite{
		ObjectMeta: metav1.ObjectMeta{Name: req.Name},
		Spec: dnsv1.DNSRewriteSpec{
			Host:    req.Host,
			Target:  req.Target,
			Enabled: req.Enabled,
		},
	}
	if err := h.client.Create(context.Background(), obj); err != nil {
		jsonError(w, "failed to create: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
	jsonOK(w, toResponse(*obj))
}

func (h *Handler) updateRewrite(w http.ResponseWriter, r *http.Request, name string) {
	var req rewriteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid body: "+err.Error(), http.StatusBadRequest)
		return
	}

	var obj dnsv1.DNSRewrite
	if err := h.client.Get(context.Background(), types.NamespacedName{Name: name}, &obj); err != nil {
		jsonError(w, "not found: "+err.Error(), http.StatusNotFound)
		return
	}

	patch := client.MergeFrom(obj.DeepCopy())
	if req.Host != "" {
		obj.Spec.Host = req.Host
	}
	if req.Target != "" {
		obj.Spec.Target = req.Target
	}
	obj.Spec.Enabled = req.Enabled

	if err := h.client.Patch(context.Background(), &obj, patch); err != nil {
		jsonError(w, "failed to update: "+err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, toResponse(obj))
}

func (h *Handler) deleteRewrite(w http.ResponseWriter, r *http.Request, name string) {
	var obj dnsv1.DNSRewrite
	if err := h.client.Get(context.Background(), types.NamespacedName{Name: name}, &obj); err != nil {
		jsonError(w, "not found: "+err.Error(), http.StatusNotFound)
		return
	}
	if err := h.client.Delete(context.Background(), &obj); err != nil {
		jsonError(w, "failed to delete: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func jsonOK(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
