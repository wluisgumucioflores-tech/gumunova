package http

import (
	"log/slog"
	nethttp "net/http"

	"gumunova/backend/internal/application/usecase/labelscan"
)

type labelResponse struct {
	// Kind is "product" or "roll"; a roll also carries its product, which holds the prices.
	Kind    string          `json:"kind"`
	Product productResponse `json:"product"`
	Roll    *rollResponse   `json:"roll,omitempty"`
}

type labelHandlers struct {
	labels *labelscan.Service
	logger *slog.Logger
}

func (h labelHandlers) resolve(w nethttp.ResponseWriter, r *nethttp.Request) {
	result, err := h.labels.Resolve(r.Context(), r.PathValue("code"))
	if err != nil {
		writeUseCaseError(w, h.logger, err)
		return
	}
	response := labelResponse{Kind: string(result.Kind), Product: toProductResponse(result.Product)}
	if result.Roll != nil {
		roll := toRollResponse(*result.Roll)
		response.Roll = &roll
	}
	writeJSON(w, nethttp.StatusOK, response)
}
