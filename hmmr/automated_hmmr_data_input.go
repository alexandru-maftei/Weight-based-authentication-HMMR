// hmmr_offline.go
package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"runtime"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/zeebo/blake3"
)

// ---------------------------
// Global Hash Function Setup (Blake3 Only)
// ---------------------------
var (
	// Only using Blake3 for this version.
	hashFunc = func(data []byte) []byte {
		sum := blake3.Sum256(data)
		return sum[:]
	}
)

// ---------------------------
// Configuration
// ---------------------------
const (
	LEAF_DATA_SIZE = 256 // Each leaf uses 256 bytes of random data (if using random data)
)

// ---------------------------
// HMMR Off-Chain Structure
// ---------------------------
type Node struct {
	Position int
	Hash     []byte
	Height   int
}

type HMMR struct {
	Nodes       []Node
	LeafIndices []int // positions (1-indexed) of leaves in Nodes
	Peaks       []Node
	NodeCount   int
}

func NewHMMR() *HMMR {
	return &HMMR{
		Nodes:       make([]Node, 0),
		LeafIndices: make([]int, 0),
		Peaks:       make([]Node, 0),
		NodeCount:   0,
	}
}

// hashLeaf computes the digest of data using Blake3.
func (h *HMMR) hashLeaf(data []byte) []byte {
	return hashFunc(data)
}

// hashGroup concatenates a slice of hashes and computes the digest.
func (h *HMMR) hashGroup(hashes [][]byte) []byte {
	combined := []byte{}
	for _, hash := range hashes {
		combined = append(combined, hash...)
	}
	return hashFunc(combined)
}

// AddLeaf adds a new leaf node and performs 6‑way merging on the peaks.
func (h *HMMR) AddLeaf(data []byte) []byte {
	leafHash := h.hashLeaf(data)
	h.NodeCount++
	position := h.NodeCount
	leafNode := Node{Position: position, Hash: leafHash, Height: 0}
	h.Nodes = append(h.Nodes, leafNode)
	h.LeafIndices = append(h.LeafIndices, position)
	h.Peaks = append(h.Peaks, leafNode)

	// Check if the last group of peaks has 6 nodes with the same height.
	for {
		if len(h.Peaks) < 6 {
			break
		}
		count := 0
		lastHeight := h.Peaks[len(h.Peaks)-1].Height
		for i := len(h.Peaks) - 1; i >= 0; i-- {
			if h.Peaks[i].Height == lastHeight {
				count++
			} else {
				break
			}
		}
		if count < 6 {
			break
		}
		// Remove the last 6 peaks and merge them.
		group := h.Peaks[len(h.Peaks)-6:]
		h.Peaks = h.Peaks[:len(h.Peaks)-6]
		hashes := make([][]byte, 6)
		for i, node := range group {
			hashes[i] = node.Hash
		}
		mergedHash := h.hashGroup(hashes)
		h.NodeCount++
		mergedNode := Node{Position: h.NodeCount, Hash: mergedHash, Height: lastHeight + 1}
		h.Nodes = append(h.Nodes, mergedNode)
		// Append the merged node back to the peaks.
		h.Peaks = append(h.Peaks, mergedNode)
	}
	return h.GetRoot()
}

// GetRoot computes the H‑MMR root by concatenating all peak hashes and hashing the result.
func (h *HMMR) GetRoot() []byte {
	if len(h.Peaks) == 0 {
		return make([]byte, 32)
	}
	combined := []byte{}
	for _, peak := range h.Peaks {
		combined = append(combined, peak.Hash...)
	}
	return hashFunc(combined)
}

// Depth returns the maximum height among the current peaks.
func (h *HMMR) Depth() int {
	max := 0
	for _, peak := range h.Peaks {
		if peak.Height > max {
			max = peak.Height
		}
	}
	return max
}

// GenerateProof computes a Merkle-style proof for the leaf at index leafIndex using 6‑way grouping.
func (h *HMMR) GenerateProof(leafIndex int) ([][]byte, time.Duration) {
	start := time.Now()
	leaves := make([][]byte, len(h.LeafIndices))
	for i, pos := range h.LeafIndices {
		leaves[i] = h.Nodes[pos-1].Hash
	}
	if leafIndex < 0 || leafIndex >= len(leaves) {
		return nil, time.Since(start)
	}
	proof := [][]byte{}
	level := leaves
	index := leafIndex
	groupSize := 6

	// Traverse up the tree level by level.
	for len(level) > 1 {
		groupIndex := index / groupSize
		groupStart := groupIndex * groupSize
		groupEnd := groupStart + groupSize
		if groupEnd > len(level) {
			groupEnd = len(level)
		}
		// Add sibling hashes (all items in the group except the one at "index").
		for i := groupStart; i < groupEnd; i++ {
			if i == index {
				continue
			}
			proof = append(proof, level[i])
		}
		// Build the next level by grouping nodes in groups of 6.
		nextLevel := [][]byte{}
		for i := 0; i < len(level); i += groupSize {
			end := i + groupSize
			if end > len(level) {
				end = len(level)
			}
			group := level[i:end]
			// If the group is incomplete, duplicate the last element until it has 6 members.
			if len(group) < groupSize {
				last := group[len(group)-1]
				for len(group) < groupSize {
					group = append(group, last)
				}
			}
			parentHash := hashFunc(appendBytes(group))
			nextLevel = append(nextLevel, parentHash)
		}
		index = index / groupSize
		level = nextLevel
	}
	return proof, time.Since(start)
}

// Helper function to concatenate a slice of byte slices.
func appendBytes(slices [][]byte) []byte {
	combined := []byte{}
	for _, b := range slices {
		combined = append(combined, b...)
	}
	return combined
}

// saveSummaryMetricsToCSV writes the summary metrics (multiple rows) into a CSV file.
func saveSummaryMetricsToCSV(filename string, rows [][]string) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("Error creating CSV file: %v", err)
	}
	defer file.Close()
	writer := csv.NewWriter(file)
	defer writer.Flush()

	for _, record := range rows {
		if err := writer.Write(record); err != nil {
			return fmt.Errorf("Error writing record to CSV: %v", err)
		}
	}
	return nil
}

// saveTreeToFile saves the full HMMR tree structure into a file with a .tree extension.
func saveTreeToFile(filename string, tree *HMMR) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("Error creating tree file: %v", err)
	}
	defer file.Close()

	// Write header.
	_, err = fmt.Fprintln(file, "Position\tHeight\tHash")
	if err != nil {
		return fmt.Errorf("Error writing header: %v", err)
	}
	// Write each node.
	for _, node := range tree.Nodes {
		_, err = fmt.Fprintf(file, "%d\t%d\t%x\n", node.Position, node.Height, node.Hash)
		if err != nil {
			return fmt.Errorf("Error writing node data: %v", err)
		}
	}
	return nil
}

// ---------------------------
// Main Execution for HMMR with JSON Data Input
// ---------------------------
func main() {
	// Usage: go run hmmr_offline.go test <input_json_file>
	// The JSON file should contain an array of objects (each representing a patient record).
	args := os.Args[1:]
	if len(args) < 2 {
		fmt.Println("Usage: go run hmmr_offline.go test <input_json_file>")
		os.Exit(1)
	}
	mode := strings.ToLower(args[0])
	if mode != "test" {
		log.Fatalf("Only test mode is implemented. Use 'test' as the first argument.")
	}

	// Read JSON file.
	jsonFilename := args[1]
	jsonData, err := os.ReadFile(jsonFilename)
	if err != nil {
		log.Fatalf("Error reading JSON file: %v", err)
	}

	// Parse the JSON file to get an array of objects.
	var dataItems []map[string]interface{}
	if err := json.Unmarshal(jsonData, &dataItems); err != nil {
		log.Fatalf("Error parsing JSON file: %v", err)
	}
	numLeaves := len(dataItems)
	fmt.Printf("Running test for %d leaves with data from %s...\n", numLeaves, jsonFilename)

	// Create an HMMR structure.
	hmmr := NewHMMR()

	// For each object, marshal it back to a JSON string and use that as the leaf data.
	startBuild := time.Now()
	for _, item := range dataItems {
		itemBytes, err := json.Marshal(item)
		if err != nil {
			log.Fatalf("Error marshaling JSON item: %v", err)
		}
		hmmr.AddLeaf(itemBytes)
	}
	buildLeavesTime := time.Since(startBuild)

	// Force GC and measure memory usage.
	runtime.GC()
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	memoryUsed := memStats.Alloc

	// Measure proof generation time and proof size for a specific leaf (using the middle leaf).
	proofLeafIndex := numLeaves / 2
	proof, proofTime := hmmr.GenerateProof(proofLeafIndex)
	proofSize := len(proof) * 32 // each hash is 32 bytes

	// Convert times to milliseconds.
	buildTimeMs := float64(buildLeavesTime.Nanoseconds()) / 1e6
	proofTimeMs := float64(proofTime.Nanoseconds()) / 1e6

	// Prepare results.
	results := [][]string{
		{"NumLeaves", "BuildTimeMs", "MemoryUsedBytes", "ProofTimeMs", "ProofSizeBytes", "Depth"},
		{strconv.Itoa(numLeaves), fmt.Sprintf("%.3f", buildTimeMs), strconv.FormatUint(memoryUsed, 10), fmt.Sprintf("%.3f", proofTimeMs), strconv.Itoa(proofSize), strconv.Itoa(hmmr.Depth())},
	}

	// Print results in an ASCII table.
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', tabwriter.Debug)
	fmt.Fprintln(tw, "NumLeaves\tBuildTimeMs\tMemoryUsedBytes\tProofTimeMs\tProofSizeBytes\tDepth\t")
	fmt.Fprintf(tw, "%d\t%.3f\t%s\t%.3f\t%d\t%d\t\n", numLeaves, buildTimeMs, strconv.FormatUint(memoryUsed, 10), proofTimeMs, proofSize, hmmr.Depth())
	tw.Flush()

	// Save summary metrics into a CSV file.
	csvFilename := "hmmr_summary_metrics.csv"
	if err := saveSummaryMetricsToCSV(csvFilename, results); err != nil {
		log.Fatalf("Error saving summary metrics to CSV: %v", err)
	} else {
		fmt.Printf("Summary metrics saved to %s\n", csvFilename)
	}

	// Save the full tree into a file with a .tree extension.
	treeFilename := "hmmr_full.tree"
	if err := saveTreeToFile(treeFilename, hmmr); err != nil {
		log.Fatalf("Error saving tree to file: %v", err)
	} else {
		fmt.Printf("Full HMMR tree saved to %s\n", treeFilename)
	}
}
