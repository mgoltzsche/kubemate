package runner

import (
	"io"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestStreamLines(t *testing.T) {
	bufferCapacity = 5
	input := "aaaa\nbbbb\ncccccc\ndddd\nee\n"
	r, w := io.Pipe()
	go func() {
		w.Write([]byte(input))
		w.Write([]byte(input))
		w.Close()
	}()
	var lines []string
	log := func(line string) {
		lines = append(lines, line)
	}
	ch := make(chan struct{}, 1)
	go func() {
		streamLines(r, logrus.NewEntry(logrus.StandardLogger()), log)
		ch <- struct{}{}
	}()
	<-ch
	expected := []string{
		"aaaa",
		"bbbb",
		"ccccc",
		"c",
		"dddd",
		"ee",
	}
	expected = append(expected, expected...)
	require.Equal(t, expected, lines)
}
