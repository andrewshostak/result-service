package handler

import (
	"errors"
	"net/http"

	"github.com/andrewshostak/result-service/errs"
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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "code": errs.CodeInvalidRequest})

		return
	}

	err := h.checkResultService.CheckResult(c.Request.Context(), params.MatchID)
	if errors.As(err, &errs.ResourceNotFoundError{}) {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "code": errs.CodeResourceNotFound})

		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "code": errs.CodeInternalServerError})

		return
	}

	c.Status(http.StatusNoContent)
}

func (h *TriggerHandler) NotifySubscriber(c *gin.Context) {
	var params TriggerSubscriptionNotificationRequest
	if err := c.ShouldBindJSON(&params); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "code": errs.CodeInvalidRequest})
		return
	}

	err := h.subscriberNotifierService.NotifySubscriber(c.Request.Context(), params.SubscriptionID)
	if errors.As(err, &errs.ResourceNotFoundError{}) {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "code": errs.CodeResourceNotFound})

		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "code": errs.CodeInternalServerError})

		return
	}

	c.Status(http.StatusNoContent)
}
