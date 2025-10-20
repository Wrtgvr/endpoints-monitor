package storage

import (
	"context"

	"github.com/wrtgvr/websites-monitor/internal/domain"
	errs "github.com/wrtgvr/websites-monitor/internal/errors"
)

type Storage interface {
	// Close storage connection
	Close() error
	//* Projects
	CreateProject(ctx context.Context, projectInfo *domain.Project) *errs.AppError
	GetProjectIDByAPIKey(ctx context.Context, apiKey string) (projectId string, appErr *errs.AppError)
	GetProjectInfo(ctx context.Context, projectId string) (project *domain.Project, appErr *errs.AppError)
	ChangeProjectName(ctx context.Context, projectId, newName string) *errs.AppError
	DeleteProject(ctx context.Context, projectId string) *errs.AppError
	//* api key
	UpdateProjectAdminAPIKey(ctx context.Context, oldApiKey, newApiKey *domain.APIKey) *errs.AppError
	AddReadonlyAPIKey(ctx context.Context, key *domain.APIKey) *errs.AppError
	RemoveReadonlyAPIKey(ctx context.Context, projectId, key string) *errs.AppError
	GetReadonlyKeys(ctx context.Context, projectId string) (readOnlyKeys []*domain.APIKey, appErr *errs.AppError)
	GetKeyInfo(ctx context.Context, projectId, key string) (keyInfo *domain.APIKey, appErr *errs.AppError)
	//* Endpoints
	CreateEndpoint(ctx context.Context, endpointInfo *domain.EndpointInfo) *errs.AppError
	GetEndpoints(ctx context.Context, projectId, endpointId string) (endpoints []*domain.Endpoint, appErr *errs.AppError)
	GetEndpointsForMonitoring(ctx context.Context, projectId string) (endpointsInfo []*domain.EndpointInfo, appErr *errs.AppError)
	UpdateEndpointInfo(ctx context.Context, endpointInfo *domain.EndpointInfo) *errs.AppError
	UpdateEndpointStatus(ctx context.Context, projectId string, endpointStatus *domain.EndpointStatus) *errs.AppError
	DeleteEndpoint(ctx context.Context, projectId string, endpointId string) *errs.AppError
}
