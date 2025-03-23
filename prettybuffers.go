package prettybuffers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// ColumnType represents the type of column to display
type ColumnType int

const (
	// ColumnOffset displays the byte offset
	ColumnOffset ColumnType = iota
	// ColumnHex displays hexadecimal representation
	ColumnHex
	// ColumnASCII displays ASCII representation
	ColumnASCII
	// ColumnJSON displays JSON representation if possible
	ColumnJSON
)

// jsonObject represents a detected JSON object in the byte stream
type jsonObject struct {
	startOffset int
	endOffset   int
	data        []byte
	parsed      interface{}
}

// Layout represents a specific arrangement of columns
type Layout struct {
	Name    string
	Columns []ColumnType
}

// PredefinedLayouts contains the available layouts
var PredefinedLayouts = []Layout{
	{Name: "Hex View", Columns: []ColumnType{ColumnOffset, ColumnHex, ColumnASCII}},
	{Name: "Smart View", Columns: []ColumnType{ColumnOffset, ColumnHex, ColumnJSON, ColumnASCII}},
}

// model represents the application state
type model struct {
	data        []byte
	offset      int
	bytesPerRow int
	width       int
	height      int
	layout      Layout
	layoutIndex int
	jsonObjects []jsonObject
}

func initialModel() model {
	return model{
		data:        []byte{},
		offset:      0,
		bytesPerRow: 16, // Default value, will be adjusted based on terminal width
		width:       80,
		height:      24,
		layout:      PredefinedLayouts[0], // Default to first layout (Hex View)
		layoutIndex: 0,
		jsonObjects: []jsonObject{},
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "up", "k":
			if m.offset >= m.bytesPerRow {
				m.offset -= m.bytesPerRow
			}
		case "down", "j":
			if m.offset+m.bytesPerRow < len(m.data) {
				m.offset += m.bytesPerRow
			}
		case "page_up":
			rowsPerPage := m.height - 2
			if m.offset >= m.bytesPerRow*rowsPerPage {
				m.offset -= m.bytesPerRow * rowsPerPage
			} else {
				m.offset = 0
			}
		case "page_down":
			rowsPerPage := m.height - 2
			if m.offset+m.bytesPerRow*rowsPerPage < len(m.data) {
				m.offset += m.bytesPerRow * rowsPerPage
			}
		case "l":
			// Switch to next layout
			m.layoutIndex = (m.layoutIndex + 1) % len(PredefinedLayouts)
			m.layout = PredefinedLayouts[m.layoutIndex]
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Adjust bytes per row based on terminal width
		// Each byte needs about 3 characters in hex view (2 hex digits + space)
		// Plus offset (12 chars), separators (4 chars), and ASCII view (1 char per byte)
		// We'll leave some margin for safety
		availableWidth := m.width - 20
		if availableWidth > 0 {
			// Calculate how many bytes we can fit
			m.bytesPerRow = availableWidth / 4 // 3 for hex + 1 for ASCII
			// Ensure it's at least 8 bytes and a multiple of 8 for clean display
			if m.bytesPerRow < 8 {
				m.bytesPerRow = 8
			} else {
				m.bytesPerRow = (m.bytesPerRow / 8) * 8
			}
		}
	case bytesMsg:
		m.data = []byte(msg)
		// Detect JSON objects in the data
		m.jsonObjects = findJSONObjects(m.data)
	case layoutMsg:
		layoutIndex := int(msg)
		if layoutIndex >= 0 && layoutIndex < len(PredefinedLayouts) {
			m.layoutIndex = layoutIndex
			m.layout = PredefinedLayouts[layoutIndex]
		}
	}

	return m, nil
}

func (m model) View() string {
	if len(m.data) == 0 {
		return "No data to display. Press q to quit."
	}

	var sb strings.Builder

	// Display current layout name
	sb.WriteString(fmt.Sprintf("Layout: %s\n\n", m.layout.Name))

	// Calculate how many rows we can display
	rowsToDisplay := m.height - 5 // Leave room for header, separator, layout name, and footer
	if rowsToDisplay < 1 {
		rowsToDisplay = 1
	}

	// Check which view we're using
	if m.layout.Name == "Smart View" {
		return m.renderSmartView(rowsToDisplay)
	}

	// Create dynamic header based on bytes per row and columns
	hasOffset := containsColumn(m.layout.Columns, ColumnOffset)
	hasHex := containsColumn(m.layout.Columns, ColumnHex)
	hasASCII := containsColumn(m.layout.Columns, ColumnASCII)

	// Header
	if hasOffset {
		sb.WriteString("Offset    ")
	}

	hexHeaderWidth := m.bytesPerRow*3 - 1 // 3 chars per byte (2 hex + 1 space) minus trailing space
	asciiHeaderWidth := m.bytesPerRow

	if hasHex {
		if hasOffset {
			sb.WriteString("| ")
		}
		sb.WriteString(fmt.Sprintf("%-*s ", hexHeaderWidth, "Hexadecimal"))
	}

	if hasASCII {
		sb.WriteString("| ")
		sb.WriteString(fmt.Sprintf("%-*s", asciiHeaderWidth, "ASCII"))
	}
	sb.WriteString("\n")

	// Separator line
	if hasOffset {
		sb.WriteString("----------")
	}

	if hasHex {
		if hasOffset {
			sb.WriteString("+-")
		} else {
			sb.WriteString("-")
		}
		sb.WriteString(strings.Repeat("-", hexHeaderWidth))
	}

	if hasASCII {
		sb.WriteString("-+-")
		sb.WriteString(strings.Repeat("-", asciiHeaderWidth))
	}
	sb.WriteString("\n")

	// Calculate the starting offset
	startOffset := m.offset - (m.offset % m.bytesPerRow)

	// Display rows
	for row := 0; row < rowsToDisplay; row++ {
		currentOffset := startOffset + (row * m.bytesPerRow)
		if currentOffset >= len(m.data) {
			break
		}

		// Offset column
		if hasOffset {
			sb.WriteString(fmt.Sprintf("0x%08X ", currentOffset))
		}

		// Hex columns
		var hexPart strings.Builder
		var asciiPart strings.Builder

		for col := 0; col < m.bytesPerRow; col++ {
			pos := currentOffset + col
			if pos < len(m.data) {
				if hasHex {
					hexPart.WriteString(fmt.Sprintf("%02X ", m.data[pos]))
				}

				// ASCII representation
				if hasASCII {
					if m.data[pos] >= 32 && m.data[pos] <= 126 {
						asciiPart.WriteRune(rune(m.data[pos]))
					} else {
						asciiPart.WriteRune('.')
					}
				}
			} else {
				if hasHex {
					hexPart.WriteString("   ")
				}
				if hasASCII {
					asciiPart.WriteRune(' ')
				}
			}
		}

		if hasHex {
			// Trim the trailing space from hex part
			hexStr := strings.TrimRight(hexPart.String(), " ")

			// Ensure the hex part fills the allocated space
			hexWidth := m.bytesPerRow*3 - 1

			if hasOffset {
				sb.WriteString("| ")
			}
			sb.WriteString(fmt.Sprintf("%-*s", hexWidth, hexStr))
		}

		// ASCII column
		if hasASCII {
			sb.WriteString(" | ")
			sb.WriteString(asciiPart.String())
		}
		sb.WriteString("\n")
	}

	// Footer
	sb.WriteString(
		fmt.Sprintf(
			"\nShowing %d/%d bytes. Use arrow keys to navigate, 'l' to switch layout, 'q' to quit.",
			min(len(m.data), m.bytesPerRow*rowsToDisplay),
			len(m.data),
		),
	)

	return sb.String()
}

// sanitizeString converts a string to ASCII-safe representation
func sanitizeString(s string) string {
	var result strings.Builder
	for _, ch := range s {
		if ch >= 32 && ch <= 126 {
			result.WriteRune(ch)
		} else {
			result.WriteRune('.')
		}
	}
	return result.String()
}

func (m model) renderSmartView(rowsToDisplay int) string {
	var sb strings.Builder

	// Display current layout name
	sb.WriteString(fmt.Sprintf("Layout: %s\n\n", m.layout.Name))

	if len(m.data) == 0 {
		sb.WriteString("No data to display.\n\n")
		sb.WriteString("Press 'l' to switch layout, 'q' to quit.")
		return sb.String()
	}

	// Use a responsive hex column based on terminal width
	hexBytesPerRow := 8 // Default
	if m.width > 100 {
		hexBytesPerRow = 16
	} else if m.width < 80 {
		hexBytesPerRow = 4
	}

	// Determine if we're currently viewing a JSON object
	currentJSONIndex := -1
	for i, obj := range m.jsonObjects {
		if m.offset >= obj.startOffset && m.offset <= obj.endOffset {
			currentJSONIndex = i
			break
		}
	}

	// Pre-process ALL JSON objects to determine display requirements
	var maxHexColWidth int = 65 // Default minimum width to ensure sufficient space
	
	// Analyze all JSON objects to find the max required width
	for _, obj := range m.jsonObjects {
		var prettyJSON bytes.Buffer
		err := json.Indent(&prettyJSON, obj.data, "", "  ")
		if err == nil {
			// Find the maximum line length in the prettified JSON
			jsonLines := strings.Split(prettyJSON.String(), "\n")
			for _, line := range jsonLines {
				content := strings.TrimSpace(line)
				contentLen := len(content)
				if contentLen > 0 {
					// Each byte needs 3 characters in hex (2 for hex, 1 for space)
					requiredWidth := contentLen * 3
					if requiredWidth > maxHexColWidth {
						maxHexColWidth = requiredWidth
					}
				}
			}
		}
	}

	// Ensure the column width is reasonable
	maxHexColWidth = min(maxHexColWidth, m.width/2)
	
	// Header with updated width
	sb.WriteString(fmt.Sprintf("%-10s | %-*s | Content\n", "Offset", maxHexColWidth, "Hex"))

	// Calculate the content column width
	contentColWidth := m.width - (maxHexColWidth + 15) // Account for offset column, hex column and separators
	if contentColWidth < 20 {
		contentColWidth = 20 // Ensure minimum readable width
	}

	// Separator line
	sb.WriteString(fmt.Sprintf("%s+-%s-+-%s\n",
		strings.Repeat("-", 10),
		strings.Repeat("-", maxHexColWidth),
		strings.Repeat("-", contentColWidth)))

	// Keep track of which parts of the data are covered by JSON objects
	jsonCovered := make(map[int]bool)

	// Mark which bytes are part of JSON objects
	for _, obj := range m.jsonObjects {
		for i := obj.startOffset; i <= obj.endOffset; i++ {
			jsonCovered[i] = true
		}
	}

	// Find the JSON object that contains the current offset, if any
	var currentObj *jsonObject
	if currentJSONIndex >= 0 {
		currentObj = &m.jsonObjects[currentJSONIndex]
	}

	rowsRendered := 0
	startPos := m.offset

	// If we're in the middle of a JSON object, adjust our offset to show it correctly
	if currentObj != nil {
		// If we're in a JSON object, start from the beginning of it
		startPos = currentObj.startOffset
	}

	// Start rendering from the calculated position
	currentPos := startPos

	// Render data
	for rowsRendered < rowsToDisplay && currentPos < len(m.data) {
		// Check if the current position is the start of a JSON object
		jsonObjIndex := -1
		for i, obj := range m.jsonObjects {
			if obj.startOffset == currentPos {
				jsonObjIndex = i
				break
			}
		}

		// If we're at the start of a JSON object, render it
		if jsonObjIndex >= 0 {
			obj := m.jsonObjects[jsonObjIndex]

			// Format the JSON prettily
			var prettyJSON bytes.Buffer
			err := json.Indent(&prettyJSON, obj.data, "", "  ")

			if err != nil {
				// If we can't prettify, just show a single row with hex and raw JSON
				hexPart := formatHexBytes(obj.data[:min(hexBytesPerRow, len(obj.data))], hexBytesPerRow)
				sb.WriteString(fmt.Sprintf("0x%08X | %-*s | %s\n",
					obj.startOffset,
					maxHexColWidth,
					hexPart,
					sanitizeString(string(obj.data))))
				rowsRendered++
				currentPos = obj.endOffset + 1
				continue
			}

			// Split the pretty JSON into lines
			jsonLines := strings.Split(prettyJSON.String(), "\n")

			// Display each line of the JSON
			for i, line := range jsonLines {
				if rowsRendered >= rowsToDisplay {
					break
				}

				// Format the row with hex of the actual characters on this line
				hexValues := ""
				if i == 0 {
					// First line - the opening brace
					hexValues = formatDynamicHexBytes([]byte{'{'}, maxHexColWidth)
				} else if i == len(jsonLines)-1 {
					// Last line - the closing brace
					hexValues = formatDynamicHexBytes([]byte{'}'}, maxHexColWidth)
				} else if len(line) > 0 {
					// Process the actual characters in this line (skip whitespace)
					lineContent := strings.TrimSpace(line)
					
					// If the line has content, show its hex
					if len(lineContent) > 0 {
						// Convert string to bytes safely - only include ASCII characters
						hexPart := []byte{}
						for _, ch := range lineContent {
							if ch < 128 && ch >= 32 {
								hexPart = append(hexPart, byte(ch))
							}
						}
						
						// Only process if we have valid hex bytes
						if len(hexPart) > 0 {
							hexValues = formatDynamicHexBytes(hexPart, maxHexColWidth)
						} else {
							// Empty but properly formatted padding if no valid bytes
							hexValues = strings.Repeat(" ", maxHexColWidth)
						}
					}
				}
				
				// Sanitize the line to prevent display issues
				cleanLine := sanitizeString(line)

				// Format the row
				sb.WriteString(fmt.Sprintf("0x%08X | %-*s | %s\n", 
					obj.startOffset + i, 
					maxHexColWidth,
					hexValues,
					cleanLine))
				rowsRendered++

				// If we've shown the last line, move to the next byte after this JSON object
				if i == len(jsonLines)-1 {
					currentPos = obj.endOffset + 1
				}
			}
		} else {
			// Not the start of a JSON object, check if it's part of one
			if jsonCovered[currentPos] {
				// This position is covered by a JSON object but not the start
				// Skip to the next position that's not part of this JSON object
				foundNextPos := false
				for i := currentPos + 1; i < len(m.data); i++ {
					if !jsonCovered[i] {
						currentPos = i
						foundNextPos = true
						break
					}
				}

				// If we didn't find a non-JSON position, we're done
				if !foundNextPos {
					break
				}
			} else {
				// Not part of a JSON object, render as hex and ASCII
				// Determine how far we can go before hitting a JSON object
				endPos := currentPos + hexBytesPerRow - 1
				for i := currentPos; i <= endPos && i < len(m.data); i++ {
					if jsonCovered[i] {
						endPos = i - 1
						break
					}
				}

				// Make sure we don't go beyond the data
				endPos = min(endPos, len(m.data)-1)

				// Get the bytes for this row
				rowBytes := m.data[currentPos : endPos+1]

				// Create the hex representation
				hexPart := formatDynamicHexBytes(rowBytes, maxHexColWidth)

				// Create the ASCII representation
				asciiPart := formatASCIIBytes(rowBytes)

				// Render this line
				sb.WriteString(fmt.Sprintf("0x%08X | %-*s | %s\n",
					currentPos,
					maxHexColWidth,
					hexPart,
					asciiPart))
				rowsRendered++
				currentPos = endPos + 1
			}
		}
	}

	// Footer
	sb.WriteString(
		fmt.Sprintf(
			"\nFound %d JSON objects. Use arrow keys to navigate, 'l' to switch layout, 'q' to quit.",
			len(m.jsonObjects),
		),
	)

	return sb.String()
}

// formatDynamicHexBytes formats bytes with a specified column width
func formatDynamicHexBytes(data []byte, colWidth int) string {
	var sb strings.Builder
	
	// Calculate how many bytes can fit in the column
	// Each byte takes 3 characters (2 for hex, 1 for space)
	bytesInCol := colWidth / 3
	
	// Handle nil or empty data
	if len(data) == 0 || data == nil {
		return strings.Repeat(" ", colWidth)
	}
	
	// Show as many bytes as will fit in column width
	for i := 0; i < min(len(data), bytesInCol); i++ {
		sb.WriteString(fmt.Sprintf("%02X ", data[i]))
	}
	
	// Calculate remaining space for padding
	usedSpace := min(len(data), bytesInCol) * 3
	if usedSpace > colWidth {
		usedSpace = colWidth
	}
	
	spacesNeeded := colWidth - usedSpace
	if spacesNeeded > 0 {
		sb.WriteString(strings.Repeat(" ", spacesNeeded))
	}
	
	// Ensure proper length
	result := sb.String()
	if len(result) > colWidth {
		return result[:colWidth]
	}
	
	return result
}

// formatSpecificHexBytes formats the exact bytes given without padding to a fixed width
func formatSpecificHexBytes(data []byte) string {
	var sb strings.Builder
	
	// Pad to at least 16 bytes (48 characters including spaces)
	for i := 0; i < min(len(data), 16); i++ {
		sb.WriteString(fmt.Sprintf("%02X ", data[i]))
	}
	
	// Add padding spaces if we have fewer than 16 bytes
	for i := len(data); i < 16; i++ {
		sb.WriteString("   ")
	}
	
	return strings.TrimRight(sb.String(), " ")
}

// formatHexBytes formats a slice of bytes as a hex string, padding to the specified width
func formatHexBytes(data []byte, width int) string {
	var sb strings.Builder

	for i := 0; i < width; i++ {
		if i < len(data) {
			sb.WriteString(fmt.Sprintf("%02X ", data[i]))
		} else {
			sb.WriteString("   ") // Padding for alignment
		}
	}

	return strings.TrimRight(sb.String(), " ")
}

// formatASCIIBytes formats a slice of bytes as ASCII, replacing non-printable chars with periods
func formatASCIIBytes(data []byte) string {
	var sb strings.Builder

	for _, b := range data {
		if b >= 32 && b <= 126 {
			sb.WriteRune(rune(b))
		} else {
			sb.WriteRune('.')
		}
	}

	return sb.String()
}

// containsColumn checks if a column type is in the layout
func containsColumn(columns []ColumnType, column ColumnType) bool {
	for _, c := range columns {
		if c == column {
			return true
		}
	}
	return false
}

// min returns the smaller of x or y
func min(x, y int) int {
	if x < y {
		return x
	}
	return y
}

// max returns the larger of x or y
func max(x, y int) int {
	if x > y {
		return x
	}
	return y
}

// bytesMsg is a custom message type for passing byte data
type bytesMsg []byte

// layoutMsg is a custom message type for changing layouts
type layoutMsg int

var globalProgram *tea.Program

// ShowBytes displays the given bytes in the TUI
func ShowBytes(data []byte) {
	if globalProgram != nil {
		globalProgram.Send(bytesMsg(data))
	}
}

// SetLayout sets the current layout by index
func SetLayout(layoutIndex int) {
	if globalProgram != nil && layoutIndex >= 0 && layoutIndex < len(PredefinedLayouts) {
		globalProgram.Send(layoutMsg(layoutIndex))
	}
}

// findJSONObjects scans a byte slice for valid JSON objects/arrays
func findJSONObjects(data []byte) []jsonObject {
	var objects []jsonObject

	// Define JSON start characters
	jsonStartChars := map[byte]byte{
		'{': '}', // object start -> expected end
		'[': ']', // array start -> expected end
	}

	for i := 0; i < len(data); i++ {
		// Check for potential JSON start
		endChar, isStart := jsonStartChars[data[i]]
		if !isStart {
			continue
		}

		// Check if the potential JSON object is likely valid
		if i+1 >= len(data) {
			continue
		}

		// Check if the next character suggests a valid JSON structure
		nextChar := data[i+1]
		if nextChar != '"' && nextChar != '{' && nextChar != '[' && 
		   !(nextChar >= '0' && nextChar <= '9') {
			// Skip if not promising
			continue
		}

		// Found a potential JSON start
		startOffset := i
		nestLevel := 1

		// Scan for matching end character
		validJSON := false
		for j := i + 1; j < len(data); j++ {
			if data[j] == data[i] {
				// Found nested start of same type
				nestLevel++
			} else if data[j] == endChar {
				// Found an end character
				nestLevel--

				// If all brackets match, we might have valid JSON
				if nestLevel == 0 {
					jsonData := data[startOffset : j+1]

					// Try to parse as JSON
					var parsed interface{}
					if err := json.Unmarshal(jsonData, &parsed); err == nil {
						// Valid JSON found
						objects = append(objects, jsonObject{
							startOffset: startOffset,
							endOffset:   j,
							data:        jsonData,
							parsed:      parsed,
						})
						validJSON = true
					} else if len(jsonData) > 10 {
						// If parsing failed but structure seems valid, 
						// still consider it as a JSON object
						objects = append(objects, jsonObject{
							startOffset: startOffset,
							endOffset:   j,
							data:        jsonData,
							parsed:      nil,
						})
						validJSON = true
					}

					// Move outer loop forward
					i = j
					break
				}
			}
		}

		if !validJSON {
			continue
		}
	}

	return objects
}

// StartTUI initializes and starts the terminal UI
func StartTUI() {
	model := initialModel()
	p := tea.NewProgram(model, tea.WithAltScreen())
	globalProgram = p

	go func() {
		if _, err := p.Run(); err != nil {
			fmt.Printf("Error running program: %v", err)
			os.Exit(1)
		}
	}()
}