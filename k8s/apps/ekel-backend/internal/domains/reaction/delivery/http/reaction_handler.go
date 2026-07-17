package http

import (
	"app/internal/domains/reaction"
	"app/internal/models"

	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
)

type ReactionHandler struct {
	uc reaction.ReactionUsecase
}

func NewReactionHandler(e *echo.Group, uc reaction.ReactionUsecase) {
	handler := &ReactionHandler{uc: uc}

	e.GET("/reactions", handler.List)
	e.POST("/reactions/:slug/add", handler.Add)
	e.POST("/reactions/:slug/remove", handler.Remove)
}

func (h *ReactionHandler) List(c echo.Context) error {
	resp, err := h.uc.List(c.Request().Context())
	if err != nil {
		logrus.WithError(err).Error("failed to list reactions")
		return models.ErrorResponse(c, "failed to list reactions")
	}

	return models.SuccessResponse(c, resp, "reactions retrieved successfully")
}

func (h *ReactionHandler) Add(c echo.Context) error {
	slug := c.Param("slug")

	resp, err := h.uc.Add(c.Request().Context(), slug)
	if err != nil {
		logrus.WithError(err).Error("failed to add reaction")
		return models.ErrorResponse(c, "failed to add reaction")
	}

	return models.SuccessResponse(c, resp, "reaction added successfully")
}

func (h *ReactionHandler) Remove(c echo.Context) error {
	slug := c.Param("slug")

	resp, err := h.uc.Remove(c.Request().Context(), slug)
	if err != nil {
		logrus.WithError(err).Error("failed to remove reaction")
		return models.ErrorResponse(c, "failed to remove reaction")
	}

	return models.SuccessResponse(c, resp, "reaction removed successfully")
}
