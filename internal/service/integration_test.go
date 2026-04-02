package service_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"submanager/internal/domain"
	"submanager/internal/fetcher"
	"submanager/internal/parser"
	"submanager/internal/service"
	"submanager/internal/store"
)

// ─── helpers ─────────────────────────────────────────────────────────────────

func newTestManager(t *testing.T) (*service.Manager, *store.MemoryStore) {
	t.Helper()
	mem := store.NewMemoryStore()
	mgr := service.NewManager(mem, fetcher.NewHTTPFetcher())
	return mgr, mem
}

func seedSubscription(t *testing.T, mem *store.MemoryStore, id, name string, proxyYAML string) domain.SubscriptionSource {
	t.Helper()
	parsed, err := parser.ParseClashMetaSubscription([]byte(proxyYAML), id, "https://example.com/"+id)
	if err != nil {
		t.Fatalf("ParseClashMetaSubscription(%s) error = %v", id, err)
	}
	now := time.Now().UTC()
	src := domain.SubscriptionSource{
		ID:      id,
		Name:    name,
		URL:     "https://example.com/" + id + ".yaml",
		Enabled: true,
		Status:  domain.RefreshStatusSucceeded,
		Snapshot: domain.SubscriptionSnapshot{
			RawProxies: parsed.RawProxies,
			Proxies:    parsed.Proxies,
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
	if _, err := mem.SaveSubscription(src); err != nil {
		t.Fatalf("SaveSubscription(%s) error = %v", id, err)
	}
	return src
}

func seedRule(t *testing.T, mem *store.MemoryStore, id, name string) domain.RuleSource {
	t.Helper()
	now := time.Now().UTC()
	rule := domain.RuleSource{
		ID:      id,
		Name:    name,
		URL:     "https://example.com/" + id + ".yaml",
		Mode:    domain.RuleSourceModeFetchText,
		Enabled: true,
		Status:  domain.RefreshStatusSucceeded,
		Snapshot: domain.RuleSnapshot{
			IR: domain.RuleDocumentIR{
				Storage: domain.RuleStorageInlineText,
				Entries: []domain.RuleEntryIR{
					{Index: 0, Type: "RAW", Raw: "DOMAIN-SUFFIX," + id + ".example,DIRECT"},
				},
			},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
	if _, err := mem.SaveRuleSource(rule); err != nil {
		t.Fatalf("SaveRuleSource(%s) error = %v", id, err)
	}
	return rule
}

const simpleProxy = `
proxies:
  - name: HK-01
    type: trojan
    server: hk.example.com
    port: 443
    password: hk-pass
    sni: hk.example.com
    udp: true
  - name: US-01
    type: trojan
    server: us.example.com
    port: 443
    password: us-pass
    sni: us.example.com
    udp: true
`

// ─── CRUD: Build Profile ──────────────────────────────────────────────────────

func TestBuildProfile_CreateListGetUpdate(t *testing.T) {
	ctx := context.Background()
	mgr, mem := newTestManager(t)
	sub := seedSubscription(t, mem, "sub1", "Sub1", simpleProxy)

	// Create
	profile, err := mgr.CreateBuildProfile(ctx, service.CreateBuildProfileInput{
		Name:                  "test-profile",
		SubscriptionSourceIDs: []string{sub.ID},
		Enabled:               true,
	})
	if err != nil {
		t.Fatalf("CreateBuildProfile error = %v", err)
	}
	if profile.ID == "" {
		t.Fatal("profile.ID is empty")
	}

	// List
	list, err := mgr.ListBuildProfiles(ctx)
	if err != nil {
		t.Fatalf("ListBuildProfiles error = %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("len(profiles) = %d, want 1", len(list))
	}

	// Get
	got, err := mgr.GetBuildProfile(ctx, profile.ID)
	if err != nil {
		t.Fatalf("GetBuildProfile error = %v", err)
	}
	if got.Name != "test-profile" {
		t.Errorf("profile.Name = %q, want %q", got.Name, "test-profile")
	}

	// Update name
	newName := "updated-profile"
	updated, err := mgr.UpdateBuildProfile(ctx, profile.ID, service.UpdateBuildProfileInput{
		Name: &newName,
	})
	if err != nil {
		t.Fatalf("UpdateBuildProfile error = %v", err)
	}
	if updated.Name != newName {
		t.Errorf("updated.Name = %q, want %q", updated.Name, newName)
	}
}

// ─── Build Profile: PreviewBuild ─────────────────────────────────────────────

func TestBuildProfile_Preview(t *testing.T) {
	ctx := context.Background()
	mgr, mem := newTestManager(t)
	sub := seedSubscription(t, mem, "sub1", "Sub1", simpleProxy)
	rule := seedRule(t, mem, "rule1", "Rule1")

	profile, err := mgr.CreateBuildProfile(ctx, service.CreateBuildProfileInput{
		Name:                  "preview-profile",
		SubscriptionSourceIDs: []string{sub.ID},
		RuleBindings: []domain.BuildRuleBinding{
			{RuleSourceID: rule.ID, Mode: domain.BuildRuleBindingModeInline, Policy: "Proxy"},
		},
		Groups: []domain.ProxyGroupSpec{
			{Name: "Proxy", Type: domain.ProxyGroupTypeSelect, IncludeAll: true},
		},
		DefaultGroup: "Proxy",
		Enabled:      true,
	})
	if err != nil {
		t.Fatalf("CreateBuildProfile error = %v", err)
	}

	preview, err := mgr.PreviewBuildProfile(ctx, profile.ID)
	if err != nil {
		t.Fatalf("PreviewBuildProfile error = %v", err)
	}
	if len(preview.Proxies) != 2 {
		t.Errorf("preview.Proxies count = %d, want 2", len(preview.Proxies))
	}
	if !strings.Contains(preview.YAML, "HK-01") {
		t.Error("preview YAML missing HK-01")
	}
	if !strings.Contains(preview.YAML, "Proxy") {
		t.Error("preview YAML missing group Proxy")
	}
	if !strings.Contains(preview.YAML, "rule1.example") {
		t.Error("preview YAML missing rule entry")
	}
}

// ─── Delete: Subscription triggers alert ─────────────────────────────────────

func TestDelete_Subscription_CreatesSystemAlert(t *testing.T) {
	ctx := context.Background()
	mgr, mem := newTestManager(t)
	sub := seedSubscription(t, mem, "sub1", "Sub1", simpleProxy)

	_, err := mgr.CreateBuildProfile(ctx, service.CreateBuildProfileInput{
		Name:                  "dep-profile",
		SubscriptionSourceIDs: []string{sub.ID},
		Enabled:               true,
	})
	if err != nil {
		t.Fatalf("CreateBuildProfile error = %v", err)
	}

	// Delete the subscription that the profile depends on
	if err := mgr.DeleteSubscriptionSource(ctx, sub.ID); err != nil {
		t.Fatalf("DeleteSubscriptionSource error = %v", err)
	}

	// Verify subscription is gone
	subs, _ := mgr.ListSubscriptionSources(ctx)
	for _, s := range subs {
		if s.ID == sub.ID {
			t.Error("subscription still present after deletion")
		}
	}

	// Verify system alert was created
	alerts, err := mgr.ListSystemAlerts(ctx)
	if err != nil {
		t.Fatalf("ListSystemAlerts error = %v", err)
	}
	if len(alerts) == 0 {
		t.Fatal("expected at least 1 system alert after deleting a used subscription, got 0")
	}
	if alerts[0].Level != domain.AlertLevelError {
		t.Errorf("alert.Level = %q, want %q", alerts[0].Level, domain.AlertLevelError)
	}
	if !strings.Contains(alerts[0].Message, sub.ID) {
		t.Errorf("alert.Message = %q, does not mention sub.ID %q", alerts[0].Message, sub.ID)
	}
}

// ─── Delete: Rule triggers alert ─────────────────────────────────────────────

func TestDelete_Rule_CreatesSystemAlert(t *testing.T) {
	ctx := context.Background()
	mgr, mem := newTestManager(t)
	sub := seedSubscription(t, mem, "sub1", "Sub1", simpleProxy)
	rule := seedRule(t, mem, "rule1", "Rule1")

	_, err := mgr.CreateBuildProfile(ctx, service.CreateBuildProfileInput{
		Name:                  "dep-profile",
		SubscriptionSourceIDs: []string{sub.ID},
		RuleBindings:          []domain.BuildRuleBinding{{RuleSourceID: rule.ID, Policy: "Proxy", Mode: domain.BuildRuleBindingModeInline}},
		Enabled:               true,
	})
	if err != nil {
		t.Fatalf("CreateBuildProfile error = %v", err)
	}

	if err := mgr.DeleteRuleSource(ctx, rule.ID); err != nil {
		t.Fatalf("DeleteRuleSource error = %v", err)
	}

	alerts, err := mgr.ListSystemAlerts(ctx)
	if err != nil {
		t.Fatalf("ListSystemAlerts error = %v", err)
	}
	if len(alerts) == 0 {
		t.Fatal("expected system alert after deleting a rule used by a profile")
	}
	if !strings.Contains(alerts[0].Message, rule.ID) {
		t.Errorf("alert does not mention deleted rule id %q, got: %q", rule.ID, alerts[0].Message)
	}
}

// ─── Delete: BuildProfile triggers alert for tokens ──────────────────────────

func TestDelete_BuildProfile_CreatesAlertForDependentToken(t *testing.T) {
	ctx := context.Background()
	mgr, mem := newTestManager(t)
	sub := seedSubscription(t, mem, "sub1", "Sub1", simpleProxy)

	profile, err := mgr.CreateBuildProfile(ctx, service.CreateBuildProfileInput{
		Name:                  "my-profile",
		SubscriptionSourceIDs: []string{sub.ID},
		Enabled:               true,
	})
	if err != nil {
		t.Fatalf("CreateBuildProfile error = %v", err)
	}

	tokenResult, err := mgr.CreateDownloadToken(ctx, service.CreateDownloadTokenInput{
		Name:           "my-token",
		BuildProfileID: profile.ID,
		Enabled:        true,
	})
	if err != nil {
		t.Fatalf("CreateDownloadToken error = %v", err)
	}

	// Delete the profile
	if err := mgr.DeleteBuildProfile(ctx, profile.ID); err != nil {
		t.Fatalf("DeleteBuildProfile error = %v", err)
	}

	// Profile must be gone
	profiles, _ := mgr.ListBuildProfiles(ctx)
	for _, p := range profiles {
		if p.ID == profile.ID {
			t.Error("build profile still present after deletion")
		}
	}

	// Token must still exist (soft-dependency model)
	_, err = mgr.GetDownloadToken(ctx, tokenResult.Item.ID)
	if err != nil {
		t.Errorf("GetDownloadToken after profile delete error = %v (token should still exist)", err)
	}

	// System alert must be recorded
	alerts, err := mgr.ListSystemAlerts(ctx)
	if err != nil {
		t.Fatalf("ListSystemAlerts error = %v", err)
	}
	if len(alerts) == 0 {
		t.Fatal("expected system alert after deleting a profile used by a token")
	}
	if !strings.Contains(alerts[0].Message, "my-token") {
		t.Errorf("alert missing token name, got: %q", alerts[0].Message)
	}
}

// ─── Delete: Token ────────────────────────────────────────────────────────────

func TestDelete_DownloadToken(t *testing.T) {
	ctx := context.Background()
	mgr, mem := newTestManager(t)
	sub := seedSubscription(t, mem, "sub1", "Sub1", simpleProxy)

	profile, _ := mgr.CreateBuildProfile(ctx, service.CreateBuildProfileInput{
		Name:                  "profile",
		SubscriptionSourceIDs: []string{sub.ID},
		Enabled:               true,
	})
	tokenResult, err := mgr.CreateDownloadToken(ctx, service.CreateDownloadTokenInput{
		Name:           "delme",
		BuildProfileID: profile.ID,
		Enabled:        true,
	})
	if err != nil {
		t.Fatalf("CreateDownloadToken error = %v", err)
	}

	if err := mgr.DeleteDownloadToken(ctx, tokenResult.Item.ID); err != nil {
		t.Fatalf("DeleteDownloadToken error = %v", err)
	}

	// Token should be gone
	tokens, _ := mgr.ListDownloadTokens(ctx)
	for _, tok := range tokens {
		if tok.ID == tokenResult.Item.ID {
			t.Error("token still present after deletion")
		}
	}

	// No alerts expected when deleting a token directly
	alerts, _ := mgr.ListSystemAlerts(ctx)
	if len(alerts) != 0 {
		t.Errorf("expected 0 alerts after token delete, got %d", len(alerts))
	}
}

// ─── SystemAlerts: Clear ─────────────────────────────────────────────────────

func TestSystemAlerts_ClearAll(t *testing.T) {
	ctx := context.Background()
	mgr, mem := newTestManager(t)
	sub := seedSubscription(t, mem, "sub1", "Sub1", simpleProxy)

	// Create a profile that references sub1, then delete sub1 → 1 alert
	_, _ = mgr.CreateBuildProfile(ctx, service.CreateBuildProfileInput{
		Name:                  "p",
		SubscriptionSourceIDs: []string{sub.ID},
		Enabled:               true,
	})
	_ = mgr.DeleteSubscriptionSource(ctx, sub.ID)

	alerts, _ := mgr.ListSystemAlerts(ctx)
	if len(alerts) == 0 {
		t.Fatal("expected alerts before clear, got 0")
	}

	if err := mgr.ClearSystemAlerts(ctx); err != nil {
		t.Fatalf("ClearSystemAlerts error = %v", err)
	}

	alerts, _ = mgr.ListSystemAlerts(ctx)
	if len(alerts) != 0 {
		t.Errorf("expected 0 alerts after clear, got %d", len(alerts))
	}
}

// ─── Delete: Profile cleans up BuildRuns & Artifacts ─────────────────────────

func TestDelete_BuildProfile_CleansRunsAndArtifacts(t *testing.T) {
	ctx := context.Background()
	mgr, mem := newTestManager(t)
	sub := seedSubscription(t, mem, "sub1", "Sub1", simpleProxy)

	profile, err := mgr.CreateBuildProfile(ctx, service.CreateBuildProfileInput{
		Name:                  "cleanup-profile",
		SubscriptionSourceIDs: []string{sub.ID},
		Enabled:               true,
	})
	if err != nil {
		t.Fatalf("CreateBuildProfile error = %v", err)
	}

	// Manually inject a BuildRun and BuildArtifact via the memory store
	now := time.Now().UTC()
	run := domain.BuildRun{
		ID:        "run_test",
		ProfileID: profile.ID,
		Status:    domain.BuildRunStatusSucceeded,
		CreatedAt: now,
	}
	if _, err := mem.SaveBuildRun(run); err != nil {
		t.Fatalf("SaveBuildRun error = %v", err)
	}
	artifact := domain.BuildArtifact{
		ID:        "art_test",
		ProfileID: profile.ID,
		RunID:     run.ID,
		FileName:  "out.yaml",
		Content:   "yaml content",
		CreatedAt: now,
	}
	if _, err := mem.SaveBuildArtifact(artifact); err != nil {
		t.Fatalf("SaveBuildArtifact error = %v", err)
	}

	// Delete profile
	if err := mgr.DeleteBuildProfile(ctx, profile.ID); err != nil {
		t.Fatalf("DeleteBuildProfile error = %v", err)
	}

	// Run and artifact should be gone via MemoryStore cleanup
	if _, err := mem.GetBuildRun("run_test"); err == nil {
		t.Error("BuildRun still present after profile deletion")
	}
	if _, err := mem.GetBuildArtifact("art_test"); err == nil {
		t.Error("BuildArtifact still present after profile deletion")
	}
}

// ─── Subscription: Create / List / Update ────────────────────────────────────

func TestSubscription_CreateListUpdate(t *testing.T) {
	ctx := context.Background()
	mgr, _ := newTestManager(t)

	// Create
	src, err := mgr.CreateSubscriptionSource(ctx, service.CreateSubscriptionInput{
		Name:       "my-sub",
		Type:       "remote",
		URL:        "https://example.com/sub.yaml",
		Enabled:    true,
		TimeoutSec: 15,
	})
	if err != nil {
		t.Fatalf("CreateSubscriptionSource error = %v", err)
	}
	if src.ID == "" {
		t.Fatal("subscription.ID is empty")
	}

	// List
	list, _ := mgr.ListSubscriptionSources(ctx)
	if len(list) != 1 {
		t.Fatalf("len(subs) = %d, want 1", len(list))
	}

	// Update
	newName := "renamed-sub"
	updated, err := mgr.UpdateSubscriptionSource(ctx, src.ID, service.UpdateSubscriptionInput{
		Name: &newName,
	})
	if err != nil {
		t.Fatalf("UpdateSubscriptionSource error = %v", err)
	}
	if updated.Name != newName {
		t.Errorf("updated.Name = %q, want %q", updated.Name, newName)
	}
}

// ─── Rule Source: Create / List / Update ─────────────────────────────────────

func TestRuleSource_CreateListUpdate(t *testing.T) {
	ctx := context.Background()
	mgr, _ := newTestManager(t)

	rs, err := mgr.CreateRuleSource(ctx, service.CreateRuleSourceInput{
		Name:    "my-rule",
		URL:     "https://example.com/rule.yaml",
		Mode:    domain.RuleSourceModeLinkOnly,
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("CreateRuleSource error = %v", err)
	}

	list, _ := mgr.ListRuleSources(ctx)
	if len(list) != 1 {
		t.Fatalf("len(rules) = %d, want 1", len(list))
	}

	newURL := "https://example.com/rule-v2.yaml"
	updated, err := mgr.UpdateRuleSource(ctx, rs.ID, service.UpdateRuleSourceInput{
		URL: &newURL,
	})
	if err != nil {
		t.Fatalf("UpdateRuleSource error = %v", err)
	}
	if updated.URL != newURL {
		t.Errorf("updated.URL = %q, want %q", updated.URL, newURL)
	}
}

// ─── Download Token: Create → Resolve round-trip ─────────────────────────────

func TestDownloadToken_CreateResolveRoundTrip(t *testing.T) {
	ctx := context.Background()
	mgr, mem := newTestManager(t)
	sub := seedSubscription(t, mem, "sub1", "Sub1", simpleProxy)

	profile, _ := mgr.CreateBuildProfile(ctx, service.CreateBuildProfileInput{
		Name:                  "profile",
		SubscriptionSourceIDs: []string{sub.ID},
		Enabled:               true,
	})

	result, err := mgr.CreateDownloadToken(ctx, service.CreateDownloadTokenInput{
		Name:           "mytoken",
		BuildProfileID: profile.ID,
		Enabled:        true,
	})
	if err != nil {
		t.Fatalf("CreateDownloadToken error = %v", err)
	}
	if result.Token == "" {
		t.Fatal("result.Token is empty — plain token not returned on creation")
	}
	if result.Item.ID == "" {
		t.Fatal("result.Item.ID is empty")
	}

	// Resolve back from the plain token
	artifact, err := mgr.ResolveDownloadArtifact(ctx, result.Token, "127.0.0.1")
	if err != nil {
		t.Fatalf("ResolveDownloadArtifact error = %v", err)
	}
	if !strings.Contains(artifact.Content, "HK-01") {
		t.Error("resolved artifact YAML missing HK-01")
	}
	if artifact.FileName == "" {
		t.Error("artifact.FileName is empty")
	}
}

// ─── Download Token: disabled token denied ────────────────────────────────────

func TestDownloadToken_DisabledDenied(t *testing.T) {
	ctx := context.Background()
	mgr, mem := newTestManager(t)
	sub := seedSubscription(t, mem, "sub1", "Sub1", simpleProxy)

	profile, _ := mgr.CreateBuildProfile(ctx, service.CreateBuildProfileInput{
		Name:                  "profile",
		SubscriptionSourceIDs: []string{sub.ID},
		Enabled:               true,
	})

	result, err := mgr.CreateDownloadToken(ctx, service.CreateDownloadTokenInput{
		Name:           "disabled-token",
		BuildProfileID: profile.ID,
		Enabled:        false,
	})
	if err != nil {
		t.Fatalf("CreateDownloadToken error = %v", err)
	}

	_, err = mgr.ResolveDownloadArtifact(ctx, result.Token, "127.0.0.1")
	if err == nil {
		t.Fatal("expected error resolving disabled token, got nil")
	}
}

// ─── Download Token: unknown token not found ──────────────────────────────────

func TestDownloadToken_UnknownTokenNotFound(t *testing.T) {
	ctx := context.Background()
	mgr, _ := newTestManager(t)

	_, err := mgr.ResolveDownloadArtifact(ctx, "totally-invalid-token-value", "")
	if err == nil {
		t.Fatal("expected error for unknown token, got nil")
	}
}

// ─── SQLite Store: Delete cascade uses correct JSON field ────────────────────

func TestSQLiteStore_DeleteBuildRunsByProfile(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "test.db")
	sqliteStore, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore error = %v", err)
	}
	defer sqliteStore.Close()

	now := time.Now().UTC()
	profileID := "profile_test"

	run := domain.BuildRun{
		ID:        "run1",
		ProfileID: profileID, // json: "profile_id"
		Status:    domain.BuildRunStatusSucceeded,
		CreatedAt: now,
	}
	if _, err := sqliteStore.SaveBuildRun(run); err != nil {
		t.Fatalf("SaveBuildRun error = %v", err)
	}

	artifact := domain.BuildArtifact{
		ID:        "art1",
		ProfileID: profileID, // json: "profile_id"
		RunID:     "run1",
		FileName:  "out.yaml",
		Content:   "yml",
		CreatedAt: now,
	}
	if _, err := sqliteStore.SaveBuildArtifact(artifact); err != nil {
		t.Fatalf("SaveBuildArtifact error = %v", err)
	}

	// Delete by profile ID
	if err := sqliteStore.DeleteBuildRunsByProfile(profileID); err != nil {
		t.Fatalf("DeleteBuildRunsByProfile error = %v", err)
	}
	if err := sqliteStore.DeleteBuildArtifactsByProfile(profileID); err != nil {
		t.Fatalf("DeleteBuildArtifactsByProfile error = %v", err)
	}

	// Verify cascades worked
	if _, err := sqliteStore.GetBuildRun("run1"); err == nil {
		t.Error("BuildRun still present after DeleteBuildRunsByProfile — json_extract field name mismatch?")
	}
	if _, err := sqliteStore.GetBuildArtifact("art1"); err == nil {
		t.Error("BuildArtifact still present after DeleteBuildArtifactsByProfile — json_extract field name mismatch?")
	}
}

// ─── SQLite Store: SystemAlert save/list/clear ───────────────────────────────

func TestSQLiteStore_SystemAlerts(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "test.db")
	sqliteStore, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore error = %v", err)
	}
	defer sqliteStore.Close()
	_ = os.Remove(dbPath) // ensure fresh on next test

	now := time.Now().UTC()
	alert := domain.SystemAlert{
		ID:        "alrt_1",
		Level:     domain.AlertLevelError,
		Message:   "test alert message",
		CreatedAt: now,
	}
	if _, err := sqliteStore.SaveSystemAlert(alert); err != nil {
		t.Fatalf("SaveSystemAlert error = %v", err)
	}

	list, err := sqliteStore.ListSystemAlerts()
	if err != nil {
		t.Fatalf("ListSystemAlerts error = %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("len(alerts) = %d, want 1", len(list))
	}
	if list[0].Message != "test alert message" {
		t.Errorf("alert.Message = %q", list[0].Message)
	}

	if err := sqliteStore.ClearSystemAlerts(); err != nil {
		t.Fatalf("ClearSystemAlerts error = %v", err)
	}
	list, _ = sqliteStore.ListSystemAlerts()
	if len(list) != 0 {
		t.Errorf("expected 0 alerts after clear, got %d", len(list))
	}
}
