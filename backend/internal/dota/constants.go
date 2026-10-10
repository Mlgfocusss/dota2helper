package dota

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"d2helper/internal/clients/opendota"
)

const (
	kvHeroes = "constants:heroes"
	kvPatch  = "constants:patch"
	// RefreshEvery — как часто перечитывать константы из OpenDota
	RefreshEvery = 24 * time.Hour
)

// KV — хранилище, где константы переживают перезапуск сервера.
type KV interface {
	GetKV(ctx context.Context, key string) ([]byte, time.Time, error)
	PutKV(ctx context.Context, key string, value []byte) error
}

// OpenDotaConstants — то, что нужно от клиента OpenDota.
type OpenDotaConstants interface {
	HeroConstants(ctx context.Context) (map[string]opendota.HeroConstant, error)
	PatchConstants(ctx context.Context) ([]opendota.PatchConstant, error)
}

type constantsState struct {
	heroes     []Hero
	byID       map[int]Hero
	heroesJSON []byte
	heroesETag string
	patch      Patch
	patchStart time.Time
	updatedAt  time.Time
}

// Constants держит справочник героев и текущий патч в памяти. При старте
// данные берутся из БД (быстро и без внешних запросов), затем обновляются
// из OpenDota раз в сутки.
type Constants struct {
	od         OpenDotaConstants
	kv         KV
	patchLabel string
	logger     *slog.Logger
	state      atomic.Pointer[constantsState]
	now        func() time.Time
}

func NewConstants(od OpenDotaConstants, kv KV, patchLabel string, logger *slog.Logger) *Constants {
	return &Constants{od: od, kv: kv, patchLabel: patchLabel, logger: logger, now: time.Now}
}

type cachedPatch struct {
	Number string    `json:"number"`
	Date   time.Time `json:"date"`
}

// LoadCached читает константы, сохранённые в БД при прошлом обновлении.
func (c *Constants) LoadCached(ctx context.Context) error {
	heroesRaw, updatedAt, err := c.kv.GetKV(ctx, kvHeroes)
	if err != nil {
		return fmt.Errorf("load cached heroes: %w", err)
	}
	patchRaw, _, err := c.kv.GetKV(ctx, kvPatch)
	if err != nil {
		return fmt.Errorf("load cached patch: %w", err)
	}
	var heroes []Hero
	if err := json.Unmarshal(heroesRaw, &heroes); err != nil {
		return fmt.Errorf("decode cached heroes: %w", err)
	}
	var p cachedPatch
	if err := json.Unmarshal(patchRaw, &p); err != nil {
		return fmt.Errorf("decode cached patch: %w", err)
	}
	return c.set(heroes, p.Number, p.Date, updatedAt)
}

// Refresh загружает константы из OpenDota и сохраняет их в БД.
func (c *Constants) Refresh(ctx context.Context) error {
	heroConsts, err := c.od.HeroConstants(ctx)
	if err != nil {
		return fmt.Errorf("fetch heroes: %w", err)
	}
	patches, err := c.od.PatchConstants(ctx)
	if err != nil {
		return fmt.Errorf("fetch patches: %w", err)
	}
	heroes := HeroesFromOpenDota(heroConsts)
	number, ok := LatestPatch(patches)
	if len(heroes) == 0 || !ok {
		return errors.New("opendota returned empty constants")
	}
	date := patchDate(patches, number)
	if err := c.set(heroes, number, date, c.now()); err != nil {
		return err
	}

	st := c.state.Load()
	patchRaw, _ := json.Marshal(cachedPatch{Number: number, Date: date})
	if err := c.kv.PutKV(ctx, kvHeroes, st.heroesJSON); err != nil {
		c.logger.Warn("save heroes to db", "err", err)
	}
	if err := c.kv.PutKV(ctx, kvPatch, patchRaw); err != nil {
		c.logger.Warn("save patch to db", "err", err)
	}
	c.logger.Info("constants refreshed", "heroes", len(heroes), "patch", number)
	return nil
}

// RefreshIfStale — для фоновой джобы: не ходить в OpenDota при каждом перезапуске.
func (c *Constants) RefreshIfStale(ctx context.Context) error {
	// без даты патча (константы сохранены старой версией) перечитываем сразу
	if st := c.state.Load(); st != nil && !st.patchStart.IsZero() && c.now().Sub(st.updatedAt) < RefreshEvery-time.Hour {
		return nil
	}
	return c.Refresh(ctx)
}

func (c *Constants) set(heroes []Hero, patchNumber string, patchDate, updatedAt time.Time) error {
	body, err := json.Marshal(heroes)
	if err != nil {
		return fmt.Errorf("encode heroes: %w", err)
	}
	sum := sha256.Sum256(body)
	byID := make(map[int]Hero, len(heroes))
	for _, h := range heroes {
		byID[h.ID] = h
	}
	c.state.Store(&constantsState{
		heroes:     heroes,
		byID:       byID,
		heroesJSON: body,
		heroesETag: `"` + hex.EncodeToString(sum[:8]) + `"`,
		patch:      Patch{Number: patchNumber, Label: PatchLabel(patchNumber, c.patchLabel)},
		patchStart: patchDate,
		updatedAt:  updatedAt,
	})
	return nil
}

// Heroes — все герои по алфавиту; false, пока константы не загружены.
func (c *Constants) Heroes() ([]Hero, bool) {
	st := c.state.Load()
	if st == nil {
		return nil, false
	}
	return st.heroes, true
}

func (c *Constants) Hero(id int) (Hero, bool) {
	st := c.state.Load()
	if st == nil {
		return Hero{}, false
	}
	h, ok := st.byID[id]
	return h, ok
}

// HeroesJSON — готовый ответ для /api/heroes и его ETag.
func (c *Constants) HeroesJSON() (body []byte, etag string, ok bool) {
	st := c.state.Load()
	if st == nil {
		return nil, "", false
	}
	return st.heroesJSON, st.heroesETag, true
}

func (c *Constants) Patch() (Patch, bool) {
	st := c.state.Load()
	if st == nil {
		return Patch{}, false
	}
	return st.patch, true
}

// PatchStart — когда вышел текущий патч; false, если дата неизвестна.
func (c *Constants) PatchStart() (time.Time, bool) {
	st := c.state.Load()
	if st == nil || st.patchStart.IsZero() {
		return time.Time{}, false
	}
	return st.patchStart, true
}

func patchDate(patches []opendota.PatchConstant, number string) time.Time {
	for _, p := range patches {
		if p.Name == number {
			return p.Date
		}
	}
	return time.Time{}
}
