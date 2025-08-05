package parser

import (
	"fmt"
	"os"
	"strings"
)

type FeatureType int

const (
	FEAT_EXTRN FeatureType = iota
	FEAT_ASM
	FEAT_B_ESCAPES
	FEAT_C_ESCAPES
	FEAT_B_OPS
	FEAT_C_OPS
	FEAT_C_COMMENTS
	FEAT_COUNT // Just to get the number of features
)

type FeatureInfo struct {
	Name        string
	Enabled     bool
	Description string
}

var Features = map[FeatureType]FeatureInfo{
	FEAT_EXTRN:      {"extrn", true, "Allow the 'extrn' keyword"},
	FEAT_ASM:        {"asm", true, "Allow the '__asm__' keyword and blocks"},
	FEAT_B_ESCAPES:  {"b-escapes", true, "Recognize B-style '*' character escapes"},
	FEAT_C_ESCAPES:  {"c-escapes", true, "Recognize C-style '\\' character escapes"},
	FEAT_B_OPS:      {"b-ops", true, "Recognize B-style assignment operators like '=+'"},
	FEAT_C_OPS:      {"c-ops", true, "Recognize C-style assignment operators like '+='"},
	FEAT_C_COMMENTS: {"c-comments", true, "Recognize C-style '//' comments"},
}

var FeatureMap = make(map[string]FeatureType)

type WarningType int

const (
	WARN_C_ESCAPES WarningType = iota
	WARN_B_ESCAPES
	WARN_B_OPS
	WARN_C_OPS
	WARN_UNRECOGNIZED_ESCAPE
	WARN_TRUNCATED_CHAR
	WARN_LONG_CHAR_CONST
	WARN_C_COMMENTS
	WARN_OVERFLOW
	WARN_PEDANTIC
	WARN_UNREACHABLE_CODE
	WARN_EXTRA
	WARN_COUNT // Just to get the number of warnings
)

type WarningInfo struct {
	Name        string
	Enabled     bool
	Description string
}

var Warnings = map[WarningType]WarningInfo{
	WARN_C_ESCAPES:           {"c-escapes", true, "Using C-style '\\' escapes instead of B's '*'"},
	WARN_B_ESCAPES:           {"b-escapes", true, "Using historical B-style '*' escapes instead of C's '\\'"},
	WARN_B_OPS:               {"b-ops", true, "Using historical B assignment operators like '=+'"},
	WARN_C_OPS:               {"c-ops", true, "Using C-style assignment operators like '+=' in -std=B mode"},
	WARN_UNRECOGNIZED_ESCAPE: {"unrecognized-escape", true, "Using unrecognized escape sequences"},
	WARN_TRUNCATED_CHAR:      {"truncated-char", true, "Character escape value is too large for a byte and has been truncated"},
	WARN_LONG_CHAR_CONST:     {"long-char-const", true, "Multi-character constant is too long for a word"},
	WARN_C_COMMENTS:          {"c-comments", false, "Using non-standard C-style '//' comments"},
	WARN_OVERFLOW:            {"overflow", true, "Integer constant is out of range for its type"},
	WARN_PEDANTIC:            {"pedantic", false, "Issues that violate the current strict -std="},
	WARN_UNREACHABLE_CODE:    {"unreachable-code", true, "Unreachable code"},
	WARN_EXTRA:               {"extra", true, "Extra warnings (e.g., poor choices, unrecognized flags)"},
}

var WarningMap = make(map[string]WarningType)

// init function to populate the lookup maps
func init() {
	for ft, info := range Features {
		FeatureMap[info.Name] = ft
	}
	for wt, info := range Warnings {
		WarningMap[info.Name] = wt
	}
}

// SetFeature enables or disables a specific feature
func SetFeature(ft FeatureType, enabled bool) {
	if info, ok := Features[ft]; ok {
		info.Enabled = enabled
		Features[ft] = info
	}
}

// IsFeatureEnabled checks if a specific feature is currently enabled
func IsFeatureEnabled(ft FeatureType) bool {
	if info, ok := Features[ft]; ok {
		return info.Enabled
	}
	return false
}

// PrintFeatures prints the current status of all features
func PrintFeatures() {
	for i := FeatureType(0); i < FEAT_COUNT; i++ {
		info := Features[i]
		fmt.Printf("  - %-20s: %v (%s)\n", info.Name, info.Enabled, info.Description)
	}
}

// setAllWarnings enables or disables all warnings at once
func SetAllWarnings(enabled bool) {
	for i := WarningType(0); i < WARN_COUNT; i++ {
		// -Wall should not enable pedantic warnings by default.
		if i == WARN_PEDANTIC && enabled {
			continue
		}
		SetWarning(i, enabled)
	}
}

// SetWarning enables or disables a specific warning
func SetWarning(wt WarningType, enabled bool) {
	if info, ok := Warnings[wt]; ok {
		info.Enabled = enabled
		Warnings[wt] = info
	}
}

// IsWarningEnabled checks if a specific warning is currently enabled.
func IsWarningEnabled(wt WarningType) bool {
	if info, ok := Warnings[wt]; ok {
		return info.Enabled
	}
	return false
}

// PrintWarnings prints the current status of all warnings.
func PrintWarnings() {
	for i := WarningType(0); i < WARN_COUNT; i++ {
		info := Warnings[i]
		fmt.Printf("  - %-20s: %v (%s)\n", info.Name, info.Enabled, info.Description)
	}
}

// SourceFileRecord tracks the name and content of a single source file.
type SourceFileRecord struct {
	Name    string
	Content []rune
}

var sourceFiles []SourceFileRecord

// SetSourceFiles stores the source code for all input files for rich error messages
func SetSourceFiles(files []SourceFileRecord) {
	sourceFiles = files
}

// findFileAndLine converts a global token to a file-specific location
func findFileAndLine(tok Token) (filename string, line, col int) {
	if tok.FileIndex < 0 || tok.FileIndex >= len(sourceFiles) {
		return "unknown", tok.Line, tok.Column
	}
	return sourceFiles[tok.FileIndex].Name, tok.Line, tok.Column
}

// printErrorLine prints the source line and a caret indicating the error position
func printErrorLine(stream *os.File, tok Token) {
	if tok.FileIndex < 0 || tok.FileIndex >= len(sourceFiles) || tok.Line == 0 {
		return
	}

	content := sourceFiles[tok.FileIndex].Content
	lineNum := tok.Line
	lineStart := 0
	// Find the start of the error line
	for i, r := range content {
		if lineNum <= 1 {
			break
		}
		if r == '\n' {
			lineNum--
			lineStart = i + 1
		}
	}

	// Find the end of the error line
	lineEnd := len(content)
	for i := lineStart; i < len(content); i++ {
		if content[i] == '\n' {
			lineEnd = i
			break
		}
	}

	// Print the line
	fmt.Fprintf(stream, "  %s\n", string(content[lineStart:lineEnd]))

	// Print the caret
	fmt.Fprintf(stream, "  %s\033[32m^", strings.Repeat(" ", tok.Column-1))
	if tok.Len > 1 {
		fmt.Fprintf(stream, "%s", strings.Repeat("~", tok.Len-1))
	}
	fmt.Fprintln(stream, "\033[0m")
}

// Error prints a formatted error message and exits the program
func Error(tok Token, format string, args ...interface{}) {
	filename, line, col := findFileAndLine(tok)
	fmt.Fprintf(os.Stderr, "%s:%d:%d: \033[31merror:\033[0m ", filename, line, col)
	fmt.Fprintf(os.Stderr, format, args...)
	fmt.Fprintln(os.Stderr)
	printErrorLine(os.Stderr, tok)
	os.Exit(1)
}

// Warning prints a formatted warning message if the corresponding warning is enabled
func Warning(wt WarningType, tok Token, format string, args ...interface{}) {
	if !IsWarningEnabled(wt) {
		return
	}
	filename, line, col := findFileAndLine(tok)
	warningName := Warnings[wt].Name
	fmt.Fprintf(os.Stderr, "%s:%d:%d: \033[33mwarning:\033[0m ", filename, line, col)
	fmt.Fprintf(os.Stderr, format, args...)
	fmt.Fprintf(os.Stderr, " [-W%s]\n", warningName)
	printErrorLine(os.Stderr, tok)
}
