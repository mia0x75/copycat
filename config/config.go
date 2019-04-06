package config

import (
	"github.com/BurntSushi/toml"
	"github.com/sirupsen/logrus"

	"github.com/mia0x75/nova/app"
	"github.com/mia0x75/nova/file"
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

func GetAppConfig() (*AppConfig, error) {
	var appCfg AppConfig
	cfgFile := app.APP_CONFIG_FILE
	wfile := file.FileInfo{cfgFile}
	if !wfile.Exists() {
		log.Println(app.ErrorFileNotFound)
		return nil, app.ErrorFileNotFound
	}
	if _, err := toml.DecodeFile(cfgFile, &appCfg); err != nil {
		log.Println(app.ErrorFileParse)
		return nil, app.ErrorFileParse
	}

	if appCfg.TimeZone == "" {
		appCfg.TimeZone = "Local"
	}

	return &appCfg, nil
}
