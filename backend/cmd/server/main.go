// Command server — HTTP API и фоновые джобы D2Helper.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	_ "time/tzdata" // Europe/Moscow для лидерборда — в образе нет базы часовых поясов

	"github.com/joho/godotenv"

	"d2helper/internal/analysis"
	"d2helper/internal/auth"
	"d2helper/internal/clients/dota2"
	"d2helper/internal/clients/opendota"
	"d2helper/internal/clients/steam"
	"d2helper/internal/clients/stratz"
	"d2helper/internal/clients/telegram"
	"d2helper/internal/coach"
	"d2helper/internal/config"
	"d2helper/internal/dota"
	"d2helper/internal/drafter"
	"d2helper/internal/games"
	"d2helper/internal/heropage"
	"d2helper/internal/httpapi"
	"d2helper/internal/jobs"
	"d2helper/internal/llm"
	"d2helper/internal/matchups"
	"d2helper/internal/meta"
	"d2helper/internal/news"
	"d2helper/internal/party"
	"d2helper/internal/patchnotes"
	"d2helper/internal/quiz"
	"d2helper/internal/store"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "-healthcheck" {
		os.Exit(healthcheck())
	}
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
}

func run() error {
	// .env нужен для локального `go run`; в Docker переменные задаёт compose,
	// а godotenv не перезаписывает уже заданные.
	_ = godotenv.Load(".env")
	_ = godotenv.Load("../.env")

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config:\n%w", err)
	}
	logger := newLogger(cfg)
	slog.SetDefault(logger)
	for _, w := range cfg.Warnings {
		logger.Warn(w)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := store.Connect(ctx, cfg.DatabaseURL, logger)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := store.Migrate(ctx, pool, logger); err != nil {
		return err
	}
	st := store.New(pool)

	od := opendota.New(cfg.OpenDotaAPIKey, cfg.OpenDotaRatePerMin, logger)
	steamAPI := steam.New(cfg.SteamAPIKey, logger)
	stratzAPI := stratz.New(cfg.StratzToken, cfg.StratzRatePerMin, logger)

	// справочник героев сразу из БД, свежий — фоновой джобой из OpenDota
	constants := dota.NewConstants(od, st, cfg.PatchLabel, logger)
	if err := constants.LoadCached(ctx); err != nil && !errors.Is(err, store.ErrNotFound) {
		logger.Warn("constants: cached copy unavailable", "err", err)
	}
	heroTags, err := dota.LoadHeroTags(filepath.Join(cfg.DataDir, "heroes_tags.json"))
	if err != nil {
		return err
	}
	warnMissingTags := missingTagsReporter(constants, heroTags, logger)
	warnMissingTags()

	metaSvc, err := meta.New(meta.Deps{
		Stratz:     stratzAPI,
		OpenDota:   od,
		Tags:       heroTags,
		Store:      st,
		Patch:      constants,
		Bracket:    cfg.MetaBracket,
		MinMatches: cfg.MetaMinMatches,
		Logger:     logger,
	})
	if err != nil {
		return err
	}
	newsSvc := news.New(steamAPI, st, logger)
	patchSvc := patchnotes.New(dota2.New(logger), logger)
	coachSvc := coach.New(od, steamAPI, st, constants, logger)
	gamesSvc := games.New(stratzAPI, od, st, logger)
	quizData, err := quiz.Load(cfg.DataDir)
	if err != nil {
		return err
	}
	quizSvc := quiz.NewService(quizData, heroTags, metaSvc)
	matchupsSvc := matchups.New(matchups.Deps{Stratz: stratzAPI, OpenDota: od, Heroes: constants, Store: st, Logger: logger})
	drafterSvc := drafter.New(drafter.Deps{
		Matrix: matchupsSvc,
		Meta:   metaSvc,
		Tags:   heroTags,
		Store:  st,
		Coef: drafter.Coefficients{
			Scale:         cfg.DrafterScoreScale,
			MetaWeight:    cfg.DrafterMetaWeight,
			OffPosPenalty: cfg.DrafterOffPosPenalty,
		},
		TrainingsPerHour: cfg.DrafterTrainingsPerHour,
		Logger:           logger,
	})
	heroPageSvc := heropage.New(heropage.Deps{
		Heroes: constants, Meta: metaSvc, Tags: heroTags, OpenDota: od, Store: st,
		MinMatches: cfg.MetaMinMatches, Logger: logger,
	})
	llmClient := llm.New(llm.Options{
		BaseURL: cfg.LLMBaseURL, APIKey: cfg.LLMAPIKey, Models: cfg.LLMModel,
		Referer: cfg.PublicURL, Title: cfg.AppName, Logger: logger,
	})
	analysisExample, err := analysis.LoadExample(cfg.DataDir)
	if err != nil {
		return err
	}
	analysisSvc := analysis.New(analysis.Deps{
		Store: st, OpenDota: od, LLM: llmClient, Heroes: constants, Logger: logger, Example: analysisExample,
		Config: analysis.Config{
			Enabled: cfg.FeatureDeepAnalysis, PerDay: cfg.AnalysesPerDay, MaxParallel: cfg.AnalysisMaxParallel,
		},
	})
	if analysisSvc.Available() {
		logger.Info("deep analysis enabled", "models", llmClient.Models(), "per_day", cfg.AnalysesPerDay)
	}
	partySvc := party.New(st, knownHero(constants), logger)
	var notifier httpapi.Notifier
	if tg := telegram.New(cfg.TelegramFeedbackBotToken, cfg.TelegramFeedbackChatID, logger); tg.Enabled() {
		notifier = tg
	}

	router := httpapi.NewRouter(httpapi.Deps{
		Config:       cfg,
		Logger:       logger,
		DB:           st,
		Users:        st,
		Sessions:     auth.NewSessions(st),
		States:       auth.NewStateSigner(cfg.SessionSecret),
		OpenID:       auth.NewOpenID(&http.Client{Timeout: 10 * time.Second}),
		Profiles:     auth.NewProfiles(steamAPI, od, logger),
		Constants:    constants,
		Meta:         metaSvc,
		News:         newsSvc,
		Comments:     st,
		Patches:      patchSvc,
		Coach:        coachSvc,
		Tracked:      st,
		Games:        gamesSvc,
		Quiz:         quizSvc,
		QuizResults:  st,
		Feedback:     st,
		Party:        partySvc,
		DrafterStats: st,
		Drafter:      drafterSvc,
		HeroPages:    heroPageSvc,
		Analysis:     analysisSvc,
		AnalysisRefs: st,
		Notifier:     notifier,
	})

	waitJobs := jobs.Start(ctx, logger,
		jobs.Job{
			Name:          "constants_refresh",
			Interval:      time.Hour, // обновление раз в сутки решает RefreshIfStale
			RetryInterval: 5 * time.Minute,
			Run: func(ctx context.Context) error {
				if err := constants.RefreshIfStale(ctx); err != nil {
					return err
				}
				warnMissingTags()
				return nil
			},
		},
		jobs.Job{
			Name:          "meta_positions",
			Interval:      30 * time.Minute, // пересборка раз в 3 часа — решает RefreshIfStale
			RetryInterval: 2 * time.Minute,
			Delay:         10 * time.Second, // сначала константы: нужен номер патча
			Run:           metaSvc.RefreshIfStale,
		},
		jobs.Job{
			Name:          "higher_lower_snapshot",
			Interval:      time.Hour, // раз в сутки решает RefreshIfStale
			RetryInterval: 5 * time.Minute,
			Delay:         20 * time.Second,
			Run:           gamesSvc.RefreshIfStale,
		},
		jobs.Job{
			Name:          "news_refresh",
			Interval:      news.RefreshEvery,
			RetryInterval: 5 * time.Minute,
			Delay:         5 * time.Second,
			Run:           newsSvc.Refresh,
		},
		jobs.Job{
			Name:          "matchups",
			Interval:      time.Hour, // пересборка раз в сутки — решает RefreshIfStale
			RetryInterval: 10 * time.Minute,
			Delay:         30 * time.Second, // сначала справочник героев
			Run:           matchupsSvc.RefreshIfStale,
		},
		jobs.Job{
			Name:     "drafter_cleanup",
			Interval: 6 * time.Hour,
			Delay:    3 * time.Minute,
			Run:      drafterSvc.Cleanup,
		},
		jobs.Job{
			Name:     "party_expire",
			Interval: time.Hour,
			Delay:    2 * time.Minute,
			Run:      partySvc.Expire,
		},
		jobs.Job{
			Name:     "sessions_cleanup",
			Interval: 6 * time.Hour,
			Delay:    time.Minute,
			Run: func(ctx context.Context) error {
				n, err := st.DeleteExpiredSessions(ctx)
				if n > 0 {
					logger.Info("expired sessions deleted", "count", n)
				}
				return err
			},
		},
		jobs.Job{
			Name:     "stratz_check",
			Interval: 6 * time.Hour,
			Run: func(ctx context.Context) error {
				if cfg.StratzToken == "" {
					return nil
				}
				err := stratzAPI.Check(ctx)
				if errors.Is(err, stratz.ErrUnavailable) {
					return nil // причина уже в логе, работаем через OpenDota
				}
				if err == nil {
					logger.Info("STRATZ доступен")
				}
				return err
			},
		},
	)

	// воркер очереди разборов работает, пока жив ctx
	var workers sync.WaitGroup
	if analysisSvc.Available() {
		workers.Add(1)
		go func() {
			defer workers.Done()
			analysisSvc.Run(ctx)
		}()
	}

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("http server started", "addr", cfg.HTTPAddr, "app", cfg.AppName)
		errCh <- srv.ListenAndServe()
	}()

	var serveErr error
	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			serveErr = fmt.Errorf("http server: %w", err)
		}
	case <-ctx.Done():
		logger.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		serveErr = srv.Shutdown(shutdownCtx)
		cancel()
	}
	stop()
	waitJobs()
	workers.Wait()
	return serveErr
}

// knownHero проверяет id героя по справочнику; пока справочник не загружен, принимает любой.
func knownHero(constants *dota.Constants) func(int) bool {
	return func(id int) bool {
		heroes, ok := constants.Heroes()
		if !ok {
			return true
		}
		for _, h := range heroes {
			if h.ID == id {
				return true
			}
		}
		return false
	}
}

// missingTagsReporter предупреждает о героях без записи в heroes_tags.json
// (новый герой в игре) — один раз на каждый новый набор.
func missingTagsReporter(constants *dota.Constants, tags *dota.HeroTagSet, logger *slog.Logger) func() {
	var reported string
	return func() {
		heroes, ok := constants.Heroes()
		if !ok {
			return
		}
		var names []string
		for _, h := range tags.Missing(heroes) {
			names = append(names, fmt.Sprintf("%s (%d)", h.LocalizedName, h.ID))
		}
		key := strings.Join(names, ", ")
		if key != "" && key != reported {
			logger.Warn("heroes without tags in heroes_tags.json", "heroes", key)
		}
		reported = key
	}
}

func newLogger(cfg config.Config) *slog.Logger {
	var level slog.Level
	_ = level.UnmarshalText([]byte(cfg.LogLevel)) // значение уже проверено в config
	opts := &slog.HandlerOptions{Level: level}
	if cfg.LogFormat == "json" {
		return slog.New(slog.NewJSONHandler(os.Stdout, opts))
	}
	return slog.New(slog.NewTextHandler(os.Stdout, opts))
}

// healthcheck — для HEALTHCHECK в docker compose: в distroless‑образе нет curl.
func healthcheck() int {
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return 1
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://127.0.0.1:"+port+"/api/health", nil)
	if err != nil {
		return 1
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 1
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}
