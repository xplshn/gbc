//go:build !windows

package codegen

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/xplshn/gbc/pkg/config"
	"github.com/xplshn/gbc/pkg/ir"
	"modernc.org/libqbe"
)

func (b *qbeBackend) Generate(prog *ir.Program, cfg *config.Config) (*bytes.Buffer, error) {
	qbeIR, err := b.GenerateIR(prog, cfg)
	if err != nil {
		return nil, err
	}

	var asmBuf bytes.Buffer
	err = libqbe.Main(cfg.BackendTarget, "input.ssa", strings.NewReader(qbeIR), &asmBuf, nil)
	if err != nil {
		return nil, fmt.Errorf("\n--- QBE Compilation Failed ---\nGenerated IR:\n%s\n\nlibqbe error: %w", qbeIR, err)
	}
	return &asmBuf, nil
}
