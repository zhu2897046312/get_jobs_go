package boss

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
	"get_jobs_go/application/service"
	"github.com/playwright-community/playwright-go"
)

type Blacklist struct {
	Companies  []string `json:"companies"`
	Recruiters []string `json:"recruiters"`
	Jobs       []string `json:"jobs"`
}

type BossWorker struct {
	page         playwright.Page
	config       *BossConfig
	db           service.DBService
	aiService    AIService
	blacklist    *Blacklist
	stopChan     chan struct{}
	progressChan chan ProgressMessage
}

type ProgressMessage struct {
	Message string `json:"message"`
	Current int    `json:"current"`
	Total   int    `json:"total"`
}

// type DBService interface {
// 	GetBlacklists() (*Blacklist, error)
// 	SaveJob(job *JobDetail) error
// 	UpdateDeliveryStatus(encryptID, encryptUserID, status string) error
// 	JobExists(encryptID, encryptUserID string) bool
// }

type AIService interface {
	GenerateGreeting(introduce, keyword, jobName, jobDesc, reference string) (string, error)
}

func NewBossWorker(cfg *BossConfig, db service.DBService, ai AIService) *BossWorker {
	return &BossWorker{
		config:       cfg,
		db:           db,
		aiService:    ai,
		stopChan:     make(chan struct{}),
		progressChan: make(chan ProgressMessage, 100),
	}
}

func (b *BossWorker) SetPage(page playwright.Page) {
	b.page = page
}

func (b *BossWorker) Progress() <-chan ProgressMessage {
	return b.progressChan
}

// 准备阶段：加载黑名单
func (b *BossWorker) Prepare() error {
	blacklists, err := b.db.GetBlacklists()
	if err != nil {
		return fmt.Errorf("加载黑名单失败: %w", err)
	}
	b.blacklist = blacklists

	log.Printf("黑名单加载完成: 公司(%d) 招聘者(%d) 职位(%d)",
		len(blacklists.Companies), len(blacklists.Recruiters), len(blacklists.Jobs))

	return nil
}

// 主执行入口
func (b *BossWorker) Execute(ctx context.Context) (int, error) {
	var totalPosted int

	for _, cityCode := range b.config.CityCode {
		if b.shouldStop() {
			b.sendProgress("用户取消投递", 0, 0)
			return totalPosted, nil
		}

		posted, err := b.postJobByCity(ctx, cityCode)
		if err != nil {
			return totalPosted, err
		}
		totalPosted += posted

		if b.shouldStop() {
			b.sendProgress("用户取消投递", 0, 0)
			break
		}
	}

	return totalPosted, nil
}

// core/city_poster.go
func (b *BossWorker) postJobByCity(ctx context.Context, cityCode string) (int, error) {
	var totalPosted int

	for _, keyword := range b.config.Keywords {
		if b.shouldStop() {
			return totalPosted, nil
		}

		posted, err := b.searchAndPostJobs(ctx, cityCode, keyword)
		if err != nil {
			return totalPosted, err
		}
		totalPosted += posted
	}

	return totalPosted, nil
}

func (b *BossWorker) searchAndPostJobs(ctx context.Context, cityCode, keyword string) (int, error) {
	// 构建搜索URL
	searchURL := b.buildSearchURL(cityCode, keyword)

	// 导航到搜索页面
	if _, err := b.page.Goto(searchURL, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
	}); err != nil {
		return 0, fmt.Errorf("导航到搜索页面失败: %w", err)
	}

	// 等待职位列表加载
	// 修正后的代码
	// 等待职位列表加载
	_, err := b.page.WaitForSelector("ul.rec-job-list", playwright.PageWaitForSelectorOptions{
		Timeout: playwright.Float(60000),
	})
	if err != nil {
		return 0, fmt.Errorf("等待职位列表超时: %w", err)
	}

	// 滚动加载所有职位
	if err := b.scrollToLoadAllJobs(); err != nil {
		return 0, fmt.Errorf("滚动加载职位失败: %w", err)
	}

	// 获取所有职位卡片
	cards, err := b.page.Locator("ul.rec-job-list li.job-card-box").All()
	if err != nil {
		return 0, fmt.Errorf("获取职位卡片失败: %w", err)
	}

	log.Printf("【%s】岗位已全部加载，总数:%d", keyword, len(cards))
	b.sendProgress("岗位加载完成："+keyword, 0, len(cards))

	var postedCount int
	for i := 0; i < len(cards); i++ {
		if b.shouldStop() {
			return postedCount, nil
		}

		// 重新获取卡片（避免元素过期）
		cards, _ = b.page.Locator("ul.rec-job-list li.job-card-box").All()
		if i >= len(cards) {
			break
		}

		card := cards[i]
		if err := b.processSingleJob(ctx, card, keyword, i, len(cards)); err != nil {
			log.Printf("处理职位失败: %v", err)
			continue
		}
		postedCount++

		// 滚动避免页面刷新问题
		if i >= 5 {
			b.page.Evaluate("window.scrollBy(0, 140)")
			time.Sleep(1 * time.Second)
		}
	}

	log.Printf("【%s】岗位已投递完毕！已投递岗位数量:%d", keyword, postedCount)
	return postedCount, nil
}

// core/job_processor.go
func (b *BossWorker) processSingleJob(ctx context.Context, card playwright.Locator, keyword string, index, total int) error {
	// 点击卡片获取详情
	jobDetail, err := b.clickAndGetJobDetail(card, index)
	if err != nil {
		return fmt.Errorf("获取职位详情失败: %w", err)
	}

	// 保存职位数据到数据库
	if !b.db.JobExists(jobDetail.EncryptID, jobDetail.EncryptUserID) {
		if err := b.db.SaveJob(jobDetail); err != nil {
			log.Printf("保存职位数据失败: %v", err)
		}
	}

	// 过滤检查
	if b.shouldFilterJob(jobDetail) {
		return nil // 被过滤，不投递
	}

	// 执行投递
	b.sendProgress("正在投递："+jobDetail.JobName, index+1, total)
	if err := b.resumeSubmission(ctx, jobDetail, keyword); err != nil {
		// 更新状态为投递失败
		b.db.UpdateDeliveryStatus(jobDetail.EncryptID, jobDetail.EncryptUserID, "投递失败")
		return fmt.Errorf("投递失败: %w", err)
	}

	// 更新状态为已投递
	b.db.UpdateDeliveryStatus(jobDetail.EncryptID, jobDetail.EncryptUserID, "已投递")
	return nil
}

func (b *BossWorker) clickAndGetJobDetail(card playwright.Locator, index int) (*JobDetail, error) {
	var jobDetail *JobDetail

	// 监听岗位详情API响应
	responseChan := make(chan playwright.Response, 1)
	b.page.OnResponse(func(response playwright.Response) {
		if strings.Contains(response.URL(), "/wapi/zpgeek/job/detail.json") &&
			response.Request().Method() == "GET" {
			select {
			case responseChan <- response:
			default:
			}
		}
	})

	// 点击卡片
	if index == 0 {
		// 第一个卡片特殊处理：先点第二个再点第一个
		cards := b.page.Locator("ul.rec-job-list li.job-card-box")
		cards_count, err := cards.Count()
		if err != nil {
			return nil, err
		}
		if cards_count > 1 {
			cards.Nth(1).Click()
			time.Sleep(1 * time.Second)
		}
		cards.Nth(0).Click()
	} else {
		card.Click()
	}

	// 等待API响应
	select {
	case response := <-responseChan:
		body, err := response.Body()
		if err != nil {
			return nil, err
		}
		jobDetail = b.parseJobDetailJSON(string(body))
	case <-time.After(5 * time.Second):
		return nil, fmt.Errorf("获取职位详情超时")
	}

	time.Sleep(1 * time.Second)
	return jobDetail, nil
}

func (b *BossWorker) parseJobDetailJSON(body string) *JobDetail {
	// 解析JSON响应，提取职位信息
	// 这里需要根据实际的Boss直聘API响应结构来解析
	// 返回JobDetail结构体
	return &JobDetail{}
}

// core/filter.go
func (b *BossWorker) shouldFilterJob(job *JobDetail) bool {
	// 职位黑名单过滤
	if b.isInBlacklist(job.JobName, b.blacklist.Jobs) {
		log.Printf("被过滤：职位黑名单命中 | 公司：%s | 岗位：%s", job.CompanyName, job.JobName)
		return true
	}

	// 公司黑名单过滤
	if b.isInBlacklist(job.CompanyName, b.blacklist.Companies) {
		log.Printf("被过滤：公司黑名单命中 | 公司：%s | 岗位：%s", job.CompanyName, job.JobName)
		return true
	}

	// 招聘者黑名单过滤
	if b.isInBlacklist(job.HRPosition, b.blacklist.Recruiters) {
		log.Printf("被过滤：招聘者黑名单命中 | 公司：%s | 岗位：%s | 招聘者：%s",
			job.CompanyName, job.JobName, job.HRPosition)
		return true
	}

	// HR活跃状态过滤
	if b.config.FilterDeadHR && b.isHRInactive(job.HRActiveStatus) {
		log.Printf("被过滤：HR不活跃 | 公司：%s | 岗位：%s | 活跃：%s",
			job.CompanyName, job.JobName, job.HRActiveStatus)
		return true
	}

	// 薪资过滤
	if b.isSalaryNotExpected(job.Salary) {
		log.Printf("被过滤：薪资不符合预期 | 公司：%s | 岗位：%s | 薪资：%s",
			job.CompanyName, job.JobName, job.Salary)
		return true
	}

	return false
}

func (b *BossWorker) isInBlacklist(text string, blacklist []string) bool {
	if text == "" || len(blacklist) == 0 {
		return false
	}

	for _, item := range blacklist {
		if strings.Contains(text, item) {
			return true
		}
	}
	return false
}

func (b *BossWorker) isHRInactive(activeStatus string) bool {
	// 包含"年"视为不活跃
	return strings.Contains(activeStatus, "年")
}

func (b *BossWorker) isSalaryNotExpected(salary string) bool {
	if len(b.config.ExpectedSalary) < 2 {
		return false
	}

	// 解析薪资范围并与期望薪资比较
	// 实现薪资解析逻辑
	return false
}

func (b *BossWorker) openJobDetailPage() (playwright.Page, string, error) {
	// 查找"查看更多信息"按钮
	moreInfoBtn := b.page.Locator("a.more-job-btn")
	moreInfoBtn_count, err := moreInfoBtn.Count()
	if err != nil {
		return nil, "", err
	}
	if moreInfoBtn_count == 0 {
		return nil, "", fmt.Errorf("未找到\"查看更多信息\"按钮")
	}

	// 获取详情页链接
	href, err := moreInfoBtn.First().GetAttribute("href")
	if err != nil || !strings.HasPrefix(href, "/job_detail/") {
		return nil, "", fmt.Errorf("未获取到有效的岗位详情链接")
	}

	detailURL := "https://www.zhipin.com" + href

	// 在新窗口打开详情页
	detailPage, err := b.page.Context().NewPage()
	if err != nil {
		return nil, "", err
	}

	if _, err := detailPage.Goto(detailURL); err != nil {
		detailPage.Close()
		return nil, "", err
	}

	time.Sleep(1 * time.Second)
	return detailPage, detailURL, nil
}

// 修正后的代码
func (b *BossWorker) clickChatButton(detailPage playwright.Page) error {
	// 查找立即沟通按钮
	chatBtn := detailPage.Locator("a.btn-startchat, a.op-btn-chat")

	for i := 0; i < 5; i++ {
		if b.shouldStop() {
			return fmt.Errorf("用户取消操作")
		}

		chatBtnCount, err := chatBtn.Count()
		if err != nil {
			log.Printf("查找立即沟通按钮失败: %v", err)
			time.Sleep(1 * time.Second)
			continue
		}

		if chatBtnCount > 0 {
			// 获取按钮文本内容
			textContent, err := chatBtn.First().TextContent()
			if err != nil {
				log.Printf("获取按钮文本失败: %v", err)
				time.Sleep(1 * time.Second)
				continue
			}

			if strings.Contains(textContent, "立即沟通") {
				if err := chatBtn.First().Click(); err != nil {
					log.Printf("点击立即沟通按钮失败: %v", err)
					time.Sleep(1 * time.Second)
					continue
				}
				return nil
			}
		}

		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("未找到立即沟通按钮")
}

func (b *BossWorker) generateGreetingMessage(keyword string, job *JobDetail) string {
	if !b.config.EnableAI || job.JobInfo == "" {
		return b.config.SayHi
	}

	message, err := b.aiService.GenerateGreeting("", keyword, job.JobName, job.JobInfo, b.config.SayHi)
	if err != nil {
		log.Printf("AI生成招呼语失败: %v", err)
		return b.config.SayHi
	}

	return message
}

// 发送聊天消息的函数也需要调整
func (b *BossWorker) sendChatMessage(detailPage playwright.Page, message string) error {
	// 定位聊天输入框
	inputLocator := detailPage.Locator("div#chat-input.chat-input[contenteditable='true'], textarea.input-area")

	for i := 0; i < 10; i++ {
		if b.shouldStop() {
			return fmt.Errorf("用户取消操作")
		}

		inputCount, err := inputLocator.Count()
		if err != nil {
			log.Printf("查找输入框失败: %v", err)
			time.Sleep(1 * time.Second)
			continue
		}

		if inputCount > 0 {
			isVisible, err := inputLocator.First().IsVisible()
			if err != nil {
				log.Printf("检查输入框可见性失败: %v", err)
				time.Sleep(1 * time.Second)
				continue
			}

			if isVisible {
				break
			}
		}
		time.Sleep(1 * time.Second)
	}

	inputCount, err := inputLocator.Count()
	if err != nil || inputCount == 0 {
		return fmt.Errorf("聊天输入框未出现")
	}

	input := inputLocator.First()
	if err := input.Click(); err != nil {
		return fmt.Errorf("点击输入框失败: %w", err)
	}

	// 判断输入框类型并输入文本
	tagName, err := input.Evaluate("el => el.tagName.toLowerCase()", nil)
	if err != nil {
		return fmt.Errorf("获取输入框类型失败: %w", err)
	}

	if tagName == "textarea" {
		if err := input.Fill(message); err != nil {
			return fmt.Errorf("填写消息失败: %w", err)
		}
	} else {
		if _, err := input.Evaluate(`(el, msg) => { 
            el.innerText = msg; 
            el.dispatchEvent(new Event('input')); 
        }`, message); err != nil {
			return fmt.Errorf("设置contenteditable内容失败: %w", err)
		}
	}

	// 点击发送按钮
	sendBtn := detailPage.Locator("div.send-message, button[type='send'].btn-send, button.btn-send")
	sendBtnCount, err := sendBtn.Count()
	if err != nil {
		return fmt.Errorf("查找发送按钮失败: %w", err)
	}

	if sendBtnCount > 0 {
		if err := sendBtn.First().Click(); err != nil {
			return fmt.Errorf("点击发送按钮失败: %w", err)
		}
		time.Sleep(1 * time.Second)

		// 尝试关闭可能的弹窗
		closeBtn := detailPage.Locator("i.icon-close")
		closeBtnCount, err := closeBtn.Count()
		if err == nil && closeBtnCount > 0 {
			closeBtn.First().Click()
		}
		return nil
	}

	return fmt.Errorf("未找到发送按钮")
}

// core/utils.go
func (b *BossWorker) buildSearchURL(cityCode, keyword string) string {
	baseURL := "https://www.zhipin.com/web/geek/job?"
	params := url.Values{}

	params.Add("city", cityCode)
	params.Add("query", url.QueryEscape(keyword))

	if b.config.JobType != "" {
		params.Add("jobType", b.config.JobType)
	}

	// 添加其他搜索参数...
	if len(b.config.Salary) > 0 {
		params.Add("salary", strings.Join(b.config.Salary, ","))
	}
	if len(b.config.Experience) > 0 {
		params.Add("experience", strings.Join(b.config.Experience, ","))
	}
	if len(b.config.Degree) > 0 {
		params.Add("degree", strings.Join(b.config.Degree, ","))
	}

	return baseURL + params.Encode()
}

// 滚动加载所有职位的函数也需要调整
func (b *BossWorker) scrollToLoadAllJobs() error {
	lastCount := -1
	stableTries := 0

	for i := 0; i < 120; i++ {
		if b.shouldStop() {
			return nil
		}

		// 检查是否到达底部
		footer := b.page.Locator("div#footer, #footer")
		footerCount, err := footer.Count()
		if err != nil {
			log.Printf("查找底部元素失败: %v", err)
			continue
		}

		if footerCount > 0 {
			isVisible, err := footer.First().IsVisible()
			if err == nil && isVisible {
				break
			}
		}

		// 滚动页面
		if _, err := b.page.Evaluate("() => window.scrollBy(0, Math.floor(window.innerHeight * 1.5))"); err != nil {
			log.Printf("滚动页面失败: %v", err)
		}

		// 检查卡片数量变化
		cards := b.page.Locator("ul.rec-job-list li.job-card-box")
		currentCount, err := cards.Count()
		if err != nil {
			log.Printf("获取卡片数量失败: %v", err)
			continue
		}

		if currentCount == lastCount {
			stableTries++
		} else {
			stableTries = 0
		}
		lastCount = currentCount

		// 连续无新增则强制触底
		if stableTries >= 3 {
			if _, err := b.page.Evaluate("() => window.scrollTo(0, document.body.scrollHeight)"); err != nil {
				log.Printf("强制滚动到底部失败: %v", err)
			}
		}

		time.Sleep(500 * time.Millisecond)
	}

	return nil
}

func (b *BossWorker) shouldStop() bool {
	select {
	case <-b.stopChan:
		return true
	default:
		return false
	}
}

func (b *BossWorker) sendProgress(message string, current, total int) {
	select {
	case b.progressChan <- ProgressMessage{Message: message, Current: current, Total: total}:
	default:
		// 如果channel满了，跳过发送
	}
}

func (b *BossWorker) Stop() {
	close(b.stopChan)
}

// core/chat_utils.go

// 等待聊天输入框出现
func (b *BossWorker) waitForChatInput(detailPage playwright.Page) error {
	inputLocator := detailPage.Locator("div#chat-input.chat-input[contenteditable='true'], textarea.input-area")

	for i := 0; i < 10; i++ {
		if b.shouldStop() {
			return fmt.Errorf("用户取消操作")
		}

		inputCount, err := inputLocator.Count()
		if err != nil {
			log.Printf("查找聊天输入框失败: %v", err)
			time.Sleep(1 * time.Second)
			continue
		}

		if inputCount > 0 {
			isVisible, err := inputLocator.First().IsVisible()
			if err != nil {
				log.Printf("检查输入框可见性失败: %v", err)
				time.Sleep(1 * time.Second)
				continue
			}

			if isVisible {
				log.Printf("聊天输入框已就绪")
				return nil
			}
		}

		log.Printf("等待聊天输入框出现... (%d/10)", i+1)
		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("聊天输入框未在指定时间内出现")
}

// 发送图片简历
func (b *BossWorker) sendImageResume(detailPage playwright.Page) bool {
	log.Printf("开始发送图片简历...")

	// 0) 检查图片文件是否存在
	imagePath, err := b.resolveResumeImagePath()
	if err != nil {
		log.Printf("图片简历文件不存在: %v", err)
		return false
	}

	// 1) 确保在聊天页面
	currentURL := detailPage.URL()

	if !strings.Contains(currentURL, "/web/geek/chat") {
		log.Printf("当前不在聊天页面，尝试进入聊天页面")
		if err := b.navigateToChatPage(detailPage); err != nil {
			log.Printf("进入聊天页面失败: %v", err)
			return false
		}
	}

	// 2) 查找图片发送按钮
	imgContainer := detailPage.Locator("div.btn-sendimg[aria-label='发送图片'], div[aria-label='发送图片'].btn-sendimg")
	imgContainerCount, err := imgContainer.Count()
	if err != nil {
		log.Printf("查找图片发送容器失败: %v", err)
		return false
	}

	if imgContainerCount == 0 {
		log.Printf("未找到图片发送按钮")
		return false
	}

	// 3) 查找文件输入框
	imageInput := imgContainer.Locator("input[type='file'][accept*='image']").First()
	imageInputCount, err := imageInput.Count()
	if err != nil {
		log.Printf("查找图片输入框失败: %v", err)
		return false
	}

	// 4) 如果文件输入框不存在，尝试点击触发
	if imageInputCount == 0 {
		log.Printf("图片输入框不存在，尝试点击触发")
		if err := b.triggerImageInput(detailPage, imgContainer, imagePath); err != nil {
			log.Printf("触发图片输入失败: %v", err)
			return false
		}
		return true
	}

	// 5) 直接设置文件
	log.Printf("找到图片输入框，直接上传图片")
	if err := imageInput.SetInputFiles([]string{imagePath}); err != nil {
		log.Printf("上传图片失败: %v", err)
		return false
	}

	log.Printf("图片简历发送成功")
	time.Sleep(2 * time.Second) // 等待上传完成
	return true
}

// 解析图片简历路径
func (b *BossWorker) resolveResumeImagePath() (string, error) {
	// 优先检查当前目录下的 resume.jpg
	possiblePaths := []string{
		"./resume.jpg",
		"./resources/resume.jpg",
		"./static/resume.jpg",
		"./images/resume.jpg",
	}

	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			absPath, err := filepath.Abs(path)
			if err != nil {
				continue
			}
			log.Printf("找到图片简历: %s", absPath)
			return absPath, nil
		}
	}

	// 如果本地文件不存在，检查是否在二进制文件同目录
	exePath, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exePath)
		exeResumePath := filepath.Join(exeDir, "resume.jpg")
		if _, err := os.Stat(exeResumePath); err == nil {
			log.Printf("找到图片简历: %s", exeResumePath)
			return exeResumePath, nil
		}
	}

	return "", fmt.Errorf("未找到 resume.jpg 图片文件，请将图片放置在程序同目录下")
}

// 导航到聊天页面
func (b *BossWorker) navigateToChatPage(detailPage playwright.Page) error {
	chatBtn := detailPage.Locator("a.btn-startchat, a.op-btn-chat")
	chatBtnCount, err := chatBtn.Count()
	if err != nil {
		return fmt.Errorf("查找聊天按钮失败: %w", err)
	}

	if chatBtnCount == 0 {
		return fmt.Errorf("未找到聊天按钮")
	}

	if err := chatBtn.First().Click(); err != nil {
		return fmt.Errorf("点击聊天按钮失败: %w", err)
	}

	// 等待页面跳转到聊天页面
	_, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	err = detailPage.WaitForURL("**/web/geek/chat**", playwright.PageWaitForURLOptions{
		Timeout: playwright.Float(15000),
	})
	if err != nil {
		return fmt.Errorf("等待跳转到聊天页面超时: %w", err)
	}

	log.Printf("成功进入聊天页面")
	return nil
}

// 触发图片输入（处理文件选择器）
func (b *BossWorker) triggerImageInput(detailPage playwright.Page, imgContainer playwright.Locator, imagePath string) error {
	// 设置文件选择器监听
	fileChooserChan := make(chan playwright.FileChooser, 1)
	detailPage.OnFileChooser(func(chooser playwright.FileChooser) {
		select {
		case fileChooserChan <- chooser:
		default:
		}
	})

	// 点击图片按钮
	if err := imgContainer.First().Click(); err != nil {
		return fmt.Errorf("点击图片按钮失败: %w", err)
	}

	// 等待文件选择器出现
	select {
	case chooser := <-fileChooserChan:
		log.Printf("检测到文件选择器，直接设置文件")
		if err := chooser.SetFiles([]string{imagePath}); err != nil {
			return fmt.Errorf("设置文件失败: %w", err)
		}
		log.Printf("通过文件选择器上传图片成功")
		return nil

	case <-time.After(3 * time.Second):
		log.Printf("未检测到文件选择器，尝试查找输入框")
		// 重新查找输入框
		imageInput := imgContainer.Locator("input[type='file'][accept*='image']").First()
		imageInputCount, err := imageInput.Count()
		if err != nil {
			return fmt.Errorf("重新查找输入框失败: %w", err)
		}

		if imageInputCount > 0 {
			if err := imageInput.SetInputFiles([]string{imagePath}); err != nil {
				return fmt.Errorf("上传图片失败: %w", err)
			}
			log.Printf("通过输入框上传图片成功")
			return nil
		}

		return fmt.Errorf("无法找到图片上传方式")
	}
}

// 在 resumeSubmission 函数中补充调用
func (b *BossWorker) resumeSubmission(ctx context.Context, job *JobDetail, keyword string) error {
	if b.config.Debugger {
		log.Printf("调试模式：仅遍历岗位，不投递 | 公司：%s | 岗位：%s",
			job.CompanyName, job.JobName)
		return nil
	}

	// 1. 打开详情页
	detailPage, detailURL, err := b.openJobDetailPage()
	if err != nil {
		return fmt.Errorf("打开详情页失败: %w DetailURL: %s", err, detailURL)
	}
	defer detailPage.Close()

	// 2. 点击立即沟通按钮
	if err := b.clickChatButton(detailPage); err != nil {
		return fmt.Errorf("点击沟通按钮失败: %w", err)
	}

	// 3. 等待聊天输入框（新增调用）
	if err := b.waitForChatInput(detailPage); err != nil {
		return fmt.Errorf("等待聊天输入框失败: %w", err)
	}

	// 4. 生成并发送招呼语
	message := b.generateGreetingMessage(keyword, job)
	if err := b.sendChatMessage(detailPage, message); err != nil {
		return fmt.Errorf("发送招呼语失败: %w", err)
	}

	// 5. 发送图片简历（可选，新增调用）
	var imgResumeSent bool
	if b.config.SendImgResume {
		imgResumeSent = b.sendImageResume(detailPage)
	}

	log.Printf("投递完成 | 公司：%s | 岗位：%s | 薪资：%s | 招呼语：%s | 图片简历：%v",
		job.CompanyName, job.JobName, job.Salary, message, imgResumeSent)

	return nil
}
