package handler

import (
	"net/http"

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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})

		return
	}

	err := h.checkResultService.CheckResult(c.Request.Context(), params.MatchID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})

		return
	}

	c.Status(http.StatusNoContent)
}

func (h *TriggerHandler) NotifySubscriber(c *gin.Context) {
	var params TriggerSubscriptionNotificationRequest
	if err := c.ShouldBindJSON(&params); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.subscriberNotifierService.NotifySubscriber(c.Request.Context(), params.SubscriptionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})

		return
	}

	c.Status(http.StatusNoContent)
}
