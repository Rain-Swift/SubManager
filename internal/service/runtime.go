package service

import (
	"context"
	"errors"
	"log"
	"time"

	"submanager/internal/domain"
	"submanager/internal/store"
)

func (m *Manager) Start(ctx context.Context) {
	m.startOnce.Do(func() {
		m.workQueue = make(chan workItem, 128)
		m.started.Store(true)

		for worker := 0; worker < m.workerCount; worker++ {
			go m.runWorker(ctx)
		}
		go m.runScheduler(ctx)
		go m.recoverRuntimeState(ctx)
	})
}

func (m *Manager) dispatchWork(item workItem) {
	if !m.started.Load() || m.workQueue == nil {
		go m.runWorkItem(context.Background(), item)
		return
	}

	select {
	case m.workQueue <- item:
	default:
		// When the queue is full, spawn a goroutine to wait and enqueue.
		// This prevents blocking the caller (API or Scheduler) and avoids deadlocks
		// if a worker enqueues another job. It also ensures the number of ACTIVE
		// concurrent jobs does not exceed the worker pool, preventing CPU/Memory spikes.
		go func() {
			m.workQueue <- item
		}()
	}
}

func (m *Manager) runWorker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case item := <-m.workQueue:
			m.runWorkItem(ctx, item)
		}
	}
}

func (m *Manager) runWorkItem(_ context.Context, item workItem) {
	switch item.kind {
	case workSubscriptionRefresh:
		m.runSubscriptionRefresh(item.ref, item.id)
	case workRuleSourceRefresh:
		m.runRuleSourceRefresh(item.ref, item.id)
	case workBuildProfile:
		m.runBuildProfile(item.ref, item.id)
	case workDownloadTokenWarm:
		if _, err := m.buildDownloadTokenCache(context.Background(), item.id); err != nil && !errors.Is(err, store.ErrNotFound) {
			log.Printf("download token warmup failed for %s: %v", item.id, err)
		}
	}
}

func (m *Manager) runScheduler(ctx context.Context) {
	if m.schedulerEvery <= 0 {
		return
	}

	ticker := time.NewTicker(m.schedulerEvery)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.scheduleRefreshes(context.Background())
			m.scheduleBuilds(context.Background())
			m.scheduleTokenWarmups(context.Background())
		}
	}
}

func (m *Manager) recoverRuntimeState(_ context.Context) {
	subscriptions, _ := m.store.ListSubscriptions()
	for _, source := range subscriptions {
		if source.CurrentJobID == "" {
			continue
		}
		job, err := m.store.GetJob(source.CurrentJobID)
		if err != nil {
			m.markSubscriptionRecovered(source.ID)
			continue
		}
		if job.Status == domain.JobStatusQueued || job.Status == domain.JobStatusRunning {
			if err := m.claimActive(sourceKey(domain.JobTargetSubscription, source.ID), job.ID); err == nil {
				m.dispatchWork(workItem{kind: workSubscriptionRefresh, id: source.ID, ref: job.ID})
			}
			continue
		}
		m.markSubscriptionRecovered(source.ID)
	}

	ruleSources, _ := m.store.ListRuleSources()
	for _, source := range ruleSources {
		if source.CurrentJobID == "" {
			continue
		}
		job, err := m.store.GetJob(source.CurrentJobID)
		if err != nil {
			m.markRuleSourceRecovered(source.ID)
			continue
		}
		if job.Status == domain.JobStatusQueued || job.Status == domain.JobStatusRunning {
			if err := m.claimActive(sourceKey(domain.JobTargetRuleSource, source.ID), job.ID); err == nil {
				m.dispatchWork(workItem{kind: workRuleSourceRefresh, id: source.ID, ref: job.ID})
			}
			continue
		}
		m.markRuleSourceRecovered(source.ID)
	}

	profiles, _ := m.store.ListBuildProfiles()
	for _, profile := range profiles {
		if profile.CurrentRunID == "" {
			continue
		}
		run, err := m.store.GetBuildRun(profile.CurrentRunID)
		if err != nil {
			m.markBuildProfileRecovered(profile.ID)
			continue
		}
		if run.Status == domain.BuildRunStatusQueued || run.Status == domain.BuildRunStatusRunning {
			if err := m.claimActive(buildProfileKey(profile.ID), run.ID); err == nil {
				m.dispatchWork(workItem{kind: workBuildProfile, id: profile.ID, ref: run.ID})
			}
			continue
		}
		m.markBuildProfileRecovered(profile.ID)
	}

	m.scheduleRefreshes(context.Background())
	m.scheduleBuilds(context.Background())
	m.scheduleTokenWarmups(context.Background())
}

func (m *Manager) scheduleRefreshes(ctx context.Context) {
	now := time.Now().UTC()
	subscriptions, err := m.store.ListSubscriptions()
	if err == nil {
		for _, source := range subscriptions {
			if !source.Enabled || source.RefreshIntervalSec <= 0 || source.CurrentJobID != "" {
				continue
			}
			if isDue(now, source.LastFetchedAt, source.CreatedAt, time.Duration(source.RefreshIntervalSec)*time.Second) {
				_, _ = m.RefreshSubscriptionSource(ctx, source.ID)
			}
		}
	}

	ruleSources, err := m.store.ListRuleSources()
	if err == nil {
		for _, source := range ruleSources {
			if !source.Enabled || source.RefreshIntervalSec <= 0 || source.CurrentJobID != "" {
				continue
			}
			if isDue(now, source.LastFetchedAt, source.CreatedAt, time.Duration(source.RefreshIntervalSec)*time.Second) {
				_, _ = m.RefreshRuleSource(ctx, source.ID)
			}
		}
	}
}

func (m *Manager) scheduleBuilds(ctx context.Context) {
	now := time.Now().UTC()
	profiles, err := m.store.ListBuildProfiles()
	if err != nil {
		return
	}
	for _, profile := range profiles {
		if !profile.Enabled || !profile.AutoBuild || profile.BuildIntervalSec <= 0 || profile.CurrentRunID != "" {
			continue
		}
		if isDue(now, profile.LastBuiltAt, profile.CreatedAt, time.Duration(profile.BuildIntervalSec)*time.Second) {
			_, _ = m.RunBuildProfile(ctx, profile.ID)
		}
	}
}

func (m *Manager) scheduleTokenWarmups(ctx context.Context) {
	tokens, err := m.store.ListDownloadTokens()
	if err != nil {
		return
	}
	for _, token := range tokens {
		if !token.Enabled || !token.Prebuild {
			continue
		}
		if token.CachedArtifact == nil {
			m.dispatchWork(workItem{kind: workDownloadTokenWarm, id: token.ID})
			continue
		}

		profile, err := m.store.GetBuildProfile(token.BuildProfileID)
		if err != nil {
			continue
		}
		if profile.LastBuiltAt != nil && token.CachedArtifact.LastBuiltAt.Before(*profile.LastBuiltAt) {
			m.dispatchWork(workItem{kind: workDownloadTokenWarm, id: token.ID})
			continue
		}
		if token.CachedArtifact.LastBuiltAt.Before(token.UpdatedAt) {
			m.dispatchWork(workItem{kind: workDownloadTokenWarm, id: token.ID})
		}
	}
	_ = ctx
}

func (m *Manager) enqueueAutoBuildProfilesForSubscription(sourceID string) {
	profiles, err := m.store.ListBuildProfiles()
	if err != nil {
		return
	}
	for _, profile := range profiles {
		if !profile.Enabled || !profile.AutoBuild {
			continue
		}
		if containsStringValue(profile.SubscriptionSourceIDs, sourceID) {
			_, _ = m.RunBuildProfile(context.Background(), profile.ID)
		}
	}
}

func (m *Manager) enqueueAutoBuildProfilesForRuleSource(ruleSourceID string) {
	profiles, err := m.store.ListBuildProfiles()
	if err != nil {
		return
	}
	for _, profile := range profiles {
		if !profile.Enabled || !profile.AutoBuild {
			continue
		}
		for _, binding := range profile.RuleBindings {
			if binding.RuleSourceID == ruleSourceID {
				_, _ = m.RunBuildProfile(context.Background(), profile.ID)
				break
			}
		}
	}
}

func (m *Manager) enqueuePrebuildTokensForProfile(profileID string) {
	tokens, err := m.store.ListDownloadTokens()
	if err != nil {
		return
	}
	for _, token := range tokens {
		if token.Enabled && token.Prebuild && token.BuildProfileID == profileID {
			m.dispatchWork(workItem{kind: workDownloadTokenWarm, id: token.ID})
		}
	}
}

func (m *Manager) markSubscriptionRecovered(id string) {
	source, err := m.store.GetSubscription(id)
	if err != nil {
		return
	}
	if source.CurrentJobID == "" {
		return
	}
	source.CurrentJobID = ""
	if source.Status == domain.RefreshStatusRunning {
		source.Status = domain.RefreshStatusFailed
		source.LastError = "recovered from interrupted worker"
	}
	source.UpdatedAt = time.Now().UTC()
	_, _ = m.store.SaveSubscription(source)
}

func (m *Manager) markRuleSourceRecovered(id string) {
	source, err := m.store.GetRuleSource(id)
	if err != nil {
		return
	}
	if source.CurrentJobID == "" {
		return
	}
	source.CurrentJobID = ""
	if source.Status == domain.RefreshStatusRunning {
		source.Status = domain.RefreshStatusFailed
		source.LastError = "recovered from interrupted worker"
	}
	source.UpdatedAt = time.Now().UTC()
	_, _ = m.store.SaveRuleSource(source)
}

func (m *Manager) markBuildProfileRecovered(id string) {
	profile, err := m.store.GetBuildProfile(id)
	if err != nil {
		return
	}
	if profile.CurrentRunID == "" {
		return
	}
	profile.CurrentRunID = ""
	if profile.Status == domain.RefreshStatusRunning {
		profile.Status = domain.RefreshStatusFailed
		profile.LastError = "recovered from interrupted worker"
	}
	profile.UpdatedAt = time.Now().UTC()
	_, _ = m.store.SaveBuildProfile(profile)
}

func isDue(now time.Time, last *time.Time, createdAt time.Time, interval time.Duration) bool {
	if interval <= 0 {
		return false
	}
	base := createdAt
	if last != nil && !last.IsZero() {
		base = *last
	}
	return now.Sub(base) >= interval
}

func containsStringValue(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
