// Package httpapi bildet den generierten OpenAPI-Server auf den Service ab.
package httpapi

import "bewerbungsmanager/internal/service"

// Server implementiert StrictServerInterface.
type Server struct {
	svc *service.Service
}

var _ StrictServerInterface = (*Server)(nil)
