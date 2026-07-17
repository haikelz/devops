package http

import (
	"app/internal/domains/guestbook"
	"app/internal/models"

	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
)

type GuestbookHandler struct {
	uc guestbook.GuestbookUsecase
}

func NewGuestbookHandler(e *echo.Group, uc guestbook.GuestbookUsecase) {
	handler := &GuestbookHandler{uc: uc}

	e.GET("/guestbook", handler.List)
	e.GET("/guestbook/:id", handler.GetByID)
	e.POST("/guestbook", handler.Create)
	e.PUT("/guestbook/:id", handler.Update)
	e.DELETE("/guestbook/:id", handler.Delete)
}

// Create Guestbook
// @Summary Create a guestbook entry
// @Description Create a new guestbook entry with name, email, and message.
// @Tags Guestbook
// @Accept json
// @Produce json
// @Param request body guestbook.CreateGuestbookRequest true "Guestbook Entry"
// @Success 200 {object} models.Response{data=guestbook.GuestbookResponse} "Created"
// @Failure 400 {object} models.Response "Bad Request"
// @Router /api/v1/guestbook [post]
func (h *GuestbookHandler) Create(c echo.Context) error {
	var req guestbook.CreateGuestbookRequest
	if err := c.Bind(&req); err != nil {
		logrus.WithError(err).Error("failed to bind create guestbook request")
		return models.ErrorResponse(c, "invalid request body")
	}

	if err := c.Validate(&req); err != nil {
		logrus.WithError(err).Error("failed to validate create guestbook request")
		return models.ErrorResponse(c, "invalid request data")
	}

	resp, err := h.uc.Create(c.Request().Context(), req)
	if err != nil {
		logrus.WithError(err).Error("failed to create guestbook entry")
		return models.ErrorResponse(c, "failed to create guestbook entry")
	}

	return models.SuccessResponse(c, resp, "guestbook entry created successfully")
}

// GetByID Guestbook
// @Summary Get a guestbook entry by ID
// @Description Retrieve a single guestbook entry by its ID.
// @Tags Guestbook
// @Accept json
// @Produce json
// @Param id path string true "Entry ID"
// @Success 200 {object} models.Response{data=guestbook.GuestbookResponse} "OK"
// @Failure 400 {object} models.Response "Bad Request"
// @Router /api/v1/guestbook/{id} [get]
func (h *GuestbookHandler) GetByID(c echo.Context) error {
	id := c.Param("id")

	resp, err := h.uc.GetByID(c.Request().Context(), id)
	if err != nil {
		logrus.WithError(err).Error("failed to get guestbook entry")
		return models.ErrorResponse(c, "failed to get guestbook entry")
	}

	return models.SuccessResponse(c, resp, "guestbook entry retrieved successfully")
}

// List Guestbook
// @Summary List guestbook entries
// @Description Returns all guestbook entries, newest first.
// @Tags Guestbook
// @Accept json
// @Produce json
// @Success 200 {object} models.Response "OK"
// @Failure 400 {object} models.Response "Bad Request"
// @Router /api/v1/guestbook [get]
func (h *GuestbookHandler) List(c echo.Context) error {
	resp, err := h.uc.List(c.Request().Context())
	if err != nil {
		logrus.WithError(err).Error("failed to list guestbook entries")
		return models.ErrorResponse(c, "failed to list guestbook entries")
	}

	return models.SuccessResponse(c, resp, "guestbook entries retrieved successfully")
}

// Update Guestbook
// @Summary Update a guestbook entry
// @Description Update an existing guestbook entry by ID.
// @Tags Guestbook
// @Accept json
// @Produce json
// @Param id path string true "Entry ID"
// @Param request body guestbook.UpdateGuestbookRequest true "Updated Entry"
// @Success 200 {object} models.Response{data=guestbook.GuestbookResponse} "Updated"
// @Failure 400 {object} models.Response "Bad Request"
// @Router /api/v1/guestbook/{id} [put]
func (h *GuestbookHandler) Update(c echo.Context) error {
	id := c.Param("id")

	var req guestbook.UpdateGuestbookRequest
	if err := c.Bind(&req); err != nil {
		logrus.WithError(err).Error("failed to bind update guestbook request")
		return models.ErrorResponse(c, "invalid request body")
	}

	if err := c.Validate(&req); err != nil {
		logrus.WithError(err).Error("failed to validate update guestbook request")
		return models.ErrorResponse(c, "invalid request data")
	}

	resp, err := h.uc.Update(c.Request().Context(), id, req)
	if err != nil {
		logrus.WithError(err).Error("failed to update guestbook entry")
		return models.ErrorResponse(c, "failed to update guestbook entry")
	}

	return models.SuccessResponse(c, resp, "guestbook entry updated successfully")
}

// Delete Guestbook
// @Summary Delete a guestbook entry
// @Description Delete a guestbook entry by ID.
// @Tags Guestbook
// @Accept json
// @Produce json
// @Param id path string true "Entry ID"
// @Success 200 {object} models.Response "Deleted"
// @Failure 400 {object} models.Response "Bad Request"
// @Router /api/v1/guestbook/{id} [delete]
func (h *GuestbookHandler) Delete(c echo.Context) error {
	id := c.Param("id")

	err := h.uc.Delete(c.Request().Context(), id)
	if err != nil {
		logrus.WithError(err).Error("failed to delete guestbook entry")
		return models.ErrorResponse(c, "failed to delete guestbook entry")
	}

	return models.SuccessResponse(c, nil, "guestbook entry deleted successfully")
}
