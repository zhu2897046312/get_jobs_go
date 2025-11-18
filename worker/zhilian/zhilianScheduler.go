package zhilian

import (
	"fmt"
	"get_jobs_go/config"
	"time"

	log "github.com/sirupsen/logrus"
)

// Schedule 定时执行任务
func Schedule(configPath string, interval time.Duration) {
	for {
		if err := RunOnce(configPath); err != nil {
			log.Errorf("执行任务失败: %v", err)
		}
		time.Sleep(interval)
	}
}

// RunOnce 执行一次任务
func RunOnce(configPath string) error {
	
	// 加载配置文件
	globalConfig, err := config.InitConfig()
	if err != nil {
		return fmt.Errorf("加载配置文件失败: %v", err)
	}

	zhilian := New(&globalConfig.Zhilian)
	return zhilian.Run()
}