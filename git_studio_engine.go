package main

import (
	"archive/zip"
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"debug/elf"
	"debug/macho"
	"debug/pe"
	"encoding/base64"
	"encoding/binary"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
)

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

	if cmapOffset == 0 || uint64(cmapOffset)+uint64(cmapLength) > uint64(len(data)) {
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
			// Format 4 layout: endCode[segCount] at 14, reservedPad, then the
			// startCode, idDelta and idRangeOffset arrays of segCount words
			// each, with glyphIdArray following at 16+8*segCount.
			endCodeOffset := 14
			startCodeOffset := 16 + 2*segCount
			idDeltaOffset := 16 + 4*segCount
			idRangeOffsetOffset := 16 + 6*segCount
			if len(tableData) >= 16+8*segCount {
				for s := 0; s < segCount; s++ {
					start := binary.BigEndian.Uint16(tableData[startCodeOffset+s*2 : startCodeOffset+2+s*2])
					end := binary.BigEndian.Uint16(tableData[endCodeOffset+s*2 : endCodeOffset+2+s*2])
					// Inverted segments and the 0xFFFF terminator segment
					// carry no mappings.
					if start > end || start == 0xFFFF {
						continue
					}
					idDelta := binary.BigEndian.Uint16(tableData[idDeltaOffset+s*2 : idDeltaOffset+2+s*2])
					idRangeOffset := binary.BigEndian.Uint16(tableData[idRangeOffsetOffset+s*2 : idRangeOffsetOffset+2+s*2])
					for c := int(start); c <= int(end) && c != 0xFFFF; c++ {
						var glyph uint16
						if idRangeOffset == 0 {
							glyph = (uint16(c) + idDelta) & 0xFFFF
						} else {
							// idRangeOffset is relative to its own array entry;
							// a zero entry there means no glyph for this code,
							// while nonzero entries still receive idDelta.
							addr := idRangeOffsetOffset + int(idRangeOffset) + 2*(c-int(start))
							if addr+2 > len(tableData) {
								break
							}
							if g := binary.BigEndian.Uint16(tableData[addr : addr+2]); g != 0 {
								glyph = (g + idDelta) & 0xFFFF
							}
						}
						if glyph != 0 {
							runes[rune(c)] = true
						}
					}
				}
			}
		} else if format == 12 && len(tableData) >= 16 {
			nGroups := binary.BigEndian.Uint32(tableData[12:16])
			grpOffset := 16
			const maxUnicode = uint32(0x10FFFF)
			for g := 0; g < int(nGroups); g++ {
				if grpOffset+12 > len(tableData) {
					break
				}
				startChar := binary.BigEndian.Uint32(tableData[grpOffset : grpOffset+4])
				endChar := binary.BigEndian.Uint32(tableData[grpOffset+4 : grpOffset+8])
				grpOffset += 12
				// Groups entirely outside Unicode are skipped; clamping keeps
				// the uint32 loop bounded instead of wrapping at 2^32.
				if startChar > maxUnicode {
					continue
				}
				if endChar > maxUnicode {
					endChar = maxUnicode
				}
				if startChar > endChar {
					continue
				}
				for c := startChar; c <= endChar; c++ {
					runes[rune(c)] = true
				}
			}
		}
	}

	return runes, nil
}

// =========================================================================
// Round 107: Native C++ Plugins ABI Guard (AuditNativePluginsAbi)
// =========================================================================

// readFileHeader reads at most maxBytes from the start of path without loading
// the entire file into memory. Short files (including empty ones) are not
// treated as errors: io.EOF and io.ErrUnexpectedEOF yield a shorter slice with
// a nil error. maxBytes <= 0 returns an empty header and a nil error.
func readFileHeader(path string, maxBytes int) ([]byte, error) {
	if maxBytes <= 0 {
		return nil, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	buf := make([]byte, maxBytes)
	n, err := io.ReadFull(f, buf)
	if err != nil {
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return buf[:n], nil
		}
		return nil, err
	}
	return buf[:n], nil
}

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

		fBytes, hErr := readFileHeader(p, 16)
		if hErr != nil || len(fBytes) == 0 {
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

	var warnings []string
	branch := knownBranch
	if branch == "" {
		bOut, bErr := ExecGit(dirPath, "rev-parse", "--abbrev-ref", "HEAD")
		if bErr != nil {
			warnings = append(warnings, "读取当前分支失败: "+bErr.Error())
		} else {
			branch = strings.TrimSpace(bOut)
		}
	}

	statusOut, statusErr := ExecGit(dirPath, "status", "--porcelain")
	if statusErr != nil {
		warnings = append(warnings, "读取工作区状态失败: "+statusErr.Error())
	}
	dirtyLines := 0
	for _, l := range strings.Split(statusOut, "\n") {
		if strings.TrimSpace(l) != "" {
			dirtyLines++
		}
	}

	logOut, logErr := ExecGit(dirPath, "log", "-1", "--format=%h\t%s\t%an\t%cr")
	if logErr != nil {
		warnings = append(warnings, "读取最近提交失败: "+logErr.Error())
	}
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
	if _, upErr := ExecGit(dirPath, "rev-parse", "--abbrev-ref", "@{u}"); upErr == nil {
		abOut, abErr := ExecGit(dirPath, "rev-list", "--left-right", "--count", "HEAD...@{u}")
		if abErr != nil {
			warnings = append(warnings, "读取领先/落后失败: "+abErr.Error())
		} else {
			abParts := strings.Fields(abOut)
			if len(abParts) >= 2 {
				ahead, _ = strconv.Atoi(abParts[0])
				behind, _ = strconv.Atoi(abParts[1])
			}
		}
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
	if len(warnings) > 0 {
		node.StatusWarning = strings.Join(warnings, "；")
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

// policyAuditWorkers bounds how many commits are inspected concurrently so the
// process fan-out of a large pre-push audit stays predictable.
const policyAuditWorkers = 8

// commitAuditResult captures the Rule 2 / Rule 3 outcome for a single commit.
// Workers write only their own slot; violations are merged in commit order.
type commitAuditResult struct {
	lfsViolations   []PolicyViolationItem
	blobViolations  []PolicyViolationItem
	missingLfsCount int
	oversizedCount  int
	err             error
}

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

	// Rule 2 + Rule 3 run together per commit through a bounded worker pool.
	// Each worker only touches its own slot; merging below restores the exact
	// serial ordering (all Rule 2 violations, then all Rule 3 violations).
	lfsObjDir := lfsObjectsDir(repoPath)
	reOid := regexp.MustCompile(`oid sha256:([0-9a-f]{64})`)
	results := make([]commitAuditResult, len(commits))
	sem := make(chan struct{}, policyAuditWorkers)
	var wg sync.WaitGroup
	for idx := range commits {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int) {
			defer wg.Done()
			defer func() { <-sem }()
			c := commits[i]
			res := &results[i]

			// Rule 2: Missing LFS Pointers
			diffFiles, diffErr := ExecGit(repoPath, "diff-tree", "-r", "--no-commit-id", "--name-only", c.hash)
			if diffErr != nil {
				res.err = fmt.Errorf("读取提交 %s 的文件列表失败: %v", shortHashOf(c.hash), diffErr)
				return
			}
			if diffFiles != "" {
				for _, file := range strings.Split(diffFiles, "\n") {
					file = strings.TrimSpace(file)
					if file == "" {
						continue
					}
					content, cErr := ExecGit(repoPath, "cat-file", "-p", fmt.Sprintf("%s:%s", c.hash, file))
					if cErr == nil && strings.HasPrefix(content, "version https://git-lfs.github.com/spec/v1") {
						m := reOid.FindStringSubmatch(content)
						if len(m) >= 2 {
							oid := m[1]
							localObjPath := filepath.Join(lfsObjDir, oid[:2], oid[2:4], oid)
							if _, sErr := os.Stat(localObjPath); sErr != nil {
								res.missingLfsCount++
								res.lfsViolations = append(res.lfsViolations, PolicyViolationItem{
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

			// Rule 3: Large Blobs (>100MB) without LFS
			treeOut, treeErr := ExecGit(repoPath, "ls-tree", "-r", "-l", c.hash)
			if treeErr != nil {
				res.err = fmt.Errorf("读取提交 %s 的文件树失败: %v", shortHashOf(c.hash), treeErr)
				return
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
							res.err = fmt.Errorf("解析文件 %s 大小失败: %v", fields[len(fields)-1], sizeErr)
							return
						}
						filePath := fields[len(fields)-1]
						if sizeBytes > 104857600 { // 100MB
							res.oversizedCount++
							res.blobViolations = append(res.blobViolations, PolicyViolationItem{
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
		}(idx)
	}
	wg.Wait()

	// Merge in commit order: first every Rule 2 violation, then every Rule 3
	// violation, matching the historical serial append order exactly.
	missingLfsCount := 0
	oversizedCount := 0
	for i := range results {
		if results[i].err != nil {
			return nil, results[i].err
		}
		violations = append(violations, results[i].lfsViolations...)
		missingLfsCount += results[i].missingLfsCount
	}
	for i := range results {
		violations = append(violations, results[i].blobViolations...)
		oversizedCount += results[i].oversizedCount
	}
	rulesChecked = append(rulesChecked, PolicyRuleCheckResult{
		RuleID:   "RULE_LFS_POINTER_INTEGRITY",
		RuleName: "Git LFS 远端对象完整性校验",
		Passed:   missingLfsCount == 0,
		Severity: "BLOCKING",
		Details:  fmt.Sprintf("LFS 指针扫描完成，缺失本地缓存对象: %d 个", missingLfsCount),
	})
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
	rulesSource := "default"
	ruleTerms := defaultCommitSafetyTerms()

	if err == nil && strings.TrimSpace(out) != "" {
		rules, rulesErr := loadCommitSafetyRules(repoPath)
		if rulesErr != nil || rules == nil {
			rules = &CommitSafetyRules{Terms: defaultCommitSafetyTerms(), Source: "default"}
		}
		rulesSource = rules.Source
		ruleTerms = rules.Terms
		bannedTerms := rules.Terms
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
		RulesSource:     rulesSource,
		RuleCount:       len(ruleTerms),
		RuleTerms:       ruleTerms,
	}, nil
}
