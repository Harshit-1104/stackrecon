package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	// 1. Load alias_map.json
	aliasMapData, err := os.ReadFile("data/alias_map.json")
	if err != nil {
		fmt.Printf("Error reading alias_map.json: %v\n", err)
		return
	}
	var aliasMap map[string]string
	if err := json.Unmarshal(aliasMapData, &aliasMap); err != nil {
		fmt.Printf("Error unmarshaling alias_map.json: %v\n", err)
		return
	}

	// 2. Load skill_blocklist_resolved.json
	blocklistData, err := os.ReadFile("data/skill_blocklist_resolved.json")
	if err != nil {
		fmt.Printf("Error reading skill_blocklist_resolved.json: %v\n", err)
		return
	}
	var blocklist []string
	if err := json.Unmarshal(blocklistData, &blocklist); err != nil {
		fmt.Printf("Error unmarshaling skill_blocklist_resolved.json: %v\n", err)
		return
	}

	blockedSkills := make(map[string]bool)
	for _, skill := range blocklist {
		blockedSkills[skill] = true
	}

	// 3. Load skill_categories.json
	categoriesData, err := os.ReadFile("data/skill_categories.json")
	if err != nil {
		fmt.Printf("Error reading skill_categories.json: %v\n", err)
		return
	}
	var categories map[string][]string
	if err := json.Unmarshal(categoriesData, &categories); err != nil {
		fmt.Printf("Error unmarshaling skill_categories.json: %v\n", err)
		return
	}

	categorizedSkills := make(map[string]bool)
	for _, skills := range categories {
		for _, skill := range skills {
			categorizedSkills[skill] = true
		}
	}

	// 4. Parse all extracted and reextracted data files
	var files []string
	if extractedFiles, err := filepath.Glob("data/extracted/tech/*.json"); err == nil {
		files = append(files, extractedFiles...)
	} else {
		fmt.Printf("Error globbing extracted files: %v\n", err)
	}
	
	if reextractedFiles, err := filepath.Glob("data/reextracted/*/*.json"); err == nil {
		files = append(files, reextractedFiles...)
	} else {
		fmt.Printf("Error globbing reextracted files: %v\n", err)
	}

	uniqueCanonicalSkills := make(map[string]bool)
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}

		type ExtractedSkill struct {
			Name      string `json:"name"`
			SkillName string `json:"skill_name"`
		}
		type JD struct {
			Skills []ExtractedSkill `json:"skills"`
		}

		var jobs []JD
		if err := json.Unmarshal(data, &jobs); err != nil {
			var job JD
			if err2 := json.Unmarshal(data, &job); err2 == nil {
				jobs = []JD{job}
			} else {
				continue
			}
		}

		for _, job := range jobs {
			for _, skill := range job.Skills {
				name := strings.TrimSpace(skill.Name)
				if name == "" {
					name = strings.TrimSpace(skill.SkillName)
				}
				if name == "" {
					continue
				}
				
				canonical := aliasMap[name]
				if canonical == "" {
					canonical = name
				}

				uniqueCanonicalSkills[canonical] = true
			}
		}
	}

	// 5. Find differences
	var uncategorized []string
	for skill := range uniqueCanonicalSkills {
		if !blockedSkills[skill] && !categorizedSkills[skill] {
			uncategorized = append(uncategorized, skill)
		}
	}

	sort.Strings(uncategorized)

	// 5. Save the result
	outData, err := json.MarshalIndent(uncategorized, "", "  ")
	if err != nil {
		fmt.Printf("Error marshaling output: %v\n", err)
		return
	}

	err = os.WriteFile("data/uncategorized_skills.json", outData, 0644)
	if err != nil {
		fmt.Printf("Error writing output file: %v\n", err)
		return
	}

	fmt.Printf("Found %d canonical skills that are uncategorized and not blocklisted.\n", len(uncategorized))
	fmt.Println("Saved to data/uncategorized_skills.json")
}
