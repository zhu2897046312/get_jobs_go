package playwright_manager

import (
	"encoding/json"
	"fmt"
	"get_jobs_go/service"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/playwright-community/playwright-go"
	log "github.com/sirupsen/logrus"
)

const (
	DEFAULR_TIMEOUT = 30 * time.Second
	BOSS_URL        = "https://www.zhipin.com"
)

type LoginStatusChange struct {
	Platform   string
	IsLoggedIn bool
	Timestamp  int64
}

// Listener 类型（等价 Java Consumer<LoginStatusChange>）
type LoginStatusListener func(LoginStatusChange)

// Copy-on-write listener 列表（等价 Java CopyOnWriteArrayList）
type LoginStatusListenerList struct {
	value atomic.Value // 保存 []LoginStatusListener
}

func NewLoginStatusListenerList() *LoginStatusListenerList {
	l := &LoginStatusListenerList{}
	l.value.Store([]LoginStatusListener{}) // 初始化空列表
	return l
}

// 添加 listener（等价 Java list.add）
func (l *LoginStatusListenerList) Add(fn LoginStatusListener) {
	oldList := l.value.Load().([]LoginStatusListener)
	newList := append(append([]LoginStatusListener{}, oldList...), fn)
	l.value.Store(newList)
}

// 删除 listener（等价 Java list.remove）
func (l *LoginStatusListenerList) Remove(target LoginStatusListener) {
	oldList := l.value.Load().([]LoginStatusListener)
	newList := make([]LoginStatusListener, 0, len(oldList))
	for _, fn := range oldList {
		// Go 无法直接比较函数地址，因此通常不做删除，
		// 如确需删除，可由调用方保存函数变量用于比较。
		if &fn != &target {
			newList = append(newList, fn)
		}
	}
	l.value.Store(newList)
}

// 触发事件（等价 Java listeners.forEach(consumer -> consumer.accept())）
func (l *LoginStatusListenerList) Emit(change LoginStatusChange) {
	list := l.value.Load().([]LoginStatusListener)
	for _, fn := range list {
		fn(change) // 同步调用，与 Java CopyOnWriteArrayList 行为一致
	}
}

// PlaywrightManager Playwright 管理器
type PlaywrightManager struct {
	playwright           *playwright.Playwright    // Playwright 实例
	browser              playwright.Browser        // 浏览器实例（所有平台共享）
	context              playwright.BrowserContext // 浏览器上下文（所有平台共享，在同一个窗口中打开多个标签页）
	bossPage             playwright.Page           // Boss直聘页面实例
	loginStatus          sync.Map                  // 登录状态追踪（平台 -> 是否已登录）
	listenerIDCounter    int32
	loginStatusListeners *LoginStatusListenerList // 登录状态监听器
	bossMonitoringPaused atomic.Bool              // 控制是否暂停对bossPage的后台监控，避免与任务执行并发访问同一页面
	cookieService        service.CookieService    // Cookie服务
}

// NewPlaywrightManager 创建新的Playwright管理器
func NewPlaywrightManager(cookieService service.CookieService) *PlaywrightManager {
	return &PlaywrightManager{
		cookieService:        cookieService,
		loginStatusListeners: NewLoginStatusListenerList(),
	}
}

func (m *PlaywrightManager) IsInitialized() bool {
	return m.playwright != nil &&
		m.browser != nil &&
		m.bossPage != nil
}

func (m *PlaywrightManager) Init() error {
	// 设置日志级别为 Debug
    log.SetLevel(log.DebugLevel)
	if m.IsInitialized() {
		return nil
	}

	log.Info("========================================")
	log.Info("  初始化浏览器自动化引擎")
	log.Info("========================================")

	// -------------------------------
	// 1. 启动 Playwright
	// -------------------------------
	pw, err := playwright.Run()
	if err != nil {
		log.Errorf("✗ Playwright 启动失败: %v", err)
		return err
	}
	m.playwright = pw
	log.Info("✓ Playwright 引擎已启动")

	// -------------------------------
	// 2. 启动浏览器实例
	// -------------------------------
	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(false),
		SlowMo:   playwright.Float(50),
		Args: []string{
			"--remote-debugging-port=7866",
			"--start-maximized",
		},
	})
	if err != nil {
		log.Errorf("✗ 浏览器启动失败: %v", err)
		return err
	}
	m.browser = browser
	log.Info("✓ Chrome 浏览器已启动 (调试端口: 7866)")

	// -------------------------------
	// 3. 创建共享 BrowserContext
	// -------------------------------
	context, err := browser.NewContext(playwright.BrowserNewContextOptions{
		Viewport: nil, // 不设置固定视口
		UserAgent: playwright.String(
			"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/135.0.0.0 Safari/537.36",
		),
	})
	if err != nil {
		log.Errorf("✗ BrowserContext 创建失败: %v", err)
		return err
	}
	m.context = context
	log.Info("✓ BrowserContext 已创建（所有平台共享）")

	// -------------------------------
	// 4. 创建页面（禁止并发创建 Page）
	// -------------------------------
	log.Info("开始创建所有平台的 Page...")

	bossPage, err := context.NewPage()
	if err != nil {
		log.Errorf("✗ Boss Page 创建失败: %v", err)
		return err
	}
	bossPage.SetDefaultTimeout(float64(DEFAULR_TIMEOUT.Milliseconds()))
	m.bossPage = bossPage
	log.Info("✓ Boss Page 已创建")

	// -------------------------------
	// 5. 并发初始化平台
	// -------------------------------
	log.Info("开始并发初始化所有平台...")

	var wg sync.WaitGroup

	// Boss
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := m.setupBossPlatform(); err != nil {
			log.Errorf("Boss 初始化失败: %v", err)
		}
	}()

	// 预留其他平台（与 Java 对齐）
	// go func() { ... setupLiepinPlatform() }
	// go func() { ... setup51Platform() }
	// go func() { ... setupZhilianPlatform() }

	wg.Wait()

	//定时检测登录状态
	m.startScheduledLoginCheck()

	log.Info("✓ 浏览器自动化引擎初始化完成（所有平台已并发启动）")
	log.Info("========================================")

	return nil
}

func (m *PlaywrightManager) setupBossPlatform() error {
	log.Info("开始初始化Boss直聘平台...")

	// ========= 1. 尝试从数据库加载 Cookie =========
	cookieEntity, err := m.cookieService.GetCookieByPlatform("boss")
	if err != nil {
		log.Warnf("从数据库加载Boss Cookie失败: %v", err)
	} else if cookieEntity != nil && cookieEntity.CookieValue != "" {
		cookies, err := m.parseCookiesFromString(cookieEntity.CookieValue)
		if err != nil {
			log.Warnf("解析Boss Cookie失败: %v", err)
		} else if len(cookies) > 0 {
			err = m.context.AddCookies(cookies)
			if err != nil {
				log.Warnf("注入Boss Cookie失败: %v", err)
			} else {
				log.Infof("已从数据库加载Boss Cookie并注入浏览器上下文，共 %d 条", len(cookies))
			}
		} else {
			log.Warn("解析Cookie失败，未能加载任何Cookie")
		}
	} else {
		log.Info("数据库未找到Boss Cookie或值为空，跳过Cookie注入")
	}

	// ========= 2. 导航到 Boss 首页（带重试机制）=========
	maxRetries := 3
	navigateSuccess := false

	for attempt := 1; attempt <= maxRetries; attempt++ {
		_, err := m.bossPage.Goto(BOSS_URL, playwright.PageGotoOptions{
			Timeout:   playwright.Float(60000),
			WaitUntil: playwright.WaitUntilStateDomcontentloaded,
		})

		if err == nil {
			navigateSuccess = true
			break
		}

		// Playwright 报错，但页面可能已成功加载 —— 检查 URL
		pageAccessible := false
		url := m.bossPage.URL() // 只返回 string
		if strings.Contains(url, BOSS_URL) {
			pageAccessible = true
		}

		if pageAccessible {
			navigateSuccess = true
			break
		}

		// 失败则重试
		if attempt < maxRetries {
			time.Sleep(2 * time.Second)
		}
	}

	if !navigateSuccess {
		log.Warn("Boss直聘页面导航失败")
		// 导航失败属于初始化失败 → 返回错误
		return fmt.Errorf("boss platform navigate failed")
	}

	// ========= 3. 尝试等待网络空闲状态（非关键步骤）=========
	err = m.bossPage.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State: playwright.LoadStateNetworkidle,
	})
	if err != nil {
		log.Debugf("等待Boss页面网络空闲失败: %v", err)
	}

	// ========= 4. 初始化登录状态并通知监听器 =========
	isLoggedIn,_ := m.checkIfBossLoggedIn()
	m.SetLoginStatus("boss", isLoggedIn)

	// ========= 5. 设置登录监控（类似 Java setupLoginMonitoring）=========
	m.setupLoginMonitoring(m.bossPage)

	log.Info("Boss直聘平台初始化完成")
	return nil
}

func (m *PlaywrightManager) GetBossPage() playwright.Page {
	return m.bossPage
}

// parseCookiesFromString 从JSON字符串解析Cookie列表
func (m *PlaywrightManager) parseCookiesFromString(cookieJSON string) ([]playwright.OptionalCookie, error) {
	var cookies []playwright.OptionalCookie

	if err := json.Unmarshal([]byte(cookieJSON), &cookies); err != nil {
		return nil, fmt.Errorf("解析Cookie JSON失败: %w", err)
	}

	log.Printf("成功解析Cookie，共 %d 条", len(cookies))
	return cookies, nil
}

// 检查 Boss 是否已登录（结构完全对齐 Java）
func (pm *PlaywrightManager) checkIfBossLoggedIn() (bool, error) {
	// 更稳健的登录判断：优先检测用户头像/昵称是否可见；备用检测登录入口是否可见且包含"登录"文本
	
	// 1) 用户名/昵称元素
	userName := pm.bossPage.Locator("li.nav-figure span.label-text").First()
	if err := userName.WaitFor(playwright.LocatorWaitForOptions{Timeout: playwright.Float(2000)}); err == nil {
		visible, err := userName.IsVisible()
		if err == nil && visible {
			return true, nil
		}
	}

	// 2) 头像 img
	avatar := pm.bossPage.Locator("li.nav-figure").First()
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
			// 检查文本内容是否包含"登录"
			text, err := loginAnchor.TextContent()
			if err == nil && strings.Contains(text, "登录") {
				return false, nil
			}
		}
	}

	// 无法明确检测到登录特征时，保守返回未登录
	return false, nil
}

func (m *PlaywrightManager) SetLoginStatus(platform string, isLoggedIn bool) {
	// ========== 1. 获取之前状态 ==========
	var previousStatus *bool
	if val, ok := m.loginStatus.Load(platform); ok {
		v := val.(bool)
		previousStatus = &v
	}

	// 若无变化 → 忽略
	if previousStatus != nil && *previousStatus == isLoggedIn {
		return
	}

	// ========== 2. 更新状态 ==========
	m.loginStatus.Store(platform, isLoggedIn)

	// ========== 3. Boss 平台：未登录 → 自动跳转并切二维码 ==========
	if platform == "boss" && !isLoggedIn {
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Errorf("设置Boss未登录状态时执行登录引导失败: %v", r)
				}
			}()

			if m.bossPage != nil {

				// ---- 3.1 当前 URL ----
				currentUrl := m.bossPage.URL()

				// ---- 3.2 若不在登录页，则跳转一次 ----
				if currentUrl == "" || !strings.Contains(currentUrl, "/web/user/") {
					_, _ = m.bossPage.Goto(
						BOSS_URL+"/web/user/?ka=header-login",
						playwright.PageGotoOptions{Timeout: playwright.Float(60000)},
					)
					time.Sleep(800 * time.Millisecond)
				}

				// ---- 3.3 尝试切换二维码登录 ----

				// 新版选择器
				qr := m.bossPage.Locator(".btn-sign-switch.ewm-switch")
				if visible, _ := qr.IsVisible(); visible {
					_ = qr.Click()
					return
				}

				// 文本匹配 “APP扫码登录”
				tip := m.bossPage.GetByText("APP扫码登录")
				if visible, _ := tip.IsVisible(); visible {
					_ = tip.Click()
					log.Info("已点击包含文本的二维码登录切换提示（APP扫码登录）")
					return
				}

				// 旧版选择器（li.sign-switch-tip）
				legacy := m.bossPage.Locator("li.sign-switch-tip")
				if visible, _ := legacy.IsVisible(); visible {
					_ = legacy.Click()
					log.Info("已通过旧版选择器切换二维码登录（li.sign-switch-tip）")
					return
				}

				log.Info("未找到二维码登录切换按钮，保持当前登录页")
			}
		}()
	}

	// ========== 4. 组装事件 ==========
	change := LoginStatusChange{
		Platform:   platform,
		IsLoggedIn: isLoggedIn,
		Timestamp:  time.Now().UnixMilli(),
	}

	// ========== 5. 通知监听器 ==========
	m.loginStatusListeners.Emit(change)
}

// setupLoginMonitoring 设置 Boss 登录状态监控（与 Java 版本完全对齐）
func (m *PlaywrightManager) setupLoginMonitoring(page playwright.Page) {
	if page == nil {
		log.Warn("setupLoginMonitoring: page 为 nil，无法绑定监听器")
		return
	}

	// 监听页面导航事件（等价 Java page.onFrameNavigated）
	page.OnFrameNavigated(func(frame playwright.Frame) {
		// 只处理主 Frame，等价 Java 的 frame == page.mainFrame()
		if frame != page.MainFrame() {
			return
		}

		// 若监控暂停（atomic），直接忽略
		if m.bossMonitoringPaused.Load() {
			return
		}

		// 执行登录状态检测
		m.checkLoginStatus(page, "boss")
	})

	log.Info("Boss 平台登录状态监控已启用")
}

func (m *PlaywrightManager) checkLoginStatus(page playwright.Page, platform string) {
	defer func() {
		if r := recover(); r != nil {
			log.Debugf("检查 %s 平台登录状态时发生异常: %v", platform, r)
		}
	}()

	// ========== 1. 仅 boss 支持检查（保持 Java 一致） ==========
	var isLoggedIn bool
	if platform == "boss" {
		isLoggedIn,_ = m.checkIfBossLoggedIn() // 已实现的稳定版本
	}

	// ========== 2. 获取 previousStatus ==========
	var previous *bool
	if v, ok := m.loginStatus.Load(platform); ok {
		vb := v.(bool)
		previous = &vb
	}

	// ========== 3. Java 逻辑：只有从 未登录 → 登录 才触发 onLoginSuccess ==========
	if isLoggedIn && (previous == nil || !*previous) {
		m.onLoginSuccess(platform)
	}
}

func (m *PlaywrightManager) onLoginSuccess(platform string) {
	log.Infof("%s 平台登录成功", platform)

	// 1) 更新登录状态并触发所有监听器（保持与 Java 一致）
	m.SetLoginStatus(platform, true)

	// 2) BOSS 平台：登录成功后自动保存 Cookie 到数据库
	if platform == "boss" {
		m.saveBossCookiesToDatabase("login success")
	}
}

// saveBossCookiesToDatabase 统一的 Boss Cookie 保存方法（使用 JSON 序列化）
func (m *PlaywrightManager) saveBossCookiesToDatabase(remark string) {
	defer func() {
		if r := recover(); r != nil {
			log.Warnf("保存Boss Cookie失败（panic恢复）: %v", r)
		}
	}()

	cookies, err := m.context.Cookies()
	if err != nil {
		log.Warnf("保存Boss Cookie失败，无法获取Cookies: %v", err)
		return
	}

	// 序列化为 JSON
	cookieBytes, err := json.Marshal(cookies)
	if err != nil {
		log.Warnf("保存Boss Cookie失败，序列化错误: %v", err)
		return
	}

	cookieJson := string(cookieBytes)
	ok, _ := m.cookieService.SaveOrUpdateCookie("boss", cookieJson, remark)
	if ok {
		log.Infof("保存Boss Cookie成功，共 %d 条，remark=%s", len(cookies), remark)
	}
}

// Close 关闭 PlaywrightManager 所有资源
func (m *PlaywrightManager) Close() {
	log.Info("开始关闭Playwright管理器...")

	// 捕获 panic，避免程序在关闭阶段崩溃
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("关闭Playwright管理器时发生panic: %v", r)
		}
	}()

	// 1. 关闭 Boss 页面
	if m.bossPage != nil {
		if err := m.bossPage.Close(); err != nil {
			log.Warnf("关闭Boss直聘页面时发生错误: %v", err)
		} else {
			log.Info("Boss直聘页面已关闭")
		}
		m.bossPage = nil
	}

	// 2. 关闭浏览器
	if m.browser != nil {
		if err := m.browser.Close(); err != nil {
			log.Warnf("关闭浏览器时发生错误: %v", err)
		} else {
			log.Info("浏览器已关闭")
		}
		m.browser = nil
	}

	// 3. 关闭 Playwright 实例
	if m.playwright != nil {
		if err := m.playwright.Stop(); err != nil {
			log.Warnf("关闭Playwright实例时发生错误: %v", err)
		} else {
			log.Info("Playwright实例已关闭")
		}
		m.playwright = nil
	}

	log.Info("Playwright管理器关闭完成！")
}

// IsLoggedIn 获取指定平台的登录状态
func (m *PlaywrightManager) IsLoggedIn(platform string) bool {
	if val, ok := m.loginStatus.Load(platform); ok {
		if loggedIn, ok2 := val.(bool); ok2 {
			return loggedIn
		}
	}
	return false
}

// PauseBossMonitoring 暂停 Boss 页面后台登录监控（避免与业务流程并发操作页面）
func (m *PlaywrightManager) PauseBossMonitoring() {
	m.bossMonitoringPaused.Store(true)
	log.Debug("Boss登录监控已暂停")
}

// ResumeBossMonitoring 恢复 Boss 页面后台登录监控
func (m *PlaywrightManager) ResumeBossMonitoring() {
	m.bossMonitoringPaused.Store(false)
	log.Debug("Boss登录监控已恢复")
}

// startScheduledLoginCheck 启动后台定时检查登录状态（每 3 秒）
func (m *PlaywrightManager) startScheduledLoginCheck() {
	go func() {
		log.Info("启动定时登录检测（Boss，每3秒）")

		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()

		for range ticker.C {

			log.Debug("定时器 tick：准备执行 Boss 登录检测")

			// 若监控暂停，跳过
			if m.bossMonitoringPaused.Load() {
				log.Debug("Boss 登录监控已暂停，本轮检测跳过")
				continue
			}

			// Boss 登录状态检查
			if m.bossPage != nil {
				log.Debug("执行 Boss 登录检测...")

				func() {
					defer func() {
						if r := recover(); r != nil {
							log.Debugf("定时登录检测 panic: %v", r)
						}
					}()

					m.checkLoginStatus(m.bossPage, "boss")
				}()

				log.Debug("Boss 登录检测完成")
			} else {
				log.Debug("Boss 页面为空，跳过检测")
			}
		}
	}()
}
