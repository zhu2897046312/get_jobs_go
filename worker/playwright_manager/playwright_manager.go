package playwright_manager

import (
	"context"
	"encoding/json"
	"fmt"
	"get_jobs_go/service"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/playwright-community/playwright-go"
)

// LoginStatusChange 登录状态变化事件
type LoginStatusChange struct {
	Platform   string
	IsLoggedIn bool
	Timestamp  int64
}

// LoginStatusListener 登录状态监听器
type LoginStatusListener func(change LoginStatusChange)

// PlaywrightManager Playwright 管理器
type PlaywrightManager struct {
	playwright *playwright.Playwright
	browser    playwright.Browser
	context    playwright.BrowserContext
	bossPage   playwright.Page

	// 登录状态追踪
	loginStatus          map[string]bool
	loginStatusMutex     sync.RWMutex
	loginStatusListeners []LoginStatusListener
	listenersMutex       sync.RWMutex

	// 监控控制
	bossMonitoringPaused bool
	bossMonitoringMutex  sync.RWMutex

	// 定时检查的可取消上下文
	monitoringCtx    context.Context
	monitoringCancel context.CancelFunc

	// 配置
	defaultTimeout time.Duration
	cdpPort        int

	// 服务依赖
	cookieService service.CookieService

	// 平台URL
	bossURL string
}

// NewPlaywrightManager 创建新的Playwright管理器
func NewPlaywrightManager(cookieService service.CookieService) *PlaywrightManager {
	return &PlaywrightManager{
		loginStatus:          make(map[string]bool),
		loginStatusListeners: make([]LoginStatusListener, 0),
		defaultTimeout:       30 * time.Second,
		cdpPort:              7866,
		bossURL:              "https://www.zhipin.com",
		cookieService:        cookieService,
	}
}

// Init 初始化Playwright实例
func (pm *PlaywrightManager) Init() error {
	if pm.IsInitialized() {
		return nil
	}

	log.Println("========================================")
	log.Println("  初始化浏览器自动化引擎")
	log.Println("========================================")

	var err error

	// 启动Playwright
	pm.playwright, err = playwright.Run()
	if err != nil {
		return fmt.Errorf("启动Playwright引擎失败: %w", err)
	}
	log.Println("✓ Playwright引擎已启动")

	// 创建浏览器实例
	pm.browser, err = pm.playwright.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(false), // 非无头模式
		SlowMo:   playwright.Float(50),   // 放慢操作速度
		Args: []string{
			fmt.Sprintf("--remote-debugging-port=%d", pm.cdpPort),
			"--start-maximized",
		},
	})
	if err != nil {
		return fmt.Errorf("启动Chrome浏览器失败: %w", err)
	}
	log.Printf("✓ Chrome浏览器已启动 (调试端口: %d)", pm.cdpPort)

	// 创建共享的BrowserContext
	pm.context, err = pm.browser.NewContext(playwright.BrowserNewContextOptions{
		Viewport:  &playwright.Size{Width: 0, Height: 0}, // 不设置固定视口
		UserAgent: playwright.String("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/135.0.0.0 Safari/537.36"),
	})
	if err != nil {
		return fmt.Errorf("创建BrowserContext失败: %w", err)
	}
	log.Println("✓ BrowserContext已创建（所有平台共享）")

	// 设置默认超时
	pm.context.SetDefaultTimeout(float64(pm.defaultTimeout.Milliseconds()))

	// 创建Boss直聘页面
	log.Println("开始创建所有平台的Page...")
	pm.bossPage, err = pm.context.NewPage()
	if err != nil {
		return fmt.Errorf("创建Boss Page失败: %w", err)
	}
	pm.bossPage.SetDefaultTimeout(float64(pm.defaultTimeout.Milliseconds()))
	log.Println("✓ Boss Page已创建")

	// 初始化Boss平台
	log.Println("开始初始化Boss直聘平台...")
	if err := pm.setupBossPlatform(); err != nil {
		return fmt.Errorf("初始化Boss平台失败: %w", err)
	}

	// 启动定时登录状态检查循环（可取消）
	pm.monitoringCtx, pm.monitoringCancel = context.WithCancel(context.Background())
	go pm.StartScheduledLoginCheck(pm.monitoringCtx)

	log.Println("✓ 浏览器自动化引擎初始化完成")
	log.Println("========================================")

	return nil
}

// setupBossPlatform 设置Boss直聘平台
func (pm *PlaywrightManager) setupBossPlatform() error {
	// 尝试从数据库加载Boss平台Cookie到上下文
	cookieEntity, err := pm.cookieService.GetCookieByPlatform("boss")
	if err != nil {
		log.Printf("从数据库加载Boss Cookie失败: %v", err)
	} else if cookieEntity != nil && cookieEntity.CookieValue != "" {
		cookies, err := pm.parseCookiesFromString(cookieEntity.CookieValue)
		if err != nil {
			log.Printf("解析Cookie失败: %v", err)
		} else if len(cookies) > 0 {
			if err := pm.context.AddCookies(cookies); err != nil {
				log.Printf("注入Cookie到浏览器上下文失败: %v", err)
			} else {
				log.Printf("已从数据库加载Boss Cookie并注入浏览器上下文，共 %d 条", len(cookies))
			}
		} else {
			log.Println("解析Cookie失败，未能加载任何Cookie")
		}
	} else {
		log.Println("数据库未找到Boss Cookie或值为空，跳过Cookie注入")
	}

	// 导航到Boss直聘首页（带重试机制）
	maxRetries := 3
	navigateSuccess := false

	for attempt := 1; attempt <= maxRetries; attempt++ {
		_, err := pm.bossPage.Goto(pm.bossURL, playwright.PageGotoOptions{
			Timeout:   playwright.Float(60000),
			WaitUntil: playwright.WaitUntilStateDomcontentloaded,
		})

		if err == nil {
			navigateSuccess = true
			break
		}

		// 检查页面是否实际已加载
		url := pm.bossPage.URL()
		if url != "" && strings.Contains(url, "zhipin.com") {
			navigateSuccess = true
			break
		}

		if attempt < maxRetries {
			time.Sleep(2 * time.Second)
		}
	}

	if !navigateSuccess {
		log.Println("Boss直聘页面导航失败")
	}

	// 等待页面网络空闲
	if err := pm.bossPage.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State: playwright.LoadStateNetworkidle,
	}); err != nil {
		log.Printf("等待Boss页面网络空闲失败: %v", err)
	}

	// 初始化登录状态
	isLoggedIn, err := pm.checkIfBossLoggedIn()
	if err != nil {
		log.Printf("检查Boss登录状态失败: %v", err)
		isLoggedIn = false
	}

	pm.SetLoginStatus("boss", isLoggedIn)

	// 设置登录状态监控（事件驱动）
	pm.setupBossLoginMonitoring()

	return nil
}

// checkIfBossLoggedIn 检查Boss是否已登录
func (pm *PlaywrightManager) checkIfBossLoggedIn() (bool, error) {
	// 优先检测明显的登录特征：用户头像或用户名
	// 这里尽量不去主动点击元素来判断登录状态，以免造成误判

	// 1) 用户名/昵称元素
	userName := pm.bossPage.Locator(".user-name, li.nav-figure span.label-text").First()
	if err := userName.WaitFor(playwright.LocatorWaitForOptions{Timeout: playwright.Float(2000)}); err == nil {
		visible, err := userName.IsVisible()
		if err == nil && visible {
			return true, nil
		}
	}

	// 2) 头像 img
	avatar := pm.bossPage.Locator("li.nav-figure img").First()
	if err := avatar.WaitFor(playwright.LocatorWaitForOptions{Timeout: playwright.Float(2000)}); err == nil {
		visible, err := avatar.IsVisible()
		if err == nil && visible {
			return true, nil
		}
	}

	// 3) 检查是否存在登录入口（未登录）
	loginAnchor := pm.bossPage.Locator("li.nav-sign a, .btns").First()
	if err := loginAnchor.WaitFor(playwright.LocatorWaitForOptions{Timeout: playwright.Float(2000)}); err == nil {
		visible, err := loginAnchor.IsVisible()
		if err == nil && visible {
			// 未登录（有登录入口）
			return false, nil
		}
	}

	// 无法明确检测到登录特征时，返回未登录（更保守）
	return false, nil
}

// setupBossLoginMonitoring 设置Boss登录状态监控
func (pm *PlaywrightManager) setupBossLoginMonitoring() {
	// 监听页面导航事件
	pm.bossPage.On("framenavigated", func(frame playwright.Frame) {
		if frame == pm.bossPage.MainFrame() {
			pm.bossMonitoringMutex.RLock()
			paused := pm.bossMonitoringPaused
			pm.bossMonitoringMutex.RUnlock()

			if !paused {
				pm.checkBossLoginStatus()
			}
		}
	})

	log.Println("Boss平台登录状态监控已启用")
}

// checkBossLoginStatus 检查Boss登录状态
func (pm *PlaywrightManager) checkBossLoginStatus() {
	isLoggedIn, err := pm.checkIfBossLoggedIn()
	if err != nil {
		log.Printf("检查Boss登录状态失败: %v", err)
		return
	}

	previousStatus := pm.IsLoggedIn("boss")
	if isLoggedIn && !previousStatus {
		pm.onBossLoginSuccess()
	} else if !isLoggedIn && previousStatus {
		// 登录被检测为失效：更新状态并引导登录
		pm.SetLoginStatus("boss", false)
	}
}

// onBossLoginSuccess Boss登录成功回调
func (pm *PlaywrightManager) onBossLoginSuccess() {
	log.Println("Boss平台登录成功")
	pm.SetLoginStatus("boss", true)
	pm.saveBossCookiesToDatabase("login success")
}

// saveBossCookiesToDatabase 保存Boss Cookie到数据库
func (pm *PlaywrightManager) saveBossCookiesToDatabase(remark string) {
	cookies, err := pm.context.Cookies()
	if err != nil {
		log.Printf("获取Cookie失败: %v", err)
		return
	}

	cookieJSON, err := json.Marshal(cookies)
	if err != nil {
		log.Printf("序列化Cookie失败: %v", err)
		return
	}

	result, err := pm.cookieService.SaveOrUpdateCookie("boss", string(cookieJSON), remark)
	if err != nil {
		log.Printf("保存Boss Cookie失败: %v", err)
	} else if result {
		log.Printf("保存Boss Cookie成功，共 %d 条，remark=%s", len(cookies), remark)
	}
}

// parseCookiesFromString 从JSON字符串解析Cookie列表
func (pm *PlaywrightManager) parseCookiesFromString(cookieJSON string) ([]playwright.OptionalCookie, error) {
	var cookies []playwright.OptionalCookie

	if err := json.Unmarshal([]byte(cookieJSON), &cookies); err != nil {
		return nil, fmt.Errorf("解析Cookie JSON失败: %w", err)
	}

	log.Printf("成功解析Cookie，共 %d 条", len(cookies))
	return cookies, nil
}

// SetLoginStatus 手动设置平台登录状态
func (pm *PlaywrightManager) SetLoginStatus(platform string, isLoggedIn bool) {
	pm.loginStatusMutex.Lock()
	defer pm.loginStatusMutex.Unlock()

	previousStatus := pm.loginStatus[platform]
	if previousStatus != isLoggedIn {
		pm.loginStatus[platform] = isLoggedIn

		// Boss平台：在设置未登录状态时，引导到登录页
		if platform == "boss" && !isLoggedIn {
			pm.guideBossToLogin()
		}

		// 通知监听器
		change := LoginStatusChange{
			Platform:   platform,
			IsLoggedIn: isLoggedIn,
			Timestamp:  time.Now().UnixMilli(),
		}
		pm.notifyLoginStatusListeners(change)
	}
}

// guideBossToLogin 引导Boss到登录页
func (pm *PlaywrightManager) guideBossToLogin() {
	if pm.bossPage == nil {
		return
	}

	currentURL := pm.bossPage.URL()
	if currentURL == "" {
		log.Printf("获取当前URL失败")
		return
	}

	// 避免重复导航
	if !strings.Contains(currentURL, "/web/user/") {
		_, err := pm.bossPage.Goto(pm.bossURL+"/web/user/?ka=header-login", playwright.PageGotoOptions{
			Timeout: playwright.Float(60000),
		})
		if err != nil {
			log.Printf("导航到Boss登录页失败: %v", err)
			return
		}
		time.Sleep(800 * time.Millisecond)
	}

	// 尝试切换到二维码登录
	pm.switchToBossQRLogin()
}

// switchToBossQRLogin 切换到Boss二维码登录
func (pm *PlaywrightManager) switchToBossQRLogin() {
	// 优先使用新版选择器
	qrSwitch := pm.bossPage.Locator(".btn-sign-switch.ewm-switch").First()
	// 尝试等待元素出现，超时会返回错误
	if err := qrSwitch.WaitFor(playwright.LocatorWaitForOptions{Timeout: playwright.Float(3000)}); err == nil {
		visible, err := qrSwitch.IsVisible()
		if err == nil && visible {
			if err := qrSwitch.Click(); err == nil {
				log.Println("已点击包含文本的二维码登录切换提示（APP扫码登录）")
				return
			}
		}
	}

	// 兜底：按文本匹配
	tip := pm.bossPage.Locator("text=APP扫码登录").First()
	if err := tip.WaitFor(playwright.LocatorWaitForOptions{Timeout: playwright.Float(3000)}); err == nil {
		visible, err := tip.IsVisible()
		if err == nil && visible {
			if err := tip.Click(); err == nil {
				log.Println("已点击包含文本的二维码登录切换提示（APP扫码登录）")
				return
			}
		}
	}

	// 兼容旧版选择器
	legacy := pm.bossPage.Locator("li.sign-switch-tip").First()
	if err := legacy.WaitFor(playwright.LocatorWaitForOptions{Timeout: playwright.Float(3000)}); err == nil {
		visible, err := legacy.IsVisible()
		if err == nil && visible {
			if err := legacy.Click(); err == nil {
				log.Println("已点击包含文本的二维码登录切换提示（APP扫码登录）")
				return
			}
		}
	}

	log.Println("未找到二维码登录切换按钮，保持当前登录页")
}

// AddLoginStatusListener 注册登录状态监听器
func (pm *PlaywrightManager) AddLoginStatusListener(listener LoginStatusListener) {
	pm.listenersMutex.Lock()
	defer pm.listenersMutex.Unlock()
	pm.loginStatusListeners = append(pm.loginStatusListeners, listener)
}

// RemoveLoginStatusListener 移除登录状态监听器
func (pm *PlaywrightManager) RemoveLoginStatusListener(listener LoginStatusListener) {
	pm.listenersMutex.Lock()
	defer pm.listenersMutex.Unlock()

	for i, l := range pm.loginStatusListeners {
		// 通过函数指针比较不是可靠方法，但保留原有逻辑
		if &l == &listener {
			pm.loginStatusListeners = append(pm.loginStatusListeners[:i], pm.loginStatusListeners[i+1:]...)
			break
		}
	}
}

// notifyLoginStatusListeners 通知登录状态监听器
func (pm *PlaywrightManager) notifyLoginStatusListeners(change LoginStatusChange) {
	pm.listenersMutex.RLock()
	defer pm.listenersMutex.RUnlock()

	for _, listener := range pm.loginStatusListeners {
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("通知登录状态监听器失败: %v", r)
				}
			}()
			listener(change)
		}()
	}
}

// IsLoggedIn 获取平台登录状态
func (pm *PlaywrightManager) IsLoggedIn(platform string) bool {
	pm.loginStatusMutex.RLock()
	defer pm.loginStatusMutex.RUnlock()
	return pm.loginStatus[platform]
}

// IsInitialized 检查Playwright是否已初始化
func (pm *PlaywrightManager) IsInitialized() bool {
	return pm.playwright != nil && pm.browser != nil && pm.bossPage != nil
}

// GetCDPPort 获取CDP端口号
func (pm *PlaywrightManager) GetCDPPort() int {
	return pm.cdpPort
}

// GetBossPage 获取Boss页面
func (pm *PlaywrightManager) GetBossPage() playwright.Page {
	return pm.bossPage
}

// GetContext 获取浏览器上下文
func (pm *PlaywrightManager) GetContext() playwright.BrowserContext {
	return pm.context
}

// SaveBossCookiesToDb 主动保存Boss Cookie到数据库
func (pm *PlaywrightManager) SaveBossCookiesToDb(remark string) {
	pm.saveBossCookiesToDatabase(remark)
}

// ClearBossCookies 清理Boss上下文中的Cookie
func (pm *PlaywrightManager) ClearBossCookies() error {
	if pm.context != nil {
		if err := pm.context.ClearCookies(); err != nil {
			return fmt.Errorf("清理共享上下文Cookie失败: %w", err)
		}
		log.Println("已清理共享上下文中的所有Cookie")
	} else {
		log.Println("共享上下文不存在，无法清理Cookie")
	}
	return nil
}

// PauseBossMonitoring 暂停Boss页面的后台登录监控
func (pm *PlaywrightManager) PauseBossMonitoring() {
	pm.bossMonitoringMutex.Lock()
	defer pm.bossMonitoringMutex.Unlock()
	pm.bossMonitoringPaused = true
	log.Println("Boss登录监控已暂停")
}

// ResumeBossMonitoring 恢复Boss页面的后台登录监控
func (pm *PlaywrightManager) ResumeBossMonitoring() {
	pm.bossMonitoringMutex.Lock()
	defer pm.bossMonitoringMutex.Unlock()
	pm.bossMonitoringPaused = false
	log.Println("Boss登录监控已恢复")
}

// StartScheduledLoginCheck 启动定时登录状态检查
func (pm *PlaywrightManager) StartScheduledLoginCheck(ctx context.Context) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("停止定时登录状态检查")
			return
		case <-ticker.C:
			pm.scheduledLoginCheck()
		}
	}
}

// scheduledLoginCheck 定时检查登录状态
func (pm *PlaywrightManager) scheduledLoginCheck() {
	pm.bossMonitoringMutex.RLock()
	bossPaused := pm.bossMonitoringPaused
	pm.bossMonitoringMutex.RUnlock()

	if !bossPaused {
		pm.checkBossLoginStatus()
	}
}

// Close 关闭Playwright实例
func (pm *PlaywrightManager) Close() error {
	log.Println("开始关闭Playwright管理器...")

	// 先取消后台监控
	if pm.monitoringCancel != nil {
		pm.monitoringCancel()
	}

	var errors []string

	if pm.bossPage != nil {
		if err := pm.bossPage.Close(); err != nil {
			errors = append(errors, fmt.Sprintf("关闭Boss直聘页面失败: %v", err))
		} else {
			log.Println("Boss直聘页面已关闭")
		}
	}

	if pm.context != nil {
		if err := pm.context.Close(); err != nil {
			errors = append(errors, fmt.Sprintf("关闭共享BrowserContext失败: %v", err))
		} else {
			log.Println("共享BrowserContext已关闭")
		}
	}

	if pm.browser != nil {
		if err := pm.browser.Close(); err != nil {
			errors = append(errors, fmt.Sprintf("关闭浏览器失败: %v", err))
		} else {
			log.Println("浏览器已关闭")
		}
	}

	if pm.playwright != nil {
		if err := pm.playwright.Stop(); err != nil {
			errors = append(errors, fmt.Sprintf("关闭Playwright实例失败: %v", err))
		} else {
			log.Println("Playwright实例已关闭")
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("关闭Playwright管理器时发生错误: %s", strings.Join(errors, "; "))
	}

	log.Println("Playwright管理器关闭完成！")
	return nil
}
