package http

import (
	"app/internal/domains/wakatime"
	"app/internal/models"

	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
)

type WakatimeHandler struct {
	uc wakatime.WakatimeUsecase
}

func NewWakatimeHandler(e *echo.Group, uc wakatime.WakatimeUsecase) {
	handler := &WakatimeHandler{uc: uc}
	e.GET("/wakatime/stats", handler.GetStats)
}

// GetStats Wakatime
// @Summary Get Wakatime coding stats
// @Description Fetch coding stats from Wakatime API. Use ?range=all_time (default), last_7_days, or last_day.
// @Tags Wakatime
// @Accept json
// @Produce json
// @Param range query string false "Range: all_time, last_7_days, last_day"
// @Success 200 {object} models.Response{data=wakatime.StatsResponse} "OK"
// @Failure 400 {object} models.Response "Bad Request"
// @Router /api/v1/wakatime/stats [get]
func (h *WakatimeHandler) GetStats(c echo.Context) error {
	rng := c.QueryParam("range")
	if rng == "" {
		rng = "all_time"
	}

	stats, err := h.uc.GetStats(c.Request().Context(), rng)
	if err != nil {
		logrus.WithError(err).Error("failed to get wakatime stats")
		return models.ErrorResponse(c, "failed to get wakatime stats")
	}

	return models.SuccessResponse(c, stats, "wakatime stats retrieved successfully")
}
