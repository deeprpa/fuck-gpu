package config

import (
	"os"

	"github.com/ygpkg/yg-go/config"
	"github.com/ygpkg/yg-go/logs"
	"gopkg.in/yaml.v2"
)

// GlobalConfig global config
type GlobalConfig struct {
	LogConfig config.LogConfig `yaml:"log"`

	TmpDir string       `yaml:"tmp_dir"`
	Apps   []*AppConfig `yaml:"apps"`
	// AllocatableResource 可分配资源
	AllocatableResource ResourceQuota `yaml:"allocatable"`
}

// AppConfig 应用配置
type AppConfig struct {
	Name    string        `yaml:"name"`
	Command CommandConfig `yaml:"command"`
	Restart string        `yaml:"restart"`
	Quota   ResourceQuota `yaml:"resources"`
}

// CommandConfig 命令配置
type CommandConfig struct {
	WorkDir string            `yaml:"workdir"`
	Command string            `yaml:"command"`
	Args    []string          `yaml:"args"`
	Envs    map[string]string `yaml:"envs"`
}

// ResourceQuota 资源配额
type ResourceQuota struct {
	// Require 资源需求
	Require Resource `yaml:"require"`
	// Limit   Resource `yaml:"limit"`
}

// ReplicaPolicy 副本策略
type ReplicaPolicy struct {
	// Static 静态副本数，0表示不限制
	Static int `yaml:"static"`
	// MaxReplicas 最大副本数，0表示不限制
	MaxReplicas int `yaml:"max_replicas"`
	// MinReplicas 最小副本数，0表示不限制
	MinReplicas int `yaml:"min_replicas"`
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

func LoadConfig(file string) (*GlobalConfig, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		logs.ErrorContextf(nil, "read config file %s failed, %s", file, err)
		return nil, err
	}
	cfg := &GlobalConfig{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		logs.ErrorContextf(nil, "unmarshal config file %s failed, %s", file, err)
		return nil, err
	}
	return cfg, nil
}
