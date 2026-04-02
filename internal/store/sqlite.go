package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"submanager/internal/domain"
)

type SQLiteStore struct {
	db *sql.DB
}

func NewSQLiteStore(path string) (*SQLiteStore, error) {
	if path == "" {
		path = "submanager.db"
	}

	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("sqlite store: create dir %q: %w", dir, err)
		}
	}

	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("sqlite store: open db: %w", err)
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	store := &SQLiteStore{db: db}
	if err := store.init(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *SQLiteStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *SQLiteStore) SaveSubscription(src domain.SubscriptionSource) (domain.SubscriptionSource, error) {
	return saveEntity(s.db, "subscription_sources", src.ID, src.CreatedAt, src.UpdatedAt, src)
}

func (s *SQLiteStore) GetSubscription(id string) (domain.SubscriptionSource, error) {
	return getEntity[domain.SubscriptionSource](s.db, "subscription_sources", id)
}

func (s *SQLiteStore) ListSubscriptions() ([]domain.SubscriptionSource, error) {
	return listEntities[domain.SubscriptionSource](s.db, "subscription_sources")
}

func (s *SQLiteStore) SaveRuleSource(src domain.RuleSource) (domain.RuleSource, error) {
	return saveEntity(s.db, "rule_sources", src.ID, src.CreatedAt, src.UpdatedAt, src)
}

func (s *SQLiteStore) GetRuleSource(id string) (domain.RuleSource, error) {
	return getEntity[domain.RuleSource](s.db, "rule_sources", id)
}

func (s *SQLiteStore) ListRuleSources() ([]domain.RuleSource, error) {
	return listEntities[domain.RuleSource](s.db, "rule_sources")
}

func (s *SQLiteStore) SaveJob(job domain.Job) (domain.Job, error) {
	updatedAt := maxTime(job.CreatedAt, derefTime(job.FinishedAt), derefTime(job.StartedAt))
	return saveEntity(s.db, "jobs", job.ID, job.CreatedAt, updatedAt, job)
}

func (s *SQLiteStore) GetJob(id string) (domain.Job, error) {
	return getEntity[domain.Job](s.db, "jobs", id)
}

func (s *SQLiteStore) SaveBuildProfile(profile domain.BuildProfile) (domain.BuildProfile, error) {
	return saveEntity(s.db, "build_profiles", profile.ID, profile.CreatedAt, profile.UpdatedAt, profile)
}

func (s *SQLiteStore) GetBuildProfile(id string) (domain.BuildProfile, error) {
	var payload string
	err := s.db.QueryRow(`SELECT payload FROM build_profiles WHERE id = ?`, id).Scan(&payload)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.BuildProfile{}, ErrNotFound
		}
		return domain.BuildProfile{}, fmt.Errorf("sqlite store: get build_profiles: %w", err)
	}

	return unmarshalBuildProfile(payload)
}

func (s *SQLiteStore) ListBuildProfiles() ([]domain.BuildProfile, error) {
	rows, err := s.db.Query(`SELECT payload FROM build_profiles ORDER BY created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("sqlite store: list build_profiles: %w", err)
	}
	defer rows.Close()

	out := make([]domain.BuildProfile, 0)
	for rows.Next() {
		var payload string
		if err := rows.Scan(&payload); err != nil {
			return nil, fmt.Errorf("sqlite store: scan build_profiles: %w", err)
		}
		value, err := unmarshalBuildProfile(payload)
		if err != nil {
			return nil, err
		}
		out = append(out, value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite store: iterate build_profiles: %w", err)
	}
	return out, nil
}

func (s *SQLiteStore) SaveBuildRun(run domain.BuildRun) (domain.BuildRun, error) {
	updatedAt := maxTime(run.CreatedAt, derefTime(run.FinishedAt), derefTime(run.StartedAt))
	return saveEntity(s.db, "build_runs", run.ID, run.CreatedAt, updatedAt, run)
}

func (s *SQLiteStore) GetBuildRun(id string) (domain.BuildRun, error) {
	return getEntity[domain.BuildRun](s.db, "build_runs", id)
}

func (s *SQLiteStore) SaveBuildArtifact(artifact domain.BuildArtifact) (domain.BuildArtifact, error) {
	return saveEntity(s.db, "build_artifacts", artifact.ID, artifact.CreatedAt, artifact.CreatedAt, artifact)
}

func (s *SQLiteStore) GetBuildArtifact(id string) (domain.BuildArtifact, error) {
	return getEntity[domain.BuildArtifact](s.db, "build_artifacts", id)
}

func (s *SQLiteStore) SaveDownloadToken(token domain.DownloadTokenRecord) (domain.DownloadTokenRecord, error) {
	if token.ID == "" {
		return domain.DownloadTokenRecord{}, fmt.Errorf("sqlite store: empty id for download_tokens")
	}
	if token.CreatedAt.IsZero() {
		token.CreatedAt = time.Now().UTC()
	}
	if token.UpdatedAt.IsZero() {
		token.UpdatedAt = token.CreatedAt
	}

	payload, err := json.Marshal(token)
	if err != nil {
		return domain.DownloadTokenRecord{}, fmt.Errorf("sqlite store: marshal download_tokens: %w", err)
	}

	_, err = s.db.Exec(`
		INSERT INTO download_tokens (id, token_hash, created_at, updated_at, payload)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			token_hash = excluded.token_hash,
			created_at = excluded.created_at,
			updated_at = excluded.updated_at,
			payload = excluded.payload
	`, token.ID, token.TokenHash, token.CreatedAt.UTC().Format(time.RFC3339Nano), token.UpdatedAt.UTC().Format(time.RFC3339Nano), string(payload))
	if err != nil {
		return domain.DownloadTokenRecord{}, fmt.Errorf("sqlite store: save download_tokens: %w", err)
	}
	return token.Clone(), nil
}

func (s *SQLiteStore) GetDownloadToken(id string) (domain.DownloadTokenRecord, error) {
	return getEntity[domain.DownloadTokenRecord](s.db, "download_tokens", id)
}

func (s *SQLiteStore) FindDownloadTokenByHash(hash string) (domain.DownloadTokenRecord, error) {
	var payload string
	err := s.db.QueryRow(`SELECT payload FROM download_tokens WHERE token_hash = ?`, hash).Scan(&payload)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.DownloadTokenRecord{}, ErrNotFound
		}
		return domain.DownloadTokenRecord{}, fmt.Errorf("sqlite store: get download_tokens by hash: %w", err)
	}

	var token domain.DownloadTokenRecord
	if err := json.Unmarshal([]byte(payload), &token); err != nil {
		return domain.DownloadTokenRecord{}, fmt.Errorf("sqlite store: unmarshal download_tokens: %w", err)
	}
	return token, nil
}

func (s *SQLiteStore) ListDownloadTokens() ([]domain.DownloadTokenRecord, error) {
	return listEntities[domain.DownloadTokenRecord](s.db, "download_tokens")
}

func (s *SQLiteStore) SaveSystemAlert(item domain.SystemAlert) (domain.SystemAlert, error) {
	return saveEntity(s.db, "system_alerts", item.ID, item.CreatedAt, item.CreatedAt, item)
}

func (s *SQLiteStore) ListSystemAlerts() ([]domain.SystemAlert, error) {
	return listEntities[domain.SystemAlert](s.db, "system_alerts")
}

func (s *SQLiteStore) ClearSystemAlerts() error {
	_, err := s.db.Exec(`DELETE FROM system_alerts`)
	return err
}

func (s *SQLiteStore) DeleteSubscriptionSource(id string) error {
	return deleteEntity(s.db, "subscription_sources", id)
}

func (s *SQLiteStore) DeleteRuleSource(id string) error {
	return deleteEntity(s.db, "rule_sources", id)
}

func (s *SQLiteStore) DeleteBuildProfile(id string) error {
	return deleteEntity(s.db, "build_profiles", id)
}

func (s *SQLiteStore) DeleteDownloadToken(id string) error {
	return deleteEntity(s.db, "download_tokens", id)
}

func (s *SQLiteStore) DeleteBuildRunsByProfile(profileID string) error {
	// field in JSON payload is 'profile_id' (matching domain.BuildRun.ProfileID)
	query := `DELETE FROM build_runs WHERE json_extract(payload, '$.profile_id') = ?`
	_, err := s.db.Exec(query, profileID)
	return err
}

func (s *SQLiteStore) DeleteBuildArtifactsByProfile(profileID string) error {
	// field in JSON payload is 'profile_id' (matching domain.BuildArtifact.ProfileID)
	query := `DELETE FROM build_artifacts WHERE json_extract(payload, '$.profile_id') = ?`
	_, err := s.db.Exec(query, profileID)
	return err
}

func (s *SQLiteStore) init() error {
	statements := []string{
		`PRAGMA journal_mode=WAL;`,
		`PRAGMA synchronous=NORMAL;`,
		`PRAGMA foreign_keys=ON;`,
		`PRAGMA busy_timeout=5000;`,
		`CREATE TABLE IF NOT EXISTS subscription_sources (
			id TEXT PRIMARY KEY,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			payload TEXT NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_subscription_sources_created_at ON subscription_sources(created_at);`,
		`CREATE TABLE IF NOT EXISTS rule_sources (
			id TEXT PRIMARY KEY,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			payload TEXT NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_rule_sources_created_at ON rule_sources(created_at);`,
		`CREATE TABLE IF NOT EXISTS jobs (
			id TEXT PRIMARY KEY,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			payload TEXT NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_jobs_created_at ON jobs(created_at);`,
		`CREATE TABLE IF NOT EXISTS build_profiles (
			id TEXT PRIMARY KEY,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			payload TEXT NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_build_profiles_created_at ON build_profiles(created_at);`,
		`CREATE TABLE IF NOT EXISTS build_runs (
			id TEXT PRIMARY KEY,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			payload TEXT NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_build_runs_created_at ON build_runs(created_at);`,
		`CREATE TABLE IF NOT EXISTS build_artifacts (
			id TEXT PRIMARY KEY,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			payload TEXT NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_build_artifacts_created_at ON build_artifacts(created_at);`,
		`CREATE TABLE IF NOT EXISTS download_tokens (
			id TEXT PRIMARY KEY,
			token_hash TEXT NOT NULL UNIQUE,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			payload TEXT NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_download_tokens_created_at ON download_tokens(created_at);`,
		`CREATE INDEX IF NOT EXISTS idx_download_tokens_token_hash ON download_tokens(token_hash);`,
		`CREATE TABLE IF NOT EXISTS system_alerts (
			id TEXT PRIMARY KEY,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			payload TEXT NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_system_alerts_created_at ON system_alerts(created_at);`,
	}

	for _, statement := range statements {
		if _, err := s.db.Exec(statement); err != nil {
			return fmt.Errorf("sqlite store: init schema: %w", err)
		}
	}
	return nil
}

func saveEntity[T any](db *sql.DB, table, id string, createdAt, updatedAt time.Time, value T) (T, error) {
	var zero T
	if id == "" {
		return zero, fmt.Errorf("sqlite store: empty id for %s", table)
	}
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	if updatedAt.IsZero() {
		updatedAt = createdAt
	}

	payload, err := json.Marshal(value)
	if err != nil {
		return zero, fmt.Errorf("sqlite store: marshal %s: %w", table, err)
	}

	query := fmt.Sprintf(`
		INSERT INTO %s (id, created_at, updated_at, payload)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			created_at = excluded.created_at,
			updated_at = excluded.updated_at,
			payload = excluded.payload
	`, table)
	if _, err := db.Exec(query, id, createdAt.UTC().Format(time.RFC3339Nano), updatedAt.UTC().Format(time.RFC3339Nano), string(payload)); err != nil {
		return zero, fmt.Errorf("sqlite store: save %s: %w", table, err)
	}
	return value, nil
}

func getEntity[T any](db *sql.DB, table, id string) (T, error) {
	var zero T
	query := fmt.Sprintf(`SELECT payload FROM %s WHERE id = ?`, table)
	var payload string
	err := db.QueryRow(query, id).Scan(&payload)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return zero, ErrNotFound
		}
		return zero, fmt.Errorf("sqlite store: get %s: %w", table, err)
	}

	var value T
	if err := json.Unmarshal([]byte(payload), &value); err != nil {
		return zero, fmt.Errorf("sqlite store: unmarshal %s: %w", table, err)
	}
	return value, nil
}

func deleteEntity(db *sql.DB, table, id string) error {
	query := fmt.Sprintf(`DELETE FROM %s WHERE id = ?`, table)
	_, err := db.Exec(query, id)
	return err
}

func listEntities[T any](db *sql.DB, table string) ([]T, error) {
	query := fmt.Sprintf(`SELECT payload FROM %s ORDER BY created_at ASC`, table)
	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("sqlite store: list %s: %w", table, err)
	}
	defer rows.Close()

	out := make([]T, 0)
	for rows.Next() {
		var payload string
		if err := rows.Scan(&payload); err != nil {
			return nil, fmt.Errorf("sqlite store: scan %s: %w", table, err)
		}
		var value T
		if err := json.Unmarshal([]byte(payload), &value); err != nil {
			return nil, fmt.Errorf("sqlite store: unmarshal %s: %w", table, err)
		}
		out = append(out, value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite store: iterate %s: %w", table, err)
	}
	return out, nil
}

func derefTime(value *time.Time) time.Time {
	if value == nil {
		return time.Time{}
	}
	return *value
}

func maxTime(values ...time.Time) time.Time {
	var max time.Time
	for _, value := range values {
		if value.IsZero() {
			continue
		}
		if max.IsZero() || value.After(max) {
			max = value
		}
	}
	return max
}

func unmarshalBuildProfile(payload string) (domain.BuildProfile, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(payload), &raw); err != nil {
		return domain.BuildProfile{}, fmt.Errorf("sqlite store: unmarshal build_profiles: %w", err)
	}
	if _, ok := raw["enabled"]; !ok {
		raw["enabled"] = []byte("true")
		normalized, err := json.Marshal(raw)
		if err != nil {
			return domain.BuildProfile{}, fmt.Errorf("sqlite store: normalize build_profiles: %w", err)
		}
		payload = string(normalized)
	}

	var profile domain.BuildProfile
	if err := json.Unmarshal([]byte(payload), &profile); err != nil {
		return domain.BuildProfile{}, fmt.Errorf("sqlite store: decode build_profiles: %w", err)
	}
	return profile, nil
}
