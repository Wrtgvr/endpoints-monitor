package storage

import (
	"context"

	"github.com/wrtgvr/websites-monitor/internal/domain"
	errs "github.com/wrtgvr/websites-monitor/internal/errors"
)

type EndpointsStorage interface {
	Close() error
	AddEndpoint(ctx context.Context, endpoint *domain.EndpointInfo) *errs.AppError
	DeleteEndpoint(ctx context.Context, id string) *errs.AppError
	ChangeEndpointInfo(ctx context.Context, info *domain.EndpointInfo) *errs.AppError
	UpdateEndpointStatus(ctx context.Context, endpointStatus *domain.EndpointStatus) *errs.AppError
	GetEndpoints(ctx context.Context, limit, offset int64) ([]*domain.Endpoint, *errs.AppError)
	GetEndpointsForMonitoring(ctx context.Context) ([]*domain.EndpointInfo, *errs.AppError)
}
