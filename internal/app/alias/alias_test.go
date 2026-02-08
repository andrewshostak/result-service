package alias_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/andrewshostak/result-service/internal/app/alias"
	"github.com/andrewshostak/result-service/internal/app/alias/mocks"
	"github.com/andrewshostak/result-service/internal/app/models"
	"github.com/brianvoe/gofakeit/v6"
	"github.com/stretchr/testify/assert"
)

func TestAliasService_Search(t *testing.T) {
	ctx := context.Background()
	fakeAlias := gofakeit.Word()
	aliases := []models.Alias{
		{TeamID: 1, Alias: gofakeit.Name()},
		{TeamID: 2, Alias: gofakeit.Name()},
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
				m.On("Search", ctx, fakeAlias).Return(nil, errors.New("repo error")).Once()
				return m
			},
			expectedErr: fmt.Errorf("failed to search aliases: %w", errors.New("repo error")),
		},
		{
			name: "it returns mapped aliases when aliases search returns aliases",
			aliasRepository: func(t *testing.T) *mocks.AliasRepository {
				t.Helper()
				m := mocks.NewAliasRepository(t)
				m.On("Search", ctx, fakeAlias).Return(aliases, nil).Once()
				return m
			},
			result: []string{aliases[0].Alias, aliases[1].Alias},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := alias.NewAliasService(tt.aliasRepository(t), logger)
			actual, err := s.Search(ctx, fakeAlias)
			assert.Equal(t, tt.result, actual)
			if tt.expectedErr != nil {
				assert.EqualError(t, err, tt.expectedErr.Error())
			}
		})
	}
}
