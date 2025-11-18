package config

// BossConfig Boss直聘配置数据结构
type BossConfig struct {
	SayHi         string            `json:"sayHi"`
	Debugger      bool              `json:"debugger"`
	Keywords      []string          `json:"keywords"`
	CityCode      []string          `json:"cityCode"`
	CustomCityCode map[string]string `json:"customCityCode"`
	Industry      []string          `json:"industry"`
	Experience    []string          `json:"experience"`
	JobType       string            `json:"jobType"`
	Salary        []string          `json:"salary"`
	Degree        []string          `json:"degree"`
	Scale         []string          `json:"scale"`
	Stage         []string          `json:"stage"`
	EnableAI      bool              `json:"enableAI"`
	FilterDeadHR  bool              `json:"filterDeadHR"`
	SendImgResume bool              `json:"sendImgResume"`
	ExpectedSalary []int            `json:"expectedSalary"`
	WaitTime      string            `json:"waitTime"`
	DeadStatus    []string          `json:"deadStatus"`
}