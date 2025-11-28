package api

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gowvp/gb28181/pkg/ai"
	"github.com/ixugo/goddd/pkg/reason"
	"github.com/ixugo/goddd/pkg/web"
)

// AIAPI AI 检测 API
type AIAPI struct {
	aiService *ai.AIService
}

// NewAIAPI 创建 AI API
func NewAIAPI(aiService *ai.AIService) AIAPI {
	return AIAPI{aiService: aiService}
}

// registerAIAPI 注册 AI 检测 API
func registerAIAPI(g gin.IRouter, api AIAPI, handler ...gin.HandlerFunc) {
	group := g.Group("/ai", handler...)

	// 检测接口
	group.POST("/detect", web.WrapH(api.detect))

	// 告警规则管理
	group.GET("/rules", web.WrapH(api.listRules))
	group.POST("/rules", web.WrapH(api.createRule))
	group.GET("/rules/:id", web.WrapH(api.getRule))
	group.PUT("/rules/:id", web.WrapH(api.updateRule))
	group.DELETE("/rules/:id", web.WrapH(api.deleteRule))

	// 服务状态
	group.GET("/status", web.WrapH(api.getStatus))
}

// detectInput 检测请求输入
type detectInput struct {
	ChannelID string `json:"channel_id" binding:"required"` // 通道 ID
	ImageData string `json:"image_data" binding:"required"` // Base64 编码的图片数据
	Type      string `json:"type"`                          // 检测类型: pedestrian/vehicle/face/object
}

// detectOutput 检测响应输出
type detectOutput struct {
	Success     bool                `json:"success"`
	Results     []ai.DetectionResult `json:"results"`
	ProcessTime float64             `json:"process_time_ms"`
	Alerts      []*ai.Alert         `json:"alerts,omitempty"`
}

// detect 执行 AI 检测
func (a AIAPI) detect(c *gin.Context, in *detectInput) (*detectOutput, error) {
	if a.aiService == nil || !a.aiService.IsEnabled() {
		return nil, reason.ErrServer.SetMsg("AI service is not enabled")
	}

	detectionType := ai.DetectionTypePedestrian
	if in.Type != "" {
		detectionType = ai.DetectionType(in.Type)
	}

	start := time.Now()
	resp, err := a.aiService.DetectBase64(c.Request.Context(), in.ImageData, detectionType)
	if err != nil {
		return nil, reason.ErrServer.SetMsg(err.Error())
	}
	elapsed := float64(time.Since(start).Microseconds()) / 1000.0

	// 处理检测结果，触发告警
	alerts := a.aiService.ProcessDetection(in.ChannelID, resp.Results)

	// 发送告警通知
	for _, alert := range alerts {
		NotifyAIAlert(alert)
	}

	return &detectOutput{
		Success:     resp.Success,
		Results:     resp.Results,
		ProcessTime: elapsed,
		Alerts:      alerts,
	}, nil
}

// ruleInput 告警规则输入
type ruleInput struct {
	ChannelID       string              `json:"channel_id" binding:"required"`
	DetectionType   string              `json:"detection_type" binding:"required"`
	Enabled         bool                `json:"enabled"`
	Threshold       float64             `json:"threshold"`
	CooldownSeconds int                 `json:"cooldown_seconds"`
	Region          *ai.DetectionRegion `json:"region,omitempty"`
}

// ruleOutput 告警规则输出
type ruleOutput struct {
	*ai.AlertRule
}

// listRules 列出所有告警规则
func (a AIAPI) listRules(c *gin.Context, _ *struct{}) (gin.H, error) {
	if a.aiService == nil {
		return gin.H{"rules": []any{}}, nil
	}
	rules := a.aiService.GetAllRules()
	return gin.H{"rules": rules}, nil
}

// createRule 创建告警规则
func (a AIAPI) createRule(c *gin.Context, in *ruleInput) (*ruleOutput, error) {
	if a.aiService == nil {
		return nil, reason.ErrServer.SetMsg("AI service is not available")
	}

	if in.Threshold <= 0 || in.Threshold > 1 {
		in.Threshold = 0.5
	}
	if in.CooldownSeconds <= 0 {
		in.CooldownSeconds = 60
	}

	rule := &ai.AlertRule{
		ID:              uuid.NewString(),
		ChannelID:       in.ChannelID,
		DetectionType:   ai.DetectionType(in.DetectionType),
		Enabled:         in.Enabled,
		Threshold:       in.Threshold,
		CooldownSeconds: in.CooldownSeconds,
		Region:          in.Region,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	a.aiService.AddRule(rule)

	return &ruleOutput{AlertRule: rule}, nil
}

// getRule 获取告警规则
func (a AIAPI) getRule(c *gin.Context, _ *struct{}) (*ruleOutput, error) {
	if a.aiService == nil {
		return nil, reason.ErrServer.SetMsg("AI service is not available")
	}

	ruleID := c.Param("id")
	rule, ok := a.aiService.GetRule(ruleID)
	if !ok {
		return nil, reason.ErrNotFound.SetMsg("rule not found")
	}

	return &ruleOutput{AlertRule: rule}, nil
}

// updateRule 更新告警规则
func (a AIAPI) updateRule(c *gin.Context, in *ruleInput) (*ruleOutput, error) {
	if a.aiService == nil {
		return nil, reason.ErrServer.SetMsg("AI service is not available")
	}

	ruleID := c.Param("id")
	rule, ok := a.aiService.GetRule(ruleID)
	if !ok {
		return nil, reason.ErrNotFound.SetMsg("rule not found")
	}

	// 更新规则
	rule.ChannelID = in.ChannelID
	rule.DetectionType = ai.DetectionType(in.DetectionType)
	rule.Enabled = in.Enabled
	if in.Threshold > 0 && in.Threshold <= 1 {
		rule.Threshold = in.Threshold
	}
	if in.CooldownSeconds > 0 {
		rule.CooldownSeconds = in.CooldownSeconds
	}
	rule.Region = in.Region
	rule.UpdatedAt = time.Now()

	a.aiService.AddRule(rule) // 更新规则

	return &ruleOutput{AlertRule: rule}, nil
}

// deleteRule 删除告警规则
func (a AIAPI) deleteRule(c *gin.Context, _ *struct{}) (gin.H, error) {
	if a.aiService == nil {
		return nil, reason.ErrServer.SetMsg("AI service is not available")
	}

	ruleID := c.Param("id")
	a.aiService.RemoveRule(ruleID)

	return gin.H{"msg": "ok"}, nil
}

// statusOutput AI 服务状态输出
type statusOutput struct {
	Enabled       bool   `json:"enabled"`
	InferenceMode string `json:"inference_mode,omitempty"`
	Endpoint      string `json:"endpoint,omitempty"`
	ModelType     string `json:"model_type,omitempty"`
	ModelPath     string `json:"model_path,omitempty"`
	DeviceType    string `json:"device_type,omitempty"`
	RuleCount     int    `json:"rule_count"`
}

// getStatus 获取 AI 服务状态
func (a AIAPI) getStatus(c *gin.Context, _ *struct{}) (*statusOutput, error) {
	if a.aiService == nil {
		return &statusOutput{Enabled: false}, nil
	}

	config := a.aiService.GetConfig()
	return &statusOutput{
		Enabled:       a.aiService.IsEnabled(),
		InferenceMode: string(config.InferenceMode),
		Endpoint:      config.Endpoint,
		ModelType:     config.ModelType,
		ModelPath:     config.ModelPath,
		DeviceType:    config.DeviceType,
		RuleCount:     len(a.aiService.GetAllRules()),
	}, nil
}

// NotifyAIAlert 发送 AI 告警通知
func NotifyAIAlert(alert *ai.Alert) {
	hub := GetNotificationHub()
	hub.Broadcast(Notification{
		Type:    NotifyTypeAIAlert,
		Message: "AI 检测告警",
		Data: map[string]any{
			"alert_id":   alert.ID,
			"channel_id": alert.ChannelID,
			"rule_id":    alert.RuleID,
			"type":       alert.Type,
			"detections": alert.Detections,
			"created_at": alert.CreatedAt,
		},
	})
}
