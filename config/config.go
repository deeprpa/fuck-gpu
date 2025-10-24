package config

import (
	"io/ioutil"

	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
	"gopkg.in/yaml.v2"
)

// GlobalConfig global config
type GlobalConfig struct {
	LogConfig *LogConfig `yaml:"log"`

	TmpDir string       `yaml:"tmp_dir"`
	Apps   []*AppConfig `yaml:"apps"`
}

// LogConfig 。
type LogConfig struct {
	// Writer 日志输出位置 console/file/workwx
	Writer string `yaml:"writer"`
	// Encoder 编码格式
	Encoder string        `yaml:"encoder"`
	Level   zapcore.Level `yaml:"level"`

	*lumberjack.Logger `yaml:",inline"`
}

type AppConfig struct {
	Name    string        `yaml:"name"`
	Command CommandConfig `yaml:"command"`
	Restart string        `yaml:"restart"`
}

type CommandConfig struct {
	Command string                 `yaml:"command"`
	WorkDir string                 `yaml:"workdir"`
	Args    []string               `yaml:"args"`
	VerArgs []string               `yaml:"ver_args"`
	Envs    map[string]interface{} `yaml:"envs"`
}

type ResourceQuota struct {
	Require Resource `yaml:"require"`
	// Limit   Resource `yaml:"limit"`
}

// Resource 资源
type Resource struct {
	GPUMemory MemorySize `yaml:"gpu_memory"`
}

func LoadConfig(file string) (*GlobalConfig, error) {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}
	cfg := &GlobalConfig{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
