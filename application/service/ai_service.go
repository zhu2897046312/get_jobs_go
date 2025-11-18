// internal/ai/ai_service.go
package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

type AIService struct {
    baseURL string
    apiKey  string
    model   string
    client  *http.Client
}

type AIRequest struct {
    Model       string    `json:"model"`
    Messages    []Message `json:"messages,omitempty"`
    Input       string    `json:"input,omitempty"`
    Temperature float64   `json:"temperature"`
}

type Message struct {
    Role    string `json:"role"`
    Content string `json:"content"`
}

type AIResponse struct {
    ID      string `json:"id"`
    Object  string `json:"object"`
    Created int64  `json:"created"`
    Model   string `json:"model"`
    Choices []struct {
        Message struct {
            Role    string `json:"role"`
            Content string `json:"content"`
        } `json:"message"`
    } `json:"choices"`
    Usage struct {
        PromptTokens     int `json:"prompt_tokens"`
        CompletionTokens int `json:"completion_tokens"`
        TotalTokens      int `json:"total_tokens"`
    } `json:"usage"`
    OutputText string `json:"output_text,omitempty"` // Responses API 专用字段
}

func NewAIService(baseURL, apiKey, model string) *AIService {
    return &AIService{
        baseURL: strings.TrimSuffix(baseURL, "/"),
        apiKey:  apiKey,
        model:   model,
        client: &http.Client{
            Timeout: 60 * time.Second,
        },
    }
}

// GenerateGreeting 生成AI招呼语
func (a *AIService) GenerateGreeting(introduce, keyword, jobName, jobDesc, reference string) (string, error) {
    prompt := a.buildPrompt(introduce, keyword, jobName, jobDesc, reference)
    
    // 根据模型类型选择API端点
    endpoint := a.buildEndpoint()
    
    req := AIRequest{
        Model:       a.model,
        Temperature: 0.5,
    }
    
    if a.isResponsesModel() {
        req.Input = prompt
    } else {
        req.Messages = []Message{
            {
                Role:    "user",
                Content: prompt,
            },
        }
    }
    
    response, err := a.sendRequest(endpoint, req)
    if err != nil {
        log.Printf("AI请求失败: %v", err)
        return reference, err
    }
    
    // 检查AI返回内容是否有效
    if response == "" || strings.Contains(strings.ToLower(response), "false") {
        return reference, nil
    }
    
    return response, nil
}

// sendRequest 发送AI请求
func (a *AIService) sendRequest(endpoint string, req AIRequest) (string, error) {
    jsonData, err := json.Marshal(req)
    if err != nil {
        return "", fmt.Errorf("序列化请求数据失败: %w", err)
    }
    
    httpReq, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(jsonData))
    if err != nil {
        return "", fmt.Errorf("创建HTTP请求失败: %w", err)
    }
    
    httpReq.Header.Set("Content-Type", "application/json")
    httpReq.Header.Set("Authorization", "Bearer "+a.apiKey)
    httpReq.Header.Set("api-key", a.apiKey) // 兼容Azure OpenAI
    
    resp, err := a.client.Do(httpReq)
    if err != nil {
        return "", fmt.Errorf("发送AI请求失败: %w", err)
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return "", fmt.Errorf("AI服务返回错误状态码: %d, 响应: %s", resp.StatusCode, string(body))
    }
    
    var aiResp AIResponse
    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return "", fmt.Errorf("读取响应体失败: %w", err)
    }
    
    if err := json.Unmarshal(body, &aiResp); err != nil {
        return "", fmt.Errorf("解析AI响应失败: %w", err)
    }
    
    // 记录使用情况
    log.Printf("AI响应: id=%s, model=%s, promptTokens=%d, completionTokens=%d, totalTokens=%d",
        aiResp.ID, aiResp.Model, aiResp.Usage.PromptTokens, aiResp.Usage.CompletionTokens, aiResp.Usage.TotalTokens)
    
    // 提取回复内容
    return a.extractResponseContent(&aiResp, body), nil
}

// buildEndpoint 构建API端点
func (a *AIService) buildEndpoint() string {
    if a.isResponsesModel() {
        if strings.Contains(a.baseURL, "/v1") {
            return a.baseURL + "/responses"
        }
        return a.baseURL + "/v1/responses"
    } else {
        if strings.Contains(a.baseURL, "/v1") {
            return a.baseURL + "/chat/completions"
        }
        return a.baseURL + "/v1/chat/completions"
    }
}

// isResponsesModel 判断是否使用Responses API的模型
func (a *AIService) isResponsesModel() bool {
    model := strings.ToLower(a.model)
    return strings.Contains(model, "o1") || strings.Contains(model, "o3") || 
           strings.Contains(model, "o4") || strings.Contains(model, "4.1") ||
           strings.Contains(model, "reasoner") || strings.Contains(model, "4o-mini")
}

// extractResponseContent 从响应中提取内容
func (a *AIService) extractResponseContent(resp *AIResponse, rawBody []byte) string {
    // 优先使用Responses API的输出
    if resp.OutputText != "" {
        return resp.OutputText
    }
    
    // 使用Chat Completions API的输出
    if len(resp.Choices) > 0 && resp.Choices[0].Message.Content != "" {
        return resp.Choices[0].Message.Content
    }
    
    // 兜底：返回原始响应
    return string(rawBody)
}

// buildPrompt 构建提示词
func (a *AIService) buildPrompt(introduce, keyword, jobName, jobDesc, reference string) string {
    if introduce == "" {
        introduce = "具备相关技能和经验"
    }
    
    return fmt.Sprintf(`请基于以下信息生成简洁友好的中文打招呼语，不超过60字：

个人介绍：%s
关键词：%s
职位名称：%s
职位描述：%s
参考语：%s

请生成专业、简洁的打招呼语，突出个人优势与职位匹配度。`, introduce, keyword, jobName, jobDesc, reference)
}

// 配置相关方法
type AIConfig struct {
    BaseURL string `yaml:"base_url" json:"base_url"`
    APIKey  string `yaml:"api_key" json:"api_key"`
    Model   string `yaml:"model" json:"model"`
}

// LoadAIConfig 从配置文件加载AI配置
func LoadAIConfig() (*AIConfig, error) {
    // 这里可以从配置文件、环境变量或数据库加载配置
    // 简化实现，实际使用时应该从配置源加载
    return &AIConfig{
        BaseURL: "https://api.openai.com/v1",
        APIKey:  "your-api-key-here",
        Model:   "gpt-3.5-turbo",
    }, nil
}