package binlog

import (
	"fmt"
	"testing"
)

func TestGetSession(t *testing.T) {
	session := GetSession()
	fmt.Println("session=", session)
	if session == "" {
		t.Error("get session error")
	}
	session2 := GetSession()
	fmt.Println("session=", session2)
	if session != session2 {
		t.Error("get session error")
	}
}
