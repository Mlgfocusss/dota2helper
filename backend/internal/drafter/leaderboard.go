package drafter

import (
	"context"
	"sort"
	"time"

	"d2helper/internal/store"
)

// LeaderRow — строка топа драфтеров.
type LeaderRow struct {
	Rank   int     `json:"rank"`
	UserID int64   `json:"user_id"`
	Name   string  `json:"name"`
	Avatar *string `json:"avatar"`
	// Drafts — сколько тренировок оценено за месяц
	Drafts int `json:"drafts"`
	// Score — сумма 5 лучших результатов (у кого меньше 5 — всех)
	Score float64 `json:"score"`
}

// Board — топ месяца и строка просматривающего (если он участвовал).
type Board struct {
	// Month — «2026-10»
	Month string      `json:"month"`
	Items []LeaderRow `json:"items"`
	Me    *LeaderRow  `json:"me"`
}

// MonthRange разбирает «YYYY-MM» (пусто — текущий месяц) и возвращает границы месяца по Москве.
// Будущие месяцы и месяцы до запуска сервиса — ErrBadMonth.
func MonthRange(month string, now time.Time) (label string, from, to time.Time) {
	label, from, to, err := parseMonth(month, now)
	if err != nil {
		cur := now.In(Moscow)
		from = time.Date(cur.Year(), cur.Month(), 1, 0, 0, 0, 0, Moscow)
		return from.Format("2006-01"), from, from.AddDate(0, 1, 0)
	}
	return label, from, to
}

func parseMonth(month string, now time.Time) (string, time.Time, time.Time, error) {
	cur := now.In(Moscow)
	curStart := time.Date(cur.Year(), cur.Month(), 1, 0, 0, 0, 0, Moscow)
	if month == "" {
		return curStart.Format("2006-01"), curStart, curStart.AddDate(0, 1, 0), nil
	}
	t, err := time.ParseInLocation("2006-01", month, Moscow)
	if err != nil || t.After(curStart) || t.Year() < 2025 {
		return "", time.Time{}, time.Time{}, ErrBadMonth
	}
	return t.Format("2006-01"), t, t.AddDate(0, 1, 0), nil
}

// RankScores считает счёт каждого игрока (сумма BestScoresCount лучших результатов) и
// расставляет места: больше счёт — выше; при равном счёте больше тренировок, затем меньший id.
// Равный счёт делит место (1, 2, 2, 4).
func RankScores(rows []store.MonthUserScores) []LeaderRow {
	out := make([]LeaderRow, 0, len(rows))
	for _, r := range rows {
		scores := append([]float64(nil), r.Scores...)
		sort.Sort(sort.Reverse(sort.Float64Slice(scores)))
		var sum float64
		for i := 0; i < len(scores) && i < BestScoresCount; i++ {
			sum += scores[i]
		}
		row := LeaderRow{UserID: r.UserID, Name: r.PersonaName, Drafts: len(scores), Score: round1(sum)}
		if r.AvatarURL != "" {
			avatar := r.AvatarURL
			row.Avatar = &avatar
		}
		out = append(out, row)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		switch {
		case a.Score != b.Score:
			return a.Score > b.Score
		case a.Drafts != b.Drafts:
			return a.Drafts > b.Drafts
		}
		return a.UserID < b.UserID
	})
	for i := range out {
		if i > 0 && out[i].Score == out[i-1].Score {
			out[i].Rank = out[i-1].Rank
		} else {
			out[i].Rank = i + 1
		}
	}
	return out
}

func (s *Service) invalidateBoard(month string) {
	s.boardMu.Lock()
	delete(s.boards, month)
	s.boardMu.Unlock()
}

// rows — все места месяца (кэш на 5 минут).
func (s *Service) rows(ctx context.Context, month string) ([]LeaderRow, error) {
	label, from, to, err := parseMonth(month, s.now())
	if err != nil {
		return nil, err
	}
	s.boardMu.Lock()
	if e, ok := s.boards[label]; ok && s.now().Sub(e.at) < boardTTL {
		s.boardMu.Unlock()
		return e.rows, nil
	}
	s.boardMu.Unlock()

	scores, err := s.d.Store.MonthScores(ctx, from, to)
	if err != nil {
		return nil, err
	}
	ranked := RankScores(scores)
	s.boardMu.Lock()
	s.boards[label] = boardEntry{rows: ranked, at: s.now()}
	s.boardMu.Unlock()
	return ranked, nil
}

// Leaderboard — топ-100 месяца и строка viewer (viewerID = 0 — гость).
func (s *Service) Leaderboard(ctx context.Context, month string, viewerID int64) (*Board, error) {
	label, _, _, err := parseMonth(month, s.now())
	if err != nil {
		return nil, err
	}
	rows, err := s.rows(ctx, label)
	if err != nil {
		return nil, err
	}
	board := &Board{Month: label, Items: rows[:min(len(rows), LeaderboardSize)]}
	if board.Items == nil {
		board.Items = []LeaderRow{}
	}
	if viewerID != 0 {
		for i := range rows {
			if rows[i].UserID == viewerID {
				me := rows[i]
				board.Me = &me
				break
			}
		}
	}
	return board, nil
}
