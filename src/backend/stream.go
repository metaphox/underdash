package backend

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

const (
	// sseScanBufSize / sseScanBufMax size the bufio.Scanner used to read SSE
	// streams, whose event lines can far exceed the 64K scanner default.
	sseScanBufSize = 64 * 1024
	sseScanBufMax  = 1024 * 1024
)

// readSSEStream consumes a Server-Sent Events stream, invoking process with
// the payload of every non-empty data line except the [DONE] sentinel.
// process extracts text into sb and may return an error (e.g. an *APIError
// carried in an error event) to abort the stream. The concatenated text is
// returned; an empty result is an error.
func readSSEStream(r io.Reader, process func(data string, sb *strings.Builder) error) (string, error) {
	var sb strings.Builder
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, sseScanBufSize), sseScanBufMax)

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}
		if err := process(data, &sb); err != nil {
			return "", err
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("read stream: %w", err)
	}

	if sb.Len() == 0 {
		return "", fmt.Errorf("empty response from API")
	}
	return sb.String(), nil
}
