package domain

import "time"

type RefreshStatus string

const (
	RefreshStatusIdle      RefreshStatus = "idle"
	RefreshStatusRunning   RefreshStatus = "running"
	RefreshStatusSucceeded RefreshStatus = "succeeded"
	RefreshStatusFailed    RefreshStatus = "failed"
)

type RuleSourceMode string

const (
	RuleSourceModeLinkOnly  RuleSourceMode = "link_only"
	RuleSourceModeFetchText RuleSourceMode = "fetch_text"
)

type RuleStorage string

const (
	RuleStorageReference  RuleStorage = "reference"
	RuleStorageInlineText RuleStorage = "inline_text"
)

type JobKind string

const (
	JobKindSubscriptionRefresh JobKind = "subscription_refresh"
	JobKindRuleSourceRefresh   JobKind = "rule_source_refresh"
)

type JobTargetType string

const (
	JobTargetSubscription JobTargetType = "subscription"
	JobTargetRuleSource   JobTargetType = "rule_source"
)

type JobStatus string

const (
	JobStatusQueued    JobStatus = "queued"
	JobStatusRunning   JobStatus = "running"
	JobStatusSucceeded JobStatus = "succeeded"
	JobStatusFailed    JobStatus = "failed"
)

type SubscriptionSource struct {
	ID                  string               `json:"id"`
	Name                string               `json:"name"`
	Type                string               `json:"type,omitempty"` // "remote" or "local"
	URL                 string               `json:"url,omitempty"`
	Payload             string               `json:"payload,omitempty"`
	Headers             map[string]string    `json:"headers,omitempty"`
	Enabled             bool                 `json:"enabled"`
	TimeoutSec          int                  `json:"timeout_sec"`
	UserAgent           string               `json:"user_agent,omitempty"`
	RetryAttempts       int                  `json:"retry_attempts,omitempty"`
	RetryBackoffMS      int                  `json:"retry_backoff_ms,omitempty"`
	MinFetchIntervalSec int                  `json:"min_fetch_interval_sec,omitempty"`
	CacheTTLSeconds     int                  `json:"cache_ttl_seconds,omitempty"`
	RefreshIntervalSec  int                  `json:"refresh_interval_sec,omitempty"`
	Status              RefreshStatus        `json:"status"`
	LastError           string               `json:"last_error,omitempty"`
	LastFetchedAt       *time.Time           `json:"last_fetched_at,omitempty"`
	CurrentJobID        string               `json:"current_job_id,omitempty"`
	Snapshot            SubscriptionSnapshot `json:"snapshot"`
	CreatedAt           time.Time            `json:"created_at"`
	UpdatedAt           time.Time            `json:"updated_at"`
}

type SubscriptionSnapshot struct {
	ContentType  string       `json:"content_type,omitempty"`
	ETag         string       `json:"etag,omitempty"`
	LastModified string       `json:"last_modified,omitempty"`
	RawProxies   []RawProxyIR `json:"raw_proxies,omitempty"`
	Proxies      []ProxyIR    `json:"proxies,omitempty"`
	Warnings     []string     `json:"warnings,omitempty"`
}

type RawProxyIR struct {
	Index     int            `json:"index"`
	Name      string         `json:"name"`
	Type      string         `json:"type"`
	SourceURL string         `json:"source_url"`
	Original  map[string]any `json:"original"`
}

type ProxyIR struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	OriginalName     string            `json:"original_name,omitempty"`
	Type             string            `json:"type"`
	Server           string            `json:"server"`
	Port             int               `json:"port"`
	UUID             string            `json:"uuid,omitempty"`
	Password         string            `json:"password,omitempty"`
	Cipher           string            `json:"cipher,omitempty"`
	TLS              bool              `json:"tls"`
	UDP              bool              `json:"udp"`
	Network          string            `json:"network,omitempty"`
	SNI              string            `json:"sni,omitempty"`
	Host             string            `json:"host,omitempty"`
	Path             string            `json:"path,omitempty"`
	SourceID         string            `json:"source_id"`
	SourceURL        string            `json:"source_url"`
	Metadata         map[string]string `json:"metadata,omitempty"`
	VLESSOptions     *VLESSOptions     `json:"vless_options,omitempty"`
	TUICOptions      *TUICOptions      `json:"tuic_options,omitempty"`
	Hysteria2Options *Hysteria2Options `json:"hysteria2_options,omitempty"`
}

type VLESSOptions struct {
	Flow              string          `json:"flow,omitempty"`
	PacketEncoding    string          `json:"packet_encoding,omitempty"`
	Encryption        string          `json:"encryption,omitempty"`
	SkipCertVerify    bool            `json:"skip_cert_verify"`
	Fingerprint       string          `json:"fingerprint,omitempty"`
	ClientFingerprint string          `json:"client_fingerprint,omitempty"`
	ALPN              []string        `json:"alpn,omitempty"`
	RealityOptions    *RealityOptions `json:"reality_options,omitempty"`
	SmuxOptions       *SmuxOptions    `json:"smux_options,omitempty"`
}

type TUICOptions struct {
	Token                 string   `json:"token,omitempty"`
	IP                    string   `json:"ip,omitempty"`
	HeartbeatIntervalMS   int      `json:"heartbeat_interval_ms,omitempty"`
	DisableSNI            bool     `json:"disable_sni"`
	ReduceRTT             bool     `json:"reduce_rtt"`
	RequestTimeoutMS      int      `json:"request_timeout_ms,omitempty"`
	UDPRelayMode          string   `json:"udp_relay_mode,omitempty"`
	CongestionController  string   `json:"congestion_controller,omitempty"`
	MaxUDPRelayPacketSize int      `json:"max_udp_relay_packet_size,omitempty"`
	FastOpen              bool     `json:"fast_open"`
	MaxOpenStreams        int      `json:"max_open_streams,omitempty"`
	SkipCertVerify        bool     `json:"skip_cert_verify"`
	ALPN                  []string `json:"alpn,omitempty"`
}

type Hysteria2Options struct {
	Ports                          string   `json:"ports,omitempty"`
	HopIntervalSec                 int      `json:"hop_interval_sec,omitempty"`
	Up                             string   `json:"up,omitempty"`
	Down                           string   `json:"down,omitempty"`
	Obfs                           string   `json:"obfs,omitempty"`
	ObfsPassword                   string   `json:"obfs_password,omitempty"`
	SkipCertVerify                 bool     `json:"skip_cert_verify"`
	Fingerprint                    string   `json:"fingerprint,omitempty"`
	ALPN                           []string `json:"alpn,omitempty"`
	InitialStreamReceiveWindow     int      `json:"initial_stream_receive_window,omitempty"`
	MaxStreamReceiveWindow         int      `json:"max_stream_receive_window,omitempty"`
	InitialConnectionReceiveWindow int      `json:"initial_connection_receive_window,omitempty"`
	MaxConnectionReceiveWindow     int      `json:"max_connection_receive_window,omitempty"`
}

type RealityOptions struct {
	PublicKey             string `json:"public_key,omitempty"`
	ShortID               string `json:"short_id,omitempty"`
	SupportX25519MLKEM768 bool   `json:"support_x25519_mlkem768"`
}

type SmuxOptions struct {
	Enabled bool `json:"enabled"`
}

type RuleSource struct {
	ID                  string            `json:"id"`
	Name                string            `json:"name"`
	URL                 string            `json:"url"`
	Mode                RuleSourceMode    `json:"mode"`
	Headers             map[string]string `json:"headers,omitempty"`
	Enabled             bool              `json:"enabled"`
	TimeoutSec          int               `json:"timeout_sec"`
	UserAgent           string            `json:"user_agent,omitempty"`
	RetryAttempts       int               `json:"retry_attempts,omitempty"`
	RetryBackoffMS      int               `json:"retry_backoff_ms,omitempty"`
	MinFetchIntervalSec int               `json:"min_fetch_interval_sec,omitempty"`
	CacheTTLSeconds     int               `json:"cache_ttl_seconds,omitempty"`
	RefreshIntervalSec  int               `json:"refresh_interval_sec,omitempty"`
	Status              RefreshStatus     `json:"status"`
	LastError           string            `json:"last_error,omitempty"`
	LastFetchedAt       *time.Time        `json:"last_fetched_at,omitempty"`
	CurrentJobID        string            `json:"current_job_id,omitempty"`
	Snapshot            RuleSnapshot      `json:"snapshot"`
	CreatedAt           time.Time         `json:"created_at"`
	UpdatedAt           time.Time         `json:"updated_at"`
}

type RuleSnapshot struct {
	ContentType  string         `json:"content_type,omitempty"`
	ETag         string         `json:"etag,omitempty"`
	LastModified string         `json:"last_modified,omitempty"`
	RawText      string         `json:"raw_text,omitempty"`
	IR           RuleDocumentIR `json:"ir"`
	Warnings     []string       `json:"warnings,omitempty"`
}

type RuleDocumentIR struct {
	Storage   RuleStorage       `json:"storage"`
	Format    string            `json:"format,omitempty"`
	SourceURL string            `json:"source_url"`
	Reference string            `json:"reference,omitempty"`
	Entries   []RuleEntryIR     `json:"entries,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

type RuleEntryIR struct {
	Index  int      `json:"index"`
	Raw    string   `json:"raw"`
	Type   string   `json:"type"`
	Value  string   `json:"value,omitempty"`
	Policy string   `json:"policy,omitempty"`
	Params []string `json:"params,omitempty"`
}

type Job struct {
	ID         string        `json:"id"`
	Kind       JobKind       `json:"kind"`
	TargetType JobTargetType `json:"target_type"`
	TargetID   string        `json:"target_id"`
	Status     JobStatus     `json:"status"`
	Error      string        `json:"error,omitempty"`
	Summary    JobSummary    `json:"summary"`
	CreatedAt  time.Time     `json:"created_at"`
	StartedAt  *time.Time    `json:"started_at,omitempty"`
	FinishedAt *time.Time    `json:"finished_at,omitempty"`
}

type JobSummary struct {
	RawProxyCount int `json:"raw_proxy_count,omitempty"`
	ProxyCount    int `json:"proxy_count,omitempty"`
	RuleCount     int `json:"rule_count,omitempty"`
	WarningCount  int `json:"warning_count,omitempty"`
}

func (s SubscriptionSource) Clone() SubscriptionSource {
	out := s
	out.Headers = cloneStringMap(s.Headers)
	out.Snapshot = s.Snapshot.Clone()
	out.LastFetchedAt = cloneTimePtr(s.LastFetchedAt)
	return out
}

func (s SubscriptionSnapshot) Clone() SubscriptionSnapshot {
	out := s
	out.RawProxies = make([]RawProxyIR, 0, len(s.RawProxies))
	for _, item := range s.RawProxies {
		out.RawProxies = append(out.RawProxies, item.Clone())
	}
	out.Proxies = make([]ProxyIR, 0, len(s.Proxies))
	for _, item := range s.Proxies {
		out.Proxies = append(out.Proxies, item.Clone())
	}
	out.Warnings = cloneStringSlice(s.Warnings)
	return out
}

func (r RawProxyIR) Clone() RawProxyIR {
	out := r
	out.Original = cloneStringAnyMap(r.Original)
	return out
}

func (p ProxyIR) Clone() ProxyIR {
	out := p
	out.Metadata = cloneStringMap(p.Metadata)
	out.VLESSOptions = cloneVLESSOptions(p.VLESSOptions)
	out.TUICOptions = cloneTUICOptions(p.TUICOptions)
	out.Hysteria2Options = cloneHysteria2Options(p.Hysteria2Options)
	return out
}

func (r RuleSource) Clone() RuleSource {
	out := r
	out.Headers = cloneStringMap(r.Headers)
	out.Snapshot = r.Snapshot.Clone()
	out.LastFetchedAt = cloneTimePtr(r.LastFetchedAt)
	return out
}

func (r RuleSnapshot) Clone() RuleSnapshot {
	out := r
	out.IR = r.IR.Clone()
	out.Warnings = cloneStringSlice(r.Warnings)
	return out
}

func (r RuleDocumentIR) Clone() RuleDocumentIR {
	out := r
	out.Metadata = cloneStringMap(r.Metadata)
	out.Entries = make([]RuleEntryIR, 0, len(r.Entries))
	for _, item := range r.Entries {
		out.Entries = append(out.Entries, item.Clone())
	}
	return out
}

func (r RuleEntryIR) Clone() RuleEntryIR {
	out := r
	out.Params = cloneStringSlice(r.Params)
	return out
}

func (j Job) Clone() Job {
	out := j
	out.StartedAt = cloneTimePtr(j.StartedAt)
	out.FinishedAt = cloneTimePtr(j.FinishedAt)
	return out
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneStringSlice(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, len(in))
	copy(out, in)
	return out
}

func cloneTimePtr(in *time.Time) *time.Time {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func cloneIntPtr(in *int) *int {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func cloneBoolPtr(in *bool) *bool {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func cloneStringPtr(in *string) *string {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func cloneStringSlicePtr(in *[]string) *[]string {
	if in == nil {
		return nil
	}
	out := cloneStringSlice(*in)
	return &out
}

func cloneStringAnyMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = cloneAny(v)
	}
	return out
}

func cloneAny(in any) any {
	switch value := in.(type) {
	case map[string]any:
		return cloneStringAnyMap(value)
	case []any:
		out := make([]any, len(value))
		for i := range value {
			out[i] = cloneAny(value[i])
		}
		return out
	default:
		return value
	}
}

func cloneVLESSOptions(in *VLESSOptions) *VLESSOptions {
	if in == nil {
		return nil
	}
	out := *in
	out.ALPN = cloneStringSlice(in.ALPN)
	out.RealityOptions = cloneRealityOptions(in.RealityOptions)
	out.SmuxOptions = cloneSmuxOptions(in.SmuxOptions)
	return &out
}

func cloneTUICOptions(in *TUICOptions) *TUICOptions {
	if in == nil {
		return nil
	}
	out := *in
	out.ALPN = cloneStringSlice(in.ALPN)
	return &out
}

func cloneHysteria2Options(in *Hysteria2Options) *Hysteria2Options {
	if in == nil {
		return nil
	}
	out := *in
	out.ALPN = cloneStringSlice(in.ALPN)
	return &out
}

func cloneRealityOptions(in *RealityOptions) *RealityOptions {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func cloneSmuxOptions(in *SmuxOptions) *SmuxOptions {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

type AlertLevel string

const (
	AlertLevelWarning AlertLevel = "warning"
	AlertLevelError   AlertLevel = "error"
)

type SystemAlert struct {
	ID        string     `json:"id"`
	Level     AlertLevel `json:"level"`
	Message   string     `json:"message"`
	CreatedAt time.Time  `json:"created_at"`
}
