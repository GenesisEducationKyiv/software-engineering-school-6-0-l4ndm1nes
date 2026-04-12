package grpc

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/user/github-release-notification-api/internal/adapter/inbound/grpc/pb"
	"github.com/user/github-release-notification-api/internal/domain"
)

type SubscriptionUseCase interface {
	Subscribe(ctx context.Context, email, repo string) error
	Confirm(ctx context.Context, token string) error
	Unsubscribe(ctx context.Context, token string) error
	GetSubscriptions(ctx context.Context, email string) ([]domain.Subscription, error)
}

type Handler struct {
	pb.UnimplementedSubscriptionServiceServer
	uc SubscriptionUseCase
}

func NewHandler(uc SubscriptionUseCase) *Handler {
	return &Handler{uc: uc}
}

func (h *Handler) Subscribe(ctx context.Context, req *pb.SubscribeRequest) (*pb.SubscribeResponse, error) {
	if err := h.uc.Subscribe(ctx, req.Email, req.Repo); err != nil {
		return nil, mapDomainError(err)
	}
	return &pb.SubscribeResponse{
		Message: "Subscription successful. Confirmation email sent.",
	}, nil
}

func (h *Handler) Confirm(ctx context.Context, req *pb.ConfirmRequest) (*pb.ConfirmResponse, error) {
	if err := h.uc.Confirm(ctx, req.Token); err != nil {
		return nil, mapDomainError(err)
	}
	return &pb.ConfirmResponse{
		Message: "Subscription confirmed successfully",
	}, nil
}

func (h *Handler) Unsubscribe(ctx context.Context, req *pb.UnsubscribeRequest) (*pb.UnsubscribeResponse, error) {
	if err := h.uc.Unsubscribe(ctx, req.Token); err != nil {
		return nil, mapDomainError(err)
	}
	return &pb.UnsubscribeResponse{
		Message: "Unsubscribed successfully",
	}, nil
}

func (h *Handler) GetSubscriptions(ctx context.Context, req *pb.GetSubscriptionsRequest) (*pb.GetSubscriptionsResponse, error) {
	subs, err := h.uc.GetSubscriptions(ctx, req.Email)
	if err != nil {
		return nil, mapDomainError(err)
	}

	infos := make([]*pb.SubscriptionInfo, 0, len(subs))
	for i := range subs {
		sub := &subs[i]
		info := &pb.SubscriptionInfo{
			Email:     sub.Email,
			Confirmed: sub.Confirmed,
		}
		if sub.Repository != nil {
			info.Repo = sub.Repository.FullName()
			info.LastSeenTag = sub.Repository.LastSeenTag
		}
		infos = append(infos, info)
	}

	return &pb.GetSubscriptionsResponse{Subscriptions: infos}, nil
}

func mapDomainError(err error) error {
	switch {
	case errors.Is(err, domain.ErrInvalidEmail),
		errors.Is(err, domain.ErrInvalidRepoFormat),
		errors.Is(err, domain.ErrInvalidToken):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrRepoNotFound),
		errors.Is(err, domain.ErrTokenNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrAlreadySubscribed):
		return status.Error(codes.AlreadyExists, err.Error())
	default:
		return status.Error(codes.Internal, "internal server error")
	}
}
