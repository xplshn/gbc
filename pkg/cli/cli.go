package cli

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/term"
)

type IndentState struct { levels []uint8; baseUnit uint8 }

func NewIndentState() *IndentState {
	return &IndentState{
		levels:   []uint8{0},
		baseUnit: 4,
	}
}

func (is *IndentState) Push() {
	currentLevel := is.levels[len(is.levels)-1]
	is.levels = append(is.levels, currentLevel+1)
}

func (is *IndentState) Pop() {
	if len(is.levels) > 1 {
		is.levels = is.levels[:len(is.levels)-1]
	}
}

func (is *IndentState) Current() string {
	level := is.levels[len(is.levels)-1]
	return strings.Repeat(" ", int(is.baseUnit*level))
}

func (is *IndentState) AtLevel(level int) string {
	return strings.Repeat(" ", int(is.baseUnit*uint8(level)))
}

type Value interface {
	String() string
	Set(string) error
	Get() any
}

type stringValue struct{ p *string }

func (v *stringValue) Set(s string) error   { *v.p = s; return nil }
func (v *stringValue) String() string       { return *v.p }
func (v *stringValue) Get() any             { return *v.p }
func newStringValue(p *string) *stringValue { return &stringValue{p} }

type boolValue struct{ p *bool }

func (v *boolValue) Set(s string) error {
	val, err := strconv.ParseBool(s)
	if err != nil && s != "" {
		return fmt.Errorf("invalid boolean value '%s': %w", s, err)
	}
	*v.p = val || s == ""
	return nil
}
func (v *boolValue) String() string { return strconv.FormatBool(*v.p) }
func (v *boolValue) Get() any       { return *v.p }
func newBoolValue(p *bool) *boolValue {
	return &boolValue{p}
}

type listValue struct{ p *[]string }

func (v *listValue) Set(s string) error   { *v.p = append(*v.p, s); return nil }
func (v *listValue) String() string       { return strings.Join(*v.p, ", ") }
func (v *listValue) Get() any             { return *v.p }
func newListValue(p *[]string) *listValue { return &listValue{p} }

type Flag struct {
	Name         string
	Shorthand    string
	Usage        string
	Value        Value
	DefValue     string
	ExpectedType string
}

type FlagGroup struct {
	Name                 string
	Description          string
	Flags                []FlagGroupEntry
	GroupType            string
	AvailableFlagsHeader string
}

type FlagGroupEntry struct {
	Name     string
	Prefix   string
	Usage    string
	Enabled  *bool
	Disabled *bool
}

type FlagSet struct {
	name          string
	flags         map[string]*Flag
	shorthands    map[string]*Flag
	specialPrefix map[string]*Flag
	args          []string
	flagGroups    []FlagGroup
}

func NewFlagSet(name string) *FlagSet {
	return &FlagSet{
		name:          name,
		flags:         make(map[string]*Flag),
		shorthands:    make(map[string]*Flag),
		specialPrefix: make(map[string]*Flag),
	}
}

func (f *FlagSet) Args() []string { return f.args }

func (f *FlagSet) String(p *string, name, shorthand, value, usage, expectedType string) {
	*p = value
	f.Var(newStringValue(p), name, shorthand, usage, value, expectedType)
}

func (f *FlagSet) Bool(p *bool, name, shorthand string, value bool, usage string) {
	*p = value
	f.Var(newBoolValue(p), name, shorthand, usage, strconv.FormatBool(value), "")
}

func (f *FlagSet) List(p *[]string, name, shorthand string, value []string, usage, expectedType string) {
	*p = value
	f.Var(newListValue(p), name, shorthand, usage, fmt.Sprintf("%v", value), expectedType)
}

func (f *FlagSet) Special(p *[]string, prefix, usage, expectedType string) {
	*p = []string{}
	f.Var(newListValue(p), prefix, "", usage, "", expectedType)
	f.specialPrefix[prefix] = f.flags[prefix]
}

func (f *FlagSet) DefineGroupFlags(entries []FlagGroupEntry) {
	for i := range entries {
		if entries[i].Enabled != nil {
			f.Bool(entries[i].Enabled, entries[i].Prefix+entries[i].Name, "", *entries[i].Enabled, entries[i].Usage)
		}
		if entries[i].Disabled != nil {
			disableUsage := "Disable '" + entries[i].Name + "'"
			f.Bool(entries[i].Disabled, entries[i].Prefix+"no-"+entries[i].Name, "", *entries[i].Disabled, disableUsage)
		}
	}
}

func (f *FlagSet) AddFlagGroup(name, description, groupType, availableFlagsHeader string, entries []FlagGroupEntry) {
	f.DefineGroupFlags(entries)
	f.flagGroups = append(f.flagGroups, FlagGroup{
		Name:                 name,
		Description:          description,
		Flags:                entries,
		GroupType:            groupType,
		AvailableFlagsHeader: availableFlagsHeader,
	})
}

func (f *FlagSet) Var(value Value, name, shorthand, usage, defValue, expectedType string) {
	if name == "" {
		panic("flag name cannot be empty")
	}
	flag := &Flag{Name: name, Shorthand: shorthand, Usage: usage, Value: value, DefValue: defValue, ExpectedType: expectedType}
	if _, ok := f.flags[name]; ok {
		panic(fmt.Sprintf("flag redefined: %s", name))
	}
	f.flags[name] = flag
	if shorthand != "" {
		if _, ok := f.shorthands[shorthand]; ok {
			panic(fmt.Sprintf("shorthand flag redefined: %s", shorthand))
		}
		f.shorthands[shorthand] = flag
	}
}

func (f *FlagSet) Parse(arguments []string) error {
	f.args = []string{}
	for i := 0; i < len(arguments); i++ {
		arg := arguments[i]
		if len(arg) < 2 || arg[0] != '-' {
			f.args = append(f.args, arg)
			continue
		}
		if arg == "--" {
			f.args = append(f.args, arguments[i+1:]...)
			break
		}
		if strings.HasPrefix(arg, "--") {
			if err := f.parseLongFlag(arg, arguments, &i); err != nil {
				return err
			}
		} else {
			name := arg[1:]
			if strings.Contains(name, "=") {
				name = strings.SplitN(name, "=", 2)[0]
			}

			flag, ok := f.flags[name]
			if ok {
				parts := strings.SplitN(arg[1:], "=", 2)
				if len(parts) == 2 {
					if err := flag.Value.Set(parts[1]); err != nil {
						return err
					}
				} else {
					if _, isBool := flag.Value.(*boolValue); isBool {
						if err := flag.Value.Set(""); err != nil {
							return err
						}
					} else {
						if i+1 >= len(arguments) {
							return fmt.Errorf("flag needs an argument: -%s", name)
						}
						i++
						if err := flag.Value.Set(arguments[i]); err != nil {
							return err
						}
					}
				}
			} else {
				if err := f.parseShortFlag(arg, arguments, &i); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (f *FlagSet) parseLongFlag(arg string, arguments []string, i *int) error {
	parts := strings.SplitN(arg[2:], "=", 2)
	name := parts[0]
	if name == "" {
		return fmt.Errorf("empty flag name")
	}
	flag, ok := f.flags[name]
	if !ok {
		return fmt.Errorf("unknown flag: --%s", name)
	}
	if len(parts) == 2 {
		return flag.Value.Set(parts[1])
	}
	if _, isBool := flag.Value.(*boolValue); isBool {
		return flag.Value.Set("")
	}
	if *i+1 >= len(arguments) {
		return fmt.Errorf("flag needs an argument: --%s", name)
	}
	*i++
	return flag.Value.Set(arguments[*i])
}

func (f *FlagSet) parseShortFlag(arg string, arguments []string, i *int) error {
	for prefix, flag := range f.specialPrefix {
		if strings.HasPrefix(arg, "-"+prefix) && len(arg) > len(prefix)+1 {
			return flag.Value.Set(arg[len(prefix)+1:])
		}
	}

	shorthand := arg[1:2]
	flag, ok := f.shorthands[shorthand]
	if !ok {
		return fmt.Errorf("unknown shorthand flag: -%s", shorthand)
	}
	if _, isBool := flag.Value.(*boolValue); isBool {
		return flag.Value.Set("")
	}
	value := arg[2:]
	if value == "" {
		if *i+1 >= len(arguments) {
			return fmt.Errorf("flag needs an argument: -%s", shorthand)
		}
		*i++
		value = arguments[*i]
	}
	return flag.Value.Set(value)
}

type App struct {
	Name        string
	Synopsis    string
	Description string
	Authors     []string
	Repository  string
	Since       int
	FlagSet     *FlagSet
	Action      func(args []string) error
}

func NewApp(name string) *App {
	return &App{
		Name:    name,
		FlagSet: NewFlagSet(name),
	}
}

func (f *FlagSet) Lookup(name string) *Flag {
	return f.flags[name]
}

func (a *App) Run(arguments []string) error {
	help := false
	a.FlagSet.Bool(&help, "help", "h", false, "Display this information")

	if err := a.FlagSet.Parse(arguments); err != nil {
		fmt.Fprintln(os.Stderr, err)
		a.generateUsagePage(os.Stderr)
		return err
	}
	if help {
		a.generateHelpPage(os.Stdout)
		return nil
	}
	if a.Action != nil {
		return a.Action(a.FlagSet.Args())
	}
	return nil
}

func (a *App) generateUsagePage(w *os.File) {
	var sb strings.Builder
	termWidth := getTerminalWidth()
	indent := NewIndentState()

	fmt.Fprintf(&sb, "Usage: %s <options> [input.b] ...\n", a.Name)

	optionFlags := a.getOptionFlags()
	if len(optionFlags) > 0 {
		maxFlagWidth := 0
		maxUsageWidth := 0
		for _, flag := range optionFlags {
			flagStrLen := len(a.formatFlagString(flag))
			if flagStrLen > maxFlagWidth {
				maxFlagWidth = flagStrLen
			}
			usageLen := len(flag.Usage)
			if usageLen > maxUsageWidth {
				maxUsageWidth = usageLen
			}
		}

		sb.WriteString("\n")
		fmt.Fprintf(&sb, "%sOptions\n", indent.AtLevel(1))
		sort.Slice(optionFlags, func(i, j int) bool { return optionFlags[i].Name < optionFlags[j].Name })
		for _, flag := range optionFlags {
			a.formatFlagLine(&sb, flag, indent, termWidth, maxFlagWidth, maxUsageWidth)
		}
	}

	fmt.Fprintf(&sb, "\nRun '%s --help' for all available options and flags.\n", a.Name)
	fmt.Fprint(w, sb.String())
}

func (a *App) generateHelpPage(w *os.File) {
	var sb strings.Builder
	termWidth := getTerminalWidth()
	indent := NewIndentState()

	globalMaxWidth := a.calculateGlobalMaxWidth()

	globalMaxUsageWidth := 0
	updateMaxUsage := func(s string) {
		if len(s) > globalMaxUsageWidth {
			globalMaxUsageWidth = len(s)
		}
	}
	optionFlags := a.getOptionFlags()
	for _, flag := range optionFlags {
		updateMaxUsage(flag.Usage)
	}
	for _, group := range a.FlagSet.flagGroups {
		for _, entry := range group.Flags {
			updateMaxUsage(entry.Usage)
		}
	}

	year := time.Now().Year()
	sb.WriteString("\n")
	fmt.Fprintf(&sb, "%sCopyright (c) %d: %s\n", indent.AtLevel(1), year, strings.Join(a.Authors, ", ")+" and contributors")
	if a.Repository != "" {
		fmt.Fprintf(&sb, "%sFor more details refer to %s\n", indent.AtLevel(1), a.Repository)
	}

	if a.Synopsis != "" {
		sb.WriteString("\n")
		fmt.Fprintf(&sb, "%sSynopsis\n", indent.AtLevel(1))
		synopsis := strings.ReplaceAll(a.Synopsis, "[", "<")
		synopsis = strings.ReplaceAll(synopsis, "]", ">")
		fmt.Fprintf(&sb, "%s%s %s\n", indent.AtLevel(2), a.Name, synopsis)
	}

	if a.Description != "" {
		sb.WriteString("\n")
		fmt.Fprintf(&sb, "%sDescription\n", indent.AtLevel(1))
		fmt.Fprintf(&sb, "%s%s\n", indent.AtLevel(2), a.Description)
	}

	if len(optionFlags) > 0 {
		sb.WriteString("\n")
		fmt.Fprintf(&sb, "%sOptions\n", indent.AtLevel(1))
		sort.Slice(optionFlags, func(i, j int) bool { return optionFlags[i].Name < optionFlags[j].Name })
		for _, flag := range optionFlags {
			a.formatFlagLine(&sb, flag, indent, termWidth, globalMaxWidth, globalMaxUsageWidth)
		}
	}

	if len(a.FlagSet.flagGroups) > 0 {
		sortedGroups := make([]FlagGroup, len(a.FlagSet.flagGroups))
		copy(sortedGroups, a.FlagSet.flagGroups)
		sort.Slice(sortedGroups, func(i, j int) bool { return sortedGroups[i].Name < sortedGroups[j].Name })
		for _, group := range sortedGroups {
			a.formatFlagGroup(&sb, group, indent, termWidth, globalMaxWidth, globalMaxUsageWidth)
		}
	}
	fmt.Fprint(w, sb.String())
}

func (a *App) getOptionFlags() []*Flag {
	var optionFlags []*Flag
	for _, flag := range a.FlagSet.flags {
		if _, isSpecial := a.FlagSet.specialPrefix[flag.Name]; isSpecial {
			continue
		}
		if a.isGroupFlag(flag.Name) {
			continue
		}
		optionFlags = append(optionFlags, flag)
	}
	return optionFlags
}

func (a *App) isGroupFlag(flagName string) bool {
	for _, group := range a.FlagSet.flagGroups {
		for _, entry := range group.Flags {
			if flagName == entry.Prefix+entry.Name || flagName == entry.Prefix+"no-"+entry.Name {
				return true
			}
		}
	}
	return false
}

func (a *App) calculateGlobalMaxWidth() int {
	maxWidth := 0
	checkWidth := func(s string) {
		if len(s) > maxWidth {
			maxWidth = len(s)
		}
	}
	for _, flag := range a.getOptionFlags() {
		checkWidth(a.formatFlagString(flag))
	}
	for _, group := range a.FlagSet.flagGroups {
		prefix := group.Flags[0].Prefix
		groupType := strings.ToLower(strings.TrimSuffix(group.Name, "s"))
		checkWidth(fmt.Sprintf("-%s<%s>", prefix, groupType))
		checkWidth(fmt.Sprintf("-%sno-<%s>", prefix, groupType))
		for _, entry := range group.Flags {
			checkWidth(entry.Name)
		}
	}
	return maxWidth
}

func (a *App) formatFlagString(flag *Flag) string {
	var flagStr strings.Builder
	_, isBool := flag.Value.(*boolValue)

	if flag.Shorthand != "" {
		fmt.Fprintf(&flagStr, "-%s", flag.Shorthand)
		if !isBool {
			fmt.Fprintf(&flagStr, " <%s>", flag.ExpectedType)
		}
		fmt.Fprintf(&flagStr, ", --%s", flag.Name)
		if !isBool {
			fmt.Fprintf(&flagStr, " <%s>", flag.ExpectedType)
		}
	} else {
		fmt.Fprintf(&flagStr, "--%s", flag.Name)
		if !isBool {
			if flag.ExpectedType != "" {
				fmt.Fprintf(&flagStr, "=%s", flag.ExpectedType)
			}
		}
	}
	return flagStr.String()
}

func (a *App) formatEntry(sb *strings.Builder, indent *IndentState, termWidth int, leftPart, usagePart, rightPart string, globalLeftWidth, globalMaxUsageWidth int) {
	indentStr := indent.AtLevel(2)
	indentWidth := len(indentStr)
	spaceWidth := 1

	fixedPartsWidth := indentWidth + globalLeftWidth + spaceWidth + 2 + len(rightPart)
	maxFirstUsageWidth := termWidth - fixedPartsWidth
	if maxFirstUsageWidth < 10 {
		maxFirstUsageWidth = 10
	}

	usageLines := wrapText(usagePart, maxFirstUsageWidth)

	firstUsageLine := ""
	if len(usageLines) > 0 {
		firstUsageLine = usageLines[0]
	}

	desiredUsageWidth := globalMaxUsageWidth
	if desiredUsageWidth > maxFirstUsageWidth {
		desiredUsageWidth = maxFirstUsageWidth
	}

	if rightPart != "" {
		fmt.Fprintf(sb, "%s%-*s %-*s  %s\n", indent.AtLevel(2), globalLeftWidth, leftPart, desiredUsageWidth, firstUsageLine, rightPart)
	} else {
		fmt.Fprintf(sb, "%s%-*s %s\n", indent.AtLevel(2), globalLeftWidth, leftPart, firstUsageLine)
	}

	wrappedIndent := strings.Repeat(" ", globalLeftWidth+spaceWidth)

	availableWrappedWidth := termWidth - (indentWidth + globalLeftWidth + spaceWidth)
	if availableWrappedWidth < 10 {
		availableWrappedWidth = 10
	}

	wrappedLineMaxWidth := desiredUsageWidth + 2
	termAvailable := termWidth - (indentWidth + globalLeftWidth + spaceWidth)
	if wrappedLineMaxWidth > termAvailable {
		wrappedLineMaxWidth = termAvailable
	}

	for i := 1; i < len(usageLines); i++ {
		fmt.Fprintf(sb, "%s%s%s\n", indentStr, wrappedIndent, usageLines[i])
	}
}

func (a *App) formatFlagLine(sb *strings.Builder, flag *Flag, indent *IndentState, termWidth, globalMaxWidth, globalMaxUsageWidth int) {
	leftPart := a.formatFlagString(flag)
	usagePart := flag.Usage

	rightPart := ""
	if flag.DefValue != "" && flag.DefValue != "false" && flag.DefValue != "[]" {
		if _, isBool := flag.Value.(*boolValue); !isBool {
			rightPart = fmt.Sprintf("|%s|", flag.DefValue)
		}
	}
	a.formatEntry(sb, indent, termWidth, leftPart, usagePart, rightPart, globalMaxWidth, globalMaxUsageWidth)
}

func (a *App) formatFlagGroup(sb *strings.Builder, group FlagGroup, indent *IndentState, termWidth, globalMaxWidth, globalMaxUsageWidth int) {
	sb.WriteString("\n")
	fmt.Fprintf(sb, "%s%s\n", indent.AtLevel(1), group.Name)

	prefix := group.Flags[0].Prefix
	groupType := group.GroupType
	if groupType == "" {
		groupType = "flag"
	}

	fmt.Fprintf(sb, "%s%-*s Enable a specific %s\n", indent.AtLevel(2), globalMaxWidth, fmt.Sprintf("-%s<%s>", prefix, groupType), groupType)
	fmt.Fprintf(sb, "%s%-*s Disable a specific %s\n", indent.AtLevel(2), globalMaxWidth, fmt.Sprintf("-%sno-<%s>", prefix, groupType), groupType)

	if group.AvailableFlagsHeader != "" {
		fmt.Fprintf(sb, "%s%s\n", indent.AtLevel(1), group.AvailableFlagsHeader)
	}

	sortedEntries := make([]FlagGroupEntry, len(group.Flags))
	copy(sortedEntries, group.Flags)
	sort.Slice(sortedEntries, func(i, j int) bool { return sortedEntries[i].Name < sortedEntries[j].Name })

	for _, entry := range sortedEntries {
		rightPart := ""
		if entry.Enabled != nil && *entry.Enabled && (entry.Disabled == nil || !*entry.Disabled) {
			rightPart = "|x|"
		} else {
			rightPart = "|-|"
		}
		a.formatEntry(sb, indent, termWidth, entry.Name, entry.Usage, rightPart, globalMaxWidth, globalMaxUsageWidth)
	}
}

func getTerminalWidth() int {
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		return 80
	}
	if width < 20 {
		return 20
	}
	return width
}

func wrapText(text string, maxWidth int) []string {
	if maxWidth <= 0 {
		return []string{text}
	}
	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{}
	}

	var lines []string
	var currentLine strings.Builder
	currentLen := 0

	for _, word := range words {
		wordLen := len(word)
		if currentLen+wordLen+1 > maxWidth && currentLen > 0 {
			lines = append(lines, currentLine.String())
			currentLine.Reset()
			currentLen = 0
		}
		if currentLen > 0 {
			currentLine.WriteString(" ")
			currentLen++
		}
		currentLine.WriteString(word)
		currentLen += wordLen
	}
	if currentLine.Len() > 0 {
		lines = append(lines, currentLine.String())
	}
	return lines
}
