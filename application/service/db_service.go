// internal/database/boss_db_service.go
package service

import (
    "fmt"
    "log"
    "strings"
    "time"

    "gorm.io/gorm"
)

// 保持与 boss.go 中完全一致的 DBService 接口
type DBService interface {
    GetBlacklists() (*Blacklist, error)
    SaveJob(job *JobDetail) error
    UpdateDeliveryStatus(encryptID, encryptUserID, status string) error
    JobExists(encryptID, encryptUserID string) bool  // 注意：这里返回 bool，不是 (bool, error)
}

// 扩展接口（内部使用）
type ExtendedDBService interface {
    DBService
    GetBossConfig() (*BossConfig, error)
    SaveOrUpdateConfig(config *BossConfigEntity) error
    GetOptionsByType(optionType string) ([]BossOptionEntity, error)
    AddBlacklist(blacklistType, value string) error
    RemoveBlacklist(blacklistType, value string) error
}

type BossDBService struct {
    db *gorm.DB
}

func NewBossDBService(db *gorm.DB) *BossDBService {
    return &BossDBService{db: db}
}

// 实现基础的 DBService 接口（与 boss.go 兼容）

// GetBlacklists 获取所有黑名单
func (s *BossDBService) GetBlacklists() (*Blacklist, error) {
    blacklist := &Blacklist{
        Companies:  []string{},
        Recruiters: []string{},
        Jobs:       []string{},
    }

    // 查询公司黑名单
    var companyBlacklists []BlacklistEntity
    if err := s.db.Where("type = ?", "company").Find(&companyBlacklists).Error; err != nil {
        return nil, fmt.Errorf("查询公司黑名单失败: %w", err)
    }
    for _, item := range companyBlacklists {
        blacklist.Companies = append(blacklist.Companies, item.Value)
    }

    // 查询招聘者黑名单
    var recruiterBlacklists []BlacklistEntity
    if err := s.db.Where("type = ?", "recruiter").Find(&recruiterBlacklists).Error; err != nil {
        return nil, fmt.Errorf("查询招聘者黑名单失败: %w", err)
    }
    for _, item := range recruiterBlacklists {
        blacklist.Recruiters = append(blacklist.Recruiters, item.Value)
    }

    // 查询职位黑名单
    var jobBlacklists []BlacklistEntity
    if err := s.db.Where("type = ?", "job").Find(&jobBlacklists).Error; err != nil {
        return nil, fmt.Errorf("查询职位黑名单失败: %w", err)
    }
    for _, item := range jobBlacklists {
        blacklist.Jobs = append(blacklist.Jobs, item.Value)
    }

    return blacklist, nil
}

// SaveJob 保存职位数据
func (s *BossDBService) SaveJob(job *JobDetail) error {
    if job == nil {
        return fmt.Errorf("职位数据为空")
    }

    // 检查是否已存在
    exists := s.JobExists(job.EncryptID, job.EncryptUserID)
    if exists {
        log.Printf("职位已存在，跳过保存: %s", job.JobName)
        return nil
    }

    entity := &BossJobDataEntity{
        EncryptID:      job.EncryptID,
        EncryptUserID:  job.EncryptUserID,
        CompanyName:    job.CompanyName,
        JobName:        job.JobName,
        Salary:         job.Salary,
        Location:       job.JobArea,
        Experience:     job.Experience,
        Degree:         job.Degree,
        HRName:         job.Recruiter,
        HRPosition:     job.HRPosition,
        HRActiveStatus: job.HRActiveStatus,
        DeliveryStatus: "未投递", // 默认状态
        JobDescription: job.JobInfo,
        JobURL:         fmt.Sprintf("https://www.zhipin.com/job_detail/%s.html", job.EncryptID),
        CreatedAt:      time.Now(),
        UpdatedAt:      time.Now(),
    }

    if err := s.db.Create(entity).Error; err != nil {
        return fmt.Errorf("保存职位数据失败: %w", err)
    }

    log.Printf("职位数据保存成功: %s | 公司: %s", job.JobName, job.CompanyName)
    return nil
}

// JobExists 检查职位是否已存在 - 注意：返回 bool 而不是 (bool, error)
func (s *BossDBService) JobExists(encryptID, encryptUserID string) bool {
    if encryptID == "" {
        return false
    }

    var count int64
    query := s.db.Model(&BossJobDataEntity{})
    
    if encryptUserID != "" {
        query = query.Where("encrypt_id = ? AND encrypt_user_id = ?", encryptID, encryptUserID)
    } else {
        query = query.Where("encrypt_id = ?", encryptID)
    }

    if err := query.Count(&count).Error; err != nil {
        log.Printf("查询职位存在性失败: %v", err)
        return false
    }

    return count > 0
}

// UpdateDeliveryStatus 更新投递状态
func (s *BossDBService) UpdateDeliveryStatus(encryptID, encryptUserID, status string) error {
    if encryptID == "" || status == "" {
        return fmt.Errorf("参数为空")
    }

    updateData := map[string]interface{}{
        "delivery_status": status,
        "updated_at":      time.Now(),
    }

    query := s.db.Model(&BossJobDataEntity{})
    if encryptUserID != "" {
        query = query.Where("encrypt_id = ? AND encrypt_user_id = ?", encryptID, encryptUserID)
    } else {
        query = query.Where("encrypt_id = ?", encryptID)
    }

    if err := query.Updates(updateData).Error; err != nil {
        return fmt.Errorf("更新投递状态失败: %w", err)
    }

    log.Printf("投递状态更新成功: %s -> %s", encryptID, status)
    return nil
}

// 扩展方法（用于其他组件）

// GetBossConfig 获取Boss配置
func (s *BossDBService) GetBossConfig() (*BossConfig, error) {
    var config BossConfigEntity
    if err := s.db.First(&config).Error; err != nil {
        if err == gorm.ErrRecordNotFound {
            // 返回默认配置
            return &BossConfig{
                SayHi:         "您好，我对这个职位很感兴趣，希望能进一步沟通",
                Debugger:      false,
                EnableAI:      false,
                FilterDeadHR:  true,
                SendImgResume: false,
                WaitTime:      "5",
            }, nil
        }
        return nil, fmt.Errorf("查询配置失败: %w", err)
    }
    
    // 转换为 BossConfig
    return s.entityToConfig(&config), nil
}

// SaveOrUpdateConfig 保存或更新配置
func (s *BossDBService) SaveOrUpdateConfig(config *BossConfigEntity) error {
    if config == nil {
        return fmt.Errorf("配置为空")
    }

    var existing BossConfigEntity
    err := s.db.First(&existing).Error
    
    now := time.Now()
    if err == gorm.ErrRecordNotFound {
        // 新增
        config.CreatedAt = now
        config.UpdatedAt = now
        return s.db.Create(config).Error
    } else if err != nil {
        return fmt.Errorf("查询现有配置失败: %w", err)
    }
    
    // 更新
    config.ID = existing.ID
    config.UpdatedAt = now
    return s.db.Save(config).Error
}

// GetOptionsByType 根据类型获取选项
func (s *BossDBService) GetOptionsByType(optionType string) ([]BossOptionEntity, error) {
    var options []BossOptionEntity
    
    query := s.db.Where("type = ?", optionType)
    
    // 特殊排序规则
    if optionType == "city" || optionType == "industry" {
        query = query.Order("sort_order IS NULL, sort_order ASC, id ASC")
    } else {
        query = query.Order("id ASC")
    }
    
    if err := query.Find(&options).Error; err != nil {
        return nil, fmt.Errorf("查询选项失败: %w", err)
    }
    
    return options, nil
}

// AddBlacklist 添加黑名单
func (s *BossDBService) AddBlacklist(blacklistType, value string) error {
    // 检查是否已存在
    var count int64
    if err := s.db.Model(&BlacklistEntity{}).
        Where("type = ? AND value = ?", blacklistType, value).
        Count(&count).Error; err != nil {
        return fmt.Errorf("检查黑名单存在性失败: %w", err)
    }
    
    if count > 0 {
        return fmt.Errorf("黑名单已存在")
    }
    
    entity := &BlacklistEntity{
        Type:      blacklistType,
        Value:     value,
        CreatedAt: time.Now(),
        UpdatedAt: time.Now(),
    }
    
    return s.db.Create(entity).Error
}

// RemoveBlacklist 移除黑名单
func (s *BossDBService) RemoveBlacklist(blacklistType, value string) error {
    result := s.db.Where("type = ? AND value = ?", blacklistType, value).Delete(&BlacklistEntity{})
    if result.Error != nil {
        return fmt.Errorf("删除黑名单失败: %w", result.Error)
    }
    
    if result.RowsAffected == 0 {
        return fmt.Errorf("黑名单不存在")
    }
    
    return nil
}

// 实体转换方法
func (s *BossDBService) entityToConfig(entity *BossConfigEntity) *BossConfig {
    if entity == nil {
        return &BossConfig{}
    }
    
    return &BossConfig{
        SayHi:         entity.SayHi,
        Debugger:      entity.Debugger == 1,
        EnableAI:      entity.EnableAI == 1,
        FilterDeadHR:  entity.FilterDeadHR == 1,
        SendImgResume: entity.SendImgResume == 1,
        WaitTime:      fmt.Sprintf("%d", entity.WaitTime),
        Keywords:      s.parseListString(entity.Keywords),
        CityCode:      s.parseListString(entity.CityCode),
        Industry:      s.parseListString(entity.Industry),
        JobType:       entity.JobType,
        Experience:    s.parseListString(entity.Experience),
        Degree:        s.parseListString(entity.Degree),
        Salary:        s.parseListString(entity.Salary),
        Scale:         s.parseListString(entity.Scale),
        Stage:         s.parseListString(entity.Stage),
    }
}

func (s *BossDBService) parseListString(raw string) []string {
    if raw == "" {
        return []string{}
    }
    
    // 解析括号列表或逗号分隔的字符串
    str := strings.TrimSpace(raw)
    if strings.HasPrefix(str, "[") && strings.HasSuffix(str, "]") {
        str = str[1 : len(str)-1]
    }
    
    if str == "" {
        return []string{}
    }
    
    items := strings.Split(str, ",")
    result := make([]string, 0, len(items))
    for _, item := range items {
        item = strings.TrimSpace(item)
        item = strings.Trim(item, `"`)
        if item != "" {
            result = append(result, item)
        }
    }
    
    return result
}

// 数据模型定义
type BlacklistEntity struct {
    ID        uint      `gorm:"primaryKey" json:"id"`
    Type      string    `gorm:"size:50;index" json:"type"` // company/recruiter/job
    Value     string    `gorm:"size:255;index" json:"value"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}

type BossJobDataEntity struct {
    ID              uint      `gorm:"primaryKey" json:"id"`
    EncryptID       string    `gorm:"size:100;index" json:"encrypt_id"`
    EncryptUserID   string    `gorm:"size:100;index" json:"encrypt_user_id"`
    CompanyName     string    `gorm:"size:255" json:"company_name"`
    JobName         string    `gorm:"size:255" json:"job_name"`
    Salary          string    `gorm:"size:100" json:"salary"`
    Location        string    `gorm:"size:100" json:"location"`
    Experience      string    `gorm:"size:100" json:"experience"`
    Degree          string    `gorm:"size:100" json:"degree"`
    HRName          string    `gorm:"size:100" json:"hr_name"`
    HRPosition      string    `gorm:"size:100" json:"hr_position"`
    HRActiveStatus  string    `gorm:"size:100" json:"hr_active_status"`
    DeliveryStatus  string    `gorm:"size:50" json:"delivery_status"` // 未投递/已投递/已过滤/投递失败
    JobDescription  string    `gorm:"type:text" json:"job_description"`
    JobURL          string    `gorm:"size:500" json:"job_url"`
    RecruitmentStatus string  `gorm:"size:100" json:"recruitment_status"`
    CompanyAddress  string    `gorm:"size:500" json:"company_address"`
    Industry        string    `gorm:"size:100" json:"industry"`
    Introduce       string    `gorm:"type:text" json:"introduce"`
    FinancingStage  string    `gorm:"size:100" json:"financing_stage"`
    CompanyScale    string    `gorm:"size:100" json:"company_scale"`
    CreatedAt       time.Time `json:"created_at"`
    UpdatedAt       time.Time `json:"updated_at"`
}

type BossConfigEntity struct {
    ID                uint      `gorm:"primaryKey" json:"id"`
    SayHi             string    `gorm:"type:text" json:"say_hi"`
    Debugger          int       `gorm:"default:0" json:"debugger"`
    EnableAI          int       `gorm:"default:0" json:"enable_ai"`
    FilterDeadHR      int       `gorm:"default:1" json:"filter_dead_hr"`
    SendImgResume     int       `gorm:"default:0" json:"send_img_resume"`
    WaitTime          int       `gorm:"default:5" json:"wait_time"`
    Keywords          string    `gorm:"type:text" json:"keywords"`
    CityCode          string    `gorm:"type:text" json:"city_code"`
    Industry          string    `gorm:"type:text" json:"industry"`
    JobType           string    `gorm:"size:100" json:"job_type"`
    Experience        string    `gorm:"type:text" json:"experience"`
    Degree            string    `gorm:"type:text" json:"degree"`
    Salary            string    `gorm:"type:text" json:"salary"`
    Scale             string    `gorm:"type:text" json:"scale"`
    Stage             string    `gorm:"type:text" json:"stage"`
    ExpectedSalaryMin int       `json:"expected_salary_min"`
    ExpectedSalaryMax int       `json:"expected_salary_max"`
    DeadStatus        string    `gorm:"type:text" json:"dead_status"`
    CreatedAt         time.Time `json:"created_at"`
    UpdatedAt         time.Time `json:"updated_at"`
}

type BossOptionEntity struct {
    ID        uint      `gorm:"primaryKey" json:"id"`
    Type      string    `gorm:"size:50;index" json:"type"`
    Name      string    `gorm:"size:255" json:"name"`
    Code      string    `gorm:"size:100;index" json:"code"`
    SortOrder int       `gorm:"default:0" json:"sort_order"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}

// 需要在外部定义的结构体（与 boss.go 保持一致）
type Blacklist struct {
    Companies  []string `json:"companies"`
    Recruiters []string `json:"recruiters"`
    Jobs       []string `json:"jobs"`
}

type JobDetail struct {
    EncryptID      string `json:"encrypt_id"`
    EncryptUserID  string `json:"encrypt_user_id"`
    CompanyName    string `json:"company_name"`
    JobName        string `json:"job_name"`
    Salary         string `json:"salary"`
    JobArea        string `json:"job_area"`
    Experience     string `json:"experience"`
    Degree         string `json:"degree"`
    Recruiter      string `json:"recruiter"`
    HRPosition     string `json:"hr_position"`
    HRActiveStatus string `json:"hr_active_status"`
    JobInfo        string `json:"job_info"`
}

