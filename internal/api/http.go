package api

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"submanager/internal/domain"
	"submanager/internal/service"
	"submanager/internal/store"
)

type Handler struct {
	manager        *service.Manager
	superuserToken string
}

func NewHandler(manager *service.Manager, superuserToken string) *Handler {
	return &Handler{
		manager:        manager,
		superuserToken: superuserToken,
	}
}

func (h *Handler) Routes(staticFS http.FileSystem) http.Handler {
	root := http.NewServeMux()
	
	// Public unauthenticated routes
	root.HandleFunc("/subscribe", h.handlePublicSubscription)
	root.HandleFunc("/subscribe/", h.handlePublicSubscription)

	// API Mux
	apiMux := http.NewServeMux()
	apiMux.HandleFunc("/healthz", h.handleHealth)
	apiMux.HandleFunc("/subscriptions", h.handleSubscriptions)
	apiMux.HandleFunc("/subscriptions/", h.handleSubscriptionByID)
	apiMux.HandleFunc("/rules", h.handleRuleSources)
	apiMux.HandleFunc("/rules/", h.handleRuleSourceByID)
	apiMux.HandleFunc("/build-profiles", h.handleBuildProfiles)
	apiMux.HandleFunc("/build-profiles/", h.handleBuildProfileByID)
	apiMux.HandleFunc("/build-runs/", h.handleBuildRunByID)
	apiMux.HandleFunc("/build-artifacts/", h.handleBuildArtifactByID)
	apiMux.HandleFunc("/download-tokens", h.handleDownloadTokens)
	apiMux.HandleFunc("/download-tokens/", h.handleDownloadTokenByID)
	apiMux.HandleFunc("/jobs/", h.handleJobByID)
	apiMux.HandleFunc("/system-alerts", h.handleSystemAlerts)

	// Wrap API with CORS and Auth
	apiHandler := corsMiddleware(h.requireSuperUser(apiMux))
	root.Handle("/api/", http.StripPrefix("/api", apiHandler))

	// SPA Static Frontend (Fallback)
	if staticFS != nil {
		root.Handle("/", spaHandler{staticFS})
	}

	return root
}

func (h *Handler) requireSuperUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := bearerTokenFromRequest(r)
		if !ok || subtle.ConstantTimeCompare([]byte(token), []byte(h.superuserToken)) != 1 {
			w.Header().Set("WWW-Authenticate", `Bearer realm="submanager"`)
			writeJSON(w, http.StatusUnauthorized, map[string]string{
				"error": "unauthorized",
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Set("Access-Control-Max-Age", "86400")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type spaHandler struct {
	fs http.FileSystem
}

func (h spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	f, err := h.fs.Open(path)
	if err != nil {
		r.URL.Path = "/"
	} else {
		stat, err := f.Stat()
		if err == nil && stat.IsDir() {
			r.URL.Path = "/"
		}
		f.Close()
	}
	http.FileServer(h.fs).ServeHTTP(w, r)
}

func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) handleSubscriptions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		items, err := h.manager.ListSubscriptionSources(r.Context())
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	case http.MethodPost:
		var req createSubscriptionRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, err)
			return
		}

		enabled := true
		if req.Enabled != nil {
			enabled = *req.Enabled
		}

		item, err := h.manager.CreateSubscriptionSource(r.Context(), service.CreateSubscriptionInput{
			Name:                req.Name,
			Type:                req.Type,
			URL:                 req.URL,
			Payload:             req.Payload,
			Headers:             req.Headers,
			Enabled:             enabled,
			TimeoutSec:          req.TimeoutSec,
			UserAgent:           req.UserAgent,
			RetryAttempts:       req.RetryAttempts,
			RetryBackoffMS:      req.RetryBackoffMS,
			MinFetchIntervalSec: req.MinFetchIntervalSec,
			CacheTTLSeconds:     req.CacheTTLSeconds,
			RefreshIntervalSec:  req.RefreshIntervalSec,
		})
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, item)
	default:
		methodNotAllowed(w)
	}
}

func (h *Handler) handleSubscriptionByID(w http.ResponseWriter, r *http.Request) {
	id, action := splitTail(r.URL.Path, "/subscriptions/")
	if id == "" {
		writeError(w, errBadRequest("missing subscription id"))
		return
	}

	if action == "" {
		switch r.Method {
		case http.MethodGet:
			item, err := h.manager.GetSubscriptionSource(r.Context(), id)
			if err != nil {
				writeError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, item)
		case http.MethodPatch, http.MethodPut:
			var req updateSubscriptionRequest
			if err := decodeJSON(r, &req); err != nil {
				writeError(w, err)
				return
			}
			if req.isEmpty() {
				writeError(w, errBadRequest("at least one field is required"))
				return
			}
			item, err := h.manager.UpdateSubscriptionSource(r.Context(), id, service.UpdateSubscriptionInput{
				Name:                req.Name,
				Type:                req.Type,
				URL:                 req.URL,
				Payload:             req.Payload,
				Headers:             req.Headers,
				Enabled:             req.Enabled,
				TimeoutSec:          req.TimeoutSec,
				UserAgent:           req.UserAgent,
				RetryAttempts:       req.RetryAttempts,
				RetryBackoffMS:      req.RetryBackoffMS,
				MinFetchIntervalSec: req.MinFetchIntervalSec,
				CacheTTLSeconds:     req.CacheTTLSeconds,
				RefreshIntervalSec:  req.RefreshIntervalSec,
			})
			if err != nil {
				writeError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, item)
		case http.MethodDelete:
			if err := h.manager.DeleteSubscriptionSource(r.Context(), id); err != nil {
				writeError(w, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			methodNotAllowed(w)
		}
		return
	}

	if action == "refresh" && r.Method == http.MethodPost {
		job, err := h.manager.RefreshSubscriptionSource(r.Context(), id)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusAccepted, job)
		return
	}

	http.NotFound(w, r)
}

func (h *Handler) handleRuleSources(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		items, err := h.manager.ListRuleSources(r.Context())
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	case http.MethodPost:
		var req createRuleSourceRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, err)
			return
		}

		enabled := true
		if req.Enabled != nil {
			enabled = *req.Enabled
		}

		item, err := h.manager.CreateRuleSource(r.Context(), service.CreateRuleSourceInput{
			Name:                req.Name,
			URL:                 req.URL,
			Mode:                domain.RuleSourceMode(req.Mode),
			Headers:             req.Headers,
			Enabled:             enabled,
			TimeoutSec:          req.TimeoutSec,
			UserAgent:           req.UserAgent,
			RetryAttempts:       req.RetryAttempts,
			RetryBackoffMS:      req.RetryBackoffMS,
			MinFetchIntervalSec: req.MinFetchIntervalSec,
			CacheTTLSeconds:     req.CacheTTLSeconds,
			RefreshIntervalSec:  req.RefreshIntervalSec,
		})
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, item)
	default:
		methodNotAllowed(w)
	}
}

func (h *Handler) handleRuleSourceByID(w http.ResponseWriter, r *http.Request) {
	id, action := splitTail(r.URL.Path, "/rules/")
	if id == "" {
		writeError(w, errBadRequest("missing rule source id"))
		return
	}

	if action == "" {
		switch r.Method {
		case http.MethodGet:
			item, err := h.manager.GetRuleSource(r.Context(), id)
			if err != nil {
				writeError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, item)
		case http.MethodPatch, http.MethodPut:
			var req updateRuleSourceRequest
			if err := decodeJSON(r, &req); err != nil {
				writeError(w, err)
				return
			}
			if req.isEmpty() {
				writeError(w, errBadRequest("at least one field is required"))
				return
			}

			var mode *domain.RuleSourceMode
			if req.Mode != nil {
				value := domain.RuleSourceMode(strings.TrimSpace(*req.Mode))
				mode = &value
			}

			item, err := h.manager.UpdateRuleSource(r.Context(), id, service.UpdateRuleSourceInput{
				Name:                req.Name,
				URL:                 req.URL,
				Mode:                mode,
				Headers:             req.Headers,
				Enabled:             req.Enabled,
				TimeoutSec:          req.TimeoutSec,
				UserAgent:           req.UserAgent,
				RetryAttempts:       req.RetryAttempts,
				RetryBackoffMS:      req.RetryBackoffMS,
				MinFetchIntervalSec: req.MinFetchIntervalSec,
				CacheTTLSeconds:     req.CacheTTLSeconds,
				RefreshIntervalSec:  req.RefreshIntervalSec,
			})
			if err != nil {
				writeError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, item)
		case http.MethodDelete:
			if err := h.manager.DeleteRuleSource(r.Context(), id); err != nil {
				writeError(w, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			methodNotAllowed(w)
		}
		return
	}

	if action == "refresh" && r.Method == http.MethodPost {
		job, err := h.manager.RefreshRuleSource(r.Context(), id)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusAccepted, job)
		return
	}

	http.NotFound(w, r)
}

func (h *Handler) handleJobByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/jobs/")
	if id == "" {
		writeError(w, errBadRequest("missing job id"))
		return
	}

	job, err := h.manager.GetJob(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (h *Handler) handleBuildProfiles(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		items, err := h.manager.ListBuildProfiles(r.Context())
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	case http.MethodPost:
		var req createBuildProfileRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, err)
			return
		}

		enabled := true
		if req.Enabled != nil {
			enabled = *req.Enabled
		}

		item, err := h.manager.CreateBuildProfile(r.Context(), service.CreateBuildProfileInput{
			Name:                  req.Name,
			Description:           req.Description,
			SubscriptionSourceIDs: req.SubscriptionSourceIDs,
			RuleBindings:          req.RuleBindings,
			Template:              req.Template,
			Filters:               req.Filters,
			Renames:               req.Renames,
			Groups:                req.Groups,
			DefaultGroup:          req.DefaultGroup,
			Enabled:               enabled,
			AutoBuild:             req.AutoBuild,
			BuildIntervalSec:      req.BuildIntervalSec,
		})
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, item)
	default:
		methodNotAllowed(w)
	}
}

func (h *Handler) handleBuildProfileByID(w http.ResponseWriter, r *http.Request) {
	id, action := splitTail(r.URL.Path, "/build-profiles/")
	if id == "" {
		writeError(w, errBadRequest("missing build profile id"))
		return
	}

	if action == "" {
		switch r.Method {
		case http.MethodGet:
			item, err := h.manager.GetBuildProfile(r.Context(), id)
			if err != nil {
				writeError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, item)
		case http.MethodPatch, http.MethodPut:
			var req updateBuildProfileRequest
			if err := decodeJSON(r, &req); err != nil {
				writeError(w, err)
				return
			}
			if req.isEmpty() {
				writeError(w, errBadRequest("at least one field is required"))
				return
			}
			item, err := h.manager.UpdateBuildProfile(r.Context(), id, service.UpdateBuildProfileInput{
				Name:                  req.Name,
				Description:           req.Description,
				SubscriptionSourceIDs: req.SubscriptionSourceIDs,
				RuleBindings:          req.RuleBindings,
				Template:              req.Template,
				Filters:               req.Filters,
				Renames:               req.Renames,
				Groups:                req.Groups,
				DefaultGroup:          req.DefaultGroup,
				Enabled:               req.Enabled,
				AutoBuild:             req.AutoBuild,
				BuildIntervalSec:      req.BuildIntervalSec,
			})
			if err != nil {
				writeError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, item)
		case http.MethodDelete:
			if err := h.manager.DeleteBuildProfile(r.Context(), id); err != nil {
				writeError(w, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			methodNotAllowed(w)
		}
		return
	}

	if action == "preview" && r.Method == http.MethodGet {
		preview, err := h.manager.PreviewBuildProfile(r.Context(), id)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, preview)
		return
	}

	if action == "build" && r.Method == http.MethodPost {
		run, err := h.manager.RunBuildProfile(r.Context(), id)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusAccepted, run)
		return
	}

	http.NotFound(w, r)
}

func (h *Handler) handleBuildRunByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/build-runs/")
	if id == "" {
		writeError(w, errBadRequest("missing build run id"))
		return
	}

	item, err := h.manager.GetBuildRun(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *Handler) handleBuildArtifactByID(w http.ResponseWriter, r *http.Request) {
	id, action := splitTail(r.URL.Path, "/api/build-artifacts/")
	if id == "" {
		writeError(w, errBadRequest("missing build artifact id"))
		return
	}
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}

	item, err := h.manager.GetBuildArtifact(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}

	if action == "raw" {
		writeArtifactContent(w, r, item)
		return
	}

	if action != "" {
		http.NotFound(w, r)
		return
	}

	writeJSON(w, http.StatusOK, item)
}

func (h *Handler) handleDownloadTokens(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		items, err := h.manager.ListDownloadTokens(r.Context())
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	case http.MethodPost:
		var req createDownloadTokenRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, err)
			return
		}

		enabled := true
		if req.Enabled != nil {
			enabled = *req.Enabled
		}

		item, err := h.manager.CreateDownloadToken(r.Context(), service.CreateDownloadTokenInput{
			Name:           req.Name,
			BuildProfileID: req.BuildProfileID,
			Distribution:   req.Distribution,
			Prebuild:       req.Prebuild,
			Enabled:        enabled,
		})
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, item)
	default:
		methodNotAllowed(w)
	}
}

func (h *Handler) handleDownloadTokenByID(w http.ResponseWriter, r *http.Request) {
	id, action := splitTail(r.URL.Path, "/download-tokens/")
	if id == "" {
		writeError(w, errBadRequest("missing download token id"))
		return
	}

	if action == "preview" {
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		item, err := h.manager.PreviewDownloadToken(r.Context(), id)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, item)
		return
	}

	if action != "" {
		http.NotFound(w, r)
		return
	}

	switch r.Method {
	case http.MethodGet:
		item, err := h.manager.GetDownloadToken(r.Context(), id)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, item)
	case http.MethodPatch, http.MethodPut:
		var req updateDownloadTokenRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, err)
			return
		}
		if req.isEmpty() {
			writeError(w, errBadRequest("at least one field is required"))
			return
		}

		item, err := h.manager.UpdateDownloadToken(r.Context(), id, service.UpdateDownloadTokenInput{
			Name:         req.Name,
			Enabled:      req.Enabled,
			Distribution: req.Distribution,
			Prebuild:     req.Prebuild,
		})
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, item)
	case http.MethodDelete:
		if err := h.manager.DeleteDownloadToken(r.Context(), id); err != nil {
			writeError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		methodNotAllowed(w)
	}
}

func (h *Handler) handleSystemAlerts(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		items, err := h.manager.ListSystemAlerts(r.Context())
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	case http.MethodDelete:
		if err := h.manager.ClearSystemAlerts(r.Context()); err != nil {
			writeError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		methodNotAllowed(w)
	}
}

func (h *Handler) handlePublicSubscription(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}

	token := strings.TrimPrefix(r.URL.Path, "/subscribe")
	token = strings.Trim(token, "/")
	if token == "" {
		writeError(w, errBadRequest("missing subscription token"))
		return
	}

	clientIP := r.Header.Get("X-Forwarded-For")
	if clientIP == "" {
		clientIP = r.Header.Get("X-Real-IP")
	}
	if clientIP == "" {
		clientIP, _, _ = strings.Cut(r.RemoteAddr, ":")
	}

	item, err := h.manager.ResolveDownloadArtifact(r.Context(), token, clientIP)
	if err != nil {
		writeError(w, err)
		return
	}

	writeArtifactContent(w, r, item)
}

type createSubscriptionRequest struct {
	Name                string            `json:"name"`
	Type                string            `json:"type"`
	URL                 string            `json:"url"`
	Payload             string            `json:"payload"`
	Headers             map[string]string `json:"headers"`
	Enabled             *bool             `json:"enabled"`
	TimeoutSec          int               `json:"timeout_sec"`
	UserAgent           string            `json:"user_agent"`
	RetryAttempts       int               `json:"retry_attempts"`
	RetryBackoffMS      int               `json:"retry_backoff_ms"`
	MinFetchIntervalSec int               `json:"min_fetch_interval_sec"`
	CacheTTLSeconds     int               `json:"cache_ttl_seconds"`
	RefreshIntervalSec  int               `json:"refresh_interval_sec"`
}

type updateSubscriptionRequest struct {
	Name                *string            `json:"name"`
	Type                *string            `json:"type"`
	URL                 *string            `json:"url"`
	Payload             *string            `json:"payload"`
	Headers             *map[string]string `json:"headers"`
	Enabled             *bool              `json:"enabled"`
	TimeoutSec          *int               `json:"timeout_sec"`
	UserAgent           *string            `json:"user_agent"`
	RetryAttempts       *int               `json:"retry_attempts"`
	RetryBackoffMS      *int               `json:"retry_backoff_ms"`
	MinFetchIntervalSec *int               `json:"min_fetch_interval_sec"`
	CacheTTLSeconds     *int               `json:"cache_ttl_seconds"`
	RefreshIntervalSec  *int               `json:"refresh_interval_sec"`
}

func (r updateSubscriptionRequest) isEmpty() bool {
	return r.Name == nil && r.Type == nil && r.URL == nil && r.Payload == nil && r.Headers == nil && r.Enabled == nil &&
		r.TimeoutSec == nil && r.UserAgent == nil && r.RetryAttempts == nil &&
		r.RetryBackoffMS == nil && r.MinFetchIntervalSec == nil &&
		r.CacheTTLSeconds == nil && r.RefreshIntervalSec == nil
}

type createRuleSourceRequest struct {
	Name                string            `json:"name"`
	URL                 string            `json:"url"`
	Mode                string            `json:"mode"`
	Headers             map[string]string `json:"headers"`
	Enabled             *bool             `json:"enabled"`
	TimeoutSec          int               `json:"timeout_sec"`
	UserAgent           string            `json:"user_agent"`
	RetryAttempts       int               `json:"retry_attempts"`
	RetryBackoffMS      int               `json:"retry_backoff_ms"`
	MinFetchIntervalSec int               `json:"min_fetch_interval_sec"`
	CacheTTLSeconds     int               `json:"cache_ttl_seconds"`
	RefreshIntervalSec  int               `json:"refresh_interval_sec"`
}

type updateRuleSourceRequest struct {
	Name                *string            `json:"name"`
	URL                 *string            `json:"url"`
	Mode                *string            `json:"mode"`
	Headers             *map[string]string `json:"headers"`
	Enabled             *bool              `json:"enabled"`
	TimeoutSec          *int               `json:"timeout_sec"`
	UserAgent           *string            `json:"user_agent"`
	RetryAttempts       *int               `json:"retry_attempts"`
	RetryBackoffMS      *int               `json:"retry_backoff_ms"`
	MinFetchIntervalSec *int               `json:"min_fetch_interval_sec"`
	CacheTTLSeconds     *int               `json:"cache_ttl_seconds"`
	RefreshIntervalSec  *int               `json:"refresh_interval_sec"`
}

func (r updateRuleSourceRequest) isEmpty() bool {
	return r.Name == nil && r.URL == nil && r.Mode == nil && r.Headers == nil &&
		r.Enabled == nil && r.TimeoutSec == nil && r.UserAgent == nil &&
		r.RetryAttempts == nil && r.RetryBackoffMS == nil &&
		r.MinFetchIntervalSec == nil && r.CacheTTLSeconds == nil &&
		r.RefreshIntervalSec == nil
}

type createBuildProfileRequest struct {
	Name                  string                    `json:"name"`
	Description           string                    `json:"description"`
	SubscriptionSourceIDs []string                  `json:"subscription_source_ids"`
	RuleBindings          []domain.BuildRuleBinding `json:"rule_bindings"`
	Template              domain.BuildTemplate      `json:"template"`
	Filters               []domain.ProxyFilterRule  `json:"filters"`
	Renames               []domain.RenameRule       `json:"renames"`
	Groups                []domain.ProxyGroupSpec   `json:"groups"`
	DefaultGroup          string                    `json:"default_group"`
	Enabled               *bool                     `json:"enabled"`
	AutoBuild             bool                      `json:"auto_build"`
	BuildIntervalSec      int                       `json:"build_interval_sec"`
}

type updateBuildProfileRequest struct {
	Name                  *string                    `json:"name"`
	Description           *string                    `json:"description"`
	SubscriptionSourceIDs *[]string                  `json:"subscription_source_ids"`
	RuleBindings          *[]domain.BuildRuleBinding `json:"rule_bindings"`
	Template              *domain.BuildTemplate      `json:"template"`
	Filters               *[]domain.ProxyFilterRule  `json:"filters"`
	Renames               *[]domain.RenameRule       `json:"renames"`
	Groups                *[]domain.ProxyGroupSpec   `json:"groups"`
	DefaultGroup          *string                    `json:"default_group"`
	Enabled               *bool                      `json:"enabled"`
	AutoBuild             *bool                      `json:"auto_build"`
	BuildIntervalSec      *int                       `json:"build_interval_sec"`
}

func (r updateBuildProfileRequest) isEmpty() bool {
	return r.Name == nil && r.Description == nil && r.SubscriptionSourceIDs == nil &&
		r.RuleBindings == nil && r.Template == nil && r.Filters == nil &&
		r.Renames == nil && r.Groups == nil && r.DefaultGroup == nil &&
		r.Enabled == nil && r.AutoBuild == nil && r.BuildIntervalSec == nil
}

type createDownloadTokenRequest struct {
	Name           string                           `json:"name"`
	BuildProfileID string                           `json:"build_profile_id"`
	Distribution   domain.DownloadTokenDistribution `json:"distribution"`
	Prebuild       bool                             `json:"prebuild"`
	Enabled        *bool                            `json:"enabled"`
}

type updateDownloadTokenRequest struct {
	Name         *string                           `json:"name"`
	Distribution *domain.DownloadTokenDistribution `json:"distribution"`
	Prebuild     *bool                             `json:"prebuild"`
	Enabled      *bool                             `json:"enabled"`
}

func (r updateDownloadTokenRequest) isEmpty() bool {
	return r.Name == nil && r.Distribution == nil && r.Prebuild == nil && r.Enabled == nil
}

func decodeJSON(r *http.Request, target any) error {
	defer r.Body.Close()

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return errBadRequest("invalid json body")
	}
	return nil
}

func splitTail(path, prefix string) (id string, action string) {
	tail := strings.TrimPrefix(path, prefix)
	tail = strings.Trim(tail, "/")
	if tail == "" {
		return "", ""
	}

	parts := strings.Split(tail, "/")
	id = parts[0]
	if len(parts) > 1 {
		action = parts[1]
	}
	return id, action
}

func writeArtifactContent(w http.ResponseWriter, r *http.Request, artifact domain.BuildArtifact) {
	etag := artifactETag(artifact)
	lastModified := artifact.CreatedAt.UTC()
	if isConditionalRequestFresh(r, etag, lastModified) {
		if etag != "" {
			w.Header().Set("ETag", etag)
		}
		if !lastModified.IsZero() {
			w.Header().Set("Last-Modified", lastModified.Format(http.TimeFormat))
		}
		w.WriteHeader(http.StatusNotModified)
		return
	}

	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", artifact.FileName))
	if etag != "" {
		w.Header().Set("ETag", etag)
	}
	if !lastModified.IsZero() {
		w.Header().Set("Last-Modified", lastModified.Format(http.TimeFormat))
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(artifact.Content))
}

func artifactETag(artifact domain.BuildArtifact) string {
	if strings.TrimSpace(artifact.SHA256) == "" {
		return ""
	}
	return `W/"` + artifact.SHA256 + `"`
}

func isConditionalRequestFresh(r *http.Request, etag string, lastModified time.Time) bool {
	if match := strings.TrimSpace(r.Header.Get("If-None-Match")); match != "" && etag != "" {
		for _, token := range strings.Split(match, ",") {
			value := strings.TrimSpace(token)
			if value == "*" || value == etag {
				return true
			}
		}
	}

	if modifiedSince := strings.TrimSpace(r.Header.Get("If-Modified-Since")); modifiedSince != "" && !lastModified.IsZero() {
		if parsed, err := time.Parse(http.TimeFormat, modifiedSince); err == nil {
			return !lastModified.After(parsed)
		}
	}
	return false
}

func bearerTokenFromRequest(r *http.Request) (string, bool) {
	value := strings.TrimSpace(r.Header.Get("Authorization"))
	if value == "" {
		return "", false
	}
	parts := strings.SplitN(value, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", false
	}
	token := strings.TrimSpace(parts[1])
	if token == "" {
		return "", false
	}
	return token, true
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	var badReq badRequestError

	switch {
	case errors.Is(err, store.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, service.ErrAlreadyRunning):
		status = http.StatusConflict
	case errors.Is(err, service.ErrDisabled):
		status = http.StatusConflict
	case errors.Is(err, service.ErrAccessDenied):
		status = http.StatusForbidden
	case errors.Is(err, service.ErrInvalidInput):
		status = http.StatusBadRequest
	case errors.As(err, &badReq):
		status = http.StatusBadRequest
	}

	writeJSON(w, status, map[string]string{
		"error": err.Error(),
	})
}

func methodNotAllowed(w http.ResponseWriter) {
	writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
		"error": "method not allowed",
	})
}

type badRequestError struct {
	message string
}

func (e badRequestError) Error() string {
	return e.message
}

func errBadRequest(message string) error {
	return badRequestError{message: message}
}
