package handler

import (
	"errors"
	"net/http"

	"github.com/andrewshostak/result-service/internal/app/models"
	"github.com/gin-gonic/gin"
)

type TriggerHandler struct {
	checkResultService        ResultCheckerService
	subscriberNotifierService SubscriberNotifierService
}

func NewTriggerHandler(checkResultService ResultCheckerService, subscriberNotifierService SubscriberNotifierService) *TriggerHandler {
	return &TriggerHandler{
		checkResultService:        checkResultService,
		subscriberNotifierService: subscriberNotifierService,
	}
}

func (h *TriggerHandler) CheckResult(c *gin.Context) {
	var params TriggerResultCheckRequest
	if err := c.ShouldBindJSON(&params); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "code": models.CodeInvalidRequest})

		return
	}

	err := h.checkResultService.CheckResult(c.Request.Context(), params.MatchID)
	if errors.As(err, &models.ResourceNotFoundError{}) {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "code": models.CodeResourceNotFound})

		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "code": models.CodeInternalServerError})

		return
	}

	c.Status(http.StatusNoContent)
}

func (h *TriggerHandler) NotifySubscriber(c *gin.Context) {
	var params TriggerSubscriptionNotificationRequest
	if err := c.ShouldBindJSON(&params); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "code": models.CodeInvalidRequest})
		return
	}

	err := h.subscriberNotifierService.NotifySubscriber(c.Request.Context(), params.SubscriptionID)
	if errors.As(err, &models.ResourceNotFoundError{}) {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "code": models.CodeResourceNotFound})

		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "code": models.CodeInternalServerError})

		return
	}

	c.Status(http.StatusNoContent)
}
