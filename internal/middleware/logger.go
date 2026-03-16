package middleware

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/gin-gonic/gin"
)

// LoggerWithWriter кастомный логгер с request_id
func LoggerWithWriter(out io.Writer) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery
		c.Next()
		latency := time.Since(start)
		status := c.Writer.Status()
		method := c.Request.Method
		requestID, _ := c.Get(RequestIDKey)
		fmt.Fprintf(out, "[HTTP] %s | %3d | %12v | %15s | %7s | %s%s | req_id=%s\n",
			time.Now().Format("2006-01-02 15:04:05"),
			status,
			latency,
			c.ClientIP(),
			method,
			path,
			func() string {
				if query == "" {
					return ""
				}
				return "?" + query
			}(),
			requestID,
		)
	}
}

// PanicRecoveryHandler обрабатывает паники с логированием и request_id
// Соответствует требованию ТЗ: "Промежуточный слой: request_id, логгирование, recover"
func PanicRecoveryHandler(c *gin.Context, err any) {
	requestID, _ := c.Get(RequestIDKey)

	// Логируем детали паники со стеком для отладки
	log.Printf("[PANIC] req_id=%s | error=%+v\n", requestID, err)
	log.Printf("[PANIC] Stack trace: %s\n", string(debug.Stack()))

	// Прерываем выполнение
	c.Abort()

	// Возвращаем безопасный ответ в едином формате ошибок
	c.JSON(http.StatusInternalServerError, gin.H{
		"error":      "internal_server_error",
		"message":    "unexpected error occurred",
		"request_id": requestID,
	})
}
