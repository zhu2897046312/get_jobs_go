package boss

// config/boss_config.go
type BossConfig struct {
    SayHi         string   `yaml:"say_hi" json:"say_hi"`
    Debugger      bool     `yaml:"debugger" json:"debugger"`
    Keywords      []string `yaml:"keywords" json:"keywords"`
    CityCode      []string `yaml:"city_code" json:"city_code"`
    JobType       string   `yaml:"job_type" json:"job_type"`
    Salary        []string `yaml:"salary" json:"salary"`
    Experience    []string `yaml:"experience" json:"experience"`
    Degree        []string `yaml:"degree" json:"degree"`
    Scale         []string `yaml:"scale" json:"scale"`
    Industry      []string `yaml:"industry" json:"industry"`
    Stage         []string `yaml:"stage" json:"stage"`
    EnableAI      bool     `yaml:"enable_ai" json:"enable_ai"`
    FilterDeadHR  bool     `yaml:"filter_dead_hr" json:"filter_dead_hr"`
    SendImgResume bool     `yaml:"send_img_resume" json:"send_img_resume"`
    ExpectedSalary []int   `yaml:"expected_salary" json:"expected_salary"`
    WaitTime      string   `yaml:"wait_time" json:"wait_time"`
    DeadStatus    []string `yaml:"dead_status" json:"dead_status"`
}

type JobDetail struct {
    EncryptID      string `json:"encrypt_id"`
    EncryptUserID  string `json:"encrypt_user_id"`
    JobName        string `json:"job_name"`
    Salary         string `json:"salary"`
    CompanyName    string `json:"company_name"`
    Recruiter      string `json:"recruiter"`
    JobInfo        string `json:"job_info"`
    JobArea        string `json:"job_area"`
    HRActiveStatus string `json:"hr_active_status"`
    HRPosition     string `json:"hr_position"`
}