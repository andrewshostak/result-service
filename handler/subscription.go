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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})

		return
	}

	err := h.subscriptionService.Create(c.Request.Context(), params.ToDomain())
	if errors.As(err, &errs.SubscriptionAlreadyExistsError{}) {
		c.Status(http.StatusNoContent)

		return
	}

	if errors.As(err, &errs.WrongMatchIDError{}) {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})

		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})

		return
	}

	c.Status(http.StatusNoContent)
}

func (h *SubscriptionHandler) Delete(c *gin.Context) {
	var params DeleteSubscriptionRequest
	if err := c.ShouldBindQuery(&params); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.subscriptionService.Delete(c.Request.Context(), params.ToDomain())
	if errors.As(err, &errs.AliasNotFoundError{}) {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})

		return
	}

	if errors.As(err, &errs.MatchNotFoundError{}) {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})

		return
	}

	if errors.As(err, &errs.SubscriptionNotFoundError{}) {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})

		return
	}

	if errors.As(err, &errs.SubscriptionWrongStatusError{}) {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})

		return
	}

	if errors.As(err, &errs.SubscriptionDeleteNotAllowedError{}) {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})

		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})

		return
	}

	c.Status(http.StatusNoContent)
}
