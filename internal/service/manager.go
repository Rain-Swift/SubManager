package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"submanager/internal/domain"
	"submanager/internal/fetcher"
	"submanager/internal/parser"
	"submanager/internal/store"
)

var (
	ErrInvalidInput   = errors.New("service: invalid input")
	ErrAlreadyRunning = errors.New("service: refresh already running")
	ErrDisabled       = errors.New("service: resource is disabled")
	ErrAccessDenied   = errors.New("service: access denied")
)

type CreateSubscriptionInput struct {
	Name                string
	Type                string
	URL                 string
	Payload             string
	Headers             map[string]string
	Enabled             bool
	TimeoutSec          int
	UserAgent           string
	RetryAttempts       int
	RetryBackoffMS      int
	MinFetchIntervalSec int
	CacheTTLSeconds     int
	RefreshIntervalSec  int
}

type CreateRuleSourceInput struct {
	Name                string
	URL                 string
	Mode                domain.RuleSourceMode
	Headers             map[string]string
	Enabled             bool
	TimeoutSec          int
	UserAgent           string
	RetryAttempts       int
	RetryBackoffMS      int
	MinFetchIntervalSec int
	CacheTTLSeconds     int
	RefreshIntervalSec  int
}

type UpdateSubscriptionInput struct {
	Name                *string
	Type                *string
	URL                 *string
	Payload             *string
	Headers             *map[string]string
	Enabled             *bool
	TimeoutSec          *int
	UserAgent           *string
	RetryAttempts       *int
	RetryBackoffMS      *int
	MinFetchIntervalSec *int
	CacheTTLSeconds     *int
	RefreshIntervalSec  *int
}

type UpdateRuleSourceInput struct {
	Name                *string
	URL                 *string
	Mode                *domain.RuleSourceMode
	Headers             *map[string]string
	Enabled             *bool
	TimeoutSec          *int
	UserAgent           *string
	RetryAttempts       *int
	RetryBackoffMS      *int
	MinFetchIntervalSec *int
	CacheTTLSeconds     *int
	RefreshIntervalSec  *int
}

type workKind string

const (
	workSubscriptionRefresh workKind = "subscription_refresh"
	workRuleSourceRefresh   workKind = "rule_source_refresh"
	workBuildProfile        workKind = "build_profile"
	workDownloadTokenWarm   workKind = "download_token_warm"
)

type workItem struct {
	kind workKind
	id   string
	ref  string
}

type Manager struct {
	store   store.Store
	fetcher *fetcher.HTTPFetcher

	activeMu sync.Mutex
	active   map[string]string

	startOnce      sync.Once
	started        atomic.Bool
	workQueue      chan workItem
	workerCount    int
	schedulerEvery time.Duration
}

func NewManager(store store.Store, fetcher *fetcher.HTTPFetcher) *Manager {
	return &Manager{
		store:          store,
		fetcher:        fetcher,
		active:         make(map[string]string),
		workerCount:    4,
		schedulerEvery: 30 * time.Second,
	}
}

func (m *Manager) CreateSubscriptionSource(_ context.Context, input CreateSubscriptionInput) (domain.SubscriptionSource, error) {
	if err := validateCreateSubscriptionInput(input); err != nil {
		return domain.SubscriptionSource{}, err
	}

	now := time.Now().UTC()
	source := domain.SubscriptionSource{
		ID:                  newID("sub"),
		Name:                input.Name,
		Type:                input.Type,
		URL:                 input.URL,
		Payload:             input.Payload,
		Headers:             cloneHeaders(input.Headers),
		Enabled:             input.Enabled,
		TimeoutSec:          normalizeTimeout(input.TimeoutSec),
		UserAgent:           input.UserAgent,
		RetryAttempts:       normalizeNonNegative(input.RetryAttempts),
		RetryBackoffMS:      normalizeNonNegative(input.RetryBackoffMS),
		MinFetchIntervalSec: normalizeNonNegative(input.MinFetchIntervalSec),
		CacheTTLSeconds:     normalizeNonNegative(input.CacheTTLSeconds),
		RefreshIntervalSec:  normalizeNonNegative(input.RefreshIntervalSec),
		Status:              domain.RefreshStatusIdle,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	return m.store.SaveSubscription(source)
}

func (m *Manager) ListSubscriptionSources(_ context.Context) ([]domain.SubscriptionSource, error) {
	return m.store.ListSubscriptions()
}

func (m *Manager) GetSubscriptionSource(_ context.Context, id string) (domain.SubscriptionSource, error) {
	return m.store.GetSubscription(id)
}

func (m *Manager) RefreshSubscriptionSource(_ context.Context, id string) (domain.Job, error) {
	source, err := m.store.GetSubscription(id)
	if err != nil {
		return domain.Job{}, err
	}
	if !source.Enabled {
		return domain.Job{}, fmt.Errorf("%w: subscription source %q", ErrDisabled, source.Name)
	}

	job := domain.Job{
		ID:         newID("job"),
		Kind:       domain.JobKindSubscriptionRefresh,
		TargetType: domain.JobTargetSubscription,
		TargetID:   id,
		Status:     domain.JobStatusQueued,
		CreatedAt:  time.Now().UTC(),
	}

	if err := m.claimActive(sourceKey(domain.JobTargetSubscription, id), job.ID); err != nil {
		return domain.Job{}, err
	}

	source.Status = domain.RefreshStatusRunning
	source.LastError = ""
	source.CurrentJobID = job.ID
	source.UpdatedAt = time.Now().UTC()

	if _, err := m.store.SaveJob(job); err != nil {
		m.releaseActive(sourceKey(domain.JobTargetSubscription, id))
		return domain.Job{}, err
	}
	if _, err := m.store.SaveSubscription(source); err != nil {
		m.releaseActive(sourceKey(domain.JobTargetSubscription, id))
		return domain.Job{}, err
	}

	m.dispatchWork(workItem{kind: workSubscriptionRefresh, id: id, ref: job.ID})
	return job, nil
}

func (m *Manager) CreateRuleSource(_ context.Context, input CreateRuleSourceInput) (domain.RuleSource, error) {
	if err := validateCreateRuleSourceInput(input); err != nil {
		return domain.RuleSource{}, err
	}

	now := time.Now().UTC()
	source := domain.RuleSource{
		ID:                  newID("rule"),
		Name:                input.Name,
		URL:                 input.URL,
		Mode:                normalizeRuleMode(input.Mode),
		Headers:             cloneHeaders(input.Headers),
		Enabled:             input.Enabled,
		TimeoutSec:          normalizeTimeout(input.TimeoutSec),
		UserAgent:           input.UserAgent,
		RetryAttempts:       normalizeNonNegative(input.RetryAttempts),
		RetryBackoffMS:      normalizeNonNegative(input.RetryBackoffMS),
		MinFetchIntervalSec: normalizeNonNegative(input.MinFetchIntervalSec),
		CacheTTLSeconds:     normalizeNonNegative(input.CacheTTLSeconds),
		RefreshIntervalSec:  normalizeNonNegative(input.RefreshIntervalSec),
		Status:              domain.RefreshStatusIdle,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	return m.store.SaveRuleSource(source)
}

func (m *Manager) ListRuleSources(_ context.Context) ([]domain.RuleSource, error) {
	return m.store.ListRuleSources()
}

func (m *Manager) GetRuleSource(_ context.Context, id string) (domain.RuleSource, error) {
	return m.store.GetRuleSource(id)
}

func (m *Manager) RefreshRuleSource(_ context.Context, id string) (domain.Job, error) {
	source, err := m.store.GetRuleSource(id)
	if err != nil {
		return domain.Job{}, err
	}
	if !source.Enabled {
		return domain.Job{}, fmt.Errorf("%w: rule source %q", ErrDisabled, source.Name)
	}

	job := domain.Job{
		ID:         newID("job"),
		Kind:       domain.JobKindRuleSourceRefresh,
		TargetType: domain.JobTargetRuleSource,
		TargetID:   id,
		Status:     domain.JobStatusQueued,
		CreatedAt:  time.Now().UTC(),
	}

	if err := m.claimActive(sourceKey(domain.JobTargetRuleSource, id), job.ID); err != nil {
		return domain.Job{}, err
	}

	source.Status = domain.RefreshStatusRunning
	source.LastError = ""
	source.CurrentJobID = job.ID
	source.UpdatedAt = time.Now().UTC()

	if _, err := m.store.SaveJob(job); err != nil {
		m.releaseActive(sourceKey(domain.JobTargetRuleSource, id))
		return domain.Job{}, err
	}
	if _, err := m.store.SaveRuleSource(source); err != nil {
		m.releaseActive(sourceKey(domain.JobTargetRuleSource, id))
		return domain.Job{}, err
	}

	m.dispatchWork(workItem{kind: workRuleSourceRefresh, id: id, ref: job.ID})
	return job, nil
}

func (m *Manager) GetJob(_ context.Context, id string) (domain.Job, error) {
	return m.store.GetJob(id)
}

func (m *Manager) SetSubscriptionSourceEnabled(_ context.Context, id string, enabled bool) (domain.SubscriptionSource, error) {
	source, err := m.store.GetSubscription(id)
	if err != nil {
		return domain.SubscriptionSource{}, err
	}
	source.Enabled = enabled
	source.UpdatedAt = time.Now().UTC()
	return m.store.SaveSubscription(source)
}

func (m *Manager) UpdateSubscriptionSource(_ context.Context, id string, input UpdateSubscriptionInput) (domain.SubscriptionSource, error) {
	source, err := m.store.GetSubscription(id)
	if err != nil {
		return domain.SubscriptionSource{}, err
	}
	if input.Name != nil {
		source.Name = strings.TrimSpace(*input.Name)
	}
	if input.Type != nil {
		source.Type = strings.TrimSpace(*input.Type)
	}
	if input.URL != nil {
		source.URL = strings.TrimSpace(*input.URL)
	}
	if input.Payload != nil {
		source.Payload = *input.Payload
	}
	if input.Headers != nil {
		source.Headers = cloneHeaders(*input.Headers)
	}
	if input.Enabled != nil {
		source.Enabled = *input.Enabled
	}
	if input.TimeoutSec != nil {
		source.TimeoutSec = normalizeTimeout(*input.TimeoutSec)
	}
	if input.UserAgent != nil {
		source.UserAgent = strings.TrimSpace(*input.UserAgent)
	}
	if input.RetryAttempts != nil {
		source.RetryAttempts = normalizeNonNegative(*input.RetryAttempts)
	}
	if input.RetryBackoffMS != nil {
		source.RetryBackoffMS = normalizeNonNegative(*input.RetryBackoffMS)
	}
	if input.MinFetchIntervalSec != nil {
		source.MinFetchIntervalSec = normalizeNonNegative(*input.MinFetchIntervalSec)
	}
	if input.CacheTTLSeconds != nil {
		source.CacheTTLSeconds = normalizeNonNegative(*input.CacheTTLSeconds)
	}
	if input.RefreshIntervalSec != nil {
		source.RefreshIntervalSec = normalizeNonNegative(*input.RefreshIntervalSec)
	}
	if err := validateSource(source.Name, source.Type, source.URL, source.Payload); err != nil {
		return domain.SubscriptionSource{}, err
	}
	source.UpdatedAt = time.Now().UTC()
	return m.store.SaveSubscription(source)
}

func (m *Manager) SetRuleSourceEnabled(_ context.Context, id string, enabled bool) (domain.RuleSource, error) {
	source, err := m.store.GetRuleSource(id)
	if err != nil {
		return domain.RuleSource{}, err
	}
	source.Enabled = enabled
	source.UpdatedAt = time.Now().UTC()
	return m.store.SaveRuleSource(source)
}

func (m *Manager) UpdateRuleSource(_ context.Context, id string, input UpdateRuleSourceInput) (domain.RuleSource, error) {
	source, err := m.store.GetRuleSource(id)
	if err != nil {
		return domain.RuleSource{}, err
	}
	if input.Name != nil {
		source.Name = strings.TrimSpace(*input.Name)
	}
	if input.URL != nil {
		source.URL = strings.TrimSpace(*input.URL)
	}
	if input.Mode != nil {
		source.Mode = normalizeRuleMode(*input.Mode)
	}
	if input.Headers != nil {
		source.Headers = cloneHeaders(*input.Headers)
	}
	if input.Enabled != nil {
		source.Enabled = *input.Enabled
	}
	if input.TimeoutSec != nil {
		source.TimeoutSec = normalizeTimeout(*input.TimeoutSec)
	}
	if input.UserAgent != nil {
		source.UserAgent = strings.TrimSpace(*input.UserAgent)
	}
	if input.RetryAttempts != nil {
		source.RetryAttempts = normalizeNonNegative(*input.RetryAttempts)
	}
	if input.RetryBackoffMS != nil {
		source.RetryBackoffMS = normalizeNonNegative(*input.RetryBackoffMS)
	}
	if input.MinFetchIntervalSec != nil {
		source.MinFetchIntervalSec = normalizeNonNegative(*input.MinFetchIntervalSec)
	}
	if input.CacheTTLSeconds != nil {
		source.CacheTTLSeconds = normalizeNonNegative(*input.CacheTTLSeconds)
	}
	if input.RefreshIntervalSec != nil {
		source.RefreshIntervalSec = normalizeNonNegative(*input.RefreshIntervalSec)
	}
	if err := validateRuleSource(source.Name, source.URL, source.Mode); err != nil {
		return domain.RuleSource{}, err
	}
	source.UpdatedAt = time.Now().UTC()
	return m.store.SaveRuleSource(source)
}

func (m *Manager) runSubscriptionRefresh(jobID, sourceID string) {
	defer m.releaseActive(sourceKey(domain.JobTargetSubscription, sourceID))

	job, err := m.store.GetJob(jobID)
	if err != nil {
		return
	}
	startedAt := time.Now().UTC()
	job.Status = domain.JobStatusRunning
	job.StartedAt = &startedAt
	_, _ = m.store.SaveJob(job)

	source, err := m.store.GetSubscription(sourceID)
	if err != nil {
		m.failSubscriptionJob(jobID, sourceID, fmt.Errorf("load subscription source: %w", err))
		return
	}

	var artifact fetcher.Artifact
	var parsed parser.SubscriptionParseResult
	var errFetch error
	finishedAt := time.Now().UTC()

	if source.Type == "local" {
		parsed, errFetch = parser.ParseClashMetaSubscription([]byte(source.Payload), source.ID, "local")
		if errFetch != nil {
			m.failSubscriptionJob(jobID, sourceID, errFetch)
			return
		}
		artifact.FetchedAt = finishedAt
		artifact.ContentType = "text/plain"
	} else {
		artifact, errFetch = m.fetcher.Fetch(context.Background(), fetcher.Request{
			URL:             source.URL,
			Headers:         source.Headers,
			UserAgent:       source.UserAgent,
			Timeout:         time.Duration(source.TimeoutSec) * time.Second,
			RetryAttempts:   source.RetryAttempts,
			RetryBackoff:    time.Duration(source.RetryBackoffMS) * time.Millisecond,
			CacheKey:        "subscription:" + source.ID + ":" + source.URL,
			CacheTTL:        time.Duration(source.CacheTTLSeconds) * time.Second,
			IfNoneMatch:     source.Snapshot.ETag,
			IfModifiedSince: source.Snapshot.LastModified,
			AllowStale:      true,
			RateLimitKey:    "subscription:" + source.ID,
			MinInterval:     time.Duration(source.MinFetchIntervalSec) * time.Second,
		})
		if errFetch != nil {
			m.failSubscriptionJob(jobID, sourceID, errFetch)
			return
		}

		finishedAt = time.Now().UTC()
		if artifact.NotModified {
			source.Status = domain.RefreshStatusSucceeded
			source.LastError = ""
			source.LastFetchedAt = &artifact.FetchedAt
			source.CurrentJobID = ""
			source.UpdatedAt = finishedAt
			_, _ = m.store.SaveSubscription(source)

			job.Status = domain.JobStatusSucceeded
			job.Error = ""
			job.FinishedAt = &finishedAt
			job.Summary = domain.JobSummary{
				RawProxyCount: len(source.Snapshot.RawProxies),
				ProxyCount:    len(source.Snapshot.Proxies),
				WarningCount:  len(source.Snapshot.Warnings),
			}
			_, _ = m.store.SaveJob(job)
			m.enqueueAutoBuildProfilesForSubscription(sourceID)
			return
		}

		parsed, errFetch = parser.ParseClashMetaSubscription(artifact.Body, source.ID, source.URL)
		if errFetch != nil {
			m.failSubscriptionJob(jobID, sourceID, errFetch)
			return
		}
	}

	source.Status = domain.RefreshStatusSucceeded
	source.LastError = ""
	source.LastFetchedAt = &artifact.FetchedAt
	source.CurrentJobID = ""
	source.Snapshot = domain.SubscriptionSnapshot{
		ContentType:  artifact.ContentType,
		ETag:         artifact.ETag,
		LastModified: artifact.LastModified,
		RawProxies:   parsed.RawProxies,
		Proxies:      parsed.Proxies,
		Warnings:     parsed.Warnings,
	}
	source.UpdatedAt = finishedAt
	_, _ = m.store.SaveSubscription(source)

	job.Status = domain.JobStatusSucceeded
	job.Error = ""
	job.FinishedAt = &finishedAt
	job.Summary = domain.JobSummary{
		RawProxyCount: len(parsed.RawProxies),
		ProxyCount:    len(parsed.Proxies),
		WarningCount:  len(parsed.Warnings),
	}
	_, _ = m.store.SaveJob(job)
	m.enqueueAutoBuildProfilesForSubscription(sourceID)
}

func (m *Manager) runRuleSourceRefresh(jobID, sourceID string) {
	defer m.releaseActive(sourceKey(domain.JobTargetRuleSource, sourceID))

	job, err := m.store.GetJob(jobID)
	if err != nil {
		return
	}
	startedAt := time.Now().UTC()
	job.Status = domain.JobStatusRunning
	job.StartedAt = &startedAt
	_, _ = m.store.SaveJob(job)

	source, err := m.store.GetRuleSource(sourceID)
	if err != nil {
		m.failRuleSourceJob(jobID, sourceID, fmt.Errorf("load rule source: %w", err))
		return
	}

	if source.Mode == domain.RuleSourceModeLinkOnly {
		finishedAt := time.Now().UTC()
		source.Status = domain.RefreshStatusSucceeded
		source.LastError = ""
		source.LastFetchedAt = &finishedAt
		source.CurrentJobID = ""
		source.Snapshot = domain.RuleSnapshot{
			IR: parser.BuildRuleReferenceIR(source.URL),
		}
		source.UpdatedAt = finishedAt
		_, _ = m.store.SaveRuleSource(source)

		job.Status = domain.JobStatusSucceeded
		job.Error = ""
		job.FinishedAt = &finishedAt
		job.Summary = domain.JobSummary{}
		_, _ = m.store.SaveJob(job)
		return
	}

	artifact, err := m.fetcher.Fetch(context.Background(), fetcher.Request{
		URL:             source.URL,
		Headers:         source.Headers,
		UserAgent:       source.UserAgent,
		Timeout:         time.Duration(source.TimeoutSec) * time.Second,
		RetryAttempts:   source.RetryAttempts,
		RetryBackoff:    time.Duration(source.RetryBackoffMS) * time.Millisecond,
		CacheKey:        "rule:" + source.ID + ":" + source.URL,
		CacheTTL:        time.Duration(source.CacheTTLSeconds) * time.Second,
		IfNoneMatch:     source.Snapshot.ETag,
		IfModifiedSince: source.Snapshot.LastModified,
		AllowStale:      true,
		RateLimitKey:    "rule:" + source.ID,
		MinInterval:     time.Duration(source.MinFetchIntervalSec) * time.Second,
	})
	if err != nil {
		m.failRuleSourceJob(jobID, sourceID, err)
		return
	}

	finishedAt := time.Now().UTC()
	if artifact.NotModified {
		source.Status = domain.RefreshStatusSucceeded
		source.LastError = ""
		source.LastFetchedAt = &artifact.FetchedAt
		source.CurrentJobID = ""
		source.UpdatedAt = finishedAt
		_, _ = m.store.SaveRuleSource(source)

		job.Status = domain.JobStatusSucceeded
		job.Error = ""
		job.FinishedAt = &finishedAt
		job.Summary = domain.JobSummary{
			RuleCount:    len(source.Snapshot.IR.Entries),
			WarningCount: len(source.Snapshot.Warnings),
		}
		_, _ = m.store.SaveJob(job)
		m.enqueueAutoBuildProfilesForRuleSource(sourceID)
		return
	}

	parsed, err := parser.ParseRuleText(artifact.Body, source.URL)
	if err != nil {
		m.failRuleSourceJob(jobID, sourceID, err)
		return
	}

	source.Status = domain.RefreshStatusSucceeded
	source.LastError = ""
	source.LastFetchedAt = &artifact.FetchedAt
	source.CurrentJobID = ""
	source.Snapshot = domain.RuleSnapshot{
		ContentType:  artifact.ContentType,
		ETag:         artifact.ETag,
		LastModified: artifact.LastModified,
		RawText:      string(artifact.Body),
		IR:           parsed.Document,
		Warnings:     parsed.Warnings,
	}
	source.UpdatedAt = finishedAt
	_, _ = m.store.SaveRuleSource(source)

	job.Status = domain.JobStatusSucceeded
	job.Error = ""
	job.FinishedAt = &finishedAt
	job.Summary = domain.JobSummary{
		RuleCount:    len(parsed.Document.Entries),
		WarningCount: len(parsed.Warnings),
	}
	_, _ = m.store.SaveJob(job)
	m.enqueueAutoBuildProfilesForRuleSource(sourceID)
}

func (m *Manager) failSubscriptionJob(jobID, sourceID string, err error) {
	finishedAt := time.Now().UTC()

	source, sourceErr := m.store.GetSubscription(sourceID)
	if sourceErr == nil {
		source.Status = domain.RefreshStatusFailed
		source.LastError = err.Error()
		source.CurrentJobID = ""
		source.UpdatedAt = finishedAt
		_, _ = m.store.SaveSubscription(source)
	}

	job, jobErr := m.store.GetJob(jobID)
	if jobErr == nil {
		job.Status = domain.JobStatusFailed
		job.Error = err.Error()
		job.FinishedAt = &finishedAt
		_, _ = m.store.SaveJob(job)
	}
}

func (m *Manager) failRuleSourceJob(jobID, sourceID string, err error) {
	finishedAt := time.Now().UTC()

	source, sourceErr := m.store.GetRuleSource(sourceID)
	if sourceErr == nil {
		source.Status = domain.RefreshStatusFailed
		source.LastError = err.Error()
		source.CurrentJobID = ""
		source.UpdatedAt = finishedAt
		_, _ = m.store.SaveRuleSource(source)
	}

	job, jobErr := m.store.GetJob(jobID)
	if jobErr == nil {
		job.Status = domain.JobStatusFailed
		job.Error = err.Error()
		job.FinishedAt = &finishedAt
		_, _ = m.store.SaveJob(job)
	}
}

func (m *Manager) claimActive(key, jobID string) error {
	m.activeMu.Lock()
	defer m.activeMu.Unlock()

	if _, exists := m.active[key]; exists {
		return ErrAlreadyRunning
	}
	m.active[key] = jobID
	return nil
}

func (m *Manager) releaseActive(key string) {
	m.activeMu.Lock()
	defer m.activeMu.Unlock()
	delete(m.active, key)
}

func validateCreateSubscriptionInput(input CreateSubscriptionInput) error {
	return validateSource(input.Name, input.Type, input.URL, input.Payload)
}

func validateCreateRuleSourceInput(input CreateRuleSourceInput) error {
	return validateRuleSource(input.Name, input.URL, input.Mode)
}

func validateSource(name, srcType, rawURL, payload string) error {
	if stringsTrim(name) == "" {
		return fmt.Errorf("%w: subscription name is required", ErrInvalidInput)
	}
	if srcType == "local" {
		if stringsTrim(payload) == "" {
			return fmt.Errorf("%w: payload is required for local source", ErrInvalidInput)
		}
	} else {
		if err := validateURL(rawURL); err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidInput, err)
		}
	}
	return nil
}

func validateRuleSource(name, rawURL string, mode domain.RuleSourceMode) error {
	if stringsTrim(name) == "" {
		return fmt.Errorf("%w: rule source name is required", ErrInvalidInput)
	}
	if err := validateURL(rawURL); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}
	normalized := normalizeRuleMode(mode)
	if normalized != domain.RuleSourceModeLinkOnly && normalized != domain.RuleSourceModeFetchText {
		return fmt.Errorf("%w: unsupported rule source mode %q", ErrInvalidInput, mode)
	}
	return nil
}

func validateURL(raw string) error {
	parsed, err := url.ParseRequestURI(raw)
	if err != nil {
		return fmt.Errorf("invalid url")
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("invalid url")
	}
	return nil
}

func normalizeTimeout(timeoutSec int) int {
	if timeoutSec <= 0 {
		return 15
	}
	return timeoutSec
}

func normalizeRuleMode(mode domain.RuleSourceMode) domain.RuleSourceMode {
	if mode == "" {
		return domain.RuleSourceModeLinkOnly
	}
	return mode
}

func normalizeNonNegative(value int) int {
	if value < 0 {
		return 0
	}
	return value
}

func sourceKey(targetType domain.JobTargetType, id string) string {
	return string(targetType) + ":" + id
}

func cloneHeaders(headers map[string]string) map[string]string {
	if len(headers) == 0 {
		return nil
	}
	out := make(map[string]string, len(headers))
	for key, value := range headers {
		out[key] = value
	}
	return out
}

func newID(prefix string) string {
	var bytes [6]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		now := time.Now().UnixNano()
		return fmt.Sprintf("%s_%d", prefix, now)
	}
	return prefix + "_" + hex.EncodeToString(bytes[:])
}

func stringsTrim(value string) string {
	return strings.TrimSpace(value)
}

func (m *Manager) CreateSystemAlert(level domain.AlertLevel, message string) {
	alert := domain.SystemAlert{
		ID:        newID("alrt"),
		Level:     level,
		Message:   message,
		CreatedAt: time.Now().UTC(),
	}
	_, _ = m.store.SaveSystemAlert(alert)
}

func (m *Manager) DeleteSubscriptionSource(ctx context.Context, id string) error {
	profiles, _ := m.store.ListBuildProfiles()
	for _, p := range profiles {
		for _, sID := range p.SubscriptionSourceIDs {
			if sID == id {
				m.CreateSystemAlert(domain.AlertLevelError, fmt.Sprintf("Build Profile '%s' lost dependency: deleted Subscription '%s'", p.Name, id))
			}
		}
	}
	return m.store.DeleteSubscriptionSource(id)
}

func (m *Manager) DeleteRuleSource(ctx context.Context, id string) error {
	profiles, _ := m.store.ListBuildProfiles()
	for _, p := range profiles {
		for _, rb := range p.RuleBindings {
			if rb.RuleSourceID == id {
				m.CreateSystemAlert(domain.AlertLevelError, fmt.Sprintf("Build Profile '%s' lost dependency: deleted Rule '%s'", p.Name, id))
			}
		}
	}
	return m.store.DeleteRuleSource(id)
}

func (m *Manager) DeleteBuildProfile(ctx context.Context, id string) error {
	tokens, _ := m.store.ListDownloadTokens()
	for _, t := range tokens {
		if t.BuildProfileID == id {
			m.CreateSystemAlert(domain.AlertLevelError, fmt.Sprintf("Token '%s' lost dependency: deleted Build Profile '%s'", t.Name, id))
		}
	}
	_ = m.store.DeleteBuildRunsByProfile(id)
	_ = m.store.DeleteBuildArtifactsByProfile(id)
	return m.store.DeleteBuildProfile(id)
}

func (m *Manager) DeleteDownloadToken(ctx context.Context, id string) error {
	return m.store.DeleteDownloadToken(id)
}

func (m *Manager) ListSystemAlerts(ctx context.Context) ([]domain.SystemAlert, error) {
	return m.store.ListSystemAlerts()
}

func (m *Manager) ClearSystemAlerts(ctx context.Context) error {
	return m.store.ClearSystemAlerts()
}
