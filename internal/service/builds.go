package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"time"

	buildengine "submanager/internal/build"
	"submanager/internal/domain"
)

type CreateBuildProfileInput struct {
	Name                  string
	Description           string
	SubscriptionSourceIDs []string
	RuleBindings          []domain.BuildRuleBinding
	Template              domain.BuildTemplate
	Filters               []domain.ProxyFilterRule
	Renames               []domain.RenameRule
	Groups                []domain.ProxyGroupSpec
	DefaultGroup          string
	Enabled               bool
	AutoBuild             bool
	BuildIntervalSec      int
}

type UpdateBuildProfileInput struct {
	Name                  *string
	Description           *string
	SubscriptionSourceIDs *[]string
	RuleBindings          *[]domain.BuildRuleBinding
	Template              *domain.BuildTemplate
	Filters               *[]domain.ProxyFilterRule
	Renames               *[]domain.RenameRule
	Groups                *[]domain.ProxyGroupSpec
	DefaultGroup          *string
	Enabled               *bool
	AutoBuild             *bool
	BuildIntervalSec      *int
}

func (m *Manager) CreateBuildProfile(_ context.Context, input CreateBuildProfileInput) (domain.BuildProfile, error) {
	if err := validateCreateBuildProfileInput(input); err != nil {
		return domain.BuildProfile{}, err
	}
	if err := m.validateBuildSourceReferences(input.SubscriptionSourceIDs, input.RuleBindings); err != nil {
		return domain.BuildProfile{}, err
	}

	now := time.Now().UTC()
	profile := domain.BuildProfile{
		ID:                    newID("profile"),
		Name:                  strings.TrimSpace(input.Name),
		Description:           strings.TrimSpace(input.Description),
		SubscriptionSourceIDs: append([]string(nil), input.SubscriptionSourceIDs...),
		RuleBindings:          cloneRuleBindings(input.RuleBindings),
		Template:              input.Template.Clone(),
		Filters:               cloneFilterRules(input.Filters),
		Renames:               cloneRenameRules(input.Renames),
		Groups:                cloneGroupSpecs(input.Groups),
		DefaultGroup:          strings.TrimSpace(input.DefaultGroup),
		Enabled:               input.Enabled,
		AutoBuild:             input.AutoBuild,
		BuildIntervalSec:      normalizeNonNegative(input.BuildIntervalSec),
		Status:                domain.RefreshStatusIdle,
		CreatedAt:             now,
		UpdatedAt:             now,
	}
	return m.store.SaveBuildProfile(profile)
}

func (m *Manager) ListBuildProfiles(_ context.Context) ([]domain.BuildProfile, error) {
	return m.store.ListBuildProfiles()
}

func (m *Manager) GetBuildProfile(_ context.Context, id string) (domain.BuildProfile, error) {
	return m.store.GetBuildProfile(id)
}

func (m *Manager) SetBuildProfileEnabled(_ context.Context, id string, enabled bool) (domain.BuildProfile, error) {
	profile, err := m.store.GetBuildProfile(id)
	if err != nil {
		return domain.BuildProfile{}, err
	}
	profile.Enabled = enabled
	profile.UpdatedAt = time.Now().UTC()
	return m.store.SaveBuildProfile(profile)
}

func (m *Manager) UpdateBuildProfile(_ context.Context, id string, input UpdateBuildProfileInput) (domain.BuildProfile, error) {
	profile, err := m.store.GetBuildProfile(id)
	if err != nil {
		return domain.BuildProfile{}, err
	}
	if input.Name != nil {
		profile.Name = strings.TrimSpace(*input.Name)
	}
	if input.Description != nil {
		profile.Description = strings.TrimSpace(*input.Description)
	}
	if input.SubscriptionSourceIDs != nil {
		profile.SubscriptionSourceIDs = append([]string(nil), *input.SubscriptionSourceIDs...)
	}
	if input.RuleBindings != nil {
		profile.RuleBindings = cloneRuleBindings(*input.RuleBindings)
	}
	if input.Template != nil {
		profile.Template = input.Template.Clone()
	}
	if input.Filters != nil {
		profile.Filters = cloneFilterRules(*input.Filters)
	}
	if input.Renames != nil {
		profile.Renames = cloneRenameRules(*input.Renames)
	}
	if input.Groups != nil {
		profile.Groups = cloneGroupSpecs(*input.Groups)
	}
	if input.DefaultGroup != nil {
		profile.DefaultGroup = strings.TrimSpace(*input.DefaultGroup)
	}
	if input.Enabled != nil {
		profile.Enabled = *input.Enabled
	}
	if input.AutoBuild != nil {
		profile.AutoBuild = *input.AutoBuild
	}
	if input.BuildIntervalSec != nil {
		profile.BuildIntervalSec = normalizeNonNegative(*input.BuildIntervalSec)
	}

	if err := validateCreateBuildProfileInput(CreateBuildProfileInput{
		Name:                  profile.Name,
		Description:           profile.Description,
		SubscriptionSourceIDs: profile.SubscriptionSourceIDs,
		RuleBindings:          profile.RuleBindings,
		Template:              profile.Template,
		Filters:               profile.Filters,
		Renames:               profile.Renames,
		Groups:                profile.Groups,
		DefaultGroup:          profile.DefaultGroup,
		Enabled:               profile.Enabled,
		AutoBuild:             profile.AutoBuild,
		BuildIntervalSec:      profile.BuildIntervalSec,
	}); err != nil {
		return domain.BuildProfile{}, err
	}
	if err := m.validateBuildSourceReferences(profile.SubscriptionSourceIDs, profile.RuleBindings); err != nil {
		return domain.BuildProfile{}, err
	}

	profile.UpdatedAt = time.Now().UTC()
	return m.store.SaveBuildProfile(profile)
}

func (m *Manager) PreviewBuildProfile(_ context.Context, id string) (domain.BuildPreview, error) {
	profile, err := m.store.GetBuildProfile(id)
	if err != nil {
		return domain.BuildPreview{}, err
	}

	subscriptions, ruleSources, err := m.loadBuildInputs(profile)
	if err != nil {
		return domain.BuildPreview{}, err
	}

	preview, err := buildengine.Execute(profile, subscriptions, ruleSources)
	if err != nil {
		return domain.BuildPreview{}, err
	}
	return preview, nil
}

func (m *Manager) RunBuildProfile(_ context.Context, profileID string) (domain.BuildRun, error) {
	profile, err := m.store.GetBuildProfile(profileID)
	if err != nil {
		return domain.BuildRun{}, err
	}
	if !profile.Enabled {
		return domain.BuildRun{}, fmt.Errorf("%w: build profile %q", ErrDisabled, profile.Name)
	}

	run := domain.BuildRun{
		ID:        newID("build"),
		ProfileID: profileID,
		Status:    domain.BuildRunStatusQueued,
		CreatedAt: time.Now().UTC(),
	}

	if err := m.claimActive(buildProfileKey(profileID), run.ID); err != nil {
		return domain.BuildRun{}, err
	}

	profile.Status = domain.RefreshStatusRunning
	profile.LastError = ""
	profile.CurrentRunID = run.ID
	profile.UpdatedAt = time.Now().UTC()

	if _, err := m.store.SaveBuildRun(run); err != nil {
		m.releaseActive(buildProfileKey(profileID))
		return domain.BuildRun{}, err
	}
	if _, err := m.store.SaveBuildProfile(profile); err != nil {
		m.releaseActive(buildProfileKey(profileID))
		return domain.BuildRun{}, err
	}

	m.dispatchWork(workItem{kind: workBuildProfile, id: profileID, ref: run.ID})
	return run, nil
}

func (m *Manager) GetBuildRun(_ context.Context, id string) (domain.BuildRun, error) {
	return m.store.GetBuildRun(id)
}

func (m *Manager) GetBuildArtifact(_ context.Context, id string) (domain.BuildArtifact, error) {
	return m.store.GetBuildArtifact(id)
}

func (m *Manager) runBuildProfile(runID, profileID string) {
	defer m.releaseActive(buildProfileKey(profileID))

	run, err := m.store.GetBuildRun(runID)
	if err != nil {
		return
	}
	startedAt := time.Now().UTC()
	run.Status = domain.BuildRunStatusRunning
	run.StartedAt = &startedAt
	_, _ = m.store.SaveBuildRun(run)

	profile, err := m.store.GetBuildProfile(profileID)
	if err != nil {
		m.failBuildRun(runID, profileID, fmt.Errorf("load build profile: %w", err))
		return
	}

	subscriptions, ruleSources, err := m.loadBuildInputs(profile)
	if err != nil {
		m.failBuildRun(runID, profileID, err)
		return
	}

	preview, err := buildengine.Execute(profile, subscriptions, ruleSources)
	if err != nil {
		m.failBuildRun(runID, profileID, err)
		return
	}

	finishedAt := time.Now().UTC()
	contentBytes := []byte(preview.YAML)
	hash := sha256.Sum256(contentBytes)
	artifact := domain.BuildArtifact{
		ID:        newID("artifact"),
		ProfileID: profileID,
		RunID:     runID,
		FileName:  buildArtifactFileName(profile.Name),
		Content:   preview.YAML,
		SHA256:    hex.EncodeToString(hash[:]),
		Summary:   preview.Summary,
		CreatedAt: finishedAt,
	}

	if _, err := m.store.SaveBuildArtifact(artifact); err != nil {
		m.failBuildRun(runID, profileID, err)
		return
	}

	run.Status = domain.BuildRunStatusSucceeded
	run.Error = ""
	run.ArtifactID = artifact.ID
	run.Summary = preview.Summary
	run.FinishedAt = &finishedAt
	_, _ = m.store.SaveBuildRun(run)

	profile.Status = domain.RefreshStatusSucceeded
	profile.LastError = ""
	profile.LastBuiltAt = &finishedAt
	profile.CurrentRunID = ""
	profile.LastRunID = run.ID
	profile.LastArtifactID = artifact.ID
	profile.UpdatedAt = finishedAt
	_, _ = m.store.SaveBuildProfile(profile)
	m.enqueuePrebuildTokensForProfile(profileID)
}

func (m *Manager) failBuildRun(runID, profileID string, err error) {
	finishedAt := time.Now().UTC()

	run, runErr := m.store.GetBuildRun(runID)
	if runErr == nil {
		run.Status = domain.BuildRunStatusFailed
		run.Error = err.Error()
		run.FinishedAt = &finishedAt
		_, _ = m.store.SaveBuildRun(run)
	}

	profile, profileErr := m.store.GetBuildProfile(profileID)
	if profileErr == nil {
		profile.Status = domain.RefreshStatusFailed
		profile.LastError = err.Error()
		profile.CurrentRunID = ""
		profile.UpdatedAt = finishedAt
		_, _ = m.store.SaveBuildProfile(profile)
	}
}

func (m *Manager) loadBuildInputs(profile domain.BuildProfile) ([]domain.SubscriptionSource, []domain.RuleSource, error) {
	subscriptions := make([]domain.SubscriptionSource, 0, len(profile.SubscriptionSourceIDs))
	for _, sourceID := range profile.SubscriptionSourceIDs {
		source, err := m.store.GetSubscription(sourceID)
		if err != nil {
			return nil, nil, fmt.Errorf("load subscription source %q: %w", sourceID, err)
		}
		if !source.Enabled {
			return nil, nil, fmt.Errorf("%w: subscription source %q", ErrDisabled, source.Name)
		}
		subscriptions = append(subscriptions, source)
	}

	ruleSources := make([]domain.RuleSource, 0, len(profile.RuleBindings))
	seen := make(map[string]struct{}, len(profile.RuleBindings))
	for _, binding := range profile.RuleBindings {
		if _, ok := seen[binding.RuleSourceID]; ok {
			continue
		}
		source, err := m.store.GetRuleSource(binding.RuleSourceID)
		if err != nil {
			return nil, nil, fmt.Errorf("load rule source %q: %w", binding.RuleSourceID, err)
		}
		if !source.Enabled {
			return nil, nil, fmt.Errorf("%w: rule source %q", ErrDisabled, source.Name)
		}
		ruleSources = append(ruleSources, source)
		seen[binding.RuleSourceID] = struct{}{}
	}

	return subscriptions, ruleSources, nil
}

func (m *Manager) validateBuildSourceReferences(subscriptionIDs []string, ruleBindings []domain.BuildRuleBinding) error {
	for _, sourceID := range subscriptionIDs {
		if _, err := m.store.GetSubscription(sourceID); err != nil {
			return fmt.Errorf("%w: subscription source %q not found", ErrInvalidInput, sourceID)
		}
	}
	for _, binding := range ruleBindings {
		if _, err := m.store.GetRuleSource(binding.RuleSourceID); err != nil {
			return fmt.Errorf("%w: rule source %q not found", ErrInvalidInput, binding.RuleSourceID)
		}
	}
	return nil
}

func validateCreateBuildProfileInput(input CreateBuildProfileInput) error {
	if strings.TrimSpace(input.Name) == "" {
		return fmt.Errorf("%w: build profile name is required", ErrInvalidInput)
	}
	if len(input.SubscriptionSourceIDs) == 0 {
		return fmt.Errorf("%w: at least one subscription source is required", ErrInvalidInput)
	}

	for _, item := range input.Filters {
		if item.Pattern == "" {
			return fmt.Errorf("%w: filter pattern is required", ErrInvalidInput)
		}
		if _, err := regexp.Compile(item.Pattern); err != nil {
			return fmt.Errorf("%w: invalid filter regex %q", ErrInvalidInput, item.Pattern)
		}
	}
	for _, item := range input.Renames {
		if item.Pattern == "" {
			return fmt.Errorf("%w: rename pattern is required", ErrInvalidInput)
		}
		if _, err := regexp.Compile(item.Pattern); err != nil {
			return fmt.Errorf("%w: invalid rename regex %q", ErrInvalidInput, item.Pattern)
		}
	}
	for _, item := range input.Groups {
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
	for _, item := range input.RuleBindings {
		if strings.TrimSpace(item.RuleSourceID) == "" {
			return fmt.Errorf("%w: rule_source_id is required", ErrInvalidInput)
		}
	}
	return nil
}

func cloneRuleBindings(in []domain.BuildRuleBinding) []domain.BuildRuleBinding {
	out := make([]domain.BuildRuleBinding, 0, len(in))
	for _, item := range in {
		out = append(out, item.Clone())
	}
	return out
}

func cloneFilterRules(in []domain.ProxyFilterRule) []domain.ProxyFilterRule {
	out := make([]domain.ProxyFilterRule, 0, len(in))
	for _, item := range in {
		out = append(out, item.Clone())
	}
	return out
}

func cloneRenameRules(in []domain.RenameRule) []domain.RenameRule {
	out := make([]domain.RenameRule, 0, len(in))
	for _, item := range in {
		out = append(out, item.Clone())
	}
	return out
}

func cloneGroupSpecs(in []domain.ProxyGroupSpec) []domain.ProxyGroupSpec {
	out := make([]domain.ProxyGroupSpec, 0, len(in))
	for _, item := range in {
		out = append(out, item.Clone())
	}
	return out
}

func buildProfileKey(id string) string {
	return "build_profile:" + id
}

func buildArtifactFileName(profileName string) string {
	name := strings.TrimSpace(profileName)
	if name == "" {
		name = "submanager"
	}
	replacer := regexp.MustCompile(`[^a-zA-Z0-9._-]+`)
	name = replacer.ReplaceAllString(name, "-")
	name = strings.Trim(name, "-")
	if name == "" {
		name = "submanager"
	}
	return name + ".yaml"
}
