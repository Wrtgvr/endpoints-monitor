package storage

import (
	"context"

	"github.com/wrtgvr/websites-monitor/internal/domain"
	errs "github.com/wrtgvr/websites-monitor/internal/errors"
)

type EndpointsStorage interface {
	// Close storage connection
	Close() error
	//* Projects
	CreateProject(ctx context.Context, apiKey, name string) (appErr *errs.AppError)
	GetProjectIDByAPIKey(ctx context.Context, apiKey string) (projectId string, appErr *errs.AppError)
	GetProjectInfo(ctx context.Context, projectId string) (project *domain.Project, appErr *errs.AppError)
	ChangeProjectName(ctx context.Context, projectId, name string) *errs.AppError
	//* api key
	UpdateProjectAdminAPIKey(ctx context.Context, oldApiKey, newApiKey, projectId string) (appErr *errs.AppError)
	AddReadonlyAPIKey(ctx context.Context, projectId, key string) *errs.AppError
	RemoveReadonlyAPIKey(ctx context.Context, projectId, key string) *errs.AppError
	GetReadonlyKeys(ctx context.Context, projectId string) (readOnlyKeys []string, appErr *errs.AppError)
	//* Endpoints
	GetEndpoints(ctx context.Context, projectId, endpointId string) (endpoints []*domain.Endpoint, appErr *errs.AppError)
	GetEndpointsForMonitoring(ctx context.Context, projectId string) (endpointsInfo []*domain.EndpointInfo, appErr *errs.AppError)
	CreateEndpoint(ctx context.Context, projectId string, endpointInfo *domain.EndpointInfo) *errs.AppError
	UpdateEndpointInfo(ctx context.Context, projectId string, endpointInfo *domain.EndpointInfo) *errs.AppError
	UpdateEndpointStatus(ctx context.Context, projectId string, endpointStatus *domain.EndpointStatus) *errs.AppError
	DeleteEndpoint(ctx context.Context, projectId string, endpointId string) *errs.AppError
}
