package middleware

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/go-eagle/eagle/pkg/metric"
)

var namespace = ""

var (
	labels = []string{"status", "handler", "method", "service"}

	// QPS
	reqCount = metric.NewCounterVec(
		&metric.CounterVecOpts{
			Namespace: namespace,
			Name:      "http_requests_total",
			Help:      "Total number of HTTP requests made.",
			Labels:    labels,
		})

	// 当前正在处理请求的QPS
	curReqCount = metric.NewGaugeVec(
		&metric.GaugeVecOpts{
			Namespace: namespace,
			Name:      "http_requests_in_flight",
			Help:      "Current number of http requests in flight.",
			Labels:    labels,
		})

	// 接口响应时间
	reqDuration = metric.NewHistogramVec(
		&metric.HistogramVecOpts{
			Namespace: namespace,
			Name:      "http_request_duration_seconds",
			Help:      "HTTP request latencies in seconds.",
			Labels:    labels,
		})

	// 请求大小
	reqSizeBytes = metric.NewHistogramVec(
		&metric.HistogramVecOpts{
			Namespace: namespace,
			Name:      "http_request_size_bytes",
			Help:      "HTTP request sizes in bytes.",
			Labels:    labels,
		})

	// 响应大小
	respSizeBytes = metric.NewHistogramVec(
		&metric.HistogramVecOpts{
			Namespace: namespace,
			Name:      "http_response_size_bytes",
			Help:      "HTTP request sizes in bytes.",
			Labels:    labels,
		})
)

// calcRequestSize returns the size of request object.
func calcRequestSize(r *http.Request) float64 {
	size := 0
	if r.URL != nil {
		size = len(r.URL.String())
	}

	size += len(r.Method)
	size += len(r.Proto)

	for name, values := range r.Header {
		size += len(name)
		for _, value := range values {
			size += len(value)
		}
	}
	size += len(r.Host)

	// r.Form and r.MultipartForm are assumed to be included in r.URL.
	if r.ContentLength != -1 {
		size += int(r.ContentLength)
	}
	return float64(size)
}

// Metrics returns a gin.HandlerFunc for exporting some Web metrics
// 记录请求总数、处理中请求数、请求耗时、请求大小、响应大小
func Metrics(serviceName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 记录请求开始时间
		start := time.Now()
		c.Next()

		// 获取响应状态码
		status := fmt.Sprintf("%d", c.Writer.Status())
		// 获取请求路径
		handler := c.Request.URL.Path
		// 获取请求方法
		method := c.Request.Method

		labels := []string{status, handler, method, serviceName}

		// no response content will return -1
		// 获取响应大小
		respSize := c.Writer.Size()
		if respSize < 0 {
			respSize = 0
		}
		// 增加当前正在处理请求的QPS
		curReqCount.Inc(labels...)
		// 延迟减少当前正在处理请求的QPS
		defer curReqCount.Dec(labels...)
		// 增加请求总数
		reqCount.Inc(labels...)
		// 记录请求响应时间
		reqDuration.Observe(int64(time.Since(start).Seconds()), labels...)
		// 记录请求大小
		reqSizeBytes.Observe(int64(calcRequestSize(c.Request)), labels...)
		// 记录响应大小
		respSizeBytes.Observe(int64(respSize), labels...)
	}
}
