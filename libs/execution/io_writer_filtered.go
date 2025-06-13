package execution

import (
	"io"
	"regexp"
)

// FilteringWriter wraps an io.Writer and filters sensitive content
type FilteringWriter struct {
	writer  io.Writer
	pattern *regexp.Regexp
}

func NewFilteringWriter(w io.Writer, pattern *regexp.Regexp) *FilteringWriter {
	return &FilteringWriter{
		writer:  w,
		pattern: pattern,
	}
}

func (fw *FilteringWriter) Write(p []byte) (n int, err error) {
	// Filter the content

	if fw.pattern == nil {
		return fw.writer.Write(p)
	}

	filtered := fw.pattern.ReplaceAll(p, []byte("<REDACTED>"))

	// Write filtered content to underlying writer
	_, err = fw.writer.Write(filtered)
	// Return original length to maintain compatibility
	return len(p), err
}
