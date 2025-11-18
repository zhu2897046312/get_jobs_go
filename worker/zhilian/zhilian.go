package zhilian

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	log "github.com/sirupsen/logrus"

	"get_jobs_go/config"
)

const (
	loginURL = "https://passport.zhaopin.com/login"
	homeURL  = "https://sou.zhaopin.com/?"
	maxPage  = 50 // 默认最大页数
)

type Job struct {
	JobName     string `json:"jobName"`
	Salary      string `json:"salary"`
	CompanyName string `json:"companyName"`
	CompanyTag  string `json:"companyTag"`
	JobInfo     string `json:"jobInfo"`
}

type ZhiLian struct {
	config     *config.ZhilianConfig
	ctx        context.Context
	cancel     context.CancelFunc
	isLimit    bool
	resultList []Job
	startTime  time.Time
}

func New(config *config.ZhilianConfig) *ZhiLian {
	ctx, cancel := chromedp.NewExecAllocator(context.Background(),
		append(chromedp.DefaultExecAllocatorOptions[:],
			chromedp.Flag("headless", false),
			chromedp.Flag("disable-gpu", true),
			chromedp.Flag("no-sandbox", true),
		)...)

	ctx, cancel = chromedp.NewContext(ctx)
	return &ZhiLian{
		config:     config,
		ctx:        ctx,
		cancel:     cancel,
		resultList: make([]Job, 0),
		startTime:  time.Now(),
	}
}

func (z *ZhiLian) Run() error {
	defer z.cancel()

	if err := z.login(); err != nil {
		return fmt.Errorf("login failed: %v", err)
	}

	for _, keyword := range z.config.Keywords {
		if z.isLimit {
			break
		}

		if err := z.submitJobs(keyword); err != nil {
			log.Errorf("submit jobs for keyword %s failed: %v", keyword, err)
			continue
		}
	}

	z.printResult()
	return nil
}

func (z *ZhiLian) login() error {
	if err := chromedp.Run(z.ctx, chromedp.Navigate(loginURL)); err != nil {
		return err
	}

	// 尝试加载cookie
	if err := z.loadCookie(); err == nil {
		if err := chromedp.Run(z.ctx, chromedp.Reload()); err != nil {
			return err
		}
		time.Sleep(time.Second)
	}

	// 检查是否需要登录
	var currentURL string
	if err := chromedp.Run(z.ctx, chromedp.Location(&currentURL)); err != nil {
		return err
	}

	if !strings.Contains(currentURL, "i.zhaopin.com") {
		if err := z.scanLogin(); err != nil {
			return err
		}
	}

	return nil
}

func (z *ZhiLian) scanLogin() error {
	log.Info("等待扫码登录中...")
	err := chromedp.Run(z.ctx,
		chromedp.Click(`//div[@class='zppp-panel-normal-bar__img']`),
		chromedp.WaitVisible(`//div[@class='zp-main__personal']`),
	)
	if err != nil {
		return fmt.Errorf("scan login failed: %v", err)
	}

	log.Info("扫码登录成功！")
	return z.saveCookie()
}

func (z *ZhiLian) loadCookie() error {
	data, err := os.ReadFile("./zhilian/cookie.json")
	if err != nil {
		return err
	}

	var cookies []*network.Cookie
	if err := json.Unmarshal(data, &cookies); err != nil {
		return err
	}

	// Convert []*network.Cookie to []*network.CookieParam
	cookieParams := make([]*network.CookieParam, len(cookies))
	for i, cookie := range cookies {
		cookieParams[i] = &network.CookieParam{
			Name:     cookie.Name,
			Value:    cookie.Value,
			Domain:   cookie.Domain,
			Path:     cookie.Path,
			Secure:   cookie.Secure,
			HTTPOnly: cookie.HTTPOnly,
		}
	}

	return chromedp.Run(z.ctx, network.SetCookies(cookieParams))
}

func (z *ZhiLian) saveCookie() error {
	var cookies []*network.Cookie
	if err := chromedp.Run(z.ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		var err error
		cookies, err = network.GetCookies().Do(ctx)
		if err != nil {
			return fmt.Errorf("get cookies failed: %v", err)
		}
		return nil
	})); err != nil {
		return err
	}

	data, err := json.Marshal(cookies)
	if err != nil {
		return fmt.Errorf("marshal cookies failed: %v", err)
	}

	if err := os.WriteFile("./zhilian/cookie.json", data, 0644); err != nil {
		return fmt.Errorf("write cookie file failed: %v", err)
	}

	log.Info("Cookies saved successfully!")
	return nil
}

func (z *ZhiLian) getSearchURL(keyword string, page int) string {
	params := []string{
		fmt.Sprintf("jl=%s", z.config.CityCode),
		fmt.Sprintf("kw=%s", keyword),
		fmt.Sprintf("sl=%s", z.config.Salary),
		fmt.Sprintf("p=%d", page),
	}
	return homeURL + strings.Join(params, "&")
}

func (z *ZhiLian) submitJobs(keyword string) error {
	url := z.getSearchURL(keyword, 1)
	if err := chromedp.Run(z.ctx, chromedp.Navigate(url)); err != nil {
		return err
	}

	// 等待岗位列表加载
	if err := chromedp.Run(z.ctx,
		chromedp.WaitVisible(`//div[contains(@class, 'joblist-box__item')]`),
	); err != nil {
		return err
	}

	for page := 1; page <= maxPage; page++ {
		if page != 1 {
			url = z.getSearchURL(keyword, page)
			if err := chromedp.Run(z.ctx, chromedp.Navigate(url)); err != nil {
				return err
			}
		}

		log.Infof("开始投递【%s】关键词，第【%d】页...", keyword, page)

		// 等待岗位列表加载并全选
		err := chromedp.Run(z.ctx,
			chromedp.WaitVisible(`//div[@class='positionlist']`),
			chromedp.Click(`//i[@class='betch__checkall__checkbox']`),
			chromedp.Click(`//button[@class='betch__button']`),
		)
		if err != nil {
			log.Errorf("page %d operation failed: %v", page, err)
			continue
		}

		if z.checkIsLimit() {
			return nil
		}

		// 处理投递结果
		if err := z.handleDeliveryResult(); err != nil {
			log.Errorf("handle delivery result failed: %v", err)
		}
	}

	return nil
}

func (z *ZhiLian) checkIsLimit() bool {
	//time.Sleep(500 * time.Millisecond)
	var text string
	err := chromedp.Run(z.ctx,
		chromedp.Text(`//div[@class='a-job-apply-workflow']`, &text),
	)
	if err == nil && strings.Contains(text, "达到上限") {
		log.Info("今日投递已达上限！")
		z.isLimit = true
		return true
	}
	return false
}

func (z *ZhiLian) handleDeliveryResult() error {
	// 处理投递成功弹窗
	var text string
	err := chromedp.Run(z.ctx,
		chromedp.Text(`//div[@class='deliver-dialog']`, &text),
	)
	if err == nil && strings.Contains(text, "申请成功") {
		log.Info("岗位申请成功！")
	}

	// 关闭弹窗
	if err := chromedp.Run(z.ctx,
		chromedp.Click(`//img[@title='close-icon']`),
	); err != nil {
		if z.checkIsLimit() {
			return nil
		}
	}

	// 处理相似职位推荐
	return z.handleRecommendJobs()
}

func (z *ZhiLian) handleRecommendJobs() error {
	var jobs []Job
	err := chromedp.Run(z.ctx,
		chromedp.Click(`//div[contains(@class, 'applied-select-all')]//input`),
		chromedp.Click(`//div[contains(@class, 'applied-select-all')]//button`),
		chromedp.Evaluate(`
			Array.from(document.querySelectorAll('.recommend-job')).map(j => ({
				jobName: j.querySelector('.recommend-job__position').textContent,
				salary: j.querySelector('.recommend-job__demand__salary').textContent,
				companyName: j.querySelector('.recommend-job__cname').textContent,
				companyTag: j.querySelector('.recommend-job__demand__cinfo').textContent.replace(/\n/g, ' '),
				jobInfo: j.querySelector('.recommend-job__demand__experience').textContent.replace(/\n/g, ' ') + 
					'·' + j.querySelector('.recommend-job__demand__educational').textContent.replace(/\n/g, ' ')
			}))
		`, &jobs),
	)
	if err != nil {
		return fmt.Errorf("handle recommend jobs failed: %v", err)
	}

	for _, job := range jobs {
		log.Infof("投递【%s】公司【%s】岗位，薪资【%s】，要求【%s】，规模【%s】",
			job.CompanyName, job.JobName, job.Salary, job.JobInfo, job.CompanyTag)
		z.resultList = append(z.resultList, job)
	}

	return nil
}

func (z *ZhiLian) printResult() {
	duration := time.Since(z.startTime)
	if len(z.resultList) == 0 {
		log.Info("未投递新的岗位...")
	} else {
		log.Info("新投递公司如下:")
		for _, job := range z.resultList {
			log.Info(job)
		}
	}

	message := fmt.Sprintf("\n智联招聘投递完成，共投递%d个岗位，用时%s",
		len(z.resultList), duration.Round(time.Second))
	log.Info(message)
}