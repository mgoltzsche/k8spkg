package k8spkg

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
)

func EventChannel(ctx context.Context, kubeconfigFile string) (ch chan *Event, err chan error) {
	ch = make(chan *Event)
	err = make(chan error)
	args := []string{"get", "event", "--watch", "-o", "json", "--all-namespaces", "--sort-by=.metadata.lastTimestamp"}
	pReader, pWriter := io.Pipe()
	go func() {
		var stderr bytes.Buffer
		cmd := newKubectlCmd(ctx, kubeconfigFile)
		cmd.Stdout = pWriter
		cmd.Stderr = &stderr
		e := cmd.Run(args...)
		pWriter.CloseWithError(e)
	}()
	go func() {
		e := readEvents(pReader, ch)
		select {
		case <-ctx.Done():
			e = ctx.Err()
		default:
		}
		if e1 := pReader.CloseWithError(e); e1 != nil && e == nil {
			e = e1
		}
		err <- e
	}()
	return
}

func readEvents(reader io.Reader, ch chan *Event) (err error) {
	dec := json.NewDecoder(reader)
	evt := &Event{}
	for ; err == nil; err = dec.Decode(evt) {
		if err == nil {
			ch <- evt
			evt = &Event{}
		}
	}
	if err == io.EOF {
		err = nil
	}
	return
}
