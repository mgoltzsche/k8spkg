package k8spkg

import (
	"strings"
)

type chanWriter struct {
	ch           chan string
	lastLine     string
	writtenLines int
}

func newChanWriter() *chanWriter {
	return &chanWriter{make(chan string), "", 0}
}

func (w *chanWriter) Written() int {
	return w.writtenLines
}

func (w *chanWriter) Last() string {
	return w.lastLine
}

func (w *chanWriter) Chan() <-chan string {
	return w.ch
}

func (w *chanWriter) Write(b []byte) (int, error) {
	for _, line := range strings.Split(string(b), "\n") {
		if strings.TrimSpace(line) != "" {
			w.ch <- line
			w.lastLine = line
			w.writtenLines++
		}
	}
	return len(b), nil
}

func (w *chanWriter) Close() error {
	close(w.ch)
	return nil
}
