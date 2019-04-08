package config

import (
	"github.com/sirupsen/logrus"
)

var log = logrus.New()

type AppConfig struct {
	LogLevel int          `toml:"log_level"` //
	TimeZone string       `toml:"time_zone"` //
	PProfCfg *PProfConfig `toml:"pprof"`     //
}

type PProfConfig struct {
	Enable bool   `toml:"enable"` //
	Listen string `toml:"listen"` //
	Port   uint16 `toml:"port"`   //
}
