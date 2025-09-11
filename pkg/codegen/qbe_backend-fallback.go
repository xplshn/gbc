//go:build windows

package codegen

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/xplshn/gbc/pkg/config"
	"github.com/xplshn/gbc/pkg/ir"
)

func (b *qbeBackend) Generate(prog *ir.Program, cfg *config.Config) (*bytes.Buffer, error) {
	fmt.Println("Self-contained QBE backend is not supported on Windows. Fallbacking to system's 'qbe'.")
	_, err := exec.LookPath("qbe")
	if err != nil {
		return nil, fmt.Errorf("QBE not found in PATH: %s", err.Error())
	}

	qbeIR, err := b.GenerateIR(prog, cfg)
	if err != nil {
		return nil, err
	}

	input_file, err := os.CreateTemp("", "gbc-qbe-*.temp.ssa")
	if err != nil {
		return nil, err
	}
	defer input_file.Close()
	defer os.Remove(input_file.Name())

	if _, err = input_file.WriteString(qbeIR); err != nil {
		return nil, err
	}

	output_file_name := input_file.Name() + ".asm"
	cmd := exec.Command(
		"qbe",
		"-o", output_file_name,
		"-t", cfg.BackendTarget,
		input_file.Name(),
	)

	err = cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("\n--- QBE Compilation Failed ---\nGenerated IR:\n%s\n\nError: %w", qbeIR, err)
	}

	output_file, err := os.Open(output_file_name)
	if err != nil {
		return nil, err
	}
	defer output_file.Close()
	defer os.Remove(output_file_name)

	var asmBuf bytes.Buffer
	if _, err = io.Copy(&asmBuf, output_file); err != nil {
		return nil, err
	}

	return &asmBuf, nil
}
