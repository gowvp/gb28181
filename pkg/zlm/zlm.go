package zlm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"time"
)

const (
	Exception   = -400 // 代码抛异常
	InvalidArgs = -300 // 参数不合法
	SQLFailed   = -200 // sql执行失败
	AuthFailed  = -100 // 鉴权失败
	OtherFailed = -1   // 业务代码执行失败
	Success     = 0    // 执行成功
)

type Config struct {
	URL    string
	Secret string
}

type Engine struct {
	cfg Config
	cli *http.Client
}

func NewEngine() Engine {
	return Engine{
		cli: &http.Client{
			Timeout: 5 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        30,
				MaxIdleConnsPerHost: 30,
				MaxConnsPerHost:     100,
			},
		},
	}
}

func (e Engine) SetConfig(cfg Config) Engine {
	e.cfg = cfg
	return e
}

func (e *Engine) post(path string, data map[string]any, out any) error {
	bodyMap := make(map[string]any)
	if e.cfg.Secret != "" {
		bodyMap["secret"] = e.cfg.Secret
	}
	maps.Copy(bodyMap, data)
	body, _ := json.Marshal(bodyMap)

	resp, err := e.cli.Post(e.cfg.URL+path, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	// b, _ := io.ReadAll(resp.Body)
	// fmt.Println(string(b))
	return json.NewDecoder(resp.Body).Decode(out)
}

// post2 直接读取全部响应返回
func (e *Engine) post2(path string, data map[string]any) ([]byte, error) {
	bodyMap := map[string]any{
		"secret": e.cfg.Secret,
	}
	maps.Copy(bodyMap, data)
	body, _ := json.Marshal(bodyMap)

	resp, err := e.cli.Post(e.cfg.URL+path, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func (e *Engine) ErrHandle(code int, msg string) error {
	switch code {
	case Success:
		return nil
	case -1:
		return fmt.Errorf("zlm: %s", msg)
	case -100:
		return fmt.Errorf("zlm 鉴权失败: %s", msg)
	case -200:
		return fmt.Errorf("zlm(SQL): %s", msg)
	case -300:
		return fmt.Errorf("zlm(300): %s", msg)
	case -400:
		return fmt.Errorf("zlm(400): %s", msg)
	default:
		return fmt.Errorf("zlm 未知错误: %s", msg)
	}
}
