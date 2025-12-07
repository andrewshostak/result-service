package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type TriggerHandler struct {
	checkResultService ResultCheckerService
}

func NewTriggerHandler(checkResultService ResultCheckerService) *TriggerHandler {
	return &TriggerHandler{
		checkResultService: checkResultService,
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

func (h *TriggerHandler) NotifySubscribers(c *gin.Context) {
	// TODO
}
