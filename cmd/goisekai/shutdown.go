package main

import (
	"context"
	"time"

	"goisekai/internal/database"
	"goisekai/internal/httpserver"
	"goisekai/internal/logger"
	"goisekai/internal/pluginmanager"
)

// shutdown stops the HTTP server, then closes plugins and the database in
// order.
func shutdown(srv *httpserver.Server, mgr *pluginmanager.Manager, db *database.DB) {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	logger.Info("shutting down HTTP server")
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("http server shutdown", "error", err)
	}

	logger.Info("closing plugins")
	if err := mgr.Close(); err != nil {
		logger.Error("plugin manager close", "error", err)
	}

	logger.Info("closing database")
	if err := db.Close(); err != nil {
		logger.Error("database close", "error", err)
	}

	logger.Info("shutdown complete")
}
