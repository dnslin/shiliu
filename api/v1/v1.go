package v1

import (
	"errors"
	"math"
	"net/http"
	"reflect"
	"strconv"

	"github.com/gin-gonic/gin"
)

const (
	DefaultPage     = 1
	DefaultPageSize = 20
	MaxPageSize     = 100
)

type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

type PageRequest struct {
	Page     int `form:"page" json:"page"`
	PageSize int `form:"pageSize" json:"pageSize"`
}

type PageMeta struct {
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
	Total    int64 `json:"total"`
}

type PageData struct {
	Items interface{} `json:"items"`
	Page  PageMeta    `json:"page"`
}

func ParsePageRequest(ctx *gin.Context) PageRequest {
	page := PageRequest{
		Page:     parseQueryInt(ctx, "page", DefaultPage),
		PageSize: parseQueryInt(ctx, "pageSize", DefaultPageSize),
	}
	return page.Normalize()
}

func (p PageRequest) Normalize() PageRequest {
	if p.Page < 1 {
		p.Page = DefaultPage
	}
	if p.PageSize < 1 {
		p.PageSize = DefaultPageSize
	}
	if p.PageSize > MaxPageSize {
		p.PageSize = MaxPageSize
	}
	return p
}

func (p PageRequest) LimitOffset() (int, int) {
	_, limit, offset := p.LimitOffsetPage()
	return limit, offset
}

func (p PageRequest) LimitOffsetPage() (PageRequest, int, int) {
	page := p.Normalize()
	maxPage := math.MaxInt / page.PageSize
	if maxPage < math.MaxInt {
		maxPage++
	}
	if page.Page > maxPage {
		page.Page = maxPage
	}
	return page, page.PageSize, (page.Page - 1) * page.PageSize
}

func NewPageData(items interface{}, page PageRequest, total int64) PageData {
	normalized := page.Normalize()
	return PageData{
		Items: normalizePageItems(items),
		Page: PageMeta{
			Page:     normalized.Page,
			PageSize: normalized.PageSize,
			Total:    total,
		},
	}
}

func normalizePageItems(items interface{}) interface{} {
	if items == nil {
		return []interface{}{}
	}
	value := reflect.ValueOf(items)
	if value.Kind() == reflect.Slice && value.IsNil() {
		return reflect.MakeSlice(value.Type(), 0, 0).Interface()
	}
	return items
}

func parseQueryInt(ctx *gin.Context, key string, fallback int) int {
	value := ctx.Query(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func HandleSuccess(ctx *gin.Context, data interface{}) {
	if data == nil {
		data = map[string]interface{}{}
	}
	resp := Response{Code: errorCodeMap[ErrSuccess], Message: ErrSuccess.Error(), Data: data}
	if _, ok := errorCodeMap[ErrSuccess]; !ok {
		resp = Response{Code: 0, Message: "", Data: data}
	}
	ctx.JSON(http.StatusOK, resp)
}

func HandleError(ctx *gin.Context, httpCode int, err error, data interface{}) {
	if data == nil {
		data = map[string]string{}
	}
	resp := Response{Code: errorCodeMap[err], Message: err.Error(), Data: data}
	if _, ok := errorCodeMap[err]; !ok {
		resp = Response{Code: 500, Message: "unknown error", Data: data}
	}
	ctx.JSON(httpCode, resp)
}

type Error struct {
	Code    int
	Message string
}

var errorCodeMap = map[error]int{}

func newError(code int, msg string) error {
	err := errors.New(msg)
	errorCodeMap[err] = code
	return err
}
func (e Error) Error() string {
	return e.Message
}
