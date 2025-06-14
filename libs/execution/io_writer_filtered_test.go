package execution

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"regexp"
	"testing"
)

func TestNewFilteringWriter(t *testing.T) {
	var buf bytes.Buffer = bytes.Buffer{}
	pattern := regexp.MustCompile("sensitive")
	writer := NewFilteringWriter(nil, &buf, pattern)

	if writer.buffer != &buf {
		t.Errorf("Expected buffer to be %v, got %v", &buf, writer.buffer)
	}
	if writer.pattern != pattern {
		t.Errorf("Expected pattern to be %v, got %v", pattern, writer.pattern)
	}

	writer.Write([]byte("sensitive"))
	assert.Equal(t, "<REDACTED>", buf.String())

	writer.Write([]byte("output"))
	assert.Equal(t, "<REDACTED>output", buf.String())

}
