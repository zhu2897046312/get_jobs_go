package boss

import (
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"get_jobs_go/config"
	"get_jobs_go/service"
	"get_jobs_go/utils"

	"github.com/playwright-community/playwright-go"
)

// Boss 结构体对应Java的Boss类
type Boss struct {
	page              playwright.Page
	config            *config.BossConfig
	bossService       *service.BossService
	aiService         *service.AiService
	blackCompanies    map[string]bool
	blackRecruiters   map[string]bool
	blackJobs         map[string]bool
	encryptIdToUserId sync.Map
	progressCallback  ProgressCallback
	shouldStopCallback func() bool
	resultList        []*utils.Job
	mu                sync.RWMutex
}

// ProgressCallback 进度回调函数类型
type ProgressCallback func(message string, current, total int)

// NewBoss 创建Boss实例
func NewBoss(
	bossService *service.BossService,
	aiService *service.AiService,
) *Boss {
	return &Boss{
		bossService:     bossService,
		aiService:       aiService,
		blackCompanies:  make(map[string]bool),
		blackRecruiters: make(map[string]bool),
		blackJobs:       make(map[string]bool),
		resultList:      make([]*utils.Job, 0),
	}
}

// SetPage 设置Playwright页面
func (b *Boss) SetPage(page playwright.Page) {
	b.page = page
}

// SetConfig 设置配置
func (b *Boss) SetConfig(config *config.BossConfig) {
	b.config = config
}

// SetProgressCallback 设置进度回调
func (b *Boss) SetProgressCallback(callback ProgressCallback) {
	b.progressCallback = callback
}

// SetShouldStopCallback 设置停止回调
func (b *Boss) SetShouldStopCallback(callback func() bool) {
	b.shouldStopCallback = callback
}

// Prepare 准备阶段：加载黑名单
func (b *Boss) Prepare() error {
	// 从数据库加载黑名单
	blackCompanies, err := b.bossService.GetBlackCompanies()
	if err != nil {
		return fmt.Errorf("加载公司黑名单失败: %v", err)
	}
	b.blackCompanies = blackCompanies

	blackRecruiters, err := b.bossService.GetBlackRecruiters()
	if err != nil {
		return fmt.Errorf("加载招聘者黑名单失败: %v", err)
	}
	b.blackRecruiters = blackRecruiters

	blackJobs, err := b.bossService.GetBlackJobs()
	if err != nil {
		return fmt.Errorf("加载职位黑名单失败: %v", err)
	}
	b.blackJobs = blackJobs

	log.Printf("黑名单加载完成: 公司(%d) 招聘者(%d) 职位(%d)",
		len(b.blackCompanies), len(b.blackRecruiters), len(b.blackJobs))

	return nil
}

// Execute 执行投递
func (b *Boss) Execute() int {
	if b.shouldStopCallback != nil && b.shouldStopCallback() {
		b.progressCallback("用户取消投递", 0, 0)
		return 0
	}

	totalCount := 0
	for _, cityCode := range b.config.CityCode {
		if b.shouldStopCallback != nil && b.shouldStopCallback() {
			b.progressCallback("用户取消投递", 0, 0)
			break
		}

		count := b.postJobByCity(cityCode)
		totalCount += count

		if b.shouldStopCallback != nil && b.shouldStopCallback() {
			b.progressCallback("用户取消投递", 0, 0)
			break
		}
	}

	return totalCount
}

// GetResultList 获取结果列表
func (b *Boss) GetResultList() []*utils.Job {
	b.mu.RLock()
	defer b.mu.RUnlock()
	
	result := make([]*utils.Job, len(b.resultList))
	copy(result, b.resultList)
	return result
}

// postJobByCity 按城市投递
func (b *Boss) postJobByCity(cityCode string) int {
	searchUrl := b.getSearchUrl(cityCode)
	totalPostCount := 0

	for _, keyword := range b.config.Keywords {
		if b.shouldStopCallback != nil && b.shouldStopCallback() {
			return totalPostCount
		}

		postCount := b.postJobsByKeyword(searchUrl, keyword)
		totalPostCount += postCount
		log.Printf("【%s】岗位已投递完毕！已投递岗位数量: %d", keyword, postCount)
	}

	return totalPostCount
}

// postJobsByKeyword 按关键词投递
func (b *Boss) postJobsByKeyword(searchUrl, keyword string) int {
	encodedKeyword := url.QueryEscape(keyword)
	fullUrl := searchUrl + "&query=" + encodedKeyword

	// 导航到搜索页面
	_, err := b.page.Goto(fullUrl, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
		Timeout:   playwright.Float(15000),
	})
	if err != nil {
		log.Printf("导航到搜索页面失败: %v", err)
		return 0
	}

	// 等待列表容器出现
	_, err = b.page.WaitForSelector("//ul[contains(@class, 'rec-job-list')]", playwright.PageWaitForSelectorOptions{
		Timeout: playwright.Float(60000),
	})
	if err != nil {
		log.Printf("等待岗位列表加载失败: %v", err)
		return 0
	}

	// 滚动加载所有岗位
	b.scrollToLoadAllJobs(keyword)

	// 获取最终岗位数量
	cards, err := b.page.QuerySelectorAll("//ul[contains(@class, 'rec-job-list')]//li[contains(@class, 'job-card-box')]")
	if err != nil {
		log.Printf("获取岗位卡片失败: %v", err)
		return 0
	}

	loadedCount := len(cards)
	log.Printf("【%s】岗位已全部加载，总数: %d", keyword, loadedCount)
	b.progressCallback("岗位加载完成："+keyword, 0, loadedCount)

	// 回到页面顶部
	b.page.Evaluate("window.scrollTo(0, 0);")
	utils.Sleep(1)

	// 逐个处理岗位
	postCount := 0
	for i := 0; i < loadedCount; i++ {
		if b.shouldStopCallback != nil && b.shouldStopCallback() {
			b.progressCallback("用户取消投递", i, loadedCount)
			return postCount
		}

		// 重新获取卡片避免元素过期
		cards, err = b.page.QuerySelectorAll("//ul[contains(@class, 'rec-job-list')]//li[contains(@class, 'job-card-box')]")
		if err != nil || i >= len(cards) {
			continue
		}

		job, shouldSkip := b.processJobCard(cards[i], i, loadedCount)
		if shouldSkip {
			continue
		}

		// 投递简历
		b.progressCallback("正在投递："+job.JobName, i+1, loadedCount)
		success := b.resumeSubmission(keyword, job)
		if success {
			postCount++
		}

		// 滚动避免页面刷新问题
		if i >= 5 {
			b.page.Evaluate("window.scrollBy(0, 140);")
			utils.Sleep(1)
		}
	}

	return postCount
}

// processJobCard 处理单个岗位卡片
func (b *Boss) processJobCard(card playwright.ElementHandle, index, total int) (*utils.Job, bool) {
	var detailResp *playwright.Response
	
	// 点击卡片并等待详情响应
	if index == 0 && total > 1 {
		// 第一个卡片特殊处理
		secondCard, err := b.page.QuerySelector("//ul[contains(@class, 'rec-job-list')]//li[contains(@class, 'job-card-box')]")
		if err == nil && secondCard != nil {
			secondCard.Click()
			utils.Sleep(1)
		}
	}

	// 设置响应监听
	responseChan := make(chan *playwright.Response, 1)
	b.page.OnResponse(func(response playwright.Response) {
		url := response.URL()
		if strings.Contains(url, "/wapi/zpgeek/job/detail.json") && 
		   strings.EqualFold(response.Request().Method(), "GET") {
			select {
			case responseChan <- &response:
			default:
			}
		}
	})

	// 点击当前卡片
	card.Click()
	
	// 等待响应或超时
	select {
	case detailResp = <-responseChan:
	case <-time.After(5 * time.Second):
	}

	b.page.RemoveListeners("response")

	// 解析岗位详情
	job, shouldSkip := b.parseJobDetail(detailResp)
	return job, shouldSkip
}

// parseJobDetail 解析岗位详情
func (b *Boss) parseJobDetail(detailResp *playwright.Response) (*utils.Job, bool) {
	if detailResp == nil {
		return nil, true
	}

	body, err := (*detailResp).Body()
	if err != nil {
		return nil, true
	}

	// 解析JSON响应
	var responseData map[string]interface{}
	if err := json.Unmarshal(body, &responseData); err != nil {
		return nil, true
	}

	// 提取岗位信息
	zpData, _ := responseData["zpData"].(map[string]interface{})
	if zpData == nil {
		return nil, true
	}

	jobInfo, _ := zpData["jobInfo"].(map[string]interface{})
	brandInfo, _ := zpData["brandComInfo"].(map[string]interface{})
	bossInfo, _ := zpData["bossInfo"].(map[string]interface{})

	// 构建Job对象
	job := &utils.Job{
		JobName:    b.getStringValue(jobInfo, "jobName"),
		Salary:     b.getStringValue(jobInfo, "salaryDesc"),
		CompanyName: b.getStringValue(brandInfo, "brandName"),
		Recruiter:  b.getStringValue(bossInfo, "name"),
		JobInfo:    b.getStringValue(jobInfo, "postDescription"),
	}

	// 构建工作地区
	var tags []string
	if location := b.getStringValue(jobInfo, "locationName"); location != "" {
		tags = append(tags, location)
	}
	if experience := b.getStringValue(jobInfo, "experienceName"); experience != "" {
		tags = append(tags, experience)
	}
	if degree := b.getStringValue(jobInfo, "degreeName"); degree != "" {
		tags = append(tags, degree)
	}
	job.JobArea = strings.Join(tags, ", ")

	// 过滤检查
	if b.shouldFilterJob(job, bossInfo) {
		return nil, true
	}

	return job, false
}

// shouldFilterJob 检查是否应该过滤该岗位
func (b *Boss) shouldFilterJob(job *utils.Job, bossInfo map[string]interface{}) bool {
	// 职位黑名单过滤
	if b.isInBlacklist(job.JobName, b.blackJobs) {
		log.Printf("被过滤：职位黑名单命中 | 公司：%s | 岗位：%s", job.CompanyName, job.JobName)
		return true
	}

	// HR活跃状态过滤
	if b.config.FilterDeadHR {
		activeTime := b.getStringValue(bossInfo, "activeTimeDesc")
		if strings.Contains(activeTime, "年") {
			log.Printf("被过滤：HR活跃状态包含'年' | 公司：%s | 岗位：%s | 活跃：%s", 
				job.CompanyName, job.JobName, activeTime)
			return true
		}
	}

	// 公司黑名单过滤
	if b.isInBlacklist(job.CompanyName, b.blackCompanies) {
		log.Printf("被过滤：公司黑名单命中 | 公司：%s | 岗位：%s", job.CompanyName, job.JobName)
		return true
	}

	// 招聘者黑名单过滤
	hrPosition := b.getStringValue(bossInfo, "title")
	if b.isInBlacklist(hrPosition, b.blackRecruiters) {
		log.Printf("被过滤：招聘者黑名单命中 | 公司：%s | 岗位：%s | 招聘者：%s", 
			job.CompanyName, job.JobName, hrPosition)
		return true
	}

	return false
}

// isInBlacklist 检查是否在黑名单中
func (b *Boss) isInBlacklist(value string, blacklist map[string]bool) bool {
	for blackItem := range blacklist {
		if strings.Contains(value, blackItem) {
			return true
		}
	}
	return false
}

// resumeSubmission 投递简历
func (b *Boss) resumeSubmission(keyword string, job *utils.Job) bool {
	if b.shouldStopCallback != nil && b.shouldStopCallback() {
		log.Printf("停止指令已触发，跳过投递 | 公司：%s | 岗位：%s", job.CompanyName, job.JobName)
		return false
	}

	if b.config.Debugger {
		log.Printf("调试模式：仅遍历岗位，不投递 | 公司：%s | 岗位：%s", job.CompanyName, job.JobName)
		return false
	}

	// 查找"查看更多信息"按钮
	moreInfoBtn, err := b.page.QuerySelector("a.more-job-btn")
	if err != nil || moreInfoBtn == nil {
		log.Printf("未找到'查看更多信息'按钮，跳过...")
		return false
	}

	href, err := moreInfoBtn.GetAttribute("href")
	if err != nil || !strings.HasPrefix(href, "/job_detail/") {
		log.Printf("未获取到岗位详情链接，跳过...")
		return false
	}

	detailUrl := "https://www.zhipin.com" + href
	
	// 在新页面打开详情
	context := b.page.Context()
	newPage, err := context.NewPage()
	if err != nil {
		log.Printf("创建新页面失败: %v", err)
		return false
	}
	defer newPage.Close()

	_, err = newPage.Goto(detailUrl, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
	})
	if err != nil {
		log.Printf("导航到详情页失败: %v", err)
		return false
	}

	// 查找立即沟通按钮
	chatBtn, found := b.waitForChatButton(newPage)
	if !found {
		log.Printf("未找到立即沟通按钮，跳过岗位: %s", job.JobName)
		return false
	}

	chatBtn.Click()
	utils.Sleep(1)

	// 等待聊天输入框
	inputLocator, inputReady := b.waitForChatInput(newPage)
	if !inputReady {
		log.Printf("聊天输入框未出现，跳过: %s", job.JobName)
		return false
	}

	// 生成并发送消息
	message := b.generateMessage(keyword, job)
	b.sendChatMessage(newPage, inputLocator, message)

	// 发送图片简历
	imgResume := false
	if b.config.SendImgResume {
		imgResume = b.sendImageResume(newPage)
	}

	log.Printf("投递完成 | 公司：%s | 岗位：%s | 薪资：%s | 招呼语：%s | 图片简历：%v", 
		job.CompanyName, job.JobName, job.Salary, message, imgResume)

	// 更新投递状态
	b.updateDeliveryStatus(detailUrl, job)
	
	b.mu.Lock()
	b.resultList = append(b.resultList, job)
	b.mu.Unlock()

	return true
}

// 等待聊天按钮
func (b *Boss) waitForChatButton(page playwright.Page) (playwright.ElementHandle, bool) {
	for i := 0; i < 5; i++ {
		if b.shouldStopCallback != nil && b.shouldStopCallback() {
			return nil, false
		}

		chatBtn, err := page.QuerySelector("a.btn-startchat, a.op-btn-chat")
		if err == nil && chatBtn != nil {
			text, _ := chatBtn.TextContent()
			if strings.Contains(text, "立即沟通") {
				return chatBtn, true
			}
		}
		utils.Sleep(1)
	}
	return nil, false
}

// 等待聊天输入框
func (b *Boss) waitForChatInput(page playwright.Page) (playwright.ElementHandle, bool) {
	for i := 0; i < 10; i++ {
		if b.shouldStopCallback != nil && b.shouldStopCallback() {
			return nil, false
		}

		inputLocator, err := page.QuerySelector("div#chat-input.chat-input[contenteditable='true'], textarea.input-area")
		if err == nil && inputLocator != nil {
			visible, _ := inputLocator.IsVisible()
			if visible {
				return inputLocator, true
			}
		}
		utils.Sleep(1)
	}
	return nil, false
}

// generateMessage 生成消息内容
func (b *Boss) generateMessage(keyword string, job *utils.Job) string {
	if b.config.EnableAI && job.JobInfo != "" {
		aiMessage, err := b.aiService.SendRequest(b.buildAIPrompt(keyword, job))
		if err == nil && aiMessage != "" && !strings.Contains(strings.ToLower(aiMessage), "false") {
			return aiMessage
		}
	}
	return b.config.SayHi
}

// buildAIPrompt 构建AI提示词
func (b *Boss) buildAIPrompt(keyword string, job *utils.Job) string {
	aiConfig, err := b.aiService.GetAiConfig()
	introduce := ""
	if err == nil && aiConfig != nil {
		introduce = aiConfig.Introduce
	}

	return fmt.Sprintf("请基于以下信息生成简洁友好的中文打招呼语，不超过60字：\n个人介绍：%s\n关键词：%s\n职位名称：%s\n职位描述：%s\n参考语：%s",
		introduce, keyword, job.JobName, job.JobInfo, b.config.SayHi)
}

// sendChatMessage 发送聊天消息
func (b *Boss) sendChatMessage(page playwright.Page, input playwright.ElementHandle, message string) {
	tagName, err := input.Evaluate("el => el.tagName.toLowerCase()", nil)
	if err == nil && tagName == "textarea" {
		input.Fill(message)
	} else {
		input.Evaluate(fmt.Sprintf(`(el, msg) => { 
			el.innerText = %q; 
			el.dispatchEvent(new Event('input')); 
		}`, message), nil)
	}

	// 点击发送按钮
	sendBtn, err := page.QuerySelector("div.send-message, button[type='send'].btn-send, button.btn-send")
	if err == nil && sendBtn != nil {
		sendBtn.Click()
		utils.Sleep(1)
		
		// 尝试关闭小窗口
		closeBtn, err := page.QuerySelector("i.icon-close")
		if err == nil && closeBtn != nil {
			closeBtn.Click()
		}
	}
}

// sendImageResume 发送图片简历
func (b *Boss) sendImageResume(page playwright.Page) bool {
	// 实现图片简历发送逻辑
	// 这里需要根据实际的文件路径和页面元素来实现
	log.Printf("发送图片简历功能待实现")
	return false
}

// updateDeliveryStatus 更新投递状态
func (b *Boss) updateDeliveryStatus(detailUrl string, job *utils.Job) {
	encryptId := b.extractEncryptId(detailUrl)
	if encryptId != "" {
		// 这里需要根据实际的数据库操作来实现状态更新
		log.Printf("更新投递状态 | 公司：%s | 岗位：%s | encryptId：%s", 
			job.CompanyName, job.JobName, encryptId)
	}
}

// extractEncryptId 从URL中提取encryptId
func (b *Boss) extractEncryptId(detailUrl string) string {
	key := "/job_detail/"
	idx := strings.Index(detailUrl, key)
	if idx < 0 {
		return ""
	}

	start := idx + len(key)
	end := strings.Index(detailUrl[start:], ".html")
	if end < 0 {
		return detailUrl[start:]
	}
	return detailUrl[start : start+end]
}

// scrollToLoadAllJobs 滚动加载所有岗位
func (b *Boss) scrollToLoadAllJobs(keyword string) {
	lastCount := -1
	stableTries := 0

	for i := 0; i < 5000; i++ {
		if b.shouldStopCallback != nil && b.shouldStopCallback() {
			return
		}

		// 检查是否到达底部
		footer, err := b.page.QuerySelector("div#footer, #footer")
		if err == nil && footer != nil {
			visible, _ := footer.IsVisible()
			if visible {
				break
			}
		}

		// 滚动页面
		b.page.Evaluate("() => window.scrollBy(0, Math.floor(window.innerHeight * 1.5))")

		// 检查卡片数量变化
		cards, err := b.page.QuerySelectorAll("//ul[contains(@class, 'rec-job-list')]//li[contains(@class, 'job-card-box')]")
		if err != nil {
			continue
		}

		currentCount := len(cards)
		if currentCount == lastCount {
			stableTries++
		} else {
			stableTries = 0
		}
		lastCount = currentCount

		if stableTries >= 3 {
			b.page.Evaluate("() => window.scrollTo(0, document.body.scrollHeight)")
		}
	}
}

// getSearchUrl 构建搜索URL
func (b *Boss) getSearchUrl(cityCode string) string {
	baseUrl := "https://www.zhipin.com/web/geek/job?"
	var params []string

	if cityCode != "" && cityCode != "0" {
		params = append(params, "city="+cityCode)
	}
	if b.config.JobType != "" && b.config.JobType != "0" {
		params = append(params, "jobType="+b.config.JobType)
	}
	if len(b.config.Salary) > 0 && b.config.Salary[0] != "0" {
		params = append(params, "salary="+strings.Join(b.config.Salary, ","))
	}
	if len(b.config.Experience) > 0 && b.config.Experience[0] != "0" {
		params = append(params, "experience="+strings.Join(b.config.Experience, ","))
	}
	if len(b.config.Degree) > 0 && b.config.Degree[0] != "0" {
		params = append(params, "degree="+strings.Join(b.config.Degree, ","))
	}
	if len(b.config.Scale) > 0 && b.config.Scale[0] != "0" {
		params = append(params, "scale="+strings.Join(b.config.Scale, ","))
	}
	if len(b.config.Industry) > 0 && b.config.Industry[0] != "0" {
		params = append(params, "industry="+strings.Join(b.config.Industry, ","))
	}
	if len(b.config.Stage) > 0 && b.config.Stage[0] != "0" {
		params = append(params, "stage="+strings.Join(b.config.Stage, ","))
	}

	return baseUrl + strings.Join(params, "&")
}

// getStringValue 安全获取字符串值
func (b *Boss) getStringValue(data map[string]interface{}, key string) string {
	if data == nil {
		return ""
	}
	value, exists := data[key]
	if !exists {
		return ""
	}
	if str, ok := value.(string); ok {
		return str
	}
	return ""
}

// 薪资解析相关方法
func (b *Boss) isSalaryNotExpected(salary string) bool {
	expectedSalary := b.config.ExpectedSalary
	if len(expectedSalary) == 0 {
		return false
	}

	// 清理薪资文本
	salary = b.removeYearBonusText(salary)
	if !b.isSalaryInExpectedFormat(salary) {
		return true
	}

	salary = b.cleanSalaryText(salary)
	jobType := b.detectJobType(salary)
	salary = b.removeDayUnitIfNeeded(salary)

	jobSalaryRange := b.parseSalaryRange(salary)
	return b.isSalaryOutOfRange(jobSalaryRange, expectedSalary, jobType)
}

func (b *Boss) removeYearBonusText(salary string) string {
	re := regexp.MustCompile(`·\d+薪`)
	return re.ReplaceAllString(salary, "")
}

func (b *Boss) isSalaryInExpectedFormat(salary string) bool {
	return strings.Contains(salary, "K") || strings.Contains(salary, "k") || strings.Contains(salary, "元/天")
}

func (b *Boss) cleanSalaryText(salary string) string {
	salary = strings.ReplaceAll(salary, "K", "")
	salary = strings.ReplaceAll(salary, "k", "")
	if idx := strings.Index(salary, "·"); idx != -1 {
		salary = salary[:idx]
	}
	return salary
}

func (b *Boss) detectJobType(salary string) string {
	if strings.Contains(salary, "元/天") {
		return "day"
	}
	return "month"
}

func (b *Boss) removeDayUnitIfNeeded(salary string) string {
	return strings.ReplaceAll(salary, "元/天", "")
}

func (b *Boss) parseSalaryRange(salary string) []int {
	parts := strings.Split(salary, "-")
	var result []int

	for _, part := range parts {
		// 移除非数字字符
		re := regexp.MustCompile(`[^0-9]`)
		cleanPart := re.ReplaceAllString(part, "")
		if num, err := strconv.Atoi(cleanPart); err == nil {
			result = append(result, num)
		}
	}

	return result
}

func (b *Boss) isSalaryOutOfRange(jobSalary []int, expectedSalary []int, jobType string) bool {
	if len(jobSalary) < 2 {
		return true
	}

	minExpected := expectedSalary[0]
	var maxExpected int
	if len(expectedSalary) > 1 {
		maxExpected = expectedSalary[1]
	} else {
		maxExpected = minExpected
	}

	if jobType == "day" {
		// 转换日薪为月薪进行比较
		minExpected = b.convertDailyToMonthly(minExpected)
		maxExpected = b.convertDailyToMonthly(maxExpected)
	}

	// 如果职位薪资下限低于期望的最低薪资，返回不符合
	if jobSalary[1] < minExpected {
		return true
	}

	// 如果职位薪资上限高于期望的最高薪资，返回不符合
	return len(expectedSalary) > 1 && jobSalary[0] > maxExpected
}

func (b *Boss) convertDailyToMonthly(dailySalary int) int {
	// 按21.75个工作日计算
	daily := big.NewFloat(float64(dailySalary))
	monthly := new(big.Float).Mul(daily, big.NewFloat(21.75))
	result, _ := monthly.Int64()
	return int(result)
}