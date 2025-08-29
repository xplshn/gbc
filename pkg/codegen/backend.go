package codegen

import (
	"bytes"
	"github.com/xplshn/gbc/pkg/config"
	"github.com/xplshn/gbc/pkg/ir"
)

// Backend is an interface for code generation backends
type Backend interface {
	Generate(prog *ir.Program, cfg *config.Config) (*bytes.Buffer, error)
}
