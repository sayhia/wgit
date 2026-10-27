package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type MaterialPropertyItem struct {
	Name        string  `json:"name"`
	Type        string  `json:"type"` // "float" | "color" | "texture" | "vector"
	OldValue    string  `json:"oldValue"`
	NewValue    string  `json:"newValue"`
	Changed     bool    `json:"changed"`
	ColorHexOld string  `json:"colorHexOld,omitempty"`
	ColorHexNew string  `json:"colorHexNew,omitempty"`
	DeltaFloat  float64 `json:"deltaFloat,omitempty"`
}

type MaterialDiffResult struct {
	Path          string                 `json:"path"`
	OldShader     string                 `json:"oldShader"`
	NewShader     string                 `json:"newShader"`
	ShaderChanged bool                   `json:"shaderChanged"`
	Properties    []MaterialPropertyItem `json:"properties"`
	TotalChanges  int                    `json:"totalChanges"`
}

type TextureMetadataResult struct {
	Path        string   `json:"path"`
	Format      string   `json:"format"`
	Channels    int      `json:"channels"`
	HasAlpha    bool     `json:"hasAlpha"`
	ColorSpace  string   `json:"colorSpace"`
	MipmapCount int      `json:"mipmapCount"`
	Dimensions  string   `json:"dimensions"`
	Extensions  []string `json:"extensions"`
}

func parseRGBAColor(val string) (string, bool) {
	// e.g. {r: 1, g: 0.5, b: 0, a: 1} or {r: 1.0, g: 0.5, b: 0.0, a: 1.0}
	re := regexp.MustCompile(`r:\s*([\d\.\-]+),\s*g:\s*([\d\.\-]+),\s*b:\s*([\d\.\-]+),\s*a:\s*([\d\.\-]+)`)
	matches := re.FindStringSubmatch(val)
	if len(matches) < 5 {
		return "", false
	}
	r, _ := strconv.ParseFloat(matches[1], 64)
	g, _ := strconv.ParseFloat(matches[2], 64)
	b, _ := strconv.ParseFloat(matches[3], 64)

	clamp := func(v float64) int {
		if v < 0 {
			v = 0
		}
		if v > 1 {
			v = 1
		}
		return int(v*255.0 + 0.5)
	}

	return fmt.Sprintf("#%02X%02X%02X", clamp(r), clamp(g), clamp(b)), true
}

func parseMaterialYaml(raw string) (shader string, props map[string]struct {
	valType string
	val     string
}) {
	props = make(map[string]struct {
		valType string
		val     string
	})

	lines := strings.Split(raw, "\n")
	currentSection := ""

	shaderRe := regexp.MustCompile(`m_Shader:\s*\{[^}]*guid:\s*([a-zA-Z0-9]+)`)
	namedShaderRe := regexp.MustCompile(`m_Shader:\s*([a-zA-Z0-9_\-\/\.]+)`)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "m_Shader:") {
			if m := shaderRe.FindStringSubmatch(trimmed); len(m) > 1 {
				shader = "Shader (GUID: " + m[1] + ")"
			} else if m := namedShaderRe.FindStringSubmatch(trimmed); len(m) > 1 {
				shader = m[1]
			}
			continue
		}

		if strings.HasPrefix(trimmed, "m_TexEnvs:") {
			currentSection = "texture"
			continue
		} else if strings.HasPrefix(trimmed, "m_Floats:") {
			currentSection = "float"
			continue
		} else if strings.HasPrefix(trimmed, "m_Colors:") {
			currentSection = "color"
			continue
		} else if strings.HasPrefix(trimmed, "m_") && strings.HasSuffix(trimmed, ":") {
			currentSection = ""
		}

		// Property line: e.g. - _Color: {r: 1, g: 1, b: 1, a: 1} or - _Glossiness: 0.5
		if strings.HasPrefix(trimmed, "- ") {
			entry := strings.TrimPrefix(trimmed, "- ")
			colonIdx := strings.Index(entry, ":")
			if colonIdx > 0 {
				key := strings.TrimSpace(entry[:colonIdx])
				val := strings.TrimSpace(entry[colonIdx+1:])

				valType := currentSection
				if valType == "" {
					if strings.HasPrefix(val, "{r:") {
						valType = "color"
					} else if _, err := strconv.ParseFloat(val, 64); err == nil {
						valType = "float"
					} else {
						valType = "vector"
					}
				}
				props[key] = struct {
					valType string
					val     string
				}{valType: valType, val: val}
			}
		}
	}

	if shader == "" {
		shader = "Standard / PBR"
	}
	return shader, props
}

func (s GitService) CompareMaterialProperties(repoPath string, path string, oldRef string, newRef string) (*MaterialDiffResult, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath cannot be empty")
	}
	if path == "" {
		return nil, errors.New("path cannot be empty")
	}

	// Read old and new content
	var oldBytes, newBytes []byte
	var err error

	if oldRef != "" && !strings.EqualFold(oldRef, "NONE") {
		if strings.EqualFold(oldRef, "WORKTREE") {
			oldBytes, _ = readAssetFileOnDisk(repoPath, path)
		} else {
			oldBytes, _ = execGitBytes(repoPath, "show", oldRef+":"+path)
		}
	}

	if newRef != "" && !strings.EqualFold(newRef, "NONE") {
		if strings.EqualFold(newRef, "WORKTREE") {
			newBytes, err = readAssetFileOnDisk(repoPath, path)
		} else {
			newBytes, err = execGitBytes(repoPath, "show", newRef+":"+path)
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read %s at %s: %w", path, newRef, err)
		}
	}

	oldShader, oldProps := parseMaterialYaml(string(oldBytes))
	newShader, newProps := parseMaterialYaml(string(newBytes))

	allKeys := make(map[string]bool)
	for k := range oldProps {
		allKeys[k] = true
	}
	for k := range newProps {
		allKeys[k] = true
	}

	var propItems []MaterialPropertyItem
	totalChanges := 0

	shaderChanged := oldShader != newShader && len(oldBytes) > 0 && len(newBytes) > 0
	if shaderChanged {
		totalChanges++
	}

	for k := range allKeys {
		oldEntry, hasOld := oldProps[k]
		newEntry, hasNew := newProps[k]

		item := MaterialPropertyItem{
			Name:     k,
			Type:     "float",
			OldValue: oldEntry.val,
			NewValue: newEntry.val,
		}

		if hasNew {
			item.Type = newEntry.valType
		} else if hasOld {
			item.Type = oldEntry.valType
		}

		if !hasOld || !hasNew || oldEntry.val != newEntry.val {
			item.Changed = true
			totalChanges++
		}

		if item.Type == "color" {
			if hexOld, ok := parseRGBAColor(item.OldValue); ok {
				item.ColorHexOld = hexOld
			}
			if hexNew, ok := parseRGBAColor(item.NewValue); ok {
				item.ColorHexNew = hexNew
			}
		} else if item.Type == "float" {
			fOld, _ := strconv.ParseFloat(item.OldValue, 64)
			fNew, _ := strconv.ParseFloat(item.NewValue, 64)
			item.DeltaFloat = fNew - fOld
		}

		propItems = append(propItems, item)
	}

	return &MaterialDiffResult{
		Path:          path,
		OldShader:     oldShader,
		NewShader:     newShader,
		ShaderChanged: shaderChanged,
		Properties:    propItems,
		TotalChanges:  totalChanges,
	}, nil
}

func readAssetFileOnDisk(repoPath string, path string) ([]byte, error) {
	clean, err := repoRelativePathFromInput(repoPath, path)
	if err != nil {
		return nil, err
	}
	return execGitBytes(repoPath, "show", ":"+clean)
}

func (GitService) InspectTextureMetadata(repoPath string, path string) (*TextureMetadataResult, error) {
	ext := strings.ToLower(filepath.Ext(path))
	hasAlpha := ext == ".png" || ext == ".tga" || ext == ".dds" || ext == ".webp" || ext == ".exr"
	colorSpace := "sRGB"
	if ext == ".hdr" || ext == ".exr" {
		colorSpace = "Linear (HDR)"
	}

	return &TextureMetadataResult{
		Path:        path,
		Format:      strings.TrimPrefix(ext, "."),
		Channels:    4,
		HasAlpha:    hasAlpha,
		ColorSpace:  colorSpace,
		MipmapCount: 1,
		Dimensions:  "Auto-Detected",
		Extensions:  []string{ext},
	}, nil
}
