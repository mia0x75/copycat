package services

import (
	"errors"
	"regexp"
)

func Pack(cmd int, msg []byte) []byte {
	l := len(msg)
	r := make([]byte, l+6)
	cl := l + 2
	r[0] = byte(cl)
	r[1] = byte(cl >> 8)
	r[2] = byte(cl >> 16)
	r[3] = byte(cl >> 24)
	r[4] = byte(cmd)
	r[5] = byte(cmd >> 8)
	copy(r[6:], msg)
	return r
}

var DataLenError = errors.New("data len error")

func Unpack(data []byte) (int, []byte, error) {
	clen := int(data[0]) | int(data[1])<<8 |
		int(data[2])<<16 | int(data[3])<<24
	if len(data) < clen+4 {
		return 0, nil, DataLenError
	}
	cmd := int(data[4]) | int(data[5])<<8
	content := data[6 : clen+4]
	return cmd, content, nil
}

func hasCmd(cmd int) bool {
	return cmd == CMD_SET_PRO ||
		cmd == CMD_AUTH ||
		cmd == CMD_ERROR ||
		cmd == CMD_TICK ||
		cmd == CMD_EVENT ||
		cmd == CMD_AGENT ||
		cmd == CMD_STOP ||
		cmd == CMD_RELOAD ||
		cmd == CMD_SHOW_MEMBERS ||
		cmd == CMD_POS
}

func MatchFilters(filters []string, table string) bool {
	if filters == nil || len(filters) <= 0 {
		return true
	}
	for _, f := range filters {
		match, err := regexp.MatchString(f, table)
		if match && err == nil {
			return true
		}
	}
	return false
}

func PackPro(flag int, content []byte) []byte {
	// 数据打包
	l := len(content) + 3
	r := make([]byte, l+4)
	// 4字节数据包长度
	r[0] = byte(l)
	r[1] = byte(l >> 8)
	r[2] = byte(l >> 16)
	r[3] = byte(l >> 24)
	// 2字节cmd
	r[4] = byte(CMD_SET_PRO)
	r[5] = byte(CMD_SET_PRO >> 8)
	r[6] = byte(flag)
	// 实际数据内容
	r = append(r[:7], content...)
	return r
}
