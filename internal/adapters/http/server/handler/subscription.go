package handler

import (
	"errors"
	"net/http"

	"github.com/andrewshostak/result-service/internal/app/models"
	"github.com/gin-gonic/gin"
)

type SubscriptionHandler struct {
	subscriptionService SubscriptionService
}

func NewSubscriptionHandler(subscriptionService SubscriptionService) *SubscriptionHandler {
	return &SubscriptionHandler{subscriptionService: subscriptionService}
}

func (h *SubscriptionHandler) Create(c *gin.Context) {
	var params CreateSubscriptionRequest
	if err := c.ShouldBindJSON(&params); err != nil {
		RespondError(c, http.StatusBadRequest, models.CodeInvalidRequest, err)

		return
	}

	err := h.subscriptionService.Create(c.Request.Context(), params.ToDomain())
	if errors.As(err, &models.ResourceNotFoundError{}) {
		RespondError(c, http.StatusBadRequest, models.CodeResourceNotFound, err)

		return
	}

	if errors.As(err, &models.UnprocessableContentError{}) {
		RespondError(c, http.StatusUnprocessableEntity, models.CodeUnprocessableContent, err)

		return
	}

	if err != nil {
		RespondError(c, http.StatusInternalServerError, models.CodeInternalServerError, err)

		return
	}

	c.Status(http.StatusNoContent)
}

func (h *SubscriptionHandler) Delete(c *gin.Context) {
	var params DeleteSubscriptionRequest
	if err := c.ShouldBindQuery(&params); err != nil {
		RespondError(c, http.StatusBadRequest, models.CodeInvalidRequest, err)
		return
	}

	err := h.subscriptionService.Delete(c.Request.Context(), params.ToDomain())
	if errors.As(err, &models.ResourceNotFoundError{}) {
		RespondError(c, http.StatusBadRequest, models.CodeResourceNotFound, err)

		return
	}

	if errors.As(err, &models.UnprocessableContentError{}) {
		RespondError(c, http.StatusUnprocessableEntity, models.CodeUnprocessableContent, err)

		return
	}

	if err != nil {
		RespondError(c, http.StatusInternalServerError, models.CodeInternalServerError, err)

		return
	}

	c.Status(http.StatusNoContent)
}
