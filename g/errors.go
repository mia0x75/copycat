package g

import (
	"errors"
)

var (
	errFileNotFound = errors.New("文件不存在")
	errFileParse    = errors.New("配置解析错误")
	errOutOfRange   = errors.New("out of range")
)
