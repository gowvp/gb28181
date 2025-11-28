package conf

import (
	"time"

	"github.com/ixugo/goddd/pkg/orm"
)

func DefaultConfig() Bootstrap {
	return Bootstrap{
		Server: Server{
			Username:           "admin",
			Password:           "admin",
			RTMPSecret:         "123",
			PlayExpireMinutes:  10, // 默认 10 分钟过期
			EnableSnapshotBlur: false,
			HTTP: ServerHTTP{
				Port:      15123,
				Timeout:   Duration(60 * time.Second),
				JwtSecret: orm.GenerateRandomString(24),
				PProf: ServerPPROF{
					Enabled:   true,
					AccessIps: []string{"::1", "127.0.0.1"},
				},
			},
		},
		Data: Data{
			Database: Database{
				Dsn:             "./configs/data.db",
				MaxIdleConns:    10,
				MaxOpenConns:    50,
				ConnMaxLifetime: Duration(6 * time.Hour),
				SlowThreshold:   Duration(200 * time.Millisecond),
			},
		},
		Sip: SIP{
			Port:     15060,
			ID:       "34010000002000000001",
			Domain:   "3401000000",
			Password: "",
		},
		Media: Media{
			IP:           "127.0.0.1",
			HTTPPort:     80,
			Secret:       "",
			WebHookIP:    "127.0.0.1",
			SDPIP:        "127.0.0.1",
			RTPPortRange: "20000-20100",
		},
		GoLive: GoLive{
			Enabled:      false,
			RTMPPort:     1936,
			RTSPPort:     8555,
			HTTPFLVPort:  8088,
			HLSPort:      8088,
			PublicIP:     "",
			EnableAuth:   false,
			AuthSecret:   "",
			HLSFragment:  2,
			HLSWindow:    6,
			RecordPath:   "./records",
			EnableRecord: false,
		},
		AI: AI{
			Enabled:       false,
			InferenceMode: "remote",
			Endpoint:      "http://localhost:8080",
			APIKey:        "",
			Timeout:       30,
			ModelType:     "yolov8",
			ModelPath:     "./models/yolov8n.onnx",
			DeviceType:    "cpu",
		},
		Log: Log{
			Dir:          "./logs",
			Level:        "error",
			MaxAge:       Duration(3 * 24 * time.Hour),
			RotationTime: Duration(8 * time.Hour),
			RotationSize: 50,
		},
	}
}
