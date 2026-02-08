package handler

import (
	"errors"
	"net/http"

	"github.com/andrewshostak/result-service/errs"
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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "code": errs.CodeInvalidRequest})

		return
	}

	err := h.subscriptionService.Create(c.Request.Context(), params.ToDomain())
	if errors.As(err, &errs.ResourceNotFoundError{}) {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "code": errs.CodeResourceNotFound})

		return
	}

	if errors.As(err, &errs.UnprocessableContentError{}) {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error(), "code": errs.CodeUnprocessableContent})

		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "code": errs.CodeInternalServerError})

		return
	}

	c.Status(http.StatusNoContent)
}

func (h *SubscriptionHandler) Delete(c *gin.Context) {
	var params DeleteSubscriptionRequest
	if err := c.ShouldBindQuery(&params); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "code": errs.CodeInvalidRequest})
		return
	}

	err := h.subscriptionService.Delete(c.Request.Context(), params.ToDomain())
	if errors.As(err, &errs.ResourceNotFoundError{}) {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "code": errs.CodeResourceNotFound})

		return
	}

	if errors.As(err, &errs.UnprocessableContentError{}) {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error(), "code": errs.CodeUnprocessableContent})

		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "code": errs.CodeInternalServerError})

		return
	}

	c.Status(http.StatusNoContent)
}
