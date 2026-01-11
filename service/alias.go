package service

import (
	"context"
	"fmt"
)

type AliasService struct {
	aliasRepository AliasRepository
	logger          Logger
}

func NewAliasService(aliasRepository AliasRepository, logger Logger) *AliasService {
	return &AliasService{
		aliasRepository: aliasRepository,
		logger:          logger,
	}
}

func (s *AliasService) Search(ctx context.Context, alias string) ([]string, error) {
	result, err := s.aliasRepository.Search(ctx, alias)
	if err != nil {
		return nil, fmt.Errorf("failed to search aliases: %w", err)
	}

	aliases := make([]string, 0, len(result))
	for i := range result {
		aliases = append(aliases, result[i].Alias)
	}

	return aliases, nil
}
