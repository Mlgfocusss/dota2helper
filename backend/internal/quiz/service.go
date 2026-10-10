package quiz

import (
	"context"

	"d2helper/internal/dota"
)

// Service — квизы вместе с источниками, которые нужны для подбора героев.
type Service struct {
	data *Data
	tags *dota.HeroTagSet
	meta MetaSource
}

func NewService(data *Data, tags *dota.HeroTagSet, ms MetaSource) *Service {
	return &Service{data: data, tags: tags, meta: ms}
}

func (s *Service) PositionQuiz() PositionQuiz { return s.data.PositionForClient() }

func (s *Service) HeroQuestions(position int) ([]HeroQuestion, error) {
	return s.data.HeroQuestionsForClient(position)
}

func (s *Service) ScorePosition(answers map[string]string) (PositionResult, error) {
	return s.data.ScorePosition(answers)
}

func (s *Service) RecommendHeroes(ctx context.Context, position int, answers map[string][]string) ([]HeroPick, error) {
	return s.data.RecommendHeroes(ctx, position, answers, s.tags, s.meta)
}

// Info — название и описание позиции.
func (s *Service) Info(position int) PositionInfo { return s.data.Positions[position] }

// TopMeta — n сильнейших героев позиции по мете.
func (s *Service) TopMeta(ctx context.Context, position, n int) []MetaHero {
	return TopMetaHeroes(ctx, s.meta, s.tags, position, n)
}
