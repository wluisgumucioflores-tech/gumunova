// Package http is the inbound HTTP adapter: it translates requests into use case calls
// and results into JSON. It contains no business rules.
package http

import (
	"log/slog"
	nethttp "net/http"
	"time"

	"gumunova/backend/internal/application/usecase/catalog"
	"gumunova/backend/internal/application/usecase/inventoryquery"
	"gumunova/backend/internal/application/usecase/labelscan"
	"gumunova/backend/internal/application/usecase/purchasing"
	"gumunova/backend/internal/application/usecase/selling"
	suppliers "gumunova/backend/internal/application/usecase/supplier"
)

// NewHandler builds the API router. Routes live under /api/v1.
func NewHandler(
	catalogService *catalog.Service,
	supplierService *suppliers.Service,
	purchaseService *purchasing.Service,
	inventoryService *inventoryquery.Service,
	saleService *selling.Service,
	logger *slog.Logger,
) nethttp.Handler {
	products := productHandlers{catalog: catalogService, logger: logger}
	supplierRoutes := supplierHandlers{suppliers: supplierService, logger: logger}
	purchaseRoutes := purchaseHandlers{purchases: purchaseService, logger: logger}
	inventoryRoutes := inventoryHandlers{inventory: inventoryService, logger: logger}
	saleRoutes := saleHandlers{sales: saleService, logger: logger}
	labelRoutes := labelHandlers{labels: labelscan.NewService(catalogService, inventoryService), logger: logger}

	mux := nethttp.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w nethttp.ResponseWriter, _ *nethttp.Request) {
		writeJSON(w, nethttp.StatusOK, map[string]string{"status": "ok"})
	})

	mux.HandleFunc("POST /api/v1/products", products.create)
	mux.HandleFunc("GET /api/v1/products", products.list)
	mux.HandleFunc("GET /api/v1/products/{id}", products.get)
	mux.HandleFunc("PUT /api/v1/products/{id}", products.update)
	mux.HandleFunc("POST /api/v1/products/{id}/activate", products.activate)
	mux.HandleFunc("POST /api/v1/products/{id}/deactivate", products.deactivate)
	mux.HandleFunc("POST /api/v1/products/{id}/label-code/reset", products.resetLabelCode)
	mux.HandleFunc("PUT /api/v1/products/{id}/image", products.setImage)
	mux.HandleFunc("DELETE /api/v1/products/{id}/image", products.removeImage)

	mux.HandleFunc("POST /api/v1/suppliers", supplierRoutes.create)
	mux.HandleFunc("GET /api/v1/suppliers", supplierRoutes.list)
	mux.HandleFunc("GET /api/v1/suppliers/{id}", supplierRoutes.get)
	mux.HandleFunc("PUT /api/v1/suppliers/{id}", supplierRoutes.update)
	mux.HandleFunc("POST /api/v1/suppliers/{id}/activate", supplierRoutes.activate)
	mux.HandleFunc("POST /api/v1/suppliers/{id}/deactivate", supplierRoutes.deactivate)

	mux.HandleFunc("POST /api/v1/purchases", purchaseRoutes.register)
	mux.HandleFunc("GET /api/v1/purchases", purchaseRoutes.list)
	mux.HandleFunc("GET /api/v1/purchases/{id}", purchaseRoutes.get)
	mux.HandleFunc("PUT /api/v1/purchases/{id}", purchaseRoutes.update)

	mux.HandleFunc("GET /api/v1/labels/{code}", labelRoutes.resolve)

	mux.HandleFunc("POST /api/v1/sales", saleRoutes.register)
	mux.HandleFunc("GET /api/v1/sales", saleRoutes.list)
	mux.HandleFunc("GET /api/v1/sales/{id}", saleRoutes.get)

	mux.HandleFunc("GET /api/v1/inventory/stock", inventoryRoutes.stock)
	mux.HandleFunc("GET /api/v1/inventory/rolls", inventoryRoutes.rolls)
	mux.HandleFunc("GET /api/v1/inventory/rolls/{id}", inventoryRoutes.roll)
	mux.HandleFunc("POST /api/v1/inventory/rolls/{id}/label-code/reset", inventoryRoutes.resetRollLabelCode)
	mux.HandleFunc("GET /api/v1/inventory/movements", inventoryRoutes.movements)

	return recoverPanics(logger, logRequests(logger, secureHeaders(mux)))
}

func secureHeaders(next nethttp.Handler) nethttp.Handler {
	return nethttp.HandlerFunc(func(w nethttp.ResponseWriter, r *nethttp.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

type statusRecorder struct {
	nethttp.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(status int) {
	s.status = status
	s.ResponseWriter.WriteHeader(status)
}

// logRequests records method, path, status and duration. Bodies, query strings and
// headers are never logged, so no sensitive data ends up in the logs.
func logRequests(logger *slog.Logger, next nethttp.Handler) nethttp.Handler {
	return nethttp.HandlerFunc(func(w nethttp.ResponseWriter, r *nethttp.Request) {
		started := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, status: nethttp.StatusOK}
		next.ServeHTTP(recorder, r)
		logger.Info("request", "method", r.Method, "path", r.URL.Path,
			"status", recorder.status, "duration_ms", time.Since(started).Milliseconds())
	})
}

func recoverPanics(logger *slog.Logger, next nethttp.Handler) nethttp.Handler {
	return nethttp.HandlerFunc(func(w nethttp.ResponseWriter, r *nethttp.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.Error("panic while handling request", "method", r.Method, "path", r.URL.Path, "panic", recovered)
				writeError(w, nethttp.StatusInternalServerError, codeInternal)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
