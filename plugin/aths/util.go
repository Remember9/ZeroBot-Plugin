package aths

import (
	"gopkg.in/yaml.v3"
	"os"
)

type Config struct {
	MySQL struct {
		Host     string `yaml:"host"`
		Port     string `yaml:"port"`
		User     string `yaml:"user"`
		Password string `yaml:"password"`
		DbName   string `yaml:"dbname"`
	} `yaml:"mysql"`
	System struct {
		AutoRestart bool `yaml:"auto_restart"`
	} `yaml:"system"`
}

func (c *Config) InitFromYaml(filename string) error {
	// 读取文件
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	// 解析YAML为结构体
	if err := yaml.Unmarshal(data, c); err != nil {
		return err
	}

	return nil
}

var _config *Config

func GetConfig() *Config {
	return _config
}
