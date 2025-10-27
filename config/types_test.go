package config

import (
	"strings"
	"testing"
)

func Test_SizeMarshal(t *testing.T) {
	table := map[string]MemorySize{
		"100B": 100,
		"100b": 100,
		"100K": 102400,
		"100k": 102400,
		"100M": 1024 * 1024 * 100,
		"100m": 1024 * 1024 * 100,
		"100G": 1024 * 1024 * 1024 * 100,
		"100g": 1024 * 1024 * 1024 * 100,
		"100T": 1024 * 1024 * 1024 * 1024 * 100,
		"100t": 1024 * 1024 * 1024 * 1024 * 100,
	}

	for k, v := range table {
		size := MemorySize(0)
		err := size.UnmarshalText([]byte(k))
		if err != nil {
			t.Fatalf("unmarshal %s failed, %s", k, err)
		}
		if size != v {
			t.Fatalf("marshal %s failed, %s", k, err)
		}

		text, err := size.MarshalText()
		if err != nil {
			t.Fatalf("marshal %s failed, %s", k, err)
		}
		if string(text) != strings.ToUpper(k) {
			t.Errorf("marshal %s not equal %s", text, k)
		}

	}
}
