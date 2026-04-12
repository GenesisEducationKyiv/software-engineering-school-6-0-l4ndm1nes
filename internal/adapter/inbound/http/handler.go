package http

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/user/github-release-notification-api/internal/domain"
)

type SubscriptionUseCase interface {
	Subscribe(ctx context.Context, email, repo string) error
	Confirm(ctx context.Context, token string) error
	Unsubscribe(ctx context.Context, token string) error
	GetSubscriptions(ctx context.Context, email string) ([]domain.Subscription, error)
}

type Handler struct {
	uc SubscriptionUseCase
}

func NewHandler(uc SubscriptionUseCase) *Handler {
	return &Handler{uc: uc}
}

type subscribeRequest struct {
	Email string `json:"email" binding:"required"`
	Repo  string `json:"repo" binding:"required"`
}

type subscriptionResponse struct {
	Email       string `json:"email"`
	Repo        string `json:"repo"`
	Confirmed   bool   `json:"confirmed"`
	LastSeenTag string `json:"last_seen_tag,omitempty"`
}

func (h *Handler) Subscribe(c *gin.Context) {
	var req subscribeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if err := h.uc.Subscribe(c.Request.Context(), req.Email, req.Repo); err != nil {
		writeDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Subscription successful. Confirmation email sent."})
}

func (h *Handler) Confirm(c *gin.Context) {
	token := c.Param("token")

	if err := h.uc.Confirm(c.Request.Context(), token); err != nil {
		writeDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Subscription confirmed successfully"})
}

func (h *Handler) Unsubscribe(c *gin.Context) {
	token := c.Param("token")

	if err := h.uc.Unsubscribe(c.Request.Context(), token); err != nil {
		writeDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Unsubscribed successfully"})
}

func (h *Handler) GetSubscriptions(c *gin.Context) {
	email := c.Query("email")
	if email == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email query parameter is required"})
		return
	}

	subs, err := h.uc.GetSubscriptions(c.Request.Context(), email)
	if err != nil {
		writeDomainError(c, err)
		return
	}

	response := make([]subscriptionResponse, 0, len(subs))
	for i := range subs {
		sub := &subs[i]
		resp := subscriptionResponse{
			Email:     sub.Email,
			Confirmed: sub.Confirmed,
		}
		if sub.Repository != nil {
			resp.Repo = sub.Repository.FullName()
			resp.LastSeenTag = sub.Repository.LastSeenTag
		}
		response = append(response, resp)
	}

	c.JSON(http.StatusOK, response)
}

func writeDomainError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidEmail),
		errors.Is(err, domain.ErrInvalidRepoFormat),
		errors.Is(err, domain.ErrInvalidToken):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, domain.ErrRepoNotFound),
		errors.Is(err, domain.ErrTokenNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, domain.ErrAlreadySubscribed):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	}
}
