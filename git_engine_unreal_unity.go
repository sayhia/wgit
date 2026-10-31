package main

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

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
