package domain

import "time"

type TokenAccessLog struct {
	At time.Time `json:"at"`
	IP string    `json:"ip,omitempty"`
}

type DownloadTokenRecord struct {
	ID             string                      `json:"id"`
	Name           string                      `json:"name"`
	BuildProfileID string                      `json:"build_profile_id"`
	TokenHash      string                      `json:"token_hash"`
	TokenPrefix    string                      `json:"token_prefix"`
	Distribution   DownloadTokenDistribution   `json:"distribution"`
	Prebuild       bool                        `json:"prebuild,omitempty"`
	CachedArtifact *DownloadTokenArtifactCache `json:"cached_artifact,omitempty"`
	Enabled        bool                        `json:"enabled"`
	LastUsedAt     *time.Time                  `json:"last_used_at,omitempty"`
	FetchCount     int64                       `json:"fetch_count,omitempty"`
	AccessLog      []TokenAccessLog            `json:"access_log,omitempty"`
	CreatedAt      time.Time                   `json:"created_at"`
	UpdatedAt      time.Time                   `json:"updated_at"`
}

type DownloadToken struct {
	ID             string                     `json:"id"`
	Name           string                     `json:"name"`
	BuildProfileID string                     `json:"build_profile_id"`
	TokenPrefix    string                     `json:"token_prefix"`
	Distribution   DownloadTokenDistribution  `json:"distribution"`
	Prebuild       bool                       `json:"prebuild,omitempty"`
	CachedArtifact *DownloadTokenArtifactMeta `json:"cached_artifact,omitempty"`
	Enabled        bool                       `json:"enabled"`
	LastUsedAt     *time.Time                 `json:"last_used_at,omitempty"`
	FetchCount     int64                      `json:"fetch_count,omitempty"`
	AccessLog      []TokenAccessLog           `json:"access_log,omitempty"`
	CreatedAt      time.Time                  `json:"created_at"`
	UpdatedAt      time.Time                  `json:"updated_at"`
}

type DownloadTokenDistribution struct {
	SubscriptionSourceIDs []string               `json:"subscription_source_ids,omitempty"`
	IncludeProxyPatterns  []string               `json:"include_proxy_patterns,omitempty"`
	ExcludeProxyPatterns  []string               `json:"exclude_proxy_patterns,omitempty"`
	Filters               []ProxyFilterRule      `json:"filters,omitempty"`
	Renames               []RenameRule           `json:"renames,omitempty"`
	OverrideRuleBindings  bool                   `json:"override_rule_bindings,omitempty"`
	RuleBindings          []BuildRuleBinding     `json:"rule_bindings,omitempty"`
	OverrideGroups        bool                   `json:"override_groups,omitempty"`
	Groups                []ProxyGroupSpec       `json:"groups,omitempty"`
	DefaultGroup          string                 `json:"default_group,omitempty"`
	TemplateOverride      *BuildTemplateOverride `json:"template_override,omitempty"`
}

type DownloadTokenArtifactCache struct {
	Signature    string          `json:"signature"`
	FileName     string          `json:"file_name"`
	Content      string          `json:"content"`
	SHA256       string          `json:"sha256"`
	ETag         string          `json:"etag"`
	Summary      BuildRunSummary `json:"summary"`
	LastBuiltAt  time.Time       `json:"last_built_at"`
	LastModified time.Time       `json:"last_modified"`
}

type DownloadTokenArtifactMeta struct {
	FileName     string          `json:"file_name"`
	SHA256       string          `json:"sha256"`
	ETag         string          `json:"etag"`
	Summary      BuildRunSummary `json:"summary"`
	LastBuiltAt  time.Time       `json:"last_built_at"`
	LastModified time.Time       `json:"last_modified"`
}

type DownloadTokenPreview struct {
	TokenID      string                    `json:"token_id"`
	ProfileID    string                    `json:"profile_id"`
	Build        BuildPreview              `json:"build"`
	Diff         DownloadTokenDiff         `json:"diff"`
	ProxyMatches []DownloadTokenProxyMatch `json:"proxy_matches,omitempty"`
}

type DownloadTokenDiff struct {
	BaseSummary       BuildRunSummary `json:"base_summary"`
	TokenSummary      BuildRunSummary `json:"token_summary"`
	AddedProxyNames   []string        `json:"added_proxy_names,omitempty"`
	RemovedProxyNames []string        `json:"removed_proxy_names,omitempty"`
	AddedGroups       []string        `json:"added_groups,omitempty"`
	RemovedGroups     []string        `json:"removed_groups,omitempty"`
	BaseRuleCount     int             `json:"base_rule_count,omitempty"`
	TokenRuleCount    int             `json:"token_rule_count,omitempty"`
}

type DownloadTokenProxyMatch struct {
	Name                   string   `json:"name"`
	SourceID               string   `json:"source_id"`
	Included               bool     `json:"included"`
	MatchedIncludePatterns []string `json:"matched_include_patterns,omitempty"`
	MatchedExcludePatterns []string `json:"matched_exclude_patterns,omitempty"`
}

func (r DownloadTokenRecord) Clone() DownloadTokenRecord {
	out := r
	out.Distribution = r.Distribution.Clone()
	out.CachedArtifact = cloneDownloadTokenArtifactCache(r.CachedArtifact)
	out.LastUsedAt = cloneTimePtr(r.LastUsedAt)
	if r.AccessLog != nil {
		out.AccessLog = make([]TokenAccessLog, len(r.AccessLog))
		copy(out.AccessLog, r.AccessLog)
	}
	return out
}

func (r DownloadTokenRecord) Public() DownloadToken {
	return DownloadToken{
		ID:             r.ID,
		Name:           r.Name,
		BuildProfileID: r.BuildProfileID,
		TokenPrefix:    r.TokenPrefix,
		Distribution:   r.Distribution.Clone(),
		Prebuild:       r.Prebuild,
		CachedArtifact: artifactMetaFromCache(r.CachedArtifact),
		Enabled:        r.Enabled,
		LastUsedAt:     cloneTimePtr(r.LastUsedAt),
		FetchCount:     r.FetchCount,
		AccessLog:      append([]TokenAccessLog(nil), r.AccessLog...),
		CreatedAt:      r.CreatedAt,
		UpdatedAt:      r.UpdatedAt,
	}
}

func (d DownloadTokenDistribution) Clone() DownloadTokenDistribution {
	out := d
	out.SubscriptionSourceIDs = cloneStringSlice(d.SubscriptionSourceIDs)
	out.IncludeProxyPatterns = cloneStringSlice(d.IncludeProxyPatterns)
	out.ExcludeProxyPatterns = cloneStringSlice(d.ExcludeProxyPatterns)
	out.Filters = make([]ProxyFilterRule, 0, len(d.Filters))
	for _, item := range d.Filters {
		out.Filters = append(out.Filters, item.Clone())
	}
	out.Renames = make([]RenameRule, 0, len(d.Renames))
	for _, item := range d.Renames {
		out.Renames = append(out.Renames, item.Clone())
	}
	out.RuleBindings = make([]BuildRuleBinding, 0, len(d.RuleBindings))
	for _, item := range d.RuleBindings {
		out.RuleBindings = append(out.RuleBindings, item.Clone())
	}
	out.Groups = make([]ProxyGroupSpec, 0, len(d.Groups))
	for _, item := range d.Groups {
		out.Groups = append(out.Groups, item.Clone())
	}
	if d.TemplateOverride != nil {
		cloned := d.TemplateOverride.Clone()
		out.TemplateOverride = &cloned
	}
	return out
}

func cloneDownloadTokenArtifactCache(in *DownloadTokenArtifactCache) *DownloadTokenArtifactCache {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func artifactMetaFromCache(in *DownloadTokenArtifactCache) *DownloadTokenArtifactMeta {
	if in == nil {
		return nil
	}
	return &DownloadTokenArtifactMeta{
		FileName:     in.FileName,
		SHA256:       in.SHA256,
		ETag:         in.ETag,
		Summary:      in.Summary,
		LastBuiltAt:  in.LastBuiltAt,
		LastModified: in.LastModified,
	}
}
