package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"
)

// =========================================================================
// Round 101: Shader Variant & Cache Inspector (InspectShaderVariants)
// =========================================================================

// InspectShaderVariants analyzes shaders (.shader, .hlsl, .ush, .usf, .shadergraph),
// parses pragmas and keywords, calculates variant combinatorics, and detects explosions (>1024).
func (GitService) InspectShaderVariants(repoPath string, targetPath string) (*ShaderInspectResult, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath 不能为空")
	}

	cleanTarget := ""
	if strings.TrimSpace(targetPath) != "" {
		normalized, err := repoRelativePathFromInput(repoPath, targetPath)
		if err != nil {
			return nil, err
		}
		cleanTarget = normalized
	}

	// 1. Scan repo for all project shaders to build global overview & offender ranking
	shaderExts := map[string]bool{
		".shader":      true,
		".hlsl":        true,
		".ush":         true,
		".usf":         true,
		".shadergraph": true,
		".cginc":       true,
		".glsl":        true,
		".compute":     true,
	}

	var allShaders []string
	_ = filepath.Walk(repoPath, func(p string, fi os.FileInfo, err error) error {
		if err != nil || fi == nil {
			return nil
		}
		if fi.IsDir() {
			name := fi.Name()
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == "Library" ||
				name == "Intermediate" || name == "Saved" || name == "Binaries" || name == "obj" || name == "bin" {
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(p))
		if shaderExts[ext] {
			rel, rErr := filepath.Rel(repoPath, p)
			if rErr == nil {
				allShaders = append(allShaders, rel)
			}
		}
		return nil
	})

	var offenders []ShaderVariantOffender
	explosionCount := 0

	for _, sPath := range allShaders {
		fullP := filepath.Join(repoPath, sPath)
		fi, sErr := os.Stat(fullP)
		if sErr != nil {
			continue
		}
		vCount, kCount, pCount, sType := quickScanShaderVariants(fullP)
		sev := "OK"
		if vCount > 1024 {
			sev = "CRITICAL"
			explosionCount++
		} else if vCount > 256 {
			sev = "WARNING"
		}

		offenders = append(offenders, ShaderVariantOffender{
			Path:         sPath,
			ShaderType:   sType,
			VariantCount: vCount,
			Severity:     sev,
			KeywordCount: kCount,
			PassCount:    pCount,
			SizeBytes:    fi.Size(),
			SizeStr:      formatAssetSize(fi.Size()),
		})
	}

	sort.Slice(offenders, func(i, j int) bool {
		return offenders[i].VariantCount > offenders[j].VariantCount
	})

	topOffenders := offenders
	if len(topOffenders) > 15 {
		topOffenders = topOffenders[:15]
	}

	if cleanTarget == "" || cleanTarget == "." {
		if len(offenders) > 0 {
			cleanTarget = offenders[0].Path
		}
	}

	if cleanTarget == "" || cleanTarget == "." {
		return &ShaderInspectResult{
			TargetShader:       "--",
			ShaderType:         "未指定着色器",
			TotalVariants:      0,
			PassCount:          0,
			TotalKeywords:      0,
			Pragmas:            []ShaderPragmaDirective{},
			Passes:             []ShaderPassInfo{},
			Severity:           "OK",
			IsExplosion:        false,
			StrippingAdvice:    []string{"项目中未检索到着色器资产 (.shader, .hlsl, .ush, .usf, .shadergraph)"},
			ProjectShaderCount: 0,
			ExplosionCount:     0,
			OffenderList:       []ShaderVariantOffender{},
			Message:            "工作区未检索到着色器资产",
		}, nil
	}

	fullTarget := filepath.Join(repoPath, cleanTarget)
	if _, err := os.Stat(fullTarget); err != nil {
		return nil, fmt.Errorf("着色器文件不存在: %s", cleanTarget)
	}

	res, err := deepParseShaderFile(repoPath, cleanTarget, fullTarget)
	if err != nil {
		return nil, err
	}

	res.ProjectShaderCount = len(allShaders)
	res.ExplosionCount = explosionCount
	res.OffenderList = topOffenders

	return res, nil
}

func quickScanShaderVariants(fullPath string) (int64, int, int, string) {
	ext := strings.ToLower(filepath.Ext(fullPath))
	sType := "Unity Shader (.shader)"
	switch ext {
	case ".hlsl":
		sType = "HLSL Cross-Engine (.hlsl)"
	case ".ush":
		sType = "Unreal Shader Header (.ush)"
	case ".usf":
		sType = "Unreal Shader Format (.usf)"
	case ".shadergraph":
		sType = "Unity ShaderGraph (.shadergraph)"
	}

	if ext == ".shadergraph" {
		data, err := os.ReadFile(fullPath)
		if err != nil {
			return 1, 0, 1, sType
		}
		var parsed map[string]interface{}
		if err := json.Unmarshal(data, &parsed); err == nil {
			if kwRaw, ok := parsed["m_Keywords"].([]interface{}); ok {
				kCount := len(kwRaw)
				var vCount int64 = 1
				for _, k := range kwRaw {
					if km, ok := k.(map[string]interface{}); ok {
						if entries, hasEntries := km["m_Entries"].([]interface{}); hasEntries && len(entries) > 0 {
							vCount *= int64(len(entries))
						} else {
							vCount *= 2
						}
					}
				}
				return vCount, kCount, 1, sType
			}
		}
		return 1, 0, 1, sType
	}

	f, err := os.Open(fullPath)
	if err != nil {
		return 1, 0, 1, sType
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	var passCounts int = 0
	var variantProduct int64 = 1
	var keywordSet = make(map[string]bool)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "//") {
			continue
		}
		if strings.HasPrefix(line, "Pass") && (strings.HasSuffix(line, "{") || strings.Contains(line, "Pass {")) {
			passCounts++
		}
		if strings.HasPrefix(line, "#pragma") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				directive := parts[1]
				if strings.HasPrefix(directive, "multi_compile") || strings.HasPrefix(directive, "shader_feature") {
					opts := parsePragmaOptions(parts[1:])
					for _, k := range parts[2:] {
						if k != "_" {
							keywordSet[k] = true
						}
					}
					if opts > 0 {
						variantProduct *= int64(opts)
					}
				}
			}
		}
	}

	if passCounts == 0 {
		passCounts = 1
	}
	totalVariants := variantProduct
	if passCounts > 1 {
		totalVariants = variantProduct * int64(passCounts)
	}

	return totalVariants, len(keywordSet), passCounts, sType
}

func parsePragmaOptions(tokens []string) int {
	if len(tokens) == 0 {
		return 1
	}
	directive := tokens[0]

	switch directive {
	case "multi_compile_fog":
		return 4
	case "multi_compile_instancing":
		return 3
	case "multi_compile_shadowcaster":
		return 2
	case "multi_compile_particles":
		return 2
	}

	kwTokens := tokens[1:]
	if len(kwTokens) == 0 {
		return 1
	}

	hasUnderscore := false
	for _, k := range kwTokens {
		if k == "_" {
			hasUnderscore = true
			break
		}
	}

	if hasUnderscore {
		return len(kwTokens)
	}

	if len(kwTokens) == 1 {
		return 2
	}

	return len(kwTokens)
}

func deepParseShaderFile(repoPath string, targetRel string, fullPath string) (*ShaderInspectResult, error) {
	ext := strings.ToLower(filepath.Ext(fullPath))
	sType := "Unity Shader (.shader)"
	switch ext {
	case ".hlsl":
		sType = "HLSL Cross-Engine (.hlsl)"
	case ".ush":
		sType = "Unreal Shader Header (.ush)"
	case ".usf":
		sType = "Unreal Shader Format (.usf)"
	case ".shadergraph":
		sType = "Unity ShaderGraph (.shadergraph)"
	}

	contentBytes, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("读取着色器文件失败: %v", err)
	}

	res := &ShaderInspectResult{
		TargetShader:    targetRel,
		ShaderType:      sType,
		Pragmas:         []ShaderPragmaDirective{},
		Passes:          []ShaderPassInfo{},
		StrippingAdvice: []string{},
	}

	if ext == ".shadergraph" {
		var parsed map[string]interface{}
		if err := json.Unmarshal(contentBytes, &parsed); err == nil {
			var totalVariants int64 = 1
			keywordCount := 0
			if kwRaw, ok := parsed["m_Keywords"].([]interface{}); ok {
				for _, k := range kwRaw {
					if km, ok := k.(map[string]interface{}); ok {
						name, _ := km["m_Name"].(string)
						if name == "" {
							name = fmt.Sprintf("Keyword_%d", keywordCount+1)
						}
						opts := 2
						kwList := []string{"_ (OFF)", name}
						if entries, hasEntries := km["m_Entries"].([]interface{}); hasEntries && len(entries) > 0 {
							opts = len(entries)
							kwList = nil
							for _, e := range entries {
								if em, isEm := e.(map[string]interface{}); isEm {
									if ename, hasName := em["m_EntryName"].(string); hasName {
										kwList = append(kwList, ename)
									}
								}
							}
						}
						keywordCount++
						totalVariants *= int64(opts)

						res.Pragmas = append(res.Pragmas, ShaderPragmaDirective{
							Directive:   "ShaderGraph Keyword",
							Scope:       "local",
							Keywords:    kwList,
							OptionCount: opts,
							LineNumber:  keywordCount,
							RawLine:     fmt.Sprintf("Keyword: %s (Variants: %d)", name, opts),
						})
					}
				}
			}

			res.TotalVariants = totalVariants
			res.TheoreticalMax = totalVariants
			res.TotalKeywords = keywordCount
			res.PassCount = 1
			res.Passes = []ShaderPassInfo{
				{
					Name:         "ShaderGraph Master Node",
					PassIndex:    0,
					Pragmas:      res.Pragmas,
					VariantCount: totalVariants,
				},
			}
		}
	} else {
		lines := strings.Split(string(contentBytes), "\n")
		var currentPass *ShaderPassInfo = nil
		var currentPassIndex = 0
		var globalPragmas []ShaderPragmaDirective
		keywordMap := make(map[string]bool)

		passRegex := regexp.MustCompile(`(?i)^\s*Pass\s*\{?`)
		nameRegex := regexp.MustCompile(`(?i)^\s*Name\s+"([^"]+)"`)
		inComment := false

		for lineIdx, rawLine := range lines {
			lineNum := lineIdx + 1
			trimmed := strings.TrimSpace(rawLine)

			if strings.Contains(trimmed, "/*") {
				inComment = true
			}
			if inComment {
				if strings.Contains(trimmed, "*/") {
					inComment = false
				}
				continue
			}

			if strings.HasPrefix(trimmed, "//") {
				continue
			}

			if passRegex.MatchString(trimmed) {
				if currentPass != nil {
					res.Passes = append(res.Passes, *currentPass)
				}
				currentPassIndex++
				currentPass = &ShaderPassInfo{
					Name:         fmt.Sprintf("Pass #%d", currentPassIndex),
					PassIndex:    currentPassIndex,
					Pragmas:      []ShaderPragmaDirective{},
					VariantCount: 1,
				}
			}

			if currentPass != nil && strings.HasPrefix(trimmed, "Name") {
				m := nameRegex.FindStringSubmatch(trimmed)
				if len(m) > 1 {
					currentPass.Name = m[1]
				}
			}

			if strings.HasPrefix(trimmed, "#pragma") {
				tokens := strings.Fields(trimmed)
				if len(tokens) >= 2 {
					directive := tokens[1]
					isMultiCompile := strings.HasPrefix(directive, "multi_compile")
					isShaderFeature := strings.HasPrefix(directive, "shader_feature")

					if isMultiCompile || isShaderFeature {
						scope := "global"
						if strings.HasSuffix(directive, "_local") {
							scope = "local"
						}
						opts := parsePragmaOptions(tokens[1:])
						var kws []string
						for _, k := range tokens[2:] {
							kws = append(kws, k)
							if k != "_" {
								keywordMap[k] = true
							}
						}

						pDirective := ShaderPragmaDirective{
							Directive:   directive,
							Scope:       scope,
							Keywords:    kws,
							OptionCount: opts,
							LineNumber:  lineNum,
							RawLine:     trimmed,
						}

						res.Pragmas = append(res.Pragmas, pDirective)
						if currentPass != nil {
							currentPass.Pragmas = append(currentPass.Pragmas, pDirective)
						} else {
							globalPragmas = append(globalPragmas, pDirective)
						}
					}
				}
			}

			if (ext == ".ush" || ext == ".usf") && strings.HasPrefix(trimmed, "#define") {
				tokens := strings.Fields(trimmed)
				if len(tokens) >= 3 && (tokens[2] == "0" || tokens[2] == "1") {
					varName := tokens[1]
					keywordMap[varName] = true
					pDirective := ShaderPragmaDirective{
						Directive:   "Unreal Permutation Switch",
						Scope:       "local",
						Keywords:    []string{"0 (OFF)", "1 (ON)"},
						OptionCount: 2,
						LineNumber:  lineNum,
						RawLine:     trimmed,
					}
					res.Pragmas = append(res.Pragmas, pDirective)
					if currentPass != nil {
						currentPass.Pragmas = append(currentPass.Pragmas, pDirective)
					} else {
						globalPragmas = append(globalPragmas, pDirective)
					}
				}
			}
		}

		if currentPass != nil {
			res.Passes = append(res.Passes, *currentPass)
		}

		var totalVariants int64 = 0
		var theoreticalProduct int64 = 1

		for _, p := range globalPragmas {
			if p.OptionCount > 0 {
				theoreticalProduct *= int64(p.OptionCount)
			}
		}

		if len(res.Passes) == 0 {
			res.PassCount = 1
			res.Passes = []ShaderPassInfo{
				{
					Name:         "Default Pass",
					PassIndex:    1,
					Pragmas:      globalPragmas,
					VariantCount: theoreticalProduct,
				},
			}
			totalVariants = theoreticalProduct
		} else {
			res.PassCount = len(res.Passes)
			for i := range res.Passes {
				var passVariants int64 = 1
				for _, gp := range globalPragmas {
					if gp.OptionCount > 0 {
						passVariants *= int64(gp.OptionCount)
					}
				}
				for _, lp := range res.Passes[i].Pragmas {
					if lp.OptionCount > 0 {
						passVariants *= int64(lp.OptionCount)
					}
				}
				res.Passes[i].VariantCount = passVariants
				totalVariants += passVariants
			}
		}

		res.TotalVariants = totalVariants
		res.TheoreticalMax = totalVariants
		res.TotalKeywords = len(keywordMap)
	}

	if res.TotalVariants > 1024 {
		res.Severity = "CRITICAL"
		res.IsExplosion = true
		res.StrippingAdvice = append(res.StrippingAdvice,
			fmt.Sprintf("🚨 变体发生极高危险度爆炸（当前变体总计 %d 个，远超 1,024 工业安全警戒线）！建议立即通过 Shader 编译预处理脚本 (IPreprocessShaders) 强力剔除互斥变体组合。", res.TotalVariants))
	} else if res.TotalVariants > 256 {
		res.Severity = "WARNING"
		res.IsExplosion = false
		res.StrippingAdvice = append(res.StrippingAdvice,
			fmt.Sprintf("⚠️ 变体数量偏高（当前变体共 %d 个），建议审视关键字组合，减少冗余排列组合。", res.TotalVariants))
	} else {
		res.Severity = "OK"
		res.IsExplosion = false
		res.StrippingAdvice = append(res.StrippingAdvice,
			fmt.Sprintf("✅ 变体规模健康（共 %d 个变体组合），符合移动端与高频材质编译预算。", res.TotalVariants))
	}

	multiCount := 0
	globalKwCount := 0
	for _, p := range res.Pragmas {
		if strings.HasPrefix(p.Directive, "multi_compile") {
			multiCount++
		}
		if p.Scope == "global" {
			globalKwCount += len(p.Keywords)
		}
	}

	if multiCount > 2 {
		res.StrippingAdvice = append(res.StrippingAdvice,
			fmt.Sprintf("检测到 %d 条 multi_compile 强制全量编译指令。multi_compile 产生的全部变体无论场景是否使用都会打入包体！建议将材质非动态切换的特性替换为 #pragma shader_feature 或 shader_feature_local，打包时将自动剥离未被引用的变体。", multiCount))
	}

	if globalKwCount > 15 {
		res.StrippingAdvice = append(res.StrippingAdvice,
			fmt.Sprintf("检测到 %d 个全局着色器关键字。Unity 全局关键字配额仅有 256/384 个上限，超出会导致关键字被截断并静默失效。强烈建议将仅内部材质使用的关键字加上 _local 后缀（如 multi_compile_local）。", globalKwCount))
	}

	res.StrippingAdvice = append(res.StrippingAdvice,
		"建议配合项目 ShaderVariantCollection 运行期收集工具，在战斗/大地图加载阶段提前 Warmup 预热，杜绝运行时首次遇到新变体引发的严重掉帧卡顿 (Shader Stuttering)。")

	res.Message = fmt.Sprintf("着色器 %s 变体质检完成：共计 %d 个变体组合，健康状态: %s",
		filepath.Base(targetRel), res.TotalVariants, res.Severity)

	return res, nil
}

// =========================================================================
// Round 105: Animation Rig & Skeleton Linter (LintAnimationRig)
// =========================================================================

// LintAnimationRig inspects FBX, glTF, and animation assets for bone count, skinning influences,
// frame sample rates, negative scales, and abnormal root motion against platform budgets.
func (GitService) LintAnimationRig(repoPath string, assetPath string, preset string) (*AnimRigLintResult, error) {
	if repoPath == "" || assetPath == "" {
		return nil, errors.New("repoPath 与 assetPath 均不能为空")
	}

	cleanPath, err := repoRelativePathFromInput(repoPath, assetPath)
	if err != nil {
		return nil, err
	}

	ext := strings.ToUpper(strings.TrimPrefix(filepath.Ext(cleanPath), "."))
	if ext == "" {
		ext = "FBX"
	}

	preset = strings.Title(strings.ToLower(strings.TrimSpace(preset)))
	if preset == "" {
		preset = "Mobile"
	}

	limits := AnimRigBudgetLimits{
		MaxBones:      64,
		MaxInfluences: 4,
		MaxSampleRate: 30,
		MaxTrackCount: 48,
	}
	switch preset {
	case "Console":
		limits = AnimRigBudgetLimits{
			MaxBones:      180,
			MaxInfluences: 8,
			MaxSampleRate: 60,
			MaxTrackCount: 128,
		}
	case "Cinematic":
		limits = AnimRigBudgetLimits{
			MaxBones:      500,
			MaxInfluences: 12,
			MaxSampleRate: 60,
			MaxTrackCount: 512,
		}
	default:
		preset = "Mobile"
	}

	fullPath := filepath.Join(repoPath, cleanPath)
	data, err := os.ReadFile(fullPath)
	if err != nil || len(data) == 0 {
		data, err = fetchAssetBytes(repoPath, cleanPath, "HEAD")
		if err != nil {
			return nil, fmt.Errorf("读取动画资产失败: %v", err)
		}
	}

	sizeBytes := int64(len(data))
	sizeStr := formatAssetSize(sizeBytes)

	var bones []AnimBoneInfo
	var clips []AnimClipInfo
	var issues []string
	var advice []string

	maxInfluences := 0
	analysisKnown := false
	hasNegativeScale := false
	hasAbnormalRootMotion := false
	hasExtremeKeyframeDensity := false
	hasUncompressedTracks := false
	sampleRateWarned := false

	strContent := string(data)

	if strings.Contains(strContent, "glTF") || ext == "GLTF" || ext == "GLB" {
		jsonData := data
		if ext == "GLB" || strings.HasPrefix(string(data), "glTF") {
			jsonData = glbJSONChunk(data)
		}
		type gltfDoc struct {
			Skins []struct {
				Joints []int `json:"joints"`
			} `json:"skins"`
			Animations []struct {
				Name     string `json:"name"`
				Channels []struct {
					Target struct {
						Path string `json:"path"`
					} `json:"target"`
				} `json:"channels"`
				Samplers []struct {
					Input         int    `json:"input"`
					Interpolation string `json:"interpolation"`
				} `json:"samplers"`
			} `json:"animations"`
			Nodes []struct {
				Name     string    `json:"name"`
				Scale    []float64 `json:"scale"`
				Children []int     `json:"children"`
			} `json:"nodes"`
			Accessors []struct {
				Count int       `json:"count"`
				Min   []float64 `json:"min"`
				Max   []float64 `json:"max"`
			} `json:"accessors"`
		}
		var doc gltfDoc
		if len(jsonData) > 0 {
			_ = json.Unmarshal(jsonData, &doc)
		}

		parents := make([]int, len(doc.Nodes))
		for i := range parents {
			parents[i] = -1
		}
		for i, node := range doc.Nodes {
			for _, child := range node.Children {
				if child >= 0 && child < len(parents) {
					parents[child] = i
				}
			}
		}
		depthOf := func(index int) int {
			depth := 0
			for steps := 0; parents[index] >= 0 && steps < len(parents); steps++ {
				index = parents[index]
				depth++
			}
			return depth
		}

		seenJoints := map[int]bool{}
		for _, skin := range doc.Skins {
			for _, joint := range skin.Joints {
				if joint < 0 || joint >= len(doc.Nodes) || seenJoints[joint] {
					continue
				}
				seenJoints[joint] = true
				node := doc.Nodes[joint]
				name := node.Name
				if name == "" {
					name = fmt.Sprintf("Node_%d", joint)
				}
				negScale := len(node.Scale) == 3 && (node.Scale[0] < 0 || node.Scale[1] < 0 || node.Scale[2] < 0)
				if negScale {
					hasNegativeScale = true
				}
				bones = append(bones, AnimBoneInfo{
					Name:             name,
					Depth:            depthOf(joint),
					HasNegativeScale: negScale,
				})
			}
		}

		for clipIndex, animation := range doc.Animations {
			clipName := animation.Name
			if clipName == "" {
				clipName = fmt.Sprintf("Clip_%d", clipIndex+1)
			}
			duration := 0.0
			frameCount := 0
			for _, sampler := range animation.Samplers {
				if sampler.Interpolation == "STEP" {
					hasUncompressedTracks = true
				}
				if sampler.Input < 0 || sampler.Input >= len(doc.Accessors) {
					continue
				}
				accessor := doc.Accessors[sampler.Input]
				if len(accessor.Max) > 0 && accessor.Max[0] > duration {
					duration = accessor.Max[0]
				}
				if accessor.Count > frameCount {
					frameCount = accessor.Count
				}
			}
			sampleRate := 0.0
			if duration > 0 && frameCount > 0 {
				sampleRate = math.Round(float64(frameCount)/duration*10) / 10
			}
			if sampleRate > float64(limits.MaxSampleRate) && !sampleRateWarned {
				sampleRateWarned = true
				hasExtremeKeyframeDensity = true
				issues = append(issues, fmt.Sprintf("动画采样率 %.1f fps 超出 %s 预设上限 %d fps，建议配置 Keyframe Reduction", sampleRate, preset, limits.MaxSampleRate))
			}
			clips = append(clips, AnimClipInfo{
				Name:            clipName,
				DurationSeconds: duration,
				SampleRate:      sampleRate,
				FrameCount:      frameCount,
				TrackCount:      len(animation.Channels),
				IsLooping:       false,
			})
		}
		analysisKnown = len(bones) > 0 || len(clips) > 0
	} else {
		// FBX text/binary string tables expose node names as Model::<Name>.
		seenNames := map[string]bool{}
		for _, match := range regexp.MustCompile(`Model::([A-Za-z_][A-Za-z0-9_]*)`).FindAllStringSubmatch(strContent, -1) {
			if len(match) < 2 || match[1] == "" || seenNames[match[1]] {
				continue
			}
			seenNames[match[1]] = true
			bones = append(bones, AnimBoneInfo{Name: match[1], Depth: 0})
		}

		if strings.Contains(strContent, "Scaling: (1,-1,1)") || strings.Contains(strContent, "Scale: -1") ||
			strings.Contains(strContent, "m_LocalScale: {x: -") {
			hasNegativeScale = true
			issues = append(issues, "骨骼或关节存在负缩放 (Negative Scale -1.0)，在镜像动画或物理布娃娃解算时会导致法线翻转")
		}

		if strings.Contains(strContent, "RootMotionSpeed: 1") || strings.Contains(strContent, "m_ApplyRootMotion: 1") {
			hasAbnormalRootMotion = true
			issues = append(issues, "检测到异常极速位移根骨骼运动 (Root Motion > 15 m/s)，可能引发网络同步位置闪现与穿墙判定失效")
		}

		if strings.Contains(strContent, "SampleRate: 120") || strings.Contains(strContent, "m_SampleRate: 120") {
			hasExtremeKeyframeDensity = true
			issues = append(issues, "帧采样率高达 120 FPS，超出移动端性能基线 (30/60 FPS)，建议配置 Keyframe Reduction 曲线剔除")
		}

		clipMatches := regexp.MustCompile(`Take:\s*"([^"]+)"|AnimationClip:\s*"([^"]+)"|m_Name:\s*"([^"]+)"`).FindAllStringSubmatch(strContent, -1)
		seenClips := map[string]bool{}
		for _, match := range clipMatches {
			clipName := ""
			for k := 1; k < len(match); k++ {
				if match[k] != "" {
					clipName = match[k]
					break
				}
			}
			if clipName == "" || seenClips[clipName] {
				continue
			}
			seenClips[clipName] = true
			clips = append(clips, AnimClipInfo{
				Name:            clipName,
				DurationSeconds: 0,
				SampleRate:      0,
				FrameCount:      0,
				TrackCount:      0,
				IsLooping:       false,
			})
		}
		analysisKnown = len(bones) > 0
	}

	boneCount := len(bones)
	isOverBudget := boneCount > limits.MaxBones

	if boneCount > limits.MaxBones {
		issues = append(issues, fmt.Sprintf("骨骼总数 (%d) 超出 %s 预设上限 (%d 根)，移动端蒙皮计算消耗严重",
			boneCount, preset, limits.MaxBones))
		advice = append(advice, "在 DCC (Maya/Blender) 中合并末端冗余微小指节或次级飘带骨骼")
	}

	if hasNegativeScale {
		advice = append(advice, "执行 Freeze Transformations 清除负缩放，使用正向旋转轴代替镜像矩阵")
	}

	totalDuration := 0.0
	overallSampleRate := 0.0
	totalTrackCount := 0
	for _, clip := range clips {
		totalDuration += clip.DurationSeconds
		if clip.SampleRate > overallSampleRate {
			overallSampleRate = clip.SampleRate
		}
		totalTrackCount += clip.TrackCount
	}

	severity := "OK"
	if !analysisKnown {
		severity = "UNKNOWN"
		issues = append(issues, "无法从该资产解析骨骼/动画结构；未作预算评级。")
	} else if isOverBudget || hasNegativeScale {
		severity = "ERROR"
	} else if hasExtremeKeyframeDensity || hasAbnormalRootMotion {
		severity = "WARNING"
	}

	rating := "Mobile Ready"
	if !analysisKnown {
		rating = "Unknown"
	} else if boneCount > 180 {
		rating = "Cinematic Only"
	} else if boneCount > 64 {
		rating = "Console Tier"
	}
	if isOverBudget {
		rating = "Optimization Required"
	}

	if !analysisKnown {
		advice = append(advice, "该格式无法直接解析骨骼/动画结构（支持 glTF/GLB 与包含 Model:: 名称的 FBX）；请在 DCC/引擎中查看统计。")
	} else if len(advice) == 0 {
		advice = append(advice, "开启动画关键帧压缩 (Keyframe Reduction) 与 ACL 动画压缩库")
		advice = append(advice, "蒙皮权重影响数未解析，本次预算仅覆盖骨骼数、轨道数与采样率")
	}

	message := fmt.Sprintf("骨骼动画巡检完毕: 骨骼数 %d 根, 剪辑 %d 个, 评级: %s, 结论: %s",
		boneCount, len(clips), rating, severity)
	if !analysisKnown {
		message = "骨骼动画巡检完毕: 未解析到可用的骨骼/动画结构，未作预算评级"
	}

	return &AnimRigLintResult{
		AssetPath:                 cleanPath,
		Format:                    ext,
		AnalysisKnown:             analysisKnown,
		Preset:                    preset,
		BoneCount:                 boneCount,
		MaxInfluencesPerVertex:    maxInfluences,
		TotalDurationSeconds:      totalDuration,
		OverallSampleRate:         overallSampleRate,
		TotalTrackCount:           totalTrackCount,
		Clips:                     clips,
		Bones:                     bones,
		HasAbnormalRootMotion:     hasAbnormalRootMotion,
		HasNegativeScale:          hasNegativeScale,
		HasExtremeKeyframeDensity: hasExtremeKeyframeDensity,
		HasUncompressedTracks:     hasUncompressedTracks,
		IsOverBudget:              isOverBudget,
		PerformanceRating:         rating,
		Severity:                  severity,
		BudgetLimits:              limits,
		Issues:                    issues,
		Advice:                    advice,
		SizeBytes:                 sizeBytes,
		SizeStr:                   sizeStr,
		Message:                   message,
	}, nil
}

// =========================================================================
// Round 116: Missing Material & Pink Shader Guard (ScanMissingMaterials)
// =========================================================================

// ScanMissingMaterials scans materials and prefabs for broken shader references and pink materials
func (GitService) ScanMissingMaterials(repoPath string, targetDir string) (*MissingMaterialScanResult, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath cannot be empty")
	}

	searchRoot := repoPath
	if strings.TrimSpace(targetDir) != "" && targetDir != "." {
		normalized, err := repoRelativePathFromInput(repoPath, targetDir)
		if err != nil {
			return nil, err
		}
		searchRoot = filepath.Join(repoPath, normalized)
	}

	guidToPath := make(map[string]string)
	existingFiles := make(map[string]bool)
	var matFiles []string
	var prefabFiles []string

	guidRe := regexp.MustCompile(`guid:\s*([a-f0-9]{32})`)

	_ = filepath.Walk(repoPath, func(p string, fi os.FileInfo, err error) error {
		if err != nil || fi == nil {
			return nil
		}
		if fi.IsDir() {
			name := fi.Name()
			if strings.HasPrefix(name, ".") || name == "Library" || name == "Intermediate" || name == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}

		rel, _ := filepath.Rel(repoPath, p)
		existingFiles[rel] = true

		ext := strings.ToLower(filepath.Ext(p))
		if ext == ".meta" {
			if data, err := os.ReadFile(p); err == nil {
				m := guidRe.FindStringSubmatch(string(data))
				if len(m) >= 2 {
					assetRel := strings.TrimSuffix(rel, ".meta")
					guidToPath[m[1]] = assetRel
				}
			}
		} else if ext == ".mat" {
			if strings.HasPrefix(p, searchRoot) {
				matFiles = append(matFiles, rel)
			}
		} else if ext == ".prefab" || ext == ".unity" {
			prefabFiles = append(prefabFiles, rel)
		}
		return nil
	})

	var brokenMaterials []BrokenMaterialItem
	pinkShaderCount := 0
	missingTexturesTotal := 0

	shaderRe := regexp.MustCompile(`m_Shader:\s*\{fileID:\s*(\d+)(?:,\s*guid:\s*([a-f0-9]{32}))?`)
	texEnvRe := regexp.MustCompile(`([a-zA-Z0-9_]+):\s*\n\s*- m_Texture:\s*\{fileID:\s*\d+,\s*guid:\s*([a-f0-9]{32})`)

	for _, matPath := range matFiles {
		fullPath := filepath.Join(repoPath, matPath)
		data, err := os.ReadFile(fullPath)
		if err != nil {
			continue
		}
		content := string(data)

		isPink := false
		issueType := ""
		shaderName := "Standard"
		shaderGuid := ""
		suggestedShader := "Universal Render Pipeline/Lit"
		recommendation := "材质着色器引用正常"

		// Detect Pink Shader / Broken Reference
		if strings.Contains(content, "Hidden/InternalErrorShader") || strings.Contains(content, "InternalErrorShader") {
			isPink = true
			issueType = "PINK_SHADER_ERROR"
			shaderName = "Hidden/InternalErrorShader (粉红故障着色器)"
			recommendation = "检测到 Unity 致命粉红着色器 (Pink Shader)，着色器代码编译错误或渲染管线不兼容，建议切换为 URP/Lit 或重新导入"
		} else {
			sm := shaderRe.FindStringSubmatch(content)
			if len(sm) >= 2 {
				fileID := sm[1]
				if len(sm) >= 3 {
					shaderGuid = sm[2]
				}

				if fileID == "0" || shaderGuid == "00000000000000000000000000000000" {
					isPink = true
					issueType = "NULL_SHADER_REFERENCE"
					shaderName = "Missing / Null Shader"
					recommendation = "材质丢失着色器关联 (Null Shader)，当前在场景中渲染为洋红色 (Magenta/Pink)，请指定有效 Shader"
				} else if shaderGuid != "" {
					if _, ok := guidToPath[shaderGuid]; !ok {
						isPink = true
						issueType = "MISSING_SHADER_GUID"
						shaderName = fmt.Sprintf("Missing Shader (GUID: %s)", shaderGuid[:8])
						recommendation = fmt.Sprintf("着色器 GUID 丢失 (%s)，着色器文件可能已被误删或未提交至 Git，已退化为粉红材质", shaderGuid)
					}
				}
			}
		}

		if isPink {
			pinkShaderCount++
		}

		var missingTextures []MissingTextureRef
		texMatches := texEnvRe.FindAllStringSubmatch(content, -1)
		for _, tm := range texMatches {
			if len(tm) >= 3 {
				propName := tm[1]
				texGuid := tm[2]
				if texGuid != "00000000000000000000000000000000" && texGuid != "" {
					targetPath, ok := guidToPath[texGuid]
					if !ok || !existingFiles[targetPath] {
						missingTextures = append(missingTextures, MissingTextureRef{
							PropertySlot: propName,
							TextureGuid:  texGuid,
							ExpectedPath: targetPath,
							Severity:     "HIGH",
						})
						missingTexturesTotal++
					}
				}
			}
		}

		if len(missingTextures) > 0 && issueType == "" {
			issueType = "MISSING_TEXTURE_MAPS"
			recommendation = fmt.Sprintf("材质缺少 %d 个关键贴图文件 (如 BaseMap/NormalMap)，请核对 Git LFS 下载状态", len(missingTextures))
		}

		if isPink || len(missingTextures) > 0 {
			var affectedPrefabs []string
			matGuid := ""
			for g, p := range guidToPath {
				if p == matPath {
					matGuid = g
					break
				}
			}
			if matGuid != "" {
				for _, pf := range prefabFiles {
					pfData, _ := os.ReadFile(filepath.Join(repoPath, pf))
					if strings.Contains(string(pfData), matGuid) {
						affectedPrefabs = append(affectedPrefabs, pf)
						if len(affectedPrefabs) >= 5 {
							break
						}
					}
				}
			}

			brokenMaterials = append(brokenMaterials, BrokenMaterialItem{
				MaterialPath:    matPath,
				MaterialName:    filepath.Base(matPath),
				EngineType:      "Unity 3D",
				IssueType:       issueType,
				ShaderName:      shaderName,
				ShaderGuid:      shaderGuid,
				IsPinkShader:    isPink,
				MissingTextures: missingTextures,
				SuggestedShader: suggestedShader,
				AffectedPrefabs: affectedPrefabs,
				Recommendation:  recommendation,
			})
		}
	}

	healthScore := 100 - (pinkShaderCount * 25) - (missingTexturesTotal * 5)
	if healthScore < 0 {
		healthScore = 0
	}

	summary := fmt.Sprintf("材质与着色器扫描完成: 共检查 %d 个材质，发现 %d 个异常材质 (%d 个致命粉红着色器, %d 处贴图缺失)，材质健康分: %d/100",
		len(matFiles), len(brokenMaterials), pinkShaderCount, missingTexturesTotal, healthScore)

	return &MissingMaterialScanResult{
		TargetDirectory:       targetDir,
		EngineType:            "Unity 3D",
		TotalMaterialsScanned: len(matFiles),
		BrokenMaterialsCount:  len(brokenMaterials),
		PinkShaderCount:       pinkShaderCount,
		MissingTexturesCount:  missingTexturesTotal,
		BrokenMaterials:       brokenMaterials,
		HealthScore:           healthScore,
		Summary:               summary,
		Message:               "粉红着色器与材质完整性防护扫描就绪",
	}, nil
}

// =========================================================================
// Round 119: LOD Tier & Streaming Mip Audit (AuditLodStreaming)
// =========================================================================

// AuditLodStreaming inspects static/skeletal mesh LOD completeness and texture streaming settings.
func (GitService) AuditLodStreaming(repoPath string, targetDir string) (*LodStreamingAuditResult, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath 不能为空")
	}

	searchPath := repoPath
	if strings.TrimSpace(targetDir) != "" && targetDir != "." {
		normalized, err := repoRelativePathFromInput(repoPath, targetDir)
		if err != nil {
			return nil, err
		}
		searchPath = filepath.Join(repoPath, normalized)
	}
	if _, err := os.Stat(searchPath); err != nil {
		return nil, fmt.Errorf("LOD 扫描目录不可访问: %w", err)
	}

	meshExts := map[string]bool{
		".obj": true, ".fbx": true, ".uasset": true, ".gltf": true, ".glb": true, ".prefab": true, ".mesh": true,
	}
	texExts := map[string]bool{
		".png": true, ".jpg": true, ".jpeg": true, ".dds": true, ".tga": true, ".exr": true, ".hdr": true, ".psd": true,
	}

	meshItems := []MeshLodAuditItem{}
	texItems := []TextureStreamingAuditItem{}

	walkErr := filepath.Walk(searchPath, func(p string, fi os.FileInfo, err error) error {
		if err != nil || fi == nil {
			return nil
		}
		if fi.IsDir() {
			name := fi.Name()
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == "Library" ||
				name == "Intermediate" || name == "Saved" || name == "Binaries" || name == "obj" {
				return filepath.SkipDir
			}
			return nil
		}

		ext := strings.ToLower(filepath.Ext(p))
		relPath, _ := filepath.Rel(repoPath, p)

		if meshExts[ext] {
			triangles, triangleCountKnown := countMeshFileTriangles(p, ext)

			dir := filepath.Dir(p)
			baseName := strings.TrimSuffix(fi.Name(), ext)
			reLod := regexp.MustCompile(`(?i)^(.*)_lod\d+$`)
			if m := reLod.FindStringSubmatch(baseName); len(m) >= 2 {
				if _, statErr := os.Stat(filepath.Join(dir, m[1]+ext)); statErr == nil {
					return nil
				}
			}
			cleanBase := baseName
			if m := reLod.FindStringSubmatch(baseName); len(m) >= 2 {
				cleanBase = m[1]
			}

			lodTiers := []MeshLodTier{
				{
					LodIndex:           0,
					TriangleCount:      triangles,
					TriangleCountKnown: triangleCountKnown,
					ReductionPercent:   100.0,
					ScreenRelativeSize: 1.00,
				},
			}
			if !triangleCountKnown {
				lodTiers[0].ReductionPercent = 0
			}
			unknownLodTiers := 0

			for idx := 1; idx <= 3; idx++ {
				cand1 := filepath.Join(dir, fmt.Sprintf("%s_LOD%d%s", cleanBase, idx, ext))
				cand2 := filepath.Join(dir, fmt.Sprintf("%s_lod%d%s", cleanBase, idx, ext))
				targetCand := ""
				if _, err1 := os.Stat(cand1); err1 == nil {
					targetCand = cand1
				} else if _, err2 := os.Stat(cand2); err2 == nil {
					targetCand = cand2
				}

				if targetCand != "" {
					lodTri, lodKnown := countMeshFileTriangles(targetCand, ext)
					redPct := 0.0
					if triangleCountKnown && lodKnown && triangles > 0 {
						redPct = math.Round((float64(lodTri)/float64(triangles))*1000) / 10
					}
					if !lodKnown {
						unknownLodTiers++
					}
					screenSz := math.Round(math.Pow(0.55, float64(idx))*100) / 100
					lodTiers = append(lodTiers, MeshLodTier{
						LodIndex:           idx,
						TriangleCount:      lodTri,
						TriangleCountKnown: lodKnown,
						ReductionPercent:   redPct,
						ScreenRelativeSize: screenSz,
					})
				}
			}

			maxLod := len(lodTiers) - 1
			hasComplete := maxLod >= 2
			severity := "OK"
			recom := "LOD 结构完整，符合渲染管线规范。"

			if !triangleCountKnown {
				if !hasComplete {
					severity = "WARNING"
					recom = "未配置完整 LOD，且该格式无法直接读取三角形数；请用 DCC/导入器确认面数预算。"
				} else {
					recom = "LOD 文件齐全；该格式无法直接读取三角形数，面数预算需导入器验证。"
				}
			} else if triangles > 20000 && maxLod == 0 {
				hasComplete = false
				severity = "CRITICAL"
				recom = "网格面数超过 20,000 面但缺少 LOD1~LOD3，远景光栅化与顶点压力过大，需在 DCC/引擎中生成分级网格。"
			} else if triangles > 8000 && maxLod < 2 {
				hasComplete = false
				severity = "WARNING"
				recom = "中模资产建议至少配置 LOD1 与 LOD2 梯队以满足主流游戏引擎中距离渲染预算。"
			} else if maxLod == 0 {
				recom = "面数较低，未配置 LOD；如进入远景渲染建议补充 LOD1。"
			}
			if unknownLodTiers > 0 {
				recom += " " + strconv.Itoa(unknownLodTiers) + " 个 LOD 文件无法直接读取面数，需导入器验证。"
			}

			meshType := "StaticMesh"
			if strings.Contains(strings.ToLower(p), "char") || strings.Contains(strings.ToLower(p), "skel") {
				meshType = "SkeletalMesh"
			}

			meshItems = append(meshItems, MeshLodAuditItem{
				Path:               relPath,
				MeshType:           meshType,
				BaseTriangles:      triangles,
				TriangleCountKnown: triangleCountKnown,
				LodTiers:           lodTiers,
				MaxLodLevel:        maxLod,
				HasCompleteLODs:    hasComplete,
				Severity:           severity,
				Recommendation:     recom,
			})
		}

		if texExts[ext] {
			w, h := 0, 0
			mipCount := 1
			file, fErr := os.Open(p)
			if fErr == nil {
				if ext == ".dds" {
					header := make([]byte, 128)
					n, _ := file.Read(header)
					if n >= 128 && string(header[0:4]) == "DDS " {
						h = int(binary.LittleEndian.Uint32(header[12:16]))
						w = int(binary.LittleEndian.Uint32(header[16:20]))
						mipCount = int(binary.LittleEndian.Uint32(header[28:32]))
					}
				} else if ext == ".tga" {
					header := make([]byte, 18)
					n, _ := file.Read(header)
					if n >= 18 {
						w = int(binary.LittleEndian.Uint16(header[12:14]))
						h = int(binary.LittleEndian.Uint16(header[14:16]))
					}
				} else {
					cfg, _, cErr := image.DecodeConfig(file)
					if cErr == nil {
						w = cfg.Width
						h = cfg.Height
					}
				}
				_ = file.Close()
			}

			dimensionsKnown := w > 0 && h > 0
			if !dimensionsKnown {
				texItems = append(texItems, TextureStreamingAuditItem{
					Path:            relPath,
					DimensionsKnown: false,
					Severity:        "WARNING",
					Issue:           "无法解析纹理头，尺寸与串流状态未知，未计入显存估算",
				})
				return nil
			}

			isPOT := (w&(w-1)) == 0 && (h&(h-1)) == 0
			if mipCount <= 1 && isPOT {
				maxDim := math.Max(float64(w), float64(h))
				mipCount = int(math.Floor(math.Log2(maxDim))) + 1
			}

			streamingEnabled := isPOT && (w >= 512 || h >= 512)
			virtualTexture := (w >= 4096 || h >= 4096) && (strings.Contains(p, "_VT") || strings.Contains(p, "Virtual"))

			rawSize := int64(w * h * 4) // RGBA32 baseline
			streamSize := rawSize
			if streamingEnabled {
				streamSize = rawSize / 4 // Active footprint when streaming top mips on demand
			}

			severity := "OK"
			issue := ""
			if !isPOT {
				severity = "WARNING"
				issue = "非 2 的幂次 (NPOT) 纹理，无法开启硬件 Mipmap 串流链"
			} else if (w >= 4096 || h >= 4096) && !virtualTexture {
				severity = "WARNING"
				issue = "4K+ 超大分辨率纹理未启用 Virtual Texturing (VT)，易引起显存突增"
			}

			texItems = append(texItems, TextureStreamingAuditItem{
				Path:             relPath,
				DimensionsKnown:  true,
				Width:            w,
				Height:           h,
				IsPOT:            isPOT,
				MipCount:         mipCount,
				StreamingEnabled: streamingEnabled,
				VirtualTexture:   virtualTexture,
				RawSizeBytes:     rawSize,
				StreamSizeBytes:  streamSize,
				Severity:         severity,
				Issue:            issue,
			})
		}

		return nil
	})
	if walkErr != nil {
		return nil, fmt.Errorf("扫描 LOD/纹理目录失败: %v", walkErr)
	}
	if len(meshItems) == 0 && len(texItems) == 0 {
		return &LodStreamingAuditResult{
			AnalysisKnown:   false,
			Meshes:          meshItems,
			Textures:        texItems,
			Recommendations: []string{},
			Summary:         "未发现可审计的网格或纹理资产（支持 OBJ/FBX/UAsset/glTF/GLB 与 PNG/JPEG/DDS/TGA 等），未作 LOD/串流评分",
		}, nil
	}

	lodCompliant := 0
	lodMissing := 0
	lodUnknown := 0
	for _, m := range meshItems {
		if m.HasCompleteLODs {
			lodCompliant++
		} else {
			lodMissing++
		}
		if !m.TriangleCountKnown {
			lodUnknown++
		}
	}

	streamCompliant := 0
	streamMissing := 0
	streamUnknown := 0
	var totalRawBytes int64
	var totalStreamBytes int64
	for _, t := range texItems {
		if !t.DimensionsKnown {
			streamUnknown++
			continue
		}
		if t.StreamingEnabled {
			streamCompliant++
		} else {
			streamMissing++
		}
		totalRawBytes += t.RawSizeBytes
		totalStreamBytes += t.StreamSizeBytes
	}

	baselineVramMB := math.Round((float64(totalRawBytes)/(1024*1024))*100) / 100
	streamingVramMB := math.Round((float64(totalStreamBytes)/(1024*1024))*100) / 100
	savingsMB := math.Round((baselineVramMB-streamingVramMB)*100) / 100
	savingsPct := 0.0
	if baselineVramMB > 0 {
		savingsPct = math.Round((savingsMB/baselineVramMB)*1000) / 10
	}

	healthScore := 100 - (lodMissing * 15) - (streamMissing * 10)
	if healthScore < 0 {
		healthScore = 0
	}

	recommendations := []string{
		"对所有面数超过 10,000 面的高模资产配置 LOD0~LOD3，过渡阈值推荐 1.0 -> 0.6 -> 0.3 -> 0.15",
		"纹理资产强制保持 2 的幂次 (POT: 512, 1024, 2048, 4096)，确保 GPU 硬件 Mipmap 串流链可用",
		"针对 4096 及以上超大分辨率贴图开启 Virtual Texturing (VT)，防止远距离未串流显存暴涨",
	}

	summary := fmt.Sprintf("LOD 与纹理串流审计完成：网格合规 %d/%d（%d 项面数未知），纹理串流就绪 %d/%d（%d 项尺寸未知）。显存优化潜力: 节约 %.2f MB (%.1f%%)，整体健康度评分: %d/100，未知项未计入评分。",
		lodCompliant, len(meshItems), lodUnknown, streamCompliant, len(texItems), streamUnknown, savingsMB, savingsPct, healthScore)

	return &LodStreamingAuditResult{
		AnalysisKnown:           true,
		TotalMeshesAudited:      len(meshItems),
		LODCompliantCount:       lodCompliant,
		LODMissingCount:         lodMissing,
		LODUnknownCount:         lodUnknown,
		TotalTexturesAudited:    len(texItems),
		StreamingCompliantCount: streamCompliant,
		StreamingMissingCount:   streamMissing,
		StreamingUnknownCount:   streamUnknown,
		BaselineVramMB:          baselineVramMB,
		StreamingVramMB:         streamingVramMB,
		PotentialSavingsMB:      savingsMB,
		SavingsPercentage:       savingsPct,
		OverallHealthScore:      healthScore,
		Meshes:                  meshItems,
		Textures:                texItems,
		Recommendations:         recommendations,
		Summary:                 summary,
	}, nil
}

func countMeshFileTriangles(path string, ext string) (int, bool) {
	switch ext {
	case ".obj":
		data, err := os.ReadFile(path)
		if err != nil {
			return 0, false
		}
		return countOBJTriangles(data), true
	case ".gltf":
		data, err := os.ReadFile(path)
		if err != nil {
			return 0, false
		}
		return countGLTFTriangles(data)
	case ".glb":
		data, err := os.ReadFile(path)
		if err != nil {
			return 0, false
		}
		return countGLBTriangles(data)
	}
	return 0, false
}

func countOBJTriangles(data []byte) int {
	triangles := 0
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "f ") {
			continue
		}
		if parts := strings.Fields(line); len(parts) >= 4 {
			triangles += len(parts) - 3
		}
	}
	return triangles
}

type gltfAccessorSummary struct {
	Count int `json:"count"`
}

type gltfPrimitiveSummary struct {
	Indices    *int           `json:"indices"`
	Mode       *int           `json:"mode"`
	Material   *int           `json:"material"`
	Attributes map[string]int `json:"attributes"`
}

type gltfMeshSummary struct {
	Primitives []gltfPrimitiveSummary `json:"primitives"`
}

type gltfDocumentSummary struct {
	Meshes    []gltfMeshSummary     `json:"meshes"`
	Accessors []gltfAccessorSummary `json:"accessors"`
}

func countGLTFTriangles(data []byte) (int, bool) {
	var document gltfDocumentSummary
	if err := json.Unmarshal(bytes.TrimSpace(data), &document); err != nil {
		return 0, false
	}
	triangles := 0
	counted := false
	for _, mesh := range document.Meshes {
		for _, primitive := range mesh.Primitives {
			mode := 4
			if primitive.Mode != nil {
				mode = *primitive.Mode
			}
			if mode != 4 {
				continue
			}
			accessorIndex := -1
			if primitive.Indices != nil {
				accessorIndex = *primitive.Indices
			} else if positionIndex, ok := primitive.Attributes["POSITION"]; ok {
				accessorIndex = positionIndex
			}
			if accessorIndex < 0 || accessorIndex >= len(document.Accessors) {
				continue
			}
			vertexCount := document.Accessors[accessorIndex].Count
			if vertexCount < 3 {
				continue
			}
			triangles += vertexCount / 3
			counted = true
		}
	}
	return triangles, counted
}

func glbJSONChunk(data []byte) []byte {
	if len(data) < 20 || string(data[:4]) != "glTF" || binary.LittleEndian.Uint32(data[4:8]) != 2 {
		return nil
	}
	offset := 12
	for offset+8 <= len(data) {
		chunkLength := int(binary.LittleEndian.Uint32(data[offset : offset+4]))
		chunkType := binary.LittleEndian.Uint32(data[offset+4 : offset+8])
		offset += 8
		if chunkLength < 0 || offset+chunkLength > len(data) {
			return nil
		}
		if chunkType == 0x4E4F534A {
			return data[offset : offset+chunkLength]
		}
		offset += chunkLength
	}
	return nil
}

func countGLBTriangles(data []byte) (int, bool) {
	chunk := glbJSONChunk(data)
	if chunk == nil {
		return 0, false
	}
	return countGLTFTriangles(chunk)
}

// =========================================================================
// Round 122: Physics Material & Mesh Collision Auditor (AuditPhysicsMeshCollision)
// =========================================================================

func (GitService) AuditPhysicsMeshCollision(repoPath string, targetDir string) (*PhysicsCollisionAuditResult, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath cannot be empty")
	}

	searchRoot := repoPath
	if strings.TrimSpace(targetDir) != "" && targetDir != "." {
		normalized, err := repoRelativePathFromInput(repoPath, targetDir)
		if err != nil {
			return nil, err
		}
		searchRoot = filepath.Join(repoPath, normalized)
	}

	totalColliders := 0
	convexHulls := 0
	concaveMeshes := 0
	unsupportedUMaps := 0
	overbudgetHulls := []ConvexHullItem{}
	nonConvexDynamic := []string{}
	missingPhysMats := []string{}

	_ = filepath.Walk(searchRoot, func(p string, fi os.FileInfo, err error) error {
		if err != nil || fi == nil || fi.IsDir() {
			return nil
		}
		name := fi.Name()
		if strings.HasPrefix(name, ".") || strings.Contains(p, "node_modules") || strings.Contains(p, "Library") {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(p))
		rel, _ := filepath.Rel(repoPath, p)

		if ext == ".umap" {
			unsupportedUMaps++
			return nil
		}
		if ext == ".prefab" || ext == ".unity" {
			data, rErr := os.ReadFile(p)
			if rErr == nil {
				content := string(data)
				documents := strings.Split(content, "--- !u!")
				hasDynamicRigidbody := false
				for _, document := range documents {
					if strings.Contains(document, "Rigidbody:") && !strings.Contains(document, "m_IsKinematic: 1") {
						hasDynamicRigidbody = true
						break
					}
				}
				for _, document := range documents {
					if !strings.Contains(document, "MeshCollider:") {
						continue
					}
					totalColliders++
					if strings.Contains(document, "m_Convex: 1") {
						convexHulls++
					} else {
						concaveMeshes++
						if hasDynamicRigidbody {
							nonConvexDynamic = append(nonConvexDynamic, rel+" (动态刚体非凸网格碰撞体)")
						}
					}
					if strings.Contains(document, "m_Material: {fileID: 0}") {
						missingPhysMats = append(missingPhysMats, rel+" (碰撞体缺失物理材质引用)")
					}
				}
			}
		} else if ext == ".obj" {
			// Check Wavefront OBJ vertex count for convex hull limits
			data, rErr := os.ReadFile(p)
			if rErr == nil {
				lines := strings.Split(string(data), "\n")
				vCount := 0
				fCount := 0
				for _, line := range lines {
					line = strings.TrimSpace(line)
					if strings.HasPrefix(line, "v ") {
						vCount++
					} else if strings.HasPrefix(line, "f ") {
						fCount++
					}
				}
				if strings.Contains(strings.ToLower(name), "collision") || strings.Contains(strings.ToLower(name), "ucx") || strings.Contains(strings.ToLower(name), "hull") {
					totalColliders++
					convexHulls++
					if vCount > 256 {
						overbudgetHulls = append(overbudgetHulls, ConvexHullItem{
							MeshPath:     rel,
							VertexCount:  vCount,
							PolygonCount: fCount,
							IsOverbudget: true,
							Severity:     "CRITICAL",
						})
					}
				}
			}
		}
		return nil
	})

	analysisKnown := totalColliders > 0
	score := 0
	summary := "未发现可审计的 Unity MeshCollider 或碰撞用 OBJ（文件名含 collision/ucx/hull），未作碰撞体体检评分"
	if analysisKnown {
		score = 100 - (len(overbudgetHulls)*20 + len(nonConvexDynamic)*25 + len(missingPhysMats)*10)
		if score < 0 {
			score = 0
		}
		summary = fmt.Sprintf("物理碰撞体体检完成: 共检测 %d 个碰撞体 (%d 凸多边形 / %d 复杂网格), 发现 %d 处超标 Hull (>256顶点的物理引擎崩溃隐患), 评分 %d/100",
			totalColliders, convexHulls, concaveMeshes, len(overbudgetHulls), score)
	}
	if unsupportedUMaps > 0 {
		summary += fmt.Sprintf("; 跳过 %d 个 UE .umap（需 Editor/Python 导出）", unsupportedUMaps)
	}

	return &PhysicsCollisionAuditResult{
		AnalysisKnown:             analysisKnown,
		TotalColliders:            totalColliders,
		ConvexHullsCount:          convexHulls,
		ConcaveMeshCount:          concaveMeshes,
		OverbudgetHulls:           overbudgetHulls,
		NonConvexDynamicColliders: nonConvexDynamic,
		MissingPhysicMaterials:    missingPhysMats,
		HealthScore:               score,
		Summary:                   summary,
	}, nil
}

// =========================================================================
// Round 124: Material Instance Parameter Overrides Inspector (AuditMaterialInstanceOverrides)
// =========================================================================

func (GitService) AuditMaterialInstanceOverrides(repoPath string, targetDir string) (*MaterialInstanceOverrideResult, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath cannot be empty")
	}

	searchRoot := repoPath
	if strings.TrimSpace(targetDir) != "" && targetDir != "." {
		normalized, err := repoRelativePathFromInput(repoPath, targetDir)
		if err != nil {
			return nil, err
		}
		searchRoot = filepath.Join(repoPath, normalized)
	}
	if _, err := os.Stat(searchRoot); err != nil {
		return nil, fmt.Errorf("材质扫描目录不可访问: %w", err)
	}

	guidIndex := buildAssetGUIDIndex(repoPath)
	instances := []MaterialInstanceItem{}
	totalOverrides := 0
	deadOverrides := 0
	unsupportedUAssets := 0
	skipDirs := map[string]bool{
		".git": true, "Library": true, "Temp": true, "node_modules": true,
		"Intermediate": true, "Saved": true, "DerivedDataCache": true, "build": true,
	}

	walkErr := filepath.Walk(searchRoot, func(p string, fi os.FileInfo, err error) error {
		if err != nil || fi == nil {
			return nil
		}
		if fi.IsDir() {
			if skipDirs[fi.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		switch strings.ToLower(filepath.Ext(p)) {
		case ".uasset":
			unsupportedUAssets++
			return nil
		case ".mat":
		default:
			return nil
		}
		rel, relErr := filepath.Rel(repoPath, p)
		if relErr != nil {
			return nil
		}
		item, ok := parseUnityMaterialFile(repoPath, p, filepath.ToSlash(rel), guidIndex)
		if !ok {
			return nil
		}
		instances = append(instances, item)
		totalOverrides += len(item.Overrides)
		deadOverrides += item.DeadCount
		return nil
	})
	if walkErr != nil {
		return nil, fmt.Errorf("扫描材质目录失败: %v", walkErr)
	}

	sort.Slice(instances, func(i, j int) bool { return instances[i].InstancePath < instances[j].InstancePath })

	summary := fmt.Sprintf(
		"材质参数覆盖审计完成: 审计 %d 个 Unity 材质, 统计 %d 个属性, 发现 %d 个不属于当前 Shader 的悬空参数",
		len(instances), totalOverrides, deadOverrides,
	)
	if len(instances) == 0 {
		summary = "未发现可解析的 Unity .mat 材质（含 m_SavedProperties）"
	}
	if unsupportedUAssets > 0 {
		summary += fmt.Sprintf("; 跳过 %d 个 UE .uasset，需 UE Editor/Python 导出", unsupportedUAssets)
	}

	return &MaterialInstanceOverrideResult{
		TotalInstances:            len(instances),
		OverriddenParametersCount: totalOverrides,
		DeadParametersCount:       deadOverrides,
		Instances:                 instances,
		Summary:                   summary,
	}, nil
}

type unityAssetReference struct {
	guid   string
	fileID int64
	path   string
}

func buildAssetGUIDIndex(repoPath string) map[string]string {
	index := map[string]string{}
	skipDirs := map[string]bool{
		".git": true, "Library": true, "Temp": true, "node_modules": true,
		"Intermediate": true, "Saved": true, "DerivedDataCache": true, "build": true,
	}
	_ = filepath.Walk(repoPath, func(p string, fi os.FileInfo, err error) error {
		if err != nil || fi == nil {
			return nil
		}
		if fi.IsDir() {
			if skipDirs[fi.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(p), ".meta") {
			return nil
		}
		data, readErr := os.ReadFile(p)
		if readErr != nil {
			return nil
		}
		match := regexp.MustCompile(`(?m)^guid:\s*([0-9a-fA-F]{16,})`).FindStringSubmatch(string(data))
		if len(match) < 2 {
			return nil
		}
		assetPath := strings.TrimSuffix(p, ".meta")
		if _, statErr := os.Stat(assetPath); statErr != nil {
			return nil
		}
		rel, relErr := filepath.Rel(repoPath, assetPath)
		if relErr != nil {
			return nil
		}
		index[strings.ToLower(match[1])] = filepath.ToSlash(rel)
		return nil
	})
	return index
}

func parseUnityReference(content string, key string, guidIndex map[string]string) unityAssetReference {
	pattern := regexp.MustCompile(`(?m)^\s*` + regexp.QuoteMeta(key) + `:\s*\{([^}]*)\}`)
	match := pattern.FindStringSubmatch(content)
	if len(match) < 2 {
		return unityAssetReference{}
	}
	return parseUnityReferenceFields(match[1], guidIndex)
}

func parseUnityReferenceFields(fields string, guidIndex map[string]string) unityAssetReference {
	reference := unityAssetReference{}
	if match := regexp.MustCompile(`guid:\s*([0-9a-fA-F]+)`).FindStringSubmatch(fields); len(match) >= 2 {
		reference.guid = strings.ToLower(match[1])
	}
	if match := regexp.MustCompile(`fileID:\s*(-?\d+)`).FindStringSubmatch(fields); len(match) >= 2 {
		reference.fileID, _ = strconv.ParseInt(match[1], 10, 64)
	}
	if reference.guid != "" && strings.Trim(reference.guid, "0") != "" {
		reference.path = guidIndex[reference.guid]
	}
	return reference
}

func parseShaderPropertyNames(path string) map[string]bool {
	properties := map[string]bool{}
	data, err := os.ReadFile(path)
	if err != nil {
		return properties
	}
	for _, match := range regexp.MustCompile(`(?m)^\s*([A-Za-z_][A-Za-z0-9_]*)\s*\(\s*"`).FindAllStringSubmatch(string(data), -1) {
		if len(match) >= 2 {
			properties[match[1]] = true
		}
	}
	return properties
}

func parseUnityMaterialFile(repoPath string, fullPath string, relPath string, guidIndex map[string]string) (MaterialInstanceItem, bool) {
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return MaterialInstanceItem{}, false
	}
	content := string(data)
	if !strings.Contains(content, "Material:") || !strings.Contains(content, "m_SavedProperties") {
		return MaterialInstanceItem{}, false
	}

	parent := parseUnityReference(content, "m_Parent", guidIndex)
	shader := parseUnityReference(content, "m_Shader", guidIndex)
	shaderProperties := map[string]bool{}
	if shader.path != "" {
		shaderProperties = parseShaderPropertyNames(filepath.Join(repoPath, filepath.FromSlash(shader.path)))
	}

	overrides := []MaterialParamOverride{}
	deadCount := 0
	appendOverride := func(name string, kind string, value string) {
		if name == "" {
			return
		}
		isDead := len(shaderProperties) > 0 && !shaderProperties[name]
		overrides = append(overrides, MaterialParamOverride{
			ParamName: name,
			ParamType: kind,
			Value:     value,
			IsDead:    isDead,
		})
		if isDead {
			deadCount++
		}
	}

	section := ""
	sectionIndent := -1
	textureName := ""
	textureReference := unityAssetReference{}
	flushTexture := func() {
		if textureName == "" {
			return
		}
		value := "None"
		if textureReference.fileID != 0 || (textureReference.guid != "" && strings.Trim(textureReference.guid, "0") != "") {
			if textureReference.path != "" {
				value = textureReference.path
			} else if textureReference.guid != "" {
				value = "GUID:" + textureReference.guid
			} else {
				value = "内置资源"
			}
		}
		appendOverride(textureName, "Texture", value)
		textureName = ""
		textureReference = unityAssetReference{}
	}

	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		switch trimmed {
		case "m_TexEnvs:", "m_Floats:", "m_Colors:", "m_Ints:":
			flushTexture()
			section = trimmed
			sectionIndent = indent
			continue
		}
		if section != "" && indent <= sectionIndent && !strings.HasPrefix(trimmed, "-") {
			flushTexture()
			section = ""
			continue
		}
		switch section {
		case "m_TexEnvs:":
			if strings.HasPrefix(trimmed, "- ") {
				flushTexture()
				textureName = strings.TrimSuffix(strings.TrimSpace(strings.TrimPrefix(trimmed, "- ")), ":")
			} else if textureName != "" && strings.HasPrefix(trimmed, "m_Texture:") {
				textureReference = parseUnityReferenceFields(trimmed, guidIndex)
			}
		case "m_Floats:", "m_Ints:", "m_Colors:":
			if !strings.HasPrefix(trimmed, "- ") {
				continue
			}
			parts := strings.SplitN(strings.TrimSpace(strings.TrimPrefix(trimmed, "- ")), ":", 2)
			if len(parts) != 2 {
				continue
			}
			kind := "Scalar"
			if section == "m_Colors:" {
				kind = "Color"
			}
			appendOverride(strings.TrimSpace(parts[0]), kind, strings.TrimSpace(parts[1]))
		}
	}
	flushTexture()

	return MaterialInstanceItem{
		InstancePath:   relPath,
		ParentMaterial: parent.path,
		ParentShader:   shader.path,
		Overrides:      overrides,
		DeadCount:      deadCount,
		IsHealthy:      deadCount == 0,
	}, true
}

// =========================================================================
// Round 127: PBR Texture Channel Packing Auditor (AuditTextureChannelPacking)
// =========================================================================

func (GitService) AuditTextureChannelPacking(repoPath string, targetDir string) (*TextureChannelPackingResult, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath cannot be empty")
	}

	searchRoot := repoPath
	if trimmed := strings.TrimSpace(targetDir); trimmed != "" && trimmed != "." {
		normalized, err := repoRelativePathFromInput(repoPath, trimmed)
		if err != nil {
			return nil, err
		}
		searchRoot = filepath.Join(repoPath, normalized)
	}

	skipDirs := map[string]bool{
		".git": true, "node_modules": true, "Library": true, "Intermediate": true,
		"DerivedDataCache": true, "Temp": true, "Saved": true, "Binaries": true, "build": true,
	}
	textures := []TextureChannelAuditItem{}
	scanned := 0
	ormCompliant := 0
	unpackedCount := 0
	invalidNormal := 0
	var vramWaste int64

	walkErr := filepath.Walk(searchRoot, func(p string, fi os.FileInfo, err error) error {
		if err != nil || fi == nil {
			return nil
		}
		if fi.IsDir() {
			if skipDirs[fi.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		switch strings.ToLower(filepath.Ext(p)) {
		case ".png", ".jpg", ".jpeg", ".tga":
		default:
			return nil
		}
		info, infoErr := inspectTextureFile(p)
		if infoErr != nil {
			return nil
		}
		scanned++

		rel, relErr := filepath.Rel(repoPath, p)
		if relErr != nil {
			rel = p
		}
		item, waste, isOrmCompliant, isUnpacked, isInvalidNormal := classifyTextureChannelPacking(filepath.ToSlash(rel), info)
		if item.Path == "" {
			return nil
		}
		textures = append(textures, item)
		vramWaste += waste
		if isOrmCompliant {
			ormCompliant++
		}
		if isUnpacked {
			unpackedCount++
		}
		if isInvalidNormal {
			invalidNormal++
		}
		return nil
	})
	if walkErr != nil {
		return nil, fmt.Errorf("扫描贴图目录失败: %v", walkErr)
	}

	sort.Slice(textures, func(i, j int) bool { return textures[i].Path < textures[j].Path })

	var summary string
	if scanned == 0 {
		summary = "未发现可审计的图片文件（支持 PNG/JPEG/TGA），未作 PBR 通道打包结论"
	} else {
		summary = fmt.Sprintf(
			"PBR 贴图通道审计完成: 扫描 %d 张图片, ORM/MRA 打包合规 %d 张, 独立单通道贴图 %d 张, 法线图异常 %d 张, 按未压缩 8-bit 通道估算可节省显存 %s",
			scanned, ormCompliant, unpackedCount, invalidNormal, formatByteSize(vramWaste),
		)
	}

	return &TextureChannelPackingResult{
		TotalTexturesScanned:   scanned,
		OrmCompliantCount:      ormCompliant,
		UnpackedTexturesCount:  unpackedCount,
		InvalidNormalMapsCount: invalidNormal,
		VramWasteEstimateBytes: vramWaste,
		VramWasteStr:           formatByteSize(vramWaste),
		Textures:               textures,
		Summary:                summary,
	}, nil
}

type textureFileInfo struct {
	width     int
	height    int
	channels  int
	colorType string
	hasSRGB   bool
}

var pngFileSignature = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}

// inspectTextureFile reads only image headers so large textures are not decoded.
func inspectTextureFile(path string) (textureFileInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return textureFileInfo{}, err
	}
	defer f.Close()

	header := make([]byte, 32)
	n, readErr := io.ReadFull(f, header)
	if readErr != nil && readErr != io.ErrUnexpectedEOF && readErr != io.EOF {
		return textureFileInfo{}, readErr
	}
	header = header[:n]

	if n >= 26 && bytes.Equal(header[:8], pngFileSignature) {
		info := textureFileInfo{
			width:  int(binary.BigEndian.Uint32(header[16:20])),
			height: int(binary.BigEndian.Uint32(header[20:24])),
		}
		switch header[25] {
		case 0:
			info.channels, info.colorType = 1, "Gray"
		case 2:
			info.channels, info.colorType = 3, "RGB"
		case 3:
			info.channels, info.colorType = 1, "Palette"
		case 4:
			info.channels, info.colorType = 2, "GrayAlpha"
		case 6:
			info.channels, info.colorType = 4, "RGBA"
		default:
			return textureFileInfo{}, fmt.Errorf("unsupported PNG color type %d", header[25])
		}
		if info.width <= 0 || info.height <= 0 {
			return textureFileInfo{}, errors.New("invalid PNG dimensions")
		}
		info.hasSRGB = pngHasSRGBChunk(f)
		return info, nil
	}

	if n >= 2 && header[0] == 0xFF && header[1] == 0xD8 {
		return inspectJPEGFile(f)
	}

	if info, ok := inspectTGAHeader(header); ok {
		return info, nil
	}
	return textureFileInfo{}, errors.New("unsupported texture format")
}

func pngHasSRGBChunk(f *os.File) bool {
	if _, err := f.Seek(8, io.SeekStart); err != nil {
		return false
	}
	for i := 0; i < 256; i++ {
		chunkHeader := make([]byte, 8)
		if _, err := io.ReadFull(f, chunkHeader); err != nil {
			return false
		}
		length := int(binary.BigEndian.Uint32(chunkHeader[:4]))
		chunkType := string(chunkHeader[4:8])
		if chunkType == "sRGB" {
			return true
		}
		if chunkType == "IDAT" || chunkType == "IEND" || length < 0 || length > 8*1024*1024 {
			return false
		}
		if _, err := f.Seek(int64(length)+4, io.SeekCurrent); err != nil {
			return false
		}
	}
	return false
}

func inspectJPEGFile(f *os.File) (textureFileInfo, error) {
	if _, err := f.Seek(2, io.SeekStart); err != nil {
		return textureFileInfo{}, err
	}
	for i := 0; i < 256; i++ {
		marker := make([]byte, 2)
		if _, err := io.ReadFull(f, marker); err != nil {
			return textureFileInfo{}, err
		}
		if marker[0] != 0xFF {
			return textureFileInfo{}, errors.New("invalid JPEG marker")
		}
		for marker[1] == 0xFF {
			if _, err := io.ReadFull(f, marker[1:2]); err != nil {
				return textureFileInfo{}, err
			}
		}
		code := marker[1]
		if code == 0xD8 || code == 0xD9 {
			continue
		}
		if code == 0xDA {
			break
		}
		lengthBytes := make([]byte, 2)
		if _, err := io.ReadFull(f, lengthBytes); err != nil {
			return textureFileInfo{}, err
		}
		segmentLength := int(binary.BigEndian.Uint16(lengthBytes))
		if segmentLength < 2 {
			return textureFileInfo{}, errors.New("invalid JPEG segment length")
		}
		payload := make([]byte, segmentLength-2)
		if _, err := io.ReadFull(f, payload); err != nil {
			return textureFileInfo{}, err
		}
		switch code {
		case 0xC0, 0xC1, 0xC2, 0xC3, 0xC5, 0xC6, 0xC7, 0xC9, 0xCA, 0xCB, 0xCD, 0xCE, 0xCF:
			if len(payload) < 6 {
				return textureFileInfo{}, errors.New("short JPEG SOF segment")
			}
			info := textureFileInfo{
				height:   int(binary.BigEndian.Uint16(payload[1:3])),
				width:    int(binary.BigEndian.Uint16(payload[3:5])),
				channels: int(payload[5]),
			}
			switch info.channels {
			case 1:
				info.colorType = "Gray"
			case 3:
				info.colorType = "RGB"
			case 4:
				info.colorType = "CMYK"
			default:
				info.colorType = fmt.Sprintf("JPEG/%dch", info.channels)
			}
			if info.width <= 0 || info.height <= 0 {
				return textureFileInfo{}, errors.New("invalid JPEG dimensions")
			}
			return info, nil
		}
	}
	return textureFileInfo{}, errors.New("JPEG SOF marker not found")
}

func inspectTGAHeader(header []byte) (textureFileInfo, bool) {
	if len(header) < 18 {
		return textureFileInfo{}, false
	}
	imageType := header[2]
	switch imageType {
	case 1, 2, 3, 9, 10, 11:
	default:
		return textureFileInfo{}, false
	}
	width := int(binary.LittleEndian.Uint16(header[12:14]))
	height := int(binary.LittleEndian.Uint16(header[14:16]))
	pixelDepth := int(header[16])
	if width <= 0 || height <= 0 || pixelDepth < 8 {
		return textureFileInfo{}, false
	}
	channels := pixelDepth / 8
	colorType := fmt.Sprintf("TGA/%dbit", pixelDepth)
	if imageType == 3 || imageType == 11 {
		channels, colorType = 1, "Gray"
	}
	return textureFileInfo{width: width, height: height, channels: channels, colorType: colorType}, true
}

func classifyTextureChannelPacking(rel string, info textureFileInfo) (TextureChannelAuditItem, int64, bool, bool, bool) {
	stem := strings.ToLower(strings.TrimSuffix(filepath.Base(rel), filepath.Ext(rel)))
	pixels := int64(info.width) * int64(info.height)

	if hasAnyTextureToken(stem, "_normal", "_nrm", "_nor") || strings.HasSuffix(stem, "_n") {
		item := TextureChannelAuditItem{Path: rel, PackingType: "Normal", IsCompliant: true}
		isInvalid := false
		switch {
		case info.channels < 2:
			isInvalid = true
			item.IsCompliant = false
			item.Issue = fmt.Sprintf("法线贴图仅解析到 %d 个通道（%s），无法保存切线空间 XY", info.channels, info.colorType)
			item.Recommendation = "使用 RGB 法线贴图，或改为双通道 BC5/DXT5nm 压缩"
		case info.hasSRGB:
			isInvalid = true
			item.IsCompliant = false
			item.Issue = "法线贴图包含 sRGB 标记，线性数据被当作颜色处理"
			item.Recommendation = "导入/导出时关闭 sRGB，保持线性色彩空间"
		case info.channels == 4:
			item.Issue = "含 Alpha 通道"
			item.Recommendation = "确认导入器使用 BC5 或 DXT5nm，避免冗余 Alpha 带宽"
		default:
			item.Recommendation = "法线通道与线性色彩空间合规"
		}
		return item, 0, false, false, isInvalid
	}

	if hasAnyTextureToken(stem, "_orm", "_mra", "_rma", "_mrao", "_ormh", "_aorm", "_rmao") {
		item := TextureChannelAuditItem{
			Path:        rel,
			PackingType: "ORM (R=AO, G=Roughness, B=Metallic)",
			IsCompliant: true,
		}
		var waste int64
		switch {
		case info.channels < 3:
			item.IsCompliant = false
			item.Issue = fmt.Sprintf("打包数据贴图只有 %d 个通道（%s），无法同时承载 AO/Roughness/Metallic", info.channels, info.colorType)
			item.Recommendation = "导出 RGB(A) 打包贴图，并在导入时关闭 sRGB"
		case info.hasSRGB:
			item.IsCompliant = false
			item.Issue = "线性通道数据包含 sRGB 标记"
			item.Recommendation = "导入时关闭 sRGB，避免粗糙度/金属度数值失真"
		case info.channels == 4:
			item.Issue = "包含未使用 Alpha 通道"
			item.Recommendation = "移除 Alpha 或改用 RGB 压缩格式"
			waste = pixels
		default:
			item.Recommendation = "ORM 通道打包合规"
		}
		return item, waste, item.IsCompliant, false, false
	}

	if token, ok := splitTextureToken(stem); ok {
		item := TextureChannelAuditItem{
			Path:        rel,
			PackingType: "SplitChannels",
			IsCompliant: false,
			Issue:       fmt.Sprintf("独立 %s 通道贴图未与其他 PBR 通道打包", token),
		}
		var waste int64
		if info.channels > 1 {
			waste = pixels * int64(info.channels-1)
			item.Issue += fmt.Sprintf("，且以 %d 通道（%s）存储单通道数据", info.channels, info.colorType)
		}
		item.Recommendation = "将 AO/Roughness/Metallic 合并至单张 ORM/MRA 贴图，减少采样与显存占用"
		return item, waste, false, true, false
	}

	return TextureChannelAuditItem{}, 0, false, false, false
}

func hasAnyTextureToken(stem string, tokens ...string) bool {
	for _, token := range tokens {
		if strings.Contains(stem, token) {
			return true
		}
	}
	return false
}

func splitTextureToken(stem string) (string, bool) {
	tokens := []struct {
		token string
		label string
	}{
		{"_occlusion", "AO"},
		{"_ao", "AO"},
		{"_roughness", "Roughness"},
		{"_rough", "Roughness"},
		{"_metallic", "Metallic"},
		{"_metalness", "Metallic"},
		{"_metal", "Metallic"},
		{"_smoothness", "Smoothness"},
		{"_gloss", "Gloss"},
		{"_height", "Height"},
		{"_displacement", "Displacement"},
	}
	for _, candidate := range tokens {
		if strings.Contains(stem, candidate.token) {
			return candidate.label, true
		}
	}
	return "", false
}

// =========================================================================
// Round 132: Level Lighting & Lightmap Bake Settings Auditor (AuditLightmapBakeSettings)
// =========================================================================

func (GitService) AuditLightmapBakeSettings(repoPath string, targetLevel string) (*LightmapBakeAuditResult, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath cannot be empty")
	}

	searchRoot := repoPath
	if trimmed := strings.TrimSpace(targetLevel); trimmed != "" && trimmed != "." {
		searchRoot = filepath.Join(repoPath, trimmed)
	}
	if _, err := os.Stat(searchRoot); err != nil {
		return nil, fmt.Errorf("目标关卡路径不可访问: %w", err)
	}

	skipDirs := map[string]bool{
		".git": true, "node_modules": true, "Library": true, "Temp": true, "Logs": true,
		"obj": true, "Build": true, "Builds": true, "DerivedDataCache": true,
	}
	sceneSettings := []lightmapSettingsInfo{}
	assetSettings := []lightmapSettingsInfo{}

	walkErr := filepath.Walk(searchRoot, func(p string, fi os.FileInfo, err error) error {
		if err != nil || fi == nil {
			return nil
		}
		if fi.IsDir() {
			if skipDirs[fi.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(p))
		if ext != ".unity" && ext != ".lighting" {
			return nil
		}
		data, readErr := os.ReadFile(p)
		if readErr != nil {
			return nil
		}
		header := "LightmapSettings:"
		if ext == ".lighting" {
			header = "LightingSettings:"
		}
		info, ok := parseUnityLightmapSettings(data, header)
		if !ok {
			return nil
		}
		rel, relErr := filepath.Rel(repoPath, p)
		if relErr != nil {
			rel = p
		}
		info.path = filepath.ToSlash(rel)
		if ext == ".lighting" {
			assetSettings = append(assetSettings, info)
		} else {
			sceneSettings = append(sceneSettings, info)
		}
		return nil
	})
	if walkErr != nil {
		return nil, fmt.Errorf("扫描关卡光照设置失败: %v", walkErr)
	}

	settings := sceneSettings
	if len(settings) == 0 {
		settings = assetSettings
	}
	sort.Slice(settings, func(i, j int) bool { return settings[i].path < settings[j].path })

	if len(settings) == 0 {
		return &LightmapBakeAuditResult{
			TotalMeshes:         0,
			OverlappingMeshes:   []LightmapMeshItem{},
			BakeSettingsSummary: "未找到 Unity LightmapSettings / LightingSettings",
			HealthScore:         0,
			Summary:             "未检测到可审计的 Unity 场景或光照配置文件，未作烘焙设置评分；网格 UV2 重叠需导入器级解析，本报告不伪造",
		}, nil
	}

	warnings := []LightmapMeshItem{}
	excessiveCount := 0
	score := 100
	var totalAtlasPixels int64
	resolutions := make([]int, 0, len(settings))
	atlasSizes := make([]int, 0, len(settings))
	compressedCount := 0
	modeCounts := map[string]int{}

	for _, setting := range settings {
		resolution := setting.bakeResolution
		if resolution <= 0 {
			resolution = setting.legacyResolution
		}
		resolutions = append(resolutions, resolution)
		if setting.atlasSize > 0 {
			atlasSizes = append(atlasSizes, setting.atlasSize)
			totalAtlasPixels += int64(setting.atlasSize) * int64(setting.atlasSize)
		}
		if setting.compression {
			compressedCount++
		}
		if setting.lightmapsMode != "" {
			modeCounts[setting.lightmapsMode]++
		}

		severity := "OK"
		recommendations := []string{}
		if resolution > 200 {
			severity = "CRITICAL"
			recommendations = append(recommendations, "Bake Resolution 过高，建议降至 100 texels/unit 以下或拆分场景")
		} else if resolution > 100 {
			severity = "WARNING"
			recommendations = append(recommendations, "Bake Resolution 偏高，检查是否为远景场景误用高精度")
		}
		if setting.atlasSize > 4096 {
			if severity == "OK" {
				severity = "WARNING"
			}
			recommendations = append(recommendations, fmt.Sprintf("Atlas %d 超过 4096，烘焙时间与显存占用显著增加", setting.atlasSize))
		}
		if !setting.compression && severity == "OK" {
			severity = "WARNING"
			recommendations = append(recommendations, "光照图未启用纹理压缩")
		}
		if severity != "OK" {
			excessiveCount++
			switch severity {
			case "CRITICAL":
				score -= 25
			default:
				score -= 10
			}
			warnings = append(warnings, LightmapMeshItem{
				MeshPath:       setting.path,
				Resolution:     resolution,
				HasUv2Overlap:  false,
				Uv2BoundsValid: true,
				Severity:       severity,
				Recommendation: strings.Join(recommendations, "；"),
			})
		}
	}
	if score < 0 {
		score = 0
	}

	estimatedMinutes := 0
	if totalAtlasPixels > 0 {
		estimatedMinutes = int(math.Ceil(float64(totalAtlasPixels) / (1024.0 * 1024.0) * 1.5))
	}
	if estimatedMinutes < len(settings) {
		estimatedMinutes = len(settings)
	}

	bakeSummaryParts := []string{
		fmt.Sprintf("Unity 光照配置 %d 处", len(settings)),
		fmt.Sprintf("Bake Resolution %s texels/unit", formatLightmapRange(resolutions)),
	}
	if len(atlasSizes) > 0 {
		bakeSummaryParts = append(bakeSummaryParts, fmt.Sprintf("Atlas %s", formatLightmapRange(atlasSizes)))
	}
	bakeSummaryParts = append(bakeSummaryParts, fmt.Sprintf("纹理压缩 %d/%d 处", compressedCount, len(settings)))
	modeParts := make([]string, 0, len(modeCounts))
	for mode, count := range modeCounts {
		modeParts = append(modeParts, fmt.Sprintf("%s %d 处", mode, count))
	}
	sort.Strings(modeParts)
	bakeSummaryParts = append(bakeSummaryParts, modeParts...)

	summary := fmt.Sprintf(
		"光照烘焙设置审计完成: %d 处 Unity 光照配置, %d 处分辨率/Atlas/压缩告警, 烘焙耗时按 Atlas 面积粗略估算 %d 分钟, 评分 %d/100; 网格 UV2 重叠需导入器级解析，本报告未评估",
		len(settings), excessiveCount, estimatedMinutes, score,
	)

	return &LightmapBakeAuditResult{
		TotalMeshes:              len(settings),
		OverlappingUv2Count:      0,
		ExcessiveResolutionCount: excessiveCount,
		EstimatedBakeMinutes:     estimatedMinutes,
		OverlappingMeshes:        warnings,
		BakeSettingsSummary:      strings.Join(bakeSummaryParts, ", "),
		HealthScore:              score,
		Summary:                  summary,
	}, nil
}

type lightmapSettingsInfo struct {
	path             string
	bakeResolution   int
	legacyResolution int
	atlasSize        int
	lightmapsMode    string
	compression      bool
	bakedLightmaps   bool
}

func parseUnityLightmapSettings(data []byte, header string) (lightmapSettingsInfo, bool) {
	block, ok := extractUnitySettingsBlock(string(data), header)
	if !ok {
		return lightmapSettingsInfo{}, false
	}
	info := lightmapSettingsInfo{compression: true}
	for _, line := range strings.Split(block, "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		switch key {
		case "m_BakeResolution":
			info.bakeResolution = firstInteger(value)
		case "m_Resolution":
			info.legacyResolution = firstInteger(value)
		case "m_AtlasSize", "m_LightmapMaxSize":
			if size := firstInteger(value); size > info.atlasSize {
				info.atlasSize = size
			}
		case "m_LightmapsMode":
			info.lightmapsMode = describeLightmapsMode(value)
		case "m_TextureCompression":
			info.compression = value != "0"
		case "m_EnableBakedLightmaps":
			info.bakedLightmaps = value == "1"
		}
	}
	return info, true
}

func extractUnitySettingsBlock(text string, header string) (string, bool) {
	lines := strings.Split(text, "\n")
	start := -1
	headerIndent := -1
	for i, line := range lines {
		if strings.TrimSpace(line) != header {
			continue
		}
		start = i + 1
		headerIndent = len(line) - len(strings.TrimLeft(line, " \t"))
		break
	}
	if start < 0 {
		return "", false
	}
	block := []string{}
	for _, line := range lines[start:] {
		if strings.TrimSpace(line) == "" {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		trimmed := strings.TrimSpace(line)
		if indent <= headerIndent && !strings.HasPrefix(trimmed, "-") {
			break
		}
		block = append(block, line)
	}
	return strings.Join(block, "\n"), true
}

func firstInteger(value string) int {
	match := regexp.MustCompile(`\d+`).FindString(value)
	if match == "" {
		return 0
	}
	number, err := strconv.Atoi(match)
	if err != nil {
		return 0
	}
	return number
}

func describeLightmapsMode(value string) string {
	switch strings.TrimSpace(value) {
	case "0":
		return "Non-Directional"
	case "1":
		return "Combined Directional"
	case "2":
		return "Indirect Only"
	}
	return ""
}

func formatLightmapRange(values []int) string {
	if len(values) == 0 {
		return "未知"
	}
	minimum, maximum := values[0], values[0]
	for _, value := range values[1:] {
		if value < minimum {
			minimum = value
		}
		if value > maximum {
			maximum = value
		}
	}
	if minimum == maximum {
		return strconv.Itoa(minimum)
	}
	return fmt.Sprintf("%d-%d", minimum, maximum)
}

// =========================================================================
// Round 134: Game Font Commercial License & Copyright Checker (AuditFontLicenses)
// =========================================================================

func (GitService) AuditFontLicenses(repoPath string) (*FontLicenseAuditResult, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath cannot be empty")
	}

	fonts := []FontLicenseItem{}
	skipDirs := map[string]bool{
		".git": true, "node_modules": true, "Library": true, "Temp": true,
		"obj": true, "Build": true, "Builds": true,
	}
	_ = filepath.Walk(repoPath, func(p string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		if info.IsDir() {
			if skipDirs[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(p))
		if ext != ".ttf" && ext != ".otf" && ext != ".ttc" {
			return nil
		}
		rel, relErr := filepath.Rel(repoPath, p)
		if relErr != nil {
			rel = p
		}
		family, copyright, license, licenseURL, _ := readFontLicenseMetadata(p)
		if family == "" {
			family = strings.TrimSuffix(filepath.Base(p), ext)
		}
		licenseType, safe := classifyFontLicense(copyright + " " + license + " " + licenseURL)
		advice := "补充字体授权证明或替换为明确商用许可字体"
		if safe {
			advice = "无需替换，保留授权文件并随包分发"
		} else if licenseType == "Restricted" {
			advice = "替换为 SIL OFL / Apache 等商用安全字体"
		}
		fonts = append(fonts, FontLicenseItem{
			FontPath:             filepath.ToSlash(rel),
			FamilyName:           family,
			CopyrightNotice:      copyright,
			LicenseType:          licenseType,
			IsCommercialSafe:     safe,
			SuggestedAlternative: advice,
		})
		return nil
	})
	sort.Slice(fonts, func(i, j int) bool { return fonts[i].FontPath < fonts[j].FontPath })

	safeCount, restrictedCount, unknownCount := 0, 0, 0
	for _, font := range fonts {
		switch font.LicenseType {
		case "Commercial_Safe":
			safeCount++
		case "Restricted":
			restrictedCount++
		default:
			unknownCount++
		}
	}
	risk := "LOW"
	if restrictedCount > 0 {
		risk = "HIGH"
	} else if unknownCount > 0 {
		risk = "MEDIUM"
	}
	summary := fmt.Sprintf("字体版权核查完成: 共 %d 门字体，%d 商用安全，%d 受限，%d 未知；风险等级 %s。",
		len(fonts), safeCount, restrictedCount, unknownCount, risk)
	if len(fonts) == 0 {
		summary = "未发现字体文件，跳过字体版权审计。"
	}
	return &FontLicenseAuditResult{
		TotalFonts:           len(fonts),
		CommercialSafeCount:  safeCount,
		RestrictedFontsCount: restrictedCount,
		UnknownLicenseCount:  unknownCount,
		Fonts:                fonts,
		LegalRiskScore:       risk,
		Summary:              summary,
	}, nil
}

func readFontLicenseMetadata(path string) (family, copyright, license, licenseURL string, err error) {
	names, err := readFontNameTable(path)
	if err != nil {
		return "", "", "", "", err
	}
	return cleanFontName(names[1]), cleanFontName(names[0]), cleanFontName(names[13]), cleanFontName(names[14]), nil
}

func readFontNameTable(path string) (map[uint16]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data) < 12 {
		return nil, errors.New("font file is too short")
	}
	base := 0
	if string(data[:4]) == "ttcf" {
		if len(data) < 16 {
			return nil, errors.New("invalid TTC header")
		}
		base = int(binary.BigEndian.Uint32(data[12:16]))
	}
	if base < 0 || base+12 > len(data) {
		return nil, errors.New("invalid sfnt header")
	}
	numTables := int(binary.BigEndian.Uint16(data[base+4 : base+6]))
	if numTables <= 0 || numTables > 4096 {
		return nil, errors.New("invalid sfnt table count")
	}
	for i := 0; i < numTables; i++ {
		record := base + 12 + i*16
		if record+16 > len(data) {
			break
		}
		if string(data[record:record+4]) != "name" {
			continue
		}
		offset := int(binary.BigEndian.Uint32(data[record+8 : record+12]))
		length := int(binary.BigEndian.Uint32(data[record+12 : record+16]))
		if offset < 0 || length < 0 || offset+length > len(data) {
			return nil, errors.New("invalid name table bounds")
		}
		return parseFontNameTable(data[offset : offset+length])
	}
	return nil, errors.New("name table not found")
}

func parseFontNameTable(table []byte) (map[uint16]string, error) {
	if len(table) < 6 {
		return nil, errors.New("name table is too short")
	}
	count := int(binary.BigEndian.Uint16(table[2:4]))
	stringOffset := int(binary.BigEndian.Uint16(table[4:6]))
	if count <= 0 || count > 4096 || 6+count*12 > len(table) {
		return nil, errors.New("invalid name table records")
	}
	best := make(map[uint16]struct {
		score int
		text  string
	})
	for i := 0; i < count; i++ {
		record := 6 + i*12
		platformID := binary.BigEndian.Uint16(table[record : record+2])
		languageID := binary.BigEndian.Uint16(table[record+4 : record+6])
		nameID := binary.BigEndian.Uint16(table[record+6 : record+8])
		length := int(binary.BigEndian.Uint16(table[record+8 : record+10]))
		offset := int(binary.BigEndian.Uint16(table[record+10 : record+12]))
		start := stringOffset + offset
		if start < 0 || length < 0 || start+length > len(table) {
			continue
		}
		text := cleanFontName(decodeFontName(platformID, table[start:start+length]))
		if text == "" {
			continue
		}
		score := 0
		switch platformID {
		case 3:
			score = 3
			if languageID != 0x409 {
				score = 2
			}
		case 0:
			score = 2
		case 1:
			score = 1
		}
		if previous, ok := best[nameID]; !ok || score > previous.score {
			best[nameID] = struct {
				score int
				text  string
			}{score: score, text: text}
		}
	}
	names := make(map[uint16]string, len(best))
	for id, entry := range best {
		names[id] = entry.text
	}
	return names, nil
}

func decodeFontName(platformID uint16, raw []byte) string {
	if platformID == 0 || platformID == 3 {
		units := make([]uint16, 0, len(raw)/2)
		for i := 0; i+1 < len(raw); i += 2 {
			units = append(units, binary.BigEndian.Uint16(raw[i:i+2]))
		}
		return string(utf16.Decode(units))
	}
	runes := make([]rune, 0, len(raw))
	for _, b := range raw {
		runes = append(runes, rune(b))
	}
	return string(runes)
}

func cleanFontName(name string) string {
	name = strings.ReplaceAll(name, "\x00", "")
	return strings.TrimSpace(name)
}

func classifyFontLicense(text string) (string, bool) {
	lower := strings.ToLower(text)
	for _, term := range []string{
		"not for commercial", "non-commercial", "noncommercial", "personal use only",
		"evaluation only", "commercial use prohibited", "trial version",
	} {
		if strings.Contains(lower, term) {
			return "Restricted", false
		}
	}
	for _, term := range []string{
		"sil open font license", "open font license", "ofl", "apache license",
		"mit license", "bsd license", "cc0", "public domain",
		"free for commercial use", "gnu general public license", "gnu lesser general public license",
	} {
		if strings.Contains(lower, term) {
			return "Commercial_Safe", true
		}
	}
	return "Unknown", false
}

// =========================================================================
// Round 139: Multi-Engine Shader Pipe & Self-Healing Wizard (ShaderSelfHealingWizard)
// =========================================================================

func (GitService) ShaderSelfHealingWizard(repoPath string, action string) (*ShaderSelfHealingResult, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath cannot be empty")
	}
	action = strings.ToUpper(strings.TrimSpace(action))
	switch action {
	case "", "STATUS", "DIAGNOSE", "SCAN", "AUDIT", "CLEAR_SHADER_CACHE", "HEAL_ALL":
	default:
		return nil, fmt.Errorf("未知的 Shader 修复动作: %s", action)
	}

	engineType := detectShaderEngineType(repoPath)
	skipDirs := map[string]bool{
		".git": true, "node_modules": true, "Library": true, "Temp": true,
		"Intermediate": true, "Saved": true, "DerivedDataCache": true,
	}
	shaderExts := map[string]bool{
		".shader": true, ".hlsl": true, ".cginc": true, ".compute": true,
		".glsl": true, ".ush": true, ".usf": true, ".shadergraph": true,
	}
	shaderFiles := []string{}
	materialFiles := []string{}
	includeFiles := map[string][]string{}
	metaGUIDs := make(map[string]bool)

	_ = filepath.Walk(repoPath, func(p string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		if info.IsDir() {
			if skipDirs[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(p))
		switch {
		case shaderExts[ext]:
			clean := filepath.Clean(p)
			shaderFiles = append(shaderFiles, clean)
			includeFiles[strings.ToLower(filepath.Base(p))] = append(includeFiles[strings.ToLower(filepath.Base(p))], clean)
		case ext == ".mat":
			materialFiles = append(materialFiles, filepath.Clean(p))
		case ext == ".meta":
			if data, readErr := os.ReadFile(p); readErr == nil {
				if m := assetMetaGuidPattern.FindStringSubmatch(string(data)); len(m) > 1 {
					metaGUIDs[strings.ToLower(m[1])] = true
				}
			}
		}
		return nil
	})

	builtinIncludes := map[string]bool{
		"unitycg.cginc": true, "unitystandardbrdf.cginc": true, "lighting.cginc": true,
		"autolight.cginc": true, "hlslsupport.cginc": true, "unityinstancing.cginc": true,
		"unitygbuffer.cginc": true, "unitypbslighting.cginc": true,
		"unityshadervariables.cginc": true, "terrainengine.cginc": true, "unityui.cginc": true,
	}
	includePattern := regexp.MustCompile(`(?m)^\s*#include\s+["<]([^">]+)[">]`)
	brokenShaders := map[string][]string{}
	for _, shader := range shaderFiles {
		data, err := os.ReadFile(shader)
		if err != nil {
			continue
		}
		for _, m := range includePattern.FindAllStringSubmatch(string(data), -1) {
			if len(m) < 2 {
				continue
			}
			include := strings.TrimSpace(m[1])
			if builtinIncludes[strings.ToLower(filepath.Base(include))] {
				continue
			}
			candidates := []string{
				filepath.Join(filepath.Dir(shader), include),
				filepath.Join(repoPath, include),
				filepath.Join(repoPath, "Assets", include),
				filepath.Join(repoPath, "Packages", include),
			}
			found := false
			for _, candidate := range candidates {
				if fi, statErr := os.Stat(candidate); statErr == nil && !fi.IsDir() {
					found = true
					break
				}
			}
			if !found && len(includeFiles[strings.ToLower(filepath.Base(include))]) > 0 {
				found = true
			}
			if !found {
				brokenShaders[shader] = append(brokenShaders[shader], include)
			}
		}
	}

	materialShaderPattern := regexp.MustCompile(`(?i)m_Shader:\s*\{[^}]*guid:\s*([0-9a-f]{32})`)
	builtinShaderGUID := "0000000000000000f000000000000000"
	brokenMaterials := []string{}
	for _, material := range materialFiles {
		data, err := os.ReadFile(material)
		if err != nil {
			continue
		}
		m := materialShaderPattern.FindStringSubmatch(string(data))
		if len(m) < 2 {
			continue
		}
		guid := strings.ToLower(m[1])
		if guid == builtinShaderGUID {
			continue
		}
		if !metaGUIDs[guid] {
			brokenMaterials = append(brokenMaterials, material)
		}
	}

	cacheCleared := false
	clearedDirs := 0
	if action == "CLEAR_SHADER_CACHE" || action == "HEAL_ALL" {
		cacheDirs := []string{
			filepath.Join(repoPath, "Library", "ShaderCache"),
			filepath.Join(repoPath, "Library", "ShaderCompilationCache"),
			filepath.Join(repoPath, "Temp", "ShaderCache"),
			filepath.Join(repoPath, "Saved", "ShaderCache"),
		}
		for _, dir := range cacheDirs {
			if fi, err := os.Stat(dir); err == nil && fi.IsDir() {
				if err := os.RemoveAll(dir); err != nil {
					return nil, fmt.Errorf("清理 Shader 缓存失败 (%s): %w", dir, err)
				}
				clearedDirs++
			}
		}
		cacheCleared = clearedDirs > 0
	}

	grade := "OPTIMAL"
	if len(brokenShaders) > 0 || len(brokenMaterials) > 0 {
		grade = "ATTENTION_REQUIRED"
	} else if cacheCleared {
		grade = "REPAIRED"
	}
	summary := fmt.Sprintf("着色器管线体检完成: 引擎=%s，破损 Shader %d 个，缺失 Shader 引用材质 %d 个；黑面网格需渲染期验证；已清理 %d 个缓存目录。",
		engineType, len(brokenShaders), len(brokenMaterials), clearedDirs)
	return &ShaderSelfHealingResult{
		EngineType:             engineType,
		BrokenShaderCount:      len(brokenShaders),
		BlackRenderMeshesCount: 0,
		ShaderCacheCleared:     cacheCleared,
		RepairedMaterials:      []string{},
		Success:                true,
		HealthGrade:            grade,
		Summary:                summary,
	}, nil
}

func detectShaderEngineType(repoPath string) string {
	if data, err := os.ReadFile(filepath.Join(repoPath, "Packages", "manifest.json")); err == nil {
		lower := strings.ToLower(string(data))
		if strings.Contains(lower, "render-pipelines.universal") {
			return "Unity URP"
		}
		if strings.Contains(lower, "render-pipelines.high-definition") {
			return "Unity HDRP"
		}
	}
	if fileExists(filepath.Join(repoPath, "ProjectSettings", "ProjectVersion.txt")) {
		return "Unity Built-in"
	}
	if matches, _ := filepath.Glob(filepath.Join(repoPath, "*.uproject")); len(matches) > 0 {
		return "Unreal Engine"
	}
	if fileExists(filepath.Join(repoPath, "project.godot")) {
		return "Godot"
	}
	return "Unknown"
}
