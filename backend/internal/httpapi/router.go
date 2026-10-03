package httpapi

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	nethttpmiddleware "github.com/oapi-codegen/nethttp-middleware"

	"bewerbungsmanager/internal/service"
)

const docsPage = `<!doctype html>
<html lang="de">
<head>
  <meta charset="utf-8">
  <title>Bewerbungs-Tracker API</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>SwaggerUIBundle({ url: "/api/openapi.json", dom_id: "#swagger-ui" });</script>
</body>
</html>`

// NewRouter baut den kompletten HTTP-Handler: API, Validierung, Doku, Logging.
func NewRouter(svc *service.Service, logger *slog.Logger) (http.Handler, error) {
	spec, err := GetSwagger()
	if err != nil {
		return nil, fmt.Errorf("openapi-spec laden: %w", err)
	}
	spec.Servers = nil // Pfade in der Spec sind absolut
	specJSON, err := spec.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("openapi-spec serialisieren: %w", err)
	}

	validator := nethttpmiddleware.OapiRequestValidatorWithOptions(spec, &nethttpmiddleware.Options{
		SilenceServersWarning: true,
		ErrorHandler: func(w http.ResponseWriter, message string, status int) {
			// Text von http.MaxBytesReader (limitBody), den der Validator beim Lesen des Bodys weiterreicht.
			if strings.Contains(message, "request body too large") {
				writeProblem(w, problem{Type: problemBase + "payload-too-large", Title: "Anfrage zu groß",
					Status: http.StatusRequestEntityTooLarge, Detail: "Die Anfrage ist zu groß (max. 1 MB)."})
				return
			}
			p := badRequest(message)
			p.Status = status
			writeProblem(w, p)
		},
	})

	strict := NewStrictHandlerWithOptions(&Server{svc: svc}, nil, StrictHTTPServerOptions{
		RequestErrorHandlerFunc:  requestErrorHandler,
		ResponseErrorHandlerFunc: responseErrorHandler(logger),
	})

	mux := http.NewServeMux()
	HandlerWithOptions(strict, StdHTTPServerOptions{
		BaseRouter:       mux,
		Middlewares:      []MiddlewareFunc{validator},
		ErrorHandlerFunc: requestErrorHandler,
	})
	mux.HandleFunc("GET /api/openapi.json", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(specJSON)
	})
	mux.HandleFunc("GET /api/docs", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, docsPage)
	})
	mux.HandleFunc("/api/", func(w http.ResponseWriter, _ *http.Request) {
		writeProblem(w, problem{Type: problemBase + "not-found", Title: "Nicht gefunden",
			Status: http.StatusNotFound, Detail: "Unbekannter Endpunkt"})
	})
	return logRequests(logger, recoverPanics(logger, limitBody(mux))), nil
}

// maxBodyBytes begrenzt die Größe von Request-Bodys.
const maxBodyBytes = 1 << 20

func limitBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
		}
		next.ServeHTTP(w, r)
	})
}

// recoverPanics fängt Panics ab, loggt sie und antwortet mit 500 als Problem-JSON.
func recoverPanics(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			v := recover()
			if v == nil {
				return
			}
			if v == http.ErrAbortHandler { //nolint:errorlint // Panic-Wert, kein zurückgegebener Fehler; net/http vergleicht ebenso
				panic(v)
			}
			logger.Error("panic", "method", r.Method, "path", r.URL.Path, "panic", v)
			writeProblem(w, problem{Type: problemBase + "internal", Title: "Interner Fehler", Status: http.StatusInternalServerError})
		}()
		next.ServeHTTP(w, r)
	})
}
