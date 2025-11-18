package zhilian

import (
	// "fmt"
	"fmt"
	"os"
	// log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

// Config 智联招聘配置
type Config struct {
	Keywords  []string `yaml:"keywords"`   // 搜索关键词列表
	CityCode  string   `yaml:"cityCode"`   // 城市编码
	Salary    string   `yaml:"salary"`     // 薪资范围
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

// LoadConfig 加载配置文件
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile("E:\\WorkSpace\\get_job__go\\config\\config.yaml")
	
	if err != nil {
		return nil, err
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	// 转换城市编码
	if code, exists := CityCodes[config.CityCode]; exists {
		config.CityCode = code.Code
	} else {
		config.CityCode = "0" // 默认不限
	}

	// 转换薪资范围
	if config.Salary == "不限" {
		config.Salary = "0"
	}

	fmt.Println(config)
	return &config, nil
}

