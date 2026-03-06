package daemon

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrefixedLineWriter_WriteByLine(t *testing.T) {
	var out bytes.Buffer
	w := newPrefixedLineWriter(&out, "echo-app-1")

	n, err := w.Write([]byte("hello\nworld\n"))
	require.NoError(t, err)
	assert.Equal(t, len("hello\nworld\n"), n)
	assert.Equal(t, "echo-app-1 | hello\necho-app-1 | world\n", out.String())
}

func TestPrefixedLineWriter_WriteChunkedAndFlush(t *testing.T) {
	var out bytes.Buffer
	w := newPrefixedLineWriter(&out, "llm-qwen3-0")

	_, err := w.Write([]byte("par"))
	require.NoError(t, err)
	_, err = w.Write([]byte("tial"))
	require.NoError(t, err)
	_, err = w.Write([]byte(" line\nnext"))
	require.NoError(t, err)

	err = w.Flush()
	require.NoError(t, err)
	assert.Equal(t, "llm-qwen3-0 | partial line\nllm-qwen3-0 | next\n", out.String())
}
