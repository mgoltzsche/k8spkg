package kustomize

import (
	"io"
)

type RenderOptions struct {
	Source       string
	Out          io.Writer
	Unrestricted bool
}
