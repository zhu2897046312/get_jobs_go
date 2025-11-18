package model

import (
	"time"
)

// BossConfigEntity Boss配置实体类
type BossConfigEntity struct {
	ID                int64     `gorm:"primaryKey;autoIncrement;column:id"`
	Debugger          int       `gorm:"column:debugger"`           // 调试模式（1=开启，0=关闭）
	WaitTime          int       `gorm:"column:wait_time"`          // 页面操作等待时间（秒）
	Keywords          string    `gorm:"column:keywords"`           // 搜索关键词
	CityCode          string    `gorm:"column:city_code"`          // 城市（名称或代码，支持列表）
	Industry          string    `gorm:"column:industry"`           // 行业（名称或代码，支持列表）
	JobType           string    `gorm:"column:job_type"`           // 职位类型（名称或代码，单值或列表，优先取第一项）
	Experience        string    `gorm:"column:experience"`         // 工作经验（名称或代码，支持列表）
	Degree            string    `gorm:"column:degree"`             // 学历要求（名称或代码，支持列表）
	Salary            string    `gorm:"column:salary"`             // 薪资区间（名称或代码，支持列表）
	Scale             string    `gorm:"column:scale"`              // 公司规模（名称或代码，支持列表）
	Stage             string    `gorm:"column:stage"`              // 融资阶段（名称或代码，支持列表）
	SayHi             string    `gorm:"column:say_hi"`             // 默认打招呼语
	ExpectedSalaryMin int       `gorm:"column:expected_salary_min"` // 期望薪资下限
	ExpectedSalaryMax int       `gorm:"column:expected_salary_max"` // 期望薪资上限
	EnableAi          int       `gorm:"column:enable_ai"`          // 是否启用AI生成打招呼（1=启用，0=关闭）
	SendImgResume     int       `gorm:"column:send_img_resume"`    // 是否发送图片简历（1=启用，0=关闭）
	FilterDeadHr      int       `gorm:"column:filter_dead_hr"`     // 是否过滤不在线HR（1=启用，0=关闭）
	DeadStatus        string    `gorm:"column:dead_status"`        // HR不在线状态列表
	CreatedAt         time.Time `gorm:"column:created_at"`
	UpdatedAt         time.Time `gorm:"column:updated_at"`
}

func (BossConfigEntity) TableName() string {
	return "boss_config"
}

// BossIndustryEntity Boss行业实体类
type BossIndustryEntity struct {
	ID        int64     `gorm:"primaryKey;autoIncrement;column:id"`
	Name      string    `gorm:"column:name"`
	Code      int       `gorm:"column:code"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (BossIndustryEntity) TableName() string {
	return "boss_industry"
}

// BossJobDataEntity Boss职位数据实体类
type BossJobDataEntity struct {
	ID                int64     `gorm:"primaryKey;autoIncrement;column:id"`
	EncryptId         string    `gorm:"column:encrypt_id"`
	EncryptUserId     string    `gorm:"column:encrypt_user_id"`
	CompanyName       string    `gorm:"column:company_name"`
	JobName           string    `gorm:"column:job_name"`
	Salary            string    `gorm:"column:salary"`
	Location          string    `gorm:"column:location"`
	Experience        string    `gorm:"column:experience"`
	Degree            string    `gorm:"column:degree"`
	HrName            string    `gorm:"column:hr_name"`
	HrPosition        string    `gorm:"column:hr_position"`
	HrActiveStatus    string    `gorm:"column:hr_active_status"`
	DeliveryStatus    string    `gorm:"column:delivery_status"` // 默认 未投递 / 已投递 / 已过滤 / 投递失败
	JobDescription    string    `gorm:"column:job_description"`
	JobUrl            string    `gorm:"column:job_url"`
	RecruitmentStatus string    `gorm:"column:recruitment_status"`
	CompanyAddress    string    `gorm:"column:company_address"`
	Industry          string    `gorm:"column:industry"`
	Introduce         string    `gorm:"column:introduce"`
	FinancingStage    string    `gorm:"column:financing_stage"`
	CompanyScale      string    `gorm:"column:company_scale"`
	CreatedAt         time.Time `gorm:"column:created_at"`
	UpdatedAt         time.Time `gorm:"column:updated_at"`
}

func (BossJobDataEntity) TableName() string {
	return "boss_data"
}

// BossOptionEntity Boss选项实体类
type BossOptionEntity struct {
	ID        int64     `gorm:"primaryKey;autoIncrement;column:id"`
	Type      string    `gorm:"column:type"`       // 选项类型：city, industry, experience, jobType, salary, degree, scale, stage
	Name      string    `gorm:"column:name"`       // 选项名称
	Code      string    `gorm:"column:code"`       // 选项代码
	SortOrder int       `gorm:"column:sort_order"` // 显示排序（数值越小越靠前）
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (BossOptionEntity) TableName() string {
	return "boss_option"
}