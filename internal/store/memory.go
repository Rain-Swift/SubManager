package store

import (
	"errors"
	"sort"
	"sync"

	"submanager/internal/domain"
)

var ErrNotFound = errors.New("store: not found")

type Store interface {
	SaveSubscription(domain.SubscriptionSource) (domain.SubscriptionSource, error)
	GetSubscription(string) (domain.SubscriptionSource, error)
	ListSubscriptions() ([]domain.SubscriptionSource, error)

	SaveRuleSource(domain.RuleSource) (domain.RuleSource, error)
	GetRuleSource(string) (domain.RuleSource, error)
	ListRuleSources() ([]domain.RuleSource, error)

	SaveJob(domain.Job) (domain.Job, error)
	GetJob(string) (domain.Job, error)

	SaveBuildProfile(domain.BuildProfile) (domain.BuildProfile, error)
	GetBuildProfile(string) (domain.BuildProfile, error)
	ListBuildProfiles() ([]domain.BuildProfile, error)

	SaveBuildRun(domain.BuildRun) (domain.BuildRun, error)
	GetBuildRun(string) (domain.BuildRun, error)

	SaveBuildArtifact(domain.BuildArtifact) (domain.BuildArtifact, error)
	GetBuildArtifact(string) (domain.BuildArtifact, error)

	SaveDownloadToken(domain.DownloadTokenRecord) (domain.DownloadTokenRecord, error)
	GetDownloadToken(string) (domain.DownloadTokenRecord, error)
	FindDownloadTokenByHash(string) (domain.DownloadTokenRecord, error)
	ListDownloadTokens() ([]domain.DownloadTokenRecord, error)

	DeleteSubscriptionSource(string) error
	DeleteRuleSource(string) error
	DeleteBuildProfile(string) error
	DeleteBuildArtifactsByProfile(string) error
	DeleteBuildRunsByProfile(string) error
	DeleteDownloadToken(string) error

	SaveSystemAlert(domain.SystemAlert) (domain.SystemAlert, error)
	ListSystemAlerts() ([]domain.SystemAlert, error)
	ClearSystemAlerts() error
}

type MemoryStore struct {
	mu             sync.RWMutex
	subscriptions  map[string]domain.SubscriptionSource
	ruleSources    map[string]domain.RuleSource
	jobs           map[string]domain.Job
	buildProfiles  map[string]domain.BuildProfile
	buildRuns      map[string]domain.BuildRun
	buildArtifacts map[string]domain.BuildArtifact
	downloadTokens map[string]domain.DownloadTokenRecord
	systemAlerts   map[string]domain.SystemAlert
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		subscriptions:  make(map[string]domain.SubscriptionSource),
		ruleSources:    make(map[string]domain.RuleSource),
		jobs:           make(map[string]domain.Job),
		buildProfiles:  make(map[string]domain.BuildProfile),
		buildRuns:      make(map[string]domain.BuildRun),
		buildArtifacts: make(map[string]domain.BuildArtifact),
		downloadTokens: make(map[string]domain.DownloadTokenRecord),
		systemAlerts:   make(map[string]domain.SystemAlert),
	}
}

func (m *MemoryStore) SaveSubscription(src domain.SubscriptionSource) (domain.SubscriptionSource, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	copyValue := src.Clone()
	m.subscriptions[src.ID] = copyValue
	return copyValue.Clone(), nil
}

func (m *MemoryStore) GetSubscription(id string) (domain.SubscriptionSource, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	src, ok := m.subscriptions[id]
	if !ok {
		return domain.SubscriptionSource{}, ErrNotFound
	}
	return src.Clone(), nil
}

func (m *MemoryStore) ListSubscriptions() ([]domain.SubscriptionSource, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]domain.SubscriptionSource, 0, len(m.subscriptions))
	for _, src := range m.subscriptions {
		out = append(out, src.Clone())
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out, nil
}

func (m *MemoryStore) SaveRuleSource(src domain.RuleSource) (domain.RuleSource, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	copyValue := src.Clone()
	m.ruleSources[src.ID] = copyValue
	return copyValue.Clone(), nil
}

func (m *MemoryStore) GetRuleSource(id string) (domain.RuleSource, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	src, ok := m.ruleSources[id]
	if !ok {
		return domain.RuleSource{}, ErrNotFound
	}
	return src.Clone(), nil
}

func (m *MemoryStore) ListRuleSources() ([]domain.RuleSource, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]domain.RuleSource, 0, len(m.ruleSources))
	for _, src := range m.ruleSources {
		out = append(out, src.Clone())
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out, nil
}

func (m *MemoryStore) SaveJob(job domain.Job) (domain.Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	copyValue := job.Clone()
	m.jobs[job.ID] = copyValue
	return copyValue.Clone(), nil
}

func (m *MemoryStore) GetJob(id string) (domain.Job, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	job, ok := m.jobs[id]
	if !ok {
		return domain.Job{}, ErrNotFound
	}
	return job.Clone(), nil
}

func (m *MemoryStore) SaveBuildProfile(profile domain.BuildProfile) (domain.BuildProfile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	copyValue := profile.Clone()
	m.buildProfiles[profile.ID] = copyValue
	return copyValue.Clone(), nil
}

func (m *MemoryStore) GetBuildProfile(id string) (domain.BuildProfile, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	profile, ok := m.buildProfiles[id]
	if !ok {
		return domain.BuildProfile{}, ErrNotFound
	}
	return profile.Clone(), nil
}

func (m *MemoryStore) ListBuildProfiles() ([]domain.BuildProfile, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]domain.BuildProfile, 0, len(m.buildProfiles))
	for _, item := range m.buildProfiles {
		out = append(out, item.Clone())
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out, nil
}

func (m *MemoryStore) SaveBuildRun(run domain.BuildRun) (domain.BuildRun, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	copyValue := run.Clone()
	m.buildRuns[run.ID] = copyValue
	return copyValue.Clone(), nil
}

func (m *MemoryStore) GetBuildRun(id string) (domain.BuildRun, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	run, ok := m.buildRuns[id]
	if !ok {
		return domain.BuildRun{}, ErrNotFound
	}
	return run.Clone(), nil
}

func (m *MemoryStore) SaveBuildArtifact(artifact domain.BuildArtifact) (domain.BuildArtifact, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	copyValue := artifact.Clone()
	m.buildArtifacts[artifact.ID] = copyValue
	return copyValue.Clone(), nil
}

func (m *MemoryStore) GetBuildArtifact(id string) (domain.BuildArtifact, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	artifact, ok := m.buildArtifacts[id]
	if !ok {
		return domain.BuildArtifact{}, ErrNotFound
	}
	return artifact.Clone(), nil
}

func (m *MemoryStore) SaveDownloadToken(token domain.DownloadTokenRecord) (domain.DownloadTokenRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	copyValue := token.Clone()
	m.downloadTokens[token.ID] = copyValue
	return copyValue.Clone(), nil
}

func (m *MemoryStore) GetDownloadToken(id string) (domain.DownloadTokenRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	token, ok := m.downloadTokens[id]
	if !ok {
		return domain.DownloadTokenRecord{}, ErrNotFound
	}
	return token.Clone(), nil
}

func (m *MemoryStore) FindDownloadTokenByHash(hash string) (domain.DownloadTokenRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, token := range m.downloadTokens {
		if token.TokenHash == hash {
			return token.Clone(), nil
		}
	}
	return domain.DownloadTokenRecord{}, ErrNotFound
}

func (m *MemoryStore) ListDownloadTokens() ([]domain.DownloadTokenRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]domain.DownloadTokenRecord, 0, len(m.downloadTokens))
	for _, token := range m.downloadTokens {
		out = append(out, token.Clone())
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out, nil
}

func (m *MemoryStore) DeleteSubscriptionSource(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.subscriptions, id)
	return nil
}

func (m *MemoryStore) DeleteRuleSource(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.ruleSources, id)
	return nil
}

func (m *MemoryStore) DeleteBuildProfile(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.buildProfiles, id)
	return nil
}

func (m *MemoryStore) DeleteBuildArtifactsByProfile(profileID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, v := range m.buildArtifacts {
		if v.ProfileID == profileID {
			delete(m.buildArtifacts, k)
		}
	}
	return nil
}

func (m *MemoryStore) DeleteBuildRunsByProfile(profileID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, v := range m.buildRuns {
		if v.ProfileID == profileID {
			delete(m.buildRuns, k)
		}
	}
	return nil
}

func (m *MemoryStore) DeleteDownloadToken(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.downloadTokens, id)
	return nil
}

func (m *MemoryStore) SaveSystemAlert(alert domain.SystemAlert) (domain.SystemAlert, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.systemAlerts[alert.ID] = alert
	return alert, nil
}

func (m *MemoryStore) ListSystemAlerts() ([]domain.SystemAlert, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]domain.SystemAlert, 0, len(m.systemAlerts))
	for _, a := range m.systemAlerts {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out, nil
}

func (m *MemoryStore) ClearSystemAlerts() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.systemAlerts = make(map[string]domain.SystemAlert)
	return nil
}
