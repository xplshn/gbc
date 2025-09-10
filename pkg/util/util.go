package util

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/xplshn/gbc/pkg/config"
	"github.com/xplshn/gbc/pkg/token"
)

const (
	colorRed      = "\033[31m"
	colorYellow   = "\033[33m"
	colorReset    = "\033[0m"
	colorGray     = "\033[90m"
	colorBoldGray = "\033[1;90m"
	formatItalic  = "\033[3m"
)

type SourceFileRecord struct { Name string; Content []rune }

var sourceFiles []SourceFileRecord

func SetSourceFiles(files []SourceFileRecord) { sourceFiles = files }

func findFileAndLine(tok token.Token) (string, int, int) {
	if tok.FileIndex < 0 || tok.FileIndex >= len(sourceFiles) {
		return "<unknown>", tok.Line, tok.Column
	}
	return filepath.Base(sourceFiles[tok.FileIndex].Name), tok.Line, tok.Column
}

func callerFile(skip int) string {
	_, file, _, ok := runtime.Caller(skip)
	if !ok {
		return "<unknown>"
	}
	return filepath.Base(file)
}

func printSourceContext(stream *os.File, tok token.Token, isError bool, msg, caller string) {
	if tok.FileIndex < 0 || tok.FileIndex >= len(sourceFiles) || tok.Line <= 0 {
		return
	}

	content := sourceFiles[tok.FileIndex].Content
	lines := strings.Split(string(content), "\n")

	start := tok.Line - 2
	if start < 0 {
		start = 0
	}
	end := tok.Line + 1
	if end > len(lines) {
		end = len(lines)
	}
	lineNumWidth := len(fmt.Sprintf("%d", end))
	linePrefix := strings.Repeat(" ", 3)

	for i := start; i < end; i++ {
		lineNum := i + 1
		line := strings.ReplaceAll(lines[i], "\t", "    ")
		isErrorLine := lineNum == tok.Line

		var gutter string
		if isErrorLine {
			gutter = boldGray(fmt.Sprintf("%s%*d | ", linePrefix, lineNumWidth, lineNum))
		} else {
			gutter = gray(fmt.Sprintf("%s%*d | ", linePrefix, lineNumWidth, lineNum))
		}

		fmt.Fprintf(stream, " %s%s\n", gutter, line)

		if isErrorLine {
			colPos := caretColumn(line, tok.Column)
			caretLine := strings.Repeat(" ", colPos-1) + "^"
			if tok.Len > 1 {
				caretLine += strings.Repeat("~", tok.Len-1)
			}

			caretGutter := boldGray(strings.Repeat("-", lineNumWidth) + " | ")
			var caretColored, msgColored, callerColored string
			if isError {
				caretColored, msgColored = red(caretLine), italic(msg)
			} else {
				caretColored, msgColored = yellow(caretLine), italic(msg)
			}
			callerColored = italic(gray(fmt.Sprintf("(emitted from %s)", boldGray(caller))))
			fmt.Fprintf(stream, " %s%s%s %s %s%s\n", linePrefix, caretGutter, caretColored, msgColored, callerColored, colorReset)
		}
	}
	fmt.Fprintln(stream)
}

func caretColumn(line string, col int) int {
	if col < 1 {
		col = 1
	}
	runes := []rune(line)
	pos := 0
	for i := 0; i < col-1 && i < len(runes); i++ {
		if runes[i] == '\t' {
			pos += 4
		} else {
			pos++
		}
	}
	return pos + 1
}

func italic(s string) string   { return formatItalic + s + colorReset }
func gray(s string) string     { return colorGray + s + colorReset }
func boldGray(s string) string { return colorBoldGray + s + colorReset }
func red(s string) string      { return colorRed + s + colorReset }
func yellow(s string) string   { return colorYellow + s + colorReset }

func Error(tok token.Token, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	if tok.FileIndex < 0 || tok.FileIndex >= len(sourceFiles) || tok.Line <= 0 {
		fmt.Fprintf(os.Stderr, "gbc: %serror:%s %s\n", colorRed, colorReset, msg)
		os.Exit(1)
	}

	filename, line, col := findFileAndLine(tok)
	caller := callerFile(2)

	fmt.Fprintf(os.Stderr, "%s:%d:%d: %serror%s:\n", filename, line, col, colorRed, colorReset)
	printSourceContext(os.Stderr, tok, true, msg, caller)
	os.Exit(1)
}

func Warn(cfg *config.Config, wt config.Warning, tok token.Token, format string, args ...interface{}) {
	if !cfg.IsWarningEnabled(wt) {
		return
	}
	msg := fmt.Sprintf(format, args...) + fmt.Sprintf(" [-W%s]", cfg.Warnings[wt].Name)

	if tok.FileIndex < 0 || tok.FileIndex >= len(sourceFiles) || tok.Line <= 0 {
		fmt.Fprintf(os.Stderr, "gbc: %swarning:%s %s\n", colorYellow, colorReset, msg)
		return
	}

	filename, line, col := findFileAndLine(tok)
	caller := callerFile(2)

	fmt.Fprintf(os.Stderr, "%s:%d:%d: %swarning%s:\n", filename, line, col, colorYellow, colorReset)
	printSourceContext(os.Stderr, tok, false, msg, caller)
}

// AlignUp rounds n up to the next multiple of a
// a must be a power of 2
func AlignUp(n, a int64) int64 {
	if a == 0 {
		return n
	}
	return (n + a - 1) &^ (a - 1)
}
