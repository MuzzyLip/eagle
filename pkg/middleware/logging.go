package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"regexp"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/willf/pad"

	"github.com/go-eagle/eagle/pkg/app"
	"github.com/go-eagle/eagle/pkg/errcode"
	"github.com/go-eagle/eagle/pkg/log"
)

type bodyLogWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w bodyLogWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

// Logging is a middleware function that logs the each request.
func Logging() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 记录请求开始时间
		start := time.Now().UTC()
		path := c.Request.URL.Path

		// 匹配请求路径，只记录/v1/user和/login的请求日志
		reg := regexp.MustCompile("(/v1/user|/login)")
		// 如果请求路径不匹配，则返回
		if !reg.MatchString(path) {
			return
		}

		// Read the Body content
		var bodyBytes []byte
		// 如果请求体不为空，则读取请求体
		if c.Request.Body != nil {
			bodyBytes, _ = io.ReadAll(c.Request.Body)
		}

		// Restore the io.ReadCloser to its original state
		// 恢复请求体
		c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		// The basic informations.
		// 获取请求方法
		method := c.Request.Method
		// 获取客户端IP
		ip := c.ClientIP()

		//log.Debugf("New request come in, path: %s, Method: %s, body `%s`", path, method, string(bodyBytes))
		// 创建一个bodyLogWriter实例，用于记录响应体
		blw := &bodyLogWriter{
			body:           bytes.NewBufferString(""),
			ResponseWriter: c.Writer,
		}
		c.Writer = blw

		// Continue.
		c.Next()

		// Calculates the latency.
		// 计算请求处理时间
		end := time.Now().UTC()
		latency := end.Sub(start)

		// 获取响应状态码
		var code int
		var message string

		// get code and message
		var response app.Response
		if err := json.Unmarshal(blw.body.Bytes(), &response); err != nil {
			log.Errorf("response body can not unmarshal to model.Response struct, body: `%s`, err: %+v",
				blw.body.Bytes(), err)
			code = errcode.ErrInternalServer.Code()
			message = err.Error()
		} else {
			code = response.Code
			message = response.Message
		}

		// nolint: typecheck
		// 记录请求日志，格式为：请求处理时间 | 客户端IP | 请求方法 | 请求路径 | 响应状态码 | 响应状态码 | 响应消息
		log.Infof("%-13s | %-12s | %s %s | %d | {code: %d, message: %s}", latency, ip,
			pad.Right(method, 5, ""), path, blw.Status(), code, message)
	}
}
