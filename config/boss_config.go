// config/config.go
package config

import (
	"get_jobs_go/utils"
	"log"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// BossConfig Boss直聘配置数据结构
type BossConfig struct {
	SayHi          string            `yaml:"sayHi"`
	Debugger       bool              `yaml:"debugger"`
	Keywords       []string          `yaml:"keywords"`
	CityCode       []string          `yaml:"cityCode"`
	CustomCityCode map[string]string `yaml:"customCityCode"`
	Industry       []string          `yaml:"industry"`
	Experience     []string          `yaml:"experience"`
	JobType        string            `yaml:"jobType"`
	Salary         []string          `yaml:"salary"` // 改为 []string，因为Java中是List<String>
	Degree         []string          `yaml:"degree"`
	Scale          []string          `yaml:"scale"`
	Stage          []string          `yaml:"stage"`
	EnableAI       bool              `yaml:"enableAI"`
	FilterDeadHR   bool              `yaml:"filterDeadHR"`
	SendImgResume  bool              `yaml:"sendImgResume"`
	ExpectedSalary []int             `yaml:"expectedSalary"`
	WaitTime       string            `yaml:"waitTime"`
	DeadStatus     []string          `yaml:"deadStatus"`
}

var GlobalConfig Config

// Config 全局配置结构
type Config struct {
	Boss BossConfig `yaml:"boss"`
}

// LoadConfig 加载配置文件
func LoadConfig(configPath string) (*Config, error) {
	// 获取项目根目录
	root, err := utils.GetProjectRoot()
	if err != nil {
		return nil, err
	}

	// 如果未指定配置文件路径，使用默认路径
	if configPath == "" {
		configPath = filepath.Join(root, "config", "config.yaml")
	} else {
		// 如果是相对路径，转换为基于项目根目录的绝对路径
		if !filepath.IsAbs(configPath) {
			configPath = filepath.Join(root, configPath)
		}
	}

	log.Printf("尝试加载配置文件: %s", configPath)

	// 检查配置文件是否存在
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, err
	}

	// 读取配置文件
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	log.Printf("配置文件内容:\n%s", string(data))

	if err := yaml.Unmarshal(data, &GlobalConfig); err != nil {
		log.Printf("YAML解析错误: %v", err)
		return nil, err
	}
	
	log.Printf("配置加载成功: %+v", GlobalConfig)
	return &GlobalConfig, nil
}

// SaveConfig 保存配置到文件
func SaveConfig(config *Config, configPath string) error {
	// 获取项目根目录
	root, err := utils.GetProjectRoot()
	if err != nil {
		return err
	}

	if configPath == "" {
		configPath = filepath.Join(root, "config", "config.yaml")
	} else {
		// 如果是相对路径，转换为基于项目根目录的绝对路径
		if !filepath.IsAbs(configPath) {
			configPath = filepath.Join(root, configPath)
		}
	}

	// 确保目录存在
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// 序列化为YAML
	data, err := yaml.Marshal(config)
	if err != nil {
		return err
	}

	// 写入文件
	return os.WriteFile(configPath, data, 0644)
}