package config

import (
	"os"

	"github.com/ygpkg/yg-go/config"
	"github.com/ygpkg/yg-go/logs"
	"gopkg.in/yaml.v3"
)

// MainConfig global config
type MainConfig struct {
	Logger map[string][]config.LogConfig `yaml:"logger"`

	Apps []AppConfig `yaml:"apps"`

	// Global 全局配置
	Global GlobalConfig `yaml:"global"`
}

// GlobalConfig 全局配置
type GlobalConfig struct {
	// AllocatableResource 整体可分配资源
	AllocatableResource *Resource `yaml:"allocatable,omitempty"`
}

// AppConfig 应用配置
type AppConfig struct {
	Name          string        `yaml:"name"`
	Command       CommandConfig `yaml:"command"`
	RestartPolicy RestartPolicy `yaml:"restart"`
	ReplicaPolicy ReplicaPolicy `yaml:"replica"`
}

// CommandConfig 命令配置
type CommandConfig struct {
	WorkDir string   `yaml:"workdir"`
	Command string   `yaml:"command"`
	Args    []string `yaml:"args"`
	Envs    []Env    `yaml:"envs"`
}

// Env 环境变量
type Env struct {
	Key   string `yaml:"key"`
	Value string `yaml:"value"`
}

// ReplicaPolicy 副本策略
type ReplicaPolicy struct {
	// Static 静态副本数，0表示不限制
	Static *int `yaml:"static,omitempty"`
	// MaxReplicas 最大副本数，0表示不限制
	MaxReplicas *int `yaml:"max_replicas,omitempty"`
	// MinReplicas 最小副本数，0表示不限制
	MinReplicas *int `yaml:"min_replicas,omitempty"`
	// Require 需要的资源
	Require *Resource `yaml:"require,omitempty"`
}

type RestartPolicy struct {
	// MaxRetries 最大重试次数，-1表示无限制
	MaxRetries int `yaml:"max_retries"`
	// Interval 重试间隔，单位秒
	Interval int `yaml:"interval"`
}

// Resource 资源
type Resource struct {
	GPUMemory MemorySize `yaml:"gpu_memory"`
}

// AllocatableResource 可分配资源
type AllocatableResource struct {
	// Mode 资源获取模式，auto/manual
	Mode     string
	Resource `yaml:",inline"`
}

func LoadConfig(file string) (*MainConfig, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		logs.ErrorContextf(nil, "read config file %s failed, %s", file, err)
		return nil, err
	}
	cfg := &MainConfig{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		logs.ErrorContextf(nil, "unmarshal config file %s failed, %s", file, err)
		return nil, err
	}
	return cfg, nil
}
