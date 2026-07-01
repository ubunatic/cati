//go:build ignore

package main

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type DocPage struct {
	Filename string
	Title    string
	Parent   string
	Weight   int
}

func main() {
	syncExample()

	docsDir := filepath.Join(".", "docs")
	summaryPath := filepath.Join(docsDir, "SUMMARY.md")

	fmt.Printf("Scanning directory: %s for markdown files...\n", docsDir)
	var pages []DocPage

	err := filepath.WalkDir(docsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		name := d.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".md") {
			return nil
		}
		// Skip special files
		if name == "SUMMARY.md" || name == "README.md" {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		title, parent, weight, hasFM := parseFrontMatter(string(content))
		if !hasFM {
			// Fallback: extract first H1 as title, default weight
			title = extractH1(string(content))
			if title == "" {
				title = strings.TrimSuffix(name, ".md")
			}
			weight = 9999
		}

		pages = append(pages, DocPage{
			Filename: name,
			Title:    title,
			Parent:   parent,
			Weight:   weight,
		})
		return nil
	})

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error walking docs directory: %v\n", err)
		os.Exit(1)
	}

	// Group and Sort
	// Sort order: weight ascending, then title alphabetically
	sort.Slice(pages, func(i, j int) bool {
		if pages[i].Weight != pages[j].Weight {
			return pages[i].Weight < pages[j].Weight
		}
		return pages[i].Title < pages[j].Title
	})

	// Build nested hierarchy: parent filename -> list of child pages
	childrenMap := make(map[string][]DocPage)
	var rootPages []DocPage

	for _, p := range pages {
		if p.Parent != "" {
			childrenMap[p.Parent] = append(childrenMap[p.Parent], p)
		} else {
			rootPages = append(rootPages, p)
		}
	}

	// Generate TOC markdown
	var tocBuilder strings.Builder
	for _, p := range rootPages {
		writeTOCEntry(&tocBuilder, p, childrenMap, 0)
	}
	tocContent := tocBuilder.String()

	// Read SUMMARY.md and inject/replace
	fmt.Printf("Updating SUMMARY.md: %s...\n", summaryPath)
	summaryContent, err := os.ReadFile(summaryPath)
	if err != nil {
		// If it doesn't exist, create a default one
		summaryContent = []byte("# Summary\n\n[Introduction](README.md)\n\n<!-- TOC_START -->\n<!-- TOC_END -->\n")
	}

	startMarker := []byte("<!-- TOC_START -->")
	endMarker := []byte("<!-- TOC_END -->")

	startIndex := bytes.Index(summaryContent, startMarker)
	endIndex := bytes.Index(summaryContent, endMarker)

	var newSummary bytes.Buffer
	if startIndex == -1 || endIndex == -1 || startIndex >= endIndex {
		// Markers not found, append to the end
		newSummary.Write(summaryContent)
		if !bytes.HasSuffix(summaryContent, []byte("\n")) {
			newSummary.WriteString("\n")
		}
		newSummary.WriteString("\n<!-- TOC_START -->\n")
		newSummary.WriteString(tocContent)
		newSummary.WriteString("<!-- TOC_END -->\n")
	} else {
		// In-place replacement
		newSummary.Write(summaryContent[:startIndex+len(startMarker)])
		newSummary.WriteString("\n")
		newSummary.WriteString(tocContent)
		newSummary.Write(summaryContent[endIndex:])
	}

	err = os.WriteFile(summaryPath, newSummary.Bytes(), 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error writing SUMMARY.md: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Success! SUMMARY.md updated successfully.")
}

func parseFrontMatter(content string) (title string, parent string, weight int, hasFM bool) {
	trimmed := strings.TrimSpace(content)
	if !strings.HasPrefix(trimmed, "---") {
		return "", "", 0, false
	}
	lines := strings.Split(content, "\n")
	if len(lines) < 2 {
		return "", "", 0, false
	}
	fmEnd := -1
	// Start checking from line 1 since line 0 is the start "---" (or has it)
	// We'll search for the next "---" line
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			fmEnd = i
			break
		}
	}
	if fmEnd == -1 {
		return "", "", 0, false
	}

	weight = 9999
	for i := 0; i < fmEnd; i++ {
		line := strings.TrimSpace(lines[i])
		if line == "---" || line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(parts[0]))
		val := strings.Trim(strings.TrimSpace(parts[1]), `"'`)

		switch key {
		case "title":
			title = val
		case "parent":
			parent = val
		case "weight":
			w, err := strconv.Atoi(val)
			if err == nil {
				weight = w
			}
		}
	}
	return title, parent, weight, true
}

func extractH1(content string) string {
	re := regexp.MustCompile(`(?m)^#\s+(.+)$`)
	matches := re.FindStringSubmatch(content)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	return ""
}

func writeTOCEntry(sb *strings.Builder, p DocPage, childrenMap map[string][]DocPage, depth int) {
	indent := strings.Repeat("    ", depth)
	sb.WriteString(fmt.Sprintf("%s- [%s](%s)\n", indent, p.Title, p.Filename))
	
	// Print children
	if children, exists := childrenMap[p.Filename]; exists {
		for _, child := range children {
			writeTOCEntry(sb, child, childrenMap, depth+1)
		}
	}
}

func syncExample() {
	examplePath := filepath.Join("examples", "library_example.go")
	exampleBytes, err := os.ReadFile(examplePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading example file %s: %v\n", examplePath, err)
		os.Exit(1)
	}
	exampleStr := string(exampleBytes)
	
	// Strip build tag if present
	buildTag := "//go:build ignore"
	if strings.HasPrefix(exampleStr, buildTag) {
		exampleStr = strings.TrimPrefix(exampleStr, buildTag)
		exampleStr = strings.TrimSpace(exampleStr)
	}

	// Code block wrapper for markdown
	mdCodeBlock := "```go\n" + exampleStr + "\n```"

	// HTML code block wrapper for website
	htmlCodeBlock := "<pre class=\"code\"><code>" + escapeHTML(exampleStr) + "</code></pre>"

	// Sync docs/GoLibrary.md
	goLibPath := filepath.Join("docs", "GoLibrary.md")
	if err := replaceBetweenMarkers(goLibPath, "<!-- GO_EXAMPLE_START -->", "<!-- GO_EXAMPLE_END -->", mdCodeBlock); err != nil {
		fmt.Fprintf(os.Stderr, "Error updating %s: %v\n", goLibPath, err)
		os.Exit(1)
	}

	// Sync website/index.html
	webPath := filepath.Join("website", "index.html")
	if err := replaceBetweenMarkers(webPath, "<!-- GO_EXAMPLE_START -->", "<!-- GO_EXAMPLE_END -->", htmlCodeBlock); err != nil {
		fmt.Fprintf(os.Stderr, "Error updating %s: %v\n", webPath, err)
		os.Exit(1)
	}
	fmt.Println("Synced Go library example to docs/GoLibrary.md and website/index.html.")
}

func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&#39;")
	return s
}

func replaceBetweenMarkers(filePath, startMarker, endMarker, content string) error {
	fileBytes, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	startBytes := []byte(startMarker)
	endBytes := []byte(endMarker)

	startIndex := bytes.Index(fileBytes, startBytes)
	endIndex := bytes.Index(fileBytes, endBytes)

	if startIndex == -1 || endIndex == -1 || startIndex >= endIndex {
		return fmt.Errorf("markers %q and %q not found in %s", startMarker, endMarker, filePath)
	}

	var newContent bytes.Buffer
	newContent.Write(fileBytes[:startIndex+len(startBytes)])
	newContent.WriteString("\n")
	newContent.WriteString(content)
	newContent.WriteString("\n")
	newContent.Write(fileBytes[endIndex:])

	return os.WriteFile(filePath, newContent.Bytes(), 0644)
}
