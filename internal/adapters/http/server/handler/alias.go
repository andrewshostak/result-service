package handler

import (
	"net/http"

	"github.com/andrewshostak/result-service/internal/app/models"
	"github.com/gin-gonic/gin"
)

type AliasHandler struct {
	aliasService AliasService
}

func NewAliasHandler(aliasService AliasService) *AliasHandler {
	return &AliasHandler{aliasService: aliasService}
}

func (h *AliasHandler) Search(c *gin.Context) {
	var params SearchAliasRequest
	if err := c.ShouldBindQuery(&params); err != nil {
		RespondError(c, http.StatusBadRequest, models.CodeInvalidRequest, err)

		return
	}

	result, err := h.aliasService.Search(c.Request.Context(), params.Search)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, models.CodeInternalServerError, err)

		return
	}

	c.JSON(http.StatusOK, gin.H{"aliases": result})
}
