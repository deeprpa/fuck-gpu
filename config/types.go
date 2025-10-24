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
	case mf < 1024:
		return fmt.Sprintf("%dB", m)
	case mf < 1024*1024:
		return fmt.Sprintf("%.2fK", mf/1024)
	case mf < 1024*1024*1024:
		return fmt.Sprintf("%.2fM", mf/1024/1024)
	case mf < 1024*1024*1024*1024:
		return fmt.Sprintf("%.2fG", mf/1024/1024/1024)
	}
	return fmt.Sprintf("%.2fT", mf/1024/1024/1024/1024)
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
