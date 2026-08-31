package handlers

import (
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/vpsctl/vpsctl/internal/auth"
	"github.com/vpsctl/vpsctl/internal/config"
	"github.com/vpsctl/vpsctl/internal/middleware"
	"github.com/vpsctl/vpsctl/internal/terminal"
	vpsweb "github.com/vpsctl/vpsctl/web"
)

// Server bundles all hand-written handler dependencies.
type Server struct {
	Config         *config.Config
	AuthManager    *auth.Manager
	Auth           *AuthHandler
	System         *SystemHandler
	Docker         *DockerHandler
	Services       *ServiceHandler
	Files          *FileHandler
	Terminal       *terminal.Handler
	Log            func(string)
	AllowedOrigins []string
}

// NewServer constructs a Server from config and an auth manager.
func NewServer(cfg *config.Config, am *auth.Manager) *Server {
	return &Server{
		Config:         cfg,
		AuthManager:    am,
		Auth:           NewAuthHandler(am),
		System:         NewSystemHandler(),
		Docker:         NewDockerHandler(),
		Services:       NewServiceHandler(),
		Files:          NewFileHandler(cfg.BaseDir),
		Terminal:       terminal.NewHandler("/bin/bash"),
		Log:            func(string) {},
		AllowedOrigins: []string{"*"},
	}
}

// Router builds the full mux router with middleware wired up.
func (s *Server) Router() http.Handler {
	r := mux.NewRouter()

	// Global CORS + request logging + rate limiting.
	var limiter *middleware.RateLimiter
	limiter = middleware.NewRateLimiter(100, time.Minute)

	verify := func(token string) (string, string, error) {
		claims, err := s.AuthManager.ParseJWT(token)
		if err != nil {
			return "", "", err
		}
		return claims.User, string(claims.Role), nil
	}

	r.Use(middleware.CORSMiddleware(s.AllowedOrigins))
	r.Use(middleware.RequestLoggerMiddleware(s.Log))

	// ---- Public routes ----
	r.HandleFunc("/api/health", s.Health).Methods("GET")
	r.HandleFunc("/api/auth/login", s.Auth.Login).Methods("POST")
	r.HandleFunc("/api/auth/refresh", s.Auth.Refresh).Methods("POST")

	// ---- Protected API routes ----
	api := r.PathPrefix("/api").Subrouter()
	api.Use(middleware.IPWhitelistMiddleware(s.Config.AllowedIPs))
	api.Use(middleware.AuthMiddleware(verify))
	api.Use(middleware.RateLimitMiddleware(limiter))
	api.Use(middleware.SecurityHeadersMiddleware)

	// Auth sub-routes
	api.HandleFunc("/auth/logout", s.Auth.Logout).Methods("POST")
	api.HandleFunc("/auth/setup-totp", s.Auth.SetupTOTP).Methods("POST")
	api.HandleFunc("/auth/verify-totp", s.Auth.VerifyTOTP).Methods("POST")
	api.HandleFunc("/auth/me", s.Auth.Me).Methods("GET")
	api.HandleFunc("/auth/change-password", s.Auth.ChangePassword).Methods("POST")

	// System
	api.HandleFunc("/system/info", s.System.Info).Methods("GET")
	api.HandleFunc("/system/metrics", s.System.Metrics).Methods("GET")
	api.HandleFunc("/system/metrics/history", s.System.MetricsHistory).Methods("GET")
	api.HandleFunc("/system/network", s.System.Network).Methods("GET")
	api.HandleFunc("/system/processes", s.System.Processes).Methods("GET")

	// Docker
	api.HandleFunc("/docker/containers", s.Docker.Containers).Methods("GET")
	api.HandleFunc("/docker/containers/{id}/logs", s.Docker.Logs).Methods("GET")
	api.HandleFunc("/docker/containers/{id}/{action}", s.Docker.ContainerAction).
		Methods("POST").
		MatcherFunc(actionMatcher("start", "stop", "restart"))
	api.HandleFunc("/docker/containers/{id}", s.Docker.Remove).Methods("DELETE")
	api.HandleFunc("/docker/images", s.Docker.Images).Methods("GET")
	api.HandleFunc("/docker/stats", s.Docker.Stats).Methods("GET")

	// Services
	api.HandleFunc("/services", s.Services.List).Methods("GET")
	api.HandleFunc("/services/{name}/logs", s.Services.Logs).Methods("GET")
	api.HandleFunc("/services/{name}", s.Services.Get).Methods("GET")
	api.HandleFunc("/services/{name}/{action}", s.Services.Action).
		Methods("POST").
		MatcherFunc(actionMatcher("start", "stop", "restart"))

	// Files
	api.HandleFunc("/files/list", s.Files.List).Methods("GET")
	api.HandleFunc("/files/read", s.Files.Read).Methods("GET")
	api.HandleFunc("/files/download", s.Files.Download).Methods("GET")
	api.HandleFunc("/files/write", s.Files.Write).Methods("POST")
	api.HandleFunc("/files/upload", s.Files.Upload).Methods("POST")
	api.HandleFunc("/files/mkdir", s.Files.Mkdir).Methods("POST")
	api.HandleFunc("/files/rename", s.Files.Rename).Methods("POST")
	api.HandleFunc("/files/delete", s.Files.DeleteEntry).Methods("DELETE")

	// Terminal (WebSocket) - protected but must not have the security
	// headers applied in a way that breaks the upgrade; handlers handle it.
	api.Handle("/terminal", s.Terminal)

	// ---- Static files (embedded) ----
	assets := http.FileServer(http.FS(vpsweb.StaticFS()))
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", assets))
	r.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/" {
			http.NotFound(w, req)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(vpsweb.IndexHTML())
	})

	return r
}

// actionMatcher restricts an {action} route variable to the given verbs.
func actionMatcher(verbs ...string) mux.MatcherFunc {
	allowed := map[string]bool{}
	for _, v := range verbs {
		allowed[v] = true
	}
	return func(r *http.Request, m *mux.RouteMatch) bool {
		v := mux.Vars(r)
		action := v["action"]
		return allowed[action]
	}
}
