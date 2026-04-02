package domain

import "time"

type BuildRuleBindingMode string

const (
	BuildRuleBindingModeAuto     BuildRuleBindingMode = "auto"
	BuildRuleBindingModeInline   BuildRuleBindingMode = "inline"
	BuildRuleBindingModeProvider BuildRuleBindingMode = "provider"
)

type ProxyGroupType string

const (
	ProxyGroupTypeSelect      ProxyGroupType = "select"
	ProxyGroupTypeURLTest     ProxyGroupType = "url-test"
	ProxyGroupTypeFallback    ProxyGroupType = "fallback"
	ProxyGroupTypeLoadBalance ProxyGroupType = "load-balance"
)

type BuildRunStatus string

const (
	BuildRunStatusQueued    BuildRunStatus = "queued"
	BuildRunStatusRunning   BuildRunStatus = "running"
	BuildRunStatusSucceeded BuildRunStatus = "succeeded"
	BuildRunStatusFailed    BuildRunStatus = "failed"
)

type BuildProfile struct {
	ID                    string             `json:"id"`
	Name                  string             `json:"name"`
	Description           string             `json:"description,omitempty"`
	SubscriptionSourceIDs []string           `json:"subscription_source_ids"`
	RuleBindings          []BuildRuleBinding `json:"rule_bindings,omitempty"`
	Template              BuildTemplate      `json:"template"`
	Filters               []ProxyFilterRule  `json:"filters,omitempty"`
	Renames               []RenameRule       `json:"renames,omitempty"`
	Groups                []ProxyGroupSpec   `json:"groups,omitempty"`
	DefaultGroup          string             `json:"default_group,omitempty"`
	Enabled               bool               `json:"enabled"`
	AutoBuild             bool               `json:"auto_build,omitempty"`
	BuildIntervalSec      int                `json:"build_interval_sec,omitempty"`
	Status                RefreshStatus      `json:"status"`
	LastError             string             `json:"last_error,omitempty"`
	LastBuiltAt           *time.Time         `json:"last_built_at,omitempty"`
	CurrentRunID          string             `json:"current_run_id,omitempty"`
	LastRunID             string             `json:"last_run_id,omitempty"`
	LastArtifactID        string             `json:"last_artifact_id,omitempty"`
	CreatedAt             time.Time          `json:"created_at"`
	UpdatedAt             time.Time          `json:"updated_at"`
}

type BuildTemplate struct {
	Port               int            `json:"port,omitempty"`
	SocksPort          int            `json:"socks_port,omitempty"`
	MixedPort          int            `json:"mixed_port,omitempty"`
	AllowLan           bool           `json:"allow_lan"`
	Mode               string         `json:"mode,omitempty"`
	LogLevel           string         `json:"log_level,omitempty"`
	UnifiedDelay       bool           `json:"unified_delay"`
	IPv6               bool           `json:"ipv6"`
	ExternalController string         `json:"external_controller,omitempty"`
	Secret             string         `json:"secret,omitempty"`
	DNS                BuildDNSConfig `json:"dns"`
	RawBaseYAML        string         `json:"raw_base_yaml,omitempty"`
}

type BuildTemplateOverride struct {
	Port               *int                    `json:"port,omitempty"`
	SocksPort          *int                    `json:"socks_port,omitempty"`
	MixedPort          *int                    `json:"mixed_port,omitempty"`
	AllowLan           *bool                   `json:"allow_lan,omitempty"`
	Mode               *string                 `json:"mode,omitempty"`
	LogLevel           *string                 `json:"log_level,omitempty"`
	UnifiedDelay       *bool                   `json:"unified_delay,omitempty"`
	IPv6               *bool                   `json:"ipv6,omitempty"`
	ExternalController *string                 `json:"external_controller,omitempty"`
	Secret             *string                 `json:"secret,omitempty"`
	DNS                *BuildDNSConfigOverride `json:"dns,omitempty"`
}

type BuildDNSConfig struct {
	Enable                bool     `json:"enable"`
	Listen                string   `json:"listen,omitempty"`
	IPv6                  bool     `json:"ipv6"`
	EnhancedMode          string   `json:"enhanced_mode,omitempty"`
	FakeIPRange           string   `json:"fake_ip_range,omitempty"`
	DefaultNameserver     []string `json:"default_nameserver,omitempty"`
	ProxyServerNameserver []string `json:"proxy_server_nameserver,omitempty"`
	Nameserver            []string `json:"nameserver,omitempty"`
}

type BuildDNSConfigOverride struct {
	Enable                *bool     `json:"enable,omitempty"`
	Listen                *string   `json:"listen,omitempty"`
	IPv6                  *bool     `json:"ipv6,omitempty"`
	EnhancedMode          *string   `json:"enhanced_mode,omitempty"`
	FakeIPRange           *string   `json:"fake_ip_range,omitempty"`
	DefaultNameserver     *[]string `json:"default_nameserver,omitempty"`
	ProxyServerNameserver *[]string `json:"proxy_server_nameserver,omitempty"`
	Nameserver            *[]string `json:"nameserver,omitempty"`
}

type BuildRuleBinding struct {
	RuleSourceID string               `json:"rule_source_id"`
	Policy       string               `json:"policy,omitempty"`
	Mode         BuildRuleBindingMode `json:"mode,omitempty"`
	Behavior     string               `json:"behavior,omitempty"`
	Path         string               `json:"path,omitempty"`
	IntervalSec  int                  `json:"interval_sec,omitempty"`
}

type ProxyFilterRule struct {
	Pattern string `json:"pattern"`
}

type RenameRule struct {
	Pattern string `json:"pattern"`
	Replace string `json:"replace"`
}

type ProxyGroupSpec struct {
	Name            string         `json:"name"`
	Type            ProxyGroupType `json:"type"`
	Members         []string       `json:"members,omitempty"`
	IncludeAll      bool           `json:"include_all,omitempty"`
	IncludePatterns []string       `json:"include_patterns,omitempty"`
	ExcludePatterns []string       `json:"exclude_patterns,omitempty"`
	URL             string         `json:"url,omitempty"`
	IntervalSec     int            `json:"interval_sec,omitempty"`
	Tolerance       int            `json:"tolerance,omitempty"`
	Lazy            bool           `json:"lazy,omitempty"`
}

type BuildRun struct {
	ID         string          `json:"id"`
	ProfileID  string          `json:"profile_id"`
	Status     BuildRunStatus  `json:"status"`
	Error      string          `json:"error,omitempty"`
	ArtifactID string          `json:"artifact_id,omitempty"`
	Summary    BuildRunSummary `json:"summary"`
	CreatedAt  time.Time       `json:"created_at"`
	StartedAt  *time.Time      `json:"started_at,omitempty"`
	FinishedAt *time.Time      `json:"finished_at,omitempty"`
}

type BuildRunSummary struct {
	InputProxyCount   int `json:"input_proxy_count,omitempty"`
	OutputProxyCount  int `json:"output_proxy_count,omitempty"`
	GroupCount        int `json:"group_count,omitempty"`
	RuleProviderCount int `json:"rule_provider_count,omitempty"`
	RuleCount         int `json:"rule_count,omitempty"`
	WarningCount      int `json:"warning_count,omitempty"`
}

type BuildArtifact struct {
	ID        string          `json:"id"`
	ProfileID string          `json:"profile_id"`
	RunID     string          `json:"run_id"`
	FileName  string          `json:"file_name"`
	Content   string          `json:"content"`
	SHA256    string          `json:"sha256"`
	Summary   BuildRunSummary `json:"summary"`
	CreatedAt time.Time       `json:"created_at"`
}

type BuildPreview struct {
	ProfileID     string                     `json:"profile_id"`
	Summary       BuildRunSummary            `json:"summary"`
	Warnings      []string                   `json:"warnings,omitempty"`
	Proxies       []BuildProxyPreview        `json:"proxies,omitempty"`
	Groups        []BuildGroupPreview        `json:"groups,omitempty"`
	RuleProviders []BuildRuleProviderPreview `json:"rule_providers,omitempty"`
	Rules         []string                   `json:"rules,omitempty"`
	YAML          string                     `json:"yaml,omitempty"`
}

type BuildProxyPreview struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	SourceID string `json:"source_id"`
}

type BuildGroupPreview struct {
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	Members []string `json:"members,omitempty"`
}

type BuildRuleProviderPreview struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Behavior string `json:"behavior,omitempty"`
	URL      string `json:"url,omitempty"`
	Path     string `json:"path,omitempty"`
}

func (p BuildProfile) Clone() BuildProfile {
	out := p
	out.SubscriptionSourceIDs = cloneStringSlice(p.SubscriptionSourceIDs)
	out.RuleBindings = make([]BuildRuleBinding, 0, len(p.RuleBindings))
	for _, item := range p.RuleBindings {
		out.RuleBindings = append(out.RuleBindings, item.Clone())
	}
	out.Template = p.Template.Clone()
	out.Filters = make([]ProxyFilterRule, 0, len(p.Filters))
	for _, item := range p.Filters {
		out.Filters = append(out.Filters, item.Clone())
	}
	out.Renames = make([]RenameRule, 0, len(p.Renames))
	for _, item := range p.Renames {
		out.Renames = append(out.Renames, item.Clone())
	}
	out.Groups = make([]ProxyGroupSpec, 0, len(p.Groups))
	for _, item := range p.Groups {
		out.Groups = append(out.Groups, item.Clone())
	}
	out.LastBuiltAt = cloneTimePtr(p.LastBuiltAt)
	return out
}

func (t BuildTemplate) Clone() BuildTemplate {
	out := t
	out.DNS = t.DNS.Clone()
	return out
}

func (t BuildTemplateOverride) Clone() BuildTemplateOverride {
	out := t
	out.Port = cloneIntPtr(t.Port)
	out.SocksPort = cloneIntPtr(t.SocksPort)
	out.MixedPort = cloneIntPtr(t.MixedPort)
	out.AllowLan = cloneBoolPtr(t.AllowLan)
	out.Mode = cloneStringPtr(t.Mode)
	out.LogLevel = cloneStringPtr(t.LogLevel)
	out.UnifiedDelay = cloneBoolPtr(t.UnifiedDelay)
	out.IPv6 = cloneBoolPtr(t.IPv6)
	out.ExternalController = cloneStringPtr(t.ExternalController)
	out.Secret = cloneStringPtr(t.Secret)
	out.DNS = cloneBuildDNSConfigOverride(t.DNS)
	return out
}

func (d BuildDNSConfig) Clone() BuildDNSConfig {
	out := d
	out.DefaultNameserver = cloneStringSlice(d.DefaultNameserver)
	out.ProxyServerNameserver = cloneStringSlice(d.ProxyServerNameserver)
	out.Nameserver = cloneStringSlice(d.Nameserver)
	return out
}

func (d BuildDNSConfigOverride) Clone() BuildDNSConfigOverride {
	out := d
	out.Enable = cloneBoolPtr(d.Enable)
	out.Listen = cloneStringPtr(d.Listen)
	out.IPv6 = cloneBoolPtr(d.IPv6)
	out.EnhancedMode = cloneStringPtr(d.EnhancedMode)
	out.FakeIPRange = cloneStringPtr(d.FakeIPRange)
	out.DefaultNameserver = cloneStringSlicePtr(d.DefaultNameserver)
	out.ProxyServerNameserver = cloneStringSlicePtr(d.ProxyServerNameserver)
	out.Nameserver = cloneStringSlicePtr(d.Nameserver)
	return out
}

func cloneBuildDNSConfigOverride(in *BuildDNSConfigOverride) *BuildDNSConfigOverride {
	if in == nil {
		return nil
	}
	out := in.Clone()
	return &out
}

func (b BuildRuleBinding) Clone() BuildRuleBinding {
	return b
}

func (f ProxyFilterRule) Clone() ProxyFilterRule {
	return f
}

func (r RenameRule) Clone() RenameRule {
	return r
}

func (g ProxyGroupSpec) Clone() ProxyGroupSpec {
	out := g
	out.Members = cloneStringSlice(g.Members)
	out.IncludePatterns = cloneStringSlice(g.IncludePatterns)
	out.ExcludePatterns = cloneStringSlice(g.ExcludePatterns)
	return out
}

func (r BuildRun) Clone() BuildRun {
	out := r
	out.StartedAt = cloneTimePtr(r.StartedAt)
	out.FinishedAt = cloneTimePtr(r.FinishedAt)
	return out
}

func (a BuildArtifact) Clone() BuildArtifact {
	return a
}
