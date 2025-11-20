package main

import (
	"context"
	"fmt"
	"get_jobs_go/repository"
	"get_jobs_go/service"
	"get_jobs_go/worker/boss"
	"get_jobs_go/worker/playwright_manager"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type Application struct {
	db                *gorm.DB
	configService     *service.ConfigService
	cookieService     service.CookieService
	playwrightManager *playwright_manager.PlaywrightManager
	bossJobService    *boss.BossJobService
}

// NewApplication 创建新的应用程序实例
func NewApplication() *Application {
	return &Application{}
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

	// 自动迁移数据库表
	if err := db.AutoMigrate(
	// 这里应该添加需要迁移的模型
	// &model.Config{},
	// &model.Cookie{},
	// ... 其他模型
	); err != nil {
		return fmt.Errorf("数据库迁移失败: %v", err)
	}

	log.Println("✓ 数据库表迁移完成")
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
	// 初始化Playwright管理器
	if err := app.playwrightManager.Init(); err != nil {
		return fmt.Errorf("Playwright管理器初始化失败: %v", err)
	}
	log.Println("✓ 所有服务初始化完成")
	return nil
}

// Start 启动应用程序
func (app *Application) Start() error {
	log.Println("========================================")
	log.Println("   启动求职信息采集系统")
	log.Println("========================================")

	// 启动Boss直聘任务服务
	if app.bossJobService != nil {
		log.Println("启动Boss直聘数据采集任务...")
		go func() {
			// 使用默认的进度回调函数
			progressCallback := func(message boss.JobProgressMessage) {

				log.Printf("[%s][%s] %s", message.Platform, message.Type, message.Message)
				if message.Current != nil && message.Total != nil {
					log.Printf("进度: %d/%d", *message.Current, *message.Total)
				}
			}

			if err := app.bossJobService.ExecuteDelivery(progressCallback); err != nil {
				log.Printf("Boss直聘任务执行失败: %v", err)
			}
		}()
	} else {
		log.Println("⚠️ Boss直聘任务服务未初始化")
	}

	log.Println("✓ 应用程序已启动")
	return nil
}

// Stop 停止应用程序
func (app *Application) Stop() error {
	log.Println("========================================")
	log.Println("   停止应用程序")
	log.Println("========================================")

	// 停止Boss直聘任务服务
	if app.bossJobService != nil {
		log.Println("停止Boss直聘数据采集任务...")
		app.bossJobService.StopDelivery()
	}

	// 关闭Playwright管理器
	if app.playwrightManager != nil {
		log.Println("关闭Playwright管理器...")
		app.playwrightManager.Close()
	}

	// 关闭数据库连接
	if app.db != nil {
		log.Println("关闭数据库连接...")
		if sqlDB, err := app.db.DB(); err == nil {
			sqlDB.Close()
		}
	}

	log.Println("✓ 应用程序已安全停止")
	return nil
}

// waitForShutdown 等待关闭信号
func (app *Application) waitForShutdown() {
	// 创建信号监听通道
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	// 等待信号
	sig := <-sigChan
	log.Printf("接收到信号: %v，开始优雅关闭...", sig)

	// 创建带超时的上下文
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 在单独的goroutine中执行关闭操作
	done := make(chan struct{})
	go func() {
		app.Stop()
		close(done)
	}()

	// 等待关闭完成或超时
	select {
	case <-done:
		log.Println("✓ 应用程序优雅关闭完成")
	case <-ctx.Done():
		log.Println("⚠️ 关闭超时，强制退出")
	}
}

func main() {
	// 设置日志格式
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("🚀 启动求职信息采集系统...")

	// 创建应用程序实例
	app := NewApplication()

	// 初始化服务
	if err := app.InitServices(); err != nil {
		log.Fatalf("❌ 服务初始化失败: %v", err)
	}

	// 启动应用程序
	if err := app.Start(); err != nil {
		log.Fatalf("❌ 应用程序启动失败: %v", err)
	}

	// 等待关闭信号
	app.waitForShutdown()

	log.Println("👋 应用程序已退出")
}
