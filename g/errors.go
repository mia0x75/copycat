package g

import (
	"errors"
)

// Errors
var (
	ErrFileNotFound    = errors.New("文件不存在")
	ErrFileParse       = errors.New("配置解析错误")
	ErrOutOfRange      = errors.New("out of range")
	ErrKvDoesNotExists = errors.New("kv does not exists")
	ErrMembersEmpty    = errors.New("members is empty")
	ErrLeaderNotFound  = errors.New("leader not found")
	ErrNotRegister     = errors.New("service not register")
	ErrDataLenError    = errors.New("data len error")
	ErrNotConnect      = errors.New("not connect")
	ErrIsConnected     = errors.New("is connected")
	ErrWaitTimeout     = errors.New("wait timeout")
	ErrChanIsClosed    = errors.New("wait is closed")
	ErrUnknownError    = errors.New("unknown error")
	ErrMaxPackError    = errors.New("package len max then limit")
	ErrInvalidPackage  = errors.New("invalid package")
	ErrNetW            = errors.New("network is not connect")
)
