package model

type Job struct {
	Href        string `json:"href"`        //岗位链接
	JobName     string `json:"jobName"`     //岗位名称
	JobArea     string `json:"jobArea"`     //岗位地区
	JobInfo     string `json:"jobInfo"`     //岗位信息
	Salary      string `json:"salary"`      //岗位薪水
	CompanyTag  string `json:"companyTag"`  //公司标签
	Recruiter   string `json:"recruiter"`   //HR名称
	CompanyName string `json:"companyName"` //公司名字
	CompanyInfo string `json:"companyInfo"` //公司信息
}
