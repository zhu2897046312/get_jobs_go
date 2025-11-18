package service

import (
	"bytes"
	"encoding/json"
	"get_jobs_go/model"
	"get_jobs_go/repository"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// ConfigProvider 配置提供者接口（模拟Java中的ConfigService）
type ConfigProvider interface {
	GetAiConfigs() map[string]string
}

// AiService AI服务
type AiService struct {
	aiRepo        repository.AiRepository
	configService ConfigProvider
	httpClient    *http.Client
}

func NewAiService(aiRepo repository.AiRepository, configService ConfigProvider) *AiService {
	return &AiService{
		aiRepo:     aiRepo,
		configService: configService,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// AI请求结构体
type aiRequest struct {
	Model       string      `json:"model"`
	Temperature float64     `json:"temperature,omitempty"`
	Input       string      `json:"input,omitempty"`
	Messages    []aiMessage `json:"messages,omitempty"`
}

type aiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// AI响应结构体
type aiResponse struct {
	ID      string    `json:"id"`
	Model   string    `json:"model"`
	Created int64     `json:"created"`
	Choices []aiChoice `json:"choices,omitempty"`
	Usage   aiUsage   `json:"usage,omitempty"`
	OutputText string `json:"output_text,omitempty"` // Responses API 字段
}

type aiChoice struct {
	Message aiMessage `json:"message"`
}

type aiUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// SendRequest 发送AI请求并返回回复内容
func (s *AiService) SendRequest(content string) (string, error) {
	// 读取并校验配置
	cfg := s.configService.GetAiConfigs()
	baseUrl := cfg["BASE_URL"]
	apiKey := cfg["API_KEY"]
	model := cfg["MODEL"]
	
	if baseUrl == "" || apiKey == "" || model == "" {
		return "", fmt.Errorf("AI配置不完整: BASE_URL=%s, API_KEY=%s, MODEL=%s", baseUrl, apiKey, model)
	}

	// 根据模型类型选择端点
	endpoint := s.buildEndpoint(baseUrl, model)

	// 构建请求体
	requestData := s.buildRequestData(model, content, endpoint)

	// 创建HTTP请求
	req, err := s.createHttpRequest(endpoint, apiKey, requestData)
	if err != nil {
		return "", fmt.Errorf("创建HTTP请求失败: %v", err)
	}

	// 发送请求
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("AI请求失败: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应体失败: %v", err)
	}

	if resp.StatusCode == http.StatusOK {
		return s.parseSuccessResponse(body, endpoint)
	} else {
		return s.handleErrorResponse(body, endpoint, content, apiKey, model)
	}
}

// buildEndpoint 构建API端点
func (s *AiService) buildEndpoint(baseUrl, model string) string {
	normalized := s.normalizeBaseUrl(baseUrl)
	
	if s.isResponsesModel(model) {
		if strings.Contains(normalized, "/v1") {
			return normalized + "/responses"
		}
		return normalized + "/v1/responses"
	} else {
		if strings.Contains(normalized, "/v1") {
			return normalized + "/chat/completions"
		}
		return normalized + "/v1/chat/completions"
	}
}

// buildRequestData 构建请求数据
func (s *AiService) buildRequestData(model, content, endpoint string) aiRequest {
	requestData := aiRequest{
		Model:       model,
		Temperature: 0.5,
	}

	if strings.HasSuffix(endpoint, "/responses") {
		requestData.Input = content
	} else {
		requestData.Messages = []aiMessage{
			{Role: "user", Content: content},
		}
	}

	return requestData
}

// createHttpRequest 创建HTTP请求
func (s *AiService) createHttpRequest(endpoint, apiKey string, requestData aiRequest) (*http.Request, error) {
	jsonData, err := json.Marshal(requestData)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("api-key", apiKey) // 兼容Azure OpenAI

	return req, nil
}

// parseSuccessResponse 解析成功响应
func (s *AiService) parseSuccessResponse(body []byte, endpoint string) (string, error) {
	var responseObj aiResponse
	if err := json.Unmarshal(body, &responseObj); err != nil {
		return "", fmt.Errorf("解析响应JSON失败: %v", err)
	}

	var responseContent string
	if strings.HasSuffix(endpoint, "/responses") {
		// Responses API
		responseContent = responseObj.OutputText
		if responseContent == "" {
			// 兜底解析
			if len(responseObj.Choices) > 0 {
				responseContent = responseObj.Choices[0].Message.Content
			} else {
				responseContent = string(body) // 最后兜底
			}
		}
	} else {
		// Chat Completions API
		if len(responseObj.Choices) > 0 {
			responseContent = responseObj.Choices[0].Message.Content
		} else {
			return "", fmt.Errorf("响应中没有choices字段")
		}
	}

	// 记录日志
	createdTime := time.Now()
	if responseObj.Created > 0 {
		createdTime = time.Unix(responseObj.Created, 0)
	}

	log.Printf("AI响应: id=%s, time=%s, model=%s, promptTokens=%d, completionTokens=%d, totalTokens=%d",
		responseObj.ID, createdTime.Format("2006-01-02 15:04:05"), responseObj.Model,
		responseObj.Usage.PromptTokens, responseObj.Usage.CompletionTokens, responseObj.Usage.TotalTokens)

	return responseContent, nil
}

// handleErrorResponse 处理错误响应
func (s *AiService) handleErrorResponse(body []byte, endpoint, content, apiKey, model string) (string, error) {
	bodyStr := string(body)
	log.Printf("AI请求失败: endpoint=%s, body=%s", endpoint, bodyStr)

	// 检查是否需要重试到Responses API
	if !strings.HasSuffix(endpoint, "/responses") && s.containsReasoningParamError(bodyStr) {
		log.Printf("检测到 reasoning 相关参数错误，自动切换到 Responses API 重试")
		fallbackEndpoint := s.buildResponsesEndpoint(s.normalizeBaseUrl(s.configService.GetAiConfigs()["BASE_URL"]))
		return s.sendRequestViaResponses(content, apiKey, model, fallbackEndpoint)
	}

	return "", fmt.Errorf("AI请求失败，状态码，详情: %s", bodyStr)
}

// sendRequestViaResponses 使用Responses API发送请求（用于重试）
func (s *AiService) sendRequestViaResponses(content, apiKey, model, endpoint string) (string, error) {
	requestData := aiRequest{
		Model:       model,
		Temperature: 0.5,
		Input:       content,
	}

	req, err := s.createHttpRequest(endpoint, apiKey, requestData)
	if err != nil {
		return "", err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode == http.StatusOK {
		var responseObj aiResponse
		if err := json.Unmarshal(body, &responseObj); err != nil {
			return "", err
		}

		if responseObj.OutputText != "" {
			return responseObj.OutputText, nil
		}

		// 兜底解析
		if len(responseObj.Choices) > 0 {
			return responseObj.Choices[0].Message.Content, nil
		}

		return string(body), nil // 最后兜底
	}

	log.Printf("Responses API 调用失败: endpoint=%s, body=%s", endpoint, string(body))
	return "", fmt.Errorf("AI请求失败，状态码，详情: %s", string(body))
}

// ================= 工具方法 =================

func (s *AiService) normalizeBaseUrl(baseUrl string) string {
	if baseUrl == "" {
		return ""
	}
	trimmed := strings.TrimSpace(baseUrl)
	if strings.HasSuffix(trimmed, "/") {
		return trimmed[:len(trimmed)-1]
	}
	return trimmed
}

func (s *AiService) buildResponsesEndpoint(baseUrl string) string {
	normalized := s.normalizeBaseUrl(baseUrl)
	if strings.Contains(normalized, "/v1") {
		return normalized + "/responses"
	}
	return normalized + "/v1/responses"
}

func (s *AiService) isResponsesModel(model string) bool {
	if model == "" {
		return false
	}
	modelLower := strings.ToLower(model)
	return strings.Contains(modelLower, "o1") || strings.Contains(modelLower, "o3") || 
	       strings.Contains(modelLower, "o4") || strings.Contains(modelLower, "4.1") ||
	       strings.Contains(modelLower, "reasoner") || strings.Contains(modelLower, "4o-mini") ||
	       strings.Contains(modelLower, "gpt-4o-mini")
}

func (s *AiService) containsReasoningParamError(body string) bool {
	bodyLower := strings.ToLower(body)
	return (strings.Contains(bodyLower, "reasoning") && strings.Contains(bodyLower, "unsupported_value")) ||
	       strings.Contains(bodyLower, "reasoning.summary")
}

// ================= AI配置管理方法 =================

// GetAiConfig 获取AI配置（获取最新一条，如果不存在则创建默认配置）
func (s *AiService) GetAiConfig() (*model.AiEntity, error) {
	aiEntity, err := s.aiRepo.FindLatest()
	if err != nil {
		return nil, err
	}

	if aiEntity == nil {
		return s.createDefaultConfig()
	}

	return aiEntity, nil
}

// GetAllAiConfigs 获取所有AI配置
func (s *AiService) GetAllAiConfigs() ([]*model.AiEntity, error) {
	return s.aiRepo.FindAll()
}

// GetAiConfigById 根据ID获取AI配置
func (s *AiService) GetAiConfigById(id int64) (*model.AiEntity, error) {
	return s.aiRepo.FindByID(id)
}

// SaveOrUpdateAiConfig 保存或更新AI配置
func (s *AiService) SaveOrUpdateAiConfig(introduce, prompt string) (*model.AiEntity, error) {
	aiEntity, err := s.aiRepo.FindLatest()
	if err != nil {
		return nil, err
	}

	now := time.Now()
	if aiEntity == nil {
		aiEntity = &model.AiEntity{
			Introduce: introduce,
			Prompt:    prompt,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := s.aiRepo.Save(aiEntity); err != nil {
			return nil, err
		}
	} else {
		aiEntity.Introduce = introduce
		aiEntity.Prompt = prompt
		aiEntity.UpdatedAt = now
		if err := s.aiRepo.Update(aiEntity); err != nil {
			return nil, err
		}
	}

	return aiEntity, nil
}

// DeleteAiConfig 删除AI配置
func (s *AiService) DeleteAiConfig(id int64) (bool, error) {
	err := s.aiRepo.Delete(id)
	if err != nil {
		return false, err
	}
	return true, nil
}

// createDefaultConfig 创建默认配置
func (s *AiService) createDefaultConfig() (*model.AiEntity, error) {
	aiEntity := &model.AiEntity{
		Introduce: "请在此填写您的技能介绍",
		Prompt:    "请在此填写AI提示词模板",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	
	if err := s.aiRepo.Save(aiEntity); err != nil {
		return nil, err
	}
	
	log.Printf("创建默认AI配置，ID: %d", aiEntity.ID)
	return aiEntity, nil
}