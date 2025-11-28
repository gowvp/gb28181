package ai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
)

// DetectionType 检测类型
type DetectionType string

const (
	DetectionTypePedestrian DetectionType = "pedestrian" // 行人检测
	DetectionTypeVehicle    DetectionType = "vehicle"    // 车辆检测
	DetectionTypeFace       DetectionType = "face"       // 人脸检测
	DetectionTypeObject     DetectionType = "object"     // 通用物体检测
)

// InferenceMode 推理模式
type InferenceMode string

const (
	InferenceModeLocal  InferenceMode = "local"  // 本地推理
	InferenceModeRemote InferenceMode = "remote" // 远程 API
)

// DetectionResult 检测结果
type DetectionResult struct {
	Type        DetectionType `json:"type"`
	Label       string        `json:"label"`
	Confidence  float64       `json:"confidence"`
	BoundingBox struct {
		X      int `json:"x"`
		Y      int `json:"y"`
		Width  int `json:"width"`
		Height int `json:"height"`
	} `json:"bounding_box"`
}

// DetectionResponse AI 服务响应
type DetectionResponse struct {
	Success     bool              `json:"success"`
	Message     string            `json:"message,omitempty"`
	Results     []DetectionResult `json:"results"`
	ProcessTime float64           `json:"process_time_ms,omitempty"`
}

// AlertRule 告警规则
type AlertRule struct {
	ID              string           `json:"id"`
	ChannelID       string           `json:"channel_id"`
	DetectionType   DetectionType    `json:"detection_type"`
	Enabled         bool             `json:"enabled"`
	Threshold       float64          `json:"threshold"`
	CooldownSeconds int              `json:"cooldown_seconds"`
	Region          *DetectionRegion `json:"region,omitempty"`
	CreatedAt       time.Time        `json:"created_at"`
	UpdatedAt       time.Time        `json:"updated_at"`
}

// DetectionRegion 检测区域
type DetectionRegion struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// Alert 告警事件
type Alert struct {
	ID          string            `json:"id"`
	ChannelID   string            `json:"channel_id"`
	RuleID      string            `json:"rule_id"`
	Type        DetectionType     `json:"type"`
	Detections  []DetectionResult `json:"detections"`
	SnapshotURL string            `json:"snapshot_url,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
}

// AIServiceConfig AI 服务配置
type AIServiceConfig struct {
	Enabled       bool          `json:"enabled" toml:"enabled"`
	InferenceMode InferenceMode `json:"inference_mode" toml:"inference_mode"`
	Endpoint      string        `json:"endpoint" toml:"endpoint"`
	APIKey        string        `json:"api_key" toml:"api_key"`
	Timeout       int           `json:"timeout" toml:"timeout"`
	ModelType     string        `json:"model_type" toml:"model_type"`
	ModelPath     string        `json:"model_path" toml:"model_path"`
	DeviceType    string        `json:"device_type" toml:"device_type"`
}

// LocalInferencer 本地推理接口
type LocalInferencer interface {
	Detect(ctx context.Context, imageData []byte, detectionType DetectionType) (*DetectionResponse, error)
	IsAvailable() bool
	Close() error
}

// AIService AI 检测服务
type AIService struct {
	config          AIServiceConfig
	client          *http.Client
	localInferencer LocalInferencer
	rules           map[string]*AlertRule
	rulesMu         sync.RWMutex
	lastAlerts      map[string]time.Time
	alertsMu        sync.RWMutex
	alertChan       chan *Alert
}

// NewAIService 创建 AI 服务
func NewAIService(config AIServiceConfig) *AIService {
	return &AIService{
		config: config,
		client: &http.Client{
			Timeout: time.Duration(config.Timeout) * time.Second,
		},
		rules:      make(map[string]*AlertRule),
		lastAlerts: make(map[string]time.Time),
		alertChan:  make(chan *Alert, 100),
	}
}

// SetLocalInferencer 设置本地推理引擎
func (s *AIService) SetLocalInferencer(inferencer LocalInferencer) {
	s.localInferencer = inferencer
}

// IsEnabled 检查服务是否启用
func (s *AIService) IsEnabled() bool {
	if !s.config.Enabled {
		return false
	}
	if s.config.InferenceMode == InferenceModeLocal {
		return s.localInferencer != nil && s.localInferencer.IsAvailable()
	}
	return s.config.Endpoint != ""
}

// GetConfig 获取配置
func (s *AIService) GetConfig() AIServiceConfig {
	return s.config
}

// Detect 执行检测
func (s *AIService) Detect(ctx context.Context, imageData []byte, detectionType DetectionType) (*DetectionResponse, error) {
	if !s.config.Enabled {
		return nil, fmt.Errorf("AI service is not enabled")
	}
	if s.config.InferenceMode == InferenceModeLocal {
		return s.detectLocal(ctx, imageData, detectionType)
	}
	return s.detectRemote(ctx, imageData, detectionType)
}

// detectLocal 本地推理
func (s *AIService) detectLocal(ctx context.Context, imageData []byte, detectionType DetectionType) (*DetectionResponse, error) {
	if s.localInferencer == nil {
		return nil, fmt.Errorf("local inferencer is not configured")
	}
	if !s.localInferencer.IsAvailable() {
		return nil, fmt.Errorf("local inferencer is not available")
	}
	return s.localInferencer.Detect(ctx, imageData, detectionType)
}

// detectRemote 远程 API 推理
func (s *AIService) detectRemote(ctx context.Context, imageData []byte, detectionType DetectionType) (*DetectionResponse, error) {
	if s.config.Endpoint == "" {
		return nil, fmt.Errorf("AI service endpoint is not configured")
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("image", "snapshot.jpg")
	if err != nil {
		return nil, fmt.Errorf("create form file: %w", err)
	}
	if _, err := part.Write(imageData); err != nil {
		return nil, fmt.Errorf("write image data: %w", err)
	}

	if err := writer.WriteField("type", string(detectionType)); err != nil {
		return nil, fmt.Errorf("write type field: %w", err)
	}

	if s.config.ModelType != "" {
		if err := writer.WriteField("model", s.config.ModelType); err != nil {
			return nil, fmt.Errorf("write model field: %w", err)
		}
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", s.config.Endpoint+"/detect", body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if s.config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+s.config.APIKey)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("AI service returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result DetectionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

// DetectBase64 使用 Base64 编码的图片执行检测
func (s *AIService) DetectBase64(ctx context.Context, base64Image string, detectionType DetectionType) (*DetectionResponse, error) {
	imageData, err := base64.StdEncoding.DecodeString(base64Image)
	if err != nil {
		return nil, fmt.Errorf("decode base64: %w", err)
	}
	return s.Detect(ctx, imageData, detectionType)
}

// AddRule 添加告警规则
func (s *AIService) AddRule(rule *AlertRule) {
	s.rulesMu.Lock()
	defer s.rulesMu.Unlock()
	s.rules[rule.ID] = rule
}

// RemoveRule 移除告警规则
func (s *AIService) RemoveRule(ruleID string) {
	s.rulesMu.Lock()
	defer s.rulesMu.Unlock()
	delete(s.rules, ruleID)
}

// GetRule 获取告警规则
func (s *AIService) GetRule(ruleID string) (*AlertRule, bool) {
	s.rulesMu.RLock()
	defer s.rulesMu.RUnlock()
	rule, ok := s.rules[ruleID]
	return rule, ok
}

// GetRulesByChannel 获取通道的所有告警规则
func (s *AIService) GetRulesByChannel(channelID string) []*AlertRule {
	s.rulesMu.RLock()
	defer s.rulesMu.RUnlock()
	var result []*AlertRule
	for _, rule := range s.rules {
		if rule.ChannelID == channelID && rule.Enabled {
			result = append(result, rule)
		}
	}
	return result
}

// GetAllRules 获取所有告警规则
func (s *AIService) GetAllRules() []*AlertRule {
	s.rulesMu.RLock()
	defer s.rulesMu.RUnlock()
	var result []*AlertRule
	for _, rule := range s.rules {
		result = append(result, rule)
	}
	return result
}

// ProcessDetection 处理检测结果并触发告警
func (s *AIService) ProcessDetection(channelID string, results []DetectionResult) []*Alert {
	rules := s.GetRulesByChannel(channelID)
	if len(rules) == 0 {
		return nil
	}

	var alerts []*Alert
	now := time.Now()

	for _, rule := range rules {
		s.alertsMu.RLock()
		lastAlert, exists := s.lastAlerts[rule.ID]
		s.alertsMu.RUnlock()

		if exists && now.Sub(lastAlert) < time.Duration(rule.CooldownSeconds)*time.Second {
			continue
		}

		var matchedResults []DetectionResult
		for _, r := range results {
			if r.Type == rule.DetectionType && r.Confidence >= rule.Threshold {
				if rule.Region != nil {
					if !isInRegion(r.BoundingBox.X, r.BoundingBox.Y, r.BoundingBox.Width, r.BoundingBox.Height, rule.Region) {
						continue
					}
				}
				matchedResults = append(matchedResults, r)
			}
		}

		if len(matchedResults) > 0 {
			alert := &Alert{
				ID:         uuid.NewString(),
				ChannelID:  channelID,
				RuleID:     rule.ID,
				Type:       rule.DetectionType,
				Detections: matchedResults,
				CreatedAt:  now,
			}
			alerts = append(alerts, alert)

			s.alertsMu.Lock()
			s.lastAlerts[rule.ID] = now
			s.alertsMu.Unlock()

			select {
			case s.alertChan <- alert:
			default:
				slog.Warn("alert channel full, dropping alert", "alert_id", alert.ID)
			}
		}
	}

	return alerts
}

// AlertChannel 获取告警通道
func (s *AIService) AlertChannel() <-chan *Alert {
	return s.alertChan
}

// Close 关闭服务
func (s *AIService) Close() error {
	if s.localInferencer != nil {
		return s.localInferencer.Close()
	}
	return nil
}

func isInRegion(x, y, w, h int, region *DetectionRegion) bool {
	centerX := x + w/2
	centerY := y + h/2
	return centerX >= region.X && centerX <= region.X+region.Width &&
		centerY >= region.Y && centerY <= region.Y+region.Height
}

// DefaultAIServiceConfig 默认 AI 服务配置
func DefaultAIServiceConfig() AIServiceConfig {
	return AIServiceConfig{
		Enabled:       false,
		InferenceMode: InferenceModeRemote,
		Endpoint:      "http://localhost:8080",
		APIKey:        "",
		Timeout:       30,
		ModelType:     "yolov8",
		ModelPath:     "./models/yolov8n.onnx",
		DeviceType:    "cpu",
	}
}
