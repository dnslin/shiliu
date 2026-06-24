package middleware

import (
	"bytes"
	"encoding/json"
	"github.com/duke-git/lancet/v2/cryptor"
	"github.com/duke-git/lancet/v2/random"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"io"
	"shiliu/pkg/log"
	"strings"
	"time"
)

func RequestLogMiddleware(logger *log.Logger) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		// The configuration is initialized once per request
		uuid, err := random.UUIdV4()
		if err != nil {
			return
		}
		trace := cryptor.Md5String(uuid)
		logger.WithValue(ctx, zap.String("trace", trace))
		logger.WithValue(ctx, zap.String("request_method", ctx.Request.Method))
		logger.WithValue(ctx, zap.Any("request_headers", ctx.Request.Header))
		logger.WithValue(ctx, zap.String("request_url", ctx.Request.URL.String()))
		if ctx.Request.Body != nil {
			bodyBytes, _ := ctx.GetRawData()
			ctx.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes)) // 关键点
			logger.WithValue(ctx, zap.String("request_params", redactSensitiveRequestBody(bodyBytes)))
		}
		logger.WithContext(ctx).Info("Request")
		ctx.Next()
	}
}

func redactSensitiveRequestBody(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	var payload interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		if strings.Contains(strings.ToLower(string(body)), "password") {
			return "[REDACTED]"
		}
		return string(body)
	}
	redacted := redactSensitiveJSON(payload)
	redactedBody, err := json.Marshal(redacted)
	if err != nil {
		return "[REDACTED]"
	}
	return string(redactedBody)
}

func redactSensitiveJSON(value interface{}) interface{} {
	switch v := value.(type) {
	case map[string]interface{}:
		redacted := make(map[string]interface{}, len(v))
		for key, field := range v {
			if strings.Contains(strings.ToLower(key), "password") {
				redacted[key] = "[REDACTED]"
				continue
			}
			redacted[key] = redactSensitiveJSON(field)
		}
		return redacted
	case []interface{}:
		redacted := make([]interface{}, len(v))
		for i, item := range v {
			redacted[i] = redactSensitiveJSON(item)
		}
		return redacted
	default:
		return value
	}
}
func ResponseLogMiddleware(logger *log.Logger) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		blw := &bodyLogWriter{body: bytes.NewBufferString(""), ResponseWriter: ctx.Writer}
		ctx.Writer = blw
		startTime := time.Now()
		ctx.Next()
		duration := time.Since(startTime).String()
		logger.WithContext(ctx).Info("Response", zap.Any("response_body", blw.body.String()), zap.Any("time", duration))
	}
}

type bodyLogWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w bodyLogWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}
