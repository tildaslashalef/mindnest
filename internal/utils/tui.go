package utils

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jedib0t/go-pretty/v6/list"
	"github.com/jedib0t/go-pretty/v6/progress"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
)

// Theme implements a Gruvbox-inspired dark theme
var (
	// Gruvbox palette - these are only used within this file
	// The actual exported colors are in the Theme struct
	gruvboxBg0     = text.Colors{text.BgHiBlack}
	gruvboxBg1     = text.Colors{text.BgBlack}
	gruvboxFgDark  = text.Colors{text.FgHiBlack}
	gruvboxFgLight = text.Colors{text.FgWhite}
	gruvboxRed     = text.Colors{text.FgRed}
	gruvboxGreen   = text.Colors{text.FgGreen}
	gruvboxYellow  = text.Colors{text.FgYellow}
	gruvboxBlue    = text.Colors{text.FgBlue}
	gruvboxPurple  = text.Colors{text.FgMagenta}
	gruvboxAqua    = text.Colors{text.FgCyan}
	gruvboxOrange  = text.Colors{text.FgYellow, text.Bold}

	// Bright variants
	gruvboxRedBright    = text.Colors{text.FgHiRed}
	gruvboxGreenBright  = text.Colors{text.FgHiGreen}
	gruvboxYellowBright = text.Colors{text.FgHiYellow}
	gruvboxBlueBright   = text.Colors{text.FgHiBlue}
	gruvboxPurpleBright = text.Colors{text.FgHiMagenta}
	gruvboxAquaBright   = text.Colors{text.FgHiCyan}

	// Text styles
	gruvboxBold      = text.Colors{text.Bold}
	gruvboxItalic    = text.Colors{text.Italic}
	gruvboxUnderline = text.Colors{text.Underline}

	// Combined themes (text + background)
	gruvboxRedOnDark    = append(text.Colors{}, append(gruvboxRed, gruvboxBg0...)...)
	gruvboxGreenOnDark  = append(text.Colors{}, append(gruvboxGreen, gruvboxBg0...)...)
	gruvboxYellowOnDark = append(text.Colors{}, append(gruvboxYellow, gruvboxBg0...)...)
	gruvboxBlueOnDark   = append(text.Colors{}, append(gruvboxBlue, gruvboxBg0...)...)
	gruvboxPurpleOnDark = append(text.Colors{}, append(gruvboxPurple, gruvboxBg0...)...)
	gruvboxAquaOnDark   = append(text.Colors{}, append(gruvboxAqua, gruvboxBg0...)...)
	gruvboxOrangeOnDark = append(text.Colors{}, append(gruvboxOrange, gruvboxBg0...)...)
)

// Theme - exported theme colors for consistent UI
var Theme = struct {
	// Semantic colors for different message types
	Success   text.Colors
	Info      text.Colors
	Warning   text.Colors
	Error     text.Colors
	Heading   text.Colors
	Subtle    text.Colors
	Important text.Colors
	Accent    text.Colors

	// UI Elements
	Title       text.Colors
	Divider     text.Colors
	TableHeader text.Colors
	TableBorder text.Colors
	TableRow    text.Colors
	TableAltRow text.Colors
	Badge       text.Colors
	Code        text.Colors
}{
	Success:   gruvboxGreen,
	Info:      gruvboxBlue,
	Warning:   gruvboxYellow,
	Error:     gruvboxRed,
	Heading:   append(gruvboxAquaBright, text.Bold),
	Subtle:    gruvboxFgDark,
	Important: append(gruvboxPurpleBright, text.Bold),
	Accent:    gruvboxAqua,

	Title:       append(gruvboxAquaBright, text.Bold),
	Divider:     gruvboxFgDark,
	TableHeader: append(gruvboxBlueBright, text.Bold),
	TableBorder: gruvboxBlue,
	TableRow:    gruvboxFgLight,
	TableAltRow: text.Colors{text.FgWhite, text.Faint},
	Badge:       append(gruvboxYellowBright, text.Bold),
	Code:        gruvboxGreenBright,
}

// PrintHeading prints a formatted heading
func PrintHeading(title string) {
	fmt.Println(Theme.Heading.Sprint(title))
}

// PrintSuccess prints a success message
func PrintSuccess(message string) {
	fmt.Println(Theme.Success.Sprint("✓ ") + message)
}

// PrintInfo prints an info message
func PrintInfo(message string) {
	fmt.Println(Theme.Info.Sprint("ℹ ") + message)
}

// PrintWarning prints a warning message
func PrintWarning(message string) {
	fmt.Println(Theme.Warning.Sprint("⚠ ") + message)
}

// PrintError prints an error message
func PrintError(message string) {
	fmt.Println(Theme.Error.Sprint("✗ ") + message)
}

// PrintKeyValue prints a key-value pair
func PrintKeyValue(key, value string) {
	fmt.Printf("%s: %s\n", gruvboxBold.Sprint(key), value)
}

// PrintKeyValueWithColor prints a key-value pair with colored value
func PrintKeyValueWithColor(key string, value string, colors text.Colors) {
	fmt.Printf("%s: %s\n", gruvboxBold.Sprint(key), colors.Sprint(value))
}

// PrintCode prints a code block with syntax highlighting
func PrintCode(code string, language string) {
	fmt.Println(Theme.Code.Sprint(code))
}

// PrintDivider prints a horizontal divider
func PrintDivider() {
	fmt.Println(Theme.Divider.Sprint("---------------------------------------------------"))
}

// PrintBadge prints a badge-like text
func PrintBadge(message string) {
	fmt.Printf(" %s ", Theme.Badge.Sprint(message))
}

// PrintSubHeading prints a formatted sub-heading
func PrintSubHeading(title string) {
	fmt.Println(Theme.Info.Sprint(title))
}

// HighlightText returns text with highlighted styling for emphasis
func HighlightText(text string) string {
	return Theme.Error.Sprint(text)
}

// CodeBlock formats text as a code block with proper indentation
func CodeBlock(code string) string {
	// Split into lines to ensure proper indentation
	lines := strings.Split(code, "\n")

	// Add consistent indentation for all lines
	for i, line := range lines {
		lines[i] = "    " + line
	}

	// Join lines and style with the code theme
	return Theme.Code.Sprint(strings.Join(lines, "\n"))
}

// TableOptions defines options for table creation
type TableOptions struct {
	Title       string
	HeaderStyle text.Colors
	RowStyle    text.Colors
	BorderStyle text.Colors
	Style       table.Style
	// Pagination options
	EnablePagination bool
	PageSize         int
	CurrentPage      int
	TotalRows        int
}

// DefaultTableOptions returns default table options with Gruvbox theme
func DefaultTableOptions() TableOptions {
	return TableOptions{
		Title:            "Mindnest",
		HeaderStyle:      text.Colors{text.BgBlue, text.FgHiWhite, text.Bold},
		RowStyle:         text.Colors{text.FgWhite},
		BorderStyle:      text.Colors{text.FgBlue},
		Style:            table.StyleLight,
		EnablePagination: false,
		PageSize:         10,
		CurrentPage:      1,
		TotalRows:        0,
	}
}

// CreateTable creates a new table with default styling
func CreateTable(options ...TableOptions) table.Writer {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)

	// Use default options or the provided ones
	opts := DefaultTableOptions()
	if len(options) > 0 {
		opts = options[0]
	}

	// Set title if provided
	if opts.Title != "" {
		t.SetTitle(opts.Title)
	}

	// Use StyleDouble which has very visible borders
	customStyle := table.StyleDouble

	// Apply explicit colors that will be visible in the terminal
	// Use the Gruvbox theme colors defined earlier
	customStyle.Color.Header = Theme.TableHeader
	customStyle.Color.Border = Theme.TableBorder
	customStyle.Color.Row = Theme.TableRow
	customStyle.Color.RowAlternate = Theme.TableAltRow
	customStyle.Title.Colors = Theme.Title
	customStyle.Title.Align = text.AlignCenter

	// Make sure borders and separators are enabled
	customStyle.Options.DrawBorder = true
	customStyle.Options.SeparateColumns = true
	customStyle.Options.SeparateFooter = true
	customStyle.Options.SeparateHeader = true

	// Add padding for better readability
	customStyle.Box.PaddingLeft = " "
	customStyle.Box.PaddingRight = " "

	// Apply the custom style to the table
	t.SetStyle(customStyle)

	// Enable alternating rows
	t.Style().Options.SeparateRows = false

	return t
}

// PrintTable prints a table with headers and rows
func PrintTable(headers []string, rows [][]string, options ...TableOptions) {
	// Create table with provided or default options
	opts := DefaultTableOptions()
	if len(options) > 0 {
		opts = options[0]
	}

	t := CreateTable(opts)

	// Add headers
	headerRow := table.Row{}
	for _, header := range headers {
		headerRow = append(headerRow, header)
	}
	t.AppendHeader(headerRow)

	// Pagination logic
	startIndex := 0
	endIndex := len(rows)

	if opts.EnablePagination {
		if opts.TotalRows == 0 {
			opts.TotalRows = len(rows)
		}

		totalPages := (opts.TotalRows + opts.PageSize - 1) / opts.PageSize
		if opts.CurrentPage < 1 {
			opts.CurrentPage = 1
		} else if opts.CurrentPage > totalPages && totalPages > 0 {
			opts.CurrentPage = totalPages
		}

		startIndex = (opts.CurrentPage - 1) * opts.PageSize
		endIndex = startIndex + opts.PageSize
		if endIndex > len(rows) {
			endIndex = len(rows)
		}
	}

	// Add rows for current page
	for i := startIndex; i < endIndex; i++ {
		tableRow := table.Row{}
		for _, cell := range rows[i] {
			tableRow = append(tableRow, cell)
		}
		t.AppendRow(tableRow)
	}

	// Set column configurations for alignment
	configs := []table.ColumnConfig{}
	for i := range headers {
		configs = append(configs, table.ColumnConfig{
			Number:      i + 1,
			Align:       text.AlignLeft,
			AlignHeader: text.AlignCenter,
		})
	}
	t.SetColumnConfigs(configs)

	// Render the table
	t.Render()

	// Show pagination information if enabled
	if opts.EnablePagination {
		totalPages := (opts.TotalRows + opts.PageSize - 1) / opts.PageSize
		paginationInfo := fmt.Sprintf("Page %d of %d", opts.CurrentPage, totalPages)
		fmt.Println(Theme.Subtle.Sprint(paginationInfo))
	}
}

// FormatList formats a list of items with bullets
func FormatList(items []string, bullet string) string {
	if bullet == "" {
		bullet = "•"
	}

	var result strings.Builder
	for _, item := range items {
		result.WriteString(fmt.Sprintf("%s %s\n", Theme.Accent.Sprint(bullet), item))
	}

	return result.String()
}

// PrintList prints a formatted list of items
func PrintList(items []string, bullet string) {
	fmt.Print(FormatList(items, bullet))
}

// WrapText wraps text to a specific width with optional indentation
func WrapText(str string, width int, indent string) string {
	return text.WrapText(str, width)
}

// Center centers text within a given width
func Center(str string, width int) string {
	return text.AlignCenter.Apply(str, width)
}

// ProgressOptions defines options for progress tracking
type ProgressOptions struct {
	Title          string
	AutoStop       bool
	ShowPercentage bool
	ShowSpeed      bool
	ShowTime       bool
	Style          progress.Style
	TotalUnits     int64
}

// DefaultProgressOptions returns default progress options with Gruvbox theme
func DefaultProgressOptions() ProgressOptions {
	return ProgressOptions{
		Title:          "Progress",
		AutoStop:       true,
		ShowPercentage: true,
		ShowSpeed:      true,
		ShowTime:       true,
		Style:          progress.StyleDefault,
		TotalUnits:     0,
	}
}

// CreateProgressTracker creates a progress tracker for a single task
func CreateProgressTracker(message string, totalUnits int64, options ...ProgressOptions) *progress.Tracker {
	tracker := &progress.Tracker{
		Message: message,
		Total:   totalUnits,
		Units:   progress.UnitsDefault,
	}

	return tracker
}

// CreateProgressWriter creates a progress writer to track multiple tasks
func CreateProgressWriter(options ...ProgressOptions) progress.Writer {
	// Use default options or the provided ones
	opts := DefaultProgressOptions()
	if len(options) > 0 {
		opts = options[0]
	}

	pw := progress.NewWriter()
	pw.SetAutoStop(opts.AutoStop)
	pw.SetTrackerLength(25)
	pw.SetMessageLength(40)
	pw.SetNumTrackersExpected(1)
	pw.SetSortBy(progress.SortByPercentDsc)
	pw.SetStyle(opts.Style)
	pw.SetTrackerPosition(progress.PositionRight)
	pw.SetUpdateFrequency(time.Millisecond * 100)
	pw.Style().Colors.Message = Theme.Info
	pw.Style().Colors.Percent = Theme.Important
	pw.Style().Colors.Time = Theme.Subtle
	pw.Style().Colors.Value = Theme.Success
	pw.Style().Options.PercentFormat = " %.1f%%"

	pw.SetOutputWriter(os.Stdout)

	return pw
}

// RenderProgressTrackers starts tracking and rendering the progress
func RenderProgressTrackers(pw progress.Writer, trackers []*progress.Tracker) {
	// Add all trackers
	for _, tracker := range trackers {
		pw.AppendTracker(tracker)
	}

	// Start the tracking/rendering
	go pw.Render()
}

// CreateList creates a new hierarchical list with default styling
func CreateList() list.Writer {
	l := list.NewWriter()

	// Apply a connected rounded style
	l.SetStyle(list.StyleConnectedRounded)

	// Set the output to stdout
	l.SetOutputMirror(os.Stdout)

	return l
}

// RenderNestedList renders a nested list with multiple levels
func RenderNestedList(items map[string][]string) {
	l := CreateList()

	// Add all items from the map
	for title, subItems := range items {
		// Add the root item
		l.AppendItem(title)

		// Add nested items
		for _, subItem := range subItems {
			// Indent to create a nested level
			l.Indent()
			l.AppendItem(subItem)
			l.UnIndent()
		}
	}

	// Print the list
	fmt.Println(l.Render())
}

// PrintTreeList prints a tree-like list with parent-child relationships
func PrintTreeList(title string, items []string) {
	l := CreateList()
	l.AppendItem(title)

	// Indent once for all child items
	l.Indent()

	// Add all child items
	for _, item := range items {
		l.AppendItem(item)
	}

	// Reset indentation
	l.UnIndent()

	fmt.Println(l.Render())
}

// PrintPaginatedTable prints a table with pagination and handles user input for navigation
func PrintPaginatedTable(headers []string, rows [][]string, pageSize int, title string) {
	currentPage := 1
	totalRows := len(rows)
	totalPages := (totalRows + pageSize - 1) / pageSize

	// If no rows, just show empty table and return
	if totalRows == 0 {
		opts := DefaultTableOptions()
		opts.Title = title
		PrintTable(headers, rows, opts)
		fmt.Println(Theme.Subtle.Sprint("No records found."))
		return
	}

	var lastChoice string
	var invalidInput bool

	for {
		// Clear screen before printing the table
		fmt.Print("\033[H\033[2J")

		// Create pagination options
		opts := DefaultTableOptions()
		opts.EnablePagination = true
		opts.PageSize = pageSize
		opts.CurrentPage = currentPage
		opts.TotalRows = totalRows
		opts.Title = title

		// Print the table with current page
		PrintTable(headers, rows, opts)

		// Print navigation instructions with Gruvbox theme
		if totalPages > 1 {
			fmt.Println()
			fmt.Printf("%s %s\n",
				Theme.Accent.Sprint("◆"),
				Theme.Heading.Sprint("Navigation"))

			// Create a more structured navigation menu with Gruvbox colors
			fmt.Println(Theme.Divider.Sprint("───────────────────────────────────────────"))

			// Navigation options in two columns
			navOptions := []struct {
				key      string
				action   string
				keyColor text.Colors
			}{
				{"f", "First page", Theme.Badge},
				{"p", "Previous page", Theme.Badge},
				{"n", "Next page", Theme.Badge},
				{"l", "Last page", Theme.Badge},
				{"#", "Jump to page", Theme.Badge},
				{"q", "Exit", Theme.Badge},
			}

			// Display in two columns
			for i := 0; i < len(navOptions); i += 2 {
				if i+1 < len(navOptions) {
					left := fmt.Sprintf("%s %s",
						navOptions[i].keyColor.Sprint(navOptions[i].key),
						Theme.TableRow.Sprint(navOptions[i].action))

					right := fmt.Sprintf("%s %s",
						navOptions[i+1].keyColor.Sprint(navOptions[i+1].key),
						Theme.TableRow.Sprint(navOptions[i+1].action))

					fmt.Printf("%-30s %s\n", left, right)
				} else {
					fmt.Printf("%s %s\n",
						navOptions[i].keyColor.Sprint(navOptions[i].key),
						Theme.TableRow.Sprint(navOptions[i].action))
				}
			}

			// Page counter with Gruvbox style
			pageCounter := fmt.Sprintf("Page %d of %d", currentPage, totalPages)
			fmt.Println(Theme.Divider.Sprint("───────────────────────────────────────────"))
			fmt.Println(Theme.Info.Sprint(pageCounter))

			// Show error message if invalid input was provided
			if invalidInput {
				fmt.Println(Theme.Error.Sprint("Invalid choice. Please try again."))
				invalidInput = false
			}

			prompt := "Enter choice: "
			fmt.Print(Theme.Subtle.Sprint(prompt))

			// Read user input
			var choice string
			fmt.Scanln(&choice)

			// If empty, use last choice if available
			if choice == "" && lastChoice != "" {
				choice = lastChoice
			} else if choice != "" {
				lastChoice = choice
			}

			choice = strings.ToLower(choice)

			switch choice {
			case "f", "first":
				currentPage = 1
			case "p", "prev", "previous":
				if currentPage > 1 {
					currentPage--
				}
			case "n", "next":
				if currentPage < totalPages {
					currentPage++
				}
			case "l", "last":
				currentPage = totalPages
			case "q", "quit", "exit":
				return
			default:
				// Handle page number input
				var pageNum int
				if _, err := fmt.Sscanf(choice, "%d", &pageNum); err == nil && pageNum > 0 {
					if pageNum <= totalPages {
						currentPage = pageNum
					} else {
						invalidInput = true
					}
				} else {
					invalidInput = true
				}
			}
		} else {
			// If only one page, wait for user to acknowledge
			fmt.Println()
			fmt.Println(Theme.Divider.Sprint("───────────────────────────────────────────"))
			fmt.Print(Theme.Subtle.Sprint("Press Enter to continue..."))
			fmt.Scanln()
			return
		}
	}
}
