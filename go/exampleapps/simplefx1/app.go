package main

import (
	"context"
	"net/http"

	"go.uber.org/fx"
)

// Logger interface
type Logger interface {
	Log(message string)
}

// SimpleLogger implementation
type SimpleLogger struct {
	Prefix string
}

// Log logs a message
func (l *SimpleLogger) Log(message string) {

}

// Database interface
type Database interface {
	Connect() string
}

// SimpleDatabase implementation
type SimpleDatabase struct {
	Logger Logger
}

// Connect connects to the database
func (db *SimpleDatabase) Connect() string {
	return ""
}

// NewLogger provides an instance of SimpleLogger
func NewLogger() Logger {
	return &SimpleLogger{Prefix: "INFO"}
}

// NewDatabase provides an instance of SimpleDatabase
func NewDatabase(logger Logger) Database {
	return &SimpleDatabase{
		Logger: logger,
	}
}

// NewHTTPServer creates a new HTTP server.
func NewHTTPServer(logger Logger, db Database) *http.Server {
	mux := http.NewServeMux()
	logger.Log(db.Connect()) // Log the database connection
	return &http.Server{
		Addr:    "8080",
		Handler: mux,
	}
}

// StartHTTPServer starts the HTTP server.
func StartHTTPServer(lc fx.Lifecycle, server *http.Server, logger Logger) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			logger.Log("Starting HTTP server")
			go server.ListenAndServe()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			logger.Log("Stopping HTTP server")
			return server.Shutdown(ctx)
		},
	})
}

// HandlerInParams is a struct that contains the input parameters for the handler.
type HandlerInParams struct {
	fx.In
	// Logger is the logger.
	Logger *Logger
}

// HandlerOutParams is a struct that contains the output parameters for the handler.
type HandlerOutParams struct {
	fx.Out
	// DB is the db.
	DB *Database
}

func main() {
	app := fx.New(
		fx.Provide(
			func(l Logger) int {
				return 42
			},
			NewLogger,
			NewDatabase,
			NewHTTPServer,
		),
		fx.Invoke(
			StartHTTPServer,
		),
	)

	app.Run()
}
