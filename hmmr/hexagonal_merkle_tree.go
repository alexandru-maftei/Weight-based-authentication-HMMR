// honeycomb_merkle_tree.go
package main

import (
	"crypto/rand"
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/zeebo/blake3"
)

// ---------------------------
// Global Hash Function Setup (Blake3)
// ---------------------------
func hashFunc(data []byte) []byte {
	sum := blake3.Sum256(data)
	return sum[:] // returns a 32-byte slice
}

// ---------------------------
// Configuration
// ---------------------------
const (
	LEAF_DATA_SIZE = 256 // Each leaf uses 256 bytes of random data.
	GROUP_SIZE     = 4   // Honeycomb Merkle tree groups 6 nodes at a time.
	NUM_PROOF_RUNS = 1000
)

// ---------------------------
// Honeycomb Merkle Tree Structure
// ---------------------------
type MerkleTree struct {
	Leaves [][]byte   // Level 0: the leaf hashes.
	Levels [][][]byte // Levels[0] is Leaves; last level contains the root.
}

// NewMerkleTree creates a new empty tree.
func NewMerkleTree() *MerkleTree {
	return &MerkleTree{
		Leaves: make([][]byte, 0),
		Levels: make([][][]byte, 0),
	}
}

// AddLeaf computes the hash of the data and appends it to the leaf slice.
func (mt *MerkleTree) AddLeaf(data []byte) {
	leafHash := hashFunc(data)
	mt.Leaves = append(mt.Leaves, leafHash)
}

// BuildTree builds the entire tree from the leaves using 6-ary grouping.
func (mt *MerkleTree) BuildTree() {
	if len(mt.Leaves) == 0 {
		return
	}
	// Level 0 is the leaves.
	mt.Levels = append(mt.Levels, mt.Leaves)
	currentLevel := mt.Leaves
	for len(currentLevel) > 1 {
		nextLevel := make([][]byte, 0)
		for i := 0; i < len(currentLevel); i += GROUP_SIZE {
			group := currentLevel[i:min(i+GROUP_SIZE, len(currentLevel))]
			// If group is incomplete, duplicate the last element until the group size equals GROUP_SIZE.
			for len(group) < GROUP_SIZE {
				group = append(group, group[len(group)-1])
			}
			parentHash := hashFunc(appendBytes(group))
			nextLevel = append(nextLevel, parentHash)
		}
		mt.Levels = append(mt.Levels, nextLevel)
		currentLevel = nextLevel
	}
}

// GetRoot returns the root of the tree.
func (mt *MerkleTree) GetRoot() []byte {
	if len(mt.Levels) == 0 {
		return make([]byte, 32)
	}
	lastLevel := mt.Levels[len(mt.Levels)-1]
	if len(lastLevel) == 0 {
		return make([]byte, 32)
	}
	return lastLevel[0]
}

// GenerateProof computes a Merkle proof for the leaf at index leafIndex.
// Returns an object: { proof: slice of sibling hashes, duration: time.Duration }
func (mt *MerkleTree) GenerateProof(leafIndex int) ([][]byte, time.Duration) {
	start := time.Now()
	if leafIndex < 0 || leafIndex >= len(mt.Leaves) {
		return nil, time.Since(start)
	}
	proof := make([][]byte, 0)
	index := leafIndex
	// Traverse from level 0 up to the level before the root.
	for level := 0; level < len(mt.Levels)-1; level++ {
		nodes := mt.Levels[level]
		groupIndex := index / GROUP_SIZE
		groupStart := groupIndex * GROUP_SIZE
		groupEnd := groupStart + GROUP_SIZE
		if groupEnd > len(nodes) {
			groupEnd = len(nodes)
		}
		// Append sibling hashes (all in group except the node at index).
		for i := groupStart; i < groupEnd; i++ {
			if i == index {
				continue
			}
			proof = append(proof, nodes[i])
		}
		index = index / GROUP_SIZE
	}
	return proof, time.Since(start)
}

// Helper function: appendBytes concatenates a slice of byte slices.
func appendBytes(slices [][]byte) []byte {
	var combined []byte
	for _, b := range slices {
		combined = append(combined, b...)
	}
	return combined
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// generateRandomData creates a random byte slice of the given size.
func generateRandomData(numBytes int) []byte {
	data := make([]byte, numBytes)
	_, err := rand.Read(data)
	if err != nil {
		log.Fatalf("Error generating random data: %v", err)
	}
	return data
}

// saveSummaryMetricsToCSV writes the provided rows into a CSV file.
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

// ---------------------------
// Main Execution for Testing
// ---------------------------
func main() {
	// Expected usage: go run honeycomb_merkle_tree.go test <num_leaves1> <num_leaves2> ...
	if len(os.Args) < 3 {
		fmt.Println("Usage: go run honeycomb_merkle_tree.go test <num_leaves1> <num_leaves2> ...")
		os.Exit(1)
	}
	mode := strings.ToLower(os.Args[1])
	if mode != "test" {
		log.Fatalf("Only test mode is implemented. Use 'test' as the first argument.")
	}
	leafArgs := os.Args[2:]
	results := [][]string{
		{"NumLeaves", "BuildTimeMs", "MemoryUsed", "ProofTimeMs", "ProofSizeBytes", "Depth"},
	}
	fmt.Println("NumLeaves\tBuildTimeMs\tMemoryUsed\tProofTimeMs\tProofSizeBytes\tDepth")

	for _, arg := range leafArgs {
		numLeaves, err := strconv.Atoi(arg)
		if err != nil {
			log.Fatalf("Error parsing number of leaves '%s': %v", arg, err)
		}
		fmt.Printf("Running test for %d leaves using blake3...\n", numLeaves)

		tree := NewMerkleTree()
		startBuild := time.Now()
		for i := 0; i < numLeaves; i++ {
			data := generateRandomData(LEAF_DATA_SIZE)
			tree.AddLeaf(data)
		}
		tree.BuildTree()
		buildTime := time.Since(startBuild)

		runtime.GC()
		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)
		memoryUsed := memStats.Alloc

		// Measure proof generation multiple times and average the duration.
		proofLeafIndex := numLeaves / 2
		var totalProofTime time.Duration
		var lastProof [][]byte
		for i := 0; i < NUM_PROOF_RUNS; i++ {
			proof, proofTime := tree.GenerateProof(proofLeafIndex)
			totalProofTime += proofTime
			lastProof = proof
		}
		avgProofTime := totalProofTime / NUM_PROOF_RUNS
		proofSize := len(lastProof) * 32 // Each hash is 32 bytes.

		buildTimeMs := float64(buildTime.Nanoseconds()) / 1e6
		proofTimeMs := float64(avgProofTime.Nanoseconds())
		depth := len(tree.Levels) - 1

		row := []string{
			strconv.Itoa(numLeaves),
			fmt.Sprintf("%.3f", buildTimeMs),
			strconv.FormatUint(memoryUsed, 10),
			fmt.Sprintf("%.6f", proofTimeMs),
			strconv.Itoa(proofSize),
			strconv.Itoa(depth),
		}
		results = append(results, row)
		fmt.Println(strings.Join(row, "\t"))
	}

	csvFilename := "honeycomb_merkle_tree_metrics.csv"
	err := saveSummaryMetricsToCSV(csvFilename, results)
	if err != nil {
		log.Fatalf("Error saving summary metrics to CSV: %v", err)
	}
	fmt.Printf("Summary metrics saved to %s\n", csvFilename)
}
