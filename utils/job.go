package utils

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"
)

// Platform 平台枚举
type Platform int

const (
	ZHILIAN Platform = iota
	BOSS
	OTHER
)

// Job 职位信息结构体
type Job struct {
	// 岗位链接
	Href string `json:"href"`
	// 岗位名称
	JobName string `json:"jobName"`
	// 岗位地区
	JobArea string `json:"jobArea"`
	// 岗位信息
	JobInfo string `json:"jobInfo"`
	// 岗位薪水
	Salary string `json:"salary"`
	// 公司标签
	CompanyTag string `json:"companyTag"`
	// HR名称
	Recruiter string `json:"recruiter"`
	// 公司名字
	CompanyName string `json:"companyName"`
	// 公司信息
	CompanyInfo string `json:"companyInfo"`
}

// String 实现 Stringer 接口
func (j *Job) String() string {
	return fmt.Sprintf("【%s, %s, %s, %s, %s, %s】",
		j.CompanyName, j.JobName, j.JobArea, j.Salary, j.CompanyTag, j.Recruiter)
}

// ToStringWithPlatform 根据平台格式化输出
func (j *Job) ToStringWithPlatform(platform Platform) string {
	switch platform {
	case ZHILIAN:
		return fmt.Sprintf("【%s, %s, %s, %s, %s, %s, %s】",
			j.CompanyName, j.JobName, j.JobArea, j.CompanyTag, j.Salary, j.Recruiter, j.Href)
	case BOSS:
		return fmt.Sprintf("【%s, %s, %s, %s, %s, %s】",
			j.CompanyName, j.JobName, j.JobArea, j.Salary, j.CompanyTag, j.Recruiter)
	default:
		return fmt.Sprintf("【%s, %s, %s, %s, %s, %s, %s】",
			j.CompanyName, j.JobName, j.JobArea, j.Salary, j.CompanyTag, j.Recruiter, j.Href)
	}
}

// UNLIMITED_CODE 不限选项的代码
const UNLIMITED_CODE = "0"

// AppendParam 追加参数
func AppendParam(name, value string) string {
	if value == "" || value == UNLIMITED_CODE {
		return ""
	}
	return "&" + name + "=" + value
}

// AppendListParam 追加列表参数
// 如果列表包含 "0"（UNLIMITED_CODE），表示该参数不设置，直接返回空字符串
func AppendListParam(name string, values []string) string {
	if len(values) == 0 {
		return ""
	}

	// 检查是否包含不限选项
	for _, v := range values {
		if v == UNLIMITED_CODE {
			return ""
		}
	}

	return "&" + name + "=" + strings.Join(values, ",")
}

// FormatDuration 计算并格式化时间（毫秒）
// 返回格式化后的时间字符串，格式为 "HH时mm分ss秒"
func FormatDuration(startTime, endTime time.Time) string {
	duration := endTime.Sub(startTime)

	hours := int(duration.Hours())
	minutes := int(duration.Minutes()) % 60
	seconds := int(duration.Seconds()) % 60

	return fmt.Sprintf("%d时%d分%d秒", hours, minutes, seconds)
}

// FormatDurationSeconds 将给定的秒数转换为格式化的时间字符串
// 返回格式化后的时间字符串，格式为 "HH时mm分ss秒"
func FormatDurationSeconds(durationSeconds int64) string {
	hours := durationSeconds / 3600
	minutes := (durationSeconds % 3600) / 60
	seconds := durationSeconds % 60

	return fmt.Sprintf("%d时%d分%d秒", hours, minutes, seconds)
}

// GetRandomNumberInRange 获取指定范围内的随机数
func GetRandomNumberInRange(min, max int) int {
	if min > max {
		min, max = max, min
	}

	// 初始化随机数种子
	rand.Seed(time.Now().UnixNano())
	return rand.Intn(max-min+1) + min
}

// ParseInt 安全解析字符串为整数
func ParseInt(s string) int {
	if s == "" {
		return 0
	}

	val, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return val
}

// ParseFloat 安全解析字符串为浮点数
func ParseFloat(s string) float64 {
	if s == "" {
		return 0
	}

	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return val
}

// ContainsString 检查字符串切片是否包含指定字符串
func ContainsString(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}

// RemoveString 从字符串切片中移除指定字符串
func RemoveString(slice []string, str string) []string {
	result := make([]string, 0, len(slice))
	for _, s := range slice {
		if s != str {
			result = append(result, s)
		}
	}
	return result
}

// UniqueStrings 去除字符串切片中的重复元素
func UniqueStrings(slice []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(slice))

	for _, s := range slice {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

// IsEmpty 检查字符串是否为空（去除空格后）
func IsEmpty(s string) bool {
	return strings.TrimSpace(s) == ""
}

// DefaultIfEmpty 如果字符串为空，返回默认值
func DefaultIfEmpty(s, defaultValue string) string {
	if IsEmpty(s) {
		return defaultValue
	}
	return s
}

// Sleep 睡眠指定秒数
func Sleep(seconds int) {
	time.Sleep(time.Duration(seconds) * time.Second)
}

// SleepRandom 在指定范围内随机睡眠
func SleepRandom(minSeconds, maxSeconds int) {
	duration := GetRandomNumberInRange(minSeconds, maxSeconds)
	time.Sleep(time.Duration(duration) * time.Second)
}

// 示例使用函数
func ExampleUsage() {
	// 创建一个职位对象
	job := &Job{
		CompanyName: "测试公司",
		JobName:     "Go开发工程师",
		JobArea:     "北京",
		Salary:      "20-30K",
		CompanyTag:  "互联网",
		Recruiter:   "张HR",
		Href:        "https://example.com/job/123",
	}

	// 输出职位信息
	fmt.Println(job.String())
	fmt.Println(job.ToStringWithPlatform(BOSS))

	// 测试时间格式化
	start := time.Now()
	time.Sleep(2 * time.Second)
	end := time.Now()
	fmt.Println("耗时:", FormatDuration(start, end))

	// 测试随机数
	fmt.Println("随机数:", GetRandomNumberInRange(1, 100))
}
