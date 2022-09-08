package runner

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/sirupsen/logrus"
)

var bufferCapacity = 65536

// streamLines calls the provided log method for every line it has read from the stream.
// When the line buffer capacity is exceeded, the incomplete line buffer is logged as a line.
func streamLines(r io.ReadCloser, logger *logrus.Entry, log func(line string)) {
	defer r.Close()
	for {
		buf := make([]byte, bufferCapacity)
		scanner := bufio.NewScanner(r)
		scanner.Buffer(buf, bufferCapacity)
		for scanner.Scan() {
			if line := scanner.Text(); len(line) > 0 {
				log(line)
			}
		}
		err := scanner.Err()
		if err == nil {
			return
		} else if err != bufio.ErrTooLong {
			logger.Error(fmt.Errorf("read process output: %w", err))
			return
		}
		// fallback to printing the current buffer contents as line when no complete line can be read
		part := strings.TrimSpace(string(buf[:bufferCapacity]))
		if len(part) > 0 {
			log(part)
		}
	}
}
