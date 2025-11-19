// boss/boss_job_service.go
package boss

import (
	"fmt"
	"log"
	"sync"
	"time"

	"get_jobs_go/service"
	"get_jobs_go/worker/playwright_manager"
)

// JobProgressMessage 任务进度消息
type JobProgressMessage struct {
	Platform  string `json:"platform"`
	Type      string `json:"type"` // info, warning, error, progress, success
	Message   string `json:"message"`
	Current   *int   `json:"current,omitempty"`
	Total     *int   `json:"total,omitempty"`
	Timestamp int64  `json:"timestamp"`
}

// JobPlatformService 任务平台服务接口
type JobPlatformService interface {
	ExecuteDelivery(progressCallback func(message JobProgressMessage)) error
	StopDelivery() error
	GetStatus() map[string]interface{}
	GetPlatformName() string
	IsRunning() bool
}

// BossJobService Boss直聘任务服务
type BossJobService struct {
	playwrightManager *playwright_manager.PlaywrightManager
	configService     *service.ConfigService
	bossProvider      func() *Boss

	running     bool
	shouldStop  bool
	statusMutex sync.RWMutex
	platform    string
}

// NewBossJobService 创建Boss任务服务
func NewBossJobService(
	playwrightManager *playwright_manager.PlaywrightManager,
	configService *service.ConfigService,
	bossProvider func() *Boss,
) *BossJobService {
	return &BossJobService{
		playwrightManager: playwrightManager,
		configService:     configService,
		bossProvider:      bossProvider,
		platform:          "boss",
	}
}

// ExecuteDelivery 执行投递任务
func (s *BossJobService) ExecuteDelivery(progressCallback func(message JobProgressMessage)) error {
	s.statusMutex.Lock()
	if s.running {
		s.statusMutex.Unlock()
		progressCallback(JobProgressMessage{
			Platform:  s.platform,
			Type:      "warning",
			Message:   "任务已在运行中",
			Timestamp: time.Now().UnixMilli(),
		})
		return nil
	}
	s.running = true
	s.shouldStop = false
	s.statusMutex.Unlock()

	defer func() {
		s.statusMutex.Lock()
		s.running = false
		s.shouldStop = false
		s.statusMutex.Unlock()
	}()

	// 获取Boss页面实例
	page := s.playwrightManager.GetBossPage()
	if page == nil {
		progressCallback(JobProgressMessage{
			Platform:  s.platform,
			Type:      "error",
			Message:   "Boss页面未初始化",
			Timestamp: time.Now().UnixMilli(),
		})
		return nil
	}

	// 检查登录状态
	if !s.playwrightManager.IsLoggedIn(s.platform) {
		progressCallback(JobProgressMessage{
			Platform:  s.platform,
			Type:      "error",
			Message:   "请先登录Boss直聘",
			Timestamp: time.Now().UnixMilli(),
		})
		return nil
	}

	// 暂停后台监控
	s.playwrightManager.PauseBossMonitoring()
	defer s.playwrightManager.ResumeBossMonitoring()

	// 加载配置
	bossConfig, err := s.configService.GetBossConfig()
	if err != nil {
		progressCallback(JobProgressMessage{
			Platform:  s.platform,
			Type:      "error",
			Message:   "配置加载失败: " + err.Error(),
			Timestamp: time.Now().UnixMilli(),
		})
		return err
	}

	progressCallback(JobProgressMessage{
		Platform:  s.platform,
		Type:      "info",
		Message:   "配置加载成功",
		Timestamp: time.Now().UnixMilli(),
	})

	progressCallback(JobProgressMessage{
		Platform:  s.platform,
		Type:      "info",
		Message:   "开始投递任务...",
		Timestamp: time.Now().UnixMilli(),
	})

	// 创建Boss实例并执行投递
	bossInstance := s.bossProvider()
	bossInstance.SetPage(page)
	bossInstance.SetConfig(bossConfig)

	// 设置进度回调
	bossInstance.SetProgressCallback(func(message string, current, total int) {
		var msgType string
		if current >= 0 && total > 0 {
			msgType = "progress"
			progressCallback(JobProgressMessage{
				Platform:  s.platform,
				Type:      msgType,
				Message:   message,
				Current:   &current,
				Total:     &total,
				Timestamp: time.Now().UnixMilli(),
			})
		} else {
			msgType = "info"
			progressCallback(JobProgressMessage{
				Platform:  s.platform,
				Type:      msgType,
				Message:   message,
				Timestamp: time.Now().UnixMilli(),
			})
		}
	})

	// 设置停止检查回调
	bossInstance.SetShouldStopCallback(func() bool {
		s.statusMutex.RLock()
		defer s.statusMutex.RUnlock()
		return s.shouldStop
	})

	// 准备阶段
	if err := bossInstance.Prepare(); err != nil {
		progressCallback(JobProgressMessage{
			Platform:  s.platform,
			Type:      "error",
			Message:   "任务准备失败: " + err.Error(),
			Timestamp: time.Now().UnixMilli(),
		})
		return err
	}

	// 执行投递
	deliveredCount := bossInstance.Execute()

	progressCallback(JobProgressMessage{
		Platform:  s.platform,
		Type:      "success",
		Message:   fmt.Sprintf("投递任务完成，共发起聊天数：%d", deliveredCount),
		Timestamp: time.Now().UnixMilli(),
	})

	return nil
}

// StopDelivery 停止投递任务
func (s *BossJobService) StopDelivery() error {
	s.statusMutex.Lock()
	defer s.statusMutex.Unlock()

	if s.running {
		s.shouldStop = true
		log.Println("收到停止Boss投递任务的请求")
	}
	return nil
}

// GetStatus 获取任务状态
func (s *BossJobService) GetStatus() map[string]interface{} {
	s.statusMutex.RLock()
	defer s.statusMutex.RUnlock()

	return map[string]interface{}{
		"platform":   s.platform,
		"isRunning":  s.running,
		"isLoggedIn": s.playwrightManager.IsLoggedIn(s.platform),
	}
}

// GetPlatformName 获取平台名称
func (s *BossJobService) GetPlatformName() string {
	return s.platform
}

// IsRunning 检查是否正在运行
func (s *BossJobService) IsRunning() bool {
	s.statusMutex.RLock()
	defer s.statusMutex.RUnlock()
	return s.running
}

// ShouldStop 检查是否应该停止（供Boss实例调用）
func (s *BossJobService) ShouldStop() bool {
	s.statusMutex.RLock()
	defer s.statusMutex.RUnlock()
	return s.shouldStop
}
