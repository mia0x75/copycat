package consul

import (
	"fmt"

	"github.com/hashicorp/consul/api"
)

// Session TODO
type Session struct {
	session *api.Session
}

// ISession TODO
type ISession interface {
	// Create TODO
	Create(timeout int64) (string, error)
	// Destroy TODO
	Destroy(sessionID string) error
	// Renew TODO
	Renew(sessionID string) error
}

// NewSession TODO
func NewSession(session *api.Session) ISession {
	s := &Session{
		session: session,
	}
	return s
}

// Create create a session
// timeout unit is seconds
// return session id and error, if everything is ok, error should be nil
func (session *Session) Create(timeout int64) (string, error) {
	se := &api.SessionEntry{
		Behavior: api.SessionBehaviorDelete,
	}
	// timeout min value is 10 seconds
	if timeout > 0 && timeout < 10 {
		timeout = 10
	}
	if timeout > 0 {
		se.TTL = fmt.Sprintf("%ds", timeout)
	}
	ID, _, err := session.session.Create(se, nil)
	return ID, err
}

// Destroy destory a session
// sessionId is the value return from Create
func (session *Session) Destroy(sessionID string) error {
	_, err := session.session.Destroy(sessionID, nil)
	return err
}

// Renew refresh a session
// sessionId is the value return from Create
func (session *Session) Renew(sessionID string) error {
	_, _, err := session.session.Renew(sessionID, nil)
	return err
}
