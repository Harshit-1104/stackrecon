package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"html"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	goahocorasick "github.com/anknown/ahocorasick"
	"github.com/stackrecon/internal/models"
)

var (
	reNonAlphaNum = regexp.MustCompile(`[^a-z0-9\s]+`)
	reSpace       = regexp.MustCompile(`\s+`)
	reTags        = regexp.MustCompile(`(?i)<[^>]*>`)
	reEntityStray = regexp.MustCompile(`(?i)&(gt|lt|amp|quot|#39|nbsp);?`)

	// Block split boundaries
	reSplit = regexp.MustCompile(`(?i)</p>|</li>|</div>|<br\s*/?>\s*<br\s*/?>`)
)

type protectedTerm struct {
	term        string
	placeholder string
}

var symbolProtections []protectedTerm

func buildSymbolProtections(aliasMapPath string) ([]protectedTerm, error) {
	b, err := os.ReadFile(aliasMapPath)
	if err != nil {
		return nil, err
	}
	var aliasMap map[string]string
	if err := json.Unmarshal(b, &aliasMap); err != nil {
		return nil, err
	}

	uniqueTerms := make(map[string]bool)
	for k, v := range aliasMap {
		uniqueTerms[strings.ToLower(k)] = true
		uniqueTerms[strings.ToLower(v)] = true
	}

	var protections []protectedTerm
	i := 0
	for term := range uniqueTerms {
		if reNonAlphaNum.MatchString(term) {
			placeholder := reNonAlphaNum.ReplaceAllString(term, "") + fmt.Sprintf("xq%d", i)
			protections = append(protections, protectedTerm{term: term, placeholder: placeholder})
			i++
		}
	}

	sort.Slice(protections, func(i, j int) bool {
		return len(protections[i].term) > len(protections[j].term)
	})

	return protections, nil
}

func protectSymbols(s string) string {
	lower := strings.ToLower(s)
	for _, p := range symbolProtections {
		lower = strings.ReplaceAll(lower, p.term, p.placeholder)
	}
	return lower
}

const shortTermMaxLen = 4

var shortTermRegexCache = make(map[string]*regexp.Regexp)
var shortTermExactRegexCache = make(map[string]*regexp.Regexp)

func isAllLowercaseOccurrence(rawText string, term string) bool {
	lower := strings.ToLower(term)
	re, ok := shortTermRegexCache[lower]
	if !ok {
		pattern := `\b` + regexp.QuoteMeta(lower) + `\b`
		re = regexp.MustCompile(pattern)
		shortTermRegexCache[lower] = re
	}
	return re.MatchString(rawText)
}

func hasNonLowercaseOccurrence(rawText string, term string) bool {
	lower := strings.ToLower(term)
	re, ok := shortTermExactRegexCache[lower]
	if !ok {
		pattern := `(?i)\b` + regexp.QuoteMeta(lower) + `\b`
		re = regexp.MustCompile(pattern)
		shortTermExactRegexCache[lower] = re
	}
	for _, match := range re.FindAllString(rawText, -1) {
		if match != lower {
			return true
		}
	}
	return false
}

func isCompanySelfReference(matchedTerm string, companyNameTokens map[string]bool) bool {
	return companyNameTokens[matchedTerm]
}

func normalize(s string) string {
	s = protectSymbols(s)
	s = reNonAlphaNum.ReplaceAllString(s, " ")
	s = reSpace.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

var stopwords = map[string]bool{
	"the": true, "and": true, "for": true, "to": true, "of": true,
	"a": true, "in": true, "is": true, "we": true, "you": true,
	"this": true, "with": true, "are": true, "at": true, "or": true,
	"on": true, "all": true, "an": true, "as": true, "be": true,
}

func buildJaccardWordSet(normalized string) map[string]bool {
	words := strings.Fields(normalized)
	set := make(map[string]bool)
	for _, w := range words {
		if len(w) <= 2 {
			continue
		}
		if stopwords[w] {
			continue
		}
		set[w] = true
	}
	return set
}

func jaccard(a, b map[string]bool) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 0.0
	}
	intersection := 0
	for k := range a {
		if b[k] {
			intersection++
		}
	}
	union := len(a) + len(b) - intersection
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

type ShortTermDiscard struct {
	MatchedTerm      string  `json:"matched_term"`
	BucketTextSample string  `json:"bucket_text_sample"`
	Frequency        float64 `json:"frequency"`
	WouldHaveFlagged bool    `json:"would_have_flagged"`
}

type Block struct {
	JDID        int64
	Raw         string
	DisplayText string
	Normalized  string
	WordSet     map[string]bool
}

type Bucket struct {
	Normalized string
	WordSet    map[string]bool
	Blocks     []*Block
	// JDIDs tracks the distinct JDs this bucket appears in
	JDIDs map[int64]bool
}

type Cluster struct {
	Buckets    []*Bucket
	Centroid   *Bucket
	BlockCount int
	JDIDs      map[int64]bool
}

type BoilerplateDef struct {
	SampleText string  `json:"sample_text"`
	Frequency  float64 `json:"frequency"`
	BlockCount int     `json:"block_count"`
}

type BoilerplateFile struct {
	Slug              string             `json:"slug"`
	ComputedAt        string             `json:"computed_at"`
	Clusters          []BoilerplateDef   `json:"clusters"`
	LowDensityMatches []LowDensityMatch  `json:"low_density_matches,omitempty"`
	ShortTermDiscards []ShortTermDiscard `json:"short_term_discards,omitempty"`
}

type FlaggedBucket struct {
	Normalized string  `json:"normalized"`
}

type FlaggedCluster struct {
	MatchedTerm        string          `json:"matched_term"`
	MatchedBucketIndex int             `json:"matched_bucket_index"`
	ContextStr         string          `json:"context_str"`
	Density            float64         `json:"density"`
	Buckets            []FlaggedBucket `json:"buckets"`
}

type LowDensityMatch struct {
	MatchedTerm string  `json:"matched_term"`
	Density     float64 `json:"density"`
	FullText    string  `json:"full_text"`
}

type LogEntry struct {
	Slug                     string  `json:"slug"`
	JDCount                  int     `json:"jd_count"`
	TotalCharsBefore         int     `json:"total_chars_before"`
	TotalCharsAfter          int     `json:"total_chars_after"`
	PctReduction             float64 `json:"pct_reduction"`
	BoilerplateClustersFound int     `json:"boilerplate_clusters_found"`
	ClustersFlaggedForReview int     `json:"clusters_flagged_for_review"`
}

func main() {
	aliasMapPath := flag.String("alias-map", "", "path to the alias map JSON file")
	inputDir := flag.String("input-dir", "", "path to the processed tech JSON files")
	slugFlag := flag.String("slug", "", "optional specific slug to process")
	flag.Parse()

	var err error
	symbolProtections, err = buildSymbolProtections(*aliasMapPath)
	if err != nil {
		log.Fatalf("failed to build symbol protections: %v", err)
	}

	b, err := os.ReadFile(*aliasMapPath)
	if err != nil {
		log.Fatalf("failed to read alias_map.json: %v", err)
	}

	var aliasMap map[string]string
	if err := json.Unmarshal(b, &aliasMap); err != nil {
		log.Fatalf("failed to parse alias_map.json: %v", err)
	}

	var terms [][]rune
	termSet := make(map[string]bool)

	for k, v := range aliasMap {
		nk := normalize(k)
		nv := normalize(v)
		if nk != "" && !termSet[nk] {
			// word boundary matching requested.
			// ahocorasick operates on runes. We can prepend/append spaces to ensure word boundaries.
			// Or we just build it with runes and check word boundaries during match.
			// The simplest way to enforce word boundaries using Aho-Corasick on plain strings is to
			// wrap terms and the searched text in spaces. e.g. " term "
			termSet[nk] = true
			terms = append(terms, []rune(" "+nk+" "))
		}
		if nv != "" && !termSet[nv] {
			termSet[nv] = true
			terms = append(terms, []rune(" "+nv+" "))
		}
	}

	machine := new(goahocorasick.Machine)
	if err := machine.Build(terms); err != nil {
		log.Fatalf("failed to build ahocorasick: %v", err)
	}

	var files []string
	if *slugFlag != "" {
		files = append(files, filepath.Join(*inputDir, *slugFlag+".json"))
	} else {
		files, err = filepath.Glob(filepath.Join(*inputDir, "*.json"))
		if err != nil {
			log.Fatalf("failed to glob files: %v", err)
		}
	}

	dataDir := filepath.Dir(filepath.Dir(*inputDir))
	os.MkdirAll(filepath.Join(dataDir, "boilerplate"), 0755)
	os.MkdirAll(filepath.Join(dataDir, "logs"), 0755)

	var logEntries []LogEntry
	var allDensities []float64

	const minMatchDensity = 0.10

	for _, file := range files {
		slug := strings.TrimSuffix(filepath.Base(file), ".json")

		companyNameTokens := make(map[string]bool)
		for _, token := range strings.Fields(normalize(slug)) {
			companyNameTokens[token] = true
		}

		b, err := os.ReadFile(file)
		if err != nil {
			log.Printf("failed to read %s: %v", file, err)
			continue
		}

		var jobs []models.GreenhouseJob
		if err := json.Unmarshal(b, &jobs); err != nil {
			log.Printf("failed to unmarshal %s: %v", file, err)
			continue
		}

		if len(jobs) < 5 {
			log.Printf("[%s] skipping: less than 5 tech JDs (%d)", slug, len(jobs))
			continue
		}

		log.Printf("[%s] === Starting pipeline for %d JDs ===", slug, len(jobs))

		// Step 1 & 2
		var allBlocks []*Block
		
		extractBlocks := func(j *models.GreenhouseJob) []*Block {
			full := j.Content
			for k := 0; k < 3; k++ {
				full = html.UnescapeString(full)
			}
			parts := reSplit.Split(full, -1)
			
			var blocks []*Block
			for _, part := range parts {
				dt := reTags.ReplaceAllString(part, " ")
				dt = reEntityStray.ReplaceAllString(dt, "")
				dt = reSpace.ReplaceAllString(dt, " ")
				dt = strings.TrimSpace(dt)
				if dt == "" {
					continue
				}
				norm := normalize(dt)
				if norm == "" {
					continue
				}
				ws := buildJaccardWordSet(norm)
				blocks = append(blocks, &Block{
					JDID:        j.ID,
					Raw:         part,
					DisplayText: dt,
					Normalized:  norm,
					WordSet:     ws,
				})
			}
			return blocks
		}

		for i := range jobs {
			blocks := extractBlocks(&jobs[i])
			// log.Printf("[%s] JD %d: %d blocks extracted", slug, jobs[i].ID, len(blocks))
			allBlocks = append(allBlocks, blocks...)
		}

		log.Printf("[%s] Step 1 & 2: Extracted %d total blocks across %d JDs", slug, len(allBlocks), len(jobs))

		// Step 3a: Exact Bucketing
		bucketMap := make(map[string]*Bucket)
		for _, blk := range allBlocks {
			b, ok := bucketMap[blk.Normalized]
			if !ok {
				b = &Bucket{
					Normalized: blk.Normalized,
					WordSet:    blk.WordSet,
					JDIDs:      make(map[int64]bool),
				}
				bucketMap[blk.Normalized] = b
			}
			b.Blocks = append(b.Blocks, blk)
			b.JDIDs[blk.JDID] = true
		}

		var buckets []*Bucket
		for _, b := range bucketMap {
			buckets = append(buckets, b)
		}

		log.Printf("[%s] Step 3a: Grouped into %d exact buckets", slug, len(buckets))

		// Step 3b: Fuzzy Merge
		sort.Slice(buckets, func(i, j int) bool {
			return len(buckets[i].Blocks) > len(buckets[j].Blocks)
		})

		var clusters []*Cluster
		for _, b := range buckets {
			merged := false
			for _, c := range clusters {
				if jaccard(b.WordSet, c.Centroid.WordSet) >= 0.75 {
					c.Buckets = append(c.Buckets, b)
					c.BlockCount += len(b.Blocks)
					for id := range b.JDIDs {
						c.JDIDs[id] = true
					}
					// Centroid is already the largest because we process in descending size order.
					merged = true
					break
				}
			}
			if !merged {
				jdmap := make(map[int64]bool)
				for id := range b.JDIDs {
					jdmap[id] = true
				}
				clusters = append(clusters, &Cluster{
					Buckets:    []*Bucket{b},
					Centroid:   b,
					BlockCount: len(b.Blocks),
					JDIDs:      jdmap,
				})
			}
		}

		log.Printf("[%s] Step 3b: Merged exact buckets into %d fuzzy clusters", slug, len(clusters))

		// Step 4: Filtering
		totalJDs := len(jobs)
		const minFrequency = 0.60
		const nearMissFrequency = 0.50

		var boilerplateClusters []*Cluster
		var flaggedClusters []FlaggedCluster
		var lowDensityMatches []LowDensityMatch
		var shortTermDiscards []ShortTermDiscard
		nearMissClustersCount := 0

		for _, c := range clusters {
			frequency := float64(len(c.JDIDs)) / float64(totalJDs)
			
			if frequency >= nearMissFrequency {
				// Aho-Corasick check on all variants
				var flaggedMatch *FlaggedCluster
				var clusterLowDensities []LowDensityMatch
				for i, b := range c.Buckets {
					// wrap in spaces for word boundary
					searchStr := " " + b.Normalized + " "
					matches := machine.MultiPatternSearch([]rune(searchStr), false)
					
					n := 0
					var discardedTerms []string
					for _, m := range matches {
						term := strings.TrimSpace(string(m.Word))
						if !isCompanySelfReference(term, companyNameTokens) {
							if len(term) <= shortTermMaxLen {
								hasLower := false
								hasNonLower := false
								for _, blk := range b.Blocks {
									if isAllLowercaseOccurrence(blk.DisplayText, term) {
										hasLower = true
									}
									if hasNonLowercaseOccurrence(blk.DisplayText, term) {
										hasNonLower = true
									}
								}
								// If it ONLY appears in lowercase form across the entire bucket, discard it.
								if hasLower && !hasNonLower {
									discardedTerms = append(discardedTerms, term)
									continue
								}
							}
							matches[n] = m
							n++
						}
					}
					matches = matches[:n]

					searchStrRunes := []rune(searchStr)
					matchedRuneIndices := make([]bool, len(searchStrRunes))
					for _, m := range matches {
						start := m.Pos
						end := m.Pos + len([]rune(m.Word))
						for j := start; j < end; j++ {
							matchedRuneIndices[j] = true
						}
					}

					matchedWordCount := 0
					inWord := false
					for i := 0; i < len(searchStrRunes); i++ {
						if searchStrRunes[i] != ' ' && matchedRuneIndices[i] {
							if !inWord {
								matchedWordCount++
								inWord = true
							}
						} else if searchStrRunes[i] == ' ' {
							inWord = false
						}
					}
					totalWordsInBucketText := len(strings.Fields(b.Normalized))
					
					density := 0.0
					if totalWordsInBucketText > 0 {
						density = float64(matchedWordCount) / float64(totalWordsInBucketText)
					}
					
					for _, dt := range discardedTerms {
						discardedWordCount := len(strings.Fields(dt))
						hypotheticalDensity := 0.0
						if totalWordsInBucketText > 0 {
							hypotheticalDensity = float64(matchedWordCount + discardedWordCount) / float64(totalWordsInBucketText)
						}
						
						wouldHaveFlagged := hypotheticalDensity >= minMatchDensity
						
						sample := ""
						if len(b.Blocks) > 0 {
							sample = b.Blocks[0].DisplayText
							if len(sample) > 150 {
								sample = sample[:150]
							}
						}

						shortTermDiscards = append(shortTermDiscards, ShortTermDiscard{
							MatchedTerm:      dt,
							BucketTextSample: sample,
							Frequency:        frequency,
							WouldHaveFlagged: wouldHaveFlagged,
						})
					}

					if len(matches) > 0 {
						allDensities = append(allDensities, density)

						if density < minMatchDensity {
							term := strings.TrimSpace(string(matches[0].Word))
							clusterLowDensities = append(clusterLowDensities, LowDensityMatch{
								MatchedTerm: term,
								Density:     density,
								FullText:    b.Normalized,
							})
							continue
						}

						match := matches[0]
						start := match.Pos
						end := start + len(match.Word)

						// surrounding ~10 characters
						ctxStart := start - 10
						if ctxStart < 0 {
							ctxStart = 0
						}
						ctxEnd := end + 10
						if ctxEnd > len([]rune(searchStr)) {
							ctxEnd = len([]rune(searchStr))
						}

						contextStr := string([]rune(searchStr)[ctxStart:ctxEnd])

						var fb []FlaggedBucket
						for _, bucket := range c.Buckets {
							fb = append(fb, FlaggedBucket{
								Normalized: bucket.Normalized,
							})
						}

						flaggedMatch = &FlaggedCluster{
							MatchedTerm:        strings.TrimSpace(string(match.Word)),
							MatchedBucketIndex: i,
							ContextStr:         strings.TrimSpace(contextStr),
							Density:            density,
							Buckets:            fb,
						}
						break
					}
				}

				if frequency < minFrequency {
					nearMissClustersCount++
					continue
				}

				if flaggedMatch != nil {
					flaggedClusters = append(flaggedClusters, *flaggedMatch)
				} else {
					boilerplateClusters = append(boilerplateClusters, c)
					lowDensityMatches = append(lowDensityMatches, clusterLowDensities...)
				}
			}
		}

		candidateCount := len(boilerplateClusters) + len(flaggedClusters)
		log.Printf("[%s] Step 4: Evaluated %d clusters meeting frequency threshold (>=%.0f%% of %d JDs)", slug, candidateCount, minFrequency*100, totalJDs)
		log.Printf("[%s] -> %d clusters flagged for manual review (high-density safety matches)", slug, len(flaggedClusters))
		log.Printf("[%s] -> %d low-density match events safely auto-stripped", slug, len(lowDensityMatches))
		log.Printf("[%s] -> %d clusters cleared as pure boilerplate", slug, len(boilerplateClusters))
		log.Printf("[%s] -> %d near-miss clusters (50%%-60%%) logged and bypassed", slug, nearMissClustersCount)

		if len(shortTermDiscards) > 0 {
			out, _ := json.MarshalIndent(shortTermDiscards, "", "  ")
			os.WriteFile(filepath.Join(dataDir, "logs", fmt.Sprintf("boilerplate_short_term_discards_%s.json", slug)), out, 0644)
		}

		if len(lowDensityMatches) > 0 {
			out, _ := json.MarshalIndent(lowDensityMatches, "", "  ")
			os.WriteFile(filepath.Join(dataDir, "logs", fmt.Sprintf("boilerplate_low_density_matches_%s.json", slug)), out, 0644)
		}

		if len(flaggedClusters) > 0 {
			out, _ := json.MarshalIndent(flaggedClusters, "", "  ")
			os.WriteFile(filepath.Join(dataDir, "logs", fmt.Sprintf("boilerplate_flagged_%s.json", slug)), out, 0644)
		}

		// Build fast lookup for boilerplate comparison
		isBoilerplateBlock := func(norm string, ws map[string]bool) bool {
			// First try exact match across all buckets in all boilerplate clusters
			// Actually we can just compare against centroids via Jaccard
			for _, c := range boilerplateClusters {
				if jaccard(ws, c.Centroid.WordSet) >= 0.75 {
					return true
				}
			}
			return false
		}

		// Step 5 & 6
		var bfile BoilerplateFile
		bfile.Slug = slug
		bfile.ComputedAt = time.Now().Format(time.RFC3339)
		bfile.LowDensityMatches = lowDensityMatches
		bfile.ShortTermDiscards = shortTermDiscards
		for _, c := range boilerplateClusters {
			bfile.Clusters = append(bfile.Clusters, BoilerplateDef{
				SampleText: c.Centroid.Blocks[0].DisplayText,
				Frequency:  float64(len(c.JDIDs)) / float64(totalJDs),
				BlockCount: c.BlockCount,
			})
		}

		out, _ := json.MarshalIndent(bfile, "", "  ")
		os.WriteFile(filepath.Join(dataDir, "boilerplate", slug+".json"), out, 0644)

		charsBefore := 0
		charsAfter := 0

		for i := range jobs {
			j := &jobs[i]
			blocks := extractBlocks(j)
			
			var kept []string
			for _, blk := range blocks {
				if !isBoilerplateBlock(blk.Normalized, blk.WordSet) {
					kept = append(kept, blk.DisplayText)
				}
			}

			// Save to CleanedContent
			cleaned := strings.Join(kept, "\n\n")
			j.CleanedContent = cleaned

			charsBefore += len(j.Content)
			charsAfter += len(cleaned)
		}

		// Overwrite processed tech JDs
		outTech, _ := json.MarshalIndent(jobs, "", "  ")
		os.WriteFile(file, outTech, 0644)

		pctReduction := 0.0
		if charsBefore > 0 {
			pctReduction = float64(charsBefore-charsAfter) / float64(charsBefore) * 100.0
		}

		log.Printf("[%s] Step 5 & 6: Stripped boilerplate. Reduced chars from %d to %d (%.2f%%)", slug, charsBefore, charsAfter, pctReduction)

		logEntries = append(logEntries, LogEntry{
			Slug:                     slug,
			JDCount:                  totalJDs,
			TotalCharsBefore:         charsBefore,
			TotalCharsAfter:          charsAfter,
			PctReduction:             pctReduction,
			BoilerplateClustersFound: len(boilerplateClusters),
			ClustersFlaggedForReview: len(flaggedClusters),
		})
	}

	sort.Slice(logEntries, func(i, j int) bool {
		return logEntries[i].PctReduction > logEntries[j].PctReduction
	})

	timestamp := time.Now().Format("20060102_150405")
	outLogs, _ := json.MarshalIndent(logEntries, "", "  ")
	os.WriteFile(filepath.Join(dataDir, "logs", fmt.Sprintf("boilerplate_strip_%s.json", timestamp)), outLogs, 0644)

	fmt.Printf("%-20s | %-12s | %-12s | %-10s\n", "Slug", "Before Chars", "After Chars", "% Reduction")
	fmt.Println(strings.Repeat("-", 62))
	for _, l := range logEntries {
		fmt.Printf("%-20s | %-12d | %-12d | %-9.2f%%\n", l.Slug, l.TotalCharsBefore, l.TotalCharsAfter, l.PctReduction)
	}

	if len(allDensities) > 0 {
		sort.Float64s(allDensities)
		minDensity := allDensities[0]
		maxDensity := allDensities[len(allDensities)-1]
		p50 := allDensities[len(allDensities)/2]
		
		fmt.Printf("\n--- Density Distribution of Alias Matches ---\n")
		fmt.Printf("Total Matches Evaluated: %d\n", len(allDensities))
		fmt.Printf("Min Density:  %.4f\n", minDensity)
		fmt.Printf("P50 Density:  %.4f\n", p50)
		fmt.Printf("Max Density:  %.4f\n", maxDensity)
		fmt.Println("---------------------------------------------")
	}
}
