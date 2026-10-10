// Package news — новости Dota 2 из Steam: загрузка, перевод BBCode в безопасный HTML, выдача.
package news

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"d2helper/internal/clients/steam"
	"d2helper/internal/store"
)

const (
	dotaAppID    = 570
	fetchCount   = 20
	excerptChars = 220
	// RefreshEvery — период джобы news_refresh (SPEC 3.6)
	RefreshEvery = 30 * time.Minute
)

var ErrNotFound = errors.New("news: not found")

type (
	Fetcher interface {
		News(ctx context.Context, appID, count int) ([]steam.NewsItem, error)
	}
	Store interface {
		UpsertNews(ctx context.Context, items []store.News) error
		ListNews(ctx context.Context, limit int) ([]store.News, error)
		NewsByGID(ctx context.Context, gid string) (store.News, error)
	}
)

type Service struct {
	fetcher Fetcher
	store   Store
	logger  *slog.Logger
}

func New(f Fetcher, s Store, logger *slog.Logger) *Service {
	return &Service{fetcher: f, store: s, logger: logger}
}

// Refresh загружает свежие анонсы Steam и сохраняет их в БД.
func (s *Service) Refresh(ctx context.Context) error {
	items, err := s.fetcher.News(ctx, dotaAppID, fetchCount)
	if err != nil {
		return fmt.Errorf("fetch news: %w", err)
	}
	records := make([]store.News, 0, len(items))
	for _, it := range items {
		if it.GID == "" || it.Title == "" {
			continue
		}
		records = append(records, Convert(it))
	}
	if len(records) == 0 {
		return nil
	}
	if err := s.store.UpsertNews(ctx, records); err != nil {
		return fmt.Errorf("save news: %w", err)
	}
	s.logger.Debug("news refreshed", "count", len(records))
	return nil
}

// Convert превращает анонс Steam в запись для БД.
func Convert(it steam.NewsItem) store.News {
	return store.News{
		GID:     it.GID,
		Title:   it.Title,
		URL:     it.URL,
		Date:    time.Unix(it.Date, 0).UTC(),
		HTML:    Sanitize(ToHTML(it.Contents)),
		Excerpt: Excerpt(it.Contents, excerptChars),
	}
}

func (s *Service) List(ctx context.Context, limit int) ([]store.News, error) {
	return s.store.ListNews(ctx, limit)
}

func (s *Service) Get(ctx context.Context, gid string) (store.News, error) {
	n, err := s.store.NewsByGID(ctx, gid)
	if errors.Is(err, store.ErrNotFound) {
		return store.News{}, ErrNotFound
	}
	return n, err
}
