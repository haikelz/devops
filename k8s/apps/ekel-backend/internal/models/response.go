package models

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

type Response struct {
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
	Code    int    `json:"code,omitempty"`
}

type PaginateData struct {
	Data  any   `json:"data"`
	Total int64 `json:"total"`
	Limit int   `json:"limit"`
	Start int   `json:"start"`
}

func SuccessResponse(c echo.Context, data any, msg string) error {
	return c.JSON(http.StatusOK, Response{
		Message: msg,
		Data:    data,
		Code:    http.StatusOK,
	})
}

func ErrorResponse(c echo.Context, msg string) error {
	return c.JSON(http.StatusBadRequest, Response{Message: msg})
}

func UnauthorizedResponse(c echo.Context, msg string) error {
	return c.JSON(http.StatusUnauthorized, Response{Message: msg})
}

func SuccessPaginateResponse(c echo.Context, data any, total int64, limit int, offset int, msg string) error {
	return c.JSON(http.StatusOK, Response{
		Message: msg,
		Data: PaginateData{
			Data:  data,
			Total: total,
			Limit: limit,
			Start: offset,
		},
		Code: http.StatusOK,
	})
}
