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

// ControlConfig 控制配置
type ControlConfig struct {
	Listen string `json:"listen"`
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

// HTTPGroupConfig 分组配置
type HTTPGroupConfig struct {
	Name      string   `json:"name"`   // 分组名称
	Filter    []string `json:"filter"` // 过滤条件
	Endpoints []string `json:"endpoints"`
}

// TCPGroupConfig 分组配置
type TCPGroupConfig struct {
	Name   string   `json:"name"`   // 分组名称
	Filter []string `json:"filter"` // 过滤条件
}

// HTTPConfig HTTP配置
type HTTPConfig struct {
	Enabled  bool               `json:"enabled"`   //
	TimeTick uint32             `json:"time_tick"` //
	Groups   []*HTTPGroupConfig `json:"groups"`    //
}

// TCPConfig TCP配置
type TCPConfig struct {
	Enabled   bool              `json:"enabled"`    //
	Addr      string            `json:"addr"`       //
	Port      uint16            `json:"port"`       //
	ServiceIp string            `json:"service_ip"` //
	Groups    []*TCPGroupConfig `json:"groups"`     //
}

// GlobalConfig 系统配置
type GlobalConfig struct {
	Log      *LogConfig      `json:"log"`       //
	Control  *ControlConfig  `json:"control"`   //
	TimeZone string          `json:"time_zone"` //
	Database *DatabaseConfig `json:"database"`  //
	HTTP     *HTTPConfig     `json:"http"`      //
	TCP      *TCPConfig      `json:"tcp"`       //
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
			"/etc/nova/cfg.json",
			filepath.Join(baseDir, "etc", "cfg.json"),
			filepath.Join(baseDir, "cfg.json"),
		}
	} else {
		configs = []string{cfg}
	}

	for _, config := range configs {
		if _, err = os.Stat(config); err == nil {
			ConfigFile = config
			log.Printf("[I] Loading config from: %s", config)
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

	if c.HTTP.TimeTick <= 0 {
		c.HTTP.TimeTick = 1
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
