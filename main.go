// main.go
package main

import (
    "context"
    "log"
    "time"
    
    "get_jobs_go/worker/boss"
	"get_jobs_go/application/service"
    "github.com/playwright-community/playwright-go"
)

func main() {
    // 加载配置
    cfg := &boss.BossConfig{
        SayHi:         "您好，我对这个职位很感兴趣，希望有机会沟通一下！",
        Keywords:      []string{"Golang", "后端开发"},
        CityCode:      []string{"101010100"}, // 北京
        EnableAI:      true,
        FilterDeadHR:  true,
        SendImgResume: false,
    }
    
    // 初始化服务
    dbService := &service.BossDBService{}
    aiService := &service.AIService{}
    
    worker := boss.NewBossWorker(cfg, dbService, aiService)
    
    // 启动浏览器
    pw, err := playwright.Run()
    if err != nil {
        log.Fatal(err)
    }
    defer pw.Stop()
    
    browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
        Headless: false,
    })
    if err != nil {
        log.Fatal(err)
    }
    defer browser.Close()
    
    page, err := browser.NewPage()
    if err != nil {
        log.Fatal(err)
    }
    
    worker.SetPage(page)
    
    // 准备（加载黑名单等）
    if err := worker.Prepare(); err != nil {
        log.Fatal(err)
    }
    
    // 监听进度
    go func() {
        for progress := range worker.Progress() {
            log.Printf("进度: %s (%d/%d)", progress.Message, progress.Current, progress.Total)
        }
    }()
    
    // 执行投递
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
    defer cancel()
    
    go func() {
        time.Sleep(10 * time.Minute) // 10分钟后自动停止
        worker.Stop()
    }()
    
    count, err := worker.Execute(ctx)
    if err != nil {
        log.Printf("投递执行失败: %v", err)
    } else {
        log.Printf("投递完成，共投递 %d 个岗位", count)
    }
}