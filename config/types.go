package config

import (
	"fmt"
	"strconv"
	"strings"
)

type MemorySize int64

func (m MemorySize) String() string {
	mf := float64(m)
	switch {
	case mf > 1024*1024*1024*1024:
		return formatT(mf/1024/1024/1024/1024, "T")
	case mf > 1024*1024*1024:
		return formatT(mf/1024/1024/1024, "G")
	case mf > 1024*1024:
		return formatT(mf/1024/1024, "M")
	case mf > 1024:
		return formatT(mf/1024, "K")
	}
	return formatT(mf, "B")
}

func formatT(t float64, unit string) string {
	s := fmt.Sprintf("%.2f", t)   // 保留两位小数
	s = strings.TrimRight(s, "0") // 去掉末尾的 0
	s = strings.TrimRight(s, ".") // 去掉末尾的 .（如果是整数的话）
	return s + unit
}

func (m *MemorySize) UnmarshalText(text []byte) error {
	var (
		// 乘数基数
		base int64 = 1
	)

	switch {
	case strings.HasSuffix(string(text), "B"), strings.HasSuffix(string(text), "b"):
		text = text[:len(text)-1]
	case strings.HasSuffix(string(text), "K"), strings.HasSuffix(string(text), "k"):
		text = text[:len(text)-1]
		base = 1024
	case strings.HasSuffix(string(text), "M"), strings.HasSuffix(string(text), "m"):
		text = text[:len(text)-1]
		base = 1024 * 1024
	case strings.HasSuffix(string(text), "G"), strings.HasSuffix(string(text), "g"):
		text = text[:len(text)-1]
		base = 1024 * 1024 * 1024
	case strings.HasSuffix(string(text), "T"), strings.HasSuffix(string(text), "t"):
		text = text[:len(text)-1]
		base = 1024 * 1024 * 1024 * 1024
	}

	n, err := strconv.ParseInt(string(text), 10, 64)
	if err != nil {
		return err
	}
	*m = MemorySize(n * base)
	return nil
}

func (m *MemorySize) MarshalText() ([]byte, error) {
	return []byte(m.String()), nil
}

func NewMemorySize(sizeStr string) MemorySize {
	var size MemorySize
	size.UnmarshalText([]byte(sizeStr))
	return size
}
