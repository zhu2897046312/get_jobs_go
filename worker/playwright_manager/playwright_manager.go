// worker/playwright_manager/playwright_manager.go
package playwright_manager

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"get_jobs_go/service"

	"github.com/playwright-community/playwright-go"
)

// LoginStatusChange 登录状态变化
type LoginStatusChange struct {
	Platform   string `json:"platform"`
	IsLoggedIn bool   `json:"isLoggedIn"`
	Timestamp  int64  `json:"timestamp"`
}

// LoginStatusListener 登录状态监听器
type LoginStatusListener func(change LoginStatusChange)

// PlaywrightManager 浏览器自动化管理器
type PlaywrightManager struct {
	playwright *playwright.Playwright
	browser    playwright.Browser
	context    playwright.BrowserContext

	// 平台页面
	bossPage playwright.Page

	// 登录状态
	loginStatus      map[string]bool
	loginStatusMutex sync.RWMutex

	// 监听器
	listeners      []LoginStatusListener
	listenersMutex sync.RWMutex

	// 监控控制
	bossMonitoringPaused bool

	// 服务依赖
	cookieService service.CookieService
	configService *service.ConfigService
}

// NewPlaywrightManager 创建新的Playwright管理器
func NewPlaywrightManager(
	cookieService service.CookieService,
	configService *service.ConfigService,
) *PlaywrightManager {
	return &PlaywrightManager{
		loginStatus:   make(map[string]bool),
		listeners:     make([]LoginStatusListener, 0),
		cookieService: cookieService,
		configService: configService,
	}
}

// Init 初始化浏览器实例
func (pm *PlaywrightManager) Init() error {
	log.Println("========================================")
	log.Println("  初始化浏览器自动化引擎")
	log.Println("========================================")

	// 启动 Playwright
	pw, err := playwright.Run()
	if err != nil {
		return fmt.Errorf("启动Playwright失败: %v", err)
	}
	pm.playwright = pw

	// 启动浏览器
	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(false),
		Args: []string{
			"--remote-debugging-port=7866",
			"--start-maximized",
		},
	})
	if err != nil {
		return fmt.Errorf("启动浏览器失败: %v", err)
	}
	pm.browser = browser

	// 创建浏览器上下文
	context, err := browser.NewContext()
	if err != nil {
		return fmt.Errorf("创建浏览器上下文失败: %v", err)
	}
	pm.context = context

	// 创建Boss页面
	if err := pm.createBossPage(); err != nil {
		return fmt.Errorf("创建Boss页面失败: %v", err)
	}

	// 初始化Boss平台
	if err := pm.setupBossPlatform(); err != nil {
		return fmt.Errorf("初始化Boss平台失败: %v", err)
	}

	log.Println("✓ 浏览器自动化引擎初始化完成")
	log.Println("========================================")

	return nil
}

// createBossPage 创建Boss页面
func (pm *PlaywrightManager) createBossPage() error {
	page, err := pm.context.NewPage()
	if err != nil {
		return fmt.Errorf("创建Boss页面失败: %v", err)
	}
	pm.bossPage = page
	page.SetDefaultTimeout(30000) // 30秒超时
	log.Printf("✓ Boss Page已创建")
	return nil
}

// setupBossPlatform 设置Boss直聘平台
func (pm *PlaywrightManager) setupBossPlatform() error {
	// 加载Cookie
	if err := pm.loadCookiesForPlatform("boss"); err != nil {
		log.Printf("加载Boss Cookie失败: %v", err)
	}

	// 导航到Boss首页
	if _, err := pm.bossPage.Goto("https://www.zhipin.com", playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
		Timeout:   playwright.Float(60000),
	}); err != nil {
		log.Printf("Boss页面导航失败: %v", err)
	}

	// 检查登录状态
	isLoggedIn := pm.checkBossLoginStatus()
	pm.setLoginStatus("boss", isLoggedIn)

	// 设置登录监控
	pm.setupLoginMonitoring(pm.bossPage, "boss")

	return nil
}

// loadCookiesForPlatform 为平台加载Cookie
func (pm *PlaywrightManager) loadCookiesForPlatform(platform string) error {
	cookieEntity, err := pm.cookieService.GetCookieByPlatform(platform)
	if err != nil {
		return fmt.Errorf("获取%s Cookie失败: %v", platform, err)
	}

	if cookieEntity == nil || cookieEntity.CookieValue == "" {
		log.Printf("数据库未找到%s Cookie，跳过加载", platform)
		return nil
	}

	// 解析Cookie
	var cookies []playwright.OptionalCookie
	if err := json.Unmarshal([]byte(cookieEntity.CookieValue), &cookies); err != nil {
		return fmt.Errorf("解析%s Cookie失败: %v", platform, err)
	}

	// 添加到浏览器上下文 - 修复参数传递方式
	if len(cookies) > 0 {
		if err := pm.context.AddCookies(cookies); err != nil {
			return fmt.Errorf("添加%s Cookie到浏览器失败: %v", platform, err)
		}
		log.Printf("已从数据库加载%s Cookie，共%d条", platform, len(cookies))
	}

	return nil
}

// checkBossLoginStatus 检查Boss登录状态
func (pm *PlaywrightManager) checkBossLoginStatus() bool {
	// 检查用户标签是否存在 - 修复返回值接收
	userLabel := pm.bossPage.Locator("li.nav-figure span.label-text").First()
	if visible, _ := userLabel.IsVisible(); visible {
		return true
	}

	// 检查导航头像 - 修复返回值接收
	navFigure := pm.bossPage.Locator("li.nav-figure").First()
	if visible, _ := navFigure.IsVisible(); visible {
		return true
	}

	// 检查登录入口 - 修复返回值接收
	loginAnchor := pm.bossPage.Locator("li.nav-sign a, .btns").First()
	if visible, _ := loginAnchor.IsVisible(); visible {
		if text, _ := loginAnchor.TextContent(); text != "" && strings.Contains(text, "登录") {
			return false
		}
	}

	return false
}

// setupLoginMonitoring 设置登录状态监控
func (pm *PlaywrightManager) setupLoginMonitoring(page playwright.Page, platform string) {
	page.On("framenavigated", func(frame playwright.Frame) {
		if frame == page.MainFrame() {
			if !pm.isMonitoringPaused(platform) {
				pm.checkPlatformLoginStatus(page, platform)
			}
		}
	})
}

// checkPlatformLoginStatus 检查平台登录状态
func (pm *PlaywrightManager) checkPlatformLoginStatus(page playwright.Page, platform string) {
	var isLoggedIn bool
	
	switch platform {
	case "boss":
		isLoggedIn = pm.checkBossLoginStatus()
	}

	previousStatus := pm.getLoginStatus(platform)
	if isLoggedIn && !previousStatus {
		pm.onLoginSuccess(platform)
	}
	
	pm.setLoginStatus(platform, isLoggedIn)
}

// onLoginSuccess 登录成功处理
func (pm *PlaywrightManager) onLoginSuccess(platform string) {
	log.Printf("%s平台登录成功", platform)
	pm.saveCookiesForPlatform(platform, "login success")
}

// saveCookiesForPlatform 保存平台Cookie
func (pm *PlaywrightManager) saveCookiesForPlatform(platform, remark string) {
	cookies, err := pm.context.Cookies()
	if err != nil {
		log.Printf("获取%s Cookie失败: %v", platform, err)
		return
	}

	cookieJSON, err := json.Marshal(cookies)
	if err != nil {
		log.Printf("序列化%s Cookie失败: %v", platform, err)
		return
	}

	// 修复返回值接收
	success, err := pm.cookieService.SaveOrUpdateCookie(platform, string(cookieJSON), remark)
	if err != nil {
		log.Printf("保存%s Cookie失败: %v", platform, err)
	} else if success {
		log.Printf("保存%s Cookie成功，共%d条", platform, len(cookies))
	} else {
		log.Printf("保存%s Cookie失败，未知原因", platform)
	}
}

// 状态管理方法
func (pm *PlaywrightManager) setLoginStatus(platform string, isLoggedIn bool) {
	pm.loginStatusMutex.Lock()
	defer pm.loginStatusMutex.Unlock()

	previousStatus := pm.loginStatus[platform]
	if previousStatus != isLoggedIn {
		pm.loginStatus[platform] = isLoggedIn
		
		// 通知监听器
		change := LoginStatusChange{
			Platform:   platform,
			IsLoggedIn: isLoggedIn,
			Timestamp:  time.Now().UnixMilli(),
		}
		pm.notifyListeners(change)

		log.Printf("登录状态更新: platform=%s, isLoggedIn=%v", platform, isLoggedIn)
	}
}

func (pm *PlaywrightManager) getLoginStatus(platform string) bool {
	pm.loginStatusMutex.RLock()
	defer pm.loginStatusMutex.RUnlock()
	return pm.loginStatus[platform]
}

func (pm *PlaywrightManager) isMonitoringPaused(platform string) bool {
	switch platform {
	case "boss":
		return pm.bossMonitoringPaused
	default:
		return false
	}
}

// 监听器管理
func (pm *PlaywrightManager) AddLoginStatusListener(listener LoginStatusListener) {
	pm.listenersMutex.Lock()
	defer pm.listenersMutex.Unlock()
	pm.listeners = append(pm.listeners, listener)
}

func (pm *PlaywrightManager) RemoveLoginStatusListener(listener LoginStatusListener) {
	pm.listenersMutex.Lock()
	defer pm.listenersMutex.Unlock()
	
	for i, l := range pm.listeners {
		if &l == &listener {
			pm.listeners = append(pm.listeners[:i], pm.listeners[i+1:]...)
			break
		}
	}
}

func (pm *PlaywrightManager) notifyListeners(change LoginStatusChange) {
	pm.listenersMutex.RLock()
	defer pm.listenersMutex.RUnlock()
	
	for _, listener := range pm.listeners {
		go func(l LoginStatusListener) {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("通知登录状态监听器时发生panic: %v", r)
				}
			}()
			l(change)
		}(listener)
	}
}

// 页面获取方法
func (pm *PlaywrightManager) GetBossPage() playwright.Page {
	return pm.bossPage
}

// 登录状态检查
func (pm *PlaywrightManager) IsLoggedIn(platform string) bool {
	return pm.getLoginStatus(platform)
}

// 监控控制方法
func (pm *PlaywrightManager) PauseBossMonitoring() {
	pm.bossMonitoringPaused = true
	log.Println("Boss登录监控已暂停")
}

func (pm *PlaywrightManager) ResumeBossMonitoring() {
	pm.bossMonitoringPaused = false
	log.Println("Boss登录监控已恢复")
}

// 清理资源
func (pm *PlaywrightManager) Close() {
	log.Println("开始关闭Playwright管理器...")

	if pm.bossPage != nil {
		pm.bossPage.Close()
		log.Println("Boss页面已关闭")
	}
	if pm.context != nil {
		pm.context.Close()
		log.Println("浏览器上下文已关闭")
	}
	if pm.browser != nil {
		pm.browser.Close()
		log.Println("浏览器已关闭")
	}
	if pm.playwright != nil {
		pm.playwright.Stop()
		log.Println("Playwright实例已关闭")
	}

	log.Println("Playwright管理器关闭完成")
}