package common

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Response 统一响应结构
type Response struct {
	Code    int         `json:"code"`            // 状态码：0=成功，-1=失败
	Message string      `json:"message"`         // 消息
	Data    interface{} `json:"data,omitempty"`  // 数据
	Total   interface{} `json:"total,omitempty"` // 总数（用于列表）
}

// Success 成功响应
func Success(ctx *gin.Context, data interface{}, message string) {
	ctx.JSON(http.StatusOK, Response{
		Code:    0,
		Message: message,
		Data:    data,
	})
}

// SuccessWithTotal 成功响应（带总数）
func SuccessWithTotal(ctx *gin.Context, data interface{}, total interface{}, message string) {
	ctx.JSON(http.StatusOK, Response{
		Code:    0,
		Message: message,
		Data:    data,
		Total:   total,
	})
}

// Error 错误响应
func Error(ctx *gin.Context, code int, message string) {
	if code == 0 {
		code = -1
	}
	ctx.JSON(http.StatusOK, Response{
		Code:    code,
		Message: message,
	})
}

// ErrorWithData 错误响应（带数据）
func ErrorWithData(ctx *gin.Context, code int, message string, data interface{}) {
	if code == 0 {
		code = -1
	}
	ctx.JSON(http.StatusOK, Response{
		Code:    code,
		Message: message,
		Data:    data,
	})
}

// BadRequest 参数错误响应
func BadRequest(ctx *gin.Context, message string) {
	ctx.JSON(http.StatusBadRequest, Response{
		Code:    -1,
		Message: message,
	})
}

// InternalError 服务器错误响应
func InternalError(ctx *gin.Context, message string) {
	ctx.JSON(http.StatusInternalServerError, Response{
		Code:    -1,
		Message: message,
	})
}

// 兼容旧代码的响应函数
func Resp(w http.ResponseWriter, code int, data interface{}, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	h := Response{
		Code:    code,
		Data:    data,
		Message: msg,
	}
	ret, _ := json.Marshal(h)
	w.Write(ret)
}

func RespList(w http.ResponseWriter, code int, data interface{}, total interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	h := Response{
		Code:  code,
		Data:  data,
		Total: total,
	}
	ret, _ := json.Marshal(h)
	w.Write(ret)
}

func RespFail(w http.ResponseWriter, msg string) {
	Resp(w, -1, nil, msg)
}

func RespOK(w http.ResponseWriter, data interface{}, msg string) {
	Resp(w, 0, data, msg)
}

func RespOKList(w http.ResponseWriter, data interface{}, total interface{}) {
	RespList(w, 0, data, total)
}
