package devConf

import (
	"github.com/ilyakaznacheev/cleanenv"
)

type DeviceConf struct {
	IP        []string  `yaml:"ips"`
	StendData StendData `yaml:"stend_data"`
}

type StendData struct {
	Username     string `yaml:"user_name"`
	Password     string `yaml:"password"`
	TftpServerIp string `yaml:"tftp_server_ip"`
}

func InitConf() *DeviceConf {
	var cfg DeviceConf

	if err := cleanenv.ReadConfig("../build/devConf.yaml", &cfg); err != nil {
		panic("failed to read device Config: 3 " + err.Error())
	}

	return &cfg
}
