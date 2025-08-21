package service

import (
	"MyChat/common"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path"
	"time"

	"github.com/gin-gonic/gin"
)

func Image(ctx *gin.Context) {
	w := ctx.Writer
	req := ctx.Request

	srcFile, head, err := req.FormFile("file")
	if err != nil {
		common.RespFail(w, err.Error())
		return
	}
	defer srcFile.Close()

	// 确保目录存在
	os.MkdirAll("./asset/upload", os.ModePerm)

	// 获取后缀
	suffix := path.Ext(head.Filename)
	if suffix == "" {
		suffix = ".png"
	}

	fileName := fmt.Sprintf("%d%04d%s", time.Now().Unix(), rand.Int31(), suffix)

	dstFile, err := os.Create("./asset/upload/" + fileName)
	if err != nil {
		common.RespFail(w, err.Error())
		return
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		common.RespFail(w, err.Error())
		return
	}

	// 注意返回 URL，而不是相对路径
	url := "/asset/upload/" + fileName
	common.RespOK(w, url, "发送成功")
}
