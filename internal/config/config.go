package config

import (
	"flag"
	"github.com/ilyakaznacheev/cleanenv"
	"os"
	"time"
)

var DataChannel = make(chan string)

type Config struct {
	Env         string     `yaml:"env" env-default:"local"`
	StoragePath string     `yaml:"storage_path" env-required:"true"`
	GRPC        GRPCConfig `yaml:"grpc"`
	IPS         []string   `yaml:"ips"`
	Username    string     `yaml:"user_name"`
	Password    string     `yaml:"password"`
	TFTPServer  string     `yaml:"tftp_server" `
}

type GRPCConfig struct {
	Port    int           `yaml:"port"`
	Timeout time.Duration `yaml:"timeout"`
}

func MustLoad() *Config {
	path := fetchConfigPath()

	if path == "" {
		panic("config file not exist 1")
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		panic("config file not exist: 2 " + path)
	}

	var cfg Config

	if err := cleanenv.ReadConfig(path, &cfg); err != nil {
		panic("failed to read config: 3 " + err.Error())
	}

	return &cfg
}

func fetchConfigPath() string {
	var res string
	var safe string

	flag.StringVar(&res, "config", "", "path to config file")
	flag.StringVar(&safe, "safe", "", "safe config file")
	flag.Parse()

	if res == "" {
		res = os.Getenv("CONFIG_PATH")
	}
	if safe == "" {
		safe = ""
	}
	return res
}
