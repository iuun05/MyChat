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

// Image 上传图片
func Image(ctx *gin.Context) {
	w := ctx.Writer
	req := ctx.Request

	srcFile, head, err := req.FormFile("file")
	if err != nil {
		common.RespFail(w, err.Error())
		return
	}
	defer srcFile.Close()

	os.MkdirAll("./asset/upload", os.ModePerm)

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

	url := "/asset/upload/" + fileName
	common.RespOK(w, url, "发送成功")
}
