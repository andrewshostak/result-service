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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "code": models.CodeInvalidRequest})

		return
	}

	err := h.subscriptionService.Create(c.Request.Context(), params.ToDomain())
	if errors.As(err, &models.ResourceNotFoundError{}) {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "code": models.CodeResourceNotFound})

		return
	}

	if errors.As(err, &models.UnprocessableContentError{}) {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error(), "code": models.CodeUnprocessableContent})

		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "code": models.CodeInternalServerError})

		return
	}

	c.Status(http.StatusNoContent)
}

func (h *SubscriptionHandler) Delete(c *gin.Context) {
	var params DeleteSubscriptionRequest
	if err := c.ShouldBindQuery(&params); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "code": models.CodeInvalidRequest})
		return
	}

	err := h.subscriptionService.Delete(c.Request.Context(), params.ToDomain())
	if errors.As(err, &models.ResourceNotFoundError{}) {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "code": models.CodeResourceNotFound})

		return
	}

	if errors.As(err, &models.UnprocessableContentError{}) {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error(), "code": models.CodeUnprocessableContent})

		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "code": models.CodeInternalServerError})

		return
	}

	c.Status(http.StatusNoContent)
}
