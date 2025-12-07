package handler

import (
	"time"

	"github.com/andrewshostak/result-service/service"
)

type CreateMatchRequest struct {
	StartsAt  time.Time `binding:"required" json:"starts_at" time_format:"2006-01-02T15:04:05Z07:00"`
	AliasHome string    `binding:"required" json:"alias_home"`
	AliasAway string    `binding:"required" json:"alias_away"`
}

type CreateSubscriptionRequest struct {
	MatchID   uint   `binding:"required" json:"match_id"`
	URL       string `binding:"required" json:"url"`
	SecretKey string `binding:"required" json:"secret_key"`
}

type DeleteSubscriptionRequest struct {
	StartsAt  time.Time `form:"starts_at" binding:"required" time_format:"2006-01-02T15:04:05Z07:00"`
	AliasHome string    `form:"alias_home" binding:"required"`
	AliasAway string    `form:"alias_away" binding:"required"`
	BaseURL   string    `form:"base_url" binding:"required"`
	SecretKey string    `form:"secret_key" binding:"required"`
}

type SearchAliasRequest struct {
	Search string `form:"search" binding:"required"`
}

type TriggerResultCheckRequest struct {
	MatchID uint `form:"match_id" binding:"required"`
}

func (cmr *CreateMatchRequest) ToDomain() service.CreateMatchRequest {
	return service.CreateMatchRequest{
		StartsAt:  cmr.StartsAt,
		AliasHome: cmr.AliasHome,
		AliasAway: cmr.AliasAway,
	}
}

func (csr *CreateSubscriptionRequest) ToDomain() service.CreateSubscriptionRequest {
	return service.CreateSubscriptionRequest{
		MatchID:   csr.MatchID,
		URL:       csr.URL,
		SecretKey: csr.SecretKey,
	}
}

func (dsr *DeleteSubscriptionRequest) ToDomain() service.DeleteSubscriptionRequest {
	return service.DeleteSubscriptionRequest{
		StartsAt:  dsr.StartsAt,
		AliasHome: dsr.AliasHome,
		AliasAway: dsr.AliasAway,
		BaseURL:   dsr.BaseURL,
		SecretKey: dsr.SecretKey,
	}
}
