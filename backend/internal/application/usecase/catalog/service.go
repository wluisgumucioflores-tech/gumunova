// Package catalog contains the use cases for managing products and their images.
package catalog

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"gumunova/backend/internal/application/ports"
	"gumunova/backend/internal/domain/product"
)

const (
	// MaxImageBytes is the largest accepted product image.
	MaxImageBytes = 5 << 20

	imageURLTTL      = 15 * time.Minute
	defaultPageSize  = 50
	maxPageSize      = 200
	sniffLengthBytes = 512
)

var (
	// ErrImageTooLarge is returned when an image exceeds MaxImageBytes.
	ErrImageTooLarge = errors.New("image too large")
	// ErrImageTypeNotSupported is returned when the image is not JPEG, PNG or WebP.
	ErrImageTypeNotSupported = errors.New("image type not supported")
	// ErrEmptyImage is returned when the uploaded image has no content.
	ErrEmptyImage = errors.New("image is empty")
)

var imageExtensions = map[string]string{
	"image/jpeg": "jpg",
	"image/png":  "png",
	"image/webp": "webp",
}

// ProductView is a product together with a short-lived URL for its image, if it has one.
type ProductView struct {
	Product  product.Product
	ImageURL string
}

type ProductListView struct {
	Items  []ProductView
	Total  int
	Limit  int
	Offset int
}

type Service struct {
	products ports.ProductRepository
	images   ports.ImageStorage
	logger   *slog.Logger
}

func NewService(products ports.ProductRepository, images ports.ImageStorage, logger *slog.Logger) *Service {
	return &Service{products: products, images: images, logger: logger}
}

func (s *Service) CreateProduct(ctx context.Context, draft product.Draft) (ProductView, error) {
	newProduct, err := product.New(draft)
	if err != nil {
		return ProductView{}, err
	}
	created, err := s.products.Create(ctx, newProduct)
	if err != nil {
		return ProductView{}, err
	}
	return s.view(ctx, created)
}

func (s *Service) UpdateProduct(ctx context.Context, id string, draft product.Draft) (ProductView, error) {
	existing, err := s.products.GetByID(ctx, id)
	if err != nil {
		return ProductView{}, err
	}
	if err := existing.Revise(draft); err != nil {
		return ProductView{}, err
	}
	updated, err := s.products.Update(ctx, existing)
	if err != nil {
		return ProductView{}, err
	}
	return s.view(ctx, updated)
}

func (s *Service) GetProduct(ctx context.Context, id string) (ProductView, error) {
	found, err := s.products.GetByID(ctx, id)
	if err != nil {
		return ProductView{}, err
	}
	return s.view(ctx, found)
}

// GetProductByLabelCode finds the product whose label carries the code.
func (s *Service) GetProductByLabelCode(ctx context.Context, code string) (ProductView, error) {
	found, err := s.products.GetByLabelCode(ctx, code)
	if err != nil {
		return ProductView{}, err
	}
	return s.view(ctx, found)
}

func (s *Service) ListProducts(ctx context.Context, filter ports.ProductFilter) (ProductListView, error) {
	if filter.Limit <= 0 {
		filter.Limit = defaultPageSize
	}
	if filter.Limit > maxPageSize {
		filter.Limit = maxPageSize
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}

	page, err := s.products.List(ctx, filter)
	if err != nil {
		return ProductListView{}, err
	}
	items := make([]ProductView, 0, len(page.Items))
	for _, item := range page.Items {
		view, err := s.view(ctx, item)
		if err != nil {
			return ProductListView{}, err
		}
		items = append(items, view)
	}
	return ProductListView{Items: items, Total: page.Total, Limit: filter.Limit, Offset: filter.Offset}, nil
}

// DeactivateProduct hides a product from new operations while keeping its history.
func (s *Service) DeactivateProduct(ctx context.Context, id string) (ProductView, error) {
	return s.changeActive(ctx, id, (*product.Product).Deactivate)
}

func (s *Service) ActivateProduct(ctx context.Context, id string) (ProductView, error) {
	return s.changeActive(ctx, id, (*product.Product).Activate)
}

// ResetProductLabelCode gives the product a new label code. Labels printed with the old code
// stop identifying it, so they must be printed again.
func (s *Service) ResetProductLabelCode(ctx context.Context, id string) (ProductView, error) {
	updated, err := s.products.ResetLabelCode(ctx, id)
	if err != nil {
		return ProductView{}, err
	}
	return s.view(ctx, updated)
}

func (s *Service) changeActive(ctx context.Context, id string, change func(*product.Product)) (ProductView, error) {
	existing, err := s.products.GetByID(ctx, id)
	if err != nil {
		return ProductView{}, err
	}
	change(&existing)
	updated, err := s.products.Update(ctx, existing)
	if err != nil {
		return ProductView{}, err
	}
	return s.view(ctx, updated)
}

// SetProductImage validates and stores a new reference image, replacing the previous one.
// The content type is detected from the bytes, never trusted from the client.
func (s *Service) SetProductImage(ctx context.Context, id string, content io.Reader) (ProductView, error) {
	existing, err := s.products.GetByID(ctx, id)
	if err != nil {
		return ProductView{}, err
	}

	data, err := io.ReadAll(io.LimitReader(content, MaxImageBytes+1))
	if err != nil {
		return ProductView{}, fmt.Errorf("read image: %w", err)
	}
	if len(data) == 0 {
		return ProductView{}, ErrEmptyImage
	}
	if len(data) > MaxImageBytes {
		return ProductView{}, ErrImageTooLarge
	}

	sniffed := data
	if len(sniffed) > sniffLengthBytes {
		sniffed = sniffed[:sniffLengthBytes]
	}
	contentType := http.DetectContentType(sniffed)
	extension, supported := imageExtensions[contentType]
	if !supported {
		return ProductView{}, ErrImageTypeNotSupported
	}

	key, err := newImageKey(existing.ID, extension)
	if err != nil {
		return ProductView{}, err
	}
	if err := s.images.Put(ctx, key, contentType, bytes.NewReader(data), int64(len(data))); err != nil {
		return ProductView{}, fmt.Errorf("store image: %w", err)
	}
	if err := s.products.SetImageKey(ctx, existing.ID, key); err != nil {
		s.deleteQuietly(ctx, key)
		return ProductView{}, err
	}

	previousKey := existing.ImageKey
	existing.ImageKey = key
	if previousKey != "" {
		s.deleteQuietly(ctx, previousKey)
	}
	return s.view(ctx, existing)
}

func (s *Service) RemoveProductImage(ctx context.Context, id string) (ProductView, error) {
	existing, err := s.products.GetByID(ctx, id)
	if err != nil {
		return ProductView{}, err
	}
	if existing.ImageKey == "" {
		return s.view(ctx, existing)
	}
	if err := s.products.SetImageKey(ctx, existing.ID, ""); err != nil {
		return ProductView{}, err
	}
	s.deleteQuietly(ctx, existing.ImageKey)
	existing.ImageKey = ""
	return s.view(ctx, existing)
}

func (s *Service) view(ctx context.Context, p product.Product) (ProductView, error) {
	view := ProductView{Product: p}
	if p.ImageKey == "" {
		return view, nil
	}
	url, err := s.images.SignedURL(ctx, p.ImageKey, imageURLTTL)
	if err != nil {
		return ProductView{}, fmt.Errorf("sign image url: %w", err)
	}
	view.ImageURL = url
	return view, nil
}

// deleteQuietly removes an object that is no longer referenced. A failure only leaves an
// orphan object behind, so it is logged instead of failing the user's operation.
func (s *Service) deleteQuietly(ctx context.Context, key string) {
	if err := s.images.Delete(ctx, key); err != nil {
		s.logger.Warn("could not delete unreferenced image object", "key", key, "error", err)
	}
}

func newImageKey(productID, extension string) (string, error) {
	random := make([]byte, 12)
	if _, err := rand.Read(random); err != nil {
		return "", fmt.Errorf("generate image key: %w", err)
	}
	return fmt.Sprintf("products/%s/%s.%s", productID, hex.EncodeToString(random), extension), nil
}
