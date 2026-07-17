package http

import (
	"app/internal/domains/ihsg"
	"app/internal/models"

	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
)

type IhsgHandler struct {
	uc ihsg.IhsgUsecase
}

func NewIhsgHandler(e *echo.Group, uc ihsg.IhsgUsecase) {
	handler := &IhsgHandler{uc: uc}
	e.GET("/ihsg/markets", handler.GetMarkets)
}

// GetMarkets IHSG
// @Summary Get live market indices
// @Description Fetch live market data for IHSG, Nikkei 225, S&P 500, SSE Composite, and Tadawul from Yahoo Finance/Stooq. Cached for 5 minutes.
// @Tags IHSG
// @Accept json
// @Produce json
// @Success 200 {object} models.Response "OK"
// @Failure 400 {object} models.Response "Bad Request"
// @Router /api/v1/ihsg/markets [get]
func (h *IhsgHandler) GetMarkets(c echo.Context) error {
	markets, err := h.uc.GetMarkets(c.Request().Context())
	if err != nil {
		logrus.WithError(err).Error("failed to get market data")
		return models.ErrorResponse(c, "failed to get market data")
	}

	return models.SuccessResponse(c, markets, "market data retrieved successfully")
}
