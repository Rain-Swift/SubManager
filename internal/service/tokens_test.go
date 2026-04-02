package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"submanager/internal/domain"
	"submanager/internal/fetcher"
	"submanager/internal/parser"
	"submanager/internal/store"
)

func TestDownloadTokenDistributionOverrides(t *testing.T) {
	ctx := context.Background()
	memStore := store.NewMemoryStore()
	manager := NewManager(memStore, fetcher.NewHTTPFetcher())

	hkParsed, err := parser.ParseClashMetaSubscription([]byte(`
proxies:
  - name: HK-01
    type: trojan
    server: hk.example.com
    port: 443
    password: hk-pass
    sni: hk.example.com
    udp: true
  - name: HK-VIP
    type: trojan
    server: hk-vip.example.com
    port: 443
    password: hk-vip-pass
    sni: hk-vip.example.com
    udp: true
`), "sub_hk", "https://example.com/hk.yaml")
	if err != nil {
		t.Fatalf("ParseClashMetaSubscription(hk) error = %v", err)
	}

	usParsed, err := parser.ParseClashMetaSubscription([]byte(`
proxies:
  - name: US-01
    type: trojan
    server: us.example.com
    port: 443
    password: us-pass
    sni: us.example.com
    udp: true
`), "sub_us", "https://example.com/us.yaml")
	if err != nil {
		t.Fatalf("ParseClashMetaSubscription(us) error = %v", err)
	}

	now := time.Now().UTC()
	subscriptionHK := domain.SubscriptionSource{
		ID:         "sub_hk",
		Name:       "hk-subscription",
		URL:        "https://example.com/hk.yaml",
		Enabled:    true,
		TimeoutSec: 15,
		Status:     domain.RefreshStatusSucceeded,
		Snapshot: domain.SubscriptionSnapshot{
			RawProxies: hkParsed.RawProxies,
			Proxies:    hkParsed.Proxies,
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
	subscriptionUS := domain.SubscriptionSource{
		ID:         "sub_us",
		Name:       "us-subscription",
		URL:        "https://example.com/us.yaml",
		Enabled:    true,
		TimeoutSec: 15,
		Status:     domain.RefreshStatusSucceeded,
		Snapshot: domain.SubscriptionSnapshot{
			RawProxies: usParsed.RawProxies,
			Proxies:    usParsed.Proxies,
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
	if _, err := memStore.SaveSubscription(subscriptionHK); err != nil {
		t.Fatalf("SaveSubscription(subscriptionHK) error = %v", err)
	}
	if _, err := memStore.SaveSubscription(subscriptionUS); err != nil {
		t.Fatalf("SaveSubscription(subscriptionUS) error = %v", err)
	}

	baseRule := domain.RuleSource{
		ID:         "rule_base",
		Name:       "base-rule",
		URL:        "https://example.com/base.yaml",
		Mode:       domain.RuleSourceModeFetchText,
		Enabled:    true,
		TimeoutSec: 15,
		Status:     domain.RefreshStatusSucceeded,
		Snapshot: domain.RuleSnapshot{
			IR: domain.RuleDocumentIR{
				Storage: domain.RuleStorageInlineText,
				Entries: []domain.RuleEntryIR{
					{Index: 0, Type: "RAW", Raw: "DOMAIN-SUFFIX,base.example,DIRECT"},
				},
			},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
	overrideRule := domain.RuleSource{
		ID:         "rule_override",
		Name:       "override-rule",
		URL:        "https://example.com/override.yaml",
		Mode:       domain.RuleSourceModeFetchText,
		Enabled:    true,
		TimeoutSec: 15,
		Status:     domain.RefreshStatusSucceeded,
		Snapshot: domain.RuleSnapshot{
			IR: domain.RuleDocumentIR{
				Storage: domain.RuleStorageInlineText,
				Entries: []domain.RuleEntryIR{
					{Index: 0, Type: "RAW", Raw: "DOMAIN-SUFFIX,override.example,DIRECT"},
				},
			},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
	if _, err := memStore.SaveRuleSource(baseRule); err != nil {
		t.Fatalf("SaveRuleSource(baseRule) error = %v", err)
	}
	if _, err := memStore.SaveRuleSource(overrideRule); err != nil {
		t.Fatalf("SaveRuleSource(overrideRule) error = %v", err)
	}

	profile, err := manager.CreateBuildProfile(ctx, CreateBuildProfileInput{
		Name:                  "base-profile",
		SubscriptionSourceIDs: []string{subscriptionHK.ID, subscriptionUS.ID},
		RuleBindings: []domain.BuildRuleBinding{
			{RuleSourceID: baseRule.ID, Mode: domain.BuildRuleBindingModeInline, Policy: "BaseGroup"},
		},
		Groups: []domain.ProxyGroupSpec{
			{Name: "BaseGroup", Type: domain.ProxyGroupTypeSelect, IncludeAll: true},
		},
		DefaultGroup: "BaseGroup",
		Enabled:      true,
	})
	if err != nil {
		t.Fatalf("CreateBuildProfile() error = %v", err)
	}

	tokenResult, err := manager.CreateDownloadToken(ctx, CreateDownloadTokenInput{
		Name:           "hk-only",
		BuildProfileID: profile.ID,
		Enabled:        true,
		Prebuild:       true,
		Distribution: domain.DownloadTokenDistribution{
			SubscriptionSourceIDs: []string{subscriptionHK.ID},
			IncludeProxyPatterns:  []string{"HK"},
			Filters: []domain.ProxyFilterRule{
				{Pattern: "VIP"},
			},
			Renames: []domain.RenameRule{
				{Pattern: "HK-01", Replace: "HK-EDGE-01"},
			},
			OverrideRuleBindings: true,
			RuleBindings: []domain.BuildRuleBinding{
				{RuleSourceID: overrideRule.ID, Mode: domain.BuildRuleBindingModeInline, Policy: "TokenAuto"},
			},
			OverrideGroups: true,
			Groups: []domain.ProxyGroupSpec{
				{Name: "TokenAuto", Type: domain.ProxyGroupTypeSelect, IncludeAll: true},
			},
			DefaultGroup: "TokenAuto",
			TemplateOverride: &domain.BuildTemplateOverride{
				MixedPort: intPtr(9900),
				Mode:      strPtr("global"),
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateDownloadToken() error = %v", err)
	}

	preview, err := manager.PreviewDownloadToken(ctx, tokenResult.Item.ID)
	if err != nil {
		t.Fatalf("PreviewDownloadToken() error = %v", err)
	}

	if got, want := len(preview.Build.Proxies), 1; got != want {
		t.Fatalf("len(preview.Build.Proxies) = %d, want %d", got, want)
	}
	if preview.Build.Proxies[0].Name != "HK-EDGE-01" {
		t.Fatalf("preview proxy name = %q, want %q", preview.Build.Proxies[0].Name, "HK-EDGE-01")
	}
	if strings.Contains(preview.Build.YAML, "US-01") {
		t.Fatalf("preview YAML unexpectedly contains source-filtered proxy US-01")
	}
	if strings.Contains(preview.Build.YAML, "HK-VIP") {
		t.Fatalf("preview YAML unexpectedly contains token-filtered proxy HK-VIP")
	}
	if !strings.Contains(preview.Build.YAML, "TokenAuto") {
		t.Fatalf("preview YAML does not contain overridden group TokenAuto")
	}
	if strings.Contains(preview.Build.YAML, "BaseGroup") {
		t.Fatalf("preview YAML unexpectedly contains inherited group BaseGroup")
	}
	if !strings.Contains(preview.Build.YAML, "mixed-port: 9900") {
		t.Fatalf("preview YAML does not contain overridden mixed-port")
	}
	if !strings.Contains(preview.Build.YAML, "mode: global") {
		t.Fatalf("preview YAML does not contain overridden mode")
	}
	if !containsString(preview.Build.Rules, "DOMAIN-SUFFIX,override.example,TokenAuto") {
		t.Fatalf("preview rules = %#v, want override rule bound to TokenAuto", preview.Build.Rules)
	}
	if !containsString(preview.Diff.RemovedProxyNames, "US-01") {
		t.Fatalf("preview diff removed proxies = %#v, want US-01", preview.Diff.RemovedProxyNames)
	}
	if !containsString(preview.Diff.AddedProxyNames, "HK-EDGE-01") {
		t.Fatalf("preview diff added proxies = %#v, want HK-EDGE-01", preview.Diff.AddedProxyNames)
	}
	if got := len(preview.ProxyMatches); got != 2 {
		t.Fatalf("len(preview.ProxyMatches) = %d, want 2", got)
	}

	artifact, err := manager.ResolveDownloadArtifact(ctx, tokenResult.Token, "")
	if err != nil {
		t.Fatalf("ResolveDownloadArtifact() error = %v", err)
	}
	if artifact.FileName != "hk-only.yaml" {
		t.Fatalf("artifact.FileName = %q, want %q", artifact.FileName, "hk-only.yaml")
	}
	if !strings.Contains(artifact.Content, "HK-EDGE-01") || strings.Contains(artifact.Content, "US-01") {
		t.Fatalf("artifact content does not reflect token distribution")
	}

	tokenMeta, err := manager.GetDownloadToken(ctx, tokenResult.Item.ID)
	if err != nil {
		t.Fatalf("GetDownloadToken() error = %v", err)
	}
	if tokenMeta.CachedArtifact == nil {
		t.Fatalf("token cached artifact = nil, want prebuilt cache metadata")
	}
	if tokenMeta.CachedArtifact.ETag == "" {
		t.Fatalf("token cached artifact ETag is empty")
	}
}

func containsString(list []string, target string) bool {
	for _, item := range list {
		if item == target {
			return true
		}
	}
	return false
}

func intPtr(value int) *int {
	return &value
}

func strPtr(value string) *string {
	return &value
}
