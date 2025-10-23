package gpucollect

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// GPUInfo 保存单张显卡的信息
type GPUInfo struct {
	Index         int
	Name          string
	MemoryTotalMB int
	MemoryFreeMB  int
	MemoryUsedMB  int
}

// GetNvidiaGPUMemory 获取当前系统中所有 NVIDIA GPU 的显存信息
func GetNvidiaGPUMemory() ([]GPUInfo, error) {
	// 使用 nvidia-smi 查询 GPU 显存信息
	cmd := exec.Command("nvidia-smi",
		"--query-gpu=index,name,memory.total,memory.free,memory.used",
		"--format=csv,noheader,nounits")

	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("执行 nvidia-smi 失败: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	var gpus []GPUInfo

	for _, line := range lines {
		fields := strings.Split(strings.TrimSpace(line), ", ")
		if len(fields) != 5 {
			continue
		}

		index, _ := strconv.Atoi(fields[0])
		name := fields[1]
		total, _ := strconv.Atoi(fields[2])
		free, _ := strconv.Atoi(fields[3])
		used, _ := strconv.Atoi(fields[4])

		gpus = append(gpus, GPUInfo{
			Index:         index,
			Name:          name,
			MemoryTotalMB: total,
			MemoryFreeMB:  free,
			MemoryUsedMB:  used,
		})
	}

	return gpus, nil
}
