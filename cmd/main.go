package main

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/fipso/prettybuffers"
)

func main() {
	// Start the TUI
	prettybuffers.StartTUI()

	// Generate some sample data with various byte values
	data := generateSampleData(4096)

	// Display the data
	prettybuffers.ShowBytes(data)

	// Keep the program running
	fmt.Println("Press Ctrl+C to exit")
	select {}
}

// generateSampleData creates a byte slice with various patterns for demonstration
func generateSampleData(size int) []byte {
	rand.Seed(time.Now().UnixNano())

	data := make([]byte, size)

	// Fill with different patterns
	for i := 0; i < size; i++ {
		switch {
		case i < 256:
			// First 256 bytes are sequential values 0-255
			data[i] = byte(i)
		case i < 512:
			// Next 256 bytes are ASCII printable characters
			data[i] = byte(32 + (i % 95))
		default:
			// Rest is random data
			data[i] = byte(rand.Intn(256))
		}
	}

	// Insert some sample JSON objects at different positions
	sampleJSONs := []string{
		`{"id":1,"name":"Test Object","active":true,"values":[1,2,3]}`,
		`{"id":2,"type":"user","metadata":{"role":"admin","created_at":"2024-03-22"}}`,
		`[1,2,3,{"test":"nested"}]`,
		`{"nested":{"objects":{"are":{"fun":true}}}}`,
		`{"error":null,"result":{"status":"ok","count":42}}`,
	}

	// Insert JSON objects at various positions
	jsonPos := []int{600, 1024, 1800, 2500, 3200}
	for i, pos := range jsonPos {
		if pos < size && i < len(sampleJSONs) {
			// Make sure we have enough space
			jsonBytes := []byte(sampleJSONs[i])
			if pos+len(jsonBytes) < size {
				copy(data[pos:], jsonBytes)
			}
		}
	}

	return data
}
