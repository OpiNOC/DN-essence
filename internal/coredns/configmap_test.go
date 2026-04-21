package coredns

import (
	"strings"
	"testing"
)

const baseCorefile = `.:53 {
    errors
    health {
       lameduck 5s
    }
    ready
    kubernetes cluster.local in-addr.arpa ip6.arpa {
       pods insecure
       fallthrough in-addr.arpa ip6.arpa
       ttl 30
    }
    forward . /etc/resolv.conf
    cache 30
    loop
    reload
    loadbalance
}`

func TestBuildManagedBlock_empty(t *testing.T) {
	block := BuildManagedBlock(nil)
	if !strings.Contains(block, beginMarker) || !strings.Contains(block, endMarker) {
		t.Errorf("expected both markers in empty block, got: %q", block)
	}
}

func TestBuildManagedBlock_entries(t *testing.T) {
	lines := []string{
		"rewrite name api.example.com svc.default.svc.cluster.local",
		"rewrite name grafana.example.com grafana.monitoring.svc.cluster.local",
	}
	block := BuildManagedBlock(lines)
	for _, l := range lines {
		if !strings.Contains(block, l) {
			t.Errorf("expected line %q in block", l)
		}
	}
}

func TestApplyManagedBlock_inject(t *testing.T) {
	lines := []string{"rewrite name api.example.com svc.default.svc.cluster.local"}
	result, err := ApplyManagedBlock(baseCorefile, lines)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, beginMarker) {
		t.Error("expected BEGIN marker after inject")
	}
	if !strings.Contains(result, lines[0]) {
		t.Error("expected rewrite line after inject")
	}
	if !strings.Contains(result, "reload") {
		t.Error("existing config should be preserved")
	}
}

func TestApplyManagedBlock_replace(t *testing.T) {
	// First inject
	lines1 := []string{"rewrite name api.example.com svc.default.svc.cluster.local"}
	withBlock, _ := ApplyManagedBlock(baseCorefile, lines1)

	// Then replace with different rules
	lines2 := []string{
		"rewrite name api.example.com svc.default.svc.cluster.local",
		"rewrite name grafana.example.com grafana.monitoring.svc.cluster.local",
	}
	result, err := ApplyManagedBlock(withBlock, lines2)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(result, beginMarker) != 1 {
		t.Error("expected exactly one BEGIN marker after replace")
	}
	if !strings.Contains(result, lines2[1]) {
		t.Error("expected new rule after replace")
	}
	if !strings.Contains(result, "reload") {
		t.Error("existing config should be preserved after replace")
	}
}

func TestApplyManagedBlock_idempotent(t *testing.T) {
	lines := []string{"rewrite name api.example.com svc.default.svc.cluster.local"}
	result1, _ := ApplyManagedBlock(baseCorefile, lines)
	result2, _ := ApplyManagedBlock(result1, lines)
	if result1 != result2 {
		t.Error("apply should be idempotent when rules don't change")
	}
}

func TestIsUpToDate(t *testing.T) {
	lines := []string{"rewrite name api.example.com svc.default.svc.cluster.local"}
	withBlock, _ := ApplyManagedBlock(baseCorefile, lines)

	if !IsUpToDate(withBlock, lines) {
		t.Error("expected IsUpToDate=true for same rules")
	}
	if IsUpToDate(withBlock, []string{"rewrite name other.example.com other.default.svc.cluster.local"}) {
		t.Error("expected IsUpToDate=false for different rules")
	}
}

func TestApplyManagedBlock_noBrace(t *testing.T) {
	_, err := ApplyManagedBlock("no brace here", []string{"rewrite name x y"})
	if err == nil {
		t.Error("expected error when no closing brace found")
	}
}

func TestApplyManagedBlock_preservesOutsideContent(t *testing.T) {
	lines := []string{"rewrite name api.example.com svc.default.svc.cluster.local"}
	result, _ := ApplyManagedBlock(baseCorefile, lines)

	mustContain := []string{"errors", "health", "kubernetes", "forward", "cache", "loop", "reload", "loadbalance"}
	for _, keyword := range mustContain {
		if !strings.Contains(result, keyword) {
			t.Errorf("existing config keyword %q should be preserved", keyword)
		}
	}
}
