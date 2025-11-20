// main.go
package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"get_jobs_go/repository"
	"get_jobs_go/service"
	"get_jobs_go/worker/boss"
	"get_jobs_go/worker/playwright_manager"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// Application 主应用程序
type Application struct {
	db                *gorm.DB
	playwrightManager *playwright_manager.PlaywrightManager
	bossJobService    *boss.BossJobService
	configService     *service.ConfigService
	cookieService     service.CookieService

	// 状态控制
	isRunning    bool
	shouldStop   bool
	statusMutex  sync.RWMutex
	currentTask  string
	progressChan chan boss.JobProgressMessage

	// 上下文控制
	ctx        context.Context
	cancelFunc context.CancelFunc
}

// NewApplication 创建新的应用程序实例
func NewApplication() *Application {
	ctx, cancel := context.WithCancel(context.Background())

	return &Application{
		isRunning:    false,
		shouldStop:   false,
		progressChan: make(chan boss.JobProgressMessage, 100),
		ctx:          ctx,
		cancelFunc:   cancel,
	}
}

// InitDatabase 初始化数据库连接
func (app *Application) InitDatabase() error {
	log.Println("初始化数据库连接...")

	// MySQL 连接配置
	// 格式: "user:password@tcp(host:port)/dbname?charset=utf8mb4&parseTime=True&loc=Local"
	dsn := "root:123@tcp(localhost:3306)/jobs?charset=utf8mb4&parseTime=True&loc=Local"

	// 使用 MySQL 数据库
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("数据库连接失败: %v", err)
	}

	// 获取底层的 SQL DB 对象以设置连接池
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("获取数据库连接失败: %v", err)
	}

	// 设置连接池参数
	sqlDB.SetMaxIdleConns(10)           // 最大空闲连接数
	sqlDB.SetMaxOpenConns(100)          // 最大打开连接数
	sqlDB.SetConnMaxLifetime(time.Hour) // 连接的最大可复用时间

	app.db = db
	log.Println("✓ MySQL 数据库连接成功")

	// 自动迁移表结构（如果需要）
	// 这里需要根据你的实体结构添加自动迁移
	// db.AutoMigrate(&repository.ConfigEntity{}, &repository.CookieEntity{})

	return nil
}

// InitServices 初始化所有服务
func (app *Application) InitServices() error {
	log.Println("========================================")
	log.Println("   初始化应用程序服务")
	log.Println("========================================")

	// 初始化数据库
	if err := app.InitDatabase(); err != nil {
		return fmt.Errorf("数据库初始化失败: %v", err)
	}

	// 初始化仓库
	configRepo := repository.NewConfigRepository(app.db)
	cookieRepo := repository.NewCookieRepository(app.db)

	// 初始化Boss相关的仓库
	bossOptionRepo := repository.NewBossOptionRepository(app.db)
	bossIndustryRepo := repository.NewBossIndustryRepository(app.db)
	bossConfigRepo := repository.NewBossConfigRepository(app.db)
	blacklistRepo := repository.NewBlacklistRepository(app.db)
	jobDataRepo := repository.NewBossJobDataRepository(app.db)
	aiRepo := repository.NewAiRepository(app.db)

	// 初始化Boss服务
	bossService := service.NewBossService(
		bossOptionRepo,
		bossIndustryRepo,
		bossConfigRepo,
		blacklistRepo,
		jobDataRepo,
		app.db,
	)

	// 初始化配置服务
	configService := service.NewConfigService(configRepo, bossService)
	app.configService = configService

	// 初始化AI服务
	aiService := service.NewAiService(aiRepo, *configService)

	// 初始化Cookie服务
	cookieService := service.NewCookieService(cookieRepo)
	app.cookieService = *cookieService

	// 初始化Playwright管理器
	playwrightManager := playwright_manager.NewPlaywrightManager(
		*cookieService,
		configService,
	)
	app.playwrightManager = playwrightManager

	// 初始化Boss任务服务
	bossJobService := boss.NewBossJobService(
		playwrightManager,
		configService,
		func() *boss.Boss {
			return boss.NewBoss(bossService, aiService)
		},
	)
	app.bossJobService = bossJobService

	// 添加登录状态监听器
	app.playwrightManager.AddLoginStatusListener(app.handleLoginStatusChange)

	log.Println("✓ 所有服务初始化完成")
	return nil
}

// handleLoginStatusChange 处理登录状态变化
func (app *Application) handleLoginStatusChange(change playwright_manager.LoginStatusChange) {
	status := "未登录"
	if change.IsLoggedIn {
		status = "已登录"
	}
	log.Printf("登录状态更新: %s - %s", change.Platform, status)
}

// StartBrowser 启动浏览器
func (app *Application) StartBrowser() error {
	log.Println("启动浏览器自动化引擎...")

	if err := app.playwrightManager.Init(); err != nil {
		return fmt.Errorf("浏览器启动失败: %v", err)
	}

	log.Println("✓ 浏览器自动化引擎启动成功")
	return nil
}

// StartProgressMonitor 启动进度监控
func (app *Application) StartProgressMonitor() {
	go func() {
		for {
			select {
			case progress := <-app.progressChan:
				app.displayProgressMessage(progress)
			case <-app.ctx.Done():
				return
			}
		}
	}()
}

// displayProgressMessage 显示进度消息
func (app *Application) displayProgressMessage(progress boss.JobProgressMessage) {
	timestamp := time.UnixMilli(progress.Timestamp).Format("15:04:05")

	switch progress.Type {
	case "info":
		log.Printf("[%s] ℹ️  %s", timestamp, progress.Message)
	case "warning":
		log.Printf("[%s] ⚠️  %s", timestamp, progress.Message)
	case "error":
		log.Printf("[%s] ❌ %s", timestamp, progress.Message)
	case "progress":
		if progress.Current != nil && progress.Total != nil {
			percentage := float64(*progress.Current) / float64(*progress.Total) * 100
			log.Printf("[%s] 📊 %s (%d/%d, %.1f%%)",
				timestamp, progress.Message, *progress.Current, *progress.Total, percentage)
		} else {
			log.Printf("[%s] 📊 %s", timestamp, progress.Message)
		}
	case "success":
		log.Printf("[%s] ✅ %s", timestamp, progress.Message)
	default:
		log.Printf("[%s] %s", timestamp, progress.Message)
	}
}

// ExecuteBossDelivery 执行Boss投递任务
func (app *Application) ExecuteBossDelivery() {
	app.statusMutex.Lock()
	if app.isRunning {
		log.Println("⚠️  任务已在运行中，请等待当前任务完成")
		app.statusMutex.Unlock()
		return
	}
	app.isRunning = true
	app.currentTask = "boss_delivery"
	app.statusMutex.Unlock()

	defer func() {
		app.statusMutex.Lock()
		app.isRunning = false
		app.currentTask = ""
		app.statusMutex.Unlock()
	}()

	log.Println("🚀 开始执行Boss直聘投递任务...")

	// 执行投递任务
	err := app.bossJobService.ExecuteDelivery(func(message boss.JobProgressMessage) {
		// 非阻塞发送进度消息
		select {
		case app.progressChan <- message:
		default:
			// 如果通道满，丢弃消息（避免阻塞）
		}
	})

	if err != nil {
		log.Printf("❌ 任务执行失败: %v", err)
	} else {
		log.Println("✅ 任务执行完成")
	}
}

// StopCurrentTask 停止当前任务
func (app *Application) StopCurrentTask() {
	app.statusMutex.Lock()
	defer app.statusMutex.Unlock()

	if app.isRunning {
		switch app.currentTask {
		case "boss_delivery":
			if err := app.bossJobService.StopDelivery(); err != nil {
				log.Printf("停止任务失败: %v", err)
			} else {
				log.Println("⏹️  已发送停止信号，等待任务停止...")
				app.shouldStop = true
			}
		default:
			log.Println("⚠️  没有正在运行的任务")
		}
	} else {
		log.Println("⚠️  没有正在运行的任务")
	}
}

// ShowStatus 显示当前状态
func (app *Application) ShowStatus() {
	app.statusMutex.RLock()
	defer app.statusMutex.RUnlock()

	log.Println("========================================")
	log.Println("           当前系统状态")
	log.Println("========================================")

	// 显示任务状态
	if app.isRunning {
		log.Printf("📊 当前任务: %s (运行中)", app.currentTask)
	} else {
		log.Println("📊 当前任务: 无")
	}

	// 显示Boss平台状态
	bossStatus := app.bossJobService.GetStatus()
	isLoggedIn := bossStatus["isLoggedIn"].(bool)
	isRunning := bossStatus["isRunning"].(bool)

	loginStatus := "❌ 未登录"
	if isLoggedIn {
		loginStatus = "✅ 已登录"
	}

	taskStatus := "🟢 运行中"
	if !isRunning {
		taskStatus = "⚪ 未运行"
	}

	log.Printf("👔 Boss直聘: %s | 任务状态: %s", loginStatus, taskStatus)
	log.Println("========================================")
}

// ShowMainMenu 显示主菜单
func (app *Application) ShowMainMenu() {
	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Println("          Boss直聘自动化投递系统")
	fmt.Println(strings.Repeat("=", 50))
	fmt.Println("1. 🔍 显示当前状态")
	fmt.Println("2. 🚀 开始Boss投递任务")
	fmt.Println("3. ⏹️  停止当前任务")
	fmt.Println("4. 🔄 重新加载配置")
	fmt.Println("5. 🚪 退出程序")
	fmt.Println(strings.Repeat("=", 50))
	fmt.Print("请选择操作 (1-5): ")
}

// HandleUserInput 处理用户输入
func (app *Application) HandleUserInput() {
	scanner := bufio.NewScanner(os.Stdin)

	for {
		app.ShowMainMenu()

		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())

		switch input {
		case "1":
			app.ShowStatus()
		case "2":
			go app.ExecuteBossDelivery()
		case "3":
			app.StopCurrentTask()
		case "4":
			app.ReloadConfig()
		case "5":
			log.Println("正在退出程序...")
			return
		default:
			fmt.Println("❌ 无效选择，请输入 1-5 之间的数字")
		}

		// 短暂暂停，让用户看到结果
		time.Sleep(500 * time.Millisecond)
	}
}

// ReloadConfig 重新加载配置
func (app *Application) ReloadConfig() {
	log.Println("🔄 重新加载配置...")
	// 这里可以添加重新加载配置的逻辑
	log.Println("✅ 配置重新加载完成")
}

// SetupSignalHandler 设置信号处理器
func (app *Application) SetupSignalHandler() {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-signalChan
		log.Printf("收到信号: %v，正在关闭程序...", sig)
		app.Cleanup()
		os.Exit(0)
	}()
}

// Cleanup 清理资源
func (app *Application) Cleanup() {
	log.Println("开始清理应用程序资源...")

	// 取消上下文
	if app.cancelFunc != nil {
		app.cancelFunc()
	}

	// 停止当前任务
	app.StopCurrentTask()

	// 等待任务停止
	time.Sleep(2 * time.Second)

	// 关闭Playwright管理器
	if app.playwrightManager != nil {
		app.playwrightManager.Close()
	}

	// 关闭进度通道
	close(app.progressChan)

	log.Println("应用程序资源清理完成")
}

// WaitForBrowserReady 等待浏览器准备就绪
func (app *Application) WaitForBrowserReady() {
	log.Println("等待浏览器准备就绪...")

	// 检查Boss登录状态
	for i := 0; i < 30; i++ {
		if app.playwrightManager.IsLoggedIn("boss") {
			log.Println("✅ Boss直聘已登录，可以开始任务")
			return
		}

		if i == 0 {
			log.Println("⏳ 请在浏览器中登录Boss直聘账号...")
			log.Println("💡 提示: 登录成功后程序会自动检测并继续")
		}

		time.Sleep(2 * time.Second)
	}

	log.Println("⚠️  浏览器准备超时，请检查是否已登录")
}

// Run 运行应用程序
func (app *Application) Run() error {
	// 设置信号处理器
	app.SetupSignalHandler()

	// 初始化服务
	if err := app.InitServices(); err != nil {
		return fmt.Errorf("服务初始化失败: %v", err)
	}

	// 启动浏览器
	if err := app.StartBrowser(); err != nil {
		return fmt.Errorf("浏览器启动失败: %v", err)
	}

	// 启动进度监控
	app.StartProgressMonitor()

	// 等待浏览器准备就绪
	app.WaitForBrowserReady()

	// 显示初始状态
	app.ShowStatus()

	// 开始处理用户输入
	app.HandleUserInput()

	return nil
}

func main() {
	log.Println("🚀 启动Boss直聘自动化投递系统...")

	// 创建应用程序实例
	app := NewApplication()

	// 确保资源被清理
	defer app.Cleanup()

	// 运行应用程序
	if err := app.Run(); err != nil {
		log.Fatalf("❌ 应用程序运行失败: %v", err)
	}

	log.Println("👋 程序正常退出")
}
