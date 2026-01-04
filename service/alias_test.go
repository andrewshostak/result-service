package service_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/andrewshostak/result-service/repository"
	"github.com/andrewshostak/result-service/service"
	"github.com/andrewshostak/result-service/service/mocks"
	"github.com/brianvoe/gofakeit/v6"
	"github.com/stretchr/testify/assert"
)

func TestAliasService_Search(t *testing.T) {
	ctx := context.Background()
	alias := gofakeit.Word()
	aliases := []repository.Alias{
		{ID: 1, TeamID: 1, Alias: gofakeit.Name()},
		{ID: 2, TeamID: 2, Alias: gofakeit.Name()},
	}

	logger := mocks.NewLogger(t)

	tests := []struct {
		name            string
		aliasRepository func(t *testing.T) *mocks.AliasRepository
		result          []string
		expectedErr     error
	}{
		{
			name: "it returns an error when aliases search fails",
			aliasRepository: func(t *testing.T) *mocks.AliasRepository {
				t.Helper()
				m := mocks.NewAliasRepository(t)
				m.On("Search", ctx, alias).Return(nil, errors.New("repo error")).Once()
				return m
			},
			expectedErr: fmt.Errorf("failed to search aliases: %w", errors.New("repo error")),
		},
		{
			name: "it returns mapped aliases when aliases search returns aliases",
			aliasRepository: func(t *testing.T) *mocks.AliasRepository {
				t.Helper()
				m := mocks.NewAliasRepository(t)
				m.On("Search", ctx, alias).Return(aliases, nil).Once()
				return m
			},
			result: []string{aliases[0].Alias, aliases[1].Alias},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := service.NewAliasService(tt.aliasRepository(t), logger)
			actual, err := s.Search(ctx, alias)
			assert.Equal(t, tt.result, actual)
			if tt.expectedErr != nil {
				assert.EqualError(t, err, tt.expectedErr.Error())
			}
		})
	}
}
