package config

import "go.uber.org/zap/zapcore"

// GlobalConfig global config
type GlobalConfig struct {
	LogConfig *LogConfig `yaml:"log"`
}

// LogConfig 。
type LogConfig struct {
	// Writer 日志输出位置 console/file/workwx
	Writer string `yaml:"writer"`
	// Encoder 编码格式
	Encoder string        `yaml:"encoder"`
	Level   zapcore.Level `yaml:"level"`
	Key     string        `yaml:"key,omitempty"`

	*lumberjack.Logger `yaml:",inline"`
}
