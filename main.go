package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"newapiguard/internal/admin"
	"newapiguard/internal/cache"
	"newapiguard/internal/config"
	"newapiguard/internal/database"
	"newapiguard/internal/discord"
	"newapiguard/internal/newapi"
	"newapiguard/internal/proxy"
	"newapiguard/internal/settings"
	"newapiguard/internal/tasks"
	"newapiguard/internal/webutil"
)

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, x-api-key, api-key")
			w.Header().Set("Access-Control-Max-Age", "86400")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func main() {
	env := config.LoadEnv()

	db, err := database.Open(env.DBPath)
	if err != nil {
		log.Fatalf("打开数据库失败: %v", err)
	}
	defer db.Close()

	if err := database.Migrate(db); err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}

	systemSettings, err := settings.NewStore(db)
	if err != nil {
		log.Fatalf("加载系统配置失败: %v", err)
	}
	if env.AdminPassword != "" {
		_ = systemSettings.Update(map[string]string{"admin_password": env.AdminPassword})
	}
	if env.NewAPIAdminToken != "" {
		_ = systemSettings.Update(map[string]string{"newapi_admin_token": env.NewAPIAdminToken})
	}
	if env.NewAPIAdminUserID != "" {
		_ = systemSettings.Update(map[string]string{"newapi_admin_user_id": env.NewAPIAdminUserID})
	}
	if env.NewAPIURL != "" {
		_ = systemSettings.Update(map[string]string{"newapi_base_url": env.NewAPIURL})
	}

	runtimeCache := cache.New()
	newAPIClient := newapi.NewClient(systemSettings.GetString("newapi_base_url"), 30*time.Second)
	newAPIClient.SetAdminUserID(systemSettings.GetString("newapi_admin_user_id"))
	adminSessions := admin.NewPersistentSessionStore(db, env.SessionTTL)

	proxyHandler := proxy.NewHandler(env, db, systemSettings, runtimeCache, newAPIClient)
	checkinHandler := proxy.NewCheckinHandler(env, db, systemSettings, runtimeCache, newAPIClient)
	adminHandler := admin.NewHandler(env, db, systemSettings, runtimeCache, adminSessions, newAPIClient)
	discordHandler := discord.NewHandler(env, db, systemSettings, newAPIClient)

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		webutil.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
	})
	mux.Handle("/v1/", http.StripPrefix("/v1", proxyHandler))
	mux.Handle("/api/user/checkin", checkinHandler)
	mux.Handle("/guard/admin/", adminHandler)
	mux.Handle("/guard/static/", adminHandler)
	mux.Handle("/guard/api/", adminHandler)
	mux.Handle("/guard/oauth/", discordHandler)

	srv := &http.Server{
		Addr:              env.ListenAddr,
		Handler:           withCORS(mux),
		ReadHeaderTimeout: 10 * time.Second,
	}

	if env.EnableScheduler {
		tasks.Start(context.Background(), db, systemSettings, runtimeCache, newAPIClient)
	}

	go func() {
		log.Printf("Guard 已启动: %s", env.ListenAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("服务启动失败: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}
