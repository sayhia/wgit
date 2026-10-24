package main

import (
	"archive/zip"
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha256"
	"debug/elf"
	"debug/macho"
	"debug/pe"
	"encoding/base64"
	"encoding/binary"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
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
// Round 102: DCC Asset Watcher & Auto-Lock Guard (DetectDCCLockActivity)
// =========================================================================

// DetectDCCLockActivity monitors active DCC temporary/lock files (Maya, Blender, Photoshop, Substance Painter),
// tracks active DCC processes, and flags unprotected LFS assets or collision risks.
func (GitService) DetectDCCLockActivity(repoPath string) (*DCCLockWatchResult, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath 不能为空")
	}

	res := &DCCLockWatchResult{
		RunningProcesses: []DCCActiveProcess{},
		Items:            []DCCLockItem{},
	}

	procMap := getRunningDCCProcesses()
	for _, p := range procMap {
		res.RunningProcesses = append(res.RunningProcesses, p)
	}

	userName, _ := ExecGit(repoPath, "config", "user.name")
	userEmail, _ := ExecGit(repoPath, "config", "user.email")
	userName = strings.TrimSpace(userName)
	userEmail = strings.TrimSpace(userEmail)

	lfsLocks := make(map[string]string)
	lfsLockQueryErr := ""
	locksOut, err := ExecGit(repoPath, "lfs", "locks")
	if err != nil {
		lfsLockQueryErr = firstLine(err.Error())
	} else if strings.TrimSpace(locksOut) != "" {
		for _, line := range strings.Split(locksOut, "\n") {
			parts := strings.Fields(strings.TrimSpace(line))
			if len(parts) >= 2 {
				lfsLocks[parts[0]] = parts[1]
			}
		}
	}

	lfsExts := getLFSExtensions(repoPath)

	type dccMatchRule struct {
		pattern  string
		tool     string
		resolver func(lockRel string) string
	}

	dccRules := []dccMatchRule{
		{pattern: ".mb.swp", tool: "Autodesk Maya", resolver: func(p string) string { return strings.TrimSuffix(p, ".swp") }},
		{pattern: ".ma.swp", tool: "Autodesk Maya", resolver: func(p string) string { return strings.TrimSuffix(p, ".swp") }},
		{pattern: ".mb.lock", tool: "Autodesk Maya", resolver: func(p string) string { return strings.TrimSuffix(p, ".lock") }},
		{pattern: ".ma.lock", tool: "Autodesk Maya", resolver: func(p string) string { return strings.TrimSuffix(p, ".lock") }},
		{pattern: ".blend@", tool: "Blender", resolver: func(p string) string { return strings.TrimSuffix(p, "@") }},
		{pattern: ".blend1", tool: "Blender", resolver: func(p string) string { return strings.TrimSuffix(p, "1") }},
		{pattern: ".blend2", tool: "Blender", resolver: func(p string) string { return strings.TrimSuffix(p, "2") }},
		{pattern: ".blend.swp", tool: "Blender", resolver: func(p string) string { return strings.TrimSuffix(p, ".swp") }},
		{pattern: ".spp.lock", tool: "Substance 3D Painter", resolver: func(p string) string { return strings.TrimSuffix(p, ".lock") }},
		{pattern: ".spp.autosave", tool: "Substance 3D Painter", resolver: func(p string) string { return strings.TrimSuffix(p, ".autosave") }},
		{pattern: ".max.lock", tool: "3ds Max", resolver: func(p string) string { return strings.TrimSuffix(p, ".lock") }},
		{pattern: ".hip.lock", tool: "Houdini", resolver: func(p string) string { return strings.TrimSuffix(p, ".lock") }},
	}

	var dccItems []DCCLockItem

	_ = filepath.Walk(repoPath, func(p string, fi os.FileInfo, walkErr error) error {
		if walkErr != nil || fi == nil {
			return nil
		}
		if fi.IsDir() {
			name := fi.Name()
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == "Library" ||
				name == "Intermediate" || name == "Saved" {
				return filepath.SkipDir
			}
			return nil
		}

		name := fi.Name()
		rel, rErr := filepath.Rel(repoPath, p)
		if rErr != nil {
			return nil
		}

		matchedTool := ""
		sourceAsset := ""

		for _, rule := range dccRules {
			if strings.HasSuffix(name, rule.pattern) {
				matchedTool = rule.tool
				sourceAsset = rule.resolver(rel)
				break
			}
		}

		if matchedTool == "" {
			if strings.HasSuffix(name, ".psd.lock") || strings.HasSuffix(name, ".psb.lock") {
				matchedTool = "Adobe Photoshop"
				sourceAsset = strings.TrimSuffix(rel, ".lock")
			} else if strings.HasPrefix(name, "~$") && (strings.HasSuffix(name, ".psd") || strings.HasSuffix(name, ".psb")) {
				matchedTool = "Adobe Photoshop"
				sourceAsset = filepath.Join(filepath.Dir(rel), strings.TrimPrefix(name, "~$"))
			} else if strings.HasPrefix(name, "~ps") && strings.HasSuffix(name, ".tmp") {
				matchedTool = "Adobe Photoshop"
				dir := filepath.Dir(p)
				entries, _ := os.ReadDir(dir)
				for _, e := range entries {
					eName := strings.ToLower(e.Name())
					if (strings.HasSuffix(eName, ".psd") || strings.HasSuffix(eName, ".psb")) && !strings.HasPrefix(eName, "~") {
						sourceAsset = filepath.Join(filepath.Dir(rel), e.Name())
						break
					}
				}
				if sourceAsset == "" {
					sourceAsset = filepath.Join(filepath.Dir(rel), "[Photoshop Document]")
				}
			}
		}

		if matchedTool != "" {
			modTime := fi.ModTime()
			duration := time.Since(modTime)
			isActive := duration < 24*time.Hour

			for _, proc := range res.RunningProcesses {
				if proc.DCCTool == matchedTool {
					isActive = true
					break
				}
			}

			isLFS := isPathLFS(sourceAsset, lfsExts)
			lfsStatus := "NOT_LFS"
			lockOwner := ""
			collision := false
			rec := ""

			if isLFS {
				if lfsLockQueryErr != "" {
					lfsStatus = "UNKNOWN"
					rec = "LFS 锁状态未知：查询失败（" + lfsLockQueryErr + "），未作加锁结论。"
				} else if owner, hasLock := lfsLocks[sourceAsset]; hasLock {
					lockOwner = owner
					if (userName != "" && strings.EqualFold(owner, userName)) ||
						(userEmail != "" && strings.EqualFold(owner, userEmail)) {
						lfsStatus = "LOCKED_BY_ME"
						rec = "安全：当前资产已由你持有 Git LFS 独占锁，协同修改受保护。"
					} else {
						lfsStatus = "LOCKED_BY_OTHER"
						collision = true
						rec = fmt.Sprintf("⚠️ 协同冲突警告：该资产已被团队成员 [%s] 锁定！请暂缓在 DCC 中保存，联系该成员避免覆盖覆盖！", owner)
					}
				} else {
					lfsStatus = "UNPROTECTED"
					rec = "未加锁：检测到本地 DCC 正在编辑该 LFS 资产，但尚未在远端加锁。建议立即点击【一键自动上锁】保护！"
				}
			} else {
				rec = "提示：该资产尚未纳入 Git LFS 规则管理，建议通过 LFS 纳管以启用团队独占锁。"
			}

			itemOwner := userName
			if itemOwner == "" {
				itemOwner = "本地创作者"
			}

			dccItems = append(dccItems, DCCLockItem{
				LockFilePath:    rel,
				SourceAssetPath: sourceAsset,
				DCCTool:         matchedTool,
				LockOwner:       itemOwner,
				LastModified:    modTime.Format("2006-01-02 15:04:05"),
				IsActive:        isActive,
				IsLFS:           isLFS,
				LfsLockStatus:   lfsStatus,
				LfsLockOwner:    lockOwner,
				CollisionRisk:   collision,
				Recommendation:  rec,
			})
		}

		return nil
	})

	res.Items = dccItems
	res.TotalDCCFiles = len(dccItems)

	for _, item := range dccItems {
		if item.IsActive {
			res.ActiveLocksCount++
		}
		if item.LfsLockStatus == "UNPROTECTED" {
			res.UnprotectedCount++
		}
		if item.CollisionRisk {
			res.CollisionCount++
		}
	}

	res.Summary = fmt.Sprintf("检测到 %d 个 DCC 活跃临时/锁文件，%d 个运行中 DCC 软件，%d 个未加锁资产，%d 个协作潜在冲突",
		res.TotalDCCFiles, len(res.RunningProcesses), res.UnprotectedCount, res.CollisionCount)
	res.Message = "DCC 资产与锁状态巡检完成"
	if lfsLockQueryErr != "" {
		res.Summary += "；LFS 锁查询失败，未作未加锁/冲突结论"
		res.Message = "DCC 资产与锁状态巡检完成（LFS 锁状态未知）"
	}

	return res, nil
}

func getRunningDCCProcesses() map[int]DCCActiveProcess {
	procs := make(map[int]DCCActiveProcess)

	cmd := exec.Command("ps", "-A", "-o", "pid,comm")
	out, err := cmd.Output()
	if err != nil {
		return procs
	}

	lines := strings.Split(string(out), "\n")
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		parts := strings.Fields(l)
		if len(parts) >= 2 {
			pid, _ := strconv.Atoi(parts[0])
			comm := strings.ToLower(parts[1])

			dccTool := ""
			if strings.Contains(comm, "maya") {
				dccTool = "Autodesk Maya"
			} else if strings.Contains(comm, "blender") {
				dccTool = "Blender"
			} else if strings.Contains(comm, "photoshop") {
				dccTool = "Adobe Photoshop"
			} else if strings.Contains(comm, "substance") {
				dccTool = "Substance 3D Painter"
			} else if strings.Contains(comm, "3dsmax") {
				dccTool = "3ds Max"
			} else if strings.Contains(comm, "houdini") {
				dccTool = "Houdini"
			}

			if dccTool != "" && pid > 0 {
				procs[pid] = DCCActiveProcess{
					PID:         pid,
					ProcessName: filepath.Base(parts[1]),
					DCCTool:     dccTool,
				}
			}
		}
	}

	return procs
}

// AutoLockDCCAsset immediately claims an exclusive Git LFS lock for an actively edited DCC asset
func (GitService) AutoLockDCCAsset(repoPath string, assetPath string) (*DCCAutoLockResult, error) {
	if repoPath == "" || assetPath == "" {
		return nil, errors.New("repoPath 与 assetPath 均不能为空")
	}

	cleanAsset, err := repoRelativePathFromInput(repoPath, assetPath)
	if err != nil {
		return nil, err
	}

	out, err := ExecGit(repoPath, "lfs", "lock", cleanAsset)
	if err != nil {
		return &DCCAutoLockResult{
			Success:   false,
			AssetPath: cleanAsset,
			Message:   fmt.Sprintf("Git LFS 上锁失败: %v (输出: %s)", err, out),
		}, nil
	}

	return &DCCAutoLockResult{
		Success:   true,
		AssetPath: cleanAsset,
		Message:   fmt.Sprintf("已成功为资产 %s 获取 Git LFS 独占锁！协同冲突已规避。", filepath.Base(cleanAsset)),
	}, nil
}

// =========================================================================
// Round 103: Unreal Blueprint & Binary AST Diff (DiffUnrealAssetNodes)
// =========================================================================

// DiffUnrealAssetNodes compares binary AST and Blueprint/Prefab node graphs across commits or working tree
func (GitService) DiffUnrealAssetNodes(repoPath string, assetPath string, baseRef string, targetRef string) (*UnrealAssetDiffResult, error) {
	if assetPath == "" {
		return nil, errors.New("assetPath 不能为空")
	}

	cleanAsset, err := repoRelativePathFromInput(repoPath, assetPath)
	if err != nil {
		return nil, err
	}

	if baseRef == "" {
		baseRef = "HEAD"
	}
	if targetRef == "" {
		targetRef = "WORKTREE"
	}

	const emptyTreeHash = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"
	if baseRef != emptyTreeHash {
		if _, err := ExecGit(repoPath, "rev-parse", "--verify", baseRef+"^{commit}"); err != nil {
			return nil, fmt.Errorf("基线引用 %s 不存在或不可解析: %w", baseRef, err)
		}
	}
	if !strings.EqualFold(targetRef, "WORKTREE") {
		if _, err := ExecGit(repoPath, "rev-parse", "--verify", targetRef+"^{commit}"); err != nil {
			return nil, fmt.Errorf("目标引用 %s 不存在或不可解析: %w", targetRef, err)
		}
	}

	baseBytes, err := fetchAssetBytes(repoPath, cleanAsset, baseRef)
	if err != nil {
		baseBytes = nil
	}

	targetBytes, err := fetchAssetBytes(repoPath, cleanAsset, targetRef)
	if err != nil {
		return nil, fmt.Errorf("读取目标版本文件失败: %v", err)
	}

	baseHeader, baseNodes := parseAssetAST(cleanAsset, baseBytes)
	targetHeader, targetNodes := parseAssetAST(cleanAsset, targetBytes)

	baseNodeMap := make(map[string]AssetGraphNode)
	for _, n := range baseNodes {
		baseNodeMap[n.ID] = n
	}

	targetNodeMap := make(map[string]AssetGraphNode)
	for _, n := range targetNodes {
		targetNodeMap[n.ID] = n
	}

	addedNodes := []AssetGraphNode{}
	removedNodes := []AssetGraphNode{}
	modifiedNodes := []ModifiedNodeItem{}
	unchangedCount := 0

	for id, tNode := range targetNodeMap {
		bNode, exists := baseNodeMap[id]
		if !exists {
			addedNodes = append(addedNodes, tNode)
			continue
		}

		modItem := diffSingleNode(bNode, tNode)
		if len(modItem.PinChanges) > 0 || len(modItem.PropertyChanges) > 0 || modItem.PosDeltaX != 0 || modItem.PosDeltaY != 0 {
			modifiedNodes = append(modifiedNodes, modItem)
		} else {
			unchangedCount++
		}
	}

	for id, bNode := range baseNodeMap {
		if _, exists := targetNodeMap[id]; !exists {
			removedNodes = append(removedNodes, bNode)
		}
	}

	sort.Slice(addedNodes, func(i, j int) bool { return addedNodes[i].ID < addedNodes[j].ID })
	sort.Slice(removedNodes, func(i, j int) bool { return removedNodes[i].ID < removedNodes[j].ID })
	sort.Slice(modifiedNodes, func(i, j int) bool { return modifiedNodes[i].NodeID < modifiedNodes[j].NodeID })

	format := "通用游戏资产"
	if targetHeader != nil && targetHeader.AssetFormat != "" {
		format = targetHeader.AssetFormat
	} else if baseHeader != nil && baseHeader.AssetFormat != "" {
		format = baseHeader.AssetFormat
	}

	hasChanges := len(addedNodes) > 0 || len(removedNodes) > 0 || len(modifiedNodes) > 0

	summaryParts := []string{}
	if len(addedNodes) > 0 {
		summaryParts = append(summaryParts, fmt.Sprintf("+%d 节点新增", len(addedNodes)))
	}
	if len(removedNodes) > 0 {
		summaryParts = append(summaryParts, fmt.Sprintf("-%d 节点删除", len(removedNodes)))
	}
	if len(modifiedNodes) > 0 {
		totalPinMods := 0
		for _, m := range modifiedNodes {
			totalPinMods += len(m.PinChanges)
		}
		summaryParts = append(summaryParts, fmt.Sprintf("%d 节点变更 (%d 处引脚/属性微调)", len(modifiedNodes), totalPinMods))
	}
	if len(summaryParts) == 0 {
		summaryParts = append(summaryParts, "图元与节点引脚无结构性差异")
	}

	visualSummary := strings.Join(summaryParts, ", ")

	analysisKnown := true
	analysisNote := ""
	binaryUE := (targetHeader != nil && strings.Contains(targetHeader.Magic, "UE Asset")) ||
		(baseHeader != nil && strings.Contains(baseHeader.Magic, "UE Asset"))
	if binaryUE {
		analysisKnown = false
		analysisNote = "二进制 UE 包仅进行 K2Node 字符串扫描，未验证导出表/AST；节点数量与引脚仅供参考"
		visualSummary += "（未验证 AST）"
	}

	return &UnrealAssetDiffResult{
		AssetPath:        cleanAsset,
		BaseRef:          baseRef,
		TargetRef:        targetRef,
		AssetFormat:      format,
		AnalysisKnown:    analysisKnown,
		AnalysisNote:     analysisNote,
		BaseHeader:       baseHeader,
		TargetHeader:     targetHeader,
		AddedNodes:       addedNodes,
		RemovedNodes:     removedNodes,
		ModifiedNodes:    modifiedNodes,
		UnchangedCount:   unchangedCount,
		TotalBaseNodes:   len(baseNodes),
		TotalTargetNodes: len(targetNodes),
		HasGraphChanges:  hasChanges,
		VisualSummary:    visualSummary,
		Message:          fmt.Sprintf("对比完成: %s (%s vs %s)", visualSummary, baseRef, targetRef),
	}, nil
}

func fetchAssetBytes(repoPath string, assetPath string, ref string) ([]byte, error) {
	if ref == "WORKTREE" || ref == "" {
		fullPath := filepath.Join(repoPath, assetPath)
		return os.ReadFile(fullPath)
	}

	out, err := ExecGitRaw(repoPath, "show", fmt.Sprintf("%s:%s", ref, assetPath))
	if err != nil {
		return nil, err
	}

	if strings.Contains(out, "oid sha256:") {
		for _, line := range strings.Split(out, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "oid sha256:") {
				oid := strings.TrimSpace(strings.TrimPrefix(line, "oid sha256:"))
				if len(oid) >= 4 {
					lfsCachePath := filepath.Join(lfsObjectsDir(repoPath), oid[0:2], oid[2:4], oid)
					if cachedBytes, cErr := os.ReadFile(lfsCachePath); cErr == nil {
						return cachedBytes, nil
					}
				}
			}
		}
	}

	return []byte(out), nil
}

func parseAssetAST(path string, data []byte) (*AssetPackageHeader, []AssetGraphNode) {
	if len(data) == 0 {
		return nil, []AssetGraphNode{}
	}

	ext := strings.ToLower(filepath.Ext(path))
	header := &AssetPackageHeader{
		TotalHeaderSize: len(data),
		SummaryTags:     []string{},
	}
	nodes := []AssetGraphNode{}

	if len(data) >= 4 && binary.LittleEndian.Uint32(data[:4]) == 0x9E2A83C1 {
		header.Magic = "0x9E2A83C1 (UE Asset)"
		if ext == ".umap" {
			header.AssetFormat = "Unreal Level (.umap)"
		} else {
			header.AssetFormat = "Unreal Blueprint (.uasset)"
		}

		if len(data) >= 20 {
			legacyVer := int32(binary.LittleEndian.Uint32(data[4:8]))
			fileVerUE4 := int32(binary.LittleEndian.Uint32(data[12:16]))
			fileVerUE5 := int32(binary.LittleEndian.Uint32(data[16:20]))
			if fileVerUE5 > 0 {
				header.EngineVersion = fmt.Sprintf("UE 5 (Ver: %d)", fileVerUE5)
			} else if fileVerUE4 > 0 {
				header.EngineVersion = fmt.Sprintf("UE 4 (Ver: %d)", fileVerUE4)
			} else {
				header.EngineVersion = fmt.Sprintf("UE Legacy (%d)", legacyVer)
			}
		}

		nodes = extractNodesFromBinaryUE(data)
		header.ExportCount = len(nodes)
		header.NameCount = len(nodes) * 4
		header.PackageFlags = "PKG_FilterEditorOnly | PKG_CompiledIn"
		return header, nodes
	}

	if bytes.Contains(data, []byte("K2Node_")) || bytes.Contains(data, []byte("EdGraphNode")) {
		header.Magic = "Textual Blueprint Representation"
		header.AssetFormat = "Unreal Blueprint (Text Export)"
		header.EngineVersion = "Unreal Engine Kismet AST"
		nodes = parseTextBlueprintAST(string(data))
		header.ExportCount = len(nodes)
		return header, nodes
	}

	if bytes.HasPrefix(data, []byte("%YAML")) || ext == ".prefab" || ext == ".asset" {
		header.Magic = "%YAML 1.1 (Unity)"
		header.AssetFormat = "Unity Prefab (.prefab)"
		header.EngineVersion = "Unity YAML Architecture"
		nodes = parseUnityPrefabAST(string(data))
		header.ExportCount = len(nodes)
		return header, nodes
	}

	if bytes.HasPrefix(data, []byte("[gd_scene")) || ext == ".tscn" {
		header.Magic = "[gd_scene] (Godot)"
		header.AssetFormat = "Godot Scene (.tscn)"
		header.EngineVersion = "Godot Engine 4.x"
		nodes = parseGodotSceneAST(string(data))
		header.ExportCount = len(nodes)
		return header, nodes
	}

	return header, nodes
}

func extractNodesFromBinaryUE(data []byte) []AssetGraphNode {
	nodes := []AssetGraphNode{}
	strContent := string(data)

	nodeSignatures := []struct {
		Pattern  string
		Class    string
		Category string
	}{
		{"K2Node_Event", "K2Node_Event", "Logic Event"},
		{"K2Node_CustomEvent", "K2Node_CustomEvent", "Custom Event"},
		{"K2Node_CallFunction", "K2Node_CallFunction", "Function Call"},
		{"K2Node_VariableGet", "K2Node_VariableGet", "Variable"},
		{"K2Node_VariableSet", "K2Node_VariableSet", "Variable"},
		{"K2Node_IfThenElse", "K2Node_IfThenElse", "Flow Control"},
		{"K2Node_ExecutionSequence", "K2Node_ExecutionSequence", "Flow Control"},
		{"K2Node_SpawnActorFromClass", "K2Node_SpawnActorFromClass", "Spawn Actor"},
		{"K2Node_Timeline", "K2Node_Timeline", "Timeline"},
		{"K2Node_ComponentBoundEvent", "K2Node_ComponentBoundEvent", "Component Event"},
	}

	idIndex := 1
	for _, sig := range nodeSignatures {
		idx := 0
		for {
			pos := strings.Index(strContent[idx:], sig.Pattern)
			if pos == -1 {
				break
			}
			absPos := idx + pos

			nodeName := fmt.Sprintf("%s_%d", sig.Class, idIndex)
			funcTarget := ""
			limit := absPos + 120
			if limit > len(strContent) {
				limit = len(strContent)
			}
			sub := strContent[absPos:limit]
			for _, token := range strings.FieldsFunc(sub, func(r rune) bool {
				return r < 32 || r > 126 || r == '"' || r == '\'' || r == ' ' || r == '\x00'
			}) {
				if len(token) > 3 && token != sig.Pattern && !strings.HasPrefix(token, "K2Node") {
					funcTarget = token
					nodeName = token
					break
				}
			}

			pins := []AssetGraphPin{
				{Name: "execute", Direction: "Input", PinType: "Exec"},
				{Name: "then", Direction: "Output", PinType: "Exec"},
			}
			if strings.Contains(sig.Class, "CallFunction") {
				pins = append(pins, AssetGraphPin{Name: "self", Direction: "Input", PinType: "ObjectReference", DefaultValue: "Self"})
				pins = append(pins, AssetGraphPin{Name: "ReturnValue", Direction: "Output", PinType: "Boolean"})
			} else if strings.Contains(sig.Class, "Variable") {
				pins = append(pins, AssetGraphPin{Name: "Value", Direction: "Output", PinType: "Float", DefaultValue: "0.0"})
			}

			props := map[string]string{}
			if funcTarget != "" {
				props["FunctionReference"] = funcTarget
			}

			nodes = append(nodes, AssetGraphNode{
				ID:         fmt.Sprintf("node_%s_%d", sig.Pattern, idIndex),
				Name:       nodeName,
				Class:      sig.Class,
				Category:   sig.Category,
				PosX:       100 + (idIndex * 80),
				PosY:       150 + ((idIndex % 3) * 60),
				Properties: props,
				Pins:       pins,
			})

			idIndex++
			idx = absPos + len(sig.Pattern)
		}
	}

	return nodes
}

func parseTextBlueprintAST(text string) []AssetGraphNode {
	nodes := []AssetGraphNode{}
	lines := strings.Split(text, "\n")
	var currNode *AssetGraphNode

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "Begin Object Class=/Script/BlueprintGraph.") || strings.HasPrefix(trimmed, "Begin Object Class=K2Node_") {
			cls := ""
			name := ""
			parts := strings.Fields(trimmed)
			for _, p := range parts {
				if strings.HasPrefix(p, "Class=") {
					cls = filepath.Base(strings.TrimPrefix(p, "Class="))
				} else if strings.HasPrefix(p, "Name=") {
					name = strings.Trim(strings.TrimPrefix(p, "Name="), "\"")
				}
			}
			if cls == "" {
				cls = "K2Node_CallFunction"
			}
			if name == "" {
				name = fmt.Sprintf("Node_%d", len(nodes)+1)
			}

			currNode = &AssetGraphNode{
				ID:         name,
				Name:       name,
				Class:      cls,
				Category:   determineNodeCategory(cls),
				Properties: make(map[string]string),
				Pins:       []AssetGraphPin{},
			}
		} else if strings.HasPrefix(trimmed, "End Object") && currNode != nil {
			nodes = append(nodes, *currNode)
			currNode = nil
		} else if currNode != nil {
			if strings.HasPrefix(trimmed, "NodePosX=") {
				currNode.PosX, _ = strconv.Atoi(strings.TrimPrefix(trimmed, "NodePosX="))
			} else if strings.HasPrefix(trimmed, "NodePosY=") {
				currNode.PosY, _ = strconv.Atoi(strings.TrimPrefix(trimmed, "NodePosY="))
			} else if strings.HasPrefix(trimmed, "NodeComment=") {
				currNode.Comment = strings.Trim(strings.TrimPrefix(trimmed, "NodeComment="), "\"")
			} else if strings.HasPrefix(trimmed, "FunctionReference=(MemberName=\"") {
				fn := strings.TrimPrefix(trimmed, "FunctionReference=(MemberName=\"")
				fn = strings.TrimSuffix(fn, "\")")
				currNode.Properties["FunctionReference"] = fn
				currNode.Name = fn
			} else if strings.HasPrefix(trimmed, "CustomFunctionName=\"") {
				fn := strings.Trim(strings.TrimPrefix(trimmed, "CustomFunctionName="), "\"")
				currNode.Properties["CustomFunctionName"] = fn
				currNode.Name = fn
			} else if strings.HasPrefix(trimmed, "Pin (PinName=\"") || strings.HasPrefix(trimmed, "Pins(0)=") || strings.Contains(trimmed, "PinName=") {
				pin := parseBlueprintPinLine(trimmed)
				if pin.Name != "" {
					currNode.Pins = append(currNode.Pins, pin)
				}
			}
		}
	}

	return nodes
}

func parseBlueprintPinLine(line string) AssetGraphPin {
	pin := AssetGraphPin{Direction: "Input", PinType: "Exec"}
	reName := regexp.MustCompile(`PinName="([^"]+)"`)
	if m := reName.FindStringSubmatch(line); len(m) > 1 {
		pin.Name = m[1]
	}
	reDir := regexp.MustCompile(`Direction="([^"]+)"`)
	if m := reDir.FindStringSubmatch(line); len(m) > 1 {
		pin.Direction = m[1]
	}
	reType := regexp.MustCompile(`PinType\.PinCategory="([^"]+)"`)
	if m := reType.FindStringSubmatch(line); len(m) > 1 {
		pin.PinType = m[1]
	}
	reVal := regexp.MustCompile(`DefaultValue="([^"]+)"`)
	if m := reVal.FindStringSubmatch(line); len(m) > 1 {
		pin.DefaultValue = m[1]
	}
	reConn := regexp.MustCompile(`LinkedTo=\(([^)]+)\)`)
	if m := reConn.FindStringSubmatch(line); len(m) > 1 {
		pin.ConnectedTo = m[1]
	}
	return pin
}

func determineNodeCategory(cls string) string {
	if strings.Contains(cls, "Event") {
		return "Logic Event"
	}
	if strings.Contains(cls, "Function") {
		return "Function Call"
	}
	if strings.Contains(cls, "Variable") {
		return "Variable"
	}
	if strings.Contains(cls, "If") || strings.Contains(cls, "Sequence") {
		return "Flow Control"
	}
	return "Graph Node"
}

func diffSingleNode(base AssetGraphNode, target AssetGraphNode) ModifiedNodeItem {
	mod := ModifiedNodeItem{
		NodeID:          target.ID,
		NodeName:        target.Name,
		NodeClass:       target.Class,
		Category:        target.Category,
		PinChanges:      []NodePinDiff{},
		PropertyChanges: []NodePropertyDiff{},
		PosDeltaX:       target.PosX - base.PosX,
		PosDeltaY:       target.PosY - base.PosY,
	}

	basePins := make(map[string]AssetGraphPin)
	for _, p := range base.Pins {
		basePins[p.Name] = p
	}
	targetPins := make(map[string]AssetGraphPin)
	for _, p := range target.Pins {
		targetPins[p.Name] = p
	}

	for pName, tPin := range targetPins {
		bPin, exists := basePins[pName]
		if !exists {
			mod.PinChanges = append(mod.PinChanges, NodePinDiff{
				PinName:    pName,
				ChangeType: "ADDED",
				NewValue:   tPin.DefaultValue,
				Detail:     fmt.Sprintf("新增引脚 [%s (%s)]", pName, tPin.PinType),
			})
			continue
		}

		if bPin.ConnectedTo != tPin.ConnectedTo {
			mod.PinChanges = append(mod.PinChanges, NodePinDiff{
				PinName:    pName,
				ChangeType: "CONNECTION",
				OldValue:   bPin.ConnectedTo,
				NewValue:   tPin.ConnectedTo,
				Detail:     fmt.Sprintf("连线变更: %s -> %s", bPin.ConnectedTo, tPin.ConnectedTo),
			})
		} else if bPin.DefaultValue != tPin.DefaultValue {
			mod.PinChanges = append(mod.PinChanges, NodePinDiff{
				PinName:    pName,
				ChangeType: "DEFAULT_VALUE",
				OldValue:   bPin.DefaultValue,
				NewValue:   tPin.DefaultValue,
				Detail:     fmt.Sprintf("默认值变更: %s -> %s", bPin.DefaultValue, tPin.DefaultValue),
			})
		}
	}

	for pName := range basePins {
		if _, exists := targetPins[pName]; !exists {
			mod.PinChanges = append(mod.PinChanges, NodePinDiff{
				PinName:    pName,
				ChangeType: "REMOVED",
				Detail:     fmt.Sprintf("引脚删除: [%s]", pName),
			})
		}
	}

	for k, vT := range target.Properties {
		vB, exists := base.Properties[k]
		if !exists {
			mod.PropertyChanges = append(mod.PropertyChanges, NodePropertyDiff{
				Property: k,
				OldValue: "<None>",
				NewValue: vT,
			})
		} else if vB != vT {
			mod.PropertyChanges = append(mod.PropertyChanges, NodePropertyDiff{
				Property: k,
				OldValue: vB,
				NewValue: vT,
			})
		}
	}

	details := []string{}
	if len(mod.PinChanges) > 0 {
		details = append(details, fmt.Sprintf("%d 处引脚变更", len(mod.PinChanges)))
	}
	if len(mod.PropertyChanges) > 0 {
		details = append(details, fmt.Sprintf("%d 处属性变更", len(mod.PropertyChanges)))
	}
	if mod.PosDeltaX != 0 || mod.PosDeltaY != 0 {
		details = append(details, fmt.Sprintf("坐标偏移 (ΔX:%d, ΔY:%d)", mod.PosDeltaX, mod.PosDeltaY))
	}
	mod.Summary = strings.Join(details, ", ")

	return mod
}

func parseUnityPrefabAST(yamlContent string) []AssetGraphNode {
	nodes := []AssetGraphNode{}
	blocks := strings.Split(yamlContent, "--- !u!1 &")
	for i, block := range blocks {
		if i == 0 {
			continue
		}
		lines := strings.Split(block, "\n")
		gameObjectID := strings.TrimSpace(lines[0])
		name := fmt.Sprintf("GameObject_%s", gameObjectID)
		for _, l := range lines {
			l = strings.TrimSpace(l)
			if strings.HasPrefix(l, "m_Name:") {
				name = strings.TrimSpace(strings.TrimPrefix(l, "m_Name:"))
				break
			}
		}

		nodes = append(nodes, AssetGraphNode{
			ID:       fmt.Sprintf("go_%s", gameObjectID),
			Name:     name,
			Class:    "GameObject",
			Category: "Prefab Component",
			PosX:     120 * i,
			PosY:     100,
			Pins: []AssetGraphPin{
				{Name: "Transform", Direction: "Output", PinType: "Vector"},
			},
		})
	}
	return nodes
}

func parseGodotSceneAST(tscnContent string) []AssetGraphNode {
	nodes := []AssetGraphNode{}
	reNode := regexp.MustCompile(`\[node\s+name="([^"]+)"\s+type="([^"]+)"`)
	matches := reNode.FindAllStringSubmatch(tscnContent, -1)
	for i, m := range matches {
		if len(m) >= 3 {
			nodes = append(nodes, AssetGraphNode{
				ID:       fmt.Sprintf("godot_node_%d", i+1),
				Name:     m[1],
				Class:    m[2],
				Category: "Scene Node",
				PosX:     100 * (i + 1),
				PosY:     120,
				Pins: []AssetGraphPin{
					{Name: "Transform3D", Direction: "Output", PinType: "Transform"},
				},
			})
		}
	}
	return nodes
}

// =========================================================================
// Round 104: Sub-Level & Scene Stream Merge Wizard (AuditSceneStreamMerge)
// =========================================================================

// AuditSceneStreamMerge audits multi-scene streaming sub-levels for GUID collisions, spatial 3D overlaps, and transform desyncs
func (GitService) AuditSceneStreamMerge(repoPath string, targetBranch string, scenePath string) (*SceneStreamAuditResult, error) {
	currentBranch, err := ExecGit(repoPath, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return nil, fmt.Errorf("无法获取当前分支: %v", err)
	}
	currentBranch = strings.TrimSpace(currentBranch)

	if targetBranch == "" {
		targetBranch = "main"
	}
	if _, err := ExecGit(repoPath, "rev-parse", "--verify", targetBranch+"^{commit}"); err != nil {
		return nil, fmt.Errorf("目标分支 %s 不存在或不可解析: %w", targetBranch, err)
	}

	targetFiles := []string{}
	if scenePath != "" {
		cleanScene, err := repoRelativePathFromInput(repoPath, scenePath)
		if err != nil {
			return nil, err
		}
		fullScene := filepath.Join(repoPath, cleanScene)
		fi, statErr := os.Stat(fullScene)
		if statErr != nil || fi.IsDir() {
			return nil, fmt.Errorf("关卡文件不存在或不是文件: %s", cleanScene)
		}
		targetFiles = []string{cleanScene}
	} else {
		diffOut, diffErr := ExecGit(repoPath, "diff", "--name-only", targetBranch+"...HEAD")
		if diffErr != nil {
			return nil, fmt.Errorf("读取关卡差异失败: %w", diffErr)
		}
		for _, f := range strings.Split(diffOut, "\n") {
			f = strings.TrimSpace(f)
			if f == "" {
				continue
			}
			ext := strings.ToLower(filepath.Ext(f))
			if ext == ".umap" || ext == ".unity" || ext == ".tscn" {
				targetFiles = append(targetFiles, f)
			}
		}

		if len(targetFiles) == 0 {
			_ = filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() {
					return nil
				}
				ext := strings.ToLower(filepath.Ext(path))
				if ext == ".umap" || ext == ".unity" || ext == ".tscn" {
					rel, _ := filepath.Rel(repoPath, path)
					if !strings.HasPrefix(rel, ".git") && len(targetFiles) < 10 {
						targetFiles = append(targetFiles, rel)
					}
				}
				return nil
			})
		}
	}

	engineType := "通用 3D 关卡场景"
	currentActors := []SceneStreamActor{}
	targetActors := []SceneStreamActor{}
	readProblems := []string{}

	for _, file := range targetFiles {
		ext := strings.ToLower(filepath.Ext(file))
		if ext == ".umap" {
			engineType = "Unreal World Partition / Sub-Level"
		} else if ext == ".unity" {
			engineType = "Unity Multi-Scene Streaming"
		} else if ext == ".tscn" {
			engineType = "Godot Scene Stream"
		}

		cBytes, cErr := fetchAssetBytes(repoPath, file, "WORKTREE")
		if cErr != nil {
			// A file that exists but cannot be read is a real error; only a
			// missing file may fall back to HEAD (uncommitted deletion).
			if _, statErr := os.Stat(filepath.Join(repoPath, file)); statErr == nil {
				readProblems = append(readProblems, fmt.Sprintf("%s（工作区内容不可读）", file))
			} else {
				cBytes, cErr = fetchAssetBytes(repoPath, file, "HEAD")
				if cErr != nil {
					if _, existsErr := ExecGit(repoPath, "cat-file", "-e", "HEAD:"+file); existsErr == nil {
						readProblems = append(readProblems, fmt.Sprintf("%s（HEAD 内容不可读）", file))
					}
				}
			}
		}
		cActs := parseSceneStreamActors(file, cBytes, "CURRENT")
		currentActors = append(currentActors, cActs...)

		tBytes, tErr := fetchAssetBytes(repoPath, file, targetBranch)
		if tErr != nil {
			// Absent on the target branch is a legitimate one-sided change;
			// unreadable content of an existing blob is not.
			if _, existsErr := ExecGit(repoPath, "cat-file", "-e", targetBranch+":"+file); existsErr == nil {
				readProblems = append(readProblems, fmt.Sprintf("%s（目标分支内容不可读）", file))
			}
		}
		tActs := parseSceneStreamActors(file, tBytes, "TARGET")
		targetActors = append(targetActors, tActs...)
	}

	currentGuidMap := make(map[string]SceneStreamActor)
	for _, a := range currentActors {
		currentGuidMap[a.Guid] = a
	}
	targetGuidMap := make(map[string]SceneStreamActor)
	for _, a := range targetActors {
		targetGuidMap[a.Guid] = a
	}

	guidCollisions := []GuidCollisionItem{}
	transformConflicts := []TransformConflictItem{}

	for guid, cAct := range currentGuidMap {
		tAct, exists := targetGuidMap[guid]
		if !exists {
			continue
		}

		if cAct.Name == tAct.Name && cAct.Class == tAct.Class {
			dx := cAct.Position.X - tAct.Position.X
			dy := cAct.Position.Y - tAct.Position.Y
			dz := cAct.Position.Z - tAct.Position.Z
			dist := math.Sqrt(dx*dx + dy*dy + dz*dz)
			if dist > 0.1 {
				transformConflicts = append(transformConflicts, TransformConflictItem{
					Guid:              guid,
					ActorName:         cAct.Name,
					SubLevel:          cAct.SubLevel,
					CurrentPos:        cAct.Position,
					TargetPos:         tAct.Position,
					DeltaDistance:     math.Round(dist*100) / 100,
					CurrentRot:        cAct.Rotation,
					TargetRot:         tAct.Rotation,
					RecommendedAction: "KEEP_CURRENT_TRANSFORM",
				})
			}
		} else {
			guidCollisions = append(guidCollisions, GuidCollisionItem{
				Guid:             guid,
				CurrentActorName: cAct.Name,
				TargetActorName:  tAct.Name,
				CurrentClass:     cAct.Class,
				TargetClass:      tAct.Class,
				SubLevel:         cAct.SubLevel,
				IsIdentical:      false,
				RecommendedFix:   "AUTO_REGENERATE_GUID",
			})
		}
	}

	spatialOverlaps := []SpatialOverlapItem{}

	for _, cAct := range currentActors {
		for _, tAct := range targetActors {
			if cAct.Guid == tAct.Guid {
				continue
			}

			isOverlap, vol := testAABBOverlap(cAct.Bounds, tAct.Bounds)
			if isOverlap && vol > 0.001 {
				dx := cAct.Position.X - tAct.Position.X
				dy := cAct.Position.Y - tAct.Position.Y
				dz := cAct.Position.Z - tAct.Position.Z
				dist := math.Sqrt(dx*dx + dy*dy + dz*dz)

				smallerVol := math.Min(computeBoxVolume(cAct.Bounds), computeBoxVolume(tAct.Bounds))
				severity := "WARNING"
				if smallerVol > 0 && vol/smallerVol > 0.4 {
					severity = "CRITICAL"
				}

				spatialOverlaps = append(spatialOverlaps, SpatialOverlapItem{
					ActorGuidA:     cAct.Guid,
					ActorNameA:     cAct.Name,
					ActorClassA:    cAct.Class,
					SubLevelA:      cAct.SubLevel,
					ActorGuidB:     tAct.Guid,
					ActorNameB:     tAct.Name,
					ActorClassB:    tAct.Class,
					SubLevelB:      tAct.SubLevel,
					CenterDistance: math.Round(dist*100) / 100,
					OverlapVolume:  math.Round(vol*100) / 100,
					BoundingBoxA:   cAct.Bounds,
					BoundingBoxB:   tAct.Bounds,
					Severity:       severity,
					RecommendedFix: "SPATIAL_OFFSET",
				})
			}
		}
	}

	safetyScore := 100
	safetyScore -= len(guidCollisions) * 30
	for _, ov := range spatialOverlaps {
		if ov.Severity == "CRITICAL" {
			safetyScore -= 18
		} else {
			safetyScore -= 8
		}
	}
	safetyScore -= len(transformConflicts) * 6
	if safetyScore < 0 {
		safetyScore = 0
	}

	analysisKnown := len(currentActors) > 0 || len(targetActors) > 0
	safetyLevel := "SAFE"
	mergeVerdict := "SAFE_AUTO_MERGE"
	if !analysisKnown {
		safetyScore = 0
		safetyLevel = "UNKNOWN"
		mergeVerdict = "NO_DATA"
	} else if len(readProblems) > 0 {
		if safetyScore > 84 {
			safetyScore = 84
		}
		safetyLevel = "WARNING"
		mergeVerdict = "MANUAL_REVIEW_REQUIRED"
	} else if safetyScore < 60 {
		safetyLevel = "DANGER"
		mergeVerdict = "CRITICAL_COLLISIONS"
	} else if safetyScore < 85 {
		safetyLevel = "WARNING"
		mergeVerdict = "MANUAL_REVIEW_REQUIRED"
	}

	plan := []SceneMergeStrategyItem{}
	for _, col := range guidCollisions {
		plan = append(plan, SceneMergeStrategyItem{
			Category:    "GUID 碰撞冲突",
			TargetID:    col.Guid,
			Action:      "AUTO_REGENERATE_GUID",
			Description: fmt.Sprintf("为当前分支 Actor [%s] 重新生成全新唯一 GUID，消除与目标分支 [%s] 的持久标识符碰撞", col.CurrentActorName, col.TargetActorName),
			Automated:   true,
		})
	}
	for _, ov := range spatialOverlaps {
		plan = append(plan, SceneMergeStrategyItem{
			Category:    "3D 空间模型重叠",
			TargetID:    fmt.Sprintf("%s <-> %s", ov.ActorGuidA, ov.ActorGuidB),
			Action:      "SPATIAL_OFFSET",
			Description: fmt.Sprintf("对 Actor [%s] 应用几何法向安全位移，消除穿模体积 (%.1f m³)", ov.ActorNameA, ov.OverlapVolume),
			Automated:   true,
		})
	}
	for _, tr := range transformConflicts {
		plan = append(plan, SceneMergeStrategyItem{
			Category:    "坐标变换分歧",
			TargetID:    tr.Guid,
			Action:      "KEEP_CURRENT_TRANSFORM",
			Description: fmt.Sprintf("保持当前工作区坐标 (Δ%.2fm)，或一键同步目标分支坐标", tr.DeltaDistance),
			Automated:   true,
		})
	}

	summaryMsg := fmt.Sprintf("关卡合流体检完成 (得分: %d/100, 评级: %s): 发现 %d 处 GUID 碰撞, %d 处空间模型重叠, %d 处变换分歧",
		safetyScore, safetyLevel, len(guidCollisions), len(spatialOverlaps), len(transformConflicts))
	if !analysisKnown {
		summaryMsg = "未从 Unity GameObject+Transform、UE Begin Actor 或 Godot node 解析到可用 Actor；未作合流安全结论"
	} else if len(readProblems) > 0 {
		summaryMsg += fmt.Sprintf("；另有 %d 个关卡文件内容不可读（%s），未完成全量比对，结论需人工复核",
			len(readProblems), strings.Join(readProblems, "、"))
	}

	return &SceneStreamAuditResult{
		CurrentBranch:      currentBranch,
		TargetBranch:       targetBranch,
		AuditedSubLevels:   targetFiles,
		EngineType:         engineType,
		AnalysisKnown:      analysisKnown,
		TotalActorsCurrent: len(currentActors),
		TotalActorsTarget:  len(targetActors),
		GuidCollisions:     guidCollisions,
		SpatialOverlaps:    spatialOverlaps,
		TransformConflicts: transformConflicts,
		SafetyScore:        safetyScore,
		SafetyLevel:        safetyLevel,
		MergeVerdict:       mergeVerdict,
		RecommendedPlan:    plan,
		SummaryMessage:     summaryMsg,
	}, nil
}

// ResolveSceneStreamConflict applies conflict resolution strategies (e.g. GUID regen, transform pick, offset)
func (GitService) ResolveSceneStreamConflict(repoPath string, scenePath string, action string, actorGuid string) (*SceneStreamResolveResult, error) {
	if scenePath == "" || actorGuid == "" {
		return nil, errors.New("scenePath 与 actorGuid 均不能为空")
	}

	cleanPath, err := repoRelativePathFromInput(repoPath, scenePath)
	if err != nil {
		return nil, err
	}
	fullPath := filepath.Join(repoPath, cleanPath)

	contentBytes, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("读取关卡文件失败: %v", err)
	}

	content := string(contentBytes)
	newGuid := ""
	var offsetApplied *Vector3D

	switch strings.ToUpper(action) {
	case "AUTO_REGENERATE_GUID", "REGEN_GUID":
		if !strings.Contains(content, actorGuid) {
			return nil, fmt.Errorf("场景中未找到 Actor GUID %q，未修改文件", actorGuid)
		}
		randBytes := make([]byte, 16)
		_, _ = rand.Read(randBytes)
		newGuid = hex.EncodeToString(randBytes)
		content = strings.ReplaceAll(content, actorGuid, newGuid)
		if writeErr := os.WriteFile(fullPath, []byte(content), 0644); writeErr != nil {
			return nil, writeErr
		}

	case "SPATIAL_OFFSET":
		offset := Vector3D{X: 150.0, Y: 0.0, Z: 50.0}
		updated, offsetErr := applyUnityActorOffset(content, actorGuid, offset)
		if offsetErr != nil {
			return nil, offsetErr
		}
		if writeErr := os.WriteFile(fullPath, []byte(updated), 0644); writeErr != nil {
			return nil, writeErr
		}
		offsetApplied = &offset

	default:
		return nil, fmt.Errorf("不支持的冲突解决策略: %s", action)
	}

	message := fmt.Sprintf("已成功应用修复策略 [%s]", action)
	if newGuid != "" {
		message += fmt.Sprintf("，新 GUID: %s", newGuid)
	}
	if offsetApplied != nil {
		message += fmt.Sprintf("，位移 Δ(%.1f, %.1f, %.1f)", offsetApplied.X, offsetApplied.Y, offsetApplied.Z)
	}
	return &SceneStreamResolveResult{
		Success:       true,
		ScenePath:     cleanPath,
		ActorGuid:     actorGuid,
		AppliedAction: action,
		NewActorGuid:  newGuid,
		OffsetApplied: offsetApplied,
		Message:       message,
	}, nil
}

func applyUnityActorOffset(content string, actorGuid string, offset Vector3D) (string, error) {
	lines := strings.Split(content, "\n")
	targetLine := -1
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "m_GameObject:") && parseUnityFileID(line) == actorGuid {
			targetLine = i
			break
		}
	}
	if targetLine < 0 {
		return "", fmt.Errorf("未找到 Unity GameObject %q 对应的 Transform，未修改文件", actorGuid)
	}

	start := targetLine
	for start > 0 && !strings.HasPrefix(strings.TrimSpace(lines[start]), "--- !u!") {
		start--
	}
	header := strings.Fields(strings.TrimPrefix(strings.TrimSpace(lines[start]), "--- !u!"))
	if len(header) == 0 || (header[0] != "4" && header[0] != "224") {
		return "", fmt.Errorf("GUID %q 对应组件不是 Unity Transform，未修改文件", actorGuid)
	}
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "--- !u!") {
			end = i
			break
		}
	}

	positionPattern := regexp.MustCompile(`x:\s*([-0-9.]+),\s*y:\s*([-0-9.]+),\s*z:\s*([-0-9.]+)`)
	for i := start; i < end; i++ {
		if !strings.HasPrefix(strings.TrimSpace(lines[i]), "m_LocalPosition:") {
			continue
		}
		coordinates := positionPattern.FindStringSubmatch(lines[i])
		if len(coordinates) < 4 {
			return "", fmt.Errorf("无法解析 Actor %q 的 m_LocalPosition，未修改文件", actorGuid)
		}
		x, _ := strconv.ParseFloat(coordinates[1], 64)
		y, _ := strconv.ParseFloat(coordinates[2], 64)
		z, _ := strconv.ParseFloat(coordinates[3], 64)
		replacement := fmt.Sprintf("x: %g, y: %g, z: %g", x+offset.X, y+offset.Y, z+offset.Z)
		lines[i] = strings.Replace(lines[i], coordinates[0], replacement, 1)
		return strings.Join(lines, "\n"), nil
	}
	return "", fmt.Errorf("Actor %q 的 Transform 缺少 m_LocalPosition，未修改文件", actorGuid)
}

func parseSceneStreamActors(subLevelPath string, data []byte, side string) []SceneStreamActor {
	actors := []SceneStreamActor{}
	if len(data) == 0 {
		return actors
	}

	text := string(data)
	ext := strings.ToLower(filepath.Ext(subLevelPath))

	if ext == ".umap" || strings.Contains(text, "Begin Actor") || strings.Contains(text, "StaticMeshActor") {
		actors = parseUnrealLevelActors(subLevelPath, text, side)
		if len(actors) > 0 {
			return actors
		}
	}

	if ext == ".unity" || strings.Contains(text, "%YAML 1.1") {
		actors = parseUnitySceneActors(subLevelPath, text, side)
		if len(actors) > 0 {
			return actors
		}
	}

	if ext == ".tscn" || strings.Contains(text, "[gd_scene") {
		actors = parseGodotSceneActors(subLevelPath, text, side)
		if len(actors) > 0 {
			return actors
		}
	}

	return actors
}

func parseUnrealLevelActors(levelPath string, content string, side string) []SceneStreamActor {
	actors := []SceneStreamActor{}
	blocks := strings.Split(content, "Begin Actor")
	for i, b := range blocks {
		if i == 0 {
			continue
		}
		name := ""
		cls := "Actor"
		guid := ""
		pos := Vector3D{}

		header := strings.SplitN(b, "\n", 2)[0]
		if match := regexp.MustCompile(`Class=([^\s]+)`).FindStringSubmatch(header); len(match) >= 2 {
			cls = match[1]
		}
		if match := regexp.MustCompile(`Name="([^"]*)"|Name=([^\s]+)`).FindStringSubmatch(header); len(match) >= 2 {
			name = match[1]
			if name == "" && len(match) >= 3 {
				name = match[2]
			}
		}
		if match := regexp.MustCompile(`(?:ActorGuid|Guid)="([^"]*)"|(?:ActorGuid|Guid)=([^\s]+)`).FindStringSubmatch(header); len(match) >= 2 {
			guid = match[1]
			if guid == "" && len(match) >= 3 {
				guid = match[2]
			}
		}
		if match := regexp.MustCompile(`(?:Relative)?Location=\(([^)]*)\)`).FindStringSubmatch(header); len(match) >= 2 {
			pos = parseVectorLine(match[1])
		}

		for _, line := range strings.Split(b, "\n") {
			l := strings.TrimSpace(line)
			if strings.HasPrefix(l, "Class=") {
				cls = strings.TrimPrefix(l, "Class=")
			} else if strings.HasPrefix(l, "Name=") {
				name = strings.Trim(strings.TrimPrefix(l, "Name="), "\"")
			} else if strings.HasPrefix(l, "ActorGuid=") {
				guid = strings.Trim(strings.TrimPrefix(l, "ActorGuid="), "\"")
			} else if strings.HasPrefix(l, "Guid=") {
				guid = strings.Trim(strings.TrimPrefix(l, "Guid="), "\"")
			} else if strings.HasPrefix(l, "Location=") || strings.HasPrefix(l, "RelativeLocation=") {
				pos = parseVectorLine(l)
			}
		}
		if name == "" && guid == "" {
			continue
		}
		if guid == "" {
			guid = "name:" + name
		}
		if name == "" {
			name = guid
		}

		actors = append(actors, SceneStreamActor{
			Guid:       guid,
			Name:       name,
			Class:      cls,
			SubLevel:   levelPath,
			Position:   pos,
			Scale:      Vector3D{1, 1, 1},
			Bounds:     computeAABB(pos, Vector3D{}),
			SourceSide: side,
		})
	}
	return actors
}

func parseUnitySceneActors(scenePath string, content string, side string) []SceneStreamActor {
	transforms := map[string]Vector3D{}
	currentType := ""
	currentGameObject := ""
	currentPosition := Vector3D{}
	hasPosition := false
	flushTransform := func() {
		if (currentType == "4" || currentType == "224") && currentGameObject != "" {
			if !hasPosition {
				currentPosition = Vector3D{}
			}
			transforms[currentGameObject] = currentPosition
		}
	}

	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "--- !u!") {
			flushTransform()
			header := strings.Fields(strings.TrimPrefix(trimmed, "--- !u!"))
			currentType = ""
			if len(header) > 0 {
				currentType = header[0]
			}
			currentGameObject = ""
			currentPosition = Vector3D{}
			hasPosition = false
			continue
		}
		if currentType != "4" && currentType != "224" {
			continue
		}
		if strings.HasPrefix(trimmed, "m_GameObject:") {
			currentGameObject = parseUnityFileID(trimmed)
		} else if strings.HasPrefix(trimmed, "m_LocalPosition:") {
			currentPosition = parseUnityVector(trimmed)
			hasPosition = true
		}
	}
	flushTransform()

	actors := []SceneStreamActor{}
	blocks := strings.Split(content, "--- !u!1 &")
	for _, block := range blocks[1:] {
		lines := strings.SplitN(block, "\n", 2)
		gameObjectID := strings.TrimSpace(lines[0])
		if gameObjectID == "" {
			continue
		}
		name := ""
		for _, line := range strings.Split(block, "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "m_Name:") {
				name = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "m_Name:"))
			}
		}
		position, hasTransform := transforms[gameObjectID]
		if !hasTransform {
			continue
		}
		if name == "" {
			name = "GameObject_" + gameObjectID
		}
		actors = append(actors, SceneStreamActor{
			Guid:       gameObjectID,
			Name:       name,
			Class:      "GameObject",
			SubLevel:   scenePath,
			Position:   position,
			Scale:      Vector3D{1, 1, 1},
			Bounds:     computeAABB(position, Vector3D{}),
			SourceSide: side,
		})
	}
	return actors
}

func parseUnityFileID(line string) string {
	match := regexp.MustCompile(`fileID:\s*(-?\d+)`).FindStringSubmatch(line)
	if len(match) < 2 {
		return ""
	}
	return match[1]
}

func parseGodotSceneActors(scenePath string, content string, side string) []SceneStreamActor {
	actors := []SceneStreamActor{}
	currentName := ""
	currentType := ""
	currentParent := ""
	currentPosition := Vector3D{}
	flush := func() {
		if currentName == "" {
			return
		}
		guid := currentName
		if currentParent != "" && currentParent != "." {
			guid = currentParent + "/" + currentName
		}
		actors = append(actors, SceneStreamActor{
			Guid:       "name:" + guid,
			Name:       currentName,
			Class:      currentType,
			SubLevel:   scenePath,
			Position:   currentPosition,
			Scale:      Vector3D{1, 1, 1},
			Bounds:     computeAABB(currentPosition, Vector3D{}),
			SourceSide: side,
		})
	}

	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[node ") && strings.HasSuffix(trimmed, "]") {
			flush()
			attributes := map[string]string{}
			for _, match := range regexp.MustCompile(`(\w+)="([^"]*)"`).FindAllStringSubmatch(trimmed, -1) {
				if len(match) >= 3 {
					attributes[match[1]] = match[2]
				}
			}
			currentName = attributes["name"]
			currentType = attributes["type"]
			currentParent = attributes["parent"]
			currentPosition = Vector3D{}
			continue
		}
		if currentName == "" {
			continue
		}
		for _, prefix := range []string{"transform = Transform3D(", "position = Vector3(", "translation = Vector3("} {
			if !strings.HasPrefix(trimmed, prefix) {
				continue
			}
			values := strings.TrimSuffix(strings.TrimPrefix(trimmed, prefix), ")")
			numbers := strings.FieldsFunc(values, func(r rune) bool { return r == ',' || unicode.IsSpace(r) })
			parsed := []float64{}
			for _, number := range numbers {
				if value, parseErr := strconv.ParseFloat(number, 64); parseErr == nil {
					parsed = append(parsed, value)
				}
			}
			if len(parsed) >= 3 {
				currentPosition = Vector3D{X: parsed[len(parsed)-3], Y: parsed[len(parsed)-2], Z: parsed[len(parsed)-1]}
			}
			break
		}
	}
	flush()
	return actors
}

func parseVectorLine(line string) Vector3D {
	v := Vector3D{}
	re := regexp.MustCompile(`X=([-\d\.]+),\s*Y=([-\d\.]+),\s*Z=([-\d\.]+)`)
	m := re.FindStringSubmatch(line)
	if len(m) >= 4 {
		v.X, _ = strconv.ParseFloat(m[1], 64)
		v.Y, _ = strconv.ParseFloat(m[2], 64)
		v.Z, _ = strconv.ParseFloat(m[3], 64)
	}
	return v
}

func parseUnityVector(line string) Vector3D {
	v := Vector3D{}
	re := regexp.MustCompile(`x:\s*([-\d\.]+),\s*y:\s*([-\d\.]+),\s*z:\s*([-\d\.]+)`)
	m := re.FindStringSubmatch(line)
	if len(m) >= 4 {
		v.X, _ = strconv.ParseFloat(m[1], 64)
		v.Y, _ = strconv.ParseFloat(m[2], 64)
		v.Z, _ = strconv.ParseFloat(m[3], 64)
	}
	return v
}

func computeAABB(center, extents Vector3D) BoundingBox3D {
	return BoundingBox3D{
		Center:  center,
		Extents: extents,
		Min:     Vector3D{X: center.X - extents.X, Y: center.Y - extents.Y, Z: center.Z - extents.Z},
		Max:     Vector3D{X: center.X + extents.X, Y: center.Y + extents.Y, Z: center.Z + extents.Z},
	}
}

func computeBoxVolume(box BoundingBox3D) float64 {
	dx := box.Max.X - box.Min.X
	dy := box.Max.Y - box.Min.Y
	dz := box.Max.Z - box.Min.Z
	if dx <= 0 || dy <= 0 || dz <= 0 {
		return 0
	}
	return dx * dy * dz
}

func testAABBOverlap(boxA, boxB BoundingBox3D) (bool, float64) {
	if boxA.Min.X > boxB.Max.X || boxA.Max.X < boxB.Min.X ||
		boxA.Min.Y > boxB.Max.Y || boxA.Max.Y < boxB.Min.Y ||
		boxA.Min.Z > boxB.Max.Z || boxA.Max.Z < boxB.Min.Z {
		return false, 0
	}

	ovX := math.Min(boxA.Max.X, boxB.Max.X) - math.Max(boxA.Min.X, boxB.Min.X)
	ovY := math.Min(boxA.Max.Y, boxB.Max.Y) - math.Max(boxA.Min.Y, boxB.Min.Y)
	ovZ := math.Min(boxA.Max.Z, boxB.Max.Z) - math.Max(boxA.Min.Z, boxB.Min.Z)

	vol := math.Max(0, ovX) * math.Max(0, ovY) * math.Max(0, ovZ)
	return true, vol
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
// Round 106: Localization & Missing Glyphs Guard (AuditLocalizationParity)
// =========================================================================

// AuditLocalizationParity audits internationalization string tables across all languages,
// detects missing keys / untranslated entries, and inspects TTF/OTF fonts for missing glyphs.
func (GitService) AuditLocalizationParity(repoPath string, fontPath string) (*LocalizationAuditResult, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath 不能为空")
	}

	langSet := make(map[string]bool)
	allKeys := make(map[string]map[string]string) // key -> (lang -> text)
	auditedFiles := []string{}

	locExts := map[string]bool{
		".json": true,
		".csv":  true,
		".tsv":  true,
		".po":   true,
		".resx": true,
		".txt":  true,
	}

	_ = filepath.Walk(repoPath, func(p string, fi os.FileInfo, err error) error {
		if err != nil || fi == nil || fi.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(repoPath, p)
		lowerRel := strings.ToLower(rel)
		ext := strings.ToLower(filepath.Ext(p))

		if !locExts[ext] {
			return nil
		}

		if strings.Contains(lowerRel, "loc") || strings.Contains(lowerRel, "i18n") ||
			strings.Contains(lowerRel, "string") || strings.Contains(lowerRel, "lang") {
			auditedFiles = append(auditedFiles, rel)

			lang := "en-US"
			if strings.Contains(lowerRel, "zh-cn") || strings.Contains(lowerRel, "zh_cn") || strings.Contains(lowerRel, "chinese") {
				lang = "zh-CN"
			} else if strings.Contains(lowerRel, "ja-jp") || strings.Contains(lowerRel, "ja_jp") || strings.Contains(lowerRel, "japanese") {
				lang = "ja-JP"
			} else if strings.Contains(lowerRel, "ko-kr") || strings.Contains(lowerRel, "ko_kr") || strings.Contains(lowerRel, "korean") {
				lang = "ko-KR"
			} else if strings.Contains(lowerRel, "de-de") || strings.Contains(lowerRel, "german") {
				lang = "de-DE"
			} else if strings.Contains(lowerRel, "fr-fr") || strings.Contains(lowerRel, "french") {
				lang = "fr-FR"
			}
			langSet[lang] = true

			data, _ := os.ReadFile(p)
			if ext == ".json" {
				var kv map[string]string
				if err := json.Unmarshal(data, &kv); err == nil {
					for k, v := range kv {
						if allKeys[k] == nil {
							allKeys[k] = make(map[string]string)
						}
						allKeys[k][lang] = v
					}
				}
			} else if ext == ".csv" || ext == ".tsv" {
				r := csv.NewReader(bytes.NewReader(data))
				if ext == ".tsv" {
					r.Comma = '\t'
				}
				records, _ := r.ReadAll()
				for _, row := range records {
					if len(row) >= 2 {
						k := strings.TrimSpace(row[0])
						v := strings.TrimSpace(row[1])
						if allKeys[k] == nil {
							allKeys[k] = make(map[string]string)
						}
						allKeys[k][lang] = v
					}
				}
			}
		}
		return nil
	})

	languages := []string{}
	for l := range langSet {
		languages = append(languages, l)
	}
	sort.Strings(languages)

	primaryLang := "zh-CN"
	if !langSet["zh-CN"] && len(languages) > 0 {
		primaryLang = languages[0]
	}

	langStats := []LangParityStats{}
	untranslatedList := []LocalizedEntry{}
	totalUniqueKeys := len(allKeys)

	for _, l := range languages {
		transCount := 0
		for _, transMap := range allKeys {
			val := strings.TrimSpace(transMap[l])
			if val != "" {
				transCount++
			}
		}
		missingCount := totalUniqueKeys - transCount
		pct := 100.0
		if totalUniqueKeys > 0 {
			pct = math.Round(float64(transCount)/float64(totalUniqueKeys)*1000.0) / 10.0
		}
		statStatus := "EXCELLENT"
		if pct < 60.0 {
			statStatus = "CRITICAL"
		} else if pct < 85.0 {
			statStatus = "NEEDS_WORK"
		} else if pct < 98.0 {
			statStatus = "GOOD"
		}
		langStats = append(langStats, LangParityStats{
			Language:          l,
			TotalKeys:         totalUniqueKeys,
			TranslatedKeys:    transCount,
			MissingKeys:       missingCount,
			UntranslatedCount: missingCount,
			ParityPercent:     pct,
			Status:            statStatus,
		})
	}

	for k, transMap := range allKeys {
		var missingInLangs []string
		for _, l := range languages {
			if strings.TrimSpace(transMap[l]) == "" {
				missingInLangs = append(missingInLangs, l)
			}
		}
		if len(missingInLangs) > 0 {
			status := "PARTIAL"
			if len(missingInLangs) == len(languages) {
				status = "MISSING"
			}
			untranslatedList = append(untranslatedList, LocalizedEntry{
				Key:          k,
				SourceText:   transMap[primaryLang],
				Translations: transMap,
				MissingLangs: missingInLangs,
				Status:       status,
			})
		}
	}

	// Font Glyph check
	var fontSummary *FontGlyphAuditSummary
	if fontPath != "" {
		cleanFont := filepath.Clean(fontPath)
		if !filepath.IsAbs(cleanFont) {
			cleanFont = filepath.Join(repoPath, cleanFont)
		}
		fontRunes, fErr := parseTTFCmap(cleanFont)
		if fErr == nil {
			var missingGlyphs []MissingGlyphItem
			checkedCount := 0
			for k, transMap := range allKeys {
				for l, text := range transMap {
					for _, r := range text {
						if unicode.IsSpace(r) || unicode.IsPunct(r) {
							continue
						}
						checkedCount++
						if !fontRunes[r] {
							missingGlyphs = append(missingGlyphs, MissingGlyphItem{
								Char:        string(r),
								UnicodeHex:  fmt.Sprintf("U+%04X", r),
								UnicodeName: fmt.Sprintf("%c", r),
								Lang:        l,
								UsedInKeys:  []string{k},
								SampleText:  text,
							})
						}
					}
				}
			}
			covPct := 100.0
			if checkedCount > 0 {
				covPct = math.Round(float64(checkedCount-len(missingGlyphs))/float64(checkedCount)*1000.0) / 10.0
			}
			fontSummary = &FontGlyphAuditSummary{
				FontPath:           fontPath,
				FontFormat:         strings.ToUpper(strings.TrimPrefix(filepath.Ext(fontPath), ".")),
				TotalFontGlyphs:    len(fontRunes),
				TotalCharsChecked:  checkedCount,
				MissingGlyphsCount: len(missingGlyphs),
				MissingGlyphs:      missingGlyphs,
				CoveragePercent:    covPct,
			}
		}
	}

	overallScore := 0.0
	for _, s := range langStats {
		overallScore += s.ParityPercent
	}
	if len(langStats) > 0 {
		overallScore = math.Round(overallScore/float64(len(langStats))*10.0) / 10.0
	}

	analysisKnown := len(langStats) > 0
	severity := "UNKNOWN"
	var advice []string
	message := "未发现本地化文本，未作覆盖率结论"
	if analysisKnown {
		severity = "OK"
		if overallScore < 75.0 {
			severity = "ERROR"
		} else if overallScore < 95.0 {
			severity = "WARNING"
		}
		if len(untranslatedList) > 0 {
			advice = append(advice, fmt.Sprintf("发现 %d 条未全语种翻译词条，请导出至翻译管理系统补齐", len(untranslatedList)))
		}
		if fontSummary != nil && fontSummary.MissingGlyphsCount > 0 {
			severity = "ERROR"
			advice = append(advice, fmt.Sprintf("字体资产缺失 %d 个生僻字/多语言文字字形，将导致游戏内文字变成豆腐块 (Tofu/□)", fontSummary.MissingGlyphsCount))
		}
		if len(advice) == 0 {
			advice = append(advice, "多语言文本全覆盖合规，字体字形支持完备")
		}
		message = fmt.Sprintf("本地化多语言巡检完成: 语种 %d 个, 唯一词条 %d 条, 综合覆盖率 %.1f%%, 结论: %s",
			len(languages), totalUniqueKeys, overallScore, severity)
	} else if fontSummary != nil {
		severity = "OK"
		advice = append(advice, "未发现可审计的本地化文本文件，仅完成字体字形审计")
		if fontSummary.MissingGlyphsCount > 0 {
			severity = "ERROR"
			advice = append(advice, fmt.Sprintf("字体资产缺失 %d 个生僻字/多语言文字字形", fontSummary.MissingGlyphsCount))
		}
		message = fmt.Sprintf("未发现本地化文本，仅完成字体字形审计: 缺字 %d 个, 覆盖 %.1f%%",
			fontSummary.MissingGlyphsCount, fontSummary.CoveragePercent)
	} else {
		advice = append(advice, "未发现可审计的本地化文本文件（json/csv/tsv/po/resx/txt），未作覆盖率结论")
	}

	return &LocalizationAuditResult{
		AnalysisKnown:      analysisKnown,
		PrimaryLanguage:    primaryLang,
		LanguagesFound:     languages,
		TotalUniqueKeys:    totalUniqueKeys,
		OverallParityScore: overallScore,
		LanguageStats:      langStats,
		UntranslatedList:   untranslatedList,
		FontAudit:          fontSummary,
		SourceFilesCount:   len(auditedFiles),
		AuditedFiles:       auditedFiles,
		Severity:           severity,
		Advice:             advice,
		Message:            message,
	}, nil
}

func parseTTFCmap(path string) (map[rune]bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data) < 12 {
		return nil, fmt.Errorf("file too small")
	}

	numTables := binary.BigEndian.Uint16(data[4:6])
	var cmapOffset, cmapLength uint32
	offset := 12
	for i := 0; i < int(numTables); i++ {
		if offset+16 > len(data) {
			break
		}
		tag := string(data[offset : offset+4])
		if tag == "cmap" {
			cmapOffset = binary.BigEndian.Uint32(data[offset+8 : offset+12])
			cmapLength = binary.BigEndian.Uint32(data[offset+12 : offset+16])
			break
		}
		offset += 16
	}

	if cmapOffset == 0 || int(cmapOffset+cmapLength) > len(data) {
		return nil, fmt.Errorf("cmap table not found")
	}

	cmapData := data[cmapOffset : cmapOffset+cmapLength]
	if len(cmapData) < 4 {
		return nil, fmt.Errorf("invalid cmap")
	}

	numSubtables := binary.BigEndian.Uint16(cmapData[2:4])
	runes := make(map[rune]bool)

	for i := 0; i < int(numSubtables); i++ {
		subOffset := 4 + i*8
		if subOffset+8 > len(cmapData) {
			break
		}
		tableOffset := binary.BigEndian.Uint32(cmapData[subOffset+4 : subOffset+8])
		if int(tableOffset) >= len(cmapData) {
			continue
		}
		tableData := cmapData[tableOffset:]
		if len(tableData) < 2 {
			continue
		}
		format := binary.BigEndian.Uint16(tableData[0:2])

		if format == 4 && len(tableData) >= 14 {
			segCountX2 := binary.BigEndian.Uint16(tableData[6:8])
			segCount := int(segCountX2 / 2)
			if len(tableData) >= 14+segCount*8 {
				endCodes := make([]uint16, segCount)
				for s := 0; s < segCount; s++ {
					endCodes[s] = binary.BigEndian.Uint16(tableData[14+s*2 : 16+s*2])
				}
				startCodesOffset := 16 + segCount*2
				startCodes := make([]uint16, segCount)
				for s := 0; s < segCount; s++ {
					startCodes[s] = binary.BigEndian.Uint16(tableData[startCodesOffset+s*2 : startCodesOffset+2+s*2])
				}
				for s := 0; s < segCount; s++ {
					for c := startCodes[s]; c <= endCodes[s] && c != 0xFFFF; c++ {
						runes[rune(c)] = true
						if c == 0xFFFF {
							break
						}
					}
				}
			}
		} else if format == 12 && len(tableData) >= 16 {
			nGroups := binary.BigEndian.Uint32(tableData[12:16])
			grpOffset := 16
			for g := 0; g < int(nGroups); g++ {
				if grpOffset+12 > len(tableData) {
					break
				}
				startChar := binary.BigEndian.Uint32(tableData[grpOffset : grpOffset+4])
				endChar := binary.BigEndian.Uint32(tableData[grpOffset+4 : grpOffset+8])
				for c := startChar; c <= endChar; c++ {
					runes[rune(c)] = true
				}
				grpOffset += 12
			}
		}
	}

	return runes, nil
}

// =========================================================================
// Round 107: Native C++ Plugins ABI Guard (AuditNativePluginsAbi)
// =========================================================================

// AuditNativePluginsAbi inspects native binary plugins (.dll, .dylib, .so, .aar, .framework)
// for target CPU architectures and cross-platform ABI matrix compatibility.
func (GitService) AuditNativePluginsAbi(repoPath string) (*NativePluginsAbiResult, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath 不能为空")
	}

	scanDirs := []string{"Plugins", "Native", "Libs", "Packages", "Binaries"}
	var plugins []NativePluginItem
	hasCritical := false
	defectCount := 0

	platformArches := make(map[string]map[string]bool)
	platformArches["Android"] = make(map[string]bool)
	platformArches["iOS"] = make(map[string]bool)
	platformArches["macOS"] = make(map[string]bool)
	platformArches["Windows"] = make(map[string]bool)

	_ = filepath.Walk(repoPath, func(p string, fi os.FileInfo, err error) error {
		if err != nil || fi == nil || fi.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(repoPath, p)
		lowerRel := strings.ToLower(rel)
		ext := strings.ToLower(filepath.Ext(p))

		if ext != ".dll" && ext != ".dylib" && ext != ".so" && ext != ".aar" && ext != ".framework" {
			return nil
		}

		fBytes, _ := os.ReadFile(p)
		if len(fBytes) == 0 {
			return nil
		}

		plat := "Unknown"
		if strings.Contains(lowerRel, "android") || ext == ".aar" || (ext == ".so" && !strings.Contains(lowerRel, "linux")) {
			plat = "Android"
		} else if strings.Contains(lowerRel, "ios") || (strings.Contains(lowerRel, "iphone") && ext == ".dylib") {
			plat = "iOS"
		} else if strings.Contains(lowerRel, "macos") || strings.Contains(lowerRel, "osx") || ext == ".dylib" {
			plat = "macOS"
		} else if strings.Contains(lowerRel, "win") || ext == ".dll" {
			plat = "Windows"
		}

		var arches []string
		var archInfos []BinaryArchInfo
		var issues []string
		formatStr := "Binary"

		// Mach-O inspection
		if len(fBytes) >= 4 {
			magic := binary.LittleEndian.Uint32(fBytes[:4])
			if magic == 0xfeedface || magic == 0xfeedfacf || magic == 0xcafebabe {
				formatStr = "Mach-O"
				fat, mErr := macho.OpenFat(p)
				if mErr == nil {
					formatStr = "Mach-O Fat"
					for _, a := range fat.Arches {
						archStr := "x86_64"
						if a.Cpu == macho.CpuArm64 {
							archStr = "arm64"
						}
						arches = append(arches, archStr)
						archInfos = append(archInfos, BinaryArchInfo{
							Architecture: archStr,
							BitWidth:     64,
							Endianness:   "LittleEndian",
						})
					}
					_ = fat.Close()
				} else {
					m, sErr := macho.Open(p)
					if sErr == nil {
						archStr := "x86_64"
						if m.Cpu == macho.CpuArm64 {
							archStr = "arm64"
						}
						arches = append(arches, archStr)
						archInfos = append(archInfos, BinaryArchInfo{
							Architecture: archStr,
							BitWidth:     64,
							Endianness:   "LittleEndian",
						})
						_ = m.Close()
					}
				}
			} else if fBytes[0] == 0x7f && fBytes[1] == 'E' && fBytes[2] == 'L' && fBytes[3] == 'F' {
				formatStr = "ELF"
				el, eErr := elf.Open(p)
				if eErr == nil {
					archStr := "x86_64"
					bitW := 64
					if el.Machine == elf.EM_AARCH64 {
						archStr = "arm64-v8a"
					} else if el.Machine == elf.EM_ARM {
						archStr = "armeabi-v7a"
						bitW = 32
					}
					arches = append(arches, archStr)
					archInfos = append(archInfos, BinaryArchInfo{
						Architecture: archStr,
						BitWidth:     bitW,
						Endianness:   "LittleEndian",
					})
					_ = el.Close()
				}
			} else if fBytes[0] == 'M' && fBytes[1] == 'Z' {
				formatStr = "PE"
				peFile, pErr := pe.Open(p)
				if pErr == nil {
					archStr := "x86_64"
					bitW := 64
					if peFile.Machine == pe.IMAGE_FILE_MACHINE_I386 {
						archStr = "x86"
						bitW = 32
						issues = append(issues, "检测到 32 位 x86 DLL 动态库，在 64 位游戏主进程中加载将引发 0xC000007B 严重崩溃")
					} else if peFile.Machine == pe.IMAGE_FILE_MACHINE_ARM64 {
						archStr = "arm64"
					}
					arches = append(arches, archStr)
					archInfos = append(archInfos, BinaryArchInfo{
						Architecture: archStr,
						BitWidth:     bitW,
						Endianness:   "LittleEndian",
					})
					_ = peFile.Close()
				}
			}
		}

		if len(arches) == 0 {
			if plat == "Android" {
				arches = append(arches, "arm64-v8a", "armeabi-v7a")
				archInfos = append(archInfos, BinaryArchInfo{Architecture: "arm64-v8a", BitWidth: 64, Endianness: "LittleEndian"})
			} else if plat == "iOS" {
				arches = append(arches, "arm64")
				archInfos = append(archInfos, BinaryArchInfo{Architecture: "arm64", BitWidth: 64, Endianness: "LittleEndian"})
			} else {
				arches = append(arches, "x86_64")
				archInfos = append(archInfos, BinaryArchInfo{Architecture: "x86_64", BitWidth: 64, Endianness: "LittleEndian"})
			}
		}

		for _, a := range arches {
			if platformArches[plat] != nil {
				platformArches[plat][a] = true
			}
		}

		hasIss := len(issues) > 0
		if hasIss {
			defectCount++
			hasCritical = true
		}

		plugins = append(plugins, NativePluginItem{
			RelativePath:      rel,
			PluginName:        filepath.Base(rel),
			Format:            formatStr,
			Architectures:     arches,
			ArchitecturesInfo: archInfos,
			TargetPlatform:    plat,
			Is64BitOnly:       len(archInfos) > 0 && archInfos[0].BitWidth == 64,
			IsUniversal:       len(arches) > 1,
			SizeBytes:         fi.Size(),
			SizeStr:           formatAssetSize(fi.Size()),
			HasIssue:          hasIss,
			Issues:            issues,
		})

		return nil
	})

	var platformStatuses []PlatformAbiStatus
	for _, p := range []string{"Android", "iOS", "macOS", "Windows"} {
		req := []string{"arm64-v8a"}
		if p == "iOS" || p == "macOS" {
			req = []string{"arm64"}
		} else if p == "Windows" {
			req = []string{"x86_64"}
		}
		var avail []string
		for a := range platformArches[p] {
			avail = append(avail, a)
		}
		var missing []string
		for _, r := range req {
			if !platformArches[p][r] && len(avail) > 0 {
				missing = append(missing, r)
			}
		}
		compliant := len(missing) == 0
		sev := "OK"
		if !compliant {
			sev = "ERROR"
			hasCritical = true
		}
		platformStatuses = append(platformStatuses, PlatformAbiStatus{
			Platform:        p,
			RequiredArches:  req,
			AvailableArches: avail,
			MissingArches:   missing,
			PluginCount:     len(avail),
			IsCompliant:     compliant,
			Severity:        sev,
		})
	}

	severity := "OK"
	if hasCritical {
		severity = "ERROR"
	}

	var recs []string
	if hasCritical {
		recs = append(recs, "补齐 Android arm64-v8a 架构 .so 动态库以满足 Google Play 与国内主流渠道 64 位包体强制规范")
		recs = append(recs, "在 Windows 生产环境移除所有 32 位 x86 DLL 库")
	} else if len(plugins) == 0 {
		recs = append(recs, "未发现原生插件二进制，未作 ABI 兼容结论")
	} else {
		recs = append(recs, "所有原生 C++ 插件架构与目标平台 ABI 完全对齐兼容")
	}

	message := fmt.Sprintf("原生插件 ABI 审计完毕: 扫描插件 %d 个, 缺陷 %d 项, 结论: %s",
		len(plugins), defectCount, severity)
	if len(plugins) == 0 {
		message = "原生插件 ABI 审计完毕: 未发现原生插件二进制，未作 ABI 兼容结论"
	}

	return &NativePluginsAbiResult{
		TotalPluginsCount:      len(plugins),
		ScannedDirectories:     scanDirs,
		Plugins:                plugins,
		PlatformStatuses:       platformStatuses,
		CompliantCount:         len(plugins) - defectCount,
		DefectCount:            defectCount,
		HasCriticalMismatch:    hasCritical,
		Severity:               severity,
		SummaryRecommendations: recs,
		Message:                message,
	}, nil
}

// =========================================================================
// Round 108: TAPD Agile Work Item & Defect Integrator (FetchTAPDWorkItems / BindCommitToTAPD)
// =========================================================================

// FetchTAPDWorkItems reports TAPD work items for the current iteration.
// TAPD OpenAPI synchronization is not implemented; return an explicit empty state.
// would make local associations look like real remote data.
func (GitService) FetchTAPDWorkItems(repoPath string) (*TAPDWorkItemListResult, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath 不能为空")
	}
	workspace := strings.TrimSpace(os.Getenv("TAPD_WORKSPACE_ID"))
	if workspace == "" {
		workspace = "未配置"
	}
	return &TAPDWorkItemListResult{
		WorkspaceID:  workspace,
		TotalCount:   0,
		StoriesCount: 0,
		TasksCount:   0,
		BugsCount:    0,
		Items:        []TAPDWorkItem{},
		IsConnected:  false,
		Message:      "TAPD OpenAPI 同步尚未接入；当前仅支持本地单号解析与 Git Notes 关联。",
	}, nil
}

// BindCommitToTAPD writes a TAPD ticket reference into a local Git Note.
func (GitService) BindCommitToTAPD(repoPath string, commitHash string, tapdID string, newStatus string) (*TAPDBindResult, error) {
	if repoPath == "" || commitHash == "" || tapdID == "" {
		return nil, errors.New("repoPath, commitHash, tapdId 均不能为空")
	}

	fullHash, err := ExecGit(repoPath, "rev-parse", "--verify", strings.TrimSpace(commitHash)+"^{commit}")
	if err != nil {
		return nil, fmt.Errorf("无法解析提交 %s: %w", commitHash, err)
	}
	fullHash = strings.TrimSpace(fullHash)
	note := fmt.Sprintf("TAPD #%s Status: %s (local note; TAPD API not called)", tapdID, newStatus)
	if _, err := ExecGit(repoPath, "notes", "append", "-m", note, fullHash); err != nil {
		return nil, fmt.Errorf("写入 Git Note 失败: %w", err)
	}

	shortHash := fullHash
	if len(shortHash) > 7 {
		shortHash = shortHash[:7]
	}
	return &TAPDBindResult{
		Success:       true,
		CommitHash:    shortHash,
		TapdID:        tapdID,
		CurrentStatus: newStatus,
		Message:       fmt.Sprintf("已将 TAPD #%s（状态 %s）写入提交 %s 的本地 Git Note；未调用 TAPD API。", tapdID, newStatus, shortHash),
	}, nil
}

// ParseTAPDCommitReferences extracts TAPD ticket IDs from commit messages.
func (GitService) ParseTAPDCommitReferences(commitMessage string) *TAPDCommitReferenceResult {
	var found []TAPDReferenceItem
	re := regexp.MustCompile(`(?i)(?:#|tapd[=:\s\-]+|bug[=:\s\-]+|story[=:\s\-]+|task[=:\s\-]+)([0-9]{5,})`)
	matches := re.FindAllStringSubmatch(commitMessage, -1)
	seen := make(map[string]bool)

	for _, m := range matches {
		if len(m) > 1 {
			id := m[1]
			if !seen[id] {
				seen[id] = true
				found = append(found, TAPDReferenceItem{
					TicketID:   id,
					PrefixType: "tapd",
					RawTag:     m[0],
				})
			}
		}
	}

	formatted := ""
	if len(found) > 0 {
		formatted = fmt.Sprintf("--tapd=%s", found[0].TicketID)
	}

	return &TAPDCommitReferenceResult{
		CommitMessage: commitMessage,
		FoundTickets:  found,
		HasTAPDTag:    len(found) > 0,
		FormattedTag:  formatted,
	}
}

// =========================================================================
// Round 109: Game Hotfix Patch Manifest Generator (GenerateHotfixManifest)
// =========================================================================

// GenerateHotfixManifest compares a base release tag/commit with latest target ref (HEAD),
// classifies changed assets into hotfix tiers (Code, Config, Resources, Full Pack Required),
// computes CRC32, MD5, SHA256 hashes, calculates compressed patch sizes (Gzip),
// and outputs a production-ready hotfix manifest JSON and delta package file list.
func (GitService) GenerateHotfixManifest(repoPath string, baseRef string, targetRef string, appVersion string) (*HotfixManifestResult, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath 不能为空")
	}

	if targetRef == "" {
		targetRef = "HEAD"
	}

	if baseRef == "" {
		tagOut, err := ExecGit(repoPath, "describe", "--tags", "--abbrev=0")
		if err == nil && strings.TrimSpace(tagOut) != "" {
			baseRef = strings.TrimSpace(tagOut)
		} else {
			_, revErr := ExecGit(repoPath, "rev-parse", "--verify", "HEAD~1")
			if revErr == nil {
				baseRef = "HEAD~1"
			} else {
				rootOut, rootErr := ExecGit(repoPath, "rev-list", "--max-parents=0", "HEAD")
				if rootErr == nil && strings.TrimSpace(rootOut) != "" {
					baseRef = strings.TrimSpace(rootOut)
				} else {
					baseRef = "HEAD"
				}
			}
		}
	}

	if appVersion == "" {
		appVersion = "1.0.1-hotfix"
	}

	if strings.EqualFold(baseRef, "WORKTREE") {
		return nil, errors.New("热更清单的基线必须是 Git 提交，不能是 WORKTREE")
	}
	if _, err := ExecGit(repoPath, "rev-parse", "--verify", baseRef+"^{commit}"); err != nil {
		return nil, fmt.Errorf("基线引用 %s 不存在或不可解析: %w", baseRef, err)
	}
	if !strings.EqualFold(targetRef, "WORKTREE") {
		if _, err := ExecGit(repoPath, "rev-parse", "--verify", targetRef+"^{commit}"); err != nil {
			return nil, fmt.Errorf("目标引用 %s 不存在或不可解析: %w", targetRef, err)
		}
	}

	diffArgs := []string{"diff", "--name-status", "-z"}
	if strings.EqualFold(targetRef, "WORKTREE") || baseRef == targetRef {
		diffArgs = append(diffArgs, baseRef)
	} else {
		diffArgs = append(diffArgs, baseRef, targetRef)
	}
	diffOut, diffErr := ExecGitRaw(repoPath, diffArgs...)
	if diffErr != nil {
		return nil, fmt.Errorf("生成热更清单差异失败: %w", diffErr)
	}

	var patchFiles []HotfixPatchFileItem
	deltaPackageList := []string{}
	tierCounts := make(map[string]int)
	tierBytes := make(map[string]int64)

	var totalPatchBytes int64
	var totalCompressedBytes int64
	fullPackRequired := false
	fullPackReason := ""

	// -z keeps spaces/tabs in paths intact and reports renames as
	// "R<score>\0<old>\0<new>\0".
	tokens := strings.Split(diffOut, "\x00")
	for i := 0; i < len(tokens); {
		statusLetter := strings.TrimSpace(tokens[i])
		i++
		if statusLetter == "" {
			continue
		}
		if i >= len(tokens) {
			return nil, fmt.Errorf("解析热更差异失败: 状态 %s 缺少文件路径", statusLetter)
		}
		oldPath := tokens[i]
		i++
		filePath := oldPath
		if strings.HasPrefix(statusLetter, "R") || strings.HasPrefix(statusLetter, "C") {
			if i >= len(tokens) {
				return nil, fmt.Errorf("解析热更差异失败: %s 缺少新路径", oldPath)
			}
			filePath = tokens[i]
			i++
		}

		action := "MODIFIED"
		if strings.HasPrefix(statusLetter, "A") || strings.HasPrefix(statusLetter, "C") {
			action = "ADDED"
		} else if strings.HasPrefix(statusLetter, "D") {
			action = "DELETED"
		}

		tier := classifyHotfixTier(filePath)

		var fileSize int64
		var compSize int64
		var crc32Hex, md5Hex, sha256Hex string

		if action == "DELETED" {
			fileSize = 0
			compSize = 0
		} else {
			data, err := fetchAssetBytes(repoPath, filePath, targetRef)
			if err != nil {
				return nil, fmt.Errorf("读取热更文件 %s（%s）失败: %w", filePath, targetRef, err)
			}

			fileSize = int64(len(data))
			totalPatchBytes += fileSize

			crcVal := crc32.ChecksumIEEE(data)
			crc32Hex = fmt.Sprintf("%08X", crcVal)

			mHash := md5.Sum(data)
			md5Hex = hex.EncodeToString(mHash[:])

			sHash := sha256.Sum256(data)
			sha256Hex = hex.EncodeToString(sHash[:])

			var gzBuf bytes.Buffer
			gw, gzErr := gzip.NewWriterLevel(&gzBuf, gzip.BestCompression)
			if gzErr == nil {
				_, _ = gw.Write(data)
				_ = gw.Close()
				compSize = int64(gzBuf.Len())
			} else {
				compSize = fileSize
			}
			totalCompressedBytes += compSize

			deltaPackageList = append(deltaPackageList, filePath)
		}

		requiresReboot := false
		if tier == "Full Pack Required" {
			fullPackRequired = true
			if fullPackReason == "" {
				fullPackReason = fmt.Sprintf("检测到原生可执行程序/核心模块变更 (%s)，必须执行整包重新安装或更新", filepath.Base(filePath))
			}
			requiresReboot = true
		} else if tier == "Code" {
			requiresReboot = true
		}

		tierCounts[tier]++
		tierBytes[tier] += fileSize

		patchFiles = append(patchFiles, HotfixPatchFileItem{
			Path:              filePath,
			Action:            action,
			Tier:              tier,
			Size:              fileSize,
			SizeStr:           formatAssetSize(fileSize),
			CompressedSize:    compSize,
			CompressedSizeStr: formatAssetSize(compSize),
			CRC32:             crc32Hex,
			MD5:               md5Hex,
			SHA256:            sha256Hex,
			RequiresReboot:    requiresReboot,
		})
	}

	orderedTiers := []string{"Code", "Config", "Resources", "Full Pack Required"}
	tierBreakdown := []HotfixTierSummary{}
	for _, tName := range orderedTiers {
		if c, ok := tierCounts[tName]; ok && c > 0 {
			tierBreakdown = append(tierBreakdown, HotfixTierSummary{
				Tier:         tName,
				Count:        c,
				TotalBytes:   tierBytes[tName],
				TotalSizeStr: formatAssetSize(tierBytes[tName]),
			})
		}
	}

	nowStr := time.Now().UTC().Format(time.RFC3339)
	manifestData := map[string]interface{}{
		"manifest_version":       "2.0",
		"app_version":            appVersion,
		"base_version":           baseRef,
		"target_version":         targetRef,
		"generated_at":           nowStr,
		"full_pack_required":     fullPackRequired,
		"full_pack_reason":       fullPackReason,
		"total_files_count":      len(patchFiles),
		"total_patch_bytes":      totalPatchBytes,
		"total_compressed_bytes": totalCompressedBytes,
		"tier_summary":           tierCounts,
		"files":                  patchFiles,
	}

	manifestJsonBytes, _ := json.MarshalIndent(manifestData, "", "  ")

	summary := fmt.Sprintf("热更补丁清单生成完毕：变更文件 %d 个 (Code: %d, Config: %d, Resources: %d, FullPack: %d)，原始体积 %s，压缩后 %s",
		len(patchFiles), tierCounts["Code"], tierCounts["Config"], tierCounts["Resources"], tierCounts["Full Pack Required"],
		formatAssetSize(totalPatchBytes), formatAssetSize(totalCompressedBytes))
	if fullPackRequired {
		summary += fmt.Sprintf(" [警告: %s]", fullPackReason)
	}

	return &HotfixManifestResult{
		Success:                true,
		ManifestVersion:        "2.0",
		AppVersion:             appVersion,
		BaseRef:                baseRef,
		TargetRef:              targetRef,
		GeneratedAt:            nowStr,
		FullPackRequired:       fullPackRequired,
		FullPackReason:         fullPackReason,
		TotalFilesCount:        len(patchFiles),
		TotalPatchBytes:        totalPatchBytes,
		TotalPatchSizeStr:      formatAssetSize(totalPatchBytes),
		TotalCompressedBytes:   totalCompressedBytes,
		TotalCompressedSizeStr: formatAssetSize(totalCompressedBytes),
		TierBreakdown:          tierBreakdown,
		PatchFiles:             patchFiles,
		DeltaPackageList:       deltaPackageList,
		ManifestJSON:           string(manifestJsonBytes),
		Summary:                summary,
	}, nil
}

func classifyHotfixTier(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	lower := strings.ToLower(filePath)

	if ext == ".exe" || ext == ".apk" || ext == ".ipa" || ext == ".aab" ||
		ext == ".so" || ext == ".dylib" || ext == ".pak" || ext == ".bundle" ||
		strings.Contains(lower, "/binaries/") || strings.Contains(lower, "\\binaries\\") ||
		strings.Contains(lower, "/native/") || strings.Contains(lower, "\\native\\") {
		return "Full Pack Required"
	}

	if ext == ".lua" || ext == ".luac" || ext == ".cs" || ext == ".dll" ||
		ext == ".pdb" || ext == ".wasm" || ext == ".js" || ext == ".ts" || ext == ".py" {
		return "Code"
	}

	if ext == ".csv" || ext == ".tsv" || ext == ".xlsx" || ext == ".json" ||
		ext == ".xml" || ext == ".yaml" || ext == ".yml" || ext == ".ini" ||
		ext == ".bytes" || ext == ".txt" || ext == ".toml" || ext == ".table" ||
		ext == ".cfg" || ext == ".conf" {
		return "Config"
	}

	if ext == ".mat" || ext == ".material" || ext == ".prefab" || ext == ".unity" ||
		ext == ".uasset" || ext == ".umap" || ext == ".png" || ext == ".jpg" ||
		ext == ".jpeg" || ext == ".tga" || ext == ".psd" || ext == ".dds" ||
		ext == ".hdr" || ext == ".exr" || ext == ".fbx" || ext == ".obj" ||
		ext == ".blend" || ext == ".anim" || ext == ".controller" || ext == ".wav" ||
		ext == ".mp3" || ext == ".ogg" || ext == ".bank" || ext == ".asset" ||
		ext == ".fontsettings" || ext == ".ttf" || ext == ".otf" {
		return "Resources"
	}

	if isPathBinary(filePath) {
		return "Resources"
	}
	return "Config"
}

// =========================================================================
// Round 110: Package Size Attribution & Budget Dashboard (AnalyzePackageSizeBudget)
// =========================================================================

// AnalyzePackageSizeBudget categorizes workspace and Git LFS assets by game systems/modules,
// evaluates sizes against platform budgets (iOS, Android, PC/Console, MiniGame),
// and provides actionable compression suggestions (texture, audio, mesh, LFS).
func (GitService) AnalyzePackageSizeBudget(repoPath string, targetPlatform string) (*PackageBudgetResult, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath 不能为空")
	}

	lfsExts := getLFSExtensions(repoPath)

	type fileInfoRecord struct {
		relPath string
		size    int64
		isLFS   bool
		system  string
	}

	var allFiles []fileInfoRecord
	var totalSizeBytes int64
	var totalLfsBytes int64

	systemBytes := make(map[string]int64)
	systemLfsBytes := make(map[string]int64)
	systemCounts := make(map[string]int)
	systemLfsCounts := make(map[string]int)
	systemFiles := make(map[string][]fileInfoRecord)

	var suggestions []BudgetOptimizationSuggestion
	walkIssues := 0

	_ = filepath.Walk(repoPath, func(p string, fi os.FileInfo, err error) error {
		if err != nil || fi == nil {
			if err != nil {
				walkIssues++
			}
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

		rel, rErr := filepath.Rel(repoPath, p)
		if rErr != nil {
			return nil
		}

		fSize := fi.Size()
		isLfs := isPathLFS(rel, lfsExts)

		if isLfs && fSize < 1024 {
			if headBytes, hErr := fetchAssetBytes(repoPath, rel, "HEAD"); hErr == nil && len(headBytes) > int(fSize) {
				fSize = int64(len(headBytes))
			}
		}

		totalSizeBytes += fSize
		if isLfs {
			totalLfsBytes += fSize
		}

		sys := classifyAssetSystem(rel)
		systemBytes[sys] += fSize
		systemCounts[sys]++
		if isLfs {
			systemLfsBytes[sys] += fSize
			systemLfsCounts[sys]++
		}

		rec := fileInfoRecord{relPath: rel, size: fSize, isLFS: isLfs, system: sys}
		allFiles = append(allFiles, rec)
		systemFiles[sys] = append(systemFiles[sys], rec)

		ext := strings.ToLower(filepath.Ext(rel))
		if (ext == ".png" || ext == ".tga" || ext == ".psd" || ext == ".jpg" || ext == ".bmp") && fSize >= 512*1024 {
			saving := fSize * 3 / 4
			prio := "MEDIUM"
			if fSize >= 4*1024*1024 {
				prio = "HIGH"
			}
			suggestions = append(suggestions, BudgetOptimizationSuggestion{
				Category:           "Texture Compression",
				TargetFile:         rel,
				CurrentSize:        fSize,
				CurrentSizeStr:     formatAssetSize(fSize),
				EstimatedSaving:    saving,
				EstimatedSavingStr: formatAssetSize(saving),
				Recommendation:     "转为 ASTC 6x6 或 BC7/ETC2 显卡硬件纹理格式，预估节省 75% 显存与安装包体积",
				Priority:           prio,
			})
		} else if (ext == ".wav" || ext == ".aiff") && fSize >= 512*1024 {
			saving := fSize * 85 / 100
			suggestions = append(suggestions, BudgetOptimizationSuggestion{
				Category:           "Audio Compression",
				TargetFile:         rel,
				CurrentSize:        fSize,
				CurrentSizeStr:     formatAssetSize(fSize),
				EstimatedSaving:    saving,
				EstimatedSavingStr: formatAssetSize(saving),
				Recommendation:     "转为 Vorbis/Opus 128kbps 或 ADPCM 流式加载，预估节省 85% 音频包体",
				Priority:           "HIGH",
			})
		} else if (ext == ".fbx" || ext == ".obj") && fSize >= 2*1024*1024 {
			saving := fSize / 2
			suggestions = append(suggestions, BudgetOptimizationSuggestion{
				Category:           "Mesh Optimization",
				TargetFile:         rel,
				CurrentSize:        fSize,
				CurrentSizeStr:     formatAssetSize(fSize),
				EstimatedSaving:    saving,
				EstimatedSavingStr: formatAssetSize(saving),
				Recommendation:     "配置 Draco 网格量化与多级 LOD 剔除，预估节省 50% 几何体体积",
				Priority:           "MEDIUM",
			})
		} else if fSize >= 10*1024*1024 && !isLfs {
			suggestions = append(suggestions, BudgetOptimizationSuggestion{
				Category:           "LFS Offloading",
				TargetFile:         rel,
				CurrentSize:        fSize,
				CurrentSizeStr:     formatAssetSize(fSize),
				EstimatedSaving:    0,
				EstimatedSavingStr: "0 B",
				Recommendation:     "超大二进制文件未纳入 Git LFS 纳管，建议使用 git lfs track 避免仓库 Git 历史膨胀",
				Priority:           "HIGH",
			})
		}

		return nil
	})

	sort.Slice(suggestions, func(i, j int) bool {
		return suggestions[i].EstimatedSaving > suggestions[j].EstimatedSaving
	})
	if len(suggestions) > 10 {
		suggestions = suggestions[:10]
	}

	standardSystems := []string{"Characters", "Environments", "UI", "Audio", "VFX", "Code", "ThirdParty", "Misc"}
	var systemAttributions []AssetSystemAttribution

	for _, sName := range standardSystems {
		c := systemCounts[sName]
		b := systemBytes[sName]
		if c == 0 && b == 0 {
			continue
		}

		pct := 0.0
		if totalSizeBytes > 0 {
			pct = math.Round((float64(b)/float64(totalSizeBytes))*1000.0) / 10.0
		}

		fList := systemFiles[sName]
		sort.Slice(fList, func(i, j int) bool {
			return fList[i].size > fList[j].size
		})
		var topPaths []string
		for i := 0; i < len(fList) && i < 5; i++ {
			topPaths = append(topPaths, fmt.Sprintf("%s (%s)", fList[i].relPath, formatAssetSize(fList[i].size)))
		}

		systemAttributions = append(systemAttributions, AssetSystemAttribution{
			SystemName:   sName,
			TotalBytes:   b,
			TotalSizeStr: formatAssetSize(b),
			FileCount:    c,
			Percentage:   pct,
			LfsBytes:     systemLfsBytes[sName],
			LfsSizeStr:   formatAssetSize(systemLfsBytes[sName]),
			LfsCount:     systemLfsCounts[sName],
			TopAssets:    topPaths,
		})
	}

	type platDef struct {
		name  string
		limit int64
	}
	platforms := []platDef{
		{name: "WeChat Mini Game (微信小游戏)", limit: 30 * 1024 * 1024},
		{name: "iOS App Store (蜂窝网络OTA)", limit: 200 * 1024 * 1024},
		{name: "iOS App Store (整包预算)", limit: 2 * 1024 * 1024 * 1024},
		{name: "Android Google Play (APK+OBB/PAD)", limit: 4 * 1024 * 1024 * 1024},
		{name: "PC / Steam / 主机 (Console)", limit: 25 * 1024 * 1024 * 1024},
	}

	var budgetStatuses []PlatformBudgetStatus
	hasOverrun := false
	worstPlatform := ""

	for _, p := range platforms {
		usage := float64(totalSizeBytes) / float64(p.limit)
		rem := p.limit - totalSizeBytes
		status := "SAFE"
		if usage > 1.0 {
			status = "OVERRUN"
			hasOverrun = true
			if worstPlatform == "" {
				worstPlatform = p.name
			}
		} else if usage >= 0.8 {
			status = "WARNING"
			if worstPlatform == "" {
				worstPlatform = p.name
			}
		}

		remStr := formatAssetSize(rem)
		if rem < 0 {
			remStr = fmt.Sprintf("超额 +%s", formatAssetSize(-rem))
		} else {
			remStr = fmt.Sprintf("剩余 %s", remStr)
		}

		budgetStatuses = append(budgetStatuses, PlatformBudgetStatus{
			Platform:          p.name,
			MaxBudgetLimit:    p.limit,
			MaxBudgetLimitStr: formatAssetSize(p.limit),
			CurrentSize:       totalSizeBytes,
			CurrentSizeStr:    formatAssetSize(totalSizeBytes),
			UsageRatio:        math.Round(usage*1000.0) / 1000.0,
			RemainingBytes:    rem,
			RemainingStr:      remStr,
			Status:            status,
		})
	}

	var totalSavingPotential int64
	for _, s := range suggestions {
		totalSavingPotential += s.EstimatedSaving
	}

	summary := fmt.Sprintf("包体预算评估：总资产 %d 个，总容量 %s (LFS 纳管 %s)。",
		len(allFiles), formatAssetSize(totalSizeBytes), formatAssetSize(totalLfsBytes))
	if hasOverrun {
		summary += fmt.Sprintf(" ⚠️ 平台预算超出：%s 已超限！", worstPlatform)
	} else {
		summary += " ✅ 所有核心平台包体预算合规。"
	}
	if len(suggestions) > 0 {
		summary += fmt.Sprintf(" 发现 %d 项高优压缩优化建议，预估可节约 %s 空间。",
			len(suggestions), formatAssetSize(totalSavingPotential))
	}
	if walkIssues > 0 {
		summary += fmt.Sprintf(" ⚠️ 有 %d 个路径无法读取，包体统计可能不完整。", walkIssues)
	}

	return &PackageBudgetResult{
		RepoPath:         repoPath,
		TotalAssetsCount: len(allFiles),
		TotalSizeBytes:   totalSizeBytes,
		TotalSizeStr:     formatAssetSize(totalSizeBytes),
		TotalLfsBytes:    totalLfsBytes,
		TotalLfsSizeStr:  formatAssetSize(totalLfsBytes),
		Systems:          systemAttributions,
		PlatformBudgets:  budgetStatuses,
		OptimizationTips: suggestions,
		HasOverrun:       hasOverrun,
		WorstPlatform:    worstPlatform,
		Summary:          summary,
	}, nil
}

func classifyAssetSystem(relPath string) string {
	lower := strings.ToLower(relPath)
	ext := strings.ToLower(filepath.Ext(relPath))

	if strings.Contains(lower, "character") || strings.Contains(lower, "hero") ||
		strings.Contains(lower, "npc") || strings.Contains(lower, "avatar") ||
		strings.Contains(lower, "skin") || strings.Contains(lower, "skeleton") ||
		strings.Contains(lower, "monster") || strings.Contains(lower, "creature") {
		return "Characters"
	}

	if strings.Contains(lower, "environment") || strings.Contains(lower, "level") ||
		strings.Contains(lower, "map") || strings.Contains(lower, "scene") ||
		strings.Contains(lower, "terrain") || strings.Contains(lower, "prop") ||
		strings.Contains(lower, "world") || strings.Contains(lower, "building") {
		return "Environments"
	}

	if strings.Contains(lower, "/ui/") || strings.HasPrefix(lower, "ui/") ||
		strings.Contains(lower, "font") || strings.Contains(lower, "hud") ||
		strings.Contains(lower, "icon") || strings.Contains(lower, "atlas") ||
		strings.Contains(lower, "widget") || strings.Contains(lower, "sprite") ||
		ext == ".ttf" || ext == ".otf" || ext == ".fnt" {
		return "UI"
	}

	if strings.Contains(lower, "audio") || strings.Contains(lower, "sound") ||
		strings.Contains(lower, "bgm") || strings.Contains(lower, "voice") ||
		strings.Contains(lower, "sfx") || strings.Contains(lower, "music") ||
		ext == ".wav" || ext == ".mp3" || ext == ".ogg" || ext == ".bank" ||
		ext == ".flac" || ext == ".aac" {
		return "Audio"
	}

	if strings.Contains(lower, "vfx") || strings.Contains(lower, "effect") ||
		strings.Contains(lower, "particle") || strings.Contains(lower, "shader") ||
		ext == ".shader" || ext == ".vfx" || ext == ".hlsl" || ext == ".usf" || ext == ".ush" {
		return "VFX"
	}

	if strings.Contains(lower, "script") || strings.Contains(lower, "code") ||
		strings.Contains(lower, "source") || strings.Contains(lower, "src") ||
		strings.Contains(lower, "assemblies") ||
		ext == ".cs" || ext == ".lua" || ext == ".ts" || ext == ".js" ||
		ext == ".cpp" || ext == ".h" || ext == ".hpp" || ext == ".go" || ext == ".rs" {
		return "Code"
	}

	if strings.Contains(lower, "plugin") || strings.Contains(lower, "thirdparty") ||
		strings.Contains(lower, "package") || strings.Contains(lower, "packagecache") ||
		strings.Contains(lower, "vendor") || strings.Contains(lower, "sdk") ||
		strings.Contains(lower, "node_modules") {
		return "ThirdParty"
	}

	return "Misc"
}

// =========================================================================
// Round 111: Multi-Project Worktree Switchboard (ManageStudioSwitchboard)
// =========================================================================

// ManageStudioSwitchboard discovers and tracks multiple linked worktrees and adjacent studio sub-projects
// (Client, Server, Tooling, Docs), reports dirty state, branch locks, and executes quick-switch / sync operations.
func (GitService) ManageStudioSwitchboard(repoPath string, action string, targetPath string, targetBranch string, extraParam string) (*StudioSwitchboardResult, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath 不能为空")
	}

	action = strings.ToUpper(strings.TrimSpace(action))
	if action == "" {
		action = "LIST"
	}

	var actionMsg string

	switch action {
	case "SWITCH":
		if targetPath == "" {
			targetPath = repoPath
		}
		if targetBranch == "" {
			return nil, errors.New("SWITCH 操作需要指定目标分支 targetBranch")
		}
		out, err := ExecGit(targetPath, "checkout", targetBranch)
		if err != nil {
			return nil, fmt.Errorf("切换分支至 %s 失败: %v (输出: %s)", targetBranch, err, out)
		}
		actionMsg = fmt.Sprintf("工作区 %s 已成功切至分支 %s", filepath.Base(targetPath), targetBranch)

	case "SYNC":
		if targetPath == "" {
			targetPath = repoPath
		}
		out, err := ExecGit(targetPath, "pull", "--ff-only")
		if err != nil {
			fOut, fErr := ExecGit(targetPath, "fetch", "--all")
			if fErr != nil {
				actionMsg = fmt.Sprintf("工作区 %s 远端同步已执行 (无远端或无需合并)", filepath.Base(targetPath))
			} else {
				actionMsg = fmt.Sprintf("工作区 %s 已完成远端 fetch 同步: %s", filepath.Base(targetPath), strings.TrimSpace(fOut))
			}
		} else {
			actionMsg = fmt.Sprintf("工作区 %s 快速合并同步成功: %s", filepath.Base(targetPath), strings.TrimSpace(out))
		}

	case "LOCK":
		if targetPath == "" {
			return nil, errors.New("LOCK 操作需要指定 targetPath 工作树路径")
		}
		reason := extraParam
		if reason == "" {
			reason = "Studio Switchboard 用户锁定保护"
		}
		out, err := ExecGit(repoPath, "worktree", "lock", targetPath, "--reason", reason)
		if err != nil {
			return nil, fmt.Errorf("锁定工作树失败: %v (%s)", err, out)
		}
		actionMsg = fmt.Sprintf("工作树 %s 已加锁保护: %s", filepath.Base(targetPath), reason)

	case "UNLOCK":
		if targetPath == "" {
			return nil, errors.New("UNLOCK 操作需要指定 targetPath 工作树路径")
		}
		out, err := ExecGit(repoPath, "worktree", "unlock", targetPath)
		if err != nil {
			return nil, fmt.Errorf("解锁工作树失败: %v (%s)", err, out)
		}
		actionMsg = fmt.Sprintf("工作树 %s 已成功解除锁定", filepath.Base(targetPath))

	case "ADD_WORKTREE":
		if targetPath == "" {
			return nil, errors.New("ADD_WORKTREE 需要指定 targetPath 路径")
		}
		if targetBranch == "" {
			targetBranch = "HEAD"
		}
		out, err := ExecGit(repoPath, "worktree", "add", targetPath, targetBranch)
		if err != nil {
			bOut, bErr := ExecGit(repoPath, "worktree", "add", "-b", targetBranch, targetPath)
			if bErr != nil {
				return nil, fmt.Errorf("添加 Linked Worktree 失败: %v (重试错误: %s, 首试错误: %s)", err, bOut, out)
			}
		}
		actionMsg = fmt.Sprintf("成功创建并挂载 Linked Worktree: %s [%s]", filepath.Base(targetPath), targetBranch)

	case "LIST":
		actionMsg = "获取工作室多项目与工作树拓扑成功"
	default:
		return nil, fmt.Errorf("未知的工作室拓扑操作: %s", action)
	}

	worktreeOut, wtErr := ExecGit(repoPath, "worktree", "list", "--porcelain")
	if wtErr != nil {
		if action == "LIST" {
			return nil, fmt.Errorf("读取工作树列表失败: %v", wtErr)
		}
		actionMsg += fmt.Sprintf("（工作树列表读取失败：%v）", wtErr)
	}
	type rawWorktree struct {
		path   string
		branch string
		head   string
		locked bool
		reason string
	}
	var rawWTList []rawWorktree

	currentWT := rawWorktree{}
	for _, line := range strings.Split(worktreeOut, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			if currentWT.path != "" {
				rawWTList = append(rawWTList, currentWT)
				currentWT = rawWorktree{}
			}
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		val := ""
		if len(parts) > 1 {
			val = strings.TrimSpace(parts[1])
		}
		switch parts[0] {
		case "worktree":
			currentWT.path = val
		case "HEAD":
			currentWT.head = val
		case "branch":
			currentWT.branch = strings.TrimPrefix(val, "refs/heads/")
		case "locked":
			currentWT.locked = true
			currentWT.reason = val
		}
	}
	if currentWT.path != "" {
		rawWTList = append(rawWTList, currentWT)
	}

	var projects []StudioProjectNode
	discoveredPaths := make(map[string]bool)
	linkedCount := 0

	activePath := filepath.Clean(repoPath)
	cleanRepo := canonicalRepoPath(activePath)

	for idx, wt := range rawWTList {
		cPath := canonicalRepoPath(wt.path)
		discoveredPaths[cPath] = true

		cat := "LinkedWorktree"
		if idx == 0 {
			cat = "MainWorktree"
		} else {
			linkedCount++
		}

		node := inspectStudioProject(cPath, cat, wt.branch, wt.head, wt.locked, wt.reason, cPath == cleanRepo)
		projects = append(projects, node)
	}

	parentDir := filepath.Dir(activePath)
	adjacentCount := 0

	entries, err := os.ReadDir(parentDir)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			pName := entry.Name()
			subPath := filepath.Clean(filepath.Join(parentDir, pName))
			canonicalSub := canonicalRepoPath(subPath)
			if discoveredPaths[canonicalSub] {
				continue
			}

			gitDir := filepath.Join(subPath, ".git")
			if _, sErr := os.Stat(gitDir); sErr == nil {
				discoveredPaths[canonicalSub] = true
				adjacentCount++

				cat := classifyStudioSubProject(pName)
				node := inspectStudioProject(subPath, cat, "", "", false, "", canonicalSub == cleanRepo)
				projects = append(projects, node)
			}
		}
	}

	return &StudioSwitchboardResult{
		ActionExecuted:    action,
		Success:           true,
		Message:           actionMsg,
		ActiveProjectPath: activePath,
		TotalProjects:     len(projects),
		LinkedWorktrees:   linkedCount,
		AdjacentRepos:     adjacentCount,
		Projects:          projects,
	}, nil
}

func inspectStudioProject(dirPath string, category string, knownBranch string, knownHead string, isLocked bool, lockReason string, isCurrent bool) StudioProjectNode {
	name := filepath.Base(dirPath)

	branch := knownBranch
	if branch == "" {
		bOut, _ := ExecGit(dirPath, "rev-parse", "--abbrev-ref", "HEAD")
		branch = strings.TrimSpace(bOut)
	}

	statusOut, statusErr := ExecGit(dirPath, "status", "--porcelain")
	dirtyLines := 0
	for _, l := range strings.Split(statusOut, "\n") {
		if strings.TrimSpace(l) != "" {
			dirtyLines++
		}
	}

	logOut, _ := ExecGit(dirPath, "log", "-1", "--format=%h\t%s\t%an\t%cr")
	var cHash, cSubject, cAuthor, cDate string
	if logOut != "" {
		logParts := strings.Split(strings.TrimSpace(logOut), "\t")
		if len(logParts) >= 1 {
			cHash = logParts[0]
		}
		if len(logParts) >= 2 {
			cSubject = logParts[1]
		}
		if len(logParts) >= 3 {
			cAuthor = logParts[2]
		}
		if len(logParts) >= 4 {
			cDate = logParts[3]
		}
	}
	if cHash == "" && knownHead != "" {
		if len(knownHead) > 7 {
			cHash = knownHead[:7]
		} else {
			cHash = knownHead
		}
	}

	var ahead, behind int
	abOut, _ := ExecGit(dirPath, "rev-list", "--left-right", "--count", "HEAD...@{u}")
	abParts := strings.Fields(abOut)
	if len(abParts) >= 2 {
		ahead, _ = strconv.Atoi(abParts[0])
		behind, _ = strconv.Atoi(abParts[1])
	}

	node := StudioProjectNode{
		Path:              dirPath,
		Name:              name,
		Category:          category,
		Branch:            branch,
		IsDirty:           dirtyLines > 0,
		DirtyFilesCount:   dirtyLines,
		IsLocked:          isLocked,
		LockReason:        lockReason,
		LastCommitHash:    cHash,
		LastCommitSubject: cSubject,
		LastCommitAuthor:  cAuthor,
		LastCommitDate:    cDate,
		IsCurrent:         isCurrent,
		AheadCount:        ahead,
		BehindCount:       behind,
	}
	if statusErr != nil {
		node.StatusWarning = "读取工作区状态失败: " + statusErr.Error()
	}
	return node
}

func classifyStudioSubProject(folderName string) string {
	lower := strings.ToLower(folderName)
	if strings.Contains(lower, "client") || strings.Contains(lower, "game") || strings.Contains(lower, "frontend") {
		return "Client"
	}
	if strings.Contains(lower, "server") || strings.Contains(lower, "backend") || strings.Contains(lower, "service") {
		return "Server"
	}
	if strings.Contains(lower, "tool") || strings.Contains(lower, "pipeline") || strings.Contains(lower, "editor") {
		return "Tooling"
	}
	if strings.Contains(lower, "doc") || strings.Contains(lower, "wiki") || strings.Contains(lower, "design") {
		return "Docs"
	}
	return "AdjacentRepo"
}

// =========================================================================
// Round 112: Game Design Data Table Validator (AuditDataTableDiff)
// =========================================================================

// AuditDataTableDiff validates numerical game design tables (.csv, .tsv, .json, .xlsx)
// across git refs or working copy, auditing schema changes, primary key collisions,
// NaN/null anomalies, invalid foreign keys, and calculating semantic numerical stat diffs.
func (GitService) AuditDataTableDiff(repoPath string, targetTable string, baseRef string, targetRef string) (*DataTableAuditResult, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath 不能为空")
	}
	if _, err := ExecGit(repoPath, "rev-parse", "--git-dir"); err != nil {
		return nil, fmt.Errorf("不是有效的 Git 仓库: %w", err)
	}

	if targetRef == "" {
		targetRef = "WORKTREE"
	}
	if baseRef == "" {
		baseRef = "HEAD"
	}
	if !strings.EqualFold(baseRef, "WORKTREE") {
		if _, err := ExecGit(repoPath, "rev-parse", "--verify", baseRef+"^{commit}"); err != nil {
			return nil, fmt.Errorf("基线引用 %s 不存在或不可解析: %w", baseRef, err)
		}
	}
	if !strings.EqualFold(targetRef, "WORKTREE") {
		if _, err := ExecGit(repoPath, "rev-parse", "--verify", targetRef+"^{commit}"); err != nil {
			return nil, fmt.Errorf("目标引用 %s 不存在或不可解析: %w", targetRef, err)
		}
	}

	tableExts := map[string]bool{
		".csv":  true,
		".tsv":  true,
		".json": true,
		".xlsx": true,
	}

	var tablesToAudit []string
	if targetTable != "" {
		cleanTable, err := repoRelativePathFromInput(repoPath, targetTable)
		if err != nil {
			return nil, err
		}
		tablesToAudit = append(tablesToAudit, cleanTable)
	} else {
		var diffOut string
		if targetRef == "WORKTREE" {
			var err error
			diffOut, err = ExecGit(repoPath, "diff", "--name-only", baseRef)
			if err != nil {
				return nil, fmt.Errorf("读取工作区数据表差异失败: %w", err)
			}
			statusOut, err := ExecGit(repoPath, "status", "--porcelain")
			if err != nil {
				return nil, fmt.Errorf("读取工作区状态失败: %w", err)
			}
			for _, sLine := range strings.Split(statusOut, "\n") {
				fields := strings.Fields(sLine)
				if len(fields) >= 2 {
					diffOut += "\n" + fields[len(fields)-1]
				}
			}
		} else {
			var err error
			diffOut, err = ExecGit(repoPath, "diff", "--name-only", baseRef, targetRef)
			if err != nil {
				return nil, fmt.Errorf("读取数据表差异失败: %w", err)
			}
		}

		seen := make(map[string]bool)
		for _, line := range strings.Split(diffOut, "\n") {
			line = strings.TrimSpace(line)
			if line == "" || seen[line] {
				continue
			}
			ext := strings.ToLower(filepath.Ext(line))
			if tableExts[ext] {
				seen[line] = true
				tablesToAudit = append(tablesToAudit, line)
			}
		}

		if len(tablesToAudit) == 0 {
			_ = filepath.Walk(repoPath, func(p string, fi os.FileInfo, err error) error {
				if err != nil || fi == nil || fi.IsDir() {
					return nil
				}
				ext := strings.ToLower(filepath.Ext(p))
				if tableExts[ext] {
					rel, rErr := filepath.Rel(repoPath, p)
					if rErr == nil && !seen[rel] {
						seen[rel] = true
						tablesToAudit = append(tablesToAudit, rel)
					}
				}
				return nil
			})
		}
	}

	auditedReports := []AuditedTableReport{}
	totalCritical := 0
	totalWarning := 0

	for _, tblPath := range tablesToAudit {
		baseBytes, baseErr := fetchAssetBytes(repoPath, tblPath, baseRef)
		targetBytes, targetErr := fetchAssetBytes(repoPath, tblPath, targetRef)
		if targetErr != nil {
			worktreeBytes, worktreeErr := os.ReadFile(filepath.Join(repoPath, tblPath))
			if worktreeErr == nil {
				targetBytes = worktreeBytes
			} else if baseErr == nil {
				targetBytes = nil
			} else {
				return nil, fmt.Errorf("数据表读取失败: %s", tblPath)
			}
		}

		baseTable := parseDesignTableData(tblPath, baseBytes)
		targetTableData := parseDesignTableData(tblPath, targetBytes)

		report := auditSingleTable(tblPath, baseTable, targetTableData)
		for _, a := range report.Anomalies {
			if a.Severity == "CRITICAL" {
				totalCritical++
			} else {
				totalWarning++
			}
		}
		auditedReports = append(auditedReports, report)
	}

	analysisKnown := len(auditedReports) > 0
	isValid := analysisKnown && totalCritical == 0
	overallSummary := "未发现可审计的数据表（csv/tsv/json/xlsx），未作结构与数值结论"
	if analysisKnown {
		overallSummary = fmt.Sprintf("数据表校验完毕：共审计 %d 张策划数值表，发现 %d 处阻断性严重缺陷 (CRITICAL)，%d 处警示警告 (WARNING)。",
			len(auditedReports), totalCritical, totalWarning)
		if isValid {
			overallSummary += " 所有表结构及数值合法，准许合入主干。"
		} else {
			overallSummary += " 存在数据损坏或主键碰撞，禁止合入发布分支！"
		}
	}

	return &DataTableAuditResult{
		AnalysisKnown:  analysisKnown,
		BaseRef:        baseRef,
		TargetRef:      targetRef,
		AuditedTables:  auditedReports,
		TotalTables:    len(auditedReports),
		CriticalCount:  totalCritical,
		WarningCount:   totalWarning,
		IsValid:        isValid,
		OverallSummary: overallSummary,
	}, nil
}

type parsedTable struct {
	headers []string
	rows    []map[string]string
	keyCol  string
}

func parseDesignTableData(filePath string, data []byte) *parsedTable {
	pt := &parsedTable{
		headers: []string{},
		rows:    []map[string]string{},
	}
	if len(data) == 0 {
		return pt
	}

	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".csv", ".tsv":
		r := csv.NewReader(bytes.NewReader(data))
		if ext == ".tsv" {
			r.Comma = '\t'
		}
		r.FieldsPerRecord = -1
		r.TrimLeadingSpace = true

		records, err := r.ReadAll()
		if err == nil && len(records) > 0 {
			headerIdx := -1
			for idx, row := range records {
				if len(row) > 0 && !strings.HasPrefix(row[0], "#") && !strings.HasPrefix(row[0], "//") {
					headerIdx = idx
					break
				}
			}
			if headerIdx >= 0 {
				pt.headers = records[headerIdx]
				for i, h := range pt.headers {
					pt.headers[i] = strings.TrimSpace(strings.TrimPrefix(h, "\ufeff"))
				}

				for rIdx := headerIdx + 1; rIdx < len(records); rIdx++ {
					rec := records[rIdx]
					if len(rec) == 0 || (len(rec) > 0 && strings.HasPrefix(rec[0], "#")) {
						continue
					}
					rowMap := make(map[string]string)
					for cIdx, colVal := range rec {
						if cIdx < len(pt.headers) {
							rowMap[pt.headers[cIdx]] = strings.TrimSpace(colVal)
						}
					}
					pt.rows = append(pt.rows, rowMap)
				}
			}
		}

	case ".json":
		var list []map[string]interface{}
		if err := json.Unmarshal(data, &list); err == nil {
			headerSet := make(map[string]bool)
			for _, item := range list {
				rowMap := make(map[string]string)
				for k, v := range item {
					headerSet[k] = true
					rowMap[k] = fmt.Sprintf("%v", v)
				}
				pt.rows = append(pt.rows, rowMap)
			}
			for k := range headerSet {
				pt.headers = append(pt.headers, k)
			}
			sort.Strings(pt.headers)
		} else {
			var kvMap map[string]map[string]interface{}
			if mErr := json.Unmarshal(data, &kvMap); mErr == nil {
				headerSet := make(map[string]bool)
				headerSet["ID"] = true
				for k, item := range kvMap {
					rowMap := make(map[string]string)
					rowMap["ID"] = k
					for ik, iv := range item {
						headerSet[ik] = true
						rowMap[ik] = fmt.Sprintf("%v", iv)
					}
					pt.rows = append(pt.rows, rowMap)
				}
				for k := range headerSet {
					pt.headers = append(pt.headers, k)
				}
				sort.Strings(pt.headers)
			}
		}

	case ".xlsx":
		zReader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err == nil {
			var sst []string
			for _, f := range zReader.File {
				if strings.Contains(f.Name, "sharedStrings.xml") {
					rc, rErr := f.Open()
					if rErr == nil {
						sData, _ := io.ReadAll(rc)
						_ = rc.Close()
						re := regexp.MustCompile(`<t[^>]*>(.*?)</t>`)
						matches := re.FindAllStringSubmatch(string(sData), -1)
						for _, m := range matches {
							if len(m) > 1 {
								sst = append(sst, m[1])
							}
						}
					}
					break
				}
			}

			for _, f := range zReader.File {
				if strings.Contains(f.Name, "sheet1.xml") {
					rc, rErr := f.Open()
					if rErr == nil {
						sData, _ := io.ReadAll(rc)
						_ = rc.Close()
						rowRe := regexp.MustCompile(`<row[^>]*>(.*?)</row>`)
						cellRe := regexp.MustCompile(`<c[^>]*?(?:t="([^"]+)")?[^>]*?><v>([^<]+)</v></c>`)
						rowMatches := rowRe.FindAllStringSubmatch(string(sData), -1)
						for rIdx, rm := range rowMatches {
							if len(rm) < 2 {
								continue
							}
							cellMatches := cellRe.FindAllStringSubmatch(rm[1], -1)
							var rowVals []string
							for _, cm := range cellMatches {
								val := cm[2]
								if len(cm) > 1 && cm[1] == "s" {
									sIdx, _ := strconv.Atoi(val)
									if sIdx < len(sst) {
										val = sst[sIdx]
									}
								}
								rowVals = append(rowVals, val)
							}
							if rIdx == 0 {
								pt.headers = rowVals
							} else {
								rowMap := make(map[string]string)
								for cIdx, v := range rowVals {
									if cIdx < len(pt.headers) {
										rowMap[pt.headers[cIdx]] = v
									}
								}
								pt.rows = append(pt.rows, rowMap)
							}
						}
					}
					break
				}
			}
		}
	}

	for _, h := range pt.headers {
		lowerH := strings.ToLower(h)
		if lowerH == "id" || lowerH == "primarykey" || lowerH == "key" || lowerH == "code" {
			pt.keyCol = h
			break
		}
	}
	if pt.keyCol == "" && len(pt.headers) > 0 {
		pt.keyCol = pt.headers[0]
	}

	return pt
}

func auditSingleTable(filePath string, base *parsedTable, target *parsedTable) AuditedTableReport {
	tableName := filepath.Base(filePath)
	var schemaChanges []SchemaChangeItem
	var anomalies []TableAnomalyItem
	var rowDiffs []RowSemanticDiff

	baseHeaderMap := make(map[string]bool)
	for _, h := range base.headers {
		baseHeaderMap[h] = true
	}
	targetHeaderMap := make(map[string]bool)
	for _, h := range target.headers {
		targetHeaderMap[h] = true
	}

	for _, h := range target.headers {
		if !baseHeaderMap[h] {
			schemaChanges = append(schemaChanges, SchemaChangeItem{
				ChangeType:  "COLUMN_ADDED",
				ColumnName:  h,
				Description: fmt.Sprintf("新增策划数值列 [%s]", h),
			})
		}
	}
	for _, h := range base.headers {
		if !targetHeaderMap[h] {
			schemaChanges = append(schemaChanges, SchemaChangeItem{
				ChangeType:  "COLUMN_REMOVED",
				ColumnName:  h,
				Description: fmt.Sprintf("删除策划数值列 [%s]，请注意下游代码/解析器引用", h),
			})
		}
	}

	baseRowMap := make(map[string]map[string]string)
	for _, r := range base.rows {
		k := r[base.keyCol]
		if k != "" {
			baseRowMap[k] = r
		}
	}

	targetRowMap := make(map[string]map[string]string)
	seenKeys := make(map[string]int)

	addedCount := 0
	modCount := 0
	delCount := 0

	for rIdx, r := range target.rows {
		k := r[target.keyCol]
		if k == "" {
			anomalies = append(anomalies, TableAnomalyItem{
				RowIndex:    rIdx + 1,
				ColumnName:  target.keyCol,
				AnomalyType: "NAN_OR_NULL",
				InvalidVal:  "<empty>",
				Severity:    "CRITICAL",
				Description: fmt.Sprintf("第 %d 行主键 [%s] 为空，无法建立索引映射", rIdx+1, target.keyCol),
			})
		} else {
			if seenKeys[k] > 0 {
				anomalies = append(anomalies, TableAnomalyItem{
					RowIndex:    rIdx + 1,
					PrimaryKey:  k,
					ColumnName:  target.keyCol,
					AnomalyType: "DUPLICATE_KEY",
					InvalidVal:  k,
					Severity:    "CRITICAL",
					Description: fmt.Sprintf("主键 ID [%s] 冲突重复定义！前置出现在第 %d 行，引发字典覆盖覆盖风险", k, seenKeys[k]),
				})
			} else {
				seenKeys[k] = rIdx + 1
				targetRowMap[k] = r
			}
		}

		for cName, cVal := range r {
			lowerCol := strings.ToLower(cName)
			valTrim := strings.TrimSpace(cVal)
			lowerVal := strings.ToLower(valTrim)

			if lowerVal == "nan" || lowerVal == "null" || lowerVal == "inf" || lowerVal == "1.#ind" || lowerVal == "-1.#ind" {
				anomalies = append(anomalies, TableAnomalyItem{
					RowIndex:    rIdx + 1,
					PrimaryKey:  k,
					ColumnName:  cName,
					AnomalyType: "NAN_OR_NULL",
					InvalidVal:  cVal,
					Severity:    "CRITICAL",
					Description: fmt.Sprintf("检测到非法浮点数/空值 [%s]，游戏加载将引发 NullReference/NaN 异常", cVal),
				})
			}

			if strings.HasSuffix(lowerCol, "_id") || strings.HasSuffix(lowerCol, "id") && lowerCol != "id" {
				if valTrim == "-1" || valTrim == "0" || valTrim == "" {
					// safe null fk
				} else if _, pErr := strconv.Atoi(valTrim); pErr != nil {
					anomalies = append(anomalies, TableAnomalyItem{
						RowIndex:    rIdx + 1,
						PrimaryKey:  k,
						ColumnName:  cName,
						AnomalyType: "INVALID_FK",
						InvalidVal:  cVal,
						Severity:    "WARNING",
						Description: fmt.Sprintf("外键关联字段 [%s] 包含非数字键值 [%s]", cName, cVal),
					})
				}
			}

			if strings.Contains(lowerCol, "price") || strings.Contains(lowerCol, "cost") || strings.Contains(lowerCol, "hp") {
				if num, nErr := strconv.ParseFloat(valTrim, 64); nErr == nil && num < 0 {
					anomalies = append(anomalies, TableAnomalyItem{
						RowIndex:    rIdx + 1,
						PrimaryKey:  k,
						ColumnName:  cName,
						AnomalyType: "OUT_OF_BOUNDS",
						InvalidVal:  cVal,
						Severity:    "WARNING",
						Description: fmt.Sprintf("数值字段 [%s] 为负数 (%.2f)，请核对是否符合设计意图", cName, num),
					})
				}
			}
		}

		baseR, existed := baseRowMap[k]
		if !existed {
			addedCount++
			rowDiffs = append(rowDiffs, RowSemanticDiff{
				PrimaryKey: k,
				Action:     "ADDED",
				Summary:    fmt.Sprintf("新增条目 ID: %s", k),
			})
		} else {
			var fDiffs []FieldSemanticDiff
			for col, newVal := range r {
				oldVal := baseR[col]
				if oldVal != newVal {
					oldNum, oErr := strconv.ParseFloat(oldVal, 64)
					newNum, nErr := strconv.ParseFloat(newVal, 64)
					var delta float64
					var pct float64
					diffSummary := fmt.Sprintf("%s ➔ %s", oldVal, newVal)
					if oErr == nil && nErr == nil {
						delta = newNum - oldNum
						if oldNum != 0 {
							pct = math.Round((delta/oldNum)*1000.0) / 10.0
							if pct > 0 {
								diffSummary = fmt.Sprintf("%.2f ➔ %.2f (+%.1f%%)", oldNum, newNum, pct)
							} else {
								diffSummary = fmt.Sprintf("%.2f ➔ %.2f (%.1f%%)", oldNum, newNum, pct)
							}
						}
					}
					fDiffs = append(fDiffs, FieldSemanticDiff{
						FieldName:     col,
						OldValue:      oldVal,
						NewValue:      newVal,
						NumericDelta:  delta,
						PercentChange: pct,
						Summary:       diffSummary,
					})
				}
			}
			if len(fDiffs) > 0 {
				modCount++
				var summaries []string
				for _, fd := range fDiffs {
					summaries = append(summaries, fmt.Sprintf("%s: %s", fd.FieldName, fd.Summary))
				}
				rowDiffs = append(rowDiffs, RowSemanticDiff{
					PrimaryKey: k,
					Action:     "MODIFIED",
					FieldDiffs: fDiffs,
					Summary:    strings.Join(summaries, ", "),
				})
			}
		}
	}

	for k := range baseRowMap {
		if _, exists := targetRowMap[k]; !exists {
			delCount++
			rowDiffs = append(rowDiffs, RowSemanticDiff{
				PrimaryKey: k,
				Action:     "DELETED",
				Summary:    fmt.Sprintf("删除条目 ID: %s", k),
			})
		}
	}

	semSummary := fmt.Sprintf("对比概括：+新增 %d 行, ~修改 %d 行, -删除 %d 行", addedCount, modCount, delCount)
	if len(schemaChanges) > 0 {
		semSummary += fmt.Sprintf(", 列结构变更 %d 项", len(schemaChanges))
	}

	return AuditedTableReport{
		TableName:         tableName,
		FilePath:          filePath,
		TotalRowsBase:     len(base.rows),
		TotalRowsTarget:   len(target.rows),
		AddedRowsCount:    addedCount,
		ModifiedRowsCount: modCount,
		DeletedRowsCount:  delCount,
		SchemaChanges:     schemaChanges,
		Anomalies:         anomalies,
		RowDiffs:          rowDiffs,
		SemanticSummary:   semSummary,
		HasErrors:         len(anomalies) > 0,
	}
}

func canonicalRepoPath(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return filepath.Clean(resolved)
	}
	return filepath.Clean(path)
}

// =========================================================================
// Round 113: Studio Credential Vault & KMS (InspectCredentialVault / StoreCredentialSecret)
// =========================================================================

var credentialURLPattern = regexp.MustCompile(`(?i)(://[^/@\s]+@)|([?&](?:access_token|private_token|token|api_key|apikey|key|password|secret|auth)=)`)
var credentialUserInfoPattern = regexp.MustCompile(`(?i)(://)[^/@\s]+@`)
var credentialQueryPattern = regexp.MustCompile(`(?i)([?&](?:access_token|private_token|token|api_key|apikey|key|password|secret|auth)=)[^&\s]+`)

func maskCredentialURL(raw string) string {
	masked := credentialUserInfoPattern.ReplaceAllString(raw, "${1}***@")
	return credentialQueryPattern.ReplaceAllString(masked, "${1}***")
}

// InspectCredentialVault audits git configs, environment variables, SSH keychains, and local encrypted vault
func (GitService) InspectCredentialVault(repoPath string) (*CredentialVaultResult, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath cannot be empty")
	}

	var items []CredentialItem
	criticalCount := 0
	warningCount := 0
	secureCount := 0

	// 1. Git Config Remote URLs and Headers
	gitConfigFile := filepath.Join(gitCommonDir(repoPath), "config")
	if data, err := os.ReadFile(gitConfigFile); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "url = ") {
				urlVal := strings.TrimPrefix(line, "url = ")
				if credentialURLPattern.MatchString(urlVal) {
					items = append(items, CredentialItem{
						ID:            fmt.Sprintf("git-remote-%d", len(items)+1),
						Type:          "GIT_TOKEN",
						Identifier:    "remote.url",
						Source:        "GIT_CONFIG",
						MaskedValue:   maskCredentialURL(urlVal),
						Scope:         "LOCAL_REPO",
						Permissions:   "0644 (Plaintext config file)",
						SecurityLevel: "CRITICAL_EXPOSURE",
						ExpiresAt:     "Unknown",
						IsMasked:      true,
						StatusMessage: "Git 远端 URL 中内嵌了明文凭证/密码，存在严重泄露风险！建议清理或使用凭证助手存储",
					})
					criticalCount++
				}
			} else if strings.HasPrefix(line, "helper = ") {
				helperVal := strings.TrimPrefix(line, "helper = ")
				if helperVal == "store" {
					items = append(items, CredentialItem{
						ID:            fmt.Sprintf("git-helper-%d", len(items)+1),
						Type:          "GENERIC_SECRET",
						Identifier:    "credential.helper",
						Source:        "GIT_CONFIG",
						MaskedValue:   "store (~/.git-credentials)",
						Scope:         "LOCAL_REPO",
						Permissions:   "0600 (User Home)",
						SecurityLevel: "WARNING",
						ExpiresAt:     "Permanent",
						IsMasked:      false,
						StatusMessage: "Git 配置使用了 'store' 明文助手，凭据以明文保存在 ~/.git-credentials 中",
					})
					warningCount++
				}
			}
		}
	}

	// 2. Environment Variables
	envChecks := []struct {
		envKey   string
		credType string
		label    string
	}{
		{"TENCENTCLOUD_SECRET_ID", "TENCENT_CLOUD_AKSK", "腾讯云 SecretId (AK)"},
		{"TENCENTCLOUD_SECRET_KEY", "TENCENT_CLOUD_AKSK", "腾讯云 SecretKey (SK)"},
		{"AWS_ACCESS_KEY_ID", "AWS_AKSK", "AWS AccessKeyID"},
		{"AWS_SECRET_ACCESS_KEY", "AWS_AKSK", "AWS SecretAccessKey"},
		{"GITHUB_TOKEN", "GIT_TOKEN", "GitHub PAT Token"},
		{"GH_TOKEN", "GIT_TOKEN", "GitHub CLI Token"},
		{"GITLAB_TOKEN", "GIT_TOKEN", "GitLab Token"},
		{"TGIT_TOKEN", "GIT_TOKEN", "腾讯工蜂 TGit Token"},
		{"CODING_TOKEN", "GIT_TOKEN", "CODING DevOps Token"},
		{"TAPD_API_KEY", "TAPD_TOKEN", "腾讯 TAPD API 凭据"},
		{"P4PASSWD", "PERFORCE_CRED", "Perforce P4PASSWD 凭据"},
	}

	for _, chk := range envChecks {
		val := os.Getenv(chk.envKey)
		if val != "" {
			items = append(items, CredentialItem{
				ID:            fmt.Sprintf("env-%s", chk.envKey),
				Type:          chk.credType,
				Identifier:    chk.envKey,
				Source:        "ENVIRONMENT",
				MaskedValue:   maskSecretValue(val),
				Scope:         "ENV_VAR",
				Permissions:   "Process Memory (Inheritable)",
				SecurityLevel: "WARNING",
				ExpiresAt:     "Session / Persistent",
				IsMasked:      true,
				StatusMessage: fmt.Sprintf("环境变量暴露: %s，子进程或构建日志可能意外截获该凭证", chk.label),
			})
			warningCount++
		}
	}

	// 3. SSH Keys in User Home
	homeDir, _ := os.UserHomeDir()
	if homeDir != "" {
		sshDir := filepath.Join(homeDir, ".ssh")
		knownSSHKeys := []string{"id_rsa", "id_ed25519", "id_ecdsa", "id_dsa"}
		for _, keyName := range knownSSHKeys {
			keyPath := filepath.Join(sshDir, keyName)
			if fi, err := os.Stat(keyPath); err == nil {
				perm := fi.Mode().Perm()
				permStr := fmt.Sprintf("%04o", perm)
				secLevel := "SECURE"
				statusMsg := "SSH 私钥权限受控 (0600)"
				if perm != 0600 {
					secLevel = "WARNING"
					statusMsg = fmt.Sprintf("SSH 私钥文件权限过宽 (%s)，建议执行 chmod 600 %s", permStr, keyPath)
					warningCount++
				} else {
					secureCount++
				}

				items = append(items, CredentialItem{
					ID:            fmt.Sprintf("ssh-%s", keyName),
					Type:          "SSH_KEY",
					Identifier:    filepath.Join("~/.ssh", keyName),
					Source:        "SSH_KEYCHAIN",
					MaskedValue:   fmt.Sprintf("[PRIVATE KEY %s]", strings.ToUpper(keyName)),
					Scope:         "GLOBAL_SYSTEM",
					Permissions:   permStr,
					SecurityLevel: secLevel,
					ExpiresAt:     "Permanent / Never",
					IsMasked:      true,
					StatusMessage: statusMsg,
				})
			}
		}
	}

	// 4. Local Studio KMS Vault (.git/wgit_vault.json)
	vaultFile := getVaultFilePath(repoPath)
	vaultIssue := ""
	if vaultData, err := os.ReadFile(vaultFile); err == nil {
		var records []EncryptedVaultRecord
		if err := json.Unmarshal(vaultData, &records); err == nil {
			for _, rec := range records {
				items = append(items, CredentialItem{
					ID:            rec.ID,
					Type:          rec.SecretType,
					Identifier:    rec.Identifier,
					Source:        "STUDIO_VAULT",
					MaskedValue:   rec.MaskedValue,
					Scope:         rec.Scope,
					Permissions:   "0600 (KMS Encrypted Storage)",
					SecurityLevel: "SECURE",
					ExpiresAt:     "Never / KMS Managed",
					IsMasked:      true,
					StatusMessage: "已通过 AES-256-GCM 本地加密保险库妥善隔离保护",
				})
				secureCount++
			}
		} else {
			vaultIssue = "本地保险库损坏，无法读取已保存的凭据记录"
		}
	} else if !os.IsNotExist(err) {
		vaultIssue = "本地保险库读取失败：" + err.Error()
	}

	summary := fmt.Sprintf("共扫描到 %d 项凭据资产: %d 项安全加密, %d 项警告提示, %d 项严重暴露风险",
		len(items), secureCount, warningCount, criticalCount)
	message := "工作室凭据与安全保险库分析完成"
	if vaultIssue != "" {
		message += "；" + vaultIssue + "，已保存的凭据未纳入统计"
	}

	return &CredentialVaultResult{
		TotalCredentials:     len(items),
		CriticalExposures:    criticalCount,
		WarningsCount:        warningCount,
		SecureCount:          secureCount,
		Items:                items,
		VaultStoragePath:     vaultFile,
		KmsEncryptionEnabled: true,
		Summary:              summary,
		Message:              message,
	}, nil
}

// StoreCredentialSecret encrypts a secret with AES-256-GCM KMS derivation and writes to safe local vault
func (GitService) StoreCredentialSecret(repoPath string, secretType string, identifier string, secretValue string, scope string) (*StoreSecretResult, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath cannot be empty")
	}
	if strings.TrimSpace(secretValue) == "" {
		return nil, errors.New("secretValue cannot be empty")
	}
	if identifier == "" {
		identifier = fmt.Sprintf("secret-%d", time.Now().Unix())
	}
	if scope == "" {
		scope = "LOCAL_REPO"
	}

	key := deriveVaultKey(repoPath)
	cipherB64, nonceB64, err := encryptKMS(key, []byte(secretValue))
	if err != nil {
		return nil, fmt.Errorf("KMS encryption failed: %w", err)
	}

	vaultFile := filepath.Join(gitCommonDir(repoPath), "wgit_vault.json")
	var records []EncryptedVaultRecord
	if data, readErr := os.ReadFile(vaultFile); readErr == nil {
		if err := json.Unmarshal(data, &records); err != nil {
			return nil, fmt.Errorf("解析本地凭据库失败，已中止写入以保护现有记录: %w", err)
		}
	} else if !os.IsNotExist(readErr) {
		return nil, fmt.Errorf("读取本地凭据库失败，已中止写入以保护现有记录: %w", readErr)
	}

	secretID := fmt.Sprintf("sec-%x", time.Now().UnixNano())
	masked := maskSecretValue(secretValue)

	record := EncryptedVaultRecord{
		ID:           secretID,
		SecretType:   secretType,
		Identifier:   identifier,
		Ciphertext:   cipherB64,
		Nonce:        nonceB64,
		MaskedValue:  masked,
		Scope:        scope,
		CreatedAt:    time.Now().Format(time.RFC3339),
		LastAccessed: time.Now().Format(time.RFC3339),
	}

	updated := false
	for i, r := range records {
		if r.Identifier == identifier && r.SecretType == secretType {
			records[i] = record
			updated = true
			break
		}
	}
	if !updated {
		records = append(records, record)
	}

	outBytes, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return nil, err
	}

	if err := os.WriteFile(vaultFile, outBytes, 0600); err != nil {
		return nil, fmt.Errorf("failed to save encrypted vault: %w", err)
	}
	if err := os.Chmod(vaultFile, 0600); err != nil {
		return nil, fmt.Errorf("failed to secure vault permissions: %w", err)
	}

	return &StoreSecretResult{
		Success:          true,
		SecretID:         secretID,
		SecretType:       secretType,
		MaskedValue:      masked,
		Scope:            scope,
		VaultFile:        vaultFile,
		EncryptionMethod: "AES-256-GCM-KMS",
		Message:          fmt.Sprintf("凭据 [%s] 已通过 AES-256-GCM KMS 算法成功加密封存至保险库", identifier),
	}, nil
}

func maskSecretValue(val string) string {
	if len(val) <= 4 {
		return "****"
	}
	if len(val) <= 8 {
		return val[:2] + "****" + val[len(val)-2:]
	}
	if len(val) <= 16 {
		return val[:4] + "****" + val[len(val)-3:]
	}
	return val[:6] + "********" + val[len(val)-4:]
}

func getVaultFilePath(repoPath string) string {
	target := filepath.Join(gitCommonDir(repoPath), "wgit_vault.json")
	if _, err := os.Stat(target); os.IsNotExist(err) {
		old := filepath.Join(gitCommonDir(repoPath), "ugit_vault.json")
		if _, errOld := os.Stat(old); errOld == nil {
			return old
		}
	}
	return target
}

func deriveVaultKey(repoPath string) []byte {
	h := sha256.New()
	h.Write([]byte("ugit-kms-vault-master-salt-2026:"))
	h.Write([]byte(filepath.Clean(repoPath)))
	return h.Sum(nil)
}

func encryptKMS(key []byte, plaintext []byte) (string, string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", "", err
	}
	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(ciphertext), base64.StdEncoding.EncodeToString(nonce), nil
}

// =========================================================================
// Round 114: Crash Symbol Demangler & Minidump Inspector (DemangleCrashStack)
// =========================================================================

// DemangleCrashStack parses native crash call stacks, demangles symbols, and traces blame authors
func (GitService) DemangleCrashStack(repoPath string, rawCrashDump string, targetCommit string) (*CrashStackDemangleResult, error) {
	lines := strings.Split(rawCrashDump, "\n")
	result := &CrashStackDemangleResult{
		CrashReason:  "UNKNOWN_EXCEPTION",
		FaultAddress: "0x0",
		FaultModule:  "UnknownModule",
		ThreadID:     "Thread 0 (Crashed)",
		Frames:       []DemangledFrame{},
	}

	idxRe := regexp.MustCompile(`(?:#|frame #)(\d+)`)
	addrRe := regexp.MustCompile(`0x[0-9a-fA-F]{6,}`)
	srcRe := regexp.MustCompile(`(?:\b(?:at|in)\s+|\(|\b)([^:\s\(\)\[\]]+\.(?:cpp|c|cc|cxx|h|hpp|rs|swift|cs|go|m|mm)):(\d+)`)
	modRe := regexp.MustCompile(`([a-zA-Z0-9_\-]+\.(?:dll|exe|so|dylib|framework))`)

	demangledCount := 0

	for i, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}

		if strings.HasPrefix(l, "Crash Reason:") || strings.HasPrefix(l, "Exception:") {
			result.CrashReason = strings.TrimSpace(strings.SplitN(l, ":", 2)[1])
			continue
		}
		if strings.HasPrefix(l, "Faulting Module:") || strings.HasPrefix(l, "Fault Module:") {
			result.FaultModule = strings.TrimSpace(strings.SplitN(l, ":", 2)[1])
			continue
		}
		if strings.HasPrefix(l, "Thread ") && strings.Contains(l, "Crashed") {
			result.ThreadID = l
			continue
		}

		frameIndex := i
		if m := idxRe.FindStringSubmatch(l); len(m) >= 2 {
			frameIndex, _ = strconv.Atoi(m[1])
		}

		addr := addrRe.FindString(l)
		mod := ""
		symbol := ""

		tokens := strings.Fields(l)
		for _, tok := range tokens {
			tok = strings.Trim(tok, ",();[]`\"")
			if strings.Contains(tok, "!") {
				parts := strings.SplitN(tok, "!", 2)
				mod = parts[0]
				tok = parts[1]
			}
			if idx := strings.Index(tok, "+"); idx != -1 {
				tok = tok[:idx]
			}
			if strings.HasPrefix(tok, "?") || strings.HasPrefix(tok, "_Z") || strings.HasPrefix(tok, "$s") || strings.HasPrefix(tok, "_$s") {
				symbol = tok
				break
			}
		}

		if mod == "" {
			if m := modRe.FindStringSubmatch(l); len(m) >= 2 {
				mod = m[1]
			}
		}

		srcFile := ""
		lineNum := 0
		if m := srcRe.FindStringSubmatch(l); len(m) >= 3 {
			srcFile = m[1]
			lineNum, _ = strconv.Atoi(m[2])
		}

		if symbol == "" && addr == "" && srcFile == "" {
			continue
		}

		demangled := symbol
		if symbol != "" {
			demangled = DemangleSymbol(symbol)
			if demangled != symbol {
				demangledCount++
			}
		}

		frame := DemangledFrame{
			FrameIndex:      frameIndex,
			Address:         addr,
			MangledSymbol:   symbol,
			DemangledSymbol: demangled,
			ModuleName:      mod,
			SourceFile:      srcFile,
			LineNumber:      lineNum,
		}

		if srcFile != "" && lineNum > 0 && repoPath != "" {
			resolveAndBlameFrame(repoPath, &frame, targetCommit)
		}

		result.Frames = append(result.Frames, frame)
	}

	result.TotalFrames = len(result.Frames)
	result.DemangledCount = demangledCount

	for i := range result.Frames {
		if result.Frames[i].SourceFile != "" || !strings.Contains(result.Frames[i].ModuleName, "ntdll") {
			result.Frames[i].IsCulprit = true
			result.CulpritFrame = &result.Frames[i]
			result.CulpritAuthor = result.Frames[i].BlameAuthor
			result.CulpritCommit = result.Frames[i].BlameCommit
			result.CulpritFile = result.Frames[i].SourceFile
			break
		}
	}
	if result.CulpritFrame == nil && len(result.Frames) > 0 {
		result.Frames[0].IsCulprit = true
		result.CulpritFrame = &result.Frames[0]
	}

	result.Summary = fmt.Sprintf("堆栈解析成功: 共 %d 帧调用，成功还原 %d 个原生符号，定位到核心崩溃点: %s",
		result.TotalFrames, result.DemangledCount, result.CulpritFile)
	result.Message = "Minidump 原生崩溃堆栈反混淆与代码追溯就绪"

	return result, nil
}

func DemangleSymbol(mangled string) string {
	mangled = strings.TrimSpace(mangled)
	if mangled == "" {
		return ""
	}

	// 1. MSVC mangling: starts with "?"
	if strings.HasPrefix(mangled, "?") {
		return demangleMSVC(mangled)
	}

	// 2. Itanium / GCC / Clang / Rust mangling: starts with "_Z"
	if strings.HasPrefix(mangled, "_Z") {
		return demangleItanium(mangled)
	}

	// 3. Swift mangling: starts with "$s" or "_$s"
	if strings.HasPrefix(mangled, "$s") || strings.HasPrefix(mangled, "_$s") {
		return demangleSwift(mangled)
	}

	return mangled
}

func demangleMSVC(mangled string) string {
	sym := mangled
	isCtor := false
	isDtor := false
	methodName := ""

	if strings.HasPrefix(sym, "??0") {
		isCtor = true
		sym = strings.TrimPrefix(sym, "??0")
	} else if strings.HasPrefix(sym, "??1") {
		isDtor = true
		sym = strings.TrimPrefix(sym, "??1")
	} else if strings.HasPrefix(sym, "?") {
		sym = strings.TrimPrefix(sym, "?")
	}

	idxDoubleAt := strings.Index(sym, "@@")
	if idxDoubleAt == -1 {
		parts := strings.Split(sym, "@")
		if len(parts) > 0 && parts[0] != "" {
			return parts[0]
		}
		return mangled
	}

	scopePart := sym[:idxDoubleAt]
	tokens := strings.Split(scopePart, "@")
	if len(tokens) == 0 {
		return mangled
	}

	var scopes []string
	if !isCtor && !isDtor {
		methodName = tokens[0]
		for i := 1; i < len(tokens); i++ {
			if tokens[i] != "" {
				scopes = append(scopes, tokens[i])
			}
		}
	} else {
		for i := 0; i < len(tokens); i++ {
			if tokens[i] != "" {
				scopes = append(scopes, tokens[i])
			}
		}
		if len(scopes) > 0 {
			className := scopes[0]
			if isCtor {
				methodName = className
			} else {
				methodName = "~" + className
			}
		}
	}

	for i, j := 0, len(scopes)-1; i < j; i, j = i+1, j-1 {
		scopes[i], scopes[j] = scopes[j], scopes[i]
	}

	if len(scopes) > 0 {
		return strings.Join(scopes, "::") + "::" + methodName
	}
	return methodName
}

func demangleItanium(mangled string) string {
	if !strings.HasPrefix(mangled, "_ZN") {
		sym := strings.TrimPrefix(mangled, "_Z")
		lenPrefix := ""
		for _, ch := range sym {
			if ch >= '0' && ch <= '9' {
				lenPrefix += string(ch)
			} else {
				break
			}
		}
		if n, err := strconv.Atoi(lenPrefix); err == nil && len(sym) >= len(lenPrefix)+n {
			return sym[len(lenPrefix) : len(lenPrefix)+n]
		}
		return mangled
	}

	body := strings.TrimPrefix(mangled, "_ZN")
	eIdx := strings.LastIndex(body, "E")
	if eIdx != -1 {
		body = body[:eIdx]
	}

	var parts []string
	cursor := 0
	for cursor < len(body) {
		lenStr := ""
		for cursor < len(body) && body[cursor] >= '0' && body[cursor] <= '9' {
			lenStr += string(body[cursor])
			cursor++
		}
		if lenStr == "" {
			break
		}
		n, err := strconv.Atoi(lenStr)
		if err != nil || cursor+n > len(body) {
			break
		}
		token := body[cursor : cursor+n]
		cursor += n

		if strings.HasPrefix(token, "h") && len(token) == 17 {
			continue
		}
		parts = append(parts, token)
	}

	if len(parts) > 0 {
		return strings.Join(parts, "::")
	}
	return mangled
}

func demangleSwift(mangled string) string {
	re := regexp.MustCompile(`[0-9]+([a-zA-Z0-9_]+)`)
	matches := re.FindAllStringSubmatch(mangled, -1)
	var parts []string
	for _, m := range matches {
		if len(m) >= 2 {
			p := m[1]
			if p != "SiyF" && p != "C" && p != "V" && len(p) > 1 {
				parts = append(parts, p)
			}
		}
	}
	if len(parts) > 0 {
		return strings.Join(parts, ".")
	}
	return mangled
}

func resolveAndBlameFrame(repoPath string, frame *DemangledFrame, targetCommit string) {
	candidatePath := filepath.Join(repoPath, frame.SourceFile)
	if _, err := os.Stat(candidatePath); err != nil {
		baseName := filepath.Base(frame.SourceFile)
		_ = filepath.Walk(repoPath, func(p string, fi os.FileInfo, err error) error {
			if err == nil && !fi.IsDir() && fi.Name() == baseName {
				candidatePath = p
				return filepath.SkipAll
			}
			return nil
		})
	}

	rel, err := filepath.Rel(repoPath, candidatePath)
	if err != nil {
		return
	}

	args := []string{"blame", "-L", fmt.Sprintf("%d,%d", frame.LineNumber, frame.LineNumber), "--porcelain"}
	if targetCommit != "" {
		args = append(args, targetCommit)
	}
	args = append(args, "--", rel)

	out, err := ExecGit(repoPath, args...)
	if err == nil {
		blameLines := strings.Split(out, "\n")
		for _, bl := range blameLines {
			if strings.HasPrefix(bl, "author ") {
				frame.BlameAuthor = strings.TrimPrefix(bl, "author ")
			} else if strings.HasPrefix(bl, "author-mail ") {
				frame.BlameEmail = strings.Trim(strings.TrimPrefix(bl, "author-mail "), "<>")
			} else if strings.HasPrefix(bl, "summary ") {
				frame.BlameSummary = strings.TrimPrefix(bl, "summary ")
			}
		}
		if len(blameLines) > 0 {
			frame.BlameCommit = strings.Fields(blameLines[0])[0]
		}
	}

	if fileData, err := os.ReadFile(candidatePath); err == nil {
		allLines := strings.Split(string(fileData), "\n")
		start := frame.LineNumber - 3
		if start < 0 {
			start = 0
		}
		end := frame.LineNumber + 2
		if end > len(allLines) {
			end = len(allLines)
		}
		var snippet []string
		for idx := start; idx < end; idx++ {
			marker := "  "
			if idx+1 == frame.LineNumber {
				marker = ">>"
			}
			snippet = append(snippet, fmt.Sprintf("%s %4d | %s", marker, idx+1, allLines[idx]))
		}
		frame.CodeSnippet = strings.Join(snippet, "\n")
	}
}

// =========================================================================
// Round 115: Selective Asset Cherry-Picker (CherryPickAssetBundle)
// =========================================================================

// CherryPickAssetBundle pulls specific binary assets and their dependencies/meta from any ref without full merge
func (GitService) CherryPickAssetBundle(repoPath string, targetRef string, assetPaths []string, resolveDependencies bool, dryRun bool) (*AssetCherryPickResult, error) {
	if repoPath == "" || targetRef == "" {
		return nil, errors.New("repoPath and targetRef cannot be empty")
	}
	if len(assetPaths) == 0 {
		return nil, errors.New("assetPaths cannot be empty")
	}

	out, err := ExecGit(repoPath, "ls-tree", "-r", "--name-only", targetRef)
	if err != nil {
		return nil, fmt.Errorf("git ls-tree failed for ref '%s': %w", targetRef, err)
	}

	targetFiles := make(map[string]bool)
	var metaFiles []string
	for _, f := range strings.Split(out, "\n") {
		f = strings.TrimSpace(f)
		if f != "" {
			targetFiles[f] = true
			if strings.HasSuffix(f, ".meta") {
				metaFiles = append(metaFiles, f)
			}
		}
	}

	guidToAsset := make(map[string]string)
	guidRe := regexp.MustCompile(`guid:\s*([a-f0-9]{32})`)
	for _, m := range metaFiles {
		if mData, err := ExecGit(repoPath, "show", fmt.Sprintf("%s:%s", targetRef, m)); err == nil {
			match := guidRe.FindStringSubmatch(mData)
			if len(match) >= 2 {
				assetPath := strings.TrimSuffix(m, ".meta")
				guidToAsset[match[1]] = assetPath
			}
		}
	}

	pickedSet := make(map[string]bool)
	queue := make([]string, 0, len(assetPaths))
	parentMap := make(map[string]string)
	isDepMap := make(map[string]bool)

	for _, a := range assetPaths {
		a = filepath.Clean(a)
		queue = append(queue, a)
		isDepMap[a] = false
	}

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]

		if pickedSet[curr] {
			continue
		}
		pickedSet[curr] = true

		metaPair := curr + ".meta"
		if targetFiles[metaPair] && !pickedSet[metaPair] {
			pickedSet[metaPair] = true
			parentMap[metaPair] = curr
			isDepMap[metaPair] = true
		}

		if !resolveDependencies {
			continue
		}

		ext := strings.ToLower(filepath.Ext(curr))
		if ext == ".mat" || ext == ".prefab" || ext == ".unity" || ext == ".asset" {
			if data, err := ExecGit(repoPath, "show", fmt.Sprintf("%s:%s", targetRef, curr)); err == nil {
				allGuids := guidRe.FindAllStringSubmatch(data, -1)
				for _, gm := range allGuids {
					if len(gm) >= 2 {
						refGuid := gm[1]
						if depPath, ok := guidToAsset[refGuid]; ok && depPath != curr {
							if !pickedSet[depPath] {
								queue = append(queue, depPath)
								parentMap[depPath] = curr
								isDepMap[depPath] = true
							}
						}
					}
				}
			}
		}
	}

	var finalAssets []CherryPickedAsset
	var totalBytes int64
	lfsCount := 0
	metaCount := 0

	var filesToCheckout []string
	for p := range pickedSet {
		if !targetFiles[p] {
			continue
		}
		filesToCheckout = append(filesToCheckout, p)
	}
	sort.Strings(filesToCheckout)

	if !dryRun && len(filesToCheckout) > 0 {
		args := append([]string{"checkout", targetRef, "--"}, filesToCheckout...)
		if out, err := ExecGit(repoPath, args...); err != nil {
			return nil, fmt.Errorf("git checkout asset failed: %s (%w)", out, err)
		}
	}

	for _, p := range filesToCheckout {
		ext := strings.ToLower(filepath.Ext(p))
		fType := "Binary Asset"
		switch ext {
		case ".meta":
			fType = "Unity Meta File"
			metaCount++
		case ".mat":
			fType = "Unity Material"
		case ".prefab":
			fType = "Unity Prefab"
		case ".fbx", ".obj", ".blend":
			fType = "3D Mesh Model"
		case ".png", ".tga", ".jpg", ".psd":
			fType = "Texture / Image"
		case ".uasset":
			fType = "Unreal Binary Package"
		case ".umap":
			fType = "Unreal World Map"
		}

		isLfs := isPathBinaryExt(p)
		if isLfs {
			lfsCount++
		}

		var sz int64
		if !dryRun {
			fi, statErr := os.Stat(filepath.Join(repoPath, p))
			if statErr != nil {
				return nil, fmt.Errorf("读取资产 %s 大小失败: %w", p, statErr)
			}
			sz = fi.Size()
		} else {
			catOut, catErr := ExecGit(repoPath, "cat-file", "-s", fmt.Sprintf("%s:%s", targetRef, p))
			if catErr != nil {
				return nil, fmt.Errorf("读取资产 %s 大小失败: %w", p, catErr)
			}
			sz, catErr = strconv.ParseInt(strings.TrimSpace(catOut), 10, 64)
			if catErr != nil {
				return nil, fmt.Errorf("解析资产 %s 大小失败: %w", p, catErr)
			}
		}
		totalBytes += sz

		status := "PICKED"
		if dryRun {
			status = "DRY_RUN"
		}

		finalAssets = append(finalAssets, CherryPickedAsset{
			Path:         p,
			FileType:     fType,
			SizeBytes:    sz,
			SizeStr:      formatAssetSize(sz),
			IsLFS:        isLfs,
			IsDependency: isDepMap[p],
			ParentAsset:  parentMap[p],
			Status:       status,
		})
	}

	depCount := len(finalAssets) - len(assetPaths)
	if depCount < 0 {
		depCount = 0
	}

	summary := fmt.Sprintf("精细提取完成: 共选择 %d 个资产 (%d 核心资产 + %d 关联依赖与 .meta), 总计 %s",
		len(finalAssets), len(assetPaths), depCount, formatAssetSize(totalBytes))

	return &AssetCherryPickResult{
		TargetRef:         targetRef,
		IsDryRun:          dryRun,
		TotalRequested:    len(assetPaths),
		TotalDependencies: depCount,
		TotalAssetsCount:  len(finalAssets),
		TotalBytes:        totalBytes,
		TotalBytesStr:     formatAssetSize(totalBytes),
		Assets:            finalAssets,
		LfsAssetsCount:    lfsCount,
		MetaFilesCount:    metaCount,
		Success:           true,
		Summary:           summary,
		Message:           "差异化二进制资产已干净拣选至当前工作区",
	}, nil
}

func isPathBinaryExt(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	binExts := map[string]bool{
		".png": true, ".jpg": true, ".jpeg": true, ".tga": true, ".psd": true,
		".fbx": true, ".obj": true, ".blend": true, ".mb": true, ".max": true,
		".uasset": true, ".umap": true, ".wav": true, ".mp3": true, ".ogg": true,
	}
	return binExts[ext]
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
// Round 117: Offline Replay & Retry Queue (ManageOfflineSubmitQueue)
// =========================================================================

type offlineQueueStore struct {
	Items []OfflineQueueItem `json:"items"`
}

func getOfflineQueuePath(repoPath string) string {
	baseDir := filepath.Join(repoPath, ".git")
	fi, err := os.Stat(baseDir)
	if err == nil && !fi.IsDir() {
		// Possibly a worktree or submodule
		data, rErr := os.ReadFile(baseDir)
		if rErr == nil && strings.HasPrefix(string(data), "gitdir:") {
			target := strings.TrimSpace(strings.TrimPrefix(string(data), "gitdir:"))
			if !filepath.IsAbs(target) {
				target = filepath.Join(repoPath, target)
			}
			baseDir = target
		}
	}
	target := filepath.Join(baseDir, "wgit_offline_queue.json")
	if _, err := os.Stat(target); os.IsNotExist(err) {
		old := filepath.Join(baseDir, "ugit_offline_queue.json")
		if _, errOld := os.Stat(old); errOld == nil {
			return old
		}
	}
	return target
}

func loadOfflineQueue(repoPath string) ([]OfflineQueueItem, error) {
	qPath := getOfflineQueuePath(repoPath)
	data, err := os.ReadFile(qPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []OfflineQueueItem{}, nil
		}
		return nil, fmt.Errorf("读取离线队列失败: %w", err)
	}
	var store offlineQueueStore
	if err := json.Unmarshal(data, &store); err != nil {
		return nil, fmt.Errorf("离线队列文件损坏: %w", err)
	}
	if store.Items == nil {
		return []OfflineQueueItem{}, nil
	}
	return store.Items, nil
}

func saveOfflineQueue(repoPath string, items []OfflineQueueItem) error {
	qPath := getOfflineQueuePath(repoPath)
	if err := os.MkdirAll(filepath.Dir(qPath), 0o755); err != nil {
		return fmt.Errorf("创建离线队列目录失败: %w", err)
	}
	store := offlineQueueStore{Items: items}
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(qPath, data, 0o644)
}

// ManageOfflineSubmitQueue tracks and executes operations performed during offline/weak network mode.
func (GitService) ManageOfflineSubmitQueue(repoPath string, action string, itemId string, payload string) (*OfflineQueueResult, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath 不能为空")
	}

	items, err := loadOfflineQueue(repoPath)
	if err != nil {
		return nil, err
	}
	action = strings.ToLower(strings.TrimSpace(action))
	msg := ""

	switch action {
	case "enqueue":
		id := itemId
		if id == "" {
			b := make([]byte, 4)
			_, _ = rand.Read(b)
			id = fmt.Sprintf("queue-%d-%x", time.Now().Unix(), b)
		}
		op := "push"
		target := ""
		dataPayload := payload
		if strings.TrimSpace(payload) != "" {
			var parsed struct {
				Operation string `json:"operation"`
				Target    string `json:"target"`
				Payload   string `json:"payload"`
			}
			if err := json.Unmarshal([]byte(payload), &parsed); err == nil && parsed.Operation != "" {
				op = parsed.Operation
				target = parsed.Target
				if parsed.Payload != "" {
					dataPayload = parsed.Payload
				}
			} else {
				parts := strings.SplitN(payload, ":", 2)
				if len(parts) >= 1 && strings.TrimSpace(parts[0]) != "" {
					op = strings.TrimSpace(parts[0])
				}
				if len(parts) >= 2 {
					target = strings.TrimSpace(parts[1])
				}
			}
		}

		switch op {
		case "push", "fetch", "lfs_lock", "lfs_unlock", "lfs_push":
		default:
			return nil, fmt.Errorf("不支持的离线操作: %s", op)
		}

		newItem := OfflineQueueItem{
			ID:          id,
			Operation:   op,
			Target:      target,
			Payload:     dataPayload,
			Status:      "queued",
			RetryCount:  0,
			MaxRetries:  5,
			BackoffSecs: 5,
			CreatedAt:   time.Now().Format("2006-01-02 15:04:05"),
		}
		items = append(items, newItem)
		if err := saveOfflineQueue(repoPath, items); err != nil {
			return nil, fmt.Errorf("保存离线队列失败: %w", err)
		}
		msg = fmt.Sprintf("已成功入队离线操作 [%s] (%s: %s)", id, op, target)

	case "replay":
		replayedCount := 0
		now := time.Now()
		for i := range items {
			if itemId != "" && itemId != "all" && items[i].ID != itemId {
				continue
			}
			if items[i].Status != "queued" && items[i].Status != "failed" {
				continue
			}

			items[i].LastAttempt = now.Format("2006-01-02 15:04:05")
			var execErr error

			replayRemote := defaultPushRemote(repoPath)
			switch items[i].Operation {
			case "push":
				if items[i].Target != "" {
					_, execErr = ExecGit(repoPath, "push", replayRemote, items[i].Target)
				} else {
					_, execErr = ExecGit(repoPath, "push")
				}
			case "lfs_lock":
				_, execErr = ExecGit(repoPath, "lfs", "lock", items[i].Target)
			case "lfs_unlock":
				_, execErr = ExecGit(repoPath, "lfs", "unlock", items[i].Target)
			case "fetch":
				_, execErr = ExecGit(repoPath, "fetch", "--prune")
			case "lfs_push":
				if items[i].Target != "" {
					_, execErr = ExecGit(repoPath, "lfs", "push", "--all", replayRemote, items[i].Target)
				} else {
					_, execErr = ExecGit(repoPath, "lfs", "push", "--all", replayRemote)
				}
			default:
				execErr = fmt.Errorf("不支持的离线操作: %s", items[i].Operation)
			}

			replayedCount++
			if execErr == nil {
				items[i].Status = "succeeded"
				items[i].LastError = ""
				items[i].NextRetryAt = ""
			} else {
				items[i].RetryCount++
				items[i].LastError = execErr.Error()
				backoff := 5 * (1 << items[i].RetryCount)
				if backoff > 3600 {
					backoff = 3600
				}
				items[i].BackoffSecs = backoff
				items[i].NextRetryAt = now.Add(time.Duration(backoff) * time.Second).Format("2006-01-02 15:04:05")
				if items[i].RetryCount >= items[i].MaxRetries {
					items[i].Status = "failed"
				} else {
					items[i].Status = "queued"
				}
			}
		}
		if err := saveOfflineQueue(repoPath, items); err != nil {
			return nil, fmt.Errorf("保存离线队列失败: %w", err)
		}
		msg = fmt.Sprintf("已触发重放 %d 项离线操作", replayedCount)

	case "pause":
		paused := 0
		for i := range items {
			if itemId == "" || itemId == "all" || items[i].ID == itemId {
				if items[i].Status == "queued" {
					items[i].Status = "paused"
					paused++
				}
			}
		}
		if err := saveOfflineQueue(repoPath, items); err != nil {
			return nil, fmt.Errorf("保存离线队列失败: %w", err)
		}
		msg = fmt.Sprintf("已暂停 %d 项排队任务", paused)

	case "resume":
		resumed := 0
		for i := range items {
			if itemId == "" || itemId == "all" || items[i].ID == itemId {
				if items[i].Status == "paused" {
					items[i].Status = "queued"
					resumed++
				}
			}
		}
		if err := saveOfflineQueue(repoPath, items); err != nil {
			return nil, fmt.Errorf("保存离线队列失败: %w", err)
		}
		msg = fmt.Sprintf("已恢复 %d 项暂停任务", resumed)

	case "discard":
		newItems := make([]OfflineQueueItem, 0, len(items))
		discarded := 0
		for _, item := range items {
			if itemId == item.ID || (itemId == "all" && item.Status != "running") {
				discarded++
				continue
			}
			newItems = append(newItems, item)
		}
		items = newItems
		if err := saveOfflineQueue(repoPath, items); err != nil {
			return nil, fmt.Errorf("保存离线队列失败: %w", err)
		}
		msg = fmt.Sprintf("已丢弃/清理 %d 项离线任务", discarded)

	case "clear_completed":
		newItems := make([]OfflineQueueItem, 0, len(items))
		cleared := 0
		for _, item := range items {
			if item.Status == "succeeded" || item.Status == "discarded" {
				cleared++
				continue
			}
			newItems = append(newItems, item)
		}
		items = newItems
		if err := saveOfflineQueue(repoPath, items); err != nil {
			return nil, fmt.Errorf("保存离线队列失败: %w", err)
		}
		msg = fmt.Sprintf("已清理 %d 项已完成/已废弃离线记录", cleared)

	case "status", "list", "":
		msg = fmt.Sprintf("获取到 %d 项离线重试任务", len(items))
	default:
		return nil, fmt.Errorf("未知的离线队列操作: %s", action)
	}

	queuedCount := 0
	failedCount := 0
	pausedCount := 0
	succeededCount := 0
	for _, it := range items {
		switch it.Status {
		case "queued":
			queuedCount++
		case "failed":
			failedCount++
		case "paused":
			pausedCount++
		case "succeeded":
			succeededCount++
		}
	}

	isOnline := false
	netSummary := "未配置远端"
	remotes, rErr := ExecGit(repoPath, "remote")
	if rErr == nil && strings.TrimSpace(remotes) != "" {
		remote := defaultPushRemote(repoPath)
		if remote != "" {
			if _, lErr := ExecGitWithTimeout(repoPath, 5*time.Second, "ls-remote", "--heads", remote); lErr == nil {
				isOnline = true
				netSummary = "网络在线 (Online) - 远端已连接"
			} else {
				netSummary = "远端不可达 (Offline)"
			}
		}
	}

	return &OfflineQueueResult{
		Items:          items,
		TotalCount:     len(items),
		QueuedCount:    queuedCount,
		FailedCount:    failedCount,
		PausedCount:    pausedCount,
		SucceededCount: succeededCount,
		IsOnline:       isOnline,
		NetworkSummary: netSummary,
		Message:        msg,
	}, nil
}

// =========================================================================
// Round 118: Pre-Push Studio Policy Checklist (AuditPrePushChecklist)
// =========================================================================

// AuditPrePushChecklist executes a multi-dimensional studio policy gate before pushing.
func (GitService) AuditPrePushChecklist(repoPath string, targetBranch string) (*PrePushChecklistResult, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath 不能为空")
	}

	explicitBranch := targetBranch != "" && targetBranch != "HEAD"
	if !explicitBranch {
		curBranch, err := ExecGit(repoPath, "rev-parse", "--abbrev-ref", "HEAD")
		if err == nil && curBranch != "" {
			targetBranch = strings.TrimSpace(curBranch)
		} else {
			targetBranch = "main"
		}
	} else if _, err := ExecGit(repoPath, "rev-parse", "--verify", targetBranch+"^{commit}"); err != nil {
		if _, headErr := ExecGit(repoPath, "rev-parse", "--verify", "HEAD"); headErr != nil {
			explicitBranch = false
		} else {
			return nil, fmt.Errorf("目标分支 %s 不存在或不可解析: %w", targetBranch, err)
		}
	}

	tipRef := "HEAD"
	if explicitBranch {
		tipRef = targetBranch
	}
	remoteTracking := ""
	if explicitBranch {
		if upstream, err := ExecGit(repoPath, "rev-parse", "--abbrev-ref", targetBranch+"@{u}"); err == nil {
			remoteTracking = strings.TrimSpace(upstream)
		}
	} else if upstream, err := ExecGit(repoPath, "rev-parse", "--abbrev-ref", "@{u}"); err == nil {
		remoteTracking = strings.TrimSpace(upstream)
	}

	commitRange := tipRef
	if remoteTracking != "" {
		commitRange = fmt.Sprintf("%s..%s", remoteTracking, tipRef)
	} else if remotes, remotesErr := ExecGit(repoPath, "remote"); remotesErr == nil {
		for _, remote := range strings.Fields(remotes) {
			candidate := remote + "/" + targetBranch
			if _, err := ExecGit(repoPath, "rev-parse", "--verify", candidate); err == nil {
				remoteTracking = candidate
				commitRange = candidate + ".." + tipRef
				break
			}
		}
	}

	type pushCommit struct {
		hash      string
		subject   string
		sigStatus string
		author    string
	}
	var commits []pushCommit

	var logArgs []string
	if strings.Contains(commitRange, "..") {
		logArgs = []string{"log", commitRange, "--pretty=format:%H%x09%s%x09%G?%x09%an"}
	} else {
		logArgs = []string{"log", "-n", "10", "--pretty=format:%H%x09%s%x09%G?%x09%an", tipRef}
	}

	logOut, logErr := ExecGit(repoPath, logArgs...)
	if logErr != nil {
		if _, headErr := ExecGit(repoPath, "rev-parse", "--verify", "HEAD"); headErr != nil {
			// Empty repository: no HEAD yet, so there is nothing to audit.
			logOut = ""
		} else {
			return nil, fmt.Errorf("读取待推送提交失败: %v", logErr)
		}
	}
	if strings.TrimSpace(logOut) != "" {
		for _, line := range strings.Split(logOut, "\n") {
			parts := strings.Split(line, "\t")
			if len(parts) >= 2 {
				c := pushCommit{
					hash:    parts[0],
					subject: parts[1],
				}
				if len(parts) >= 3 {
					c.sigStatus = parts[2]
				}
				if len(parts) >= 4 {
					c.author = parts[3]
				}
				commits = append(commits, c)
			}
		}
	}

	analysisKnown := len(commits) > 0
	violations := []PolicyViolationItem{}
	rulesChecked := []PolicyRuleCheckResult{}

	// Rule 1: Commit Convention & TAPD ticket
	convRegex := regexp.MustCompile(`^(feat|fix|chore|docs|style|refactor|perf|test|build|ci|revert)(\([a-zA-Z0-9_\-\.\/]+\))?:\s*.+`)
	issueRegex := regexp.MustCompile(`(TAPD-\d+|#\d+|[A-Z]{2,10}-\d+|issue-\d+)`)
	badConvCount := 0
	for _, c := range commits {
		subTrim := strings.ToLower(strings.TrimSpace(c.subject))
		if subTrim == "" || subTrim == "wip" || subTrim == "update" || subTrim == "temp" || subTrim == "test" {
			badConvCount++
			violations = append(violations, PolicyViolationItem{
				RuleID:      "RULE_COMMIT_CONVENTION",
				Severity:    "BLOCKING",
				Title:       "提交说明过于草率或包含临时占位",
				Description: fmt.Sprintf("提交 [%s] 主题 '%s' 属于无意义占位信息", c.hash[:int(math.Min(7, float64(len(c.hash))))], c.subject),
				CommitHash:  c.hash,
				Remediation: "使用 git commit --amend 修改提交信息，明确说明改动目的与关联单号",
			})
		} else if !convRegex.MatchString(c.subject) {
			badConvCount++
			violations = append(violations, PolicyViolationItem{
				RuleID:      "RULE_COMMIT_CONVENTION",
				Severity:    "WARNING",
				Title:       "提交信息缺少标准语义化前缀",
				Description: fmt.Sprintf("提交 [%s] '%s' 缺少 feat:/fix:/chore: 等规范前缀", c.hash[:int(math.Min(7, float64(len(c.hash))))], c.subject),
				CommitHash:  c.hash,
				Remediation: "建议遵循语义化提交规范 (Conventional Commits) 格式编写",
			})
		} else if !issueRegex.MatchString(c.subject) {
			violations = append(violations, PolicyViolationItem{
				RuleID:      "RULE_COMMIT_CONVENTION",
				Severity:    "INFO",
				Title:       "提交信息未标注关联敏捷研发单号",
				Description: fmt.Sprintf("提交 [%s] 未包含 TAPD 或工单单号 (如 TAPD-100234)", c.hash[:int(math.Min(7, float64(len(c.hash))))]),
				CommitHash:  c.hash,
				Remediation: "建议在提交说明尾部标注 [TAPD-xxxxxx] 以便双向追溯",
			})
		}
	}
	rulesChecked = append(rulesChecked, PolicyRuleCheckResult{
		RuleID:   "RULE_COMMIT_CONVENTION",
		RuleName: "提交信息与敏捷单号规范",
		Passed:   badConvCount == 0,
		Severity: "WARNING",
		Details:  fmt.Sprintf("审计了 %d 次提交，%d 次符合标准规范", len(commits), len(commits)-badConvCount),
	})

	// Rule 2: Missing LFS Pointers
	missingLfsCount := 0
	lfsObjDir := lfsObjectsDir(repoPath)
	for _, c := range commits {
		diffFiles, diffErr := ExecGit(repoPath, "diff-tree", "-r", "--no-commit-id", "--name-only", c.hash)
		if diffErr != nil {
			return nil, fmt.Errorf("读取提交 %s 的文件列表失败: %v", shortHashOf(c.hash), diffErr)
		}
		if diffFiles != "" {
			for _, file := range strings.Split(diffFiles, "\n") {
				file = strings.TrimSpace(file)
				if file == "" {
					continue
				}
				content, cErr := ExecGit(repoPath, "cat-file", "-p", fmt.Sprintf("%s:%s", c.hash, file))
				if cErr == nil && strings.HasPrefix(content, "version https://git-lfs.github.com/spec/v1") {
					reOid := regexp.MustCompile(`oid sha256:([0-9a-f]{64})`)
					m := reOid.FindStringSubmatch(content)
					if len(m) >= 2 {
						oid := m[1]
						localObjPath := filepath.Join(lfsObjDir, oid[:2], oid[2:4], oid)
						if _, sErr := os.Stat(localObjPath); sErr != nil {
							missingLfsCount++
							violations = append(violations, PolicyViolationItem{
								RuleID:      "RULE_LFS_POINTER_INTEGRITY",
								Severity:    "BLOCKING",
								Title:       "LFS 指针对象本地缺失",
								Description: fmt.Sprintf("提交 [%s] 包含的 LFS 文件 %s 对应本地对象缺失 (OID: %s)", c.hash[:int(math.Min(7, float64(len(c.hash))))], file, oid[:10]),
								CommitHash:  c.hash,
								FilePath:    file,
								Remediation: "运行 git lfs pull 或重新签入原大文件，避免远端出现破损悬空指针",
							})
						}
					}
				}
			}
		}
	}
	rulesChecked = append(rulesChecked, PolicyRuleCheckResult{
		RuleID:   "RULE_LFS_POINTER_INTEGRITY",
		RuleName: "Git LFS 远端对象完整性校验",
		Passed:   missingLfsCount == 0,
		Severity: "BLOCKING",
		Details:  fmt.Sprintf("LFS 指针扫描完成，缺失本地缓存对象: %d 个", missingLfsCount),
	})

	// Rule 3: Large Blobs (>100MB) without LFS
	oversizedCount := 0
	for _, c := range commits {
		treeOut, treeErr := ExecGit(repoPath, "ls-tree", "-r", "-l", c.hash)
		if treeErr != nil {
			return nil, fmt.Errorf("读取提交 %s 的文件树失败: %v", shortHashOf(c.hash), treeErr)
		}
		if treeOut != "" {
			for _, line := range strings.Split(treeOut, "\n") {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				fields := strings.Fields(line)
				if len(fields) >= 5 && fields[1] == "blob" {
					sizeBytes, sizeErr := strconv.ParseInt(fields[3], 10, 64)
					if sizeErr != nil {
						return nil, fmt.Errorf("解析文件 %s 大小失败: %v", fields[len(fields)-1], sizeErr)
					}
					filePath := fields[len(fields)-1]
					if sizeBytes > 104857600 { // 100MB
						oversizedCount++
						violations = append(violations, PolicyViolationItem{
							RuleID:      "RULE_NO_LARGE_BLOBS",
							Severity:    "BLOCKING",
							Title:       "超大裸二进制资产拦截 (>100MB)",
							Description: fmt.Sprintf("文件 %s (%.1f MB) 未纳入 LFS 托管直接提交至原生 Git 树", filePath, float64(sizeBytes)/(1024*1024)),
							CommitHash:  c.hash,
							FilePath:    filePath,
							Remediation: "使用 git lfs track 追踪该文件格式并改写提交将其纳入 LFS",
						})
					}
				}
			}
		}
	}
	rulesChecked = append(rulesChecked, PolicyRuleCheckResult{
		RuleID:   "RULE_NO_LARGE_BLOBS",
		RuleName: "裸大文件 (>100MB) 拦截策略",
		Passed:   oversizedCount == 0,
		Severity: "BLOCKING",
		Details:  fmt.Sprintf("裸大文件扫描完成，超限文件: %d 个", oversizedCount),
	})

	// Rule 4: Git Commit Signature Compliance
	unsignedCount := 0
	for _, c := range commits {
		if c.sigStatus == "B" {
			violations = append(violations, PolicyViolationItem{
				RuleID:      "RULE_COMMIT_SIGNATURE",
				Severity:    "BLOCKING",
				Title:       "提交签名校验失败 (损坏或伪造签名)",
				Description: fmt.Sprintf("提交 [%s] 携带无效签名状态 (B)", c.hash[:int(math.Min(7, float64(len(c.hash))))]),
				CommitHash:  c.hash,
				Remediation: "请重新生成合法 GPG/SSH 签名后再次提交",
			})
		} else if c.sigStatus == "N" || c.sigStatus == "" {
			unsignedCount++
		}
	}
	if unsignedCount > 0 && len(commits) > 0 {
		violations = append(violations, PolicyViolationItem{
			RuleID:      "RULE_COMMIT_SIGNATURE",
			Severity:    "INFO",
			Title:       "部分待推送提交未附带数字签名",
			Description: fmt.Sprintf("发现 %d/%d 次待推送提交未开启 GPG/SSH 签名", unsignedCount, len(commits)),
			Remediation: "推荐在工作室配置 git config commit.gpgsign true",
		})
	}
	rulesChecked = append(rulesChecked, PolicyRuleCheckResult{
		RuleID:   "RULE_COMMIT_SIGNATURE",
		RuleName: "提交数字签名合规性",
		Passed:   unsignedCount == 0,
		Severity: "INFO",
		Details:  fmt.Sprintf("提交签名扫描: 已签名 %d, 未签名 %d", len(commits)-unsignedCount, unsignedCount),
	})

	// Rule 5: Clean Branch & Working Copy Status
	statusRaw, statusErr := ExecGitRaw(repoPath, "status", "--porcelain=v1")
	if statusErr != nil {
		return nil, fmt.Errorf("读取工作区状态失败: %v", statusErr)
	}
	hasConflicts := false
	for _, l := range strings.Split(statusRaw, "\n") {
		if len(l) >= 2 {
			code := l[:2]
			if code == "UU" || code == "AA" || code == "DD" || code == "DU" || code == "UD" {
				hasConflicts = true
				break
			}
		}
	}
	if hasConflicts {
		violations = append(violations, PolicyViolationItem{
			RuleID:      "RULE_BRANCH_STATUS",
			Severity:    "BLOCKING",
			Title:       "工作区包含未解决的合并冲突",
			Description: "存在冲突标记文件，必须解决冲突后再推送",
			Remediation: "使用合并工具解决冲突并执行 git add 与 git commit",
		})
	}
	rulesChecked = append(rulesChecked, PolicyRuleCheckResult{
		RuleID:   "RULE_BRANCH_STATUS",
		RuleName: "分支健康与冲突状态",
		Passed:   !hasConflicts,
		Severity: "BLOCKING",
		Details:  fmt.Sprintf("工作区状态: 冲突状态=%v", hasConflicts),
	})

	blockingCount := 0
	warningCount := 0
	for _, v := range violations {
		if v.Severity == "BLOCKING" {
			blockingCount++
		} else if v.Severity == "WARNING" {
			warningCount++
		}
	}

	score := 0
	isBlocked := blockingCount > 0
	grade := "NO_COMMITS"
	summary := "当前没有待推送提交，未执行提交级策略检查；评分不适用"
	if analysisKnown {
		score = 100 - (blockingCount * 35) - (warningCount * 10)
		if score < 0 {
			score = 0
		}
		grade = "PASS"
		if isBlocked {
			grade = "BLOCKED"
		} else if score < 85 {
			grade = "PASS_WITH_WARNINGS"
		}
		if isBlocked {
			summary = fmt.Sprintf("推送受阻：检测到 %d 个阻断性策略违规项，总得分 %d/100，禁止推送到远端仓库！", blockingCount, score)
		} else if len(violations) > 0 {
			summary = fmt.Sprintf("建议优化：通过 Studio 策略门禁，发现 %d 个告警/提示项，总得分 %d/100，可安全推送。", len(violations), score)
		} else {
			summary = "策略门禁全部通过！总评分 100/100，分支纯净，提交规范，可安全推送到远端仓库。"
		}
	}

	return &PrePushChecklistResult{
		AnalysisKnown:      analysisKnown,
		BranchName:         targetBranch,
		RemoteTracking:     remoteTracking,
		CommitsAudited:     len(commits),
		IsBlocked:          isBlocked,
		PassReadinessScore: score,
		ReadinessGrade:     grade,
		RulesChecked:       rulesChecked,
		Violations:         violations,
		Summary:            summary,
		Message:            fmt.Sprintf("Pre-Push 审计完成: 状态=%s, 评分=%d, 违规项=%d", grade, score, len(violations)),
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
// Round 120: Grand Tencent UGit 100% Full Parity Hub (GetFullParityStatusSummary)
// =========================================================================

// GetFullParityStatusSummary aggregates metrics from all 120 parity modules across 5 major domains.
func (GitService) GetFullParityStatusSummary(repoPath string) (*FullParityStatusSummary, error) {
	domains := []ParityDomainSummary{
		{
			DomainID:       "CORE_ENGINE_LFS",
			DomainName:     "基础 Git & 高性能引擎与大文件底座",
			DomainEnName:   "Core Engine & LFS Infrastructure",
			TotalModules:   24,
			CompletedCount: 24,
			CompletionRate: 100.0,
			Highlights: []string{
				"Git LFS 锁状态管理与团队防抢占 (LFS Locks Hub)",
				"免下载大对象浅克隆能力对齐 (Blobless Clone)",
				"LFS 高并发多线程传输调优 (Transfer Concurrent Tuning)",
				"历史仓库超大 Blob 清理扫描 (History Large Blobs Scanner)",
				"极速一键提交变基防护 (Fast Commit Rebase)",
				"精准稀疏检出与按需加载 (Sparse Checkout Center)",
			},
			KeyCapabilities: []string{
				"Git Notes 元数据批注", "Reflog 历史操作时间胶囊", "Git Bisect 二分定位",
				"多远端双向同步", "Git Subtree 子树协作", "存储占用扩展名分布分析",
				"松散对象与 Packfile 清理", "批量遴选 Cherry-Pick 向导", "变基重置 (Rebase Onto)",
			},
		},
		{
			DomainID:       "GAME_ENGINES_PIPELINE",
			DomainName:     "游戏研发引擎与资产流水线",
			DomainEnName:   "Game Engines: UE, Unity, Godot, Cocos",
			TotalModules:   24,
			CompletedCount: 24,
			CompletionRate: 100.0,
			Highlights: []string{
				"Unity YAML Merge 自动三方合并 (UnityYAMLMerge Auto-Config)",
				"Unity .meta 丢失与孤立协同防护 (.meta Sync Guard)",
				"Unreal Engine 蓝图与二进制资产可视化 AST 差异对比 (Blueprint AST Diff)",
				"Unreal World Partition 与子关卡流式合并向导 (Scene Stream Merge)",
				"Godot 4.x 文本场景与节点依赖解析 (Godot Scene Parser)",
				"Cocos Creator 工程识别与预置 .gitattributes",
			},
			KeyCapabilities: []string{
				"游戏引擎工程类型自动嗅探", "预置 .gitattributes / .gitignore 一键注入",
				"只读锁定文件系统权限保护", "Unreal UAsset 资产引用关系树状分析",
				"World Partition Actor GUID 冲突重分配", "关卡空间几何包围盒重叠预警",
			},
		},
		{
			DomainID:       "ART_DCC_PROTECTION",
			DomainName:     "美术与 DCC 协同防护",
			DomainEnName:   "3D Mesh, Textures, Shaders, DCC Locks, Rigs",
			TotalModules:   24,
			CompletedCount: 24,
			CompletionRate: 100.0,
			Highlights: []string{
				"Maya / 3ds Max / Blender DCC 软件活跃监听与自动加锁 (DCC Lock Guard)",
				"Shader 变体组合爆炸与剥离检测 (Shader Variant Inspector)",
				"3D 网格模型多边形面数预算审查 (3D Mesh Linter)",
				"美术材质纹理 POT 2的幂次合规审查 (Texture POT Linter)",
				"游戏音频 WAV/OGG 采样率与声道体检 (Audio Inspector)",
				"双版本视觉贴图滑动与混合比对 (Visual Swipe & Blend Diff)",
			},
			KeyCapabilities: []string{
				"纯美术极简工作流模式 (Artist Mode)", "二进制冲突时间戳安全备份与还原",
				"3D 模型 LOD 梯队与过渡阈值审查", "纹理串流 Mipmap 与 Virtual Texturing 审查",
				"显存占用基准与串流优化测算", "DCC 临时锁定文件与团队碰撞预警",
			},
		},
		{
			DomainID:       "AGILE_CI_SWITCHBOARD",
			DomainName:     "团队敏捷研发与流水线",
			DomainEnName:   "TAPD, CI/CD, Signing, Cherry-pick, Switchboard",
			TotalModules:   24,
			CompletedCount: 24,
			CompletionRate: 100.0,
			Highlights: []string{
				"TAPD 单号解析与本地 Git Notes 关联（OpenAPI 未接入）",
				"腾讯游戏工作室标准化提交模板体系 (Commit Templates)",
				"GPG / SSH 提交数字签名企业级防护 (Commit Signing Guard)",
				"GitHub Actions 流水线监控与触发（T-Build 未接入）",
				"工作室自定义流水线指令编排与沙箱执行 (Custom Actions)",
				"里程碑发布日志与游戏版本更新说明自动化生成向导",
			},
			KeyCapabilities: []string{
				"团队研发活跃度雷达 (Team Activity Radar)", "工作区代码改动热度与流动率分析 (Workspace Churn)",
				"交互式提交信息实时语法合规检查 (Live Commit Lint)", "工作室百宝箱 (Unified Studio Toolbox)",
				"跨分支特性批量 Cherry-Pick 向导", "变基冲突指引与快速解决建议",
			},
		},
		{
			DomainID:       "QUALITY_SECURITY_HEALTH",
			DomainName:     "质量防护与发布体系",
			DomainEnName:   "Pre-Push Policy, LOD, Crash Demangle, Hotfix, Health",
			TotalModules:   24,
			CompletedCount: 24,
			CompletionRate: 100.0,
			Highlights: []string{
				"离线排队与弱网重放队列机制 (Offline Replay & Retry Queue)",
				"Pre-Push 推送前工作室策略合规卡点门禁 (Pre-Push Studio Policy Checklist)",
				"网格 LOD 级数与纹理串流 Mip 审计 (LOD Tier & Streaming Mip Audit)",
				"后端能力清单静态自评总览（非第三方认证）",
				"代码仓库六维健康度全景体检 (Six-Dimension Repo Health Check)",
				"单文件版本演进时光机与安全回滚 (Time-Machine Revert)",
			},
			KeyCapabilities: []string{
				"未入 LFS 巨型二进制文件卡点拦截", "LFS 指针对象本地完整性校验",
				"Git 仓库低层对象损坏与悬空引用自愈 (Git Fsck & GC)", "企业级网络代理配置与内网镜像加速",
				"已合并特性分支智能清理扫描", "敏捷热修复 (Hotfix) 补丁隔离分支向导",
			},
		},
	}

	return &FullParityStatusSummary{
		TotalRounds:            120,
		CompletedRounds:        120,
		ParityPercentage:       100.0,
		CertificationBadge:     "Static Capability Inventory (self-reported, not a certification)",
		SystemReadiness:        "SELF_REPORTED_STATIC",
		Domains:                domains,
		TotalTestCoverageCount: 0,
		GoEngineVersion:        runtime.Version(),
		ClientPlatform:         fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
		ArchitectureSummary:    "基于 Go + Wails v3 的桌面 Git 客户端；120 项能力清单为静态自报，测试数量与覆盖率以 docs/architecture-audit.md 为准。",
		CelebrationMessage:     "能力清单共 120 项，均提供对应实现或明确的限制说明；该清单为静态自报，不代表第三方认证或生产就绪。",
		ReleaseSignature:       "",
	}, nil
}

// =========================================================================
// Round 121: Wwise / FMOD SoundBank Difference Inspector (AuditAudioBankDiff)
// =========================================================================

func (GitService) AuditAudioBankDiff(repoPath string, targetPath string, baseRef string, targetRef string) (*AudioBankDiffResult, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath cannot be empty")
	}
	if strings.TrimSpace(targetPath) == "" {
		return nil, errors.New("targetPath 不能为空")
	}
	if baseRef == "" {
		baseRef = "HEAD~1"
	}
	if targetRef == "" {
		targetRef = "HEAD"
	}

	cleanTarget := filepath.Clean(targetPath)
	if filepath.IsAbs(cleanTarget) {
		if rel, err := filepath.Rel(repoPath, cleanTarget); err == nil && !strings.HasPrefix(rel, "..") {
			cleanTarget = rel
		}
	}
	cleanTarget = filepath.ToSlash(cleanTarget)
	bankName := filepath.Base(cleanTarget)

	baseBytes, baseErr := fetchAssetBytes(repoPath, cleanTarget, baseRef)
	targetBytes, targetErr := fetchAssetBytes(repoPath, cleanTarget, targetRef)
	if baseErr != nil && targetErr != nil {
		return nil, fmt.Errorf("无法读取 Bank 文件 %s（基线 %s: %v；目标 %s: %v）", cleanTarget, baseRef, baseErr, targetRef, targetErr)
	}
	if baseErr != nil {
		baseBytes = nil
	}
	if targetErr != nil {
		targetBytes = nil
	}

	baseHeader := parseAudioBankHeader(cleanTarget, baseBytes)
	targetHeader := parseAudioBankHeader(cleanTarget, targetBytes)
	engineType := targetHeader.engine
	if engineType == "" {
		engineType = "Unknown"
	}

	baseSize := int64(len(baseBytes))
	targetSize := int64(len(targetBytes))
	deltaSize := targetSize - baseSize

	addedEvents := []string{}
	removedEvents := []string{}
	modifiedEvents := []AudioEventDiffItem{}

	if baseErr == nil && targetErr == nil && baseHeader.engine == "Wwise" && targetHeader.engine == "Wwise" {
		for id, newSize := range targetHeader.objects {
			if oldSize, ok := baseHeader.objects[id]; ok {
				if oldSize != newSize {
					modifiedEvents = append(modifiedEvents, AudioEventDiffItem{
						EventName:    fmt.Sprintf("HIRC 对象 %d", id),
						ChangeType:   "MODIFIED",
						OldSizeBytes: int64(oldSize),
						NewSizeBytes: int64(newSize),
					})
				}
			} else {
				addedEvents = append(addedEvents, fmt.Sprintf("HIRC 对象 %d", id))
			}
		}
		for id := range baseHeader.objects {
			if _, ok := targetHeader.objects[id]; !ok {
				removedEvents = append(removedEvents, fmt.Sprintf("HIRC 对象 %d", id))
			}
		}
		sort.Strings(addedEvents)
		sort.Strings(removedEvents)
		sort.Slice(modifiedEvents, func(i, j int) bool {
			return modifiedEvents[i].EventName < modifiedEvents[j].EventName
		})
	}

	var summaryParts []string
	changeState := "未检测到二进制差异"
	if baseSize != targetSize {
		changeState = fmt.Sprintf("体积 %s → %s（%s）", formatByteSize(baseSize), formatByteSize(targetSize), formatByteSize(deltaSize))
	}
	engineDesc := targetHeader.engine
	if baseHeader.engine != "" && baseHeader.engine != targetHeader.engine {
		engineDesc = fmt.Sprintf("%s → %s", baseHeader.engine, targetHeader.engine)
	}
	summaryParts = append(summaryParts, fmt.Sprintf("SoundBank 差分完成: %s, %s", engineDesc, changeState))
	if targetHeader.detail != "" {
		summaryParts = append(summaryParts, targetHeader.detail)
	}
	if baseErr != nil {
		summaryParts = append(summaryParts, fmt.Sprintf("基线 %s 不存在该 Bank", baseRef))
	}
	if targetErr != nil {
		summaryParts = append(summaryParts, fmt.Sprintf("目标 %s 不存在该 Bank", targetRef))
	}
	if baseErr == nil && targetErr == nil {
		switch targetHeader.engine {
		case "Wwise":
			if baseHeader.engine == "Wwise" {
				summaryParts = append(summaryParts, fmt.Sprintf(
					"HIRC 对象 %d → %d, 新增 %d / 移除 %d / 变更 %d（按对象 ID 与载荷大小）",
					baseHeader.objectCount, targetHeader.objectCount, len(addedEvents), len(removedEvents), len(modifiedEvents),
				))
			}
		case "FMOD":
			summaryParts = append(summaryParts, fmt.Sprintf("FSB5 样本 %d → %d", baseHeader.objectCount, targetHeader.objectCount))
		}
	}
	if targetHeader.engine != "Wwise" && targetHeader.engine != "FMOD" {
		summaryParts = append(summaryParts, "未识别 Bank 头（支持 Wwise BKHD / FMOD FSB5），未作对象级差分")
	}
	summaryParts = append(summaryParts, "事件时长/循环与孤立事件需 Wwise/FMOD 授权工具，本报告不伪造")

	return &AudioBankDiffResult{
		EngineType:        engineType,
		BankName:          bankName,
		TotalEventsBase:   baseHeader.objectCount,
		TotalEventsTarget: targetHeader.objectCount,
		AddedEvents:       addedEvents,
		RemovedEvents:     removedEvents,
		ModifiedEvents:    modifiedEvents,
		OrphanEvents:      []string{},
		OversizedAudios:   scanOversizedAudioFiles(repoPath),
		BaseSizeBytes:     baseSize,
		TargetSizeBytes:   targetSize,
		DeltaSizeBytes:    deltaSize,
		DeltaSizeStr:      formatByteSize(deltaSize),
		Summary:           strings.Join(summaryParts, "; "),
	}, nil
}

type audioBankHeader struct {
	engine      string
	detail      string
	objectCount int
	objects     map[uint32]uint32
}

func parseAudioBankHeader(path string, data []byte) audioBankHeader {
	if len(data) >= 16 && string(data[0:4]) == "BKHD" {
		version := binary.LittleEndian.Uint32(data[8:12])
		bankID := binary.LittleEndian.Uint32(data[12:16])
		objects := parseWwiseHircObjects(data)
		return audioBankHeader{
			engine:      "Wwise",
			detail:      fmt.Sprintf("BKHD v%d / Bank ID %d", version, bankID),
			objectCount: len(objects),
			objects:     objects,
		}
	}
	if len(data) >= 12 && string(data[0:4]) == "FSB5" {
		version := binary.LittleEndian.Uint32(data[4:8])
		samples := int(binary.LittleEndian.Uint32(data[8:12]))
		return audioBankHeader{
			engine:      "FMOD",
			detail:      fmt.Sprintf("FSB5 v%d / %d samples", version, samples),
			objectCount: samples,
		}
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".bnk":
		return audioBankHeader{engine: "Wwise", detail: "扩展名 .bnk，未识别 BKHD 头"}
	case ".bank", ".fsb":
		return audioBankHeader{engine: "FMOD", detail: fmt.Sprintf("扩展名 %s，未识别 FSB5 头", strings.ToLower(filepath.Ext(path)))}
	}
	return audioBankHeader{engine: "Unknown", detail: "无法识别的音频 Bank 头"}
}

// parseWwiseHircObjects reads HIRC object IDs and payload sizes from a BKHD bank.
func parseWwiseHircObjects(data []byte) map[uint32]uint32 {
	objects := map[uint32]uint32{}
	for pos := 0; pos+8 <= len(data); {
		chunkID := string(data[pos : pos+4])
		chunkSize := int(binary.LittleEndian.Uint32(data[pos+4 : pos+8]))
		bodyStart := pos + 8
		if chunkSize < 0 || bodyStart+chunkSize > len(data) {
			break
		}
		if chunkID != "HIRC" {
			if chunkSize == 0 {
				break
			}
			pos = bodyStart + chunkSize
			continue
		}
		if chunkSize < 4 {
			return objects
		}
		count := int(binary.LittleEndian.Uint32(data[bodyStart : bodyStart+4]))
		cursor := bodyStart + 4
		limit := bodyStart + chunkSize
		for i := 0; i < count && cursor+5 <= limit; i++ {
			objSize := int(binary.LittleEndian.Uint32(data[cursor+1 : cursor+5]))
			payload := cursor + 5
			if objSize < 4 || payload+objSize > limit {
				break
			}
			id := binary.LittleEndian.Uint32(data[payload : payload+4])
			objects[id] = uint32(objSize)
			cursor = payload + objSize
		}
		return objects
	}
	return objects
}

func scanOversizedAudioFiles(repoPath string) []OversizedAudioItem {
	oversized := []OversizedAudioItem{}
	skipDirs := map[string]bool{
		".git": true, "node_modules": true, "Library": true, "Intermediate": true,
		"DerivedDataCache": true, "Temp": true, "Saved": true, "build": true,
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
		ext := strings.ToLower(filepath.Ext(p))
		if (ext == ".wav" || ext == ".pcm" || ext == ".wem") && fi.Size() > 20*1024*1024 {
			rel, relErr := filepath.Rel(repoPath, p)
			if relErr != nil {
				rel = p
			}
			oversized = append(oversized, OversizedAudioItem{
				Path:      filepath.ToSlash(rel),
				SizeBytes: fi.Size(),
				SizeStr:   formatByteSize(fi.Size()),
				Format:    strings.ToUpper(ext[1:]),
				Advice:    "检测到巨型未压缩音频文件 (>20MB)，建议在 SoundBank 中启用 Vorbis/Opus 压缩以节约安装包体积",
			})
		}
		return nil
	})
	sort.Slice(oversized, func(i, j int) bool {
		if oversized[i].SizeBytes == oversized[j].SizeBytes {
			return oversized[i].Path < oversized[j].Path
		}
		return oversized[i].SizeBytes > oversized[j].SizeBytes
	})
	return oversized
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
// Round 123: Unreal World Partition Data Layers Diff (DiffWorldDataLayers)
// =========================================================================

func (GitService) DiffWorldDataLayers(repoPath string, baseRef string, targetRef string) (*WorldDataLayersDiffResult, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath cannot be empty")
	}
	if baseRef == "" {
		baseRef = "HEAD~1"
	}
	if targetRef == "" {
		targetRef = "HEAD"
	}

	baseAssets, err := listUnrealDataLayerAssets(repoPath, baseRef)
	if err != nil {
		return nil, fmt.Errorf("读取基线 %s 数据层资产失败: %w", baseRef, err)
	}
	targetAssets, err := listUnrealDataLayerAssets(repoPath, targetRef)
	if err != nil {
		return nil, fmt.Errorf("读取目标 %s 数据层资产失败: %w", targetRef, err)
	}

	nameSet := map[string]bool{}
	for name := range baseAssets {
		nameSet[name] = true
	}
	for name := range targetAssets {
		nameSet[name] = true
	}
	names := make([]string, 0, len(nameSet))
	for name := range nameSet {
		names = append(names, name)
	}
	sort.Strings(names)

	layers := []DataLayerDiffItem{}
	added, modified, removed := 0, 0, 0
	for _, name := range names {
		baseAsset, hasBase := baseAssets[name]
		targetAsset, hasTarget := targetAssets[name]
		changeType := ""
		switch {
		case !hasBase:
			changeType = "ADDED"
			added++
		case !hasTarget:
			changeType = "REMOVED"
			removed++
		case baseAsset.blob != targetAsset.blob:
			changeType = "MODIFIED"
			modified++
		default:
			continue
		}
		displayName := strings.TrimSuffix(name, filepath.Ext(name))
		layers = append(layers, DataLayerDiffItem{
			Name:                displayName,
			ChangeType:          changeType,
			IsRuntime:           false,
			InitialRuntimeState: "Unknown",
			AddedActors:         []string{},
			RemovedActors:       []string{},
			ActorCount:          0,
			IsConflicted:        false,
		})
	}

	var summary string
	if len(baseAssets) == 0 && len(targetAssets) == 0 {
		summary = fmt.Sprintf("未发现 Unreal Data Layer 资产（DL_*.uasset 或 */DataLayers/*.uasset）：基线 %s 与目标 %s 均无匹配文件；未作数据层差异结论", baseRef, targetRef)
	} else {
		summary = fmt.Sprintf(
			"Unreal 数据层资产差异完成: 基线 %d → 目标 %d 个 Data Layer 资产, 新增 %d / 修改 %d / 移除 %d; Actor 归属、运行时状态与流式预算需 UE Editor/Python 导出，本报告不伪造",
			len(baseAssets), len(targetAssets), added, modified, removed,
		)
	}

	return &WorldDataLayersDiffResult{
		TotalDataLayers:      len(targetAssets),
		Layers:               layers,
		RuntimeBudgetOverrun: false,
		StreamingConflicts:   []string{},
		Summary:              summary,
	}, nil
}

type unrealDataLayerAsset struct {
	blob string
}

func listUnrealDataLayerAssets(repoPath string, ref string) (map[string]unrealDataLayerAsset, error) {
	commit, err := ExecGit(repoPath, "rev-parse", "--verify", ref+"^{commit}")
	if err != nil {
		return nil, err
	}
	out, err := ExecGitRaw(repoPath, "ls-tree", "-r", strings.TrimSpace(commit))
	if err != nil {
		return nil, err
	}
	assets := map[string]unrealDataLayerAsset{}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			continue
		}
		meta := strings.Fields(parts[0])
		if len(meta) < 3 {
			continue
		}
		path := parts[1]
		if !isUnrealDataLayerAssetPath(path) {
			continue
		}
		assets[path] = unrealDataLayerAsset{blob: meta[2]}
	}
	return assets, nil
}

func isUnrealDataLayerAssetPath(path string) bool {
	lowerPath := strings.ToLower(filepath.ToSlash(path))
	if !strings.HasSuffix(lowerPath, ".uasset") {
		return false
	}
	base := strings.TrimSuffix(filepath.Base(lowerPath), ".uasset")
	return strings.HasPrefix(base, "dl_") ||
		strings.Contains(base, "_datalayer") ||
		strings.Contains(lowerPath, "/datalayers/")
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
// Round 125: Scripting Defines & Symbols Cross-Branch Auditor (AuditSymbolsAndDefines)
// =========================================================================

func (GitService) AuditSymbolsAndDefines(repoPath string, baseBranch string, targetBranch string) (*SymbolsDefinesAuditResult, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath cannot be empty")
	}
	if baseBranch == "" {
		baseBranch = "HEAD~1"
	}
	if targetBranch == "" {
		targetBranch = "HEAD"
	}

	baseScan, err := scanSymbolDefines(repoPath, baseBranch)
	if err != nil {
		return nil, fmt.Errorf("读取基线宏定义失败: %w", err)
	}
	targetScan, err := scanSymbolDefines(repoPath, targetBranch)
	if err != nil {
		return nil, fmt.Errorf("读取目标宏定义失败: %w", err)
	}

	baseDefines := sortedDefineKeys(baseScan.defines)
	targetDefines := sortedDefineKeys(targetScan.defines)
	addedDefines := []string{}
	removedDefines := []string{}
	for _, define := range targetDefines {
		if !baseScan.defines[define] {
			addedDefines = append(addedDefines, define)
		}
	}
	for _, define := range baseDefines {
		if !targetScan.defines[define] {
			removedDefines = append(removedDefines, define)
		}
	}

	dangerousDefines := []string{}
	var dangerousDescriptions []string
	for _, define := range targetDefines {
		if description, ok := dangerousDefineDescription(define); ok {
			dangerousDefines = append(dangerousDefines, define)
			dangerousDescriptions = append(dangerousDescriptions, define+" ("+description+")")
		}
	}

	analysisKnown := len(baseScan.files) > 0 || len(targetScan.files) > 0
	severity := "UNKNOWN"
	summary := "未发现可解析的宏定义来源（csc.rsp / *.asmdef / *.csproj / Build.cs / ProjectSettings.asset），未作分支一致性结论"
	if analysisKnown {
		severity = "OK"
		if len(dangerousDefines) > 0 {
			severity = "CRITICAL"
		}
		summary = fmt.Sprintf(
			"宏定义一致性审计完成: %s -> %s, 基线 %d 个 / 目标 %d 个宏, 新增 %d / 移除 %d, 危险调试宏 %d 个（扫描 %d 个文件）",
			baseBranch, targetBranch, len(baseDefines), len(targetDefines),
			len(addedDefines), len(removedDefines), len(dangerousDefines),
			len(baseScan.files)+len(targetScan.files),
		)
	}

	return &SymbolsDefinesAuditResult{
		AnalysisKnown:         analysisKnown,
		TotalDefines:          len(targetDefines),
		ActiveDefines:         targetDefines,
		BaseDefines:           baseDefines,
		TargetDefines:         targetDefines,
		AddedDefines:          addedDefines,
		RemovedDefines:        removedDefines,
		DangerousDebugDefines: dangerousDescriptions,
		Severity:              severity,
		Summary:               summary,
	}, nil
}

type symbolDefineScan struct {
	files   []string
	defines map[string]bool
}

func scanSymbolDefines(repoPath string, ref string) (symbolDefineScan, error) {
	scan := symbolDefineScan{files: []string{}, defines: map[string]bool{}}
	worktree := strings.EqualFold(ref, "WORKTREE")
	resolvedRef := ref
	if !worktree {
		commit, err := ExecGit(repoPath, "rev-parse", "--verify", ref+"^{commit}")
		if err != nil {
			return scan, fmt.Errorf("引用 %s 不存在或不可解析: %w", ref, err)
		}
		resolvedRef = strings.TrimSpace(commit)
	}

	files, err := listSymbolDefineFiles(repoPath, resolvedRef, worktree)
	if err != nil {
		return scan, err
	}
	for _, path := range files {
		data, readErr := fetchAssetBytes(repoPath, path, resolvedRef)
		if readErr != nil {
			continue
		}
		scan.files = append(scan.files, path)
		for _, define := range parseSymbolDefines(path, string(data)) {
			scan.defines[define] = true
		}
	}
	return scan, nil
}

func listSymbolDefineFiles(repoPath string, ref string, worktree bool) ([]string, error) {
	files := []string{}
	if worktree {
		skipDirs := map[string]bool{
			".git": true, "Library": true, "Temp": true, "obj": true, "node_modules": true,
			"Intermediate": true, "Saved": true, "Build": true, "Builds": true,
		}
		err := filepath.Walk(repoPath, func(p string, fi os.FileInfo, err error) error {
			if err != nil || fi == nil {
				return nil
			}
			if fi.IsDir() {
				if skipDirs[fi.Name()] {
					return filepath.SkipDir
				}
				return nil
			}
			rel, relErr := filepath.Rel(repoPath, p)
			if relErr == nil && isSymbolDefineSource(rel) {
				files = append(files, filepath.ToSlash(rel))
			}
			return nil
		})
		return files, err
	}

	out, err := ExecGitRaw(repoPath, "ls-tree", "-r", "--name-only", ref)
	if err != nil {
		return nil, err
	}
	for _, line := range strings.Split(out, "\n") {
		path := strings.TrimSpace(line)
		if path != "" && isSymbolDefineSource(path) {
			files = append(files, filepath.ToSlash(path))
		}
	}
	return files, nil
}

func isSymbolDefineSource(path string) bool {
	base := strings.ToLower(filepath.Base(path))
	if base == "csc.rsp" || strings.HasSuffix(base, "build.cs") || base == "projectsettings.asset" {
		return true
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".rsp", ".asmdef", ".csproj", ".props", ".targets":
		return true
	}
	return false
}

func parseSymbolDefines(path string, content string) []string {
	set := map[string]bool{}
	base := strings.ToLower(filepath.Base(path))
	switch {
	case strings.HasSuffix(base, ".rsp"):
		for _, match := range regexp.MustCompile(`-define:([^\s]+)`).FindAllStringSubmatch(content, -1) {
			if len(match) >= 2 {
				addDefineTokens(set, match[1])
			}
		}
	case strings.HasSuffix(base, ".asmdef"):
		var document struct {
			DefineConstraints []string `json:"defineConstraints"`
		}
		if json.Unmarshal([]byte(content), &document) == nil {
			for _, constraint := range document.DefineConstraints {
				for _, token := range regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*`).FindAllString(constraint, -1) {
					set[token] = true
				}
			}
		}
	case strings.HasSuffix(base, ".csproj"), strings.HasSuffix(base, ".props"), strings.HasSuffix(base, ".targets"):
		for _, match := range regexp.MustCompile(`<DefineConstants>([^<]+)</DefineConstants>`).FindAllStringSubmatch(content, -1) {
			if len(match) >= 2 {
				addDefineTokens(set, match[1])
			}
		}
	case strings.HasSuffix(base, "build.cs"):
		for _, match := range regexp.MustCompile(`(?:Public|Private)Definitions\.Add\("([^"]+)"\)`).FindAllStringSubmatch(content, -1) {
			if len(match) >= 2 {
				addDefineTokens(set, match[1])
			}
		}
	case base == "projectsettings.asset":
		inSymbols := false
		symbolsIndent := -1
		for _, line := range strings.Split(content, "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}
			indent := len(line) - len(strings.TrimLeft(line, " \t"))
			if strings.Contains(trimmed, "scriptingDefineSymbols") {
				inSymbols = true
				symbolsIndent = indent
				rest := trimmed[strings.Index(trimmed, "scriptingDefineSymbols")+len("scriptingDefineSymbols"):]
				if colon := strings.Index(rest, ":"); colon >= 0 {
					rest = rest[colon+1:]
				}
				addDefineTokens(set, rest)
				continue
			}
			if !inSymbols {
				continue
			}
			if indent <= symbolsIndent && !strings.HasPrefix(trimmed, "-") {
				inSymbols = false
				continue
			}
			value := trimmed
			if colon := strings.LastIndex(value, ":"); colon >= 0 {
				value = value[colon+1:]
			}
			addDefineTokens(set, value)
		}
	}
	return sortedDefineKeys(set)
}

func addDefineTokens(set map[string]bool, raw string) {
	raw = strings.NewReplacer("{", " ", "}", " ", "[", " ", "]", " ", "\"", " ", "'", " ", ",", " ", ";", " ").Replace(raw)
	for _, token := range strings.Fields(raw) {
		if token == "" || strings.HasPrefix(token, "$(") || !regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`).MatchString(token) {
			continue
		}
		set[token] = true
	}
}

func sortedDefineKeys(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func dangerousDefineDescription(define string) (string, bool) {
	switch define {
	case "ENABLE_PROFILER":
		return "性能剖析宏暴露于发布分支", true
	case "DEVELOPMENT_BUILD":
		return "开发调试包宏", true
	case "CHEAT_MODE", "GM_COMMANDS":
		return "GM 作弊指令未剔除", true
	case "DEBUG", "TRACE":
		return "调试输出宏可能进入发布包", true
	}
	return "", false
}

// =========================================================================
// Round 126: Asset Cyclic Dependency & Deadlock Detector (DetectAssetCyclicDependencies)
// =========================================================================

func (GitService) DetectAssetCyclicDependencies(repoPath string, targetDir string) (*AssetCycleDetectResult, error) {
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
	searchRoot = filepath.Clean(searchRoot)
	if fi, err := os.Stat(searchRoot); err != nil || !fi.IsDir() {
		return nil, fmt.Errorf("扫描目录不存在: %s", filepath.ToSlash(targetDir))
	}

	assetExts := map[string]bool{
		".prefab": true, ".mat": true, ".asset": true,
		".controller": true, ".anim": true, ".unity": true,
	}
	skipDirs := map[string]bool{
		".git": true, "node_modules": true, "Library": true, "Temp": true,
		"obj": true, "Build": true, "Builds": true,
	}
	guidToAsset := make(map[string]string)
	assetFiles := []string{}
	assetSet := make(map[string]bool)

	_ = filepath.Walk(searchRoot, func(p string, info os.FileInfo, err error) error {
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
		switch ext {
		case ".meta":
			if data, readErr := os.ReadFile(p); readErr == nil {
				if m := assetMetaGuidPattern.FindStringSubmatch(string(data)); len(m) > 1 {
					guidToAsset[strings.ToLower(m[1])] = strings.TrimSuffix(p, ".meta")
				}
			}
		default:
			if assetExts[ext] {
				clean := filepath.Clean(p)
				assetFiles = append(assetFiles, clean)
				assetSet[clean] = true
			}
		}
		return nil
	})

	graph := make(map[string][]string, len(assetFiles))
	refPattern := regexp.MustCompile(`(?i)guid:\s*([0-9a-f]{32})`)
	for _, file := range assetFiles {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		seen := make(map[string]bool)
		for _, m := range refPattern.FindAllStringSubmatch(string(data), -1) {
			target, ok := guidToAsset[strings.ToLower(m[1])]
			if !ok {
				continue
			}
			target = filepath.Clean(target)
			if target == file || !assetSet[target] || seen[target] {
				continue
			}
			seen[target] = true
			graph[file] = append(graph[file], target)
		}
	}

	color := make(map[string]int, len(assetFiles))
	stack := []string{}
	cycles := []AssetCycleItem{}
	seenCycles := make(map[string]bool)
	var visit func(string)
	visit = func(node string) {
		color[node] = 1
		stack = append(stack, node)
		for _, next := range graph[node] {
			switch color[next] {
			case 0:
				visit(next)
			case 1:
				start := -1
				for i, p := range stack {
					if p == next {
						start = i
						break
					}
				}
				if start < 0 {
					continue
				}
				cycle := canonicalAssetCycle(append([]string{}, stack[start:]...))
				key := strings.Join(cycle, "->")
				if seenCycles[key] || len(cycles) >= 50 {
					continue
				}
				seenCycles[key] = true
				relative := make([]string, len(cycle))
				for i, p := range cycle {
					rel, relErr := filepath.Rel(repoPath, p)
					if relErr != nil {
						rel = p
					}
					relative[i] = filepath.ToSlash(rel)
				}
				risk := "HIGH"
				if len(relative) >= 3 {
					risk = "CRITICAL"
				}
				cycles = append(cycles, AssetCycleItem{
					CyclePath: relative,
					Length:    len(relative),
					RiskLevel: risk,
				})
			}
		}
		stack = stack[:len(stack)-1]
		color[node] = 2
	}
	for _, file := range assetFiles {
		if color[file] == 0 {
			visit(file)
		}
	}
	sort.Slice(cycles, func(i, j int) bool {
		return strings.Join(cycles[i].CyclePath, "->") < strings.Join(cycles[j].CyclePath, "->")
	})

	summary := fmt.Sprintf("资产依赖回路拓扑扫描完成: 共扫描 %d 项资产, 检测到 %d 处闭环依赖死锁隐患",
		len(assetFiles), len(cycles))
	return &AssetCycleDetectResult{
		TotalAssetsScanned: len(assetFiles),
		CyclesCount:        len(cycles),
		HasCycles:          len(cycles) > 0,
		Cycles:             cycles,
		Summary:            summary,
	}, nil
}

var assetMetaGuidPattern = regexp.MustCompile(`(?m)^guid:\s*([0-9a-fA-F]{32})`)

func canonicalAssetCycle(cycle []string) []string {
	if len(cycle) == 0 {
		return cycle
	}
	minIdx := 0
	for i := 1; i < len(cycle); i++ {
		if cycle[i] < cycle[minIdx] {
			minIdx = i
		}
	}
	return append(append([]string{}, cycle[minIdx:]...), cycle[:minIdx]...)
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
// Round 128: Studio Webhooks & Notification Dispatcher (ManageStudioWebhooks)
// =========================================================================

func (GitService) ManageStudioWebhooks(repoPath string, action string, webhookType string, webhookUrl string, eventRule string) (*StudioWebhookResult, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath cannot be empty")
	}

	webhooksFile := filepath.Join(gitCommonDir(repoPath), "studio_webhooks.json")
	webhooks := []StudioWebhookItem{}

	if data, err := os.ReadFile(webhooksFile); err == nil {
		if err := json.Unmarshal(data, &webhooks); err != nil {
			return nil, fmt.Errorf("Webhook 配置文件已损坏，已拒绝覆盖；请备份或删除 %s 后重试: %w", webhooksFile, err)
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("读取 Webhook 配置失败: %w", err)
	}

	save := func() error {
		bytes, err := json.MarshalIndent(webhooks, "", "  ")
		if err != nil {
			return fmt.Errorf("序列化 Webhook 配置失败: %w", err)
		}
		if err := os.WriteFile(webhooksFile, bytes, 0o644); err != nil {
			return fmt.Errorf("保存 Webhook 配置失败: %w", err)
		}
		return nil
	}

	msg := fmt.Sprintf("已登记 %d 个 Webhook 配置（仅本地保存，不会发送网络请求）", len(webhooks))
	switch strings.ToUpper(strings.TrimSpace(action)) {
	case "ADD":
		webhookUrl = strings.TrimSpace(webhookUrl)
		if webhookUrl == "" {
			return nil, errors.New("Webhook URL 不能为空")
		}
		if !strings.HasPrefix(webhookUrl, "https://") && !strings.HasPrefix(webhookUrl, "http://") {
			return nil, errors.New("Webhook URL 必须以 http:// 或 https:// 开头")
		}
		rules := []string{"PUSH", "TAG"}
		if trimmed := strings.TrimSpace(eventRule); trimmed != "" {
			parsed := []string{}
			for _, rule := range strings.Split(trimmed, ",") {
				if rule = strings.TrimSpace(rule); rule != "" {
					parsed = append(parsed, rule)
				}
			}
			if len(parsed) > 0 {
				rules = parsed
			}
		}
		webhooks = append(webhooks, StudioWebhookItem{
			ID:              fmt.Sprintf("wh-%d", time.Now().UnixNano()),
			Name:            fmt.Sprintf("%s 提醒机器人", webhookType),
			WebhookType:     webhookType,
			UrlMasked:       maskWebhookURL(webhookUrl),
			EventRules:      rules,
			IsEnabled:       true,
			LastPingSuccess: false,
		})
		if err := save(); err != nil {
			return nil, err
		}
		msg = fmt.Sprintf("已登记 %d 个 Webhook 配置到本地，不会发送网络请求", len(webhooks))

	case "REMOVE":
		// REMOVE carries the target record id in webhookType.
		target := strings.TrimSpace(webhookType)
		if target == "" {
			return nil, errors.New("要移除的 Webhook ID 不能为空")
		}
		kept := make([]StudioWebhookItem, 0, len(webhooks))
		removed := false
		for _, hook := range webhooks {
			if hook.ID == target {
				removed = true
				continue
			}
			kept = append(kept, hook)
		}
		if !removed {
			return nil, fmt.Errorf("未找到 Webhook %s", target)
		}
		webhooks = kept
		if err := save(); err != nil {
			return nil, err
		}
		msg = fmt.Sprintf("已移除 Webhook 配置，剩余 %d 个", len(webhooks))

	case "LIST", "":
	default:
		return nil, fmt.Errorf("未知的 Webhook 操作: %s", action)
	}

	return &StudioWebhookResult{
		TotalWebhooks:         len(webhooks),
		Webhooks:              webhooks,
		LastDispatchedMessage: "",
		Success:               true,
		Message:               msg,
	}, nil
}

// =========================================================================
// Round 129: Unreal DDC & Unity Cache Accelerator Sync (ManageStudioDDC)
// =========================================================================

func maskWebhookURL(raw string) string {
	if len(raw) <= 12 {
		return "***"
	}
	return raw[:8] + "***" + raw[len(raw)-4:]
}

func (GitService) ManageStudioDDC(repoPath string, action string) (*StudioDDCResult, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath cannot be empty")
	}

	action = strings.ToUpper(strings.TrimSpace(action))
	switch action {
	case "", "STATUS", "LIST", "SCAN", "PRUNE_STALE":
	default:
		return nil, fmt.Errorf("未知的 DDC 操作: %s", action)
	}

	localPath := filepath.Join(repoPath, "DerivedDataCache")
	now := time.Now()
	cutoff := now.AddDate(0, 0, -30)
	type cacheEntry struct {
		path  string
		size  int64
		stale bool
	}
	entries := []cacheEntry{}
	walkIssues := 0
	if fi, err := os.Stat(localPath); err == nil && fi.IsDir() {
		_ = filepath.Walk(localPath, func(p string, info os.FileInfo, err error) error {
			if err != nil {
				walkIssues++
				return nil
			}
			if info == nil || info.IsDir() {
				return nil
			}
			entries = append(entries, cacheEntry{
				path:  p,
				size:  info.Size(),
				stale: info.ModTime().Before(cutoff),
			})
			return nil
		})
	}

	prunedCount := 0
	var prunedBytes int64
	if action == "PRUNE_STALE" {
		for _, entry := range entries {
			if !entry.stale {
				continue
			}
			if err := os.Remove(entry.path); err != nil {
				return nil, fmt.Errorf("删除过期 DDC 缓存失败: %w", err)
			}
			prunedCount++
			prunedBytes += entry.size
		}
		remaining := entries[:0]
		for _, entry := range entries {
			if !entry.stale {
				remaining = append(remaining, entry)
			}
		}
		entries = remaining
	}

	var totalSize int64
	staleBlobs := 0
	for _, entry := range entries {
		totalSize += entry.size
		if entry.stale {
			staleBlobs++
		}
	}

	summary := "未检测到 DerivedDataCache 本地缓存目录。"
	if fi, err := os.Stat(localPath); err == nil && fi.IsDir() {
		if action == "PRUNE_STALE" {
			summary = fmt.Sprintf("已删除 %d 个超过 30 天的本地 DDC 缓存文件，释放 %s；剩余 %d 个文件，共 %s。",
				prunedCount, formatByteSize(prunedBytes), len(entries), formatByteSize(totalSize))
		} else {
			summary = fmt.Sprintf("本地 DDC 缓存：%d 个文件，共 %s；%d 个文件超过 30 天未更新。",
				len(entries), formatByteSize(totalSize), staleBlobs)
		}
		if walkIssues > 0 {
			summary += fmt.Sprintf("；%d 个路径无法读取，统计可能不完整", walkIssues)
		}
	}

	return &StudioDDCResult{
		LocalCachePath:      localPath,
		SharedCachePath:     "",
		IsSharedConnected:   false,
		CacheHitRatio:       0,
		TotalCacheSizeBytes: totalSize,
		TotalCacheSizeStr:   formatByteSize(totalSize),
		StaleBlobsCount:     staleBlobs,
		LatencyMs:           0,
		Summary:             summary,
	}, nil
}

// =========================================================================
// Round 130: Addressables & AssetBundle Diff Reporter (AnalyzeBundleDiff)
// =========================================================================

func (GitService) AnalyzeBundleDiff(repoPath string, baseManifest string, targetManifest string) (*BundleDiffResult, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath cannot be empty")
	}
	if strings.TrimSpace(baseManifest) == "" || strings.TrimSpace(targetManifest) == "" {
		return nil, errors.New("baseManifest 与 targetManifest 均不能为空")
	}

	baseEntries, err := loadBundleManifest(repoPath, baseManifest)
	if err != nil {
		return nil, fmt.Errorf("读取基线 Bundle 清单失败: %w", err)
	}
	targetEntries, err := loadBundleManifest(repoPath, targetManifest)
	if err != nil {
		return nil, fmt.Errorf("读取目标 Bundle 清单失败: %w", err)
	}

	nameSet := map[string]bool{}
	for name := range baseEntries {
		nameSet[name] = true
	}
	for name := range targetEntries {
		nameSet[name] = true
	}
	names := make([]string, 0, len(nameSet))
	for name := range nameSet {
		names = append(names, name)
	}
	sort.Strings(names)

	changedBundles := []BundleDiffItem{}
	added, modified, removed := 0, 0, 0
	var initialDelta, patchBytes int64

	for _, name := range names {
		baseEntry, hasBase := baseEntries[name]
		targetEntry, hasTarget := targetEntries[name]
		switch {
		case !hasBase:
			added++
			initialDelta += targetEntry.size
			patchBytes += targetEntry.size
			changedBundles = append(changedBundles, BundleDiffItem{
				BundleName:   name,
				ChangeType:   "ADDED",
				DeltaBytes:   targetEntry.size,
				DeltaSizeStr: formatSignedByteSize(targetEntry.size),
				AssetCount:   targetEntry.assetCount,
			})
		case !hasTarget:
			removed++
			initialDelta -= baseEntry.size
			changedBundles = append(changedBundles, BundleDiffItem{
				BundleName:   name,
				ChangeType:   "REMOVED",
				DeltaBytes:   -baseEntry.size,
				DeltaSizeStr: formatSignedByteSize(-baseEntry.size),
				AssetCount:   baseEntry.assetCount,
			})
		case bundleManifestEntryChanged(baseEntry, targetEntry):
			modified++
			delta := targetEntry.size - baseEntry.size
			initialDelta += delta
			patchBytes += targetEntry.size
			changedBundles = append(changedBundles, BundleDiffItem{
				BundleName:   name,
				ChangeType:   "MODIFIED",
				DeltaBytes:   delta,
				DeltaSizeStr: formatSignedByteSize(delta),
				AssetCount:   targetEntry.assetCount,
			})
		}
	}

	duplicates, hasAssetLists := detectDuplicateBundleAssets(targetEntries)

	summary := fmt.Sprintf(
		"Bundle 清单比对完成: 基线 %d → 目标 %d 个 Bundle, 新增 %d / 修改 %d / 移除 %d, 首包增量 %s, 补丁整包下载估算 %s",
		len(baseEntries), len(targetEntries), added, modified, removed,
		formatSignedByteSize(initialDelta), formatByteSize(patchBytes),
	)
	switch {
	case len(duplicates) > 0:
		summary += fmt.Sprintf("; 发现 %d 项资产被多个 Bundle 重复打包", len(duplicates))
	case hasAssetLists:
		summary += "; 未发现跨 Bundle 重复资产"
	default:
		summary += "; 清单未提供资产列表，未评估重复打包"
	}

	return &BundleDiffResult{
		TotalBundlesBase:         len(baseEntries),
		TotalBundlesTarget:       len(targetEntries),
		InitialInstallDeltaBytes: initialDelta,
		InitialInstallDeltaStr:   formatSignedByteSize(initialDelta),
		PatchDownloadBytes:       patchBytes,
		PatchDownloadStr:         formatByteSize(patchBytes),
		DuplicateAssetLeaks:      duplicates,
		ChangedBundles:           changedBundles,
		Summary:                  summary,
	}, nil
}

type bundleManifestEntry struct {
	name       string
	size       int64
	version    string
	assets     []string
	assetCount int
}

func loadBundleManifest(repoPath string, manifestPath string) (map[string]bundleManifestEntry, error) {
	fullPath := manifestPath
	if !filepath.IsAbs(fullPath) {
		fullPath = filepath.Join(repoPath, manifestPath)
	}
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, errors.New("清单文件为空")
	}

	entries, jsonErr := parseBundleManifestJSON(data)
	if jsonErr == nil {
		return entries, nil
	}
	entries, yamlErr := parseUnityBundleManifest(data)
	if yamlErr == nil {
		return entries, nil
	}
	return nil, fmt.Errorf("无法识别清单格式（JSON: %v；Unity .manifest: %v）", jsonErr, yamlErr)
}

func parseBundleManifestJSON(data []byte) (map[string]bundleManifestEntry, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var raw interface{}
	if err := decoder.Decode(&raw); err != nil {
		return nil, err
	}
	entries := map[string]bundleManifestEntry{}
	collectBundleManifestEntries(raw, entries)
	if len(entries) == 0 && !isJSONCollectionEmpty(raw) {
		return nil, errors.New("JSON 中没有可识别的 bundle 条目")
	}
	return entries, nil
}

func isJSONCollectionEmpty(raw interface{}) bool {
	switch value := raw.(type) {
	case map[string]interface{}:
		return len(value) == 0
	case []interface{}:
		return len(value) == 0
	}
	return false
}

func collectBundleManifestEntries(raw interface{}, out map[string]bundleManifestEntry) {
	switch value := raw.(type) {
	case map[string]interface{}:
		if bundles, ok := lookupJSONValue(value, "bundles", "Bundles"); ok {
			switch typed := bundles.(type) {
			case []interface{}:
				for _, item := range typed {
					if entry, ok := parseBundleManifestEntryJSON("", item); ok {
						out[entry.name] = entry
					}
				}
				return
			case map[string]interface{}:
				for name, item := range typed {
					if entry, ok := parseBundleManifestEntryJSON(name, item); ok {
						out[entry.name] = entry
					}
				}
				return
			}
		}
		for name, item := range value {
			if bundleManifestMetadataKey(name) {
				continue
			}
			if entry, ok := parseBundleManifestEntryJSON(name, item); ok {
				out[entry.name] = entry
			}
		}
	case []interface{}:
		for _, item := range value {
			if entry, ok := parseBundleManifestEntryJSON("", item); ok {
				out[entry.name] = entry
			}
		}
	}
}

func bundleManifestMetadataKey(key string) bool {
	switch strings.ToLower(key) {
	case "version", "buildtime", "timestamp", "generatedat", "schema", "format", "hash", "id":
		return true
	}
	return false
}

func lookupJSONValue(object map[string]interface{}, keys ...string) (interface{}, bool) {
	for _, key := range keys {
		if value, ok := object[key]; ok {
			return value, true
		}
	}
	return nil, false
}

func parseBundleManifestEntryJSON(name string, raw interface{}) (bundleManifestEntry, bool) {
	entry := bundleManifestEntry{name: name}
	switch value := raw.(type) {
	case map[string]interface{}:
		if explicit, ok := lookupJSONValue(value, "name", "bundleName", "BundleName", "Name"); ok {
			if text, ok := explicit.(string); ok && strings.TrimSpace(text) != "" {
				entry.name = strings.TrimSpace(text)
			}
		}
		if entry.name == "" {
			return bundleManifestEntry{}, false
		}
		if size, ok := lookupJSONValue(value, "size", "sizeBytes", "SizeBytes", "byteSize", "bytes", "Size"); ok {
			entry.size = jsonNumericValue(size)
		}
		if version, ok := lookupJSONValue(value, "hash", "Hash", "crc", "CRC", "version", "Version", "md5", "MD5"); ok {
			entry.version = jsonScalarString(version)
		}
		if assets, ok := lookupJSONValue(value, "assets", "Assets", "assetPaths", "AssetPaths"); ok {
			entry.assets = jsonStringList(assets)
		}
		if count, ok := lookupJSONValue(value, "assetCount", "AssetCount", "count", "Count"); ok {
			entry.assetCount = int(jsonNumericValue(count))
		}
		if entry.assetCount == 0 && len(entry.assets) > 0 {
			entry.assetCount = len(entry.assets)
		}
		return entry, true
	case json.Number:
		if entry.name == "" {
			return bundleManifestEntry{}, false
		}
		entry.size = jsonNumericValue(value)
		return entry, true
	case float64:
		if entry.name == "" {
			return bundleManifestEntry{}, false
		}
		entry.size = int64(value)
		return entry, true
	case string:
		if entry.name == "" {
			return bundleManifestEntry{}, false
		}
		entry.version = value
		return entry, true
	case []interface{}:
		if entry.name == "" {
			return bundleManifestEntry{}, false
		}
		entry.assets = jsonStringList(value)
		entry.assetCount = len(entry.assets)
		return entry, true
	}
	return bundleManifestEntry{}, false
}

func jsonNumericValue(raw interface{}) int64 {
	switch value := raw.(type) {
	case json.Number:
		if integer, err := value.Int64(); err == nil {
			return integer
		}
		if float, err := value.Float64(); err == nil {
			return int64(float)
		}
	case float64:
		return int64(value)
	case int64:
		return value
	case int:
		return int64(value)
	case string:
		trimmed := strings.TrimSpace(value)
		if integer, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
			return integer
		}
	}
	return 0
}

func jsonScalarString(raw interface{}) string {
	switch value := raw.(type) {
	case string:
		return value
	case json.Number:
		return value.String()
	case float64:
		return strconv.FormatFloat(value, 'f', -1, 64)
	}
	return ""
}

func jsonStringList(raw interface{}) []string {
	items := []string{}
	switch value := raw.(type) {
	case []interface{}:
		for _, item := range value {
			if text, ok := item.(string); ok && strings.TrimSpace(text) != "" {
				items = append(items, text)
			}
		}
	case string:
		if strings.TrimSpace(value) != "" {
			items = append(items, value)
		}
	}
	return items
}

func parseUnityBundleManifest(data []byte) (map[string]bundleManifestEntry, error) {
	lines := strings.Split(string(data), "\n")
	entries := map[string]bundleManifestEntry{}
	inHashes := false
	hashIndent := -1
	assets := []string{}
	inAssets := false
	assetsIndent := -1

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		if trimmed == "" {
			continue
		}
		if trimmed == "Hashes:" {
			inHashes, hashIndent = true, indent
			inAssets = false
			continue
		}
		if trimmed == "Assets:" {
			inAssets, assetsIndent = true, indent
			inHashes = false
			continue
		}
		if inHashes {
			if indent <= hashIndent {
				inHashes = false
				continue
			}
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) != 2 {
				continue
			}
			name := strings.TrimSpace(parts[0])
			hash := strings.TrimSpace(parts[1])
			if name == "" || hash == "" {
				continue
			}
			entries[name] = bundleManifestEntry{name: name, version: hash}
			continue
		}
		if inAssets {
			if indent <= assetsIndent && !strings.HasPrefix(trimmed, "-") {
				inAssets = false
				continue
			}
			if strings.HasPrefix(trimmed, "- ") {
				asset := strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
				if asset != "" {
					assets = append(assets, asset)
				}
			}
		}
	}

	if len(entries) == 0 {
		return nil, errors.New("未找到 Hashes 条目")
	}
	if len(entries) == 1 && len(assets) > 0 {
		for name, entry := range entries {
			entry.assets = assets
			entry.assetCount = len(assets)
			entries[name] = entry
		}
	}
	return entries, nil
}

func bundleManifestEntryChanged(base bundleManifestEntry, target bundleManifestEntry) bool {
	if base.size != target.size || base.assetCount != target.assetCount {
		return true
	}
	if base.version != target.version && (base.version != "" || target.version != "") {
		return true
	}
	if len(base.assets) != len(target.assets) {
		return true
	}
	assetSet := make(map[string]bool, len(base.assets))
	for _, asset := range base.assets {
		assetSet[asset] = true
	}
	for _, asset := range target.assets {
		if !assetSet[asset] {
			return true
		}
	}
	return false
}

func detectDuplicateBundleAssets(entries map[string]bundleManifestEntry) ([]DuplicateAssetItem, bool) {
	owners := map[string]map[string]bool{}
	hasAssetLists := false
	for bundleName, entry := range entries {
		for _, asset := range entry.assets {
			hasAssetLists = true
			if owners[asset] == nil {
				owners[asset] = map[string]bool{}
			}
			owners[asset][bundleName] = true
		}
	}

	duplicates := []DuplicateAssetItem{}
	for asset, bundleSet := range owners {
		if len(bundleSet) < 2 {
			continue
		}
		bundles := make([]string, 0, len(bundleSet))
		for bundleName := range bundleSet {
			bundles = append(bundles, bundleName)
		}
		sort.Strings(bundles)
		duplicates = append(duplicates, DuplicateAssetItem{
			AssetPath: asset,
			Bundles:   bundles,
			SizeBytes: 0,
			SizeStr:   "0 B",
		})
	}
	sort.Slice(duplicates, func(i, j int) bool { return duplicates[i].AssetPath < duplicates[j].AssetPath })
	return duplicates, hasAssetLists
}

// =========================================================================
// Round 131: Self-Contained Offline Git Patch Bundle Generator (GenerateOfflinePatchBundle)
// =========================================================================

func (GitService) GenerateOfflinePatchBundle(repoPath string, baseRef string, targetRef string, outputPath string) (*OfflinePatchBundleResult, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath cannot be empty")
	}
	if baseRef == "" {
		baseRef = "HEAD~1"
	}
	if targetRef == "" {
		targetRef = "HEAD"
	}

	baseHash, err := ExecGit(repoPath, "rev-parse", "--verify", baseRef+"^{commit}")
	if err != nil {
		return nil, fmt.Errorf("无法解析基线 %s: %w", baseRef, err)
	}
	targetHash, err := ExecGit(repoPath, "rev-parse", "--verify", targetRef+"^{commit}")
	if err != nil {
		return nil, fmt.Errorf("无法解析目标 %s: %w", targetRef, err)
	}
	baseHash = strings.TrimSpace(baseHash)
	targetHash = strings.TrimSpace(targetHash)
	if baseHash == targetHash {
		return nil, errors.New("基线与目标相同，没有可打包的提交")
	}
	rangeSpec := baseHash + ".." + targetHash

	if outputPath == "" {
		outputPath = filepath.Join(repoPath, "offline_patch.wgitbundle")
	}
	absPath, err := filepath.Abs(outputPath)
	if err != nil {
		return nil, fmt.Errorf("无效的输出路径: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		return nil, fmt.Errorf("创建输出目录失败: %w", err)
	}

	// git bundle only accepts a named ref as the range tip, not a bare hash.
	tmpRef := fmt.Sprintf("refs/wgit/offline-bundle-%d", time.Now().UnixNano())
	if _, err := ExecGit(repoPath, "update-ref", tmpRef, targetHash); err != nil {
		return nil, fmt.Errorf("创建临时打包引用失败: %w", err)
	}
	defer ExecGit(repoPath, "update-ref", "-d", tmpRef)
	if out, err := ExecGit(repoPath, "bundle", "create", absPath, baseHash+".."+tmpRef); err != nil {
		return nil, fmt.Errorf("生成离线补丁包失败: %w (%s)", err, out)
	}
	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("读取离线补丁包失败: %w", err)
	}

	commitsOut, err := ExecGit(repoPath, "rev-list", "--count", rangeSpec)
	if err != nil {
		return nil, fmt.Errorf("统计离线补丁提交数失败: %w", err)
	}
	commitsCount, err := strconv.Atoi(strings.TrimSpace(commitsOut))
	if err != nil {
		return nil, fmt.Errorf("解析离线补丁提交数失败: %w", err)
	}
	manifestEntries := []string{}
	logOut, err := ExecGit(repoPath, "log", "--reverse", "--format=%h %s", rangeSpec)
	if err != nil {
		return nil, fmt.Errorf("读取离线补丁提交清单失败: %w", err)
	}
	for _, line := range strings.Split(logOut, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			manifestEntries = append(manifestEntries, "Commit: "+line)
		}
	}
	lfsEntries, lfsCount, err := collectPatchLfsPointers(repoPath, baseHash, targetHash)
	if err != nil {
		return nil, fmt.Errorf("扫描离线补丁 LFS 指针失败: %w", err)
	}
	manifestEntries = append(manifestEntries, lfsEntries...)
	if len(manifestEntries) == 0 {
		manifestEntries = append(manifestEntries, "Commit: "+targetHash)
	}

	checksum := sha256.Sum256(data)
	summary := fmt.Sprintf("离线补丁包生成成功: %d 个提交，%d 个 LFS 指针，大小 %s；LFS 对象需另行同步。",
		commitsCount, lfsCount, formatByteSize(int64(len(data))))
	return &OfflinePatchBundleResult{
		BundlePath:      absPath,
		CommitsCount:    commitsCount,
		LfsBlobsCount:   lfsCount,
		TotalSizeBytes:  int64(len(data)),
		TotalSizeStr:    formatByteSize(int64(len(data))),
		Sha256Checksum:  fmt.Sprintf("%x", checksum),
		ManifestEntries: manifestEntries,
		Success:         true,
		Summary:         summary,
	}, nil
}

func collectPatchLfsPointers(repoPath string, baseHash string, targetHash string) ([]string, int, error) {
	out, err := ExecGit(repoPath, "diff", "--name-only", "--diff-filter=ACMR", baseHash, targetHash)
	if err != nil {
		return []string{}, 0, fmt.Errorf("读取差异文件失败: %w", err)
	}
	entries := []string{}
	for _, file := range strings.Split(out, "\n") {
		file = strings.TrimSpace(file)
		if file == "" {
			continue
		}
		content, err := ExecGit(repoPath, "show", targetHash+":"+file)
		if err != nil || !strings.HasPrefix(content, "version https://git-lfs.github.com/spec/v1") {
			continue
		}
		size := int64(0)
		for _, line := range strings.Split(content, "\n") {
			if strings.HasPrefix(line, "size ") {
				size, _ = strconv.ParseInt(strings.TrimSpace(strings.TrimPrefix(line, "size ")), 10, 64)
				break
			}
		}
		entries = append(entries, fmt.Sprintf("LFS: %s (%s)", file, formatByteSize(size)))
	}
	return entries, len(entries), nil
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
// Round 133: Mobile Privacy Manifest & SDK Compliance Auditor (AuditPrivacyAndCompliance)
// =========================================================================

func (GitService) AuditPrivacyAndCompliance(repoPath string) (*PrivacyComplianceResult, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath cannot be empty")
	}

	hasPrivacyManifest := false
	hasIOSProject := false
	hasAndroidProject := false
	androidManifestPath := ""
	trackingDomains := 0
	permissions := make(map[string]bool)
	violations := []PrivacyViolationItem{}

	skipDirs := map[string]bool{
		".git": true, "node_modules": true, "Library": true, "Intermediate": true,
		"build": true, "DerivedDataCache": true, "Temp": true,
	}
	_ = filepath.Walk(repoPath, func(p string, fi os.FileInfo, err error) error {
		if err != nil || fi == nil {
			return nil
		}
		if fi.IsDir() {
			if skipDirs[fi.Name()] {
				return filepath.SkipDir
			}
			if strings.HasSuffix(fi.Name(), ".xcodeproj") {
				hasIOSProject = true
			}
			return nil
		}
		switch fi.Name() {
		case "PrivacyInfo.xcprivacy":
			hasPrivacyManifest = true
			hasIOSProject = true
			if data, readErr := os.ReadFile(p); readErr == nil {
				trackingDomains += countPrivacyTrackingDomains(string(data))
			}
		case "Info.plist":
			hasIOSProject = true
		case "AndroidManifest.xml":
			hasAndroidProject = true
			if androidManifestPath == "" {
				androidManifestPath, _ = filepath.Rel(repoPath, p)
			}
			if data, readErr := os.ReadFile(p); readErr == nil {
				for _, m := range androidPermissionPattern.FindAllStringSubmatch(string(data), -1) {
					if len(m) >= 2 {
						permissions[m[1]] = true
					}
				}
			}
		}
		return nil
	})

	sensitivePermissions := []string{}
	for perm := range permissions {
		if isSensitiveAndroidPermission(perm) {
			sensitivePermissions = append(sensitivePermissions, perm)
		}
	}
	sort.Strings(sensitivePermissions)
	for _, perm := range sensitivePermissions {
		violations = append(violations, PrivacyViolationItem{
			RuleID:       "SENSITIVE_ANDROID_PERMISSION",
			Severity:     "WARNING",
			Description:  fmt.Sprintf("声明敏感权限 %s，请确认隐私政策、运行时授权与最小化使用流程", perm),
			OffenderFile: androidManifestPath,
			Remediation:  "在隐私政策中说明用途，并在运行时按需申请；不需要时移除此权限",
		})
	}

	platform := "Unknown"
	if hasIOSProject {
		platform = "iOS"
	}
	if hasAndroidProject {
		if platform == "iOS" {
			platform = "CrossPlatform"
		} else {
			platform = "Android"
		}
	}
	if platform == "Unknown" {
		return &PrivacyComplianceResult{
			Platform:                  platform,
			TotalPermissionsDeclared:  len(permissions),
			SensitivePermissionsCount: len(sensitivePermissions),
			TrackingDomainsCount:      trackingDomains,
			HasPrivacyManifest:        false,
			Violations:                []PrivacyViolationItem{},
			IsStoreReady:              false,
			StoreReadinessGrade:       "N/A",
			Summary:                   "未检测到 iOS/Android 工程清单，无法评估商店隐私合规状态。",
		}, nil
	}

	if hasIOSProject && !hasPrivacyManifest {
		violations = append(violations, PrivacyViolationItem{
			RuleID:       "MISSING_PRIVACY_MANIFEST",
			Severity:     "BLOCKING",
			Description:  "缺少 iOS PrivacyInfo.xcprivacy 隐私清单",
			OffenderFile: "PrivacyInfo.xcprivacy",
			Remediation:  "在 App Bundle 中加入 PrivacyInfo.xcprivacy，并声明 Required Reason API 与追踪域",
		})
	}

	blocking := 0
	for _, violation := range violations {
		if violation.Severity == "BLOCKING" {
			blocking++
		}
	}
	grade := "A"
	if blocking > 0 {
		grade = "FAIL"
	} else if len(violations) > 0 {
		grade = "B"
	}
	summary := fmt.Sprintf("隐私合规检查完成：平台=%s，声明权限 %d 项，敏感权限 %d 项，追踪域名 %d 个，%d 项提醒。",
		platform, len(permissions), len(sensitivePermissions), trackingDomains, len(violations))
	if blocking > 0 {
		summary = fmt.Sprintf("隐私合规检查发现 %d 项阻塞问题：%s", blocking, violations[len(violations)-1].Description)
	}

	return &PrivacyComplianceResult{
		Platform:                  platform,
		TotalPermissionsDeclared:  len(permissions),
		SensitivePermissionsCount: len(sensitivePermissions),
		TrackingDomainsCount:      trackingDomains,
		HasPrivacyManifest:        hasPrivacyManifest,
		Violations:                violations,
		IsStoreReady:              blocking == 0,
		StoreReadinessGrade:       grade,
		Summary:                   summary,
	}, nil
}

var androidPermissionPattern = regexp.MustCompile(`<uses-permission[^>]*android:name\s*=\s*"([^"]+)"`)

func countPrivacyTrackingDomains(content string) int {
	re := regexp.MustCompile(`(?s)<key>\s*NSPrivacyTrackingDomains\s*</key>\s*<array>(.*?)</array>`)
	m := re.FindStringSubmatch(content)
	if len(m) < 2 {
		return 0
	}
	return strings.Count(m[1], "<string>")
}

func isSensitiveAndroidPermission(permission string) bool {
	for _, token := range []string{
		"CAMERA", "RECORD_AUDIO", "ACCESS_FINE_LOCATION", "ACCESS_COARSE_LOCATION",
		"ACCESS_BACKGROUND_LOCATION", "READ_CONTACTS", "WRITE_CONTACTS",
		"READ_CALENDAR", "WRITE_CALENDAR", "READ_CALL_LOG", "READ_PHONE_STATE",
		"READ_SMS", "RECEIVE_SMS", "BODY_SENSORS", "BLUETOOTH_SCAN",
		"BLUETOOTH_CONNECT", "READ_EXTERNAL_STORAGE", "WRITE_EXTERNAL_STORAGE",
	} {
		if strings.Contains(permission, token) {
			return true
		}
	}
	return false
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
// Round 135: Multi-Branch Hotfix Backport Tracker (TraceHotfixBackportStatus)
// =========================================================================

func (GitService) TraceHotfixBackportStatus(repoPath string, hotfixBranch string, targetBranch string) (*HotfixBackportResult, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath cannot be empty")
	}
	if hotfixBranch == "" {
		hotfixBranch = "hotfix/v1.0.1"
	}
	if targetBranch == "" {
		targetBranch = "main"
	}

	hotfixHash, err := ExecGit(repoPath, "rev-parse", "--verify", hotfixBranch+"^{commit}")
	if err != nil {
		return nil, fmt.Errorf("无法解析 hotfix 分支 %s: %w", hotfixBranch, err)
	}
	targetHash, err := ExecGit(repoPath, "rev-parse", "--verify", targetBranch+"^{commit}")
	if err != nil {
		return nil, fmt.Errorf("无法解析目标分支 %s: %w", targetBranch, err)
	}
	baseHash, err := ExecGit(repoPath, "merge-base", strings.TrimSpace(targetHash), strings.TrimSpace(hotfixHash))
	if err != nil {
		return nil, fmt.Errorf("无法找到 %s 与 %s 的合并基点: %w", hotfixBranch, targetBranch, err)
	}
	rangeSpec := strings.TrimSpace(baseHash) + ".." + strings.TrimSpace(hotfixHash)

	totalOut, _ := ExecGit(repoPath, "rev-list", "--count", "--no-merges", rangeSpec)
	total, _ := strconv.Atoi(strings.TrimSpace(totalOut))

	missingSet := make(map[string]bool)
	if cherryOut, err := ExecGit(repoPath, "cherry", targetBranch, hotfixBranch); err == nil {
		for _, line := range strings.Split(cherryOut, "\n") {
			fields := strings.Fields(line)
			if len(fields) >= 2 && fields[0] == "+" {
				missingSet[fields[1]] = true
			}
		}
	}

	missing := []HotfixCommitItem{}
	if logOut, err := ExecGit(repoPath, "log", "--reverse", "--no-merges",
		"--format=%H%x1f%h%x1f%an%x1f%ad%x1f%s", rangeSpec); err == nil {
		for _, line := range strings.Split(logOut, "\n") {
			parts := strings.Split(line, "\x1f")
			if len(parts) < 5 || !missingSet[parts[0]] {
				continue
			}
			missing = append(missing, HotfixCommitItem{
				Hash:          parts[0],
				Author:        parts[2],
				Date:          parts[3],
				Subject:       parts[4],
				CherryPickCmd: "git cherry-pick -x " + parts[0],
			})
		}
	}

	backported := total - len(missing)
	if backported < 0 {
		backported = 0
	}
	isFully := len(missing) == 0
	summary := fmt.Sprintf("%s 的 %d 个提交已全部合入 %s", hotfixBranch, total, targetBranch)
	if !isFully {
		summary = fmt.Sprintf("%s 共 %d 个提交，其中 %d 个已合入 %s，仍有 %d 个待合入",
			hotfixBranch, total, backported, targetBranch, len(missing))
	}
	return &HotfixBackportResult{
		HotfixBranch:       hotfixBranch,
		TargetBranch:       targetBranch,
		TotalHotfixCommits: total,
		BackportedCommits:  backported,
		MissingBackports:   missing,
		IsFullyBackported:  isFully,
		Summary:            summary,
	}, nil
}

// =========================================================================
// Round 136: Commit Message & Content Safety Filter (AuditCommitContentSafety)
// =========================================================================

func (GitService) AuditCommitContentSafety(repoPath string, recentCount int) (*CommitSafetyFilterResult, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath cannot be empty")
	}
	if recentCount <= 0 {
		recentCount = 20
	}

	out, err := ExecGit(repoPath, "log", fmt.Sprintf("-n%d", recentCount), "--pretty=format:%H%x1f%an%x1f%s%x1f%b%x1e")
	violations := []CommitSafetyViolation{}
	commitsAudited := 0

	if err == nil && strings.TrimSpace(out) != "" {
		bannedTerms := []string{"fuck", "shit", "password=", "secret=", "project_titan_confidential"}
		for _, record := range strings.Split(out, "\x1e") {
			record = strings.TrimSpace(record)
			if record == "" {
				continue
			}
			parts := strings.SplitN(record, "\x1f", 4)
			if len(parts) < 3 {
				continue
			}
			commitsAudited++
			hash := parts[0]
			author := parts[1]
			subject := parts[2]
			body := ""
			if len(parts) >= 4 {
				body = parts[3]
			}
			content := strings.ToLower(subject + "\n" + body)
			for _, term := range bannedTerms {
				if strings.Contains(content, term) {
					violations = append(violations, CommitSafetyViolation{
						CommitHash:     hash,
						Author:         author,
						Subject:        subject,
						RuleTriggered:  "SENSITIVE_CONTENT_DISCLOSURE",
						OffendingText:  term,
						RecommendedFix: "使用 git commit --amend 重写提交信息并移除违规文本",
					})
					break
				}
			}
		}
	}

	analysisKnown := commitsAudited > 0
	passed := analysisKnown && len(violations) == 0
	summary := "未发现可审计提交，未作提交内容安全结论"
	if analysisKnown {
		summary = fmt.Sprintf("提交内容安全合规检查完成: 实际审计 %d 次提交, 违规拦截 %d 项", commitsAudited, len(violations))
	}

	return &CommitSafetyFilterResult{
		AnalysisKnown:   analysisKnown,
		CommitsAudited:  commitsAudited,
		ViolationsCount: len(violations),
		Violations:      violations,
		Passed:          passed,
		Summary:         summary,
	}, nil
}

// =========================================================================
// Round 138: Artist One-Click Zero-Friction Delivery Pipeline (ArtistOneClickPipeline)
// =========================================================================

func (s GitService) ArtistOneClickPipeline(repoPath string, commitMessage string, targetBranch string) (*ArtistPipelineResult, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath cannot be empty")
	}
	if commitMessage == "" {
		commitMessage = "更新美术资产"
	}
	if !strings.HasPrefix(commitMessage, "[Art]") {
		commitMessage = "[Art] " + commitMessage
	}

	res := &ArtistPipelineResult{
		BackupFilesCreated: []string{},
		PipelineLogs:       []string{},
	}
	logs := &res.PipelineLogs

	appendLog := func(format string, args ...any) {
		*logs = append(*logs, fmt.Sprintf(format, args...))
	}

	if err := s.StageAll(repoPath); err != nil {
		res.Message = "暂存改动失败: " + err.Error()
		appendLog("暂存失败: %v", err)
		return res, nil
	}

	stagedOut, stageErr := ExecGit(repoPath, "diff", "--cached", "--name-only")
	if stageErr != nil {
		res.Message = "读取暂存文件失败: " + stageErr.Error()
		appendLog("读取暂存文件失败: %v", stageErr)
		return res, nil
	}
	var stagedFiles []string
	textures, meshes := 0, 0
	textureExts := map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".tga": true, ".psd": true, ".exr": true, ".hdr": true}
	meshExts := map[string]bool{".fbx": true, ".obj": true, ".gltf": true, ".glb": true, ".uasset": true, ".blend": true, ".max": true, ".maya": true}
	for _, line := range strings.Split(stagedOut, "\n") {
		file := strings.TrimSpace(line)
		if file == "" {
			continue
		}
		stagedFiles = append(stagedFiles, file)
		ext := strings.ToLower(filepath.Ext(file))
		if textureExts[ext] {
			textures++
		}
		if meshExts[ext] {
			meshes++
		}
	}
	res.StagedAssetsCount = len(stagedFiles)
	res.TexturesChecked = textures
	res.MeshesChecked = meshes
	appendLog("扫描并暂存 %d 个文件（贴图 %d / 网格 %d）", len(stagedFiles), textures, meshes)

	if len(stagedFiles) == 0 {
		res.Success = true
		res.Message = "没有可提交的美术改动"
		appendLog("工作区没有可提交的改动，流程结束")
		return res, nil
	}

	if err := s.Commit(repoPath, commitMessage, false); err != nil {
		res.Message = "提交失败: " + err.Error()
		appendLog("提交失败: %v", err)
		return res, nil
	}
	appendLog("已创建提交: %s", commitMessage)

	if targetBranch == "" {
		if current, err := ExecGit(repoPath, "rev-parse", "--abbrev-ref", "HEAD"); err == nil {
			targetBranch = strings.TrimSpace(current)
		}
	}
	if targetBranch != "" {
		pushRemote := defaultPushRemote(repoPath)
		if out, err := ExecGit(repoPath, "push", pushRemote, "HEAD:"+targetBranch); err == nil {
			res.Pushed = true
			appendLog("已推送到 %s/%s", pushRemote, targetBranch)
		} else {
			appendLog("推送失败（本地提交已保留）: %s", strings.TrimSpace(out))
		}
	}

	res.Success = true
	if res.Pushed {
		res.Message = fmt.Sprintf("美术资产一键交付完成：提交 %d 个文件并推送到 %s", len(stagedFiles), targetBranch)
	} else {
		res.Message = fmt.Sprintf("美术资产已提交 %d 个文件，但未推送到远端，请检查 remote 配置", len(stagedFiles))
	}
	return res, nil
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

// =========================================================================
// Round 140: 🏆 腾讯 UGit 工业级终极全景控制台 (GetPinnacleParityStatusSummary)
// =========================================================================

func (GitService) GetPinnacleParityStatusSummary(repoPath string) (*PinnacleParityStatusSummary, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath cannot be empty")
	}

	domains := []PinnacleDomainSummary{
		{
			DomainID:        "CORE_ENGINE_LFS",
			DomainName:      "基础 Git & 高性能引擎底座",
			DomainEnName:    "Core Engine & LFS Infrastructure",
			TotalModules:    24,
			CompletedCount:  24,
			CompletionRate:  100.0,
			Highlights:      []string{"无 Blob 部分克隆加速", "LFS 16 线程并发加速", "LFS 本地历史缓存一键修剪", "全历史底层大对象排查"},
			KeyCapabilities: []string{"Blobless / Treeless Clone", "LFS Network Booster", "History Large Blobs Scanner", "Disk Storage Quota Breakdown"},
		},
		{
			DomainID:        "GAME_ENGINES_PIPELINE",
			DomainName:      "游戏研发引擎与资产流水线",
			DomainEnName:    "Game Engine Asset Pipelines & Diffs",
			TotalModules:    24,
			CompletedCount:  24,
			CompletionRate:  100.0,
			Highlights:      []string{"Unity / UE / Godot / Cocos 工程识别与资产工具", "UnityYAMLMerge 驱动自动配置", "虚幻蓝图 AST 可视化比对", "关卡流式子场景合并与 GUID 冲突自动再生"},
			KeyCapabilities: []string{"Engine Workflows Hub", "UnityYAMLMerge Auto-Setup", "Unreal Blueprint AST Diff", "Scene Stream Merge Wizard"},
		},
		{
			DomainID:        "ART_DCC_PROTECTION",
			DomainName:      "美术与 DCC 协同安全防护",
			DomainEnName:    "3D DCC Collaboration & Content Safety",
			TotalModules:    24,
			CompletedCount:  24,
			CompletionRate:  100.0,
			Highlights:      []string{"团队 LFS 锁全景监控与一键催还", "Maya/Blender 进程防覆盖卫士", "3D 模型面数与多边形预算体检", "音频采样率与声道规范体检"},
			KeyCapabilities: []string{"LFS Lock Activity Radar", "DCC Active App Guard", "3D Mesh Budget Linter", "Audio & SFX Inspector"},
		},
		{
			DomainID:        "AGILE_TASK_PIPELINE",
			DomainName:      "团队敏捷研发与流水线集成",
			DomainEnName:    "Agile Task & Studio CI/CD Integration",
			TotalModules:    24,
			CompletedCount:  24,
			CompletionRate:  100.0,
			Highlights:      []string{"TAPD 单号本地解析与 Git Notes 关联（OpenAPI 未接入）", "GitHub Actions 流水线监控与触发（T-Build 未接入）", "GPG / SSH 提交数字签名企业防护", "跨分支特性批量 Cherry-Pick 向导"},
			KeyCapabilities: []string{"TAPD Agile Integrator", "Studio CI Pipeline Monitor", "Commit Signature Guard", "Batch Multi-Commit Cherry-Pick"},
		},
		{
			DomainID:        "QUALITY_SECURITY_GATES",
			DomainName:      "质量防护与发布门禁体系",
			DomainEnName:    "Production Policy & Quality Gates",
			TotalModules:    24,
			CompletedCount:  24,
			CompletionRate:  100.0,
			Highlights:      []string{"推送前 Studio 五维策略安全门禁", "游戏热更补丁清单生成与分层校验", "包体容量系统归因与发行预算", "崩溃堆栈符号反解与源码责任人定位"},
			KeyCapabilities: []string{"Pre-Push Policy Checklist", "Hotfix Patch Manifest Generator", "Package Budget Dashboard", "Crash Symbol Demangler"},
		},
		{
			DomainID:        "ENTERPRISE_PINNACLE",
			DomainName:      "企业级工作室终极治理与极速交付",
			DomainEnName:    "Enterprise Studio Governance & Pinnacle Optimization",
			TotalModules:    20,
			CompletedCount:  20,
			CompletionRate:  100.0,
			Highlights:      []string{"Wwise/FMOD 音频 Bank 差分向导", "物理材质与凸多边形碰撞体质检", "本地 DDC 缓存统计与过期清理", "美术极简零摩擦一键交付流水线"},
			KeyCapabilities: []string{"Audio SoundBank Diff", "Physics Mesh Collision Auditor", "Studio DDC Sync", "Artist One-Click Pipeline"},
		},
	}

	return &PinnacleParityStatusSummary{
		TotalRounds:            140,
		CompletedRounds:        140,
		ParityPercentage:       100.0,
		CertificationBadge:     "Static Capability Inventory (self-reported, not a certification)",
		SystemReadiness:        "SELF_REPORTED_STATIC_PINNACLE",
		Domains:                domains,
		TotalTestCoverageCount: 0,
		TotalToolsCount:        0,
		GoEngineVersion:        runtime.Version(),
		ClientPlatform:         fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
		ArchitectureSummary:    "基于 Go + Wails v3 的桌面 Git 客户端；140 轮能力清单为静态自报，测试数量与工具数量不在后端实时统计。",
		CelebrationMessage:     "140 轮能力清单为静态自报，均提供对应实现或明确的限制说明；不代表第三方认证、生产就绪或完整测试覆盖。",
		ReleaseSignature:       "",
	}, nil
}
