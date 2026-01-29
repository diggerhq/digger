package pagination

import (
	"strconv"

	"github.com/labstack/echo/v4"
)

type Params struct {
	Page     int
	PageSize int
}

func (p Params) Offset() int {
	if p.Page < 1 {
		return 0
	}
	return (p.Page - 1) * p.PageSize
}

func Parse(c echo.Context, defaultSize, maxSize int) Params {
	page := parseInt(c.QueryParam("page"), 1)
	size := parseInt(c.QueryParam("page_size"), defaultSize)

	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = defaultSize
	}
	if maxSize > 0 && size > maxSize {
		size = maxSize
	}

	return Params{
		Page:     page,
		PageSize: size,
	}
}

func parseInt(val string, fallback int) int {
	if val == "" {
		return fallback
	}
	if parsed, err := strconv.Atoi(val); err == nil {
		return parsed
	}
	return fallback
}
