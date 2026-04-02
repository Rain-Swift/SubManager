package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	buildengine "submanager/internal/build"
	"submanager/internal/domain"
	"submanager/internal/store"
)

type CreateDownloadTokenInput struct {
	Name           string
	BuildProfileID string
	Distribution   domain.DownloadTokenDistribution
	Prebuild       bool
	Enabled        bool
}

type UpdateDownloadTokenInput struct {
	Name         *string
	Distribution *domain.DownloadTokenDistribution
	Prebuild     *bool
	Enabled      *bool
}

type CreateDownloadTokenResult struct {
	Token string               `json:"token"`
	Item  domain.DownloadToken `json:"item"`
}

type preparedDownloadTokenBuild struct {
	record           domain.DownloadTokenRecord
	baseProfile      domain.BuildProfile
	effectiveProfile domain.BuildProfile
	subscriptions    []domain.SubscriptionSource
	ruleSources      []domain.RuleSource
	preview          domain.BuildPreview
	signature        string
	proxyMatches     []domain.DownloadTokenProxyMatch
}

func (m *Manager) CreateDownloadToken(ctx context.Context, input CreateDownloadTokenInput) (CreateDownloadTokenResult, error) {
	if strings.TrimSpace(input.BuildProfileID) == "" {
		return CreateDownloadTokenResult{}, fmt.Errorf("%w: build_profile_id is required", ErrInvalidInput)
	}

	profile, err := m.store.GetBuildProfile(input.BuildProfileID)
	if err != nil {
		return CreateDownloadTokenResult{}, err
	}
	if err := m.validateDownloadTokenDistribution(profile, input.Distribution); err != nil {
		return CreateDownloadTokenResult{}, err
	}

	plainToken, err := newDownloadTokenValue()
	if err != nil {
		return CreateDownloadTokenResult{}, fmt.Errorf("generate download token: %w", err)
	}

	now := time.Now().UTC()
	record := domain.DownloadTokenRecord{
		ID:             newID("token"),
		Name:           firstNonEmptyString(strings.TrimSpace(input.Name), strings.TrimSpace(profile.Name)),
		BuildProfileID: profile.ID,
		TokenHash:      hashDownloadToken(plainToken),
		TokenPrefix:    downloadTokenPrefix(plainToken),
		Distribution:   input.Distribution.Clone(),
		Prebuild:       input.Prebuild,
		Enabled:        input.Enabled,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	saved, err := m.store.SaveDownloadToken(record)
	if err != nil {
		return CreateDownloadTokenResult{}, err
	}
	if saved.Prebuild {
		m.dispatchWork(workItem{kind: workDownloadTokenWarm, id: saved.ID})
	}

	return CreateDownloadTokenResult{
		Token: plainToken,
		Item:  saved.Public(),
	}, nil
}

func (m *Manager) ListDownloadTokens(_ context.Context) ([]domain.DownloadToken, error) {
	items, err := m.store.ListDownloadTokens()
	if err != nil {
		return nil, err
	}

	out := make([]domain.DownloadToken, 0, len(items))
	for _, item := range items {
		out = append(out, item.Public())
	}
	return out, nil
}

func (m *Manager) GetDownloadToken(_ context.Context, id string) (domain.DownloadToken, error) {
	item, err := m.store.GetDownloadToken(id)
	if err != nil {
		return domain.DownloadToken{}, err
	}
	return item.Public(), nil
}

func (m *Manager) UpdateDownloadToken(_ context.Context, id string, input UpdateDownloadTokenInput) (domain.DownloadToken, error) {
	item, err := m.store.GetDownloadToken(id)
	if err != nil {
		return domain.DownloadToken{}, err
	}

	if input.Name != nil {
		name := strings.TrimSpace(*input.Name)
		if name == "" {
			return domain.DownloadToken{}, fmt.Errorf("%w: token name is required", ErrInvalidInput)
		}
		item.Name = name
	}
	if input.Enabled != nil {
		item.Enabled = *input.Enabled
	}
	if input.Prebuild != nil {
		item.Prebuild = *input.Prebuild
	}
	if input.Distribution != nil {
		profile, err := m.store.GetBuildProfile(item.BuildProfileID)
		if err != nil {
			return domain.DownloadToken{}, err
		}
		if err := m.validateDownloadTokenDistribution(profile, *input.Distribution); err != nil {
			return domain.DownloadToken{}, err
		}
		item.Distribution = input.Distribution.Clone()
		item.CachedArtifact = nil
	}

	item.UpdatedAt = time.Now().UTC()
	saved, err := m.store.SaveDownloadToken(item)
	if err != nil {
		return domain.DownloadToken{}, err
	}
	if saved.Prebuild && saved.Enabled {
		m.dispatchWork(workItem{kind: workDownloadTokenWarm, id: saved.ID})
	}
	return saved.Public(), nil
}

func (m *Manager) SetDownloadTokenEnabled(ctx context.Context, id string, enabled bool) (domain.DownloadToken, error) {
	return m.UpdateDownloadToken(ctx, id, UpdateDownloadTokenInput{
		Enabled: &enabled,
	})
}

func (m *Manager) PreviewDownloadToken(_ context.Context, id string) (domain.DownloadTokenPreview, error) {
	record, err := m.store.GetDownloadToken(id)
	if err != nil {
		return domain.DownloadTokenPreview{}, err
	}

	prepared, err := m.prepareDownloadTokenBuild(record)
	if err != nil {
		return domain.DownloadTokenPreview{}, err
	}

	baseSubscriptions, baseRuleSources, err := m.loadBuildInputs(prepared.baseProfile)
	if err != nil {
		return domain.DownloadTokenPreview{}, err
	}
	basePreview, err := buildengine.Execute(prepared.baseProfile, baseSubscriptions, baseRuleSources)
	if err != nil {
		return domain.DownloadTokenPreview{}, err
	}

	return domain.DownloadTokenPreview{
		TokenID:      record.ID,
		ProfileID:    record.BuildProfileID,
		Build:        prepared.preview,
		Diff:         computeDownloadTokenDiff(basePreview, prepared.preview),
		ProxyMatches: prepared.proxyMatches,
	}, nil
}

func (m *Manager) ResolveDownloadArtifact(_ context.Context, plainToken string, clientIP string) (domain.BuildArtifact, error) {
	plainToken = strings.TrimSpace(plainToken)
	if plainToken == "" {
		return domain.BuildArtifact{}, store.ErrNotFound
	}

	record, err := m.store.FindDownloadTokenByHash(hashDownloadToken(plainToken))
	if err != nil {
		return domain.BuildArtifact{}, err
	}
	if !record.Enabled {
		return domain.BuildArtifact{}, fmt.Errorf("%w: download token %q", ErrAccessDenied, record.Name)
	}

	record, artifact, err := m.resolveDownloadTokenArtifact(record)
	if err != nil {
		return domain.BuildArtifact{}, err
	}

	now := time.Now().UTC()
	record.LastUsedAt = &now
	record.FetchCount++

	// Append to access log, keep last 50 entries
	record.AccessLog = append(record.AccessLog, domain.TokenAccessLog{
		At: now,
		IP: strings.TrimSpace(clientIP),
	})
	const maxAccessLogEntries = 50
	if len(record.AccessLog) > maxAccessLogEntries {
		record.AccessLog = record.AccessLog[len(record.AccessLog)-maxAccessLogEntries:]
	}

	_, _ = m.store.SaveDownloadToken(record)
	return artifact, nil
}

func (m *Manager) buildDownloadTokenCache(_ context.Context, id string) (domain.DownloadTokenRecord, error) {
	record, err := m.store.GetDownloadToken(id)
	if err != nil {
		return domain.DownloadTokenRecord{}, err
	}
	record, _, err = m.resolveDownloadTokenArtifact(record)
	if err != nil {
		return domain.DownloadTokenRecord{}, err
	}
	return record, nil
}

func (m *Manager) resolveDownloadTokenArtifact(record domain.DownloadTokenRecord) (domain.DownloadTokenRecord, domain.BuildArtifact, error) {
	prepared, err := m.prepareDownloadTokenBuild(record)
	if err != nil {
		return domain.DownloadTokenRecord{}, domain.BuildArtifact{}, err
	}

	if cache := record.CachedArtifact; cache != nil && cache.Signature == prepared.signature {
		return record, buildArtifactFromTokenCache(record, *cache), nil
	}

	artifact := domain.BuildArtifact{
		ID:        "",
		ProfileID: prepared.effectiveProfile.ID,
		RunID:     "",
		FileName:  buildArtifactFileName(firstNonEmptyString(record.Name, prepared.effectiveProfile.Name)),
		Content:   prepared.preview.YAML,
		SHA256:    hashText(prepared.preview.YAML),
		Summary:   prepared.preview.Summary,
		CreatedAt: time.Now().UTC(),
	}
	record.CachedArtifact = &domain.DownloadTokenArtifactCache{
		Signature:    prepared.signature,
		FileName:     artifact.FileName,
		Content:      artifact.Content,
		SHA256:       artifact.SHA256,
		ETag:         weakETag(artifact.SHA256),
		Summary:      artifact.Summary,
		LastBuiltAt:  artifact.CreatedAt,
		LastModified: artifact.CreatedAt,
	}
	_, _ = m.store.SaveDownloadToken(record)
	return record, artifact, nil
}

func (m *Manager) prepareDownloadTokenBuild(record domain.DownloadTokenRecord) (preparedDownloadTokenBuild, error) {
	profile, err := m.store.GetBuildProfile(record.BuildProfileID)
	if err != nil {
		return preparedDownloadTokenBuild{}, err
	}
	if !profile.Enabled {
		return preparedDownloadTokenBuild{}, fmt.Errorf("%w: build profile %q", ErrAccessDenied, profile.Name)
	}

	effectiveProfile := applyDownloadTokenDistribution(profile, record.Distribution)
	subscriptions, ruleSources, err := m.loadBuildInputs(effectiveProfile)
	if err != nil {
		return preparedDownloadTokenBuild{}, err
	}

	filteredSubscriptions, proxyMatches, err := filterSubscriptionsForDistribution(subscriptions, record.Distribution)
	if err != nil {
		return preparedDownloadTokenBuild{}, err
	}

	preview, err := buildengine.Execute(effectiveProfile, filteredSubscriptions, ruleSources)
	if err != nil {
		return preparedDownloadTokenBuild{}, err
	}

	signature, err := computeDownloadTokenSignature(record, effectiveProfile, filteredSubscriptions, ruleSources)
	if err != nil {
		return preparedDownloadTokenBuild{}, err
	}

	return preparedDownloadTokenBuild{
		record:           record,
		baseProfile:      profile,
		effectiveProfile: effectiveProfile,
		subscriptions:    filteredSubscriptions,
		ruleSources:      ruleSources,
		preview:          preview,
		signature:        signature,
		proxyMatches:     proxyMatches,
	}, nil
}

func applyDownloadTokenDistribution(profile domain.BuildProfile, distribution domain.DownloadTokenDistribution) domain.BuildProfile {
	out := profile.Clone()
	if len(distribution.SubscriptionSourceIDs) > 0 {
		out.SubscriptionSourceIDs = copyStringSlice(distribution.SubscriptionSourceIDs)
	}
	if distribution.OverrideRuleBindings || len(distribution.RuleBindings) > 0 {
		out.RuleBindings = cloneRuleBindings(distribution.RuleBindings)
	}
	if distribution.OverrideGroups || len(distribution.Groups) > 0 {
		out.Groups = cloneGroupSpecs(distribution.Groups)
	}
	if len(distribution.Filters) > 0 {
		out.Filters = append(out.Filters, cloneFilterRules(distribution.Filters)...)
	}
	if len(distribution.Renames) > 0 {
		out.Renames = append(out.Renames, cloneRenameRules(distribution.Renames)...)
	}
	if value := strings.TrimSpace(distribution.DefaultGroup); value != "" {
		out.DefaultGroup = value
	}
	if distribution.TemplateOverride != nil {
		out.Template = applyTemplateOverride(out.Template, *distribution.TemplateOverride)
	}
	return out
}

func filterSubscriptionsForDistribution(subscriptions []domain.SubscriptionSource, distribution domain.DownloadTokenDistribution) ([]domain.SubscriptionSource, []domain.DownloadTokenProxyMatch, error) {
	include, err := compileRegexes(distribution.IncludeProxyPatterns)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: include_proxy_patterns: %v", ErrInvalidInput, err)
	}
	exclude, err := compileRegexes(distribution.ExcludeProxyPatterns)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: exclude_proxy_patterns: %v", ErrInvalidInput, err)
	}
	if len(include) == 0 && len(exclude) == 0 {
		return subscriptions, nil, nil
	}

	out := make([]domain.SubscriptionSource, 0, len(subscriptions))
	matches := make([]domain.DownloadTokenProxyMatch, 0)
	for _, source := range subscriptions {
		cloned := source.Clone()
		selectedIndexes := make(map[int]struct{})
		filteredProxies := make([]domain.ProxyIR, 0, len(cloned.Snapshot.Proxies))
		for _, proxy := range cloned.Snapshot.Proxies {
			included, includeMatches, excludeMatches := matchProxyDistribution(proxy.Name, include, exclude)
			matches = append(matches, domain.DownloadTokenProxyMatch{
				Name:                   proxy.Name,
				SourceID:               proxy.SourceID,
				Included:               included,
				MatchedIncludePatterns: includeMatches,
				MatchedExcludePatterns: excludeMatches,
			})
			if !included {
				continue
			}
			filteredProxies = append(filteredProxies, proxy.Clone())
			if index, ok := sourceIndexFromMetadata(proxy.Metadata); ok {
				selectedIndexes[index] = struct{}{}
			}
		}

		filteredRaw := make([]domain.RawProxyIR, 0, len(cloned.Snapshot.RawProxies))
		for _, raw := range cloned.Snapshot.RawProxies {
			if _, ok := selectedIndexes[raw.Index]; ok {
				filteredRaw = append(filteredRaw, raw.Clone())
			}
		}

		cloned.Snapshot.Proxies = filteredProxies
		cloned.Snapshot.RawProxies = filteredRaw
		out = append(out, cloned)
	}
	return out, matches, nil
}

func matchProxyDistribution(name string, include, exclude []*regexp.Regexp) (bool, []string, []string) {
	includeMatches := matchRegexPatterns(name, include)
	excludeMatches := matchRegexPatterns(name, exclude)
	if len(excludeMatches) > 0 {
		return false, includeMatches, excludeMatches
	}
	if len(include) == 0 {
		return true, nil, nil
	}
	return len(includeMatches) > 0, includeMatches, excludeMatches
}

func matchRegexPatterns(name string, patterns []*regexp.Regexp) []string {
	out := make([]string, 0)
	for _, re := range patterns {
		if re.MatchString(name) {
			out = append(out, re.String())
		}
	}
	return out
}

func sourceIndexFromMetadata(metadata map[string]string) (int, bool) {
	if len(metadata) == 0 {
		return 0, false
	}
	raw, ok := metadata["source_index"]
	if !ok || strings.TrimSpace(raw) == "" {
		return 0, false
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, false
	}
	return value, true
}

func (m *Manager) validateDownloadTokenDistribution(profile domain.BuildProfile, distribution domain.DownloadTokenDistribution) error {
	for _, sourceID := range distribution.SubscriptionSourceIDs {
		if !containsStringValue(profile.SubscriptionSourceIDs, sourceID) {
			return fmt.Errorf("%w: subscription source %q is not part of build profile", ErrInvalidInput, sourceID)
		}
	}
	for _, pattern := range distribution.IncludeProxyPatterns {
		if _, err := regexp.Compile(pattern); err != nil {
			return fmt.Errorf("%w: invalid include proxy regex %q", ErrInvalidInput, pattern)
		}
	}
	for _, pattern := range distribution.ExcludeProxyPatterns {
		if _, err := regexp.Compile(pattern); err != nil {
			return fmt.Errorf("%w: invalid exclude proxy regex %q", ErrInvalidInput, pattern)
		}
	}
	for _, item := range distribution.Filters {
		if item.Pattern == "" {
			return fmt.Errorf("%w: filter pattern is required", ErrInvalidInput)
		}
		if _, err := regexp.Compile(item.Pattern); err != nil {
			return fmt.Errorf("%w: invalid filter regex %q", ErrInvalidInput, item.Pattern)
		}
	}
	for _, item := range distribution.Renames {
		if item.Pattern == "" {
			return fmt.Errorf("%w: rename pattern is required", ErrInvalidInput)
		}
		if _, err := regexp.Compile(item.Pattern); err != nil {
			return fmt.Errorf("%w: invalid rename regex %q", ErrInvalidInput, item.Pattern)
		}
	}
	for _, item := range distribution.RuleBindings {
		if strings.TrimSpace(item.RuleSourceID) == "" {
			return fmt.Errorf("%w: rule_source_id is required", ErrInvalidInput)
		}
		if _, err := m.store.GetRuleSource(item.RuleSourceID); err != nil {
			return fmt.Errorf("%w: rule source %q not found", ErrInvalidInput, item.RuleSourceID)
		}
	}
	for _, item := range distribution.Groups {
		if strings.TrimSpace(item.Name) == "" {
			return fmt.Errorf("%w: group name is required", ErrInvalidInput)
		}
		for _, pattern := range item.IncludePatterns {
			if _, err := regexp.Compile(pattern); err != nil {
				return fmt.Errorf("%w: invalid include pattern %q", ErrInvalidInput, pattern)
			}
		}
		for _, pattern := range item.ExcludePatterns {
			if _, err := regexp.Compile(pattern); err != nil {
				return fmt.Errorf("%w: invalid exclude pattern %q", ErrInvalidInput, pattern)
			}
		}
	}
	return nil
}

func computeDownloadTokenSignature(record domain.DownloadTokenRecord, profile domain.BuildProfile, subscriptions []domain.SubscriptionSource, ruleSources []domain.RuleSource) (string, error) {
	type sourceSig struct {
		ID        string `json:"id"`
		UpdatedAt string `json:"updated_at"`
		FetchedAt string `json:"fetched_at,omitempty"`
		ETag      string `json:"etag,omitempty"`
		Modified  string `json:"modified,omitempty"`
	}
	type ruleSig struct {
		ID        string `json:"id"`
		UpdatedAt string `json:"updated_at"`
		FetchedAt string `json:"fetched_at,omitempty"`
		ETag      string `json:"etag,omitempty"`
		Modified  string `json:"modified,omitempty"`
	}
	payload := struct {
		TokenID          string                           `json:"token_id"`
		TokenUpdatedAt   string                           `json:"token_updated_at"`
		Distribution     domain.DownloadTokenDistribution `json:"distribution"`
		ProfileID        string                           `json:"profile_id"`
		ProfileUpdatedAt string                           `json:"profile_updated_at"`
		Subscriptions    []sourceSig                      `json:"subscriptions"`
		RuleSources      []ruleSig                        `json:"rule_sources"`
	}{
		TokenID:          record.ID,
		TokenUpdatedAt:   record.UpdatedAt.UTC().Format(time.RFC3339Nano),
		Distribution:     record.Distribution.Clone(),
		ProfileID:        profile.ID,
		ProfileUpdatedAt: profile.UpdatedAt.UTC().Format(time.RFC3339Nano),
		Subscriptions:    make([]sourceSig, 0, len(subscriptions)),
		RuleSources:      make([]ruleSig, 0, len(ruleSources)),
	}
	for _, source := range subscriptions {
		payload.Subscriptions = append(payload.Subscriptions, sourceSig{
			ID:        source.ID,
			UpdatedAt: source.UpdatedAt.UTC().Format(time.RFC3339Nano),
			FetchedAt: formatTimePtr(source.LastFetchedAt),
			ETag:      source.Snapshot.ETag,
			Modified:  source.Snapshot.LastModified,
		})
	}
	for _, source := range ruleSources {
		payload.RuleSources = append(payload.RuleSources, ruleSig{
			ID:        source.ID,
			UpdatedAt: source.UpdatedAt.UTC().Format(time.RFC3339Nano),
			FetchedAt: formatTimePtr(source.LastFetchedAt),
			ETag:      source.Snapshot.ETag,
			Modified:  source.Snapshot.LastModified,
		})
	}
	blob, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(blob)
	return hex.EncodeToString(sum[:]), nil
}

func computeDownloadTokenDiff(base, token domain.BuildPreview) domain.DownloadTokenDiff {
	baseProxyNames := previewProxyNames(base)
	tokenProxyNames := previewProxyNames(token)
	baseGroupNames := previewGroupNames(base)
	tokenGroupNames := previewGroupNames(token)

	return domain.DownloadTokenDiff{
		BaseSummary:       base.Summary,
		TokenSummary:      token.Summary,
		AddedProxyNames:   setDifference(tokenProxyNames, baseProxyNames),
		RemovedProxyNames: setDifference(baseProxyNames, tokenProxyNames),
		AddedGroups:       setDifference(tokenGroupNames, baseGroupNames),
		RemovedGroups:     setDifference(baseGroupNames, tokenGroupNames),
		BaseRuleCount:     len(base.Rules),
		TokenRuleCount:    len(token.Rules),
	}
}

func previewProxyNames(preview domain.BuildPreview) []string {
	out := make([]string, 0, len(preview.Proxies))
	for _, item := range preview.Proxies {
		out = append(out, item.Name)
	}
	sort.Strings(out)
	return out
}

func previewGroupNames(preview domain.BuildPreview) []string {
	out := make([]string, 0, len(preview.Groups))
	for _, item := range preview.Groups {
		out = append(out, item.Name)
	}
	sort.Strings(out)
	return out
}

func setDifference(left, right []string) []string {
	rightSet := make(map[string]struct{}, len(right))
	for _, item := range right {
		rightSet[item] = struct{}{}
	}
	out := make([]string, 0)
	for _, item := range left {
		if _, ok := rightSet[item]; !ok {
			out = append(out, item)
		}
	}
	return out
}

func applyTemplateOverride(base domain.BuildTemplate, override domain.BuildTemplateOverride) domain.BuildTemplate {
	out := base.Clone()
	if override.Port != nil {
		out.Port = *override.Port
	}
	if override.SocksPort != nil {
		out.SocksPort = *override.SocksPort
	}
	if override.MixedPort != nil {
		out.MixedPort = *override.MixedPort
	}
	if override.AllowLan != nil {
		out.AllowLan = *override.AllowLan
	}
	if override.Mode != nil {
		out.Mode = strings.TrimSpace(*override.Mode)
	}
	if override.LogLevel != nil {
		out.LogLevel = strings.TrimSpace(*override.LogLevel)
	}
	if override.UnifiedDelay != nil {
		out.UnifiedDelay = *override.UnifiedDelay
	}
	if override.IPv6 != nil {
		out.IPv6 = *override.IPv6
	}
	if override.ExternalController != nil {
		out.ExternalController = strings.TrimSpace(*override.ExternalController)
	}
	if override.Secret != nil {
		out.Secret = strings.TrimSpace(*override.Secret)
	}
	if override.DNS != nil {
		if override.DNS.Enable != nil {
			out.DNS.Enable = *override.DNS.Enable
		}
		if override.DNS.Listen != nil {
			out.DNS.Listen = strings.TrimSpace(*override.DNS.Listen)
		}
		if override.DNS.IPv6 != nil {
			out.DNS.IPv6 = *override.DNS.IPv6
		}
		if override.DNS.EnhancedMode != nil {
			out.DNS.EnhancedMode = strings.TrimSpace(*override.DNS.EnhancedMode)
		}
		if override.DNS.FakeIPRange != nil {
			out.DNS.FakeIPRange = strings.TrimSpace(*override.DNS.FakeIPRange)
		}
		if override.DNS.DefaultNameserver != nil {
			out.DNS.DefaultNameserver = copyStringSlice(*override.DNS.DefaultNameserver)
		}
		if override.DNS.ProxyServerNameserver != nil {
			out.DNS.ProxyServerNameserver = copyStringSlice(*override.DNS.ProxyServerNameserver)
		}
		if override.DNS.Nameserver != nil {
			out.DNS.Nameserver = copyStringSlice(*override.DNS.Nameserver)
		}
	}
	return out
}

func buildArtifactFromTokenCache(record domain.DownloadTokenRecord, cache domain.DownloadTokenArtifactCache) domain.BuildArtifact {
	return domain.BuildArtifact{
		ID:        "",
		ProfileID: record.BuildProfileID,
		RunID:     "",
		FileName:  cache.FileName,
		Content:   cache.Content,
		SHA256:    cache.SHA256,
		Summary:   cache.Summary,
		CreatedAt: cache.LastModified,
	}
}

func weakETag(sha string) string {
	if strings.TrimSpace(sha) == "" {
		return ""
	}
	return `W/"` + sha + `"`
}

func hashText(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

func formatTimePtr(value *time.Time) string {
	if value == nil || value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}

func compileRegexes(patterns []string) ([]*regexp.Regexp, error) {
	out := make([]*regexp.Regexp, 0, len(patterns))
	for _, pattern := range patterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, err
		}
		out = append(out, re)
	}
	return out, nil
}

func hashDownloadToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func newDownloadTokenValue() (string, error) {
	var raw [18]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return "smt_" + hex.EncodeToString(raw[:]), nil
}

func downloadTokenPrefix(token string) string {
	if len(token) <= 12 {
		return token
	}
	return token[:12]
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func copyStringSlice(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, len(in))
	copy(out, in)
	return out
}
