// internal/config/config_service.go
package service

import (
    "fmt"
    "os"
    "strconv"
    "strings"

    "gopkg.in/yaml.v3"
)

type ConfigService struct {
    config *Config
}

type Config struct {
    Boss BossConfig `yaml:"boss" json:"boss"`
    AI   AIConfig   `yaml:"ai" json:"ai"`
    DB   DBConfig   `yaml:"db" json:"db"`
}

type BossConfig struct {
    SayHi         string   `yaml:"say_hi" json:"say_hi"`
    Debugger      bool     `yaml:"debugger" json:"debugger"`
    Keywords      []string `yaml:"keywords" json:"keywords"`
    CityCode      []string `yaml:"city_code" json:"city_code"`
    JobType       string   `yaml:"job_type" json:"job_type"`
    Salary        []string `yaml:"salary" json:"salary"`
    Experience    []string `yaml:"experience" json:"experience"`
    Degree        []string `yaml:"degree" json:"degree"`
    Scale         []string `yaml:"scale" json:"scale"`
    Industry      []string `yaml:"industry" json:"industry"`
    Stage         []string `yaml:"stage" json:"stage"`
    EnableAI      bool     `yaml:"enable_ai" json:"enable_ai"`
    FilterDeadHR  bool     `yaml:"filter_dead_hr" json:"filter_dead_hr"`
    SendImgResume bool     `yaml:"send_img_resume" json:"send_img_resume"`
    ExpectedSalary []int   `yaml:"expected_salary" json:"expected_salary"`
    WaitTime      string   `yaml:"wait_time" json:"wait_time"`
    DeadStatus    []string `yaml:"dead_status" json:"dead_status"`
}

type DBConfig struct {
    Driver string `yaml:"driver" json:"driver"`
    DSN    string `yaml:"dsn" json:"dsn"`
}

func NewConfigService() *ConfigService {
    return &ConfigService{}
}

// Load 加载配置
func (s *ConfigService) Load() (*Config, error) {
    // 优先从环境变量加载
    config := s.loadFromEnv()
    
    // 如果环境变量没有配置，尝试从配置文件加载
    if config.Boss.SayHi == "" {
        fileConfig, err := s.loadFromFile("config.yaml")
        if err == nil {
            config = fileConfig
        }
    }
    
    s.config = config
    return config, nil
}

// loadFromEnv 从环境变量加载配置
func (s *ConfigService) loadFromEnv() *Config {
    config := &Config{}
    
    // Boss配置
    config.Boss.SayHi = os.Getenv("BOSS_SAY_HI")
    config.Boss.Debugger, _ = strconv.ParseBool(os.Getenv("BOSS_DEBUGGER"))
    config.Boss.Keywords = strings.Split(os.Getenv("BOSS_KEYWORDS"), ",")
    config.Boss.CityCode = strings.Split(os.Getenv("BOSS_CITY_CODE"), ",")
    config.Boss.JobType = os.Getenv("BOSS_JOB_TYPE")
    config.Boss.Salary = strings.Split(os.Getenv("BOSS_SALARY"), ",")
    config.Boss.Experience = strings.Split(os.Getenv("BOSS_EXPERIENCE"), ",")
    config.Boss.Degree = strings.Split(os.Getenv("BOSS_DEGREE"), ",")
    config.Boss.Scale = strings.Split(os.Getenv("BOSS_SCALE"), ",")
    config.Boss.Industry = strings.Split(os.Getenv("BOSS_INDUSTRY"), ",")
    config.Boss.Stage = strings.Split(os.Getenv("BOSS_STAGE"), ",")
    config.Boss.EnableAI, _ = strconv.ParseBool(os.Getenv("BOSS_ENABLE_AI"))
    config.Boss.FilterDeadHR, _ = strconv.ParseBool(os.Getenv("BOSS_FILTER_DEAD_HR"))
    config.Boss.SendImgResume, _ = strconv.ParseBool(os.Getenv("BOSS_SEND_IMG_RESUME"))
    config.Boss.WaitTime = os.Getenv("BOSS_WAIT_TIME")
    config.Boss.DeadStatus = strings.Split(os.Getenv("BOSS_DEAD_STATUS"), ",")
    
    // AI配置
    config.AI.BaseURL = os.Getenv("AI_BASE_URL")
    config.AI.APIKey = os.Getenv("AI_API_KEY")
    config.AI.Model = os.Getenv("AI_MODEL")
    
    // 数据库配置
    config.DB.Driver = os.Getenv("DB_DRIVER")
    config.DB.DSN = os.Getenv("DB_DSN")
    
    return config
}

// loadFromFile 从配置文件加载
func (s *ConfigService) loadFromFile(filename string) (*Config, error) {
    data, err := os.ReadFile(filename)
    if err != nil {
        return nil, fmt.Errorf("读取配置文件失败: %w", err)
    }
    
    var config Config
    if err := yaml.Unmarshal(data, &config); err != nil {
        return nil, fmt.Errorf("解析配置文件失败: %w", err)
    }
    
    return &config, nil
}

// GetBossConfig 获取Boss配置
func (s *ConfigService) GetBossConfig() *BossConfig {
    if s.config == nil {
        return &BossConfig{}
    }
    return &s.config.Boss
}

// GetAIConfig 获取AI配置
func (s *ConfigService) GetAIConfig() *AIConfig {
    if s.config == nil {
        return &AIConfig{}
    }
    return &s.config.AI
}

// GetDBConfig 获取数据库配置
func (s *ConfigService) GetDBConfig() *DBConfig {
    if s.config == nil {
        return &DBConfig{Driver: "sqlite", DSN: "boss.db"}
    }
    return &s.config.DB
}