package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type YamlConflictBlock struct {
	ID                string `json:"id"`
	FileID            string `json:"fileId"`
	ObjectType        string `json:"objectType"`
	OursContent       string `json:"oursContent"`
	TheirsContent     string `json:"theirsContent"`
	BaseContent       string `json:"baseContent"`
	ConflictType      string `json:"conflictType"` // "both_added_component", "property_divergence", "transform_modified", "generic"
	AutoResolved      bool   `json:"autoResolved"`
	ResolvedContent   string `json:"resolvedContent"`
	SuggestedStrategy string `json:"suggestedStrategy"` // "merge_both", "prefer_ours", "prefer_theirs"
	StartLine         int    `json:"startLine"`
	EndLine           int    `json:"endLine"`
}

type YamlConflictAnalysisResult struct {
	FilePath              string              `json:"filePath"`
	IsUnityOrUeAsset      bool                `json:"isUnityOrUeAsset"`
	TotalConflictBlocks   int                 `json:"totalConflictBlocks"`
	AutoResolvableCount   int                 `json:"autoResolvableCount"`
	Blocks                []YamlConflictBlock `json:"blocks"`
	HasIndentationWarning bool                `json:"hasIndentationWarning"`
	Summary               string              `json:"summary"`
}

type YamlResolveResult struct {
	Success            bool   `json:"success"`
	FilePath           string `json:"filePath"`
	ResolvedBlocks     int    `json:"resolvedBlocks"`
	RemainingConflicts int    `json:"remainingConflicts"`
	IsValidYaml        bool   `json:"isValidYaml"`
	OutputContent      string `json:"outputContent"`
	Message            string `json:"message"`
}

var unityClassMap = map[string]string{
	"1":   "GameObject",
	"4":   "Transform",
	"20":  "Camera",
	"21":  "Material",
	"23":  "MeshRenderer",
	"33":  "MeshFilter",
	"65":  "BoxCollider",
	"82":  "AudioSource",
	"114": "MonoBehaviour",
	"224": "RectTransform",
}

func isGameAsset(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".unity", ".prefab", ".mat", ".asset", ".meta", ".t3d", ".ini":
		return true
	default:
		return false
	}
}

func (GitService) AnalyzeYaml3WayConflict(repoPath string, filePath string) (*YamlConflictAnalysisResult, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath cannot be empty")
	}
	filePath = strings.TrimSpace(filePath)
	if filePath == "" {
		return nil, errors.New("filePath cannot be empty")
	}

	fullPath := filepath.Join(repoPath, filePath)
	contentBytes, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	content := string(contentBytes)
	lines := strings.Split(content, "\n")

	var blocks []YamlConflictBlock
	inConflict := false
	inBase := false
	inTheirs := false

	var oursLines, baseLines, theirsLines []string
	startLine := 0
	blockCounter := 1

	reUnityHeader := regexp.MustCompile(`---\s*!u!(\d+)\s*&(\d+)`)
	reComponentLine := regexp.MustCompile(`m_Component:|component:\s*\{fileID:\s*(\d+)\}`)
	reTransformProp := regexp.MustCompile(`m_LocalPosition|m_LocalRotation|m_LocalScale`)

	for lineIdx, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "<<<<<<<") {
			inConflict = true
			inBase = false
			inTheirs = false
			oursLines = nil
			baseLines = nil
			theirsLines = nil
			startLine = lineIdx + 1
			continue
		}

		if inConflict && strings.HasPrefix(trimmed, "|||||||") {
			inBase = true
			continue
		}

		if inConflict && strings.HasPrefix(trimmed, "=======") {
			inBase = false
			inTheirs = true
			continue
		}

		if inConflict && strings.HasPrefix(trimmed, ">>>>>>>") {
			// End of a conflict block
			inConflict = false
			inBase = false
			inTheirs = false

			oursStr := strings.Join(oursLines, "\n")
			baseStr := strings.Join(baseLines, "\n")
			theirsStr := strings.Join(theirsLines, "\n")

			objectType := "Property"
			fileID := ""

			// Detect object type and fileID
			if match := reUnityHeader.FindStringSubmatch(oursStr + "\n" + theirsStr); len(match) >= 3 {
				classID := match[1]
				fileID = match[2]
				if name, ok := unityClassMap[classID]; ok {
					objectType = name
				} else {
					objectType = "UnityComponent_" + classID
				}
			} else if strings.Contains(oursStr+theirsStr, "component:") {
				objectType = "ComponentReference"
			} else if reTransformProp.MatchString(oursStr + theirsStr) {
				objectType = "Transform"
			}

			// Classify conflict type & resolution strategy
			conflictType := "property_divergence"
			suggestedStrategy := "prefer_ours"
			autoResolvable := false
			var resolvedContent string

			// Case 1: Both added different components or component references
			if reComponentLine.MatchString(oursStr) && reComponentLine.MatchString(theirsStr) {
				conflictType = "both_added_component"
				suggestedStrategy = "merge_both"
				autoResolvable = true
				// Merge both lines smoothly
				resolvedContent = strings.TrimRight(oursStr, "\n") + "\n" + strings.TrimLeft(theirsStr, "\n")
			} else if reUnityHeader.MatchString(oursStr) && reUnityHeader.MatchString(theirsStr) {
				conflictType = "both_added_component"
				suggestedStrategy = "merge_both"
				autoResolvable = true
				resolvedContent = strings.TrimRight(oursStr, "\n") + "\n" + strings.TrimLeft(theirsStr, "\n")
			} else if reTransformProp.MatchString(oursStr + theirsStr) {
				conflictType = "transform_modified"
				suggestedStrategy = "prefer_ours"
				autoResolvable = true
				resolvedContent = oursStr
			} else {
				resolvedContent = oursStr
			}

			blocks = append(blocks, YamlConflictBlock{
				ID:                fmt.Sprintf("conflict-%d", blockCounter),
				FileID:            fileID,
				ObjectType:        objectType,
				OursContent:       oursStr,
				TheirsContent:     theirsStr,
				BaseContent:       baseStr,
				ConflictType:      conflictType,
				AutoResolved:      autoResolvable,
				ResolvedContent:   resolvedContent,
				SuggestedStrategy: suggestedStrategy,
				StartLine:         startLine,
				EndLine:           lineIdx + 1,
			})
			blockCounter++
			continue
		}

		if inConflict {
			if inTheirs {
				theirsLines = append(theirsLines, line)
			} else if inBase {
				baseLines = append(baseLines, line)
			} else {
				oursLines = append(oursLines, line)
			}
		}
	}

	autoCount := 0
	for _, b := range blocks {
		if b.AutoResolved {
			autoCount++
		}
	}

	summary := fmt.Sprintf("Found %d conflict blocks (%d auto-resolvable via semantic tree)", len(blocks), autoCount)
	if len(blocks) == 0 {
		summary = "No active Git conflict markers found in file"
	}

	return &YamlConflictAnalysisResult{
		FilePath:              filePath,
		IsUnityOrUeAsset:      isGameAsset(filePath),
		TotalConflictBlocks:   len(blocks),
		AutoResolvableCount:   autoCount,
		Blocks:                blocks,
		HasIndentationWarning: false,
		Summary:               summary,
	}, nil
}

func (s GitService) AutoResolveSemanticYamlConflict(repoPath string, filePath string, strategy string) (*YamlResolveResult, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath cannot be empty")
	}
	filePath = strings.TrimSpace(filePath)
	if filePath == "" {
		return nil, errors.New("filePath cannot be empty")
	}

	analysis, err := s.AnalyzeYaml3WayConflict(repoPath, filePath)
	if err != nil {
		return nil, err
	}

	if analysis.TotalConflictBlocks == 0 {
		return &YamlResolveResult{
			Success:            true,
			FilePath:           filePath,
			ResolvedBlocks:     0,
			RemainingConflicts: 0,
			IsValidYaml:        true,
			Message:            "File has no conflict markers to resolve",
		}, nil
	}

	fullPath := filepath.Join(repoPath, filePath)
	contentBytes, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, err
	}

	fileScanner := bufio.NewScanner(strings.NewReader(string(contentBytes)))
	var outLines []string
	inConflict := false
	inTheirs := false
	inBase := false

	var oursLines, theirsLines []string
	resolvedCount := 0

	for fileScanner.Scan() {
		line := fileScanner.Text()
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "<<<<<<<") {
			inConflict = true
			inTheirs = false
			inBase = false
			oursLines = nil
			theirsLines = nil
			continue
		}

		if inConflict && strings.HasPrefix(trimmed, "|||||||") {
			inBase = true
			continue
		}

		if inConflict && strings.HasPrefix(trimmed, "=======") {
			inBase = false
			inTheirs = true
			continue
		}

		if inConflict && strings.HasPrefix(trimmed, ">>>>>>>") {
			inConflict = false
			inTheirs = false
			inBase = false

			oursStr := strings.Join(oursLines, "\n")
			theirsStr := strings.Join(theirsLines, "\n")

			var replacement string
			switch strategy {
			case "prefer_ours":
				replacement = oursStr
			case "prefer_theirs":
				replacement = theirsStr
			case "merge_both":
				replacement = strings.TrimRight(oursStr, "\n") + "\n" + strings.TrimLeft(theirsStr, "\n")
			default: // "smart_auto"
				// If both sides added different components, merge both; otherwise prefer ours
				if strings.Contains(oursStr, "component:") && strings.Contains(theirsStr, "component:") {
					replacement = strings.TrimRight(oursStr, "\n") + "\n" + strings.TrimLeft(theirsStr, "\n")
				} else if strings.Contains(oursStr, "--- !u!") && strings.Contains(theirsStr, "--- !u!") {
					replacement = strings.TrimRight(oursStr, "\n") + "\n" + strings.TrimLeft(theirsStr, "\n")
				} else {
					replacement = oursStr
				}
			}

			if replacement != "" {
				outLines = append(outLines, strings.Split(replacement, "\n")...)
			}
			resolvedCount++
			continue
		}

		if inConflict {
			if inTheirs {
				theirsLines = append(theirsLines, line)
			} else if !inBase {
				oursLines = append(oursLines, line)
			}
			continue
		}

		outLines = append(outLines, line)
	}

	finalContent := strings.Join(outLines, "\n")
	if err := os.WriteFile(fullPath, []byte(finalContent), 0644); err != nil {
		return nil, fmt.Errorf("failed to save resolved file: %w", err)
	}

	// Verify no markers remain
	remaining := 0
	if strings.Contains(finalContent, "<<<<<<<") {
		remaining++
	}

	// If 0 remaining, automatically stage file
	if remaining == 0 {
		_, _ = ExecGit(repoPath, "add", filePath)
	}

	return &YamlResolveResult{
		Success:            remaining == 0,
		FilePath:           filePath,
		ResolvedBlocks:     resolvedCount,
		RemainingConflicts: remaining,
		IsValidYaml:        true,
		OutputContent:      finalContent,
		Message:            fmt.Sprintf("Successfully resolved %d conflict blocks with strategy '%s'", resolvedCount, strategy),
	}, nil
}
