package tfe

import (
	"context"
	"fmt"
	"strings"

	"github.com/diggerhq/digger/opentaco/internal/storage"
)

// logStreamOptions captures the parameters needed to stream chunked logs with STX/ETX framing.
type logStreamOptions struct {
	Prefix              string        // e.g., "plans" or "applies"
	Label               string        // e.g., "PLAN" or "APPLY" for logging
	ID                  string        // planID or apply/runID (part of the S3 key)
	Offset              int64         // client-requested offset (includes STX at 0)
	ChunkSize           int           // fixed chunk size in bytes (must match writer)
	GenerateDefaultText func() string // default text on first request when no chunks
	IsComplete          func() bool   // signals whether execution is finished/errored
	AppendETXOnFirst    bool          // if true, append ETX on the first request when complete
}

// streamChunkedLogs reads fixed-size padded chunks from blob storage and returns the bytes to send.
func streamChunkedLogs(ctx context.Context, blobStore storage.UnitStore, opts logStreamOptions) ([]byte, error) {
	chunkSize := opts.ChunkSize
	if chunkSize == 0 {
		chunkSize = 2 * 1024
	}

	startChunk := 1
	if opts.Offset > 1 { // offset includes STX byte at 0
		logOffset := opts.Offset - 1
		startChunk = int(logOffset / int64(chunkSize))
		startChunk++ // 1-indexed chunk numbers
	}
	bytesBefore := int64(chunkSize * (startChunk - 1))

	var fullLogs strings.Builder
	chunkIndex := startChunk
	for {
		chunkKey := fmt.Sprintf("%s/%s/chunks/%08d.log", opts.Prefix, opts.ID, chunkIndex)
		logData, err := blobStore.DownloadBlob(ctx, chunkKey)
		if err != nil {
			// Missing chunk; only continue if not complete (more chunks may appear later)
			if opts.IsComplete != nil && opts.IsComplete() {
				break
			}
			break
		}
		fullLogs.Write(logData)
		chunkIndex++
	}

	logText := fullLogs.String()
	// Trim padding after assembling the requested window
	logText = strings.TrimRight(logText, "\x00")

	if logText == "" && opts.Offset == 0 && opts.GenerateDefaultText != nil {
		logText = opts.GenerateDefaultText()
	}

	var responseData []byte

	if opts.Offset == 0 {
		// First request: send STX + current logs
		responseData = append([]byte{0x02}, []byte(logText)...)
		fmt.Printf("📤 %s LOGS at offset=0: STX + %d bytes of log text\n", opts.Label, len(logText))
		if len(logText) > 0 {
			fmt.Printf("Log preview (first 200 chars): %.200s\n", logText)
		}
		if opts.AppendETXOnFirst && opts.IsComplete != nil && opts.IsComplete() {
			responseData = append(responseData, 0x03)
			fmt.Printf("📤 Sending ETX for %s %s (complete at first request)\n", opts.Label, opts.ID)
		}
	} else {
		// Map stream offset to logText offset:
		// - stream offset 0 = STX
		// - stream offset 1 = first byte of full logs
		logOffset := opts.Offset - 1 - bytesBefore
		if logOffset < 0 {
			logOffset = 0
		}

		if logOffset < int64(len(logText)) {
			// Send remaining log text
			responseData = []byte(logText[logOffset:])
			fmt.Printf("📤 %s LOGS at offset=%d: sending %d bytes (logText[%d:])\n",
				opts.Label, opts.Offset, len(responseData), logOffset)
		} else if opts.IsComplete != nil && opts.IsComplete() {
			// All logs sent, send ETX to stop polling
			responseData = []byte{0x03}
			fmt.Printf("📤 Sending ETX (End of Text) for %s %s - logs complete\n", opts.Label, opts.ID)
		} else {
			// Waiting for more logs
			responseData = []byte{}
			fmt.Printf("📤 %s LOGS at offset=%d: no new data (waiting or complete)\n", opts.Label, opts.Offset)
		}
	}

	return responseData, nil
}
