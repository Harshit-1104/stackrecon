package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/generative-ai-go/genai"
	"github.com/joho/godotenv"
	"golang.org/x/time/rate"
	"google.golang.org/api/option"
)

type JD struct {
	Skills []Skill `json:"skills"`
}

type Skill struct {
	Name             string `json:"name"`
	RequirementLevel string `json:"requirement_level"`
}

func main() {
	inputDir := flag.String("input-dir", "data/extracted/tech", "Directory containing the JSON files to process")
	aliasMapPath := flag.String("alias-map", "data/alias_map.json", "Path to the alias map JSON file")
	seedsPath := flag.String("seeds", "data/canonical_seeds.json", "Path to canonical seeds JSON")
	blocklistPath := flag.String("blocklist", "data/skill_blocklist.json", "Path to skill blocklist JSON")
	resolvedBlocklistPath := flag.String("resolved-blocklist", "data/skill_blocklist_resolved.json", "Path to output resolved blocklist")
	printUnresolved := flag.Bool("print-unresolved", false, "Print all unresolved skills with frequency > 50")
	workers := flag.Int("workers", 10, "Number of concurrent workers")
	flag.Parse()

	_ = godotenv.Load()

	// Step 1: Collect unique skill names
	files, err := filepath.Glob(filepath.Join(*inputDir, "*.json"))
	if err != nil {
		log.Fatalf("Error finding files: %v", err)
	}

	uniqueSkillsMap := make(map[string]bool)
	freqMap := make(map[string]int)
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			log.Printf("Error reading %s: %v", file, err)
			continue
		}

		var jobs []JD
		err = json.Unmarshal(data, &jobs)
		if err != nil {
			var job JD
			if err2 := json.Unmarshal(data, &job); err2 == nil {
				jobs = []JD{job}
			} else {
				log.Printf("Error parsing %s: %v", file, err)
				continue
			}
		}

		for _, job := range jobs {
			for _, skill := range job.Skills {
				name := strings.TrimSpace(skill.Name)
				if name != "" {
					uniqueSkillsMap[name] = true
					freqMap[name]++
				}
			}
		}
	}

	totalUniqueSkills := len(uniqueSkillsMap)
	fmt.Println("=== Pipeline C: Skill Canonicalization ===")
	fmt.Printf("\nRaw unique skill names:     %d\n", totalUniqueSkills)

	// Step 2: Load seeds and existing alias map
	seedMap := make(map[string]string)
	if data, err := os.ReadFile(*seedsPath); err == nil {
		if err := json.Unmarshal(data, &seedMap); err != nil {
			log.Printf("Error parsing %s: %v", *seedsPath, err)
		}
	} else {
		log.Fatalf("Error finding seeds: %v", err)
	}

	aliasMap := make(map[string]string)
	if data, err := os.ReadFile(*aliasMapPath); err == nil {
		if err := json.Unmarshal(data, &aliasMap); err != nil {
			log.Printf("Error parsing %s: %v", *aliasMapPath, err)
		}
	}

	var blocklist []string
	if data, err := os.ReadFile(*blocklistPath); err == nil {
		if err := json.Unmarshal(data, &blocklist); err != nil {
			log.Printf("Error parsing %s: %v", *blocklistPath, err)
		}
	}

	seedValues := make(map[string]bool)
	for _, v := range seedMap {
		seedValues[v] = true
	}
	aliasValues := make(map[string]bool)
	for _, v := range aliasMap {
		aliasValues[v] = true
	}

	// Step 3: Resolve skills (no LLM yet)
	var unresolvedSkills []string
	seedResolved := 0
	aliasResolved := 0
	alreadyCanonical := 0

	for skill := range uniqueSkillsMap {
		if _, exists := seedMap[skill]; exists {
			seedResolved++
		} else if _, exists := aliasMap[skill]; exists {
			aliasResolved++
		} else if seedValues[skill] || aliasValues[skill] {
			alreadyCanonical++
		} else {
			unresolvedSkills = append(unresolvedSkills, skill)
		}
	}

	sort.Strings(unresolvedSkills)
	unresolvedCount := len(unresolvedSkills)

	// Calculate initial unique skills
	initialUniqueSkillsMap := make(map[string]bool)
	for skill := range uniqueSkillsMap {
		if canonical, ok := aliasMap[skill]; ok {
			initialUniqueSkillsMap[canonical] = true
		} else {
			initialUniqueSkillsMap[skill] = true
		}
	}
	for _, b := range blocklist {
		delete(initialUniqueSkillsMap, b)
	}
	initialUniqueSkillsCount := len(initialUniqueSkillsMap)

	fmt.Printf("\nSeed resolved:              %d\n", seedResolved)
	fmt.Printf("Alias resolved (existing):  %d\n", aliasResolved)
	fmt.Printf("Already canonical:          %d\n", alreadyCanonical)
	fmt.Printf("Unresolved:                 %d\n", unresolvedCount)
	fmt.Printf("Initial unique skills:      %d\n", initialUniqueSkillsCount)

	if *printUnresolved {
		fmt.Println("\n--- Unresolved Skills (freq > 50) ---")
		type skillFreq struct {
			name string
			freq int
		}
		var sfs []skillFreq
		for _, skill := range unresolvedSkills {
			if freqMap[skill] > 50 {
				sfs = append(sfs, skillFreq{skill, freqMap[skill]})
			}
		}
		sort.Slice(sfs, func(i, j int) bool {
			return sfs[i].freq > sfs[j].freq
		})
		for _, sf := range sfs {
			fmt.Printf("%4d  %s\n", sf.freq, sf.name)
		}
		fmt.Println("-------------------------------------")
	}

	llmMappingsAdded := 0
	llmMappingsFlipped := 0
	llmSkipped := false

	llmMap := make(map[string]string)

	if unresolvedCount == 0 {
		fmt.Println("\nAll skills resolved. Skipping LLM.")
		llmSkipped = true
	} else {
		// Step 4: Batch send unresolved to Gemini
		fmt.Printf("\nSending %d skills to LLM...\n", unresolvedCount)
		ctx := context.Background()
		apiKey := os.Getenv("GEMINI_API_KEY")
		if apiKey == "" {
			log.Fatalf("GEMINI_API_KEY is required in environment")
		}

		client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
		if err != nil {
			log.Fatalf("Error creating Gemini client: %v", err)
		}
		defer client.Close()

		batchSize := 200
		batches := (unresolvedCount + batchSize - 1) / batchSize

		limiter := rate.NewLimiter(rate.Limit(15), 1)

		type BatchRequest struct {
			Index  int
			Skills []string
		}
		batchesChan := make(chan BatchRequest, batches)

		for i := 0; i < batches; i++ {
			start := i * batchSize
			end := start + batchSize
			if end > unresolvedCount {
				end = unresolvedCount
			}
			batchesChan <- BatchRequest{Index: i + 1, Skills: unresolvedSkills[start:end]}
		}
		close(batchesChan)

		var mapMu sync.Mutex
		var wg sync.WaitGroup

		for i := 0; i < *workers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				model := client.GenerativeModel("gemini-2.5-flash-lite")
				model.ResponseMIMEType = "application/json"

				for req := range batchesChan {
					skillsListStr := strings.Join(req.Skills, "\n")

					prompt := fmt.Sprintf(`You are a technical skill taxonomy expert.

Given raw skill names from job descriptions, return a JSON mapping of
{raw_name: canonical_name} ONLY for skills that need normalization.

Rules:
- Omit skills that are already in correct canonical form
- Use the abbreviation when it is more commonly used by engineers:
  "GCP" not "Google Cloud Platform", "AWS" not "Amazon Web Services"
- Use official product casing: "PostgreSQL" not "postgres", "GitHub" not "github"
- Collapse version suffixes to base name: "Python 3.9" → "Python", "C++17" → "C++"
- Collapse plural/variants to singular: "APIs" → "API", "Databases" → "Database"
- Do NOT collapse distinct tools into each other
- Do NOT map specific tools to generic categories:
  "Design Patterns" stays "Design Patterns", not "Software Engineering"
  "Dependency Injection" stays "Dependency Injection", not "Software Engineering"
- Do NOT map role titles: skip "Cloud Engineers", "Data Scientists", "DevOps Engineer"
- Do NOT map domain knowledge terms to generic categories:
  "Credit Risk Assessment" stays as is, not "Finance"

Return ONLY valid JSON: {"raw_name": "canonical_name"}
Return {} if nothing needs normalization.

Skills:
%s`, skillsListStr)

					if err := limiter.Wait(ctx); err != nil {
						log.Printf("Rate limiter error: %v", err)
					}

					reqCtx, cancel := context.WithTimeout(ctx, 40*time.Second)

					log.Printf("Batch %d/%d starting...", req.Index, batches)
					resp, err := model.GenerateContent(reqCtx, genai.Text(prompt))
					cancel()

					if err != nil {
						log.Printf("Batch %d/%d failed: %v", req.Index, batches, err)
						continue
					}

					if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
						log.Printf("Batch %d/%d empty response", req.Index, batches)
						continue
					}

					part := resp.Candidates[0].Content.Parts[0]
					var respText string
					if text, ok := part.(genai.Text); ok {
						respText = string(text)
					} else {
						log.Printf("Batch %d/%d unexpected response type", req.Index, batches)
						continue
					}

					respText = strings.TrimSpace(respText)
					if strings.HasPrefix(respText, "```json") {
						respText = strings.TrimPrefix(respText, "```json")
						respText = strings.TrimSuffix(respText, "```")
					} else if strings.HasPrefix(respText, "```") {
						respText = strings.TrimPrefix(respText, "```")
						respText = strings.TrimSuffix(respText, "```")
					}

					var batchMappings map[string]string
					if err := json.Unmarshal([]byte(respText), &batchMappings); err != nil {
						log.Printf("Batch %d/%d failed to parse JSON: %v\nRaw response:\n%s", req.Index, batches, err, respText)
						continue
					}

					added := 0
					mapMu.Lock()
					for k, v := range batchMappings {
						if k != v {
							llmMap[k] = v
							added++
						}
					}
					mapMu.Unlock()
					log.Printf("Batch %d/%d completed. Extracted %d mappings.", req.Index, batches, added)
				}
			}()
		}
		wg.Wait()
		llmMappingsAdded = len(llmMap)
	}

	// Step 5: Apply seed overrides on LLM output
	for k, llmVal := range llmMap {
		if seedVal, ok := seedMap[k]; ok {
			aliasMap[k] = seedVal
			continue
		}

		freqRaw := freqMap[k]
		freqCanonical := freqMap[llmVal]

		if freqRaw > 2*freqCanonical && freqCanonical > 0 {
			aliasMap[llmVal] = k
			log.Printf("Flipped LLM mapping: %s -> %s (freq %d vs %d)", llmVal, k, freqRaw, freqCanonical)
			llmMappingsFlipped++
		} else {
			aliasMap[k] = llmVal
		}
	}

	for k, v := range seedMap {
		aliasMap[k] = v
	}

	// Step 6: Post-processing

	// 6a — Detect and fix circular mappings
	circularMappingsFixed := 0
	for startKey := range aliasMap {
		visited := make(map[string]bool)
		curr := startKey
		cycleDetected := false

		var path []string
		for {
			path = append(path, curr)
			visited[curr] = true
			next, ok := aliasMap[curr]
			if !ok {
				break
			}
			if visited[next] {
				cycleDetected = true
				break
			}
			curr = next
		}

		if cycleDetected {
			cycleStart := curr
			var cycleNodes []string
			inCycle := false
			for _, node := range path {
				if node == cycleStart {
					inCycle = true
				}
				if inCycle {
					cycleNodes = append(cycleNodes, node)
				}
			}

			terminal := cycleNodes[0]
			maxFreq := -1
			for _, node := range cycleNodes {
				f := freqMap[node]
				if f > maxFreq {
					maxFreq = f
					terminal = node
				}
			}

			if len(cycleNodes) == 2 {
				nonCanonical := cycleNodes[0]
				if cycleNodes[0] == terminal {
					nonCanonical = cycleNodes[1]
				}
				log.Printf("Resolved cycle: %s -> %s (freq %d vs %d)", nonCanonical, terminal, freqMap[terminal], freqMap[nonCanonical])
			} else {
				log.Printf("Circular mapping detected: %v. Resolving to %s.", cycleNodes, terminal)
			}

			for _, node := range cycleNodes {
				if node != terminal {
					aliasMap[node] = terminal
					circularMappingsFixed++
				} else {
					delete(aliasMap, node)
				}
			}
		}
	}

	// 6b — Resolve transitive chains
	transitiveChainsResolved := 0
	for k := range aliasMap {
		curr := k
		chainLen := 0
		for {
			next, ok := aliasMap[curr]
			if !ok || next == curr {
				break
			}
			curr = next
			chainLen++
		}
		if chainLen > 1 {
			aliasMap[k] = curr
			transitiveChainsResolved++
		}
	}

	// 6c — Remove self-mappings
	selfMappingsRemoved := 0
	for k, v := range aliasMap {
		if k == v {
			delete(aliasMap, k)
			selfMappingsRemoved++
		}
	}

	// Step 6d - Resolve Blocklist
	resolvedBlocklistMap := make(map[string]bool)
	for _, b := range blocklist {
		resolvedBlocklistMap[b] = true
	}
	for k, v := range aliasMap {
		if resolvedBlocklistMap[v] {
			resolvedBlocklistMap[k] = true
		}
	}
	var resolvedBlocklist []string
	for k := range resolvedBlocklistMap {
		resolvedBlocklist = append(resolvedBlocklist, k)
	}
	sort.Strings(resolvedBlocklist)

	// Step 7: Write output
	outData, err := json.MarshalIndent(aliasMap, "", "  ")
	if err != nil {
		log.Fatalf("Error marshaling output: %v", err)
	}

	os.MkdirAll(filepath.Dir(*aliasMapPath), 0755)
	if err := os.WriteFile(*aliasMapPath, outData, 0644); err != nil {
		log.Fatalf("Error writing %s: %v", *aliasMapPath, err)
	}

	blockData, err := json.MarshalIndent(resolvedBlocklist, "", "  ")
	if err != nil {
		log.Fatalf("Error marshaling blocklist output: %v", err)
	}
	if err := os.WriteFile(*resolvedBlocklistPath, blockData, 0644); err != nil {
		log.Fatalf("Error writing %s: %v", *resolvedBlocklistPath, err)
	}

	// Calculate final unique skills
	finalUniqueSkillsMap := make(map[string]bool)
	for skill := range uniqueSkillsMap {
		if canonical, ok := aliasMap[skill]; ok {
			finalUniqueSkillsMap[canonical] = true
		} else {
			finalUniqueSkillsMap[skill] = true
		}
	}
	for _, b := range resolvedBlocklist {
		delete(finalUniqueSkillsMap, b)
	}
	finalUniqueSkillsCount := len(finalUniqueSkillsMap)

	// Step 8: Stdout summary
	fmt.Printf("\nSent to LLM:                %d\n", unresolvedCount)
	fmt.Printf("LLM mappings added:         %d\n", llmMappingsAdded)
	fmt.Printf("LLM mappings flipped:       %d\n", llmMappingsFlipped)
	fmt.Printf("Cycles resolved by frequency: %d\n", circularMappingsFixed)
	fmt.Printf("Transitive chains resolved: %d\n", transitiveChainsResolved)
	fmt.Printf("Self-mappings removed:      %d\n", selfMappingsRemoved)
	fmt.Printf("Total entries in alias_map: %d\n", len(aliasMap))
	fmt.Printf("Blocklisted skills (resolved): %d\n", len(resolvedBlocklist))
	fmt.Printf("Final unique skills (after aliases & blocks): %d\n", finalUniqueSkillsCount)
	fmt.Printf("\nLLM skipped:                %v\n", llmSkipped)
	fmt.Printf("Output: %s\n\nReview before running Pipeline 3.\n", *aliasMapPath)
}
