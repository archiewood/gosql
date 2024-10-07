package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestQueries(t *testing.T) {
	testDir := "test"
	files, err := os.ReadDir(testDir)
	if err != nil {
		t.Fatalf("Failed to read tests directory: %v", err)
	}

	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".test") {
			println(file.Name())
			t.Run(file.Name(), func(t *testing.T) {
				// Read test case file
				content, err := os.ReadFile(filepath.Join(testDir, file.Name()))
				if err != nil {
					t.Fatalf("Failed to read test case file: %v", err)
				}

				// Split content into query and expected output
				parts := strings.Split(string(content), "\n---\n")
				if len(parts) != 2 {
					t.Fatalf("Invalid test case file format")
				}
				query := strings.TrimSpace(parts[0])
				expectedOutput := strings.TrimSpace(parts[1])

				// Capture stdout
				oldStdout := os.Stdout
				r, w, _ := os.Pipe()
				os.Stdout = w

				// Run the query
				os.Args = []string{"cmd", query}
				main()

				// Restore stdout
				w.Close()
				os.Stdout = oldStdout

				var buf bytes.Buffer
				buf.ReadFrom(r)
				output := strings.TrimSpace(buf.String())

				if output != expectedOutput {
					t.Errorf("Expected output:\n%s\n\nGot:\n%s", expectedOutput, output)
				}
			})
		}
	}
}
