package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
)

func main() {
	inputFile := flag.String("input", "", "Path to the update skill categories JSON file")
	targetFile := flag.String("target", "data/skill_categories.json", "Path to the original skill categories JSON file")
	flag.Parse()

	if *inputFile == "" {
		log.Fatalf("Usage: go run cmd/merge_categories/main.go -input <path_to_update.json>")
	}

	// Read update file
	updateData, err := os.ReadFile(*inputFile)
	if err != nil {
		log.Fatalf("Error reading input file: %v", err)
	}

	var updateCategories map[string][]string
	if err := json.Unmarshal(updateData, &updateCategories); err != nil {
		log.Fatalf("Error parsing input JSON: %v", err)
	}

	// Read original file
	targetData, err := os.ReadFile(*targetFile)
	if err != nil {
		log.Fatalf("Error reading target file: %v", err)
	}

	var targetCategories map[string][]string
	if err := json.Unmarshal(targetData, &targetCategories); err != nil {
		log.Fatalf("Error parsing target JSON: %v", err)
	}

	// Merge
	addedSkills := 0
	addedCategories := 0

	for category, newSkills := range updateCategories {
		existingSkills, exists := targetCategories[category]
		if !exists {
			targetCategories[category] = newSkills
			addedCategories++
			addedSkills += len(newSkills)
			continue
		}

		// Create a map of existing skills for O(1) lookup
		existingMap := make(map[string]bool)
		for _, s := range existingSkills {
			existingMap[s] = true
		}

		// Append new skills if they don't exist
		for _, s := range newSkills {
			if !existingMap[s] {
				targetCategories[category] = append(targetCategories[category], s)
				addedSkills++
			}
		}
	}

	// Write back to target
	outData, err := json.MarshalIndent(targetCategories, "", "    ")
	if err != nil {
		log.Fatalf("Error marshaling output: %v", err)
	}

	if err := os.WriteFile(*targetFile, outData, 0644); err != nil {
		log.Fatalf("Error writing to target file: %v", err)
	}

	fmt.Printf("Successfully merged. Added %d new categories and %d new skills overall.\n", addedCategories, addedSkills)
}
