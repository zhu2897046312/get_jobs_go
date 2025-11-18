package config

import (
	"fmt"

	"github.com/spf13/viper"
)

// 全局配置结构体
type GlobalConfig struct {
	Boss   BossConfig   `mapstructure:"boss"`
	Job51  Job51Config  `mapstructure:"job51"`
	Lagou  LagouConfig  `mapstructure:"lagou"`
	Liepin LiepinConfig `mapstructure:"liepin"`
	Zhilian ZhilianConfig `mapstructure:"zhilian"`
	AI     AIConfig     `mapstructure:"ai"`
	Bot    BotConfig    `mapstructure:"bot"`
}


// Job51 配置
type Job51Config struct {
	JobArea  []string `mapstructure:"jobArea"`
	Keywords []string `mapstructure:"keywords"`
	Salary   []string `mapstructure:"salary"`
}

// Lagou 配置
type LagouConfig struct {
	Keywords []string `mapstructure:"keywords"`
	CityCode string   `mapstructure:"cityCode"`
	Salary   string   `mapstructure:"salary"`
	Scale    []string `mapstructure:"scale"`
	GJ       string   `mapstructure:"gj"`
}

// Liepin 配置
type LiepinConfig struct {
	CityCode string   `mapstructure:"cityCode"`
	Keywords []string `mapstructure:"keywords"`
	Salary   string   `mapstructure:"salary"`
}

// 智联招聘配置
type ZhilianConfig struct {
	CityCode string   `mapstructure:"cityCode"`
	Salary   string   `mapstructure:"salary"`
	Keywords []string `mapstructure:"keywords"`
}

// AI 配置
type AIConfig struct {
	Introduce string `mapstructure:"introduce"`
	Prompt    string `mapstructure:"prompt"`
}

// Bot 配置
type BotConfig struct {
	IsSend bool `mapstructure:"is_send"`
}

// CityCode 城市代码枚举
type CityCode struct {
	Name string
	Code string
}

var CityCodes = map[string]CityCode{
	"不限":  {"不限", "0"},
	"北京":  {"北京", "530"},
	"上海":  {"上海", "538"},
	"广州":  {"广州", "763"},
	"深圳":  {"深圳", "765"},
	"成都":  {"成都", "801"},
}

// InitConfig 初始化配置
func InitConfig() (*GlobalConfig, error) {
	// 设置 Viper 配置
	viper.SetConfigName("config") // 配置文件名称（不带扩展名）
	viper.SetConfigType("yaml")   // 配置文件类型
	viper.AddConfigPath("./config") // 配置文件路径

	// 读取配置文件
	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %v", err)
	}

	// 解析配置文件到结构体
	var config GlobalConfig
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %v", err)
	}

	return &config, nil
}