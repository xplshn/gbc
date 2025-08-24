package codegen

import (
	"bytes"
	"github.com/xplshn/gbc/pkg/config"
	"github.com/xplshn/gbc/pkg/ir"
)

// Backend is the interface that all code generation backends must implement.
type Backend interface {
	// Generate takes an IR program and a configuration, and produces the target
	// assembly or intermediate language as a byte buffer.
	Generate(prog *ir.Program, cfg *config.Config) (*bytes.Buffer, error)
}
