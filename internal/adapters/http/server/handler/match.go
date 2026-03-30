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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "code": models.CodeInvalidRequest})

		return
	}

	result, err := h.matchService.Create(c.Request.Context(), params.ToDomain())
	if errors.As(err, &models.UnprocessableContentError{}) {
		c.JSON(http.StatusUnprocessableEntity, NewErrorResponse(models.CodeUnprocessableContent, err))

		return
	}

	if errors.As(err, &models.ResourceNotFoundError{}) {
		c.JSON(http.StatusBadRequest, NewErrorResponse(models.CodeResourceNotFound, err))

		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, NewErrorResponse(models.CodeInternalServerError, err))

		return
	}

	c.JSON(http.StatusOK, gin.H{"match_id": result})
}
