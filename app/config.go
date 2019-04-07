package app

import (
	"time"

	"github.com/BurntSushi/toml"
	log "github.com/sirupsen/logrus"

	"github.com/mia0x75/nova/file"
)

type AppConfig struct {
	LogLevel      int    `toml:"log_level"`
	ControlListen string `toml:"control_listen"` // = "0.0.0:6061"
	PprofListen   string `toml:"pprof_listen"`
	TimeZone      string `toml:"time_zone"`
}

type HttpNodeConfig struct {
	Name   string
	Nodes  []string
	Filter []string
}

type HttpConfig struct {
	Enable   bool
	TimeTick time.Duration //故障检测的时间间隔，单位为秒
	Groups   map[string]HttpNodeConfig
}

type TcpConfig struct {
	Listen    string `toml:"listen"`
	Port      int    `toml:"port"`
	Enable    bool   `toml:"enable"`
	ServiceIp string `toml:"service_ip"`
	Groups    TcpGroupConfigs
}

type TcpGroupConfig struct {
	Name   string
	Filter []string
}

type TcpGroupConfigs map[string]TcpGroupConfig

func (cs *TcpGroupConfigs) HasName(name string) bool {
	for _, group := range *cs {
		if name == group.Name {
			return true
		}
	}
	return false
}

// mysql config
type MysqlConfig struct {
	Addr            string        `toml:"addr"`             // mysql service ip and port, like: "127.0.0.1:3306"
	User            string        `toml:"user"`             // mysql service user
	Password        string        `toml:"password"`         // mysql password
	Charset         string        `toml:"charset"`          // mysql default charset
	ServerID        uint32        `toml:"server_id"`        // mysql binlog client id, it must be unique
	Flavor          string        `toml:"flavor"`           // mysql or mariadb
	HeartbeatPeriod time.Duration `toml:"heartbeat_period"` // heartbeat interval, unit is ns, 30000000000  = 30s   1000000000 = 1s
	ReadTimeout     time.Duration `toml:"read_timeout"`     // read timeout, unit is ns, 0 is never timeout, 30000000000  = 30s   1000000000 = 1s
	BinFile         string        `toml:"bin_file"`         // read start form the binlog file
	BinPos          uint32        `toml:"bin_pos"`          // read start form the pos
}

type ConsulAddr struct {
	Address string `toml:"address"`
}

// debug mode, default is false
var DEBUG = false

func GetAppConfig() (*AppConfig, error) {
	var appConfig AppConfig
	configFile := APP_CONFIG_FILE
	if !file.Exists(configFile) {
		log.Errorf("config file %s does not exists", configFile)
		return nil, ErrorFileNotFound
	}
	if _, err := toml.DecodeFile(configFile, &appConfig); err != nil {
		log.Errorf("config file parse with error: %+v", err)
		return nil, ErrorFileParse
	}
	if appConfig.TimeZone == "" {
		appConfig.TimeZone = "Local"
	}
	return &appConfig, nil
}
