package service

import (
	"xianlv/internal/model"
	"xianlv/internal/storage"
)

type Ranking struct{ repo *storage.RankRepository }

func NewRanking(repo *storage.RankRepository) *Ranking         { return &Ranking{repo: repo} }
func (s *Ranking) Refresh() error                              { return s.repo.Refresh() }
func (s *Ranking) List(kind string) ([]model.RankEntry, error) { return s.repo.List(kind) }
