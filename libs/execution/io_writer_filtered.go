package execution

import (
	"bytes"
	"io"
	"regexp"
)

// FilteringWriter wraps an io.Writer and filters sensitive content
type FilteringWriter struct {
	writer  io.Writer
	buffer  *bytes.Buffer
	pattern *regexp.Regexp
}

func NewFilteringWriter(w io.Writer, buf *bytes.Buffer, pattern *regexp.Regexp) *FilteringWriter {
	return &FilteringWriter{
		writer:  w,
		buffer:  buf,
		pattern: pattern,
	}
}

func (fw *FilteringWriter) Write(p []byte) (n int, err error) {
	// Filter the content

	var filtered []byte
	if fw.pattern == nil {
		filtered = p
	} else {
		filtered = fw.pattern.ReplaceAll(p, []byte("<REDACTED>"))
	}

	if fw.writer != nil {
		_, err = fw.writer.Write(filtered)
		if err != nil {
			return 0, err
		}
	}
	if fw.buffer != nil {
		_, err = fw.buffer.Write(filtered)
		if err != nil {
			return 0, err
		}
	}

	// Return original length to maintain compatibility
	return len(p), err
}
