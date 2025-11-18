package service

import (
	"database/sql"
	"get_jobs_go/model"
	"get_jobs_go/config"
	"get_jobs_go/repository"
	"regexp"
	"strconv"
	"strings"
	"time"
	"sort"
	"gorm.io/gorm"
)

// 常量定义
const (
	UNLIMITED_CODE = "0"
)

// 统计相关结构体
type SalaryInfo struct {
	MinK       *int     `json:"minK"`
	MaxK       *int     `json:"maxK"`
	Months     int      `json:"months"`
	MedianK    *float64 `json:"medianK"`
	AnnualTotal *int64  `json:"annualTotal"`
}

type Kpi struct {
	Total       int64    `json:"total"`
	Delivered   int64    `json:"delivered"`
	Pending     int64    `json:"pending"`
	Filtered    int64    `json:"filtered"`
	Failed      int64    `json:"failed"`
	AvgMonthlyK *float64 `json:"avgMonthlyK"`
}

type NameValue struct {
	Name  string `json:"name"`
	Value int64  `json:"value"`
}

type BucketValue struct {
	Bucket string `json:"bucket"`
	Value  int64  `json:"value"`
}

type Charts struct {
	ByStatus      []NameValue   `json:"byStatus"`
	ByCity        []NameValue   `json:"byCity"`
	ByIndustry    []NameValue   `json:"byIndustry"`
	ByCompany     []NameValue   `json:"byCompany"`
	ByExperience  []NameValue   `json:"byExperience"`
	ByDegree      []NameValue   `json:"byDegree"`
	SalaryBuckets []BucketValue `json:"salaryBuckets"`
	DailyTrend    []NameValue   `json:"dailyTrend"`
	HrActivity    []NameValue   `json:"hrActivity"`
}

type StatsResponse struct {
	Kpi    *Kpi    `json:"kpi"`
	Charts *Charts `json:"charts"`
}

type PagedResult struct {
	Items []*model.BossJobDataEntity `json:"items"`
	Total int64                      `json:"total"`
	Page  int                        `json:"page"`
	Size  int                        `json:"size"`
}

// BossService Boss数据服务
type BossService struct {
	optionRepo     repository.BossOptionRepository
	industryRepo   repository.BossIndustryRepository
	configRepo     repository.BossConfigRepository
	blacklistRepo  repository.BlacklistRepository
	jobDataRepo    repository.BossJobDataRepository
	db             *gorm.DB
}

func NewBossService(
	optionRepo repository.BossOptionRepository,
	industryRepo repository.BossIndustryRepository,
	configRepo repository.BossConfigRepository,
	blacklistRepo repository.BlacklistRepository,
	jobDataRepo repository.BossJobDataRepository,
	db *gorm.DB,
) *BossService {
	return &BossService{
		optionRepo:    optionRepo,
		industryRepo:  industryRepo,
		configRepo:    configRepo,
		blacklistRepo: blacklistRepo,
		jobDataRepo:   jobDataRepo,
		db:            db,
	}
}

// ==================== Option相关方法 ====================

// GetOptionsByType 根据类型获取选项列表
func (s *BossService) GetOptionsByType(typeStr string) ([]*model.BossOptionEntity, error) {
	// 确保存在"不限"选项
	unlimitedOption, err := s.optionRepo.FindByTypeAndCode(typeStr, UNLIMITED_CODE)
	if err != nil {
		return nil, err
	}
	
	if unlimitedOption == nil {
		unlimitedOption = &model.BossOptionEntity{
			Type:      typeStr,
			Name:      "不限",
			Code:      UNLIMITED_CODE,
			SortOrder: 0,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		if err := s.optionRepo.Save(unlimitedOption); err != nil {
			return nil, err
		}
	}
	
	return s.optionRepo.FindByType(typeStr)
}

// GetAllOptions 获取所有选项
func (s *BossService) GetAllOptions() ([]*model.BossOptionEntity, error) {
	return s.optionRepo.FindAll()
}

// GetOptionByTypeAndCode 根据类型和代码获取选项
func (s *BossService) GetOptionByTypeAndCode(typeStr, code string) (*model.BossOptionEntity, error) {
	return s.optionRepo.FindByTypeAndCode(typeStr, code)
}

// GetCodeByTypeAndName 根据类型和名称获取代码
func (s *BossService) GetCodeByTypeAndName(typeStr, name string) string {
	option, err := s.optionRepo.FindByTypeAndCode(typeStr, name)
	if err == nil && option != nil {
		return option.Code
	}
	
	// 如果没找到，尝试按名称查找
	options, err := s.optionRepo.FindByType(typeStr)
	if err != nil {
		return UNLIMITED_CODE
	}
	
	for _, opt := range options {
		if opt.Name == name {
			return opt.Code
		}
	}
	
	return UNLIMITED_CODE
}

// ==================== City相关方法 ====================

// GetCityCodeByName 根据城市名称获取代码
func (s *BossService) GetCityCodeByName(name string) string {
	return s.GetCodeByTypeAndName("city", name)
}

// ==================== Industry相关方法 ====================

// GetAllIndustries 获取所有行业
func (s *BossService) GetAllIndustries() ([]*model.BossIndustryEntity, error) {
	return s.industryRepo.FindAll()
}

// GetIndustryByCode 根据代码获取行业
func (s *BossService) GetIndustryByCode(code int) (*model.BossIndustryEntity, error) {
	return s.industryRepo.FindByCode(code)
}

// GetIndustryCodeByName 根据行业名称获取代码
func (s *BossService) GetIndustryCodeByName(name string) string {
	industry, err := s.industryRepo.FindByName(name)
	if err != nil || industry == nil {
		return UNLIMITED_CODE
	}
	return strconv.Itoa(industry.Code)
}

// ==================== BossConfig相关方法 ====================

// GetAllConfigs 获取所有配置
func (s *BossService) GetAllConfigs() ([]*model.BossConfigEntity, error) {
	return s.configRepo.FindAll()
}

// GetConfigById 根据ID获取配置
func (s *BossService) GetConfigById(id int64) (*model.BossConfigEntity, error) {
	return s.configRepo.FindByID(id)
}

// GetFirstConfig 获取第一条配置
func (s *BossService) GetFirstConfig() (*model.BossConfigEntity, error) {
	return s.configRepo.FindFirst()
}

// SaveConfig 保存配置
func (s *BossService) SaveConfig(config *model.BossConfigEntity) error {
	now := time.Now()
	config.CreatedAt = now
	config.UpdatedAt = now
	return s.configRepo.Save(config)
}

// UpdateConfig 更新配置
func (s *BossService) UpdateConfig(config *model.BossConfigEntity) error {
	config.UpdatedAt = time.Now()
	return s.configRepo.Update(config)
}

// SaveOrUpdateFirstSelective 保存或更新配置（选择性更新）
func (s *BossService) SaveOrUpdateFirstSelective(partial *model.BossConfigEntity) (*model.BossConfigEntity, error) {
	existing, err := s.configRepo.FindFirst()
	if err != nil {
		return nil, err
	}
	
	now := time.Now()
	
	if existing == nil {
		// 表为空，插入新记录
		partial.CreatedAt = now
		partial.UpdatedAt = now
		if err := s.configRepo.Save(partial); err != nil {
			return nil, err
		}
		return partial, nil
	}
	
	// 选择性合并字段
	if partial.SayHi != "" {
		existing.SayHi = partial.SayHi
	}
	if partial.Debugger != 0 {
		existing.Debugger = partial.Debugger
	}
	if partial.EnableAi != 0 {
		existing.EnableAi = partial.EnableAi
	}
	if partial.FilterDeadHr != 0 {
		existing.FilterDeadHr = partial.FilterDeadHr
	}
	if partial.SendImgResume != 0 {
		existing.SendImgResume = partial.SendImgResume
	}
	if partial.WaitTime != 0 {
		existing.WaitTime = partial.WaitTime
	}
	
	if partial.Keywords != "" {
		existing.Keywords = partial.Keywords
	}
	if partial.CityCode != "" {
		existing.CityCode = partial.CityCode
	}
	if partial.Industry != "" {
		existing.Industry = partial.Industry
	}
	if partial.JobType != "" {
		existing.JobType = partial.JobType
	}
	if partial.Experience != "" {
		existing.Experience = partial.Experience
	}
	if partial.Degree != "" {
		existing.Degree = partial.Degree
	}
	if partial.Salary != "" {
		existing.Salary = partial.Salary
	}
	if partial.Scale != "" {
		existing.Scale = partial.Scale
	}
	if partial.Stage != "" {
		existing.Stage = partial.Stage
	}
	
	if partial.ExpectedSalaryMin != 0 {
		existing.ExpectedSalaryMin = partial.ExpectedSalaryMin
	}
	if partial.ExpectedSalaryMax != 0 {
		existing.ExpectedSalaryMax = partial.ExpectedSalaryMax
	}
	
	if partial.DeadStatus != "" {
		existing.DeadStatus = partial.DeadStatus
	}
	
	existing.UpdatedAt = now
	if err := s.configRepo.Update(existing); err != nil {
		return nil, err
	}
	
	return existing, nil
}

// DeleteConfig 删除配置
func (s *BossService) DeleteConfig(id int64) error {
	return s.configRepo.Delete(id)
}

// ==================== 配置加载方法 ====================

// LoadBossConfig 加载Boss配置
func (s *BossService) LoadBossConfig() (*config.BossConfig, error) {
	entity, err := s.configRepo.FindFirst()
	if err != nil {
		return nil, err
	}

	if entity == nil {
		return &config.BossConfig{}, nil
	}

	config := &config.BossConfig{
		SayHi: entity.SayHi,
		Debugger: entity.Debugger == 1,
		EnableAI: entity.EnableAi == 1,
		FilterDeadHR: entity.FilterDeadHr == 1,
		SendImgResume: entity.SendImgResume == 1,
		WaitTime: strconv.Itoa(entity.WaitTime),
		Keywords: s.ParseListString(entity.Keywords),
		CityCode: s.ToCodes("city", s.ParseListString(entity.CityCode)),
		Industry: s.ToCodes("industry", s.ParseListString(entity.Industry)),
		Experience: s.ToCodes("experience", s.ParseListString(entity.Experience)),
		Degree: s.ToCodes("degree", s.ParseListString(entity.Degree)),
		Scale: s.ToCodes("scale", s.ParseListString(entity.Scale)),
		Stage: s.ToCodes("stage", s.ParseListString(entity.Stage)),
		Salary: s.ToCodes("salary", s.ParseListString(entity.Salary)),
		DeadStatus: s.ParseListString(entity.DeadStatus),
	}

	// 处理职位类型
	jobTypeCodes := s.ToCodes("jobType", s.ParseListString(entity.JobType))
	if len(jobTypeCodes) > 0 {
		config.JobType = jobTypeCodes[0]
	} else if entity.JobType != "" {
		option, err := s.optionRepo.FindByTypeAndCode("jobType", entity.JobType)
		if err == nil && option != nil && option.Code != "" {
			config.JobType = option.Code
		} else {
			config.JobType = s.GetCodeByTypeAndName("jobType", entity.JobType)
		}
	}

	if config.JobType == "" {
		config.JobType = UNLIMITED_CODE
	}

	// 处理期望薪资
	if entity.ExpectedSalaryMin != 0 || entity.ExpectedSalaryMax != 0 {
		config.ExpectedSalary = []int{
			entity.ExpectedSalaryMin,
			entity.ExpectedSalaryMax,
		}
	}

	return config, nil
}

// ==================== 配置工具方法 ====================

// ParseListString 解析括号列表或逗号分隔的字符串
func (s *BossService) ParseListString(raw string) []string {
    if strings.TrimSpace(raw) == "" {
        return []string{}
    }
    
    str := strings.TrimSpace(raw)
    if strings.HasPrefix(str, "[") && strings.HasSuffix(str, "]") {
        str = str[1 : len(str)-1]
    }
    
    if strings.TrimSpace(str) == "" {
        return []string{}
    }
    
    items := strings.Split(str, ",")
    result := make([]string, 0, len(items))
    
    for _, item := range items {
        item = strings.TrimSpace(item)
        item = strings.Trim(item, "\"")
        if item != "" {
            result = append(result, item)
        }
    }
    
    return result
}

// ToBracketListString 将列表转换为括号列表字符串
func (s *BossService) ToBracketListString(list []string) string {
	if len(list) == 0 {
		return ""
	}
	return "[" + strings.Join(list, ",") + "]"
}

// ToNames 将代码列表转换为名称列表
func (s *BossService) ToNames(typeStr string, items []string) []string {
	if len(items) == 0 {
		return []string{}
	}
	
	result := make([]string, 0, len(items))
	for _, item := range items {
		option, err := s.optionRepo.FindByTypeAndCode(typeStr, item)
		if err == nil && option != nil && option.Name != "" {
			result = append(result, option.Name)
		} else {
			result = append(result, item)
		}
	}
	
	return result
}

// ToCodes 将名称列表转换为代码列表
func (s *BossService) ToCodes(typeStr string, items []string) []string {
	if len(items) == 0 {
		return []string{}
	}
	
	result := make([]string, 0, len(items))
	for _, item := range items {
		// 先检查是否是有效代码
		option, err := s.optionRepo.FindByTypeAndCode(typeStr, item)
		if err == nil && option != nil {
			result = append(result, option.Code)
			continue
		}
		
		// 否则按名称查找代码
		code := s.GetCodeByTypeAndName(typeStr, item)
		result = append(result, code)
	}
	
	return result
}

// NormalizeCityToName 统一化城市名称
func (s *BossService) NormalizeCityToName(raw string) string {
	list := s.ParseListString(raw)
	
	var first string
	if len(list) > 0 {
		first = list[0]
	} else {
		first = strings.TrimSpace(raw)
	}
	
	if first == "" {
		return ""
	}
	
	option, err := s.optionRepo.FindByTypeAndCode("city", first)
	if err == nil && option != nil && option.Name != "" {
		return option.Name
	}
	
	return first
}

// ==================== Blacklist相关方法 ====================

// GetBlacklistByType 根据类型获取黑名单
func (s *BossService) GetBlacklistByType(typeStr string) (map[string]bool, error) {
	blacklists, err := s.blacklistRepo.FindByType(typeStr)
	if err != nil {
		return nil, err
	}
	
	result := make(map[string]bool)
	for _, blacklist := range blacklists {
		result[blacklist.Value] = true
	}
	
	return result, nil
}

// GetBlackCompanies 获取公司黑名单
func (s *BossService) GetBlackCompanies() (map[string]bool, error) {
	return s.GetBlacklistByType("company")
}

// GetBlackRecruiters 获取招聘者黑名单
func (s *BossService) GetBlackRecruiters() (map[string]bool, error) {
	return s.GetBlacklistByType("recruiter")
}

// GetBlackJobs 获取职位黑名单
func (s *BossService) GetBlackJobs() (map[string]bool, error) {
	return s.GetBlacklistByType("job")
}

// AddBlacklist 添加黑名单
func (s *BossService) AddBlacklist(typeStr, value string) (bool, error) {
	count, err := s.blacklistRepo.CountByTypeAndValue(typeStr, value)
	if err != nil {
		return false, err
	}
	
	if count > 0 {
		return false, nil // 已存在
	}
	
	model := &model.BlacklistEntity{
		Type:      typeStr,
		Value:     value,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	
	if err := s.blacklistRepo.Save(model); err != nil {
		return false, err
	}
	
	return true, nil
}

// AddBlacklistBatch 批量添加黑名单
func (s *BossService) AddBlacklistBatch(typeStr string, values map[string]bool) error {
	for value := range values {
		if _, err := s.AddBlacklist(typeStr, value); err != nil {
			return err
		}
	}
	return nil
}

// RemoveBlacklist 删除黑名单
func (s *BossService) RemoveBlacklist(typeStr, value string) (bool, error) {
	err := s.blacklistRepo.DeleteByTypeAndValue(typeStr, value)
	if err != nil {
		return false, err
	}
	return true, nil
}

// GetAllBlacklist 获取所有黑名单
func (s *BossService) GetAllBlacklist() ([]*model.BlacklistEntity, error) {
	return s.blacklistRepo.FindAll()
}

// ==================== 职位数据相关方法 ====================

// EnsureBossDataColumnOrder 确保表列顺序正确
func (s *BossService) EnsureBossDataColumnOrder() error {
	return s.db.AutoMigrate(&model.BossJobDataEntity{})
}

// ExistsBossJob 判断职位是否存在
func (s *BossService) ExistsBossJob(encryptId, encryptUserId string) (bool, error) {
	if encryptId == "" || encryptUserId == "" {
		return false, nil
	}
	
	job, err := s.jobDataRepo.FindByEncryptIdAndUserId(encryptId, encryptUserId)
	if err != nil {
		return false, err
	}
	return job != nil, nil
}

// ExistsBossJobByEncryptId 根据encryptId判断职位是否存在
func (s *BossService) ExistsBossJobByEncryptId(encryptId string) (bool, error) {
	if encryptId == "" {
		return false, nil
	}
	
	job, err := s.jobDataRepo.FindByEncryptId(encryptId)
	if err != nil {
		return false, err
	}
	return job != nil, nil
}

// InsertBossJob 插入职位数据
func (s *BossService) InsertBossJob(job *model.BossJobDataEntity) error {
	now := time.Now()
	job.CreatedAt = now
	job.UpdatedAt = now
	return s.jobDataRepo.Save(job)
}

// UpdateDeliveryStatus 更新投递状态
func (s *BossService) UpdateDeliveryStatus(encryptId, encryptUserId, status string) error {
	return s.jobDataRepo.UpdateDeliveryStatus(encryptId, encryptUserId, status)
}

// ==================== 薪资解析方法 ====================

// ParseSalary 解析薪资字符串
func (s *BossService) ParseSalary(salary string) *SalaryInfo {
	if strings.TrimSpace(salary) == "" {
		return nil
	}
	
	salaryStr := strings.TrimSpace(salary)
	if strings.Contains(salaryStr, "面议") {
		return nil
	}
	
	salaryStr = strings.ReplaceAll(salaryStr, " ", "")
	
	// 提取月数
	months := 12
	monthsRegex := regexp.MustCompile(`[·\.\-]?([0-9]+)薪`)
	monthsMatch := monthsRegex.FindStringSubmatch(salaryStr)
	if len(monthsMatch) > 1 {
		if m, err := strconv.Atoi(monthsMatch[1]); err == nil {
			months = m
		}
		// 移除月数部分
		salaryStr = salaryStr[:monthsRegex.FindStringIndex(salaryStr)[0]]
	}
	
	// 提取薪资范围
	var minK, maxK *int
	rangeRegex := regexp.MustCompile(`^(\d+)-(\d+)[Kk]$`)
	singleRegex := regexp.MustCompile(`^(\d+)[Kk]$`)
	
	if match := rangeRegex.FindStringSubmatch(salaryStr); len(match) > 2 {
		if min, err := strconv.Atoi(match[1]); err == nil {
			minK = &min
		}
		if max, err := strconv.Atoi(match[2]); err == nil {
			maxK = &max
		}
	} else if match := singleRegex.FindStringSubmatch(salaryStr); len(match) > 1 {
		if val, err := strconv.Atoi(match[1]); err == nil {
			minK = &val
			maxK = &val
		}
	} else {
		// 宽松解析
		cleaned := regexp.MustCompile(`[^0-9Kk\-]`).ReplaceAllString(salaryStr, "")
		if match := rangeRegex.FindStringSubmatch(cleaned); len(match) > 2 {
			if min, err := strconv.Atoi(match[1]); err == nil {
				minK = &min
			}
			if max, err := strconv.Atoi(match[2]); err == nil {
				maxK = &max
			}
		} else if match := singleRegex.FindStringSubmatch(cleaned); len(match) > 1 {
			if val, err := strconv.Atoi(match[1]); err == nil {
				minK = &val
				maxK = &val
			}
		}
	}
	
	if minK == nil || maxK == nil {
		return nil
	}
	
	info := &SalaryInfo{
		MinK:   minK,
		MaxK:   maxK,
		Months: months,
	}
	
	median := float64(*minK+*maxK) / 2.0
	info.MedianK = &median
	
	annual := int64(median * 1000 * float64(months))
	info.AnnualTotal = &annual
	
	return info
}

// ==================== 统计分析方法 ====================

// GetBossStats 获取统计数据
func (s *BossService) GetBossStats() (*StatsResponse, error) {
	return s.GetBossStatsWithFilter(nil, "", "", "", nil, nil, "", false)
}

// GetBossStatsWithFilter 获取统计数据（带筛选条件）
func (s *BossService) GetBossStatsWithFilter(
	statuses []string,
	location string,
	experience string,
	degree string,
	minK *float64,
	maxK *float64,
	keyword string,
	filterHeadhunter bool,
) (*StatsResponse, error) {
	resp := &StatsResponse{
		Kpi: &Kpi{},
		Charts: &Charts{
			ByStatus:     []NameValue{},
			ByCity:       []NameValue{},
			ByIndustry:   []NameValue{},
			ByCompany:    []NameValue{},
			ByExperience: []NameValue{},
			ByDegree:     []NameValue{},
			SalaryBuckets: []BucketValue{},
			DailyTrend:   []NameValue{},
			HrActivity:   []NameValue{},
		},
	}

	// 构建查询条件
	wrapper := s.db.Model(&model.BossJobDataEntity{})
	
	if len(statuses) > 0 {
		wrapper = wrapper.Where("delivery_status IN ?", statuses)
	}
	if location != "" {
		wrapper = wrapper.Where("location = ?", location)
	}
	if experience != "" {
		wrapper = wrapper.Where("experience = ?", experience)
	}
	if degree != "" {
		wrapper = wrapper.Where("degree = ?", degree)
	}
	if keyword != "" {
		wrapper = wrapper.Where("company_name LIKE ? OR job_name LIKE ? OR hr_name LIKE ?", 
			"%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%")
	}
	if filterHeadhunter {
		wrapper = wrapper.Where("hr_position IS NULL OR hr_position NOT LIKE ?", "%猎头%")
	}

	// 获取基础数据
	jobs, err := s.jobDataRepo.FindByWrapper(wrapper)
	if err != nil {
		return nil, err
	}

	// 内存薪资过滤
	filteredJobs := make([]*model.BossJobDataEntity, 0)
	var sumMedian float64
	var countMedian int64

	for _, job := range jobs {
		// 薪资过滤
		passSalary := true
		if minK != nil || maxK != nil {
			info := s.ParseSalary(job.Salary)
			if info == nil || info.MedianK == nil {
				passSalary = false
			} else {
				if minK != nil && *info.MedianK < *minK {
					passSalary = false
				}
				if maxK != nil && *info.MedianK > *maxK {
					passSalary = false
				}
			}
		}

		if passSalary {
			filteredJobs = append(filteredJobs, job)
			// 计算平均薪资
			info := s.ParseSalary(job.Salary)
			if info != nil && info.MedianK != nil {
				sumMedian += *info.MedianK
				countMedian++
			}
		}
	}

	// 计算KPI
	resp.Kpi.Total = int64(len(filteredJobs))
	for _, job := range filteredJobs {
		switch job.DeliveryStatus {
		case "已投递":
			resp.Kpi.Delivered++
		case "未投递":
			resp.Kpi.Pending++
		case "已过滤":
			resp.Kpi.Filtered++
		case "投递失败":
			resp.Kpi.Failed++
		}
	}

	if countMedian > 0 {
		avg := sumMedian / float64(countMedian)
		avg = float64(int(avg*100)) / 100
		resp.Kpi.AvgMonthlyK = &avg
	}

	// 计算图表数据
	s.calculateCharts(resp.Charts, filteredJobs)

	return resp, nil
}

// ListBossJobs 列表查询（分页 + 筛选）
func (s *BossService) ListBossJobs(
	statuses []string,
	location string,
	experience string,
	degree string,
	minK *float64,
	maxK *float64,
	keyword string,
	page int,
	size int,
	filterHeadhunter bool,
) (*PagedResult, error) {
	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 20
	}

	// 构建查询条件
	wrapper := s.db.Model(&model.BossJobDataEntity{})
	
	if len(statuses) > 0 {
		wrapper = wrapper.Where("delivery_status IN ?", statuses)
	}
	if location != "" {
		wrapper = wrapper.Where("location = ?", location)
	}
	if experience != "" {
		wrapper = wrapper.Where("experience = ?", experience)
	}
	if degree != "" {
		wrapper = wrapper.Where("degree = ?", degree)
	}
	if keyword != "" {
		wrapper = wrapper.Where("company_name LIKE ? OR job_name LIKE ? OR hr_name LIKE ?", 
			"%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%")
	}
	if filterHeadhunter {
		wrapper = wrapper.Where("hr_position IS NULL OR hr_position NOT LIKE ?", "%猎头%")
	}

	wrapper = wrapper.Order("created_at DESC")

	// 获取总数
	_, err := s.jobDataRepo.CountByWrapper(wrapper)
	if err != nil {
		return nil, err
	}

	// 分页查询
	wrapper = wrapper.Offset((page - 1) * size).Limit(size)
	items, err := s.jobDataRepo.FindByWrapper(wrapper)
	if err != nil {
		return nil, err
	}

	// 内存薪资过滤
	filteredItems := make([]*model.BossJobDataEntity, 0)
	for _, item := range items {
		passSalary := true
		if minK != nil || maxK != nil {
			info := s.ParseSalary(item.Salary)
			if info == nil || info.MedianK == nil {
				passSalary = false
			} else {
				if minK != nil && *info.MedianK < *minK {
					passSalary = false
				}
				if maxK != nil && *info.MedianK > *maxK {
					passSalary = false
				}
			}
		}

		if passSalary {
			filteredItems = append(filteredItems, item)
		}
	}

	return &PagedResult{
		Items: filteredItems,
		Total: int64(len(filteredItems)),
		Page:  page,
		Size:  size,
	}, nil
}

// ReloadBossData 刷新数据
func (s *BossService) ReloadBossData() (map[string]interface{}, error) {
	result := make(map[string]interface{})
	
	// 确保列顺序
	if err := s.EnsureBossDataColumnOrder(); err != nil {
		result["success"] = false
		result["message"] = "刷新失败: " + err.Error()
		return result, err
	}

	// 执行VACUUM优化
	sqlDB, err := s.db.DB()
	if err != nil {
		result["success"] = false
		result["message"] = "刷新失败: " + err.Error()
		return result, err
	}

	_, err = sqlDB.Exec("VACUUM")
	if err != nil {
		result["success"] = false
		result["message"] = "刷新失败: " + err.Error()
		return result, err
	}

	// 获取总数
	total, err := s.jobDataRepo.CountByCondition("1=1")
	if err != nil {
		result["success"] = false
		result["message"] = "刷新失败: " + err.Error()
		return result, err
	}

	result["success"] = true
	result["message"] = "刷新完成"
	result["total"] = total

	return result, nil
}

// ==================== 辅助方法 ====================

// nullSafeString 空值安全处理（字符串版本）
func (s *BossService) nullSafeString(str string) string {
	if str == "" {
		return "未知"
	}
	return str
}

// calculateCharts 计算图表数据
func (s *BossService) calculateCharts(charts *Charts, jobs []*model.BossJobDataEntity) {
	// 状态统计
	statusMap := make(map[string]int64)
	cityMap := make(map[string]int64)
	industryMap := make(map[string]int64)
	companyMap := make(map[string]int64)
	experienceMap := make(map[string]int64)
	degreeMap := make(map[string]int64)
	dailyMap := make(map[string]int64)
	hrActivityMap := make(map[string]int64)

	// 薪资分桶
	bucket0_10 := int64(0)
	bucket10_15 := int64(0)
	bucket15_20 := int64(0)
	bucket20_top := int64(0)
	bucket_ge_top := int64(0)
	maxMedian := 0.0

	for _, job := range jobs {
		// 状态统计 - 修复：直接传递字符串值，不需要指针
		statusMap[s.nullSafeString(job.DeliveryStatus)]++
		cityMap[s.nullSafeString(job.Location)]++
		industryMap[s.nullSafeString(job.Industry)]++
		companyMap[s.nullSafeString(job.CompanyName)]++
		experienceMap[s.nullSafeString(job.Experience)]++
		degreeMap[s.nullSafeString(job.Degree)]++

		// 日期统计
		if !job.CreatedAt.IsZero() {
			date := job.CreatedAt.Format("2006-01-02")
			dailyMap[date]++
		}

		// HR活跃度统计 - 修复：直接传递字符串值
		if job.HrActiveStatus != "" {
			hrActivityMap[s.nullSafeString(job.HrName)]++
		}

		// 薪资分桶
		info := s.ParseSalary(job.Salary)
		if info != nil && info.MedianK != nil {
			median := *info.MedianK
			if median > maxMedian {
				maxMedian = median
			}

			if median < 10 {
				bucket0_10++
			} else if median < 15 {
				bucket10_15++
			} else if median < 20 {
				bucket15_20++
			} else {
				topEdge := int((maxMedian/5)+1) * 5
				if topEdge <= 20 {
					topEdge = 25
				}
				if median < float64(topEdge) {
					bucket20_top++
				} else {
					bucket_ge_top++
				}
			}
		}
	}

	// 转换为NameValue切片
	charts.ByStatus = s.mapToNameValueSlice(statusMap)
	charts.ByCity = s.getTop10(cityMap)
	charts.ByIndustry = s.getTop10(industryMap)
	charts.ByCompany = s.getTop10(companyMap)
	charts.ByExperience = s.mapToNameValueSlice(experienceMap)
	charts.ByDegree = s.mapToNameValueSlice(degreeMap)
	charts.DailyTrend = s.mapToNameValueSlice(dailyMap)
	charts.HrActivity = s.mapToNameValueSlice(hrActivityMap)

	// 薪资分桶
	topEdge := int((maxMedian/5)+1) * 5
	if topEdge <= 20 {
		topEdge = 25
	}
	charts.SalaryBuckets = []BucketValue{
		{Bucket: "0-10K", Value: bucket0_10},
		{Bucket: "10-15K", Value: bucket10_15},
		{Bucket: "15-20K", Value: bucket15_20},
		{Bucket: "20-" + strconv.Itoa(topEdge) + "K", Value: bucket20_top},
		{Bucket: ">=" + strconv.Itoa(topEdge) + "K", Value: bucket_ge_top},
	}
}

// mapToNameValueSlice 将map转换为NameValue切片
func (s *BossService) mapToNameValueSlice(m map[string]int64) []NameValue {
	result := make([]NameValue, 0, len(m))
	for k, v := range m {
		result = append(result, NameValue{Name: k, Value: v})
	}
	return result
}

// getTop10 获取前10名
// getTop10 获取前10名（按值降序排序）
func (s *BossService) getTop10(m map[string]int64) []NameValue {
	// 创建切片用于排序
	type kv struct {
		Key   string
		Value int64
	}
	
	var pairs []kv
	for k, v := range m {
		pairs = append(pairs, kv{k, v})
	}
	
	// 按值降序排序
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].Value > pairs[j].Value
	})
	
	// 取前10个
	result := make([]NameValue, 0, 10)
	for i, pair := range pairs {
		if i >= 10 {
			break
		}
		result = append(result, NameValue{Name: pair.Key, Value: pair.Value})
	}
	
	return result
}

// scalarCount 标量计数
func (s *BossService) scalarCount(db *sql.DB, query string) (int64, error) {
	var count int64
	err := db.QueryRow(query).Scan(&count)
	return count, err
}

// nullSafe 空值安全处理
func (s *BossService) nullSafe(str *string) string {
	if str == nil || *str == "" {
		return "未知"
	}
	return *str
}

