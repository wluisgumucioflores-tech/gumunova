package http

import (
	"errors"
	"time"

	"gumunova/backend/internal/application/usecase/catalog"
	"gumunova/backend/internal/domain/product"
	"gumunova/backend/internal/domain/shared/decimal"
)

// Money and measures travel as decimal strings ("15.50") so no client or server
// ever has to round-trip them through floating point.

type productRequest struct {
	SKU         string             `json:"sku"`
	Name        string             `json:"name"`
	ProductType string             `json:"product_type"`
	PVC         *pvcRequest        `json:"pvc"`
	Tablecloth  *tableclothRequest `json:"tablecloth"`
}

type pvcRequest struct {
	Material               string  `json:"material"`
	Design                 string  `json:"design"`
	Color                  string  `json:"color"`
	Texture                string  `json:"texture"`
	WidthM                 string  `json:"width_m"`
	NominalLengthM         string  `json:"nominal_length_m"`
	ThicknessMM            *string `json:"thickness_mm"`
	ReferencePricePerMeter string  `json:"reference_price_per_meter"`
	ReferencePricePerRoll  string  `json:"reference_price_per_roll"`
	MinimumStockM          string  `json:"minimum_stock_m"`
}

type tableclothRequest struct {
	Design        string  `json:"design"`
	Color         string  `json:"color"`
	Dimensions    string  `json:"dimensions"`
	ReferenceCost *string `json:"reference_cost"`
	UnitPrice     string  `json:"unit_price"`
	MinimumUnits  int     `json:"minimum_units"`
}

type productResponse struct {
	ID          string  `json:"id"`
	SKU         string  `json:"sku"`
	Name        string  `json:"name"`
	ProductType string  `json:"product_type"`
	IsActive    bool    `json:"is_active"`
	ImageURL    *string `json:"image_url"`
	// LabelCode is the unique code printed on the product's labels (QR or barcode).
	LabelCode  string              `json:"label_code"`
	PVC        *pvcResponse        `json:"pvc,omitempty"`
	Tablecloth *tableclothResponse `json:"tablecloth,omitempty"`
	CreatedAt  time.Time           `json:"created_at"`
	UpdatedAt  time.Time           `json:"updated_at"`
}

type pvcResponse struct {
	Material               string  `json:"material"`
	Design                 string  `json:"design"`
	Color                  string  `json:"color"`
	Texture                string  `json:"texture"`
	WidthM                 string  `json:"width_m"`
	NominalLengthM         string  `json:"nominal_length_m"`
	ThicknessMM            *string `json:"thickness_mm"`
	ReferencePricePerMeter string  `json:"reference_price_per_meter"`
	ReferencePricePerRoll  string  `json:"reference_price_per_roll"`
	MinimumStockM          string  `json:"minimum_stock_m"`
}

type tableclothResponse struct {
	Design        string  `json:"design"`
	Color         string  `json:"color"`
	Dimensions    string  `json:"dimensions"`
	ReferenceCost *string `json:"reference_cost"`
	UnitPrice     string  `json:"unit_price"`
	MinimumUnits  int     `json:"minimum_units"`
}

type productListResponse struct {
	Items  []productResponse `json:"items"`
	Total  int               `json:"total"`
	Limit  int               `json:"limit"`
	Offset int               `json:"offset"`
}

// Stable codes for decimal fields that cannot be read.
const (
	codeInvalidDecimal    = "invalid_decimal"
	codeTooManyDecimals   = "too_many_decimals"
	codeDecimalOutOfRange = "out_of_range"
)

// toDraft converts a transport request into a domain draft. Fields that cannot be parsed
// are reported with the same field-error shape the domain uses.
func (r productRequest) toDraft() (product.Draft, []product.FieldError) {
	draft := product.Draft{SKU: r.SKU, Name: r.Name, Type: product.Type(r.ProductType)}
	var problems []product.FieldError

	parse := func(field, value string) decimal.Decimal {
		parsed, problem := parseDecimal(field, value, true)
		if problem != nil {
			problems = append(problems, *problem)
		}
		return parsed
	}
	// parseDefault reads an optional decimal where an absent value means zero.
	parseDefault := func(field, value string) decimal.Decimal {
		parsed, problem := parseDecimal(field, value, false)
		if problem != nil {
			problems = append(problems, *problem)
		}
		return parsed
	}
	parseOptional := func(field string, value *string) *decimal.Decimal {
		if value == nil || *value == "" {
			return nil
		}
		parsed, problem := parseDecimal(field, *value, false)
		if problem != nil {
			problems = append(problems, *problem)
			return nil
		}
		return &parsed
	}

	if r.PVC != nil {
		draft.PVC = &product.PVCDetails{
			Material:               r.PVC.Material,
			Design:                 r.PVC.Design,
			Color:                  r.PVC.Color,
			Texture:                r.PVC.Texture,
			WidthM:                 parse("pvc.width_m", r.PVC.WidthM),
			NominalLengthM:         parse("pvc.nominal_length_m", r.PVC.NominalLengthM),
			ThicknessMM:            parseOptional("pvc.thickness_mm", r.PVC.ThicknessMM),
			ReferencePricePerMeter: parse("pvc.reference_price_per_meter", r.PVC.ReferencePricePerMeter),
			ReferencePricePerRoll:  parse("pvc.reference_price_per_roll", r.PVC.ReferencePricePerRoll),
			MinimumStockM:          parseDefault("pvc.minimum_stock_m", r.PVC.MinimumStockM),
		}
	}
	if r.Tablecloth != nil {
		draft.Tablecloth = &product.TableclothDetails{
			Design:        r.Tablecloth.Design,
			Color:         r.Tablecloth.Color,
			Dimensions:    r.Tablecloth.Dimensions,
			ReferenceCost: parseOptional("tablecloth.reference_cost", r.Tablecloth.ReferenceCost),
			UnitPrice:     parse("tablecloth.unit_price", r.Tablecloth.UnitPrice),
			MinimumUnits:  r.Tablecloth.MinimumUnits,
		}
	}
	return draft, problems
}

func parseDecimal(field, value string, required bool) (decimal.Decimal, *product.FieldError) {
	if value == "" {
		if required {
			return decimal.Decimal{}, &product.FieldError{Field: field, Code: product.CodeRequired}
		}
		return decimal.Decimal{}, nil
	}
	parsed, err := decimal.Parse(value)
	switch {
	case err == nil:
		return parsed, nil
	case errors.Is(err, decimal.ErrTooManyDecimals):
		return decimal.Decimal{}, &product.FieldError{Field: field, Code: codeTooManyDecimals}
	case errors.Is(err, decimal.ErrOutOfRange):
		return decimal.Decimal{}, &product.FieldError{Field: field, Code: codeDecimalOutOfRange}
	default:
		return decimal.Decimal{}, &product.FieldError{Field: field, Code: codeInvalidDecimal}
	}
}

func toProductResponse(view catalog.ProductView) productResponse {
	p := view.Product
	response := productResponse{
		ID:          p.ID,
		SKU:         p.SKU,
		Name:        p.Name,
		ProductType: string(p.Type),
		IsActive:    p.IsActive,
		LabelCode:   p.LabelCode,
		CreatedAt:   p.CreatedAt,
		UpdatedAt:   p.UpdatedAt,
	}
	if view.ImageURL != "" {
		response.ImageURL = &view.ImageURL
	}
	if p.PVC != nil {
		response.PVC = &pvcResponse{
			Material:               p.PVC.Material,
			Design:                 p.PVC.Design,
			Color:                  p.PVC.Color,
			Texture:                p.PVC.Texture,
			WidthM:                 p.PVC.WidthM.String(),
			NominalLengthM:         p.PVC.NominalLengthM.String(),
			ThicknessMM:            optionalString(p.PVC.ThicknessMM),
			ReferencePricePerMeter: p.PVC.ReferencePricePerMeter.String(),
			ReferencePricePerRoll:  p.PVC.ReferencePricePerRoll.String(),
			MinimumStockM:          p.PVC.MinimumStockM.String(),
		}
	}
	if p.Tablecloth != nil {
		response.Tablecloth = &tableclothResponse{
			Design:        p.Tablecloth.Design,
			Color:         p.Tablecloth.Color,
			Dimensions:    p.Tablecloth.Dimensions,
			ReferenceCost: optionalString(p.Tablecloth.ReferenceCost),
			UnitPrice:     p.Tablecloth.UnitPrice.String(),
			MinimumUnits:  p.Tablecloth.MinimumUnits,
		}
	}
	return response
}

func optionalString(value *decimal.Decimal) *string {
	if value == nil {
		return nil
	}
	text := value.String()
	return &text
}
