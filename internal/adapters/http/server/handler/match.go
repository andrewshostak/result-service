package handler

import (
	"errors"
	"net/http"

	"github.com/andrewshostak/result-service/internal/app/models"
	"github.com/gin-gonic/gin"
)

type MatchHandler struct {
	matchService MatchService
}

func NewMatchHandler(matchService MatchService) *MatchHandler {
	return &MatchHandler{matchService: matchService}
}

func (h *MatchHandler) Create(c *gin.Context) {
	var params CreateMatchRequest
	if err := c.ShouldBindJSON(&params); err != nil {
		RespondError(c, http.StatusBadRequest, models.CodeInvalidRequest, err)

		return
	}

	result, err := h.matchService.Create(c.Request.Context(), params.ToDomain())
	if errors.As(err, &models.UnprocessableContentError{}) {
		RespondError(c, http.StatusUnprocessableEntity, models.CodeUnprocessableContent, err)

		return
	}

	if errors.As(err, &models.ResourceNotFoundError{}) {
		RespondError(c, http.StatusBadRequest, models.CodeResourceNotFound, err)

		return
	}

	if err != nil {
		RespondError(c, http.StatusInternalServerError, models.CodeInternalServerError, err)

		return
	}

	c.JSON(http.StatusOK, gin.H{"match_id": result})
}
