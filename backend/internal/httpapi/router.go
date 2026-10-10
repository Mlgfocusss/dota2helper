// Package httpapi собирает HTTP API: роутер, middleware и хендлеры по фичам.
package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"d2helper/internal/auth"
	"d2helper/internal/config"
	"d2helper/internal/dota"
	"d2helper/internal/httpapi/middleware"
	"d2helper/internal/httpapi/render"
	"d2helper/internal/meta"
	"d2helper/internal/patchnotes"
	"d2helper/internal/store"
)

// Зависимости хендлеров описаны интерфейсами, чтобы их можно было подменить в тестах.
type (
	Pinger interface {
		Ping(ctx context.Context) error
	}
	UserStore interface {
		UpsertSteamUser(ctx context.Context, steamID64 string, accountID int64, persona, avatar string) (store.User, error)
	}
	SessionService interface {
		Create(ctx context.Context, userID int64) (string, time.Time, error)
		Resolve(ctx context.Context, token string) (store.User, time.Time, error)
		Delete(ctx context.Context, token string) error
	}
	OpenIDProvider interface {
		LoginURL(realm, returnTo string) string
		Verify(ctx context.Context, query url.Values, expectedReturnTo string) (string, error)
	}
	ProfileLookup interface {
		Lookup(ctx context.Context, steamID64 string, accountID int64) (persona, avatar string)
	}
	HeroConstants interface {
		HeroesJSON() (body []byte, etag string, ok bool)
		Patch() (dota.Patch, bool)
	}
	MetaSource interface {
		Snapshot(ctx context.Context) (*meta.Snapshot, error)
	}
	NewsSource interface {
		List(ctx context.Context, limit int) ([]store.News, error)
		Get(ctx context.Context, gid string) (store.News, error)
	}
	CommentStore interface {
		ListComments(ctx context.Context, gid string, before int64, limit int) ([]store.Comment, int, error)
		CreateComment(ctx context.Context, gid string, userID int64, text string) (store.Comment, error)
		DeleteComment(ctx context.Context, gid string, id, userID int64) error
	}
	PatchSource interface {
		Get(ctx context.Context, version string) (*patchnotes.Patch, error)
	}
	FeedbackStore interface {
		CreateFeedback(ctx context.Context, userID *int64, text, contact string) (int64, error)
	}
	Notifier interface {
		Enabled() bool
		Send(ctx context.Context, text string) error
	}
)

type Deps struct {
	Config       config.Config
	Logger       *slog.Logger
	DB           Pinger
	Users        UserStore
	Sessions     SessionService
	States       *auth.StateSigner
	OpenID       OpenIDProvider
	Profiles     ProfileLookup
	Constants    HeroConstants
	Meta         MetaSource
	News         NewsSource
	Comments     CommentStore
	Patches      PatchSource
	Coach        CoachService
	Tracked      TrackedStore
	Games        GamesService
	Quiz         QuizService
	QuizResults  QuizResults
	Feedback     FeedbackStore
	Party        PartyService
	DrafterStats DrafterStats
	Drafter      DrafterService
	HeroPages    HeroPageService
	Analysis     AnalysisService
	AnalysisRefs AnalysisRefs
	// Notifier дублирует отзывы в Telegram; nil — не дублировать
	Notifier Notifier
}

type api struct {
	cfg           config.Config
	logger        *slog.Logger
	db            Pinger
	users         UserStore
	sessions      SessionService
	states        *auth.StateSigner
	openid        OpenIDProvider
	profiles      ProfileLookup
	constants     HeroConstants
	meta          MetaSource
	news          NewsSource
	comments      CommentStore
	patches       PatchSource
	coach         CoachService
	tracked       TrackedStore
	games         GamesService
	quiz          QuizService
	quizResults   QuizResults
	feedbackStore FeedbackStore
	party         PartyService
	drafterStats  DrafterStats
	drafter       DrafterService
	heroPages     HeroPageService
	analysis      AnalysisService
	analysisRefs  AnalysisRefs
	notifier      Notifier
}

func NewRouter(d Deps) http.Handler {
	a := &api{
		cfg:           d.Config,
		logger:        d.Logger,
		db:            d.DB,
		users:         d.Users,
		sessions:      d.Sessions,
		states:        d.States,
		openid:        d.OpenID,
		profiles:      d.Profiles,
		constants:     d.Constants,
		meta:          d.Meta,
		news:          d.News,
		comments:      d.Comments,
		patches:       d.Patches,
		coach:         d.Coach,
		tracked:       d.Tracked,
		games:         d.Games,
		quiz:          d.Quiz,
		quizResults:   d.QuizResults,
		feedbackStore: d.Feedback,
		party:         d.Party,
		drafterStats:  d.DrafterStats,
		drafter:       d.Drafter,
		heroPages:     d.HeroPages,
		analysis:      d.Analysis,
		analysisRefs:  d.AnalysisRefs,
		notifier:      d.Notifier,
	}

	r := chi.NewRouter()
	r.Use(chimw.ClientIPFromHeader("X-Real-IP"))
	r.Use(chimw.RequestID)
	r.Use(middleware.Logging(d.Logger))
	r.Use(middleware.Recover(d.Logger))
	r.Use(middleware.NoStore)
	r.Use(middleware.CheckOrigin(d.Config.PublicURL, "/api/payments/webhook/"))
	r.Use(middleware.LimitMutations(middleware.NewRateLimiter(30, time.Minute)))
	r.Use(middleware.OptionalAuth(d.Sessions, d.Config.CookieSecure, d.Logger))
	r.NotFound(render.NotFound)
	r.MethodNotAllowed(render.MethodNotAllowed)

	r.Route("/api", func(r chi.Router) {
		r.Get("/health", a.health)
		r.Get("/config", a.publicConfig)
		r.Get("/me", a.me)
		r.Get("/heroes", a.heroes)
		r.Get("/heroes/{hero_id}/page", a.heroPage)
		r.Get("/patch", a.patch)
		r.Get("/meta/positions", a.metaPositions)
		r.Get("/news", a.newsList)
		r.Get("/news/{gid}", a.newsGet)
		r.Get("/news/{gid}/comments", a.commentsList)
		r.With(middleware.RequireAuth, middleware.Limit(middleware.NewRateLimiter(6, time.Minute))).
			Post("/news/{gid}/comments", a.commentCreate)
		r.With(middleware.RequireAuth).Delete("/news/{gid}/comments/{id}", a.commentDelete)
		r.Get("/patches/{version}", a.patchGet)

		r.With(middleware.Limit(middleware.NewRateLimiter(10, time.Minute))).Post("/profile/resolve", a.profileResolve)
		r.With(middleware.RequireAuth).Put("/profile/tracked", a.profileTracked)
		r.Route("/coach/{account_id}", func(r chi.Router) {
			r.Get("/summary", a.coachSummary)
			r.Get("/matches", a.coachMatches)
			r.Post("/refresh", a.coachRefresh)
		})
		r.Get("/matches/{match_id}", a.matchGet)

		r.Get("/games/higher-lower/{mode}", a.higherLowerSnapshot)
		r.With(middleware.RequireAuth).Post("/games/higher-lower/{mode}/score", a.higherLowerScore)
		r.Get("/games/records", a.gameRecords)

		r.Get("/quiz/position", a.quizPosition)
		r.Get("/quiz/position/last", a.quizPositionLast)
		r.Get("/quiz/hero/{position}", a.quizHero)
		r.Post("/quiz/position/result", a.quizPositionResult)
		r.Post("/quiz/hero/result", a.quizHeroResult)
		// отзывы — отдельный, более строгий лимит против спама
		r.With(middleware.Limit(middleware.NewRateLimiter(5, 10*time.Minute))).Post("/feedback", a.feedback)

		r.With(middleware.RequireAuth).Get("/profile/stats", a.profileStats)

		r.Route("/analyses", func(r chi.Router) {
			r.Get("/example", a.analysisExample)
			r.With(middleware.RequireAuth).Get("/", a.analysisList)
			r.With(middleware.RequireAuth).Get("/quota", a.analysisQuota)
			r.With(middleware.RequireAuth, middleware.Limit(middleware.NewRateLimiter(10, time.Minute))).Post("/", a.analysisCreate)
			r.With(middleware.RequireAuth).Get("/{id}", a.analysisGet)
		})

		r.Route("/drafter", func(r chi.Router) {
			r.Get("/matrix", a.drafterMatrix)
			r.With(middleware.Limit(middleware.NewRateLimiter(30, time.Minute))).Post("/analyze", a.drafterAnalyze)
			r.With(middleware.Limit(middleware.NewRateLimiter(20, time.Minute))).Post("/training/new", a.trainingNew)
			r.With(middleware.RequireAuth).Post("/training/{id}/submit", a.trainingSubmit)
			r.With(middleware.RequireAuth).Get("/history", a.drafterHistory)
			r.Get("/leaderboard", a.drafterLeaderboard)
		})

		r.Route("/party", func(r chi.Router) {
			r.Use(middleware.RequireAuth)
			r.Get("/", a.partySearch)
			r.Route("/me", func(r chi.Router) {
				r.Get("/", a.partyMe)
				r.Put("/", a.partySave)
				r.Patch("/", a.partyActive)
				r.Delete("/", a.partyDelete)
				r.Post("/bump", a.partyBump)
			})
			// жалобы — отдельный лимит против массовых атак на чужие анкеты
			r.With(middleware.Limit(middleware.NewRateLimiter(10, 10*time.Minute))).
				Post("/{user_id}/report", a.partyReport)
		})

		r.Route("/auth", func(r chi.Router) {
			// вход дергает Steam API — ограничиваем и GET‑запросы
			r.Use(middleware.Limit(middleware.NewRateLimiter(20, time.Minute)))
			r.Get("/steam/login", a.steamLogin)
			r.Get("/steam/callback", a.steamCallback)
			r.Post("/logout", a.logout)
		})
	})
	return r
}
