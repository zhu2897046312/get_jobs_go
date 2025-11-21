# get_jobs_go 

# model 模型层 没有依赖
# config 配置层 没有依赖
# utils 工具层 没有依赖
# loctors 层 没有依赖

# repository 数据层 依赖 model层 
## boss_repository 数据层 依赖 model层
## ai_repository 数据层 依赖 model层
## cookie_repository 数据层 依赖 model层

# service 服务层 依赖 repository层 model层 
## boss_service 服务层 依赖 repository层 model层 config层
## ai_service 服务层 依赖 repository层 model层 
## cookie_service 服务层 依赖 repository层 model层 

# 入口 worker -> service ->executeDelivery() 
# worker

## boss_worker 服务层 依赖 boss_service层 ai_service层 model层 config层 uitls层

# playwright安装路径 2025/11/20 20:02:51 INFO Downloading driver path=C:\Users\28970\AppData\Local\ms-playwright-go\1.52.0
```java
package com.getjobs.worker.manager;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.getjobs.application.entity.CookieEntity;
import com.getjobs.application.service.CookieService;
import com.microsoft.playwright.*;
import com.microsoft.playwright.options.Cookie;
import com.microsoft.playwright.options.WaitUntilState;
import com.microsoft.playwright.options.LoadState;
import jakarta.annotation.PreDestroy;
import lombok.Getter;
import lombok.extern.slf4j.Slf4j;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.context.annotation.Lazy;
import org.springframework.stereotype.Component;
import org.springframework.scheduling.annotation.Scheduled;

import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.CopyOnWriteArrayList;
import java.util.function.Consumer;

/**
 * Playwright管理器
 * Spring管理的单例Bean，在应用启动时自动初始化Playwright实例
 * 支持4个求职平台的共享BrowserContext和登录状态监控
 * 所有平台在同一个浏览器窗口的不同标签页中运行
 */
@Slf4j
@Getter
@Component
@Lazy
public class PlaywrightManager {

    // Playwright实例
    private Playwright playwright;

    // 浏览器实例（所有平台共享）
    private Browser browser;

    // 浏览器上下文（所有平台共享，在同一个窗口中打开多个标签页）
    private BrowserContext context;

    // Boss直聘页面
    private Page bossPage;

    // 登录状态追踪（平台 -> 是否已登录）
    private final Map<String, Boolean> loginStatus = new ConcurrentHashMap<>();

    // 登录状态监听器
    private final List<Consumer<LoginStatusChange>> loginStatusListeners = new CopyOnWriteArrayList<>();

    // 控制是否暂停对bossPage的后台监控，避免与任务执行并发访问同一页面
    private volatile boolean bossMonitoringPaused = false;

    // 默认超时时间（毫秒）
  private static final int DEFAULT_TIMEOUT = 30000;

    // Playwright调试端口
    private static final int CDP_PORT = 7866;

    // 平台URL常量  - 可拓展
    private static final String BOSS_URL = "https://www.zhipin.com";

    @Autowired
    private CookieService cookieService;

    /**
     * 初始化Playwright实例（延迟初始化）
     */
    public void init() {
        if (isInitialized()) {
            return;
        }
        log.info("========================================");
        log.info("  初始化浏览器自动化引擎");
        log.info("========================================");

        try {
            // 启动Playwright
            playwright = Playwright.create();
            log.info("✓ Playwright引擎已启动");

            // 创建浏览器实例，使用固定CDP端口7866，最大化启动
            browser = playwright.chromium().launch(new BrowserType.LaunchOptions()
                    .setHeadless(false) // 非无头模式，可视化调试
                    .setSlowMo(50) // 放慢操作速度，便于调试
                    .setArgs(List.of(
                            "--remote-debugging-port=" + CDP_PORT, // 使用固定CDP端口
                            "--start-maximized" // 最大化启动窗口
                    )));
            log.info("✓ Chrome浏览器已启动 (调试端口: {})", CDP_PORT);

            // 创建共享的BrowserContext（所有平台在同一个窗口的不同标签页中）
            context = browser.newContext(new Browser.NewContextOptions()
                    .setViewportSize(null) // 不设置固定视口，使用浏览器窗口实际大小
                    .setUserAgent(
                            "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/135.0.0.0 Safari/537.36"));
            log.info("✓ BrowserContext已创建（所有平台共享）");

            // 顺序创建所有Page（避免并发创建Page导致的竞态条件）
            log.info("开始创建所有平台的Page...");
            bossPage = context.newPage();
            bossPage.setDefaultTimeout(DEFAULT_TIMEOUT);
            log.info("✓ Boss Page已创建");

            // 并发执行各平台的初始化逻辑（导航、Cookie加载等）
            log.info("开始并发初始化所有平台...");
            CompletableFuture<Void> bossFuture = CompletableFuture.runAsync(this::setupBossPlatform);

            // 等待所有平台初始化完成
            CompletableFuture.allOf(bossFuture, liepinFuture, job51Future, zhilianFuture).join();

            log.info("✓ 浏览器自动化引擎初始化完成（所有平台已并发启动）");
            log.info("========================================");
        } catch (Exception e) {
            log.error("✗ 浏览器自动化引擎初始化失败", e);
            throw new RuntimeException("Playwright初始化失败", e);
        }
    }

    /**
     * 设置Boss直聘平台（加载Cookie、导航、监控）
     */
    private void setupBossPlatform() {
        log.info("开始初始化Boss直聘平台...");

        // 尝试从数据库加载Boss平台Cookie到上下文
        try {
            CookieEntity cookieEntity = cookieService.getCookieByPlatform("boss");
            if (cookieEntity != null && cookieEntity.getCookieValue() != null && !cookieEntity.getCookieValue().isBlank()) {
                String cookieStr = cookieEntity.getCookieValue();
                List<Cookie> cookies = parseCookiesFromString(cookieStr);

                if (!cookies.isEmpty()) {
                    context.addCookies(cookies);
                    log.info("已从数据库加载Boss Cookie并注入浏览器上下文，共 {} 条", cookies.size());
                } else {
                    log.warn("解析Cookie失败，未能加载任何Cookie");
                }
            } else {
                log.info("数据库未找到Boss Cookie或值为空，跳过Cookie注入");
            }
        } catch (Exception e) {
            log.warn("从数据库加载Boss Cookie失败: {}", e.getMessage());
        }

        // 导航到Boss直聘首页（带重试机制）
        int maxRetries = 3;
        boolean navigateSuccess = false;
        for (int attempt = 1; attempt <= maxRetries; attempt++) {
            try {
                bossPage.navigate(BOSS_URL, new Page.NavigateOptions()
                        .setTimeout(60000)
                        .setWaitUntil(WaitUntilState.DOMCONTENTLOADED));
                navigateSuccess = true;
                break;
            } catch (Exception e) {
                // Playwright在并发导航时可能抛出 "Object doesn't exist" 异常，但页面实际已加载
                boolean pageAccessible = false;
                try {
                    String url = bossPage.url();
                    pageAccessible = url != null && url.contains("zhipin.com");
                } catch (Exception ignored) {
                }

                if (pageAccessible) {
                    navigateSuccess = true;
                    break;
                }

                if (attempt < maxRetries) {
                    try {
                        Thread.sleep(2000);
                    } catch (InterruptedException ie) {
                        Thread.currentThread().interrupt();
                    }
                }
            }
        }

        if (!navigateSuccess) {
            log.warn("Boss直聘页面导航失败");
        }

        try {
            // 等待页面网络空闲，确保头部导航渲染完成
            try {
                bossPage.waitForLoadState(LoadState.NETWORKIDLE);
            } catch (Exception e) {
                log.debug("等待Boss页面网络空闲失败: {}", e.getMessage());
            }

            // 初始化阶段不主动跳转登录页，仅在导航后设置状态
            // 参考猎聘实现：加载Cookie并导航后，由业务侧决定是否触发后续登录流程
        } catch (Exception e) {
            log.warn("Boss直聘页面导航失败: {}", e.getMessage());
        }
        // 初始化登录状态并通知（如果有SSE连接会立即推送）
        setLoginStatus("boss", checkIfLoggedIn());
        // 设置登录状态监控
        setupLoginMonitoring(bossPage);
    }

    /**
     * 检查Boss是否已登录
     */
    private boolean checkIfLoggedIn() {
        // 更稳健的登录判断：优先检测用户头像/昵称是否可见；备用检测登录入口是否可见且包含“登录”文本
        try {
            Locator userLabel = bossPage.locator("li.nav-figure span.label-text").first();
            if (userLabel.isVisible()) {
                return true;
            }
        } catch (Exception ignored) {}

        try {
            // 有些版本仅展示头像入口，无 label-text
            Locator navFigure = bossPage.locator("li.nav-figure").first();
            if (navFigure.isVisible()) {
                return true;
            }
        } catch (Exception ignored) {}

        try {
            // 未登录时通常有“登录/注册”入口或按钮容器
            Locator loginAnchor = bossPage.locator("li.nav-sign a, .btns").first();
            if (loginAnchor.isVisible()) {
                String text = loginAnchor.textContent();
                if (text != null && text.contains("登录")) {
                    return false;
                }
            }
        } catch (Exception ignored) {}

        // 无法明确检测到登录特征时，保守返回未登录
        return false;
    }

    /**
     * 设置登录状态监控
     *
     * @param page 页面实例
     */
    private void setupLoginMonitoring(Page page) {
        // 监听页面导航事件，检测URL变化
        page.onFrameNavigated(frame -> {
            if (frame == page.mainFrame()) {
                // 事件触发的检查在Playwright内部线程执行，仍需遵守暂停标志
                if (!bossMonitoringPaused) {
                    checkLoginStatus(page, "boss");
                }
            }
        });

        log.info("{}平台登录状态监控已启用", "boss");
    }

    /**
     * 手动设置平台登录状态（会触发SSE通知）
     *
     * @param platform   平台名称
     * @param isLoggedIn 是否已登录
     */
    public void setLoginStatus(String platform, boolean isLoggedIn) {
        Boolean previousStatus = loginStatus.get(platform);

        // 只有状态真正发生变化时才更新和通知
        if (previousStatus == null || previousStatus != isLoggedIn) {
            loginStatus.put(platform, isLoggedIn);

            // Boss平台：在设置未登录状态时，顺带引导到登录页并切换二维码扫码
            if ("boss".equals(platform) && !isLoggedIn) {
                try {
                    if (bossPage != null) {
                        String currentUrl = null;
                        try { currentUrl = bossPage.url(); } catch (Exception ignored) {}

                        // 避免重复导航：若当前已在登录页则不再二次跳转
                        if (currentUrl == null || !currentUrl.contains("/web/user/")) {
                            bossPage.navigate(BOSS_URL + "/web/user/?ka=header-login");
                            try { Thread.sleep(800); } catch (InterruptedException ie) { Thread.currentThread().interrupt(); }
                        }

                        // 尝试切换到二维码登录（点击“APP扫码登录”按钮），优先使用新版选择器
                        try {
                            Locator qrSwitch = bossPage.locator(".btn-sign-switch.ewm-switch").first();
                            if (qrSwitch.isVisible()) {
                                qrSwitch.click();
                            } else {
                                // 兜底：按文本匹配内部提示
                                Locator tip = bossPage.getByText("APP扫码登录").first();
                                if (tip.isVisible()) {
                                    tip.click();
                                    log.info("已点击包含文本的二维码登录切换提示（APP扫码登录）");
                                } else {
                                    // 兼容旧版选择器
                                    Locator legacy = bossPage.locator("li.sign-switch-tip").first();
                                    if (legacy.isVisible()) {
                                        legacy.click();
                                        log.info("已通过旧版选择器切换二维码登录（li.sign-switch-tip）");
                                    } else {
                                        log.info("未找到二维码登录切换按钮，保持当前登录页");
                                    }
                                }
                            }
                        } catch (Exception e) {
                            log.debug("切换二维码登录失败: {}", e.getMessage());
                        }
                    }
                } catch (Exception e) {
                    log.debug("设置Boss未登录状态时执行登录引导失败: {}", e.getMessage());
                }
            }

            // 通知所有监听器（触发SSE推送）
            LoginStatusChange change = new LoginStatusChange(platform, isLoggedIn, System.currentTimeMillis());
            loginStatusListeners.forEach(listener -> {
                try {
                    listener.accept(change);
                } catch (Exception e) {
                    log.error("通知登录状态监听器失败: platform={}, isLoggedIn={}", platform, isLoggedIn, e);
                }
            });

//            log.info("登录状态已更新: platform={}, isLoggedIn={}", platform, isLoggedIn);
        }
    }

    /**
     * 从JSON字符串解析Cookie列表
     *
     * @param cookieJson Cookie的JSON字符串
     * @return Cookie列表
     */
    private List<Cookie> parseCookiesFromString(String cookieJson) {
        List<Cookie> cookies = new ArrayList<>();

        try {
            ObjectMapper objectMapper = new ObjectMapper();
            com.fasterxml.jackson.databind.JsonNode jsonArray = objectMapper.readTree(cookieJson);

            for (com.fasterxml.jackson.databind.JsonNode node : jsonArray) {
                // 创建Cookie对象（name和value是必需的）
                Cookie cookie = new Cookie(
                        node.get("name").asText(),
                        node.get("value").asText()
                );

                // 设置可选字段
                if (node.has("domain") && !node.get("domain").isNull()) {
                    cookie.domain = node.get("domain").asText();
                }
                if (node.has("path") && !node.get("path").isNull()) {
                    cookie.path = node.get("path").asText();
                }
                if (node.has("expires") && !node.get("expires").isNull()) {
                    cookie.expires = node.get("expires").asDouble();
                }
                if (node.has("httpOnly") && !node.get("httpOnly").isNull()) {
                    cookie.httpOnly = node.get("httpOnly").asBoolean();
                }
                if (node.has("secure") && !node.get("secure").isNull()) {
                    cookie.secure = node.get("secure").asBoolean();
                }
                if (node.has("sameSite") && !node.get("sameSite").isNull()) {
                    String sameSite = node.get("sameSite").asText();
                    if (sameSite != null && !sameSite.isEmpty()) {
                        cookie.sameSite = com.microsoft.playwright.options.SameSiteAttribute.valueOf(
                                sameSite.toUpperCase()
                        );
                    }
                }

                cookies.add(cookie);
            }

            log.debug("成功解析Cookie，共 {} 条", cookies.size());
        } catch (Exception e) {
            log.error("解析Cookie JSON失败: {}", e.getMessage(), e);
        }

        return cookies;
    }
    /**
     * 检查登录状态
     *
     * @param page     页面实例
     * @param platform 平台名称
     */
    private void checkLoginStatus(Page page, String platform) {
        try {
            boolean isLoggedIn = false;
            if (platform.equals("boss")) {
                // 统一复用更稳健的Boss登录判断逻辑
                isLoggedIn = checkIfLoggedIn();
            }
            // 如果登录状态发生变化（从未登录变为已登录）
            Boolean previousStatus = loginStatus.get(platform);
            if (isLoggedIn && (previousStatus == null || !previousStatus)) {
                onLoginSuccess(platform);
            }
        } catch (Exception e) {
            // 忽略检查过程中的异常，避免影响正常流程
            log.debug("检查{}平台登录状态时发生异常: {}", platform, e.getMessage());
        }
    }

     /**
     * 登录成功回调
     *
     * @param platform 平台名称
     */
    private void onLoginSuccess(String platform) {
        log.info("{}平台登录成功", platform);

        // 更新登录状态并通知（统一使用setLoginStatus方法）
        setLoginStatus(platform, true);

        // 登录成功时保存 Cookie 到数据库（仅 boss 平台）
        if ("boss".equals(platform)) {
            saveBossCookiesToDatabase("login success");
        }
    }


    /**
     * 统一的Boss Cookie保存方法（使用JSON序列化）
     *
     * @param remark 备注信息
     */
    private void saveBossCookiesToDatabase(String remark) {
        try {
            List<com.microsoft.playwright.options.Cookie> cookies = context.cookies();
            // 使用ObjectMapper序列化为JSON字符串
            String cookieJson = new ObjectMapper().writeValueAsString(cookies);
            boolean result = cookieService.saveOrUpdateCookie("boss", cookieJson, remark);
            if (result) {
                log.info("保存Boss Cookie成功，共 {} 条，remark={}", cookies.size(), remark);
            }
        } catch (Exception e) {
            log.warn("保存Boss Cookie失败: {}", e.getMessage());
        }
    }

     /**
     * 主动保存 Boss Cookie 到数据库（用于调试/验证）
     */
    public void saveBossCookiesToDb(String remark) {
        saveBossCookiesToDatabase(remark);
    }

     /**
     * 清理Boss上下文中的Cookie
     * 用于退出登录时清除浏览器上下文中的所有Cookie
     */
    public void clearBossCookies() {
        try {
            if (context != null) {
                context.clearCookies();
                log.info("已清理共享上下文中的所有Cookie");
            } else {
                log.warn("共享上下文不存在，无法清理Cookie");
            }
        } catch (Exception e) {
            log.error("清理共享上下文Cookie失败: {}", e.getMessage(), e);
            throw new RuntimeException("清理共享上下文Cookie失败", e);
        }
    }

    /**
     * 定时检查登录状态（每3秒）
     * 用于捕获通过DOM元素判断登录状态的场景（无导航也可触发）
     */
    @Scheduled(fixedDelay = 3000)
    public void scheduledLoginCheck() {
        try {
            if (liepinPage != null && !liepinMonitoringPaused) {
                checkLiepinLoginStatus(liepinPage);
            }
            // 其他平台如需也可启用（保留，但不强制）
            if (bossPage != null && !bossMonitoringPaused) {
                checkLoginStatus(bossPage, "boss");
            }
        } catch (Exception e) {
            log.debug("定时登录检测异常: {}", e.getMessage());
        }
    }

    /**
     * 暂停Boss页面的后台登录监控（避免与业务流程并发操作页面）
     */
    public void pauseBossMonitoring() {
        bossMonitoringPaused = true;
        log.debug("Boss登录监控已暂停");
    }

    /**
     * 恢复Boss页面的后台登录监控
     */
    public void resumeBossMonitoring() {
        bossMonitoringPaused = false;
        log.debug("Boss登录监控已恢复");
    }

     /**
     * 关闭Playwright实例
     * 在Spring容器销毁前自动执行
     */
    @PreDestroy
    public void destroy() {
        log.info("开始关闭Playwright管理器...");

        try {
            // 关闭所有页面
            if (bossPage != null) {
                bossPage.close();
                log.info("Boss直聘页面已关闭");
            }

            // 关闭共享的BrowserContext
            if (context != null) {
                context.close();
                log.info("共享BrowserContext已关闭");
            }

            // 关闭浏览器
            if (browser != null) {
                browser.close();
                log.info("浏览器已关闭");
            }
            if (playwright != null) {
                playwright.close();
                log.info("Playwright实例已关闭");
            }

            log.info("Playwright管理器关闭完成！");
        } catch (Exception e) {
            log.error("关闭Playwright管理器时发生错误", e);
        }
    }

    /**
     * 注册登录状态监听器
     *
     * @param listener 监听器
     */
    public void addLoginStatusListener(Consumer<LoginStatusChange> listener) {
        loginStatusListeners.add(listener);
    }

    /**
     * 移除登录状态监听器
     *
     * @param listener 监听器
     */
    public void removeLoginStatusListener(Consumer<LoginStatusChange> listener) {
        loginStatusListeners.remove(listener);
    }

    /**
     * 获取平台登录状态
     *
     * @param platform 平台名称
     * @return 是否已登录
     */
    public boolean isLoggedIn(String platform) {
        return loginStatus.getOrDefault(platform, false);
    }
}
```