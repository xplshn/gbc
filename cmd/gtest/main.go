// indentation logic: vibecoded
// bad logic: written by me
// sucks: absolutely, but instead of bitching about it, open a PR. Thanks.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cespare/xxhash/v2"
	"github.com/google/go-cmp/cmp"
)

type Execution struct {
	Stdout         string        `json:"stdout"`
	Stderr         string        `json:"stderr"`
	ExitCode       int           `json:"exitCode"`
	Duration       time.Duration `json:"duration"`
	TimedOut       bool          `json:"timed_out"`
	UnstableOutput bool          `json:"unstable_output,omitempty"`
}

type TestRun struct {
	Name   string   `json:"name"`
	Args   []string `json:"args,omitempty"`
	Input  string   `json:"input,omitempty"`
	Result Execution `json:"result"`
}

type TargetResult struct {
	BinaryPath string    `json:"binary_path,omitempty"`
	Compile    Execution `json:"compile"`
	Runs       []TestRun `json:"runs"`
}

type FileTestResult struct {
	File      string        `json:"file"`
	Status    string        `json:"status"` // PASS, FAIL, SKIP, ERROR
	Message   string        `json:"message,omitempty"`
	Diff      string        `json:"diff,omitempty"`
	Reference *TargetResult `json:"reference,omitempty"`
	Target    *TargetResult `json:"target,omitempty"`
}

type TestSuiteResults map[string]*FileTestResult

var (
	refCompiler    = flag.String("ref-compiler", "b", "Path to the reference compiler.")
	refArgs        = flag.String("ref-args", "", "Arguments for the reference compiler (space-separated).")
	targetCompiler = flag.String("target-compiler", "./gbc", "Path to the target compiler to test.")
	targetArgs     = flag.String("target-args", "", "Arguments for the target compiler (space-separated).")
	generateGolden = flag.String("generate-golden", "", "Generate a golden .json file for a given source file.")
	testFiles      = flag.String("test-files", "tests/*.b", "Glob pattern(s) for files to test (space-separated).")
	skipFiles      = flag.String("skip-files", "", "Files to skip (space-separated).")
	outputJSON     = flag.String("output", ".test_results.json", "Output file for the JSON test report.")
	timeout        = flag.Duration("timeout", 5*time.Second, "Timeout for each command execution.")
	jobs           = flag.Int("j", 4, "Number of parallel test jobs.")
	runs           = flag.Int("runs", 5, "Number of times to run each test case to find the minimum duration.")
	verbose        = flag.Bool("v", false, "Enable verbose logging.")
	useCache       = flag.Bool("cached", false, "Use cached golden files if available.")
	jsonDir        = flag.String("dir", "", "Directory to store/read golden JSON files (defaults to source file dir).")
	ignoreLines    = flag.String("ignore-lines", "", "Comma-separated substrings to ignore during output comparison.")
)

const (
	cRed     = "\x1b[91m"
	cYellow  = "\x1b[93m"
	cGreen   = "\x1b[92m"
	cCyan    = "\x1b[96m"
	cMagenta = "\x1b[95m"
	cBold    = "\x1b[1m"
	cNone    = "\x1b[0m"
)

func main() {
	flag.Parse()
	log.SetFlags(0)

	if *runs < 1 {
		*runs = 1
	}

	// Single tempDir for all test artifacts
	tempDir, err := os.MkdirTemp("", "gtest-*")
	if err != nil {
		log.Fatalf("%s[ERROR]%s Failed to create temp directory: %v\n", cRed, cNone, err)
	}
	defer os.RemoveAll(tempDir)
	setupInterruptHandler(tempDir)

	if *generateGolden != "" {
		handleGenerateGolden(*generateGolden, tempDir)
		return
	}

	handleRunTestSuite(tempDir)
}

// setupInterruptHandler is used to clean up on CTRL+C
func setupInterruptHandler(tempDir string) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		os.RemoveAll(tempDir)
		fmt.Printf("\n%s[INTERRUPT]%s Test run cancelled. Cleaning up...\n", cYellow, cNone)
		os.Exit(1)
	}()
}

func getJSONPath(sourceFile string) string {
	jsonFileName := "." + filepath.Base(sourceFile) + ".json"
	if *jsonDir != "" {
		return filepath.Join(*jsonDir, jsonFileName)
	}
	return filepath.Join(filepath.Dir(sourceFile), jsonFileName)
}

// hashFile computes the xxhash of a file's content
func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := xxhash.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum64()), nil
}

func handleGenerateGolden(sourceFile, tempDir string) {
	log.Printf("Generating golden file for %s...\n", sourceFile)

	fileHash, err := hashFile(sourceFile)
	if err != nil {
		log.Fatalf("%s[ERROR]%s Could not hash source file %s: %v\n", cRed, cNone, sourceFile, err)
	}

	targetResult, err := compileAndRun(*targetCompiler, strings.Fields(*targetArgs), sourceFile, tempDir, fileHash)
	if err != nil {
		log.Fatalf("%s[ERROR]%s Could not generate golden file for %s: %v\n", cRed, cNone, sourceFile, err)
	}

	jsonData, err := json.MarshalIndent(targetResult, "", "  ")
	if err != nil {
		log.Fatalf("%s[ERROR]%s Failed to marshal golden data to JSON: %v\n", cRed, cNone, err)
	}

	goldenFileName := getJSONPath(sourceFile)
	if *jsonDir != "" {
		if err := os.MkdirAll(*jsonDir, 0755); err != nil {
			log.Fatalf("%s[ERROR]%s Failed to create directory %s: %v\n", cRed, cNone, *jsonDir, err)
		}
	}

	if err := os.WriteFile(goldenFileName, jsonData, 0644); err != nil {
		log.Fatalf("%s[ERROR]%s Failed to write golden file %s: %v\n", cRed, cNone, goldenFileName, err)
	}

	log.Printf("%s[SUCCESS]%s Golden file created at %s\n", cGreen, cNone, goldenFileName)
}

func handleRunTestSuite(tempDir string) {
	_, err := exec.LookPath(*refCompiler)
	refCompilerFound := err == nil
	if !refCompilerFound && !*useCache {
		log.Printf("%s[WARN]%s Reference compiler '%s' not found. Will rely on golden files. Use --cached to suppress this warning.\n", cYellow, cNone, *refCompiler)
	}

	files, err := expandGlobPatterns(*testFiles)
	if err != nil {
		log.Fatalf("%s[ERROR]%s Invalid glob pattern(s): %v\n", cRed, cNone, err)
	}
	if len(files) == 0 {
		log.Println("No test files found matching the pattern(s).")
		return
	}

	// Load previous results for caching reference compiler output
	previousResults := make(TestSuiteResults)
	outputFile := *outputJSON
	if *jsonDir != "" {
		outputFile = filepath.Join(*jsonDir, *outputJSON)
	}
	if prevData, err := os.ReadFile(outputFile); err == nil {
		if json.Unmarshal(prevData, &previousResults) != nil {
			log.Printf("%s[WARN]%s Could not parse previous results file %s. Cache will not be used.\n", cYellow, cNone, outputFile)
			previousResults = make(TestSuiteResults) // Reset on parse error
		}
	}

	skipList := make(map[string]bool)
	for _, f := range strings.Fields(*skipFiles) {
		skipList[f] = true
	}

	tasks := make(chan string, len(files))
	resultsChan := make(chan *FileTestResult, len(files))
	var wg sync.WaitGroup

	for i := 0; i < *jobs; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for file := range tasks {
				fileHash, err := hashFile(file)
				if err != nil {
					resultsChan <- &FileTestResult{File: file, Status: "ERROR", Message: "Failed to hash source file"}
					continue
				}
				resultsChan <- testFile(file, tempDir, fileHash, refCompilerFound, previousResults)
			}
		}()
	}

	// Feed the tasks channel, skipping files with identical content
	seenHashes := make(map[string]string)
	for _, file := range files {
		if skipList[file] {
			resultsChan <- &FileTestResult{File: file, Status: "SKIP", Message: "Explicitly skipped"}
			continue
		}
		fileHash, err := hashFile(file)
		if err != nil {
			resultsChan <- &FileTestResult{File: file, Status: "ERROR", Message: fmt.Sprintf("Failed to read file for hashing: %v", err)}
			continue
		}
		if originalFile, seen := seenHashes[fileHash]; seen {
			resultsChan <- &FileTestResult{File: file, Status: "SKIP", Message: fmt.Sprintf("Content is identical to %s", originalFile)}
			continue
		}
		seenHashes[fileHash] = file
		tasks <- file
	}
	close(tasks)

	wg.Wait()
	close(resultsChan)

	var allResults []*FileTestResult
	for result := range resultsChan {
		allResults = append(allResults, result)
	}

	sort.Slice(allResults, func(i, j int) bool {
		return allResults[i].File < allResults[j].File
	})

	printSummary(allResults)
	resultsMap := writeJSONReport(allResults)

	if hasFailures(resultsMap) {
		os.Exit(1)
	}
}

func testFile(file, tempDir, fileHash string, refCompilerFound bool, previousResults TestSuiteResults) *FileTestResult {
	goldenFile := getJSONPath(file)
	_, err := os.Stat(goldenFile)
	hasGoldenFile := err == nil

	// 1st try: Use golden file if --cached is set or for non-standard extensions
	if (*useCache && hasGoldenFile) || !strings.HasSuffix(file, ".b") {
		if hasGoldenFile {
			return testWithGoldenFile(file, goldenFile, tempDir, fileHash)
		}
		return &FileTestResult{File: file, Status: "SKIP", Message: "Cannot test without a corresponding .json golden file"}
	}

	// 2nd try: It's a .b file, try reference compiler if it exists
	if refCompilerFound {
		return testWithReferenceCompiler(file, tempDir, fileHash)
	}

	// 3rd try: No reference compiler, but a golden file exists. Use it as a fallback
	if hasGoldenFile {
		log.Printf("[%s] No reference compiler, falling back to golden file: %s", file, goldenFile)
		return testWithGoldenFile(file, goldenFile, tempDir, fileHash)
	}

	// 4th try: Use the existing .test_results.json if it exists
	if prevResult, ok := previousResults[file]; ok && prevResult.Reference != nil {
		log.Printf("[%s] Using cached reference result from previous test run.", file)
		targetResult, err := compileAndRun(*targetCompiler, strings.Fields(*targetArgs), file, tempDir, fileHash)
		if err != nil {
			return &FileTestResult{
				File:      file,
				Status:    "FAIL",
				Message:   "Target compiler failed to compile, but cached reference expected success.",
				Diff:      fmt.Sprintf("Target Compiler STDERR:\n%s", targetResult.Compile.Stderr),
				Reference: prevResult.Reference,
				Target:    targetResult,
			}
		}
		comparisonResult := compareRuntimeResults(file, prevResult.Reference, targetResult)
		comparisonResult.Message += " (against cached reference)"
		return comparisonResult
	}

	// No way to test this file
	return &FileTestResult{File: file, Status: "SKIP", Message: fmt.Sprintf("Reference compiler '%s' not found and no golden file or cached result exists", *refCompiler)}
}

func testWithGoldenFile(file, goldenFile, tempDir, fileHash string) *FileTestResult {
	goldenData, err := os.ReadFile(goldenFile)
	if err != nil {
		return &FileTestResult{File: file, Status: "ERROR", Message: fmt.Sprintf("Could not read golden file %s: %v", goldenFile, err)}
	}
	var goldenResult TargetResult
	if err := json.Unmarshal(goldenData, &goldenResult); err != nil {
		return &FileTestResult{File: file, Status: "ERROR", Message: fmt.Sprintf("Could not parse golden file %s: %v", goldenFile, err)}
	}

	targetResult, err := compileAndRun(*targetCompiler, strings.Fields(*targetArgs), file, tempDir, fileHash)
	if err != nil {
		return &FileTestResult{
			File:      file,
			Status:    "FAIL",
			Message:   "Target compiler failed to compile, but golden file expected success.",
			Diff:      fmt.Sprintf("Target Compiler STDERR:\n%s", targetResult.Compile.Stderr),
			Reference: &goldenResult,
			Target:    targetResult,
		}
	}

	return compareRuntimeResults(file, &goldenResult, targetResult)
}

func testWithReferenceCompiler(file, tempDir, fileHash string) *FileTestResult {
	var refResult, targetResult *TargetResult
	var refErr, targetErr error
	var wg sync.WaitGroup

	wg.Add(2)
	go func() {
		defer wg.Done()
		refResult, refErr = compileAndRun(*refCompiler, strings.Fields(*refArgs), file, tempDir, "ref-"+fileHash)
	}()
	go func() {
		defer wg.Done()
		targetResult, targetErr = compileAndRun(*targetCompiler, strings.Fields(*targetArgs), file, tempDir, "target-"+fileHash)
	}()
	wg.Wait()

	refCompiled := refErr == nil
	targetCompiled := targetErr == nil

	if !refCompiled && !targetCompiled {
		return &FileTestResult{File: file, Status: "PASS", Message: "Both compilers failed to compile as expected", Reference: refResult, Target: targetResult}
	}
	if !targetCompiled && refCompiled {
		return &FileTestResult{
			File:      file,
			Status:    "FAIL",
			Message:   "Target compiler failed, but reference compiler succeeded",
			Diff:      fmt.Sprintf("Target Compiler STDERR:\n%s", targetResult.Compile.Stderr),
			Reference: refResult,
			Target:    targetResult,
		}
	}
	if targetCompiled && !refCompiled {
		return &FileTestResult{
			File:      file,
			Status:    "FAIL",
			Message:   "Target compiler succeeded, but reference compiler failed",
			Diff:      fmt.Sprintf("Reference Compiler STDERR:\n%s", refResult.Compile.Stderr),
			Reference: refResult,
			Target:    targetResult,
		}
	}

	return compareRuntimeResults(file, refResult, targetResult)
}

func compareRuntimeResults(file string, refResult, targetResult *TargetResult) *FileTestResult {
	var diffs strings.Builder
	var failed bool

	targetRuns := make(map[string]TestRun)
	for _, run := range targetResult.Runs {
		targetRuns[run.Name] = run
	}

	sort.Slice(refResult.Runs, func(i, j int) bool {
		return refResult.Runs[i].Name < refResult.Runs[j].Name
	})

	ignoredSubstrings := []string{}
	if *ignoreLines != "" {
		ignoredSubstrings = strings.Split(*ignoreLines, ",")
	}

	for _, refRun := range refResult.Runs {
		targetRun, ok := targetRuns[refRun.Name]
		if !ok {
			failed = true
			diffs.WriteString(fmt.Sprintf("Test run '%s' missing in target results.\n", refRun.Name))
			continue
		}

		if refRun.Result.UnstableOutput != targetRun.Result.UnstableOutput {
			failed = true
			diffs.WriteString(fmt.Sprintf("Run '%s' Output Stability Mismatch:\n  - Ref:    %v\n  - Target: %v\n", refRun.Name, refRun.Result.UnstableOutput, targetRun.Result.UnstableOutput))
		}

		if refRun.Result.ExitCode != targetRun.Result.ExitCode {
			failed = true
			diffs.WriteString(fmt.Sprintf("Run '%s' Exit Code mismatch:\n  - Ref:    %d\n  - Target: %d\n", refRun.Name, refRun.Result.ExitCode, targetRun.Result.ExitCode))
		}

		// Filter output based on --ignore-lines first
		refStdout := filterOutput(refRun.Result.Stdout, ignoredSubstrings)
		targetStdout := filterOutput(targetRun.Result.Stdout, ignoredSubstrings)
		refStderr := filterOutput(refRun.Result.Stderr, ignoredSubstrings)
		targetStderr := filterOutput(targetRun.Result.Stderr, ignoredSubstrings)

		// Normalize by replacing binary paths (argv[0]) if they exist
		// This handles cases where a program prints its own name
		// We replace both the full path and the basename with a generic placeholder
		const binaryPlaceholder = "__BINARY__"
		if refResult.BinaryPath != "" {
			refStdout = strings.ReplaceAll(refStdout, refResult.BinaryPath, binaryPlaceholder)
			refStderr = strings.ReplaceAll(refStderr, refResult.BinaryPath, binaryPlaceholder)
			refBase := filepath.Base(refResult.BinaryPath)
			refStdout = strings.ReplaceAll(refStdout, refBase, binaryPlaceholder)
			refStderr = strings.ReplaceAll(refStderr, refBase, binaryPlaceholder)
		}
		if targetResult.BinaryPath != "" {
			targetStdout = strings.ReplaceAll(targetStdout, targetResult.BinaryPath, binaryPlaceholder)
			targetStderr = strings.ReplaceAll(targetStderr, targetResult.BinaryPath, binaryPlaceholder)
			targetBase := filepath.Base(targetResult.BinaryPath)
			targetStdout = strings.ReplaceAll(targetStdout, targetBase, binaryPlaceholder)
			targetStderr = strings.ReplaceAll(targetStderr, targetBase, binaryPlaceholder)
		}

		if refStdout != targetStdout {
			failed = true
			// Show the diff of the original, unmodified output for clarity
			diffs.WriteString(fmt.Sprintf("Run '%s' STDOUT mismatch:\n%s", refRun.Name, cmp.Diff(refRun.Result.Stdout, targetRun.Result.Stdout)))
		}

		if refStderr != targetStderr {
			failed = true
			// Show the diff of the original, unmodified output for clarity
			diffs.WriteString(fmt.Sprintf("Run '%s' STDERR mismatch:\n%s", refRun.Name, cmp.Diff(refRun.Result.Stderr, targetRun.Result.Stderr)))
		}
	}

	if failed {
		return &FileTestResult{
			File:      file,
			Status:    "FAIL",
			Message:   "Runtime output or exit code mismatch",
			Diff:      diffs.String(),
			Reference: refResult,
			Target:    targetResult,
		}
	}

	return &FileTestResult{
		File:      file,
		Status:    "PASS",
		Message:   "All test cases passed",
		Reference: refResult,
		Target:    targetResult,
	}
}

// executeCommand runs a command with a timeout and captures its output, optionally piping data to stdin
func executeCommand(ctx context.Context, command string, stdinData string, args ...string) Execution {
	startTime := time.Now()
	cmd := exec.CommandContext(ctx, command, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if stdinData != "" {
		cmd.Stdin = strings.NewReader(stdinData)
	}

	err := cmd.Run()
	duration := time.Since(startTime)

	execResult := Execution{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Duration: duration,
	}

	if ctx.Err() == context.DeadlineExceeded {
		execResult.TimedOut = true
		execResult.ExitCode = -1
	} else if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			execResult.ExitCode = exitErr.ExitCode()
		} else {
			execResult.ExitCode = -2 // Should not happen often
			execResult.Stderr += "\nExecution error: " + err.Error()
		}
	} else {
		execResult.ExitCode = 0
	}

	return execResult
}

func compileAndRun(compiler string, compilerArgs []string, sourceFile, tempDir, binaryHash string) (*TargetResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	// Use the hash for a unique, deterministic binary name
	binaryPath := filepath.Join(tempDir, binaryHash)

	allArgs := []string{"-o", binaryPath}
	allArgs = append(allArgs, compilerArgs...)
	allArgs = append(allArgs, sourceFile)

	compileResult := executeCommand(ctx, compiler, "", allArgs...)
	if compileResult.ExitCode != 0 || compileResult.TimedOut {
		return &TargetResult{Compile: compileResult}, fmt.Errorf("compilation failed with exit code %d", compileResult.ExitCode)
	}

	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		return &TargetResult{Compile: compileResult}, fmt.Errorf("compilation succeeded but binary was not created at %s", binaryPath)
	}

	// Probe to see if the binary waits for stdin by running it with a very short timeout
	// If it times out, it's likely waiting for input
	probeCtx, probeCancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer probeCancel()
	probeResult := executeCommand(probeCtx, binaryPath, "")
	readsStdin := probeResult.TimedOut

	testCases := map[string][]string{
		"no_args":         {},
		"quit":            {"q"},
		"hashTable":       {"s foo 10\ns bar 50\ng\ng foo\ng bar\np\nq\n"},
		"fold":            {"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyzAB\n"},
		"fold2":           {"ABCDEFGHIJKLMNOPQRSTUVWXYZABCDEFGHIJKLMNOPQRSTUVWX\n"},
		"string_arg":      {"test"},
		"numeric_arg_pos": {"5"},
		"numeric_arg_neg": {"-5"},
		"numeric_arg_0":   {"0"},
	}
	var testCaseNames []string
	for name := range testCases {
		testCaseNames = append(testCaseNames, name)
	}
	sort.Strings(testCaseNames)

	ignoredSubstrings := []string{}
	if *ignoreLines != "" {
		ignoredSubstrings = strings.Split(*ignoreLines, ",")
	}

	runResults := make([]TestRun, 0, len(testCaseNames))
	for _, name := range testCaseNames {
		args := testCases[name]
		var durations []time.Duration
		var firstRunResult Execution
		var inputData string
		var unstableOutput bool

		// If the program is detected to read from stdin, we join the args to form the input string
		// Otherwise, we pass them as command-line arguments
		if readsStdin {
			inputData = strings.Join(args, "\n")
			if len(args) > 0 {
				inputData += "\n"
			}
			args = []string{} // Clear args as they are now used for stdin
		}

		for i := 0; i < *runs; i++ {
			runCtx, runCancel := context.WithTimeout(context.Background(), *timeout)
			runResult := executeCommand(runCtx, binaryPath, inputData, args...)
			runCancel()

			if i == 0 {
				firstRunResult = runResult
			} else {
				// Compare with the first run's output, after filtering
				filteredFirstStdout := filterOutput(firstRunResult.Stdout, ignoredSubstrings)
				filteredCurrentStdout := filterOutput(runResult.Stdout, ignoredSubstrings)
				filteredFirstStderr := filterOutput(firstRunResult.Stderr, ignoredSubstrings)
				filteredCurrentStderr := filterOutput(runResult.Stderr, ignoredSubstrings)

				if firstRunResult.ExitCode != runResult.ExitCode || filteredFirstStdout != filteredCurrentStdout || filteredFirstStderr != filteredCurrentStderr {
					unstableOutput = true
					// Inconsistent output. We'll use the first run's result but mark it
					// We stop iterating because finding the "fastest" run is meaningless
					// if the output is different each time
					break
				}
			}

			if runResult.ExitCode != 0 || runResult.TimedOut {
				firstRunResult = runResult
				break
			}
			durations = append(durations, runResult.Duration)
		}

		if len(durations) > 0 {
			sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
			firstRunResult.Duration = durations[0]
		}
		firstRunResult.UnstableOutput = unstableOutput

		runResults = append(runResults, TestRun{Name: name, Args: args, Input: inputData, Result: firstRunResult})
		if firstRunResult.ExitCode != 0 || firstRunResult.TimedOut {
			break
		}
	}

	return &TargetResult{Compile: compileResult, Runs: runResults, BinaryPath: binaryPath}, nil
}

// filterOutput removes lines containing any of the given substrings
func filterOutput(output string, ignoredSubstrings []string) string {
	if len(ignoredSubstrings) == 0 || output == "" {
		return output
	}
	// To preserve original line endings, we can split by \n and rejoin
	lines := strings.Split(output, "\n")
	filteredLines := make([]string, 0, len(lines))

	for _, line := range lines {
		ignore := false
		for _, sub := range ignoredSubstrings {
			if sub != "" && strings.Contains(line, sub) {
				ignore = true
				break
			}
		}
		if !ignore {
			filteredLines = append(filteredLines, line)
		}
	}
	return strings.Join(filteredLines, "\n")
}

func formatDuration(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%6dµs", d.Microseconds())
	}
	return fmt.Sprintf("%6dms", d.Milliseconds())
}

func printSummary(results []*FileTestResult) {
	var passed, failed, skipped, errored int
	var totalTargetCompile, totalRefCompile, totalTargetRuntime, totalRefRuntime time.Duration
	var comparedFileCount, runtimeFileCount int

	// Pre-calculate max lengths for alignment
	var maxTestNameLen int
	targetName := filepath.Base(*targetCompiler)
	refName := filepath.Base(*refCompiler)

	// Calculate max label length for alignment of performance stats
	labels := []string{
		targetName,
		refName,
		targetName + "_comp",
		refName + "_comp",
		targetName + "_runt",
		refName + "_runt",
	}
	var maxLabelLen int
	for _, label := range labels {
		if len(label) > maxLabelLen {
			maxLabelLen = len(label)
		}
	}

	// Only calculate maxTestNameLen if in verbose mode, as it's only used there
	if *verbose {
		for _, result := range results {
			isBothFailed := result.Message == "Both compilers failed to compile as expected"
			if result.Status == "PASS" && result.Target != nil && result.Reference != nil && !isBothFailed {
				for _, run := range result.Target.Runs {
					if len(run.Name) > maxTestNameLen {
						maxTestNameLen = len(run.Name)
					}
				}
			}
		}
	}

	for _, result := range results {
		fmt.Println("----------------------------------------------------------------------")
		fmt.Printf("Testing %s%s%s...\n", cCyan, result.File, cNone)

		switch result.Status {
		case "PASS":
			passed++
			fmt.Printf("  [%sPASS%s] %s\n", cGreen, cNone, result.Message)
		case "FAIL":
			failed++
			fmt.Printf("  [%sFAIL%s] %s\n", cRed, cNone, result.Message)
			fmt.Println(formatDiff(result.Diff))
		case "SKIP":
			skipped++
			fmt.Printf("  [%sSKIP%s] %s\n", cYellow, cNone, result.Message)
		case "ERROR":
			errored++
			fmt.Printf("  [%sERROR%s] %s\n", cRed, cNone, result.Message)
		}

		isBothFailed := result.Message == "Both compilers failed to compile as expected"

		if (result.Status == "PASS" || result.Status == "FAIL") && result.Target != nil && result.Reference != nil {
			comparedFileCount++
			totalTargetCompile += result.Target.Compile.Duration
			totalRefCompile += result.Reference.Compile.Duration

			if !isBothFailed {
				runtimeFileCount++
				for _, run := range result.Target.Runs {
					totalTargetRuntime += run.Result.Duration
				}
				for _, run := range result.Reference.Runs {
					totalRefRuntime += run.Result.Duration
				}
			}
		}

		if (result.Status == "PASS") && result.Target != nil && result.Reference != nil {
			if *verbose && !isBothFailed {
				refRunsMap := make(map[string]TestRun, len(result.Reference.Runs))
				for _, run := range result.Reference.Runs {
					refRunsMap[run.Name] = run
				}
				sortedTargetRuns := result.Target.Runs
				sort.Slice(sortedTargetRuns, func(i, j int) bool {
					return sortedTargetRuns[i].Name < sortedTargetRuns[j].Name
				})

				for _, targetRun := range sortedTargetRuns {
					if refRun, ok := refRunsMap[targetRun.Name]; ok {
						targetColor, refColor := cNone, cNone
						if targetRun.Result.Duration < refRun.Result.Duration {
							targetColor = cMagenta
						} else if refRun.Result.Duration < targetRun.Result.Duration {
							refColor = cMagenta
						}
						leftPart := fmt.Sprintf("%-*s: %s%s%s", maxLabelLen, targetName, targetColor, formatDuration(targetRun.Result.Duration), cNone)
						rightPart := fmt.Sprintf("%-*s: %s%s%s", maxLabelLen, refName, refColor, formatDuration(refRun.Result.Duration), cNone)

						fmt.Printf("  [%sPASS%s] %-*s [%s | %s]\n",
							cGreen, cNone, maxTestNameLen, targetRun.Name,
							leftPart, rightPart)
					}
				}
			}

			var summaryPadding string
			if *verbose && maxTestNameLen > 0 {
				// Aligns the summary block with the performance block in verbose mode
				// Prefix is "  [PASS] " (8) + name (maxTestNameLen) + " " (1)
				summaryPadding = strings.Repeat(" ", 8+maxTestNameLen+1)
			} else {
				// In non-verbose mode, just indent slightly
				summaryPadding = "  "
			}

			refCompileColor, targetCompileColor := cNone, cNone
			if result.Reference.Compile.Duration < result.Target.Compile.Duration {
				refCompileColor = cMagenta
			} else if result.Target.Compile.Duration < result.Reference.Compile.Duration {
				targetCompileColor = cMagenta
			}
			leftCompilePart := fmt.Sprintf("%-*s: %s%s%s", maxLabelLen, targetName+"_comp", targetCompileColor, formatDuration(result.Target.Compile.Duration), cNone)
			rightCompilePart := fmt.Sprintf("%-*s: %s%s%s", maxLabelLen, refName+"_comp", refCompileColor, formatDuration(result.Reference.Compile.Duration), cNone)
			fmt.Printf("%s[%s | %s]\n", summaryPadding, leftCompilePart, rightCompilePart)

			if !isBothFailed {
				var totalTargetRun, totalRefRun time.Duration
				for _, run := range result.Target.Runs {
					totalTargetRun += run.Result.Duration
				}
				for _, run := range result.Reference.Runs {
					totalRefRun += run.Result.Duration
				}
				refRunColor, targetRunColor := cNone, cNone
				if totalRefRun < totalTargetRun {
					refRunColor = cMagenta
				} else if totalTargetRun < totalRefRun {
					targetRunColor = cMagenta
				}
				leftRuntPart := fmt.Sprintf("%-*s: %s%s%s", maxLabelLen, targetName+"_runt", targetRunColor, formatDuration(totalTargetRun), cNone)
				rightRuntPart := fmt.Sprintf("%-*s: %s%s%s", maxLabelLen, refName+"_runt", refRunColor, formatDuration(totalRefRun), cNone)
				fmt.Printf("%s[%s | %s]\n", summaryPadding, leftRuntPart, rightRuntPart)
			}

		} else if result.Status != "PASS" && result.Target != nil && result.Reference != nil {
			fmt.Printf("    %s compile: %s, %s compile: %s\n",
				refName, result.Reference.Compile.Duration,
				targetName, result.Target.Compile.Duration)
		}
	}

	fmt.Println("----------------------------------------------------------------------")
	fmt.Printf("%sTest Summary:%s %s%d Passed%s, %s%d Failed%s, %s%d Skipped%s, %s%d Errored%s, %d Total\n",
		cBold, cNone, cGreen, passed, cNone, cRed, failed, cNone, cYellow, skipped, cNone, cRed, errored, cNone, len(results))

	if comparedFileCount > 0 {
		targetName, refName := filepath.Base(*targetCompiler), filepath.Base(*refCompiler)
		avgTargetCompile := totalTargetCompile / time.Duration(comparedFileCount)
		avgRefCompile := totalRefCompile / time.Duration(comparedFileCount)

		fmt.Println("---")
		if avgTargetCompile > avgRefCompile {
			if avgRefCompile > 0 {
				factor := float64(avgTargetCompile) / float64(avgRefCompile)
				fmt.Printf("On average, %s%s%s was %s%.2fx%s slower to compile than %s.\n", cBold, targetName, cNone, cRed, factor, cNone, refName)
			}
		} else if avgRefCompile > avgTargetCompile {
			if avgTargetCompile > 0 {
				factor := float64(avgRefCompile) / float64(avgTargetCompile)
				fmt.Printf("On average, %s%s%s was %s%.2fx%s faster to compile than %s.\n", cBold, targetName, cNone, cGreen, factor, cNone, refName)
			}
		}

		if runtimeFileCount > 0 {
			avgTargetRuntime := totalTargetRuntime / time.Duration(runtimeFileCount)
			avgRefRuntime := totalRefRuntime / time.Duration(runtimeFileCount)
			if avgTargetRuntime > avgRefRuntime {
				if avgRefRuntime > 0 {
					factor := float64(avgTargetRuntime) / float64(avgRefRuntime)
					fmt.Printf("Binaries from %s%s%s ran %s%.2fx%s slower than those from %s.\n", cBold, targetName, cNone, cRed, factor, cNone, refName)
				}
			} else if avgRefRuntime > avgTargetRuntime {
				if avgTargetRuntime > 0 {
					factor := float64(avgRefRuntime) / float64(avgTargetRuntime)
					fmt.Printf("Binaries from %s%s%s ran %s%.2fx%s faster than those from %s.\n", cBold, targetName, cNone, cGreen, factor, cNone, refName)
				}
			}
		}
	}
}

func formatDiff(diff string) string {
	if diff == "" {
		return ""
	}
	var builder strings.Builder
	builder.WriteString("    --- Diff ---\n")
	for _, line := range strings.Split(diff, "\n") {
		lineWithIndent := "    " + line
		trimmedLine := strings.TrimSpace(line)
		if strings.HasPrefix(trimmedLine, "-") {
			builder.WriteString(cRed)
		} else if strings.HasPrefix(trimmedLine, "+") {
			builder.WriteString(cGreen)
		}
		builder.WriteString(lineWithIndent)
		builder.WriteString(cNone)
		builder.WriteString("\n")
	}
	return builder.String()
}

func writeJSONReport(results []*FileTestResult) TestSuiteResults {
	resultsMap := make(TestSuiteResults, len(results))
	for _, r := range results {
		resultsMap[r.File] = r
	}

	jsonData, err := json.MarshalIndent(resultsMap, "", "  ")
	if err != nil {
		log.Printf("%s[ERROR]%s Failed to marshal results to JSON: %v\n", cRed, cNone, err)
		return resultsMap
	}

	outputFile := *outputJSON
	if *jsonDir != "" {
		if err := os.MkdirAll(*jsonDir, 0755); err != nil {
			log.Printf("%s[ERROR]%s Failed to create dir %s: %v\n", cRed, cNone, *jsonDir, err)
		}
		outputFile = filepath.Join(*jsonDir, *outputJSON)
	}

	if err := os.WriteFile(outputFile, jsonData, 0644); err != nil {
		log.Printf("%s[ERROR]%s Failed to write JSON report to %s: %v\n", cRed, cNone, outputFile, err)
	} else {
		fmt.Printf("Full test report saved to %s\n", outputFile)
	}
	return resultsMap
}

func hasFailures(results TestSuiteResults) bool {
	for _, result := range results {
		if result.Status == "FAIL" || result.Status == "ERROR" {
			return true
		}
	}
	return false
}

func expandGlobPatterns(patterns string) ([]string, error) {
	var allFiles []string
	seen := make(map[string]bool)
	for _, pattern := range strings.Fields(patterns) {
		files, err := filepath.Glob(pattern)
		if err != nil {
			return nil, fmt.Errorf("bad pattern %s: %w", pattern, err)
		}
		for _, file := range files {
			absFile, err := filepath.Abs(file)
			if err != nil {
				continue // Skip files we can't resolve
			}
			if !seen[absFile] {
				if info, err := os.Stat(absFile); err == nil && info.Mode().IsRegular() {
					allFiles = append(allFiles, absFile)
					seen[absFile] = true
				}
			}
		}
	}
	return allFiles, nil
}
