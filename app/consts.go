package app

const (
	APP_CONFIG_FILE   = "/etc/nova/nova.toml"
	DB_CONFIG_FILE    = "/etc/nova/db.toml"
	HTTP_CONFIG_FILE  = "/etc/nova/http.toml"
	TCP_CONFIG_FILE   = "/etc/nova/tcp.toml"
	AGENT_CONFIG_FILE = "/etc/nova/agent.toml"

	PID_FILE         = "/var/run/nova/nova.pid"
	LOG_FILE         = "/var/log/nova/nova.log"
	SESSION_FILE     = "/var/run/nova/session"
	TOKEN_FILE       = "/var/run/nova/token"
	MASTER_INFO_FILE = "/var/run/nova/nova.cache"
)
