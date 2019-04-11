package g

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"
	"github.com/toolkits/file"
)

// LogConfig 日志配置
type LogConfig struct {
	Level string `json:"level"` //
}

// AdminConfig Web配置
type AdminConfig struct {
	Enabled bool   `json:"enabled"`
	Listen  string `json:"listen"`
}

// DatabaseConfig 数据库链接及日志信息配置
type DatabaseConfig struct {
	Host            string `json:"host"`             //
	Port            uint16 `json:"port"`             //
	User            string `json:"user"`             //
	Password        string `json:"password"`         //
	Charset         string `json:"charset"`          //
	ServerID        uint32 `json:"server_id"`        //
	Flavor          string `json:"flavor"`           //
	HeartbeatPeriod uint64 `json:"heartbeat_period"` //
	ReadTimeout     uint32 `json:"read_timeout"`     //
	BinlogFile      string `json:"binlog_file"`      //
	BinlogPos       uint32 `json:"binlog_pos"`       //
}

// AgentConfig 代理配置
type AgentConfig struct {
	Enabled bool   `json:"enabled"` // 是否启用集群功能，单机模式下可以选择关闭集群
	Lock    string `json:"lock"`    // 同一个集群内的所有节点的lock key应该相同
	Listen  string `json:"listen"`  //
	Consul  string `json:"consul"`  //
}

// ConsulConfig Consul配置
type ConsulConfig struct {
	Enabled bool   `json:"enabled"`
	Addr    string `json:"addr"`
}

// GlobalConfig 系统配置
type GlobalConfig struct {
	Log      *LogConfig      `json:"log"`       //
	Admin    *AdminConfig    `json:"admin"`     //
	TimeZone string          `json:"time_zone"` //
	Database *DatabaseConfig `json:"database"`  //
	Listen   string          `json:"listen"`    //
	Consul   *ConsulConfig   `json:"consul"`    //
	Agent    *AgentConfig    `json:"agent"`     //
}

var (
	// ConfigFile 配置文件
	ConfigFile string
	config     *GlobalConfig
	configLock = new(sync.RWMutex)
)

// Config 返回当前的配置
func Config() *GlobalConfig {
	configLock.RLock()
	defer configLock.RUnlock()
	return config
}

// ParseConfig 从配置文件读取配置，反序列化成配置对象
func ParseConfig(cfg string) *GlobalConfig {
	var configs []string
	var path, content string
	var err error

	path, err = os.Executable()
	if err != nil {
		log.Fatalf("[F] 错误信息: %s", err.Error())
	}
	baseDir := filepath.Dir(path)

	// 指定了配置文件优先读配置文件，未指定配置文件按如下顺序加载，先找到哪个加载哪个
	if strings.TrimSpace(cfg) == "" {
		configs = []string{
			"/etc/copycat/cfg.json",
			filepath.Join(baseDir, "etc", "cfg.json"),
			filepath.Join(baseDir, "cfg.json"),
		}
	} else {
		configs = []string{cfg}
	}

	for _, config := range configs {
		if _, err = os.Stat(config); err == nil {
			ConfigFile = config
			break
		}
	}
	if err != nil {
		log.Fatalf("[F] 读取配置文件错误。")
	}

	content, err = file.ToTrimString(ConfigFile)
	if err != nil {
		log.Fatalf("[F] 读取配置文件 \"%s\" 错误: %s", ConfigFile, err.Error())
	}

	var c GlobalConfig
	err = json.Unmarshal([]byte(content), &c)
	if err != nil {
		log.Fatalf("[F] 解析配置文件 \"%s\" 错误: %s", ConfigFile, err.Error())
	}

	configLock.Lock()
	defer configLock.Unlock()

	config = &c

	log.Debugf("[D] 读取配置文件 \"%s\" 成功。", ConfigFile)

	return config
}

// Reload 重新加载配置文件
func Reload() *GlobalConfig {
	return ParseConfig(ConfigFile)
}
