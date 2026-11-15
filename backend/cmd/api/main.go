// Command api starts the HTTP API.
package main

import (
	"context"
	"errors"
	"log/slog"
	nethttp "net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	apihttp "gumunova/backend/internal/adapters/inbound/http"
	"gumunova/backend/internal/adapters/outbound/cloudflare_r2"
	"gumunova/backend/internal/adapters/outbound/postgres"
	"gumunova/backend/internal/application/usecase/catalog"
	"gumunova/backend/internal/application/usecase/inventoryquery"
	"gumunova/backend/internal/application/usecase/purchasing"
	"gumunova/backend/internal/application/usecase/selling"
	suppliers "gumunova/backend/internal/application/usecase/supplier"
	"gumunova/backend/internal/config"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("api stopped with an error", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	settings, err := config.Load()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := postgres.Connect(ctx, settings.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	storage := cloudflare_r2.New(cloudflare_r2.Config{
		Endpoint:       settings.ObjectStorageEndpoint,
		PublicEndpoint: settings.ObjectStoragePublicEndpoint,
		Region:         settings.ObjectStorageRegion,
		Bucket:         settings.ObjectStorageBucket,
		AccessKey:      settings.ObjectStorageAccessKey,
		SecretKey:      settings.ObjectStorageSecretKey,
	})
	if settings.AutoCreateBucket {
		if err := storage.EnsureBucket(ctx); err != nil {
			return err
		}
	}

	productRepository := postgres.NewProductRepository(pool)
	supplierRepository := postgres.NewSupplierRepository(pool)
	catalogService := catalog.NewService(productRepository, storage, logger)
	supplierService := suppliers.NewService(supplierRepository)
	saleService := selling.NewService(postgres.NewSaleRepository(pool), time.Now)
	inventoryService := inventoryquery.NewService(postgres.NewInventoryRepository(pool))
	purchaseService := purchasing.NewService(postgres.NewPurchaseRepository(pool), supplierRepository, productRepository, time.Now)
	server := &nethttp.Server{
		Addr:              settings.HTTPAddr,
		Handler:           apihttp.NewHandler(catalogService, supplierService, purchaseService, inventoryService, saleService, logger),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("api listening", "addr", settings.HTTPAddr)
		serverErrors <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErrors:
		if !errors.Is(err, nethttp.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		logger.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	}
	return nil
}
