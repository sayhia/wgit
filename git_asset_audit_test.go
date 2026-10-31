package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Self-contained asset/engine audits that do not depend on the wgit repository itself.
func TestAssetAuditsInTempRepo(t *testing.T) {
	svc := GitService{}
	tempAssetRepo := t.TempDir()
	_, _ = ExecGit(tempAssetRepo, "init")
	_, _ = ExecGit(tempAssetRepo, "config", "user.name", "Tester")
	_, _ = ExecGit(tempAssetRepo, "config", "user.email", "test@test.com")
	_ = os.MkdirAll(filepath.Join(tempAssetRepo, "Assets", "Textures"), 0755)
	_ = os.MkdirAll(filepath.Join(tempAssetRepo, "Assets", "Prefabs"), 0755)
	heroPng := filepath.Join(tempAssetRepo, "Assets", "Textures", "Hero.png")
	heroMeta := heroPng + ".meta"
	_ = os.WriteFile(heroPng, []byte("pngdata"), 0644)
	_ = os.WriteFile(heroMeta, []byte("fileFormatVersion: 2\nguid: aabbccddeeff11223344556677889900\n"), 0644)

	prefabFile := filepath.Join(tempAssetRepo, "Assets", "Prefabs", "Warrior.prefab")
	_ = os.WriteFile(prefabFile, []byte("--- !u!1 &123\nSpriteRenderer:\n  m_Sprite: {fileID: 21300000, guid: aabbccddeeff11223344556677889900, type: 3}\n"), 0644)

	refRes, err := svc.InspectAssetReferences(tempAssetRepo, "Assets/Textures/Hero.png")
	if err != nil || refRes.ReferenceCount != 1 || refRes.SafeToDelete {
		t.Fatalf("InspectAssetReferences failed: %v, res=%+v", err, refRes)
	}
	t.Logf("Asset reference inspector: found %d refs in %s (SafeToDelete: %v, GUID: %s)",
		refRes.ReferenceCount, refRes.References[0].FilePath, refRes.SafeToDelete, refRes.AssetGuid)

	// Modal 73: Test GetCommitSigningStatus & ConfigureCommitSigning
	signStatus, err := svc.GetCommitSigningStatus(tempAssetRepo)
	if err != nil {
		t.Fatalf("GetCommitSigningStatus failed: %v", err)
	}
	t.Logf("Commit signing status: enabled=%v, format=%s, compliance=%s",
		signStatus.GpgSignEnabled, signStatus.GpgFormat, signStatus.ComplianceRate)

	signCfgRes, err := svc.ConfigureCommitSigning(tempAssetRepo, true, "~/.ssh/id_rsa.pub", "ssh")
	if err != nil || !signCfgRes.Success {
		t.Fatalf("ConfigureCommitSigning failed: %v", err)
	}

	// Modal 74: Test GetLfsLockActivity
	lockAct, err := svc.GetLfsLockActivity(tempAssetRepo)
	if err != nil {
		t.Fatalf("GetLfsLockActivity failed: %v", err)
	}
	t.Logf("LFS lock activity: total=%d, my=%d, team=%d", lockAct.TotalLocks, lockAct.MyLockCount, lockAct.TeamLockCount)

	// Modal 75: Test AuditIgnoreRules & ApplyIgnoreSanitizer
	auditRes, err := svc.AuditIgnoreRules(tempAssetRepo)
	if err != nil || !auditRes.HasGitIgnore {
		// No .gitignore initially
	}
	t.Logf("Ignore rule audit: engine=%s, missing rules=%d, health=%d/100",
		auditRes.EngineType, len(auditRes.MissingRules), auditRes.HealthScore)

	// Add dangerous tracked artifact
	vsDir := filepath.Join(tempAssetRepo, ".vs")
	_ = os.MkdirAll(vsDir, 0755)
	vsFile := filepath.Join(vsDir, "config.json")
	_ = os.WriteFile(vsFile, []byte("{}"), 0644)
	_, _ = ExecGit(tempAssetRepo, "add", ".vs/config.json")

	auditRes2, _ := svc.AuditIgnoreRules(tempAssetRepo)
	if auditRes2.TrackedCount != 1 {
		t.Fatalf("Expected 1 tracked artifact, got %d", auditRes2.TrackedCount)
	}

	sanRes, err := svc.ApplyIgnoreSanitizer(tempAssetRepo, "Unity 3D", true)
	if err != nil || !sanRes.Success || len(sanRes.RulesAdded) == 0 || len(sanRes.UntrackedFiles) != 1 {
		t.Fatalf("ApplyIgnoreSanitizer failed: %v, res=%+v", err, sanRes)
	}
	t.Logf("Applied ignore sanitizer: added %d rules, untracked %d files",
		len(sanRes.RulesAdded), len(sanRes.UntrackedFiles))

	// Modal 76: Test InspectAudioFile
	audioDir := filepath.Join(tempAssetRepo, "Assets", "Audio")
	_ = os.MkdirAll(audioDir, 0755)
	wavPath := filepath.Join(audioDir, "Laser.wav")
	// Construct minimal 44-byte RIFF/WAVE header: 48000Hz, 2ch, 16bit
	wavHeader := []byte{
		'R', 'I', 'F', 'F',
		36, 0, 0, 0, // 36 + data size (0)
		'W', 'A', 'V', 'E',
		'f', 'm', 't', ' ',
		16, 0, 0, 0, // Subchunk1Size (16 for PCM)
		1, 0, // AudioFormat (1=PCM)
		2, 0, // NumChannels (2)
		0x80, 0xbb, 0, 0, // SampleRate = 48000 (0x0000bb80)
		0x00, 0xee, 0x02, 0, // ByteRate = 48000 * 2 * 2 = 192000 (0x0002ee00)
		4, 0, // BlockAlign (4)
		16, 0, // BitsPerSample (16)
		'd', 'a', 't', 'a',
		0, 0, 0, 0, // DataSize = 0
	}
	_ = os.WriteFile(wavPath, wavHeader, 0644)
	audioRes, err := svc.InspectAudioFile(tempAssetRepo, "Assets/Audio/Laser.wav")
	if err != nil || audioRes.SampleRate != 48000 || !audioRes.IsStandard48k || audioRes.Channels != 2 {
		t.Fatalf("InspectAudioFile failed: %v, res=%+v", err, audioRes)
	}
	t.Logf("Audio inspector: format=%s, sampleRate=%d, channels=%s, std48k=%v",
		audioRes.Format, audioRes.SampleRate, audioRes.ChannelStr, audioRes.IsStandard48k)

	// Modal 77: Test LintModelMesh
	modelsDir := filepath.Join(tempAssetRepo, "Assets", "Models")
	_ = os.MkdirAll(modelsDir, 0755)
	objPath := filepath.Join(modelsDir, "Sword.obj")
	objContent := "# Sword Model\nv 0.0 0.0 0.0\nv 1.0 0.0 0.0\nv 0.0 1.0 0.0\nv 1.0 1.0 0.0\nf 1 2 3\nf 2 4 3\n"
	_ = os.WriteFile(objPath, []byte(objContent), 0644)
	meshRes, err := svc.LintModelMesh(tempAssetRepo, "Assets/Models/Sword.obj", "mobile_prop")
	if err != nil || meshRes.TriangleCount != 2 || meshRes.VertexCount != 4 || meshRes.IsOverBudget {
		t.Fatalf("LintModelMesh failed: %v, res=%+v", err, meshRes)
	}
	t.Logf("Mesh linter: triangles=%d, vertices=%d, budget=%s, severity=%s",
		meshRes.TriangleCount, meshRes.VertexCount, meshRes.BudgetPreset, meshRes.Severity)

	// Modal 78: CI monitor must not fabricate runs for a repo without CI config
	ciRes, err := svc.GetStudioCIPipelines(tempAssetRepo)
	if err != nil || ciRes.LatestRun != nil || ciRes.Provider != "未检测到 CI 配置" {
		t.Fatalf("GetStudioCIPipelines failed: %v, res=%+v", err, ciRes)
	}
	if _, err := svc.TriggerStudioCIPipeline(tempAssetRepo, "Nightly-Cook"); err == nil {
		t.Fatal("TriggerStudioCIPipeline must fail without a GitHub Actions remote")
	}
	t.Logf("Studio CI monitor: provider=%s, message=%s", ciRes.Provider, ciRes.Message)

	// Modal 79: Test GetFileRevisionTimeline & RevertFileToRevision
	testDoc := filepath.Join(tempAssetRepo, "README.md")
	_ = os.WriteFile(testDoc, []byte("# Version 1"), 0644)
	// Fail fast on the setup itself: the previous version ignored commit errors,
	// which surfaced later as a confusing "timeline is empty" assertion.
	runGitTest(t, tempAssetRepo, nil, "add", "README.md")
	runGitTest(t, tempAssetRepo, nil, "commit", "-m", "docs: version 1")
	firstHash, _ := ExecGit(tempAssetRepo, "rev-parse", "HEAD")
	firstHash = strings.TrimSpace(firstHash)

	_ = os.WriteFile(testDoc, []byte("# Version 2 with changes"), 0644)
	runGitTest(t, tempAssetRepo, nil, "add", "README.md")
	runGitTest(t, tempAssetRepo, nil, "commit", "-m", "docs: version 2")

	timelineRes, err := svc.GetFileRevisionTimeline(tempAssetRepo, "README.md")
	if err != nil || len(timelineRes.Revisions) < 2 {
		t.Fatalf("GetFileRevisionTimeline failed: %v, res=%+v", err, timelineRes)
	}
	t.Logf("File revision timeline: %d revisions found for %s", len(timelineRes.Revisions), timelineRes.FilePath)

	revertRes, err := svc.RevertFileToRevision(tempAssetRepo, "README.md", firstHash)
	if err != nil || !revertRes.Success {
		t.Fatalf("RevertFileToRevision failed: %v, res=%+v", err, revertRes)
	}
	revertedBytes, _ := os.ReadFile(testDoc)
	if string(revertedBytes) != "# Version 1" {
		t.Fatalf("Expected file to be reverted to '# Version 1', got '%s'", string(revertedBytes))
	}
	t.Logf("Single file reverted successfully to %s", revertRes.CommitHash)

	// ==========================================
	// Round 101: Test InspectShaderVariants
	// ==========================================
	shadersDir := filepath.Join(tempAssetRepo, "Assets", "Shaders")
	_ = os.MkdirAll(shadersDir, 0755)

	pbrShaderPath := filepath.Join(shadersDir, "CustomPBR.shader")
	pbrContent := `Shader "Custom/PBRStandard" {
		Properties {
			_Color ("Color", Color) = (1,1,1,1)
		}
		SubShader {
			Pass {
				Name "ForwardBase"
				#pragma multi_compile _ DIRECTIONAL POINT SPOT
				#pragma multi_compile _ SHADOWS_SOFT
				#pragma shader_feature _ NORMALMAP
			}
		}
	}`
	_ = os.WriteFile(pbrShaderPath, []byte(pbrContent), 0644)

	uberShaderPath := filepath.Join(shadersDir, "UberExplosion.shader")
	var uberLines []string
	uberLines = append(uberLines, "Shader \"Custom/UberExplosion\" { SubShader { Pass { Name \"UberPass\"")
	for i := 1; i <= 11; i++ {
		uberLines = append(uberLines, fmt.Sprintf("#pragma multi_compile _ FEATURE_SWITCH_%d", i))
	}
	uberLines = append(uberLines, "} } }")
	_ = os.WriteFile(uberShaderPath, []byte(strings.Join(uberLines, "\n")), 0644)

	pbrRes, err := svc.InspectShaderVariants(tempAssetRepo, "Assets/Shaders/CustomPBR.shader")
	if err != nil || pbrRes.TotalVariants != 16 || pbrRes.IsExplosion || pbrRes.Severity != "OK" {
		t.Fatalf("InspectShaderVariants (PBR) failed: %v, res=%+v", err, pbrRes)
	}
	t.Logf("Shader inspector (PBR): variants=%d, pragmas=%d, severity=%s",
		pbrRes.TotalVariants, len(pbrRes.Pragmas), pbrRes.Severity)

	uberRes, err := svc.InspectShaderVariants(tempAssetRepo, "Assets/Shaders/UberExplosion.shader")
	if err != nil || uberRes.TotalVariants != 2048 || !uberRes.IsExplosion || uberRes.Severity != "CRITICAL" {
		t.Fatalf("InspectShaderVariants (UberExplosion) failed: %v, res=%+v", err, uberRes)
	}
	t.Logf("Shader inspector (Explosion): variants=%d, isExplosion=%v, severity=%s, adviceCount=%d",
		uberRes.TotalVariants, uberRes.IsExplosion, uberRes.Severity, len(uberRes.StrippingAdvice))

	// ==========================================
	// Round 102: Test DetectDCCLockActivity & AutoLockDCCAsset
	// ==========================================
	dccCharDir := filepath.Join(tempAssetRepo, "Assets", "Characters")
	dccPropDir := filepath.Join(tempAssetRepo, "Assets", "Props")
	_ = os.MkdirAll(dccCharDir, 0755)
	_ = os.MkdirAll(dccPropDir, 0755)

	mayaModel := filepath.Join(dccCharDir, "Warrior.mb")
	mayaLock := filepath.Join(dccCharDir, "Warrior.mb.swp")
	_ = os.WriteFile(mayaModel, []byte("MAYA BINARY DUMMY DATA"), 0644)
	_ = os.WriteFile(mayaLock, []byte("MAYA SWAP LOCK"), 0644)

	blenderModel := filepath.Join(dccPropDir, "Chest.blend")
	blenderLock := filepath.Join(dccPropDir, "Chest.blend@")
	_ = os.WriteFile(blenderModel, []byte("BLENDER DUMMY DATA"), 0644)
	_ = os.WriteFile(blenderLock, []byte("BLENDER AUTO LOCK"), 0644)

	dccRes, err := svc.DetectDCCLockActivity(tempAssetRepo)
	if err != nil || dccRes.TotalDCCFiles < 2 {
		t.Fatalf("DetectDCCLockActivity failed: %v, res=%+v", err, dccRes)
	}
	t.Logf("DCC Lock Watcher: totalDCCFiles=%d, activeLocks=%d, summary=%s",
		dccRes.TotalDCCFiles, dccRes.ActiveLocksCount, dccRes.Summary)

	lockRes, err := svc.AutoLockDCCAsset(tempAssetRepo, "Assets/Characters/Warrior.mb")
	if err != nil {
		t.Fatalf("AutoLockDCCAsset encountered fatal err: %v", err)
	}
	t.Logf("Auto-lock result: success=%v, msg=%s", lockRes.Success, lockRes.Message)

	// ==========================================
	// Round 103: Test DiffUnrealAssetNodes
	// ==========================================
	bpDir := filepath.Join(tempAssetRepo, "Content", "Blueprints")
	_ = os.MkdirAll(bpDir, 0755)
	bpFile := filepath.Join(bpDir, "BP_Player.uasset")

	bpV1Content := `Begin Object Class=/Script/BlueprintGraph.K2Node_Event Name="ReceiveBeginPlay"
   NodePosX=100
   NodePosY=200
   Pin (PinName="Output",Direction="Output",PinType.PinCategory="Exec",LinkedTo=(CallFunc_1:Input))
End Object
Begin Object Class=/Script/BlueprintGraph.K2Node_CallFunction Name="CallFunc_1"
   NodePosX=300
   NodePosY=200
   FunctionReference=(MemberName="PrintString")
   Pin (PinName="Input",Direction="Input",PinType.PinCategory="Exec")
   Pin (PinName="InString",Direction="Input",PinType.PinCategory="String",DefaultValue="Hello World")
End Object`
	_ = os.WriteFile(bpFile, []byte(bpV1Content), 0644)
	_, _ = ExecGit(tempAssetRepo, "add", "Content/Blueprints/BP_Player.uasset")
	_, _ = ExecGit(tempAssetRepo, "commit", "-m", "feat: initial player blueprint")

	bpV2Content := `Begin Object Class=/Script/BlueprintGraph.K2Node_Event Name="ReceiveBeginPlay"
   NodePosX=100
   NodePosY=200
   Pin (PinName="Output",Direction="Output",PinType.PinCategory="Exec",LinkedTo=(CallFunc_1:Input))
End Object
Begin Object Class=/Script/BlueprintGraph.K2Node_CallFunction Name="CallFunc_1"
   NodePosX=300
   NodePosY=200
   FunctionReference=(MemberName="PrintString")
   Pin (PinName="Input",Direction="Input",PinType.PinCategory="Exec")
   Pin (PinName="InString",Direction="Input",PinType.PinCategory="String",DefaultValue="Updated Greeting")
End Object
Begin Object Class=/Script/BlueprintGraph.K2Node_CallFunction Name="CallFunc_ApplyDamage"
   NodePosX=500
   NodePosY=200
   FunctionReference=(MemberName="ApplyDamage")
   Pin (PinName="Input",Direction="Input",PinType.PinCategory="Exec")
End Object`
	_ = os.WriteFile(bpFile, []byte(bpV2Content), 0644)

	diffRes, err := svc.DiffUnrealAssetNodes(tempAssetRepo, "Content/Blueprints/BP_Player.uasset", "HEAD", "WORKTREE")
	if err != nil || len(diffRes.AddedNodes) != 1 || len(diffRes.ModifiedNodes) != 1 {
		t.Fatalf("DiffUnrealAssetNodes failed: %v, res=%+v", err, diffRes)
	}
	t.Logf("Blueprint AST Diff: added=%d, modified=%d, summary=%s",
		len(diffRes.AddedNodes), len(diffRes.ModifiedNodes), diffRes.VisualSummary)

	// ==========================================
	// Round 104: Test AuditSceneStreamMerge & ResolveSceneStreamConflict
	// ==========================================
	scenesDir := filepath.Join(tempAssetRepo, "Content", "Maps")
	_ = os.MkdirAll(scenesDir, 0755)
	levelFile := filepath.Join(scenesDir, "SubLevel_Arena.umap")

	levelContent := `Begin Actor Class=StaticMeshActor Name="Statue_Boss" ActorGuid="guid-boss-1001" Location=(X=0.0, Y=0.0, Z=0.0)
End Actor
Begin Actor Class=PointLight Name="Torch_1" ActorGuid="guid-torch-2002" Location=(X=10.0, Y=10.0, Z=5.0)
End Actor`
	_ = os.WriteFile(levelFile, []byte(levelContent), 0644)
	_, _ = ExecGit(tempAssetRepo, "add", "Content/Maps/SubLevel_Arena.umap")
	_, _ = ExecGit(tempAssetRepo, "commit", "-m", "feat: arena sublevel")

	sceneAuditRes, err := svc.AuditSceneStreamMerge(tempAssetRepo, "HEAD", "Content/Maps/SubLevel_Arena.umap")
	if err != nil || len(sceneAuditRes.AuditedSubLevels) == 0 {
		t.Fatalf("AuditSceneStreamMerge failed: %v, res=%+v", err, sceneAuditRes)
	}
	t.Logf("Scene stream audit: engine=%s, actors=%d, safetyScore=%d, verdict=%s",
		sceneAuditRes.EngineType, sceneAuditRes.TotalActorsCurrent, sceneAuditRes.SafetyScore, sceneAuditRes.MergeVerdict)

	resolveRes, err := svc.ResolveSceneStreamConflict(tempAssetRepo, "Content/Maps/SubLevel_Arena.umap", "AUTO_REGENERATE_GUID", "guid-boss-1001")
	if err != nil || !resolveRes.Success || resolveRes.NewActorGuid == "" {
		t.Fatalf("ResolveSceneStreamConflict failed: %v, res=%+v", err, resolveRes)
	}
	t.Logf("Scene conflict resolved: newGuid=%s, msg=%s", resolveRes.NewActorGuid, resolveRes.Message)

	// ==========================================
	// Round 105: Test LintAnimationRig
	// ==========================================
	fbxDir := filepath.Join(tempAssetRepo, "Assets", "Animations")
	_ = os.MkdirAll(fbxDir, 0755)
	heroFbx := filepath.Join(fbxDir, "Hero_Run.fbx")
	fbxHeader := []byte("Kaydara FBX Binary  \x00\x1a\x00\x18\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00Model::RootNode\x00Model::Pelvis\x00Model::Spine\x00Model::Thigh_L\x00Model::Thigh_R\x00AnimationCurveNode\x00AnimCurve\x00AnimCurve\x00AnimCurve\x00")
	_ = os.WriteFile(heroFbx, fbxHeader, 0644)

	rigRes, err := svc.LintAnimationRig(tempAssetRepo, "Assets/Animations/Hero_Run.fbx", "MOBILE")
	if err != nil || rigRes.BoneCount == 0 {
		t.Fatalf("LintAnimationRig failed: %v, res=%+v", err, rigRes)
	}
	t.Logf("Animation Rig Linter: bones=%d, tracks=%d, duration=%.1fs, rating=%s, budgetPass=%v",
		rigRes.BoneCount, rigRes.TotalTrackCount, rigRes.TotalDurationSeconds, rigRes.PerformanceRating, !rigRes.IsOverBudget)

	// ==========================================
	// Round 106: Test AuditLocalizationParity
	// ==========================================
	locDir := filepath.Join(tempAssetRepo, "Localization")
	_ = os.MkdirAll(locDir, 0755)
	zhFile := filepath.Join(locDir, "zh-CN.json")
	enFile := filepath.Join(locDir, "en-US.json")
	_ = os.WriteFile(zhFile, []byte(`{"ui.start": "开始游戏", "ui.quit": "退出游戏", "ui.hero": "英雄"}`), 0644)
	_ = os.WriteFile(enFile, []byte(`{"ui.start": "Start Game", "ui.quit": "Quit Game"}`), 0644)

	locRes, err := svc.AuditLocalizationParity(tempAssetRepo, "")
	if err != nil || locRes.TotalUniqueKeys == 0 {
		t.Fatalf("AuditLocalizationParity failed: %v, res=%+v", err, locRes)
	}
	t.Logf("Localization audit: totalKeys=%d, languages=%d, score=%.1f%%, missing=%d",
		locRes.TotalUniqueKeys, len(locRes.LanguagesFound), locRes.OverallParityScore, len(locRes.UntranslatedList))

	// ==========================================
	// Round 107: Test AuditNativePluginsAbi
	// ==========================================
	pluginDir := filepath.Join(tempAssetRepo, "Plugins", "Android", "libs", "arm64-v8a")
	_ = os.MkdirAll(pluginDir, 0755)
	dummySo := filepath.Join(pluginDir, "libgamecore.so")
	elfHeader := []byte("\x7fELF\x02\x01\x01\x00\x00\x00\x00\x00\x00\x00\x00\x00\x03\x00\xb7\x00\x01\x00\x00\x00")
	_ = os.WriteFile(dummySo, elfHeader, 0755)

	abiRes, err := svc.AuditNativePluginsAbi(tempAssetRepo)
	if err != nil || abiRes.TotalPluginsCount == 0 {
		t.Fatalf("AuditNativePluginsAbi failed: %v, res=%+v", err, abiRes)
	}
	t.Logf("Native Plugins ABI: plugins=%d, compliantCount=%d, criticalMismatch=%v",
		abiRes.TotalPluginsCount, abiRes.CompliantCount, abiRes.HasCriticalMismatch)

	// ==========================================
	// Round 109: Test GenerateHotfixManifest
	// ==========================================
	hotfixRes, err := svc.GenerateHotfixManifest(tempAssetRepo, "HEAD~1", "HEAD", "v1.2.0")
	if err != nil || hotfixRes.AppVersion != "v1.2.0" {
		t.Fatalf("GenerateHotfixManifest failed: %v, res=%+v", err, hotfixRes)
	}
	t.Logf("Hotfix manifest: version=%s, totalFiles=%d, totalBytes=%d, fullPackRequired=%v",
		hotfixRes.ManifestVersion, hotfixRes.TotalFilesCount, hotfixRes.TotalPatchBytes, hotfixRes.FullPackRequired)

	// ==========================================
	// Round 110: Test AnalyzePackageSizeBudget
	// ==========================================
	budgetRes, err := svc.AnalyzePackageSizeBudget(tempAssetRepo, "IOS")
	if err != nil || budgetRes.TotalSizeBytes == 0 {
		t.Fatalf("AnalyzePackageSizeBudget failed: %v, res=%+v", err, budgetRes)
	}
	t.Logf("Package size budget: total=%s, overrun=%v, worstPlatform=%s",
		budgetRes.TotalSizeStr, budgetRes.HasOverrun, budgetRes.WorstPlatform)

	// ==========================================
	// Round 111: Test ManageStudioSwitchboard
	// ==========================================
	switchboardRes, err := svc.ManageStudioSwitchboard(tempAssetRepo, "list", "", "", "")
	if err != nil || switchboardRes.TotalProjects == 0 {
		t.Fatalf("ManageStudioSwitchboard failed: %v, res=%+v", err, switchboardRes)
	}
	t.Logf("Studio switchboard: totalProjects=%d, active=%s",
		switchboardRes.TotalProjects, switchboardRes.ActiveProjectPath)

	// ==========================================
	// Round 112: Test AuditDataTableDiff
	// ==========================================
	tableDir := filepath.Join(tempAssetRepo, "Config")
	_ = os.MkdirAll(tableDir, 0755)
	heroCsv := filepath.Join(tableDir, "Hero.csv")
	_ = os.WriteFile(heroCsv, []byte("ID,Name,Atk,Def\n101,Warrior,120,50\n102,Mage,150,30\n"), 0644)
	_, _ = ExecGit(tempAssetRepo, "add", "Config/Hero.csv")
	_, _ = ExecGit(tempAssetRepo, "commit", "-m", "feat: base hero table")

	_ = os.WriteFile(heroCsv, []byte("ID,Name,Atk,Def,Speed\n101,Warrior,140,50,10\n103,Archer,130,40,15\n"), 0644)
	tableRes, err := svc.AuditDataTableDiff(tempAssetRepo, "", "HEAD", "")
	if err != nil || tableRes.TotalTables == 0 {
		t.Fatalf("AuditDataTableDiff failed: %v, res=%+v", err, tableRes)
	}
	t.Logf("Data table audit: tables=%d, criticals=%d, warnings=%d, valid=%v, summary=%s",
		tableRes.TotalTables, tableRes.CriticalCount, tableRes.WarningCount, tableRes.IsValid, tableRes.OverallSummary)

	// ==========================================
	// Round 113: Test InspectCredentialVault & StoreCredentialSecret
	// ==========================================
	storeSecRes, err := svc.StoreCredentialSecret(tempAssetRepo, "TENCENT_CLOUD_AKSK", "AKID_STUDIO_TEST_1", "SecretKeyVal9876543210ABCDEF", "LOCAL_REPO")
	if err != nil || !storeSecRes.Success || storeSecRes.MaskedValue == "" || storeSecRes.EncryptionMethod != "AES-256-GCM-KMS" {
		t.Fatalf("StoreCredentialSecret failed: %v, res=%+v", err, storeSecRes)
	}
	t.Logf("KMS Store Secret: ID=%s, Masked=%s, Algo=%s", storeSecRes.SecretID, storeSecRes.MaskedValue, storeSecRes.EncryptionMethod)

	vaultRes, err := svc.InspectCredentialVault(tempAssetRepo)
	if err != nil || vaultRes.TotalCredentials == 0 || !vaultRes.KmsEncryptionEnabled {
		t.Fatalf("InspectCredentialVault failed: %v, res=%+v", err, vaultRes)
	}
	t.Logf("Credential Vault: Total=%d, Secure=%d, KMS=%v, Summary=%s",
		vaultRes.TotalCredentials, vaultRes.SecureCount, vaultRes.KmsEncryptionEnabled, vaultRes.Summary)

	// ==========================================
	// Round 114: Test DemangleCrashStack
	// ==========================================
	crashSrcDir := filepath.Join(tempAssetRepo, "Source")
	_ = os.MkdirAll(crashSrcDir, 0755)
	heroSrcFile := filepath.Join(crashSrcDir, "Hero.cpp")
	_ = os.WriteFile(heroSrcFile, []byte("// Hero implementation\n#include \"Hero.h\"\nint Hero::CalculateDamage() {\n    return 100;\n}\n"), 0644)
	_, _ = ExecGit(tempAssetRepo, "add", "Source/Hero.cpp")
	_, _ = ExecGit(tempAssetRepo, "commit", "-m", "feat: hero combat logic")

	rawCrashDump := `Crash Reason: EXCEPTION_ACCESS_VIOLATION_READ (0x0000000000000008)
Faulting Module: GameCore.dll
Thread 0 (Crashed):
#0  0x0000000100003fa0 in ?CalculateDamage@Hero@@QEAAHXZ at Source/Hero.cpp:4
#1  0x00007ff6a1b2c340 in _ZN4Hero15CalculateDamageEv at Source/Hero.cpp:4
`
	demangleRes, err := svc.DemangleCrashStack(tempAssetRepo, rawCrashDump, "HEAD")
	if err != nil || demangleRes.DemangledCount < 2 || demangleRes.CulpritFile == "" {
		t.Fatalf("DemangleCrashStack failed: %v, res=%+v", err, demangleRes)
	}
	if demangleRes.Frames[0].DemangledSymbol != "Hero::CalculateDamage" || demangleRes.Frames[1].DemangledSymbol != "Hero::CalculateDamage" {
		t.Fatalf("Demangle symbol mismatch: frame0=%s, frame1=%s", demangleRes.Frames[0].DemangledSymbol, demangleRes.Frames[1].DemangledSymbol)
	}
	t.Logf("Crash Demangler: TotalFrames=%d, DemangledCount=%d, Culprit=%s (Author: %s, Commit: %s)",
		demangleRes.TotalFrames, demangleRes.DemangledCount, demangleRes.CulpritFile, demangleRes.CulpritAuthor, demangleRes.CulpritCommit)

	// ==========================================
	// Round 115: Test CherryPickAssetBundle
	// ==========================================
	origBranch, _ := ExecGit(tempAssetRepo, "branch", "--show-current")
	origBranch = strings.TrimSpace(origBranch)
	if origBranch == "" {
		origBranch = "master"
	}
	_, _ = ExecGit(tempAssetRepo, "checkout", "-b", "feature/art-assets")
	artMatDir := filepath.Join(tempAssetRepo, "Assets", "Materials")
	artTexDir := filepath.Join(tempAssetRepo, "Assets", "Textures")
	_ = os.MkdirAll(artMatDir, 0755)
	_ = os.MkdirAll(artTexDir, 0755)

	artTexFile := filepath.Join(artTexDir, "Hero_Albedo.png")
	artTexMeta := filepath.Join(artTexDir, "Hero_Albedo.png.meta")
	_ = os.WriteFile(artTexFile, []byte("DUMMY PNG DATA"), 0644)
	_ = os.WriteFile(artTexMeta, []byte("fileFormatVersion: 2\nguid: a1b2c3d4e5f60718293a4b5c6d7e8f90\nTextureImporter:\n"), 0644)

	artMatFile := filepath.Join(artMatDir, "Hero.mat")
	artMatMeta := filepath.Join(artMatDir, "Hero.mat.meta")
	artMatContent := `YAML
m_Shader: {fileID: 4800000, guid: 0000000000000000f000000000000000, type: 3}
m_SavedProperties:
  m_TexEnvs:
    - _MainTex:
        m_Texture: {fileID: 2800000, guid: a1b2c3d4e5f60718293a4b5c6d7e8f90, type: 3}
`
	_ = os.WriteFile(artMatFile, []byte(artMatContent), 0644)
	_ = os.WriteFile(artMatMeta, []byte("fileFormatVersion: 2\nguid: f0f1f2f3f4f5f6f7f8f9fa0b1c2d3e4f\n"), 0644)
	_, _ = ExecGit(tempAssetRepo, "add", "Assets")
	_, _ = ExecGit(tempAssetRepo, "commit", "-m", "feat: art hero material and albedo texture")

	// Switch back to original branch
	_, _ = ExecGit(tempAssetRepo, "checkout", origBranch)
	if _, err := os.Stat(artMatFile); err == nil {
		t.Fatalf("Hero.mat should not exist in %s branch before cherry-pick", origBranch)
	}

	cherryRes, err := svc.CherryPickAssetBundle(tempAssetRepo, "feature/art-assets", []string{"Assets/Materials/Hero.mat"}, true, false)
	if err != nil || !cherryRes.Success || cherryRes.TotalAssetsCount < 4 {
		t.Fatalf("CherryPickAssetBundle failed: %v, res=%+v", err, cherryRes)
	}
	t.Logf("Selective Asset Cherry-Pick: Total=%d (Requested: %d, Dep: %d), Summary=%s",
		cherryRes.TotalAssetsCount, cherryRes.TotalRequested, cherryRes.TotalDependencies, cherryRes.Summary)

	// Verify both Material, Texture, and their .meta files exist on disk
	if _, err := os.Stat(artMatFile); err != nil {
		t.Fatalf("Cherry-picked material not found on disk: %v", err)
	}
	if _, err := os.Stat(artTexFile); err != nil {
		t.Fatalf("Cherry-picked dependent texture not found on disk: %v", err)
	}

	// ==========================================
	// Round 116: Test ScanMissingMaterials
	// ==========================================
	pinkMatFile := filepath.Join(artMatDir, "BrokenPink.mat")
	pinkMatMeta := filepath.Join(artMatDir, "BrokenPink.mat.meta")
	pinkContent := `YAML
m_Shader: {fileID: 4800000, guid: e1e2e3e4e5e6e7e8e9e0e1e2e3e4e5e6, type: 3}
m_ShaderName: "Hidden/InternalErrorShader"
m_SavedProperties:
  m_TexEnvs:
    - _BumpMap:
        m_Texture: {fileID: 2800000, guid: 99999999999999999999999999999999, type: 3}
`
	_ = os.WriteFile(pinkMatFile, []byte(pinkContent), 0644)
	_ = os.WriteFile(pinkMatMeta, []byte("fileFormatVersion: 2\nguid: d1d2d3d4d5d6d7d8d9d0d1d2d3d4d5d6\n"), 0644)

	scanRes, err := svc.ScanMissingMaterials(tempAssetRepo, "Assets")
	if err != nil || scanRes.PinkShaderCount == 0 || scanRes.BrokenMaterialsCount == 0 {
		t.Fatalf("ScanMissingMaterials failed: %v, res=%+v", err, scanRes)
	}
	t.Logf("Pink Shader Guard: Scanned=%d, Broken=%d, PinkCount=%d, MissingTex=%d, Score=%d",
		scanRes.TotalMaterialsScanned, scanRes.BrokenMaterialsCount, scanRes.PinkShaderCount, scanRes.MissingTexturesCount, scanRes.HealthScore)

	// ==========================================
	// Round 117: Test ManageOfflineSubmitQueue
	// ==========================================
	qRes1, err := svc.ManageOfflineSubmitQueue(tempAssetRepo, "enqueue", "", `{"operation":"push","target":"main"}`)
	if err != nil || qRes1.TotalCount != 1 || qRes1.QueuedCount != 1 {
		t.Fatalf("ManageOfflineSubmitQueue enqueue failed: %v, res=%+v", err, qRes1)
	}

	qRes2, err := svc.ManageOfflineSubmitQueue(tempAssetRepo, "pause", "all", "")
	if err != nil || qRes2.PausedCount != 1 {
		t.Fatalf("ManageOfflineSubmitQueue pause failed: %v, res=%+v", err, qRes2)
	}

	qRes3, err := svc.ManageOfflineSubmitQueue(tempAssetRepo, "resume", "all", "")
	if err != nil || qRes3.QueuedCount != 1 {
		t.Fatalf("ManageOfflineSubmitQueue resume failed: %v, res=%+v", err, qRes3)
	}

	qRes4, err := svc.ManageOfflineSubmitQueue(tempAssetRepo, "replay", "all", "")
	if err != nil {
		t.Fatalf("ManageOfflineSubmitQueue replay failed: %v", err)
	}
	t.Logf("Offline queue: total=%d, queued=%d, failed=%d, msg=%s",
		qRes4.TotalCount, qRes4.QueuedCount, qRes4.FailedCount, qRes4.Message)

	qRes5, err := svc.ManageOfflineSubmitQueue(tempAssetRepo, "discard", "all", "")
	if err != nil || qRes5.TotalCount != 0 {
		t.Fatalf("ManageOfflineSubmitQueue discard failed: %v, res=%+v", err, qRes5)
	}

	// ==========================================
	// Round 118: Test AuditPrePushChecklist
	// ==========================================
	checklistRes, err := svc.AuditPrePushChecklist(tempAssetRepo, origBranch)
	if err != nil || checklistRes.PassReadinessScore == 0 {
		t.Fatalf("AuditPrePushChecklist failed: %v, res=%+v", err, checklistRes)
	}
	t.Logf("Pre-Push Checklist: score=%d, grade=%s, blocked=%v, rules=%d, summary=%s",
		checklistRes.PassReadinessScore, checklistRes.ReadinessGrade, checklistRes.IsBlocked, len(checklistRes.RulesChecked), checklistRes.Summary)

	// ==========================================
	// Round 119: Test AuditLodStreaming
	// ==========================================
	lodDir := filepath.Join(tempAssetRepo, "Assets", "LODTest")
	_ = os.MkdirAll(lodDir, 0755)

	objMeshPath := filepath.Join(lodDir, "BossMonster.obj")
	var objLines []string
	objLines = append(objLines, "# Wavefront OBJ test")
	for i := 1; i <= 200; i++ {
		objLines = append(objLines, fmt.Sprintf("v %.2f %.2f %.2f", float64(i), float64(i*2), 0.0))
	}
	for i := 1; i <= 25000; i++ {
		objLines = append(objLines, fmt.Sprintf("f %d %d %d", (i%190)+1, (i%190)+2, (i%190)+3))
	}
	_ = os.WriteFile(objMeshPath, []byte(strings.Join(objLines, "\n")), 0644)

	// 128-byte DDS test header (POT 1024x1024, 11 mipmaps)
	ddsPath := filepath.Join(lodDir, "Boss_Diffuse.dds")
	ddsHeader := make([]byte, 128)
	copy(ddsHeader[0:4], []byte("DDS "))
	binary.LittleEndian.PutUint32(ddsHeader[12:16], 1024) // height
	binary.LittleEndian.PutUint32(ddsHeader[16:20], 1024) // width
	binary.LittleEndian.PutUint32(ddsHeader[28:32], 11)   // mipmaps
	_ = os.WriteFile(ddsPath, ddsHeader, 0644)

	lodAuditRes, err := svc.AuditLodStreaming(tempAssetRepo, "Assets/LODTest")
	if err != nil || lodAuditRes.TotalMeshesAudited == 0 || lodAuditRes.TotalTexturesAudited == 0 {
		t.Fatalf("AuditLodStreaming failed: %v, res=%+v", err, lodAuditRes)
	}
	t.Logf("LOD & Streaming Audit: meshes=%d, textures=%d, baselineVram=%.2fMB, streamingVram=%.2fMB, savings=%.1f%%, score=%d",
		lodAuditRes.TotalMeshesAudited, lodAuditRes.TotalTexturesAudited,
		lodAuditRes.BaselineVramMB, lodAuditRes.StreamingVramMB,
		lodAuditRes.SavingsPercentage, lodAuditRes.OverallHealthScore)

	// ==========================================
	// Round 120: Test GetFullParityStatusSummary
	// ==========================================
	parityHubRes, err := svc.GetFullParityStatusSummary(tempAssetRepo)
	if err != nil || parityHubRes.CompletedRounds != 120 || parityHubRes.ParityPercentage != 100.0 {
		t.Fatalf("GetFullParityStatusSummary failed: %v, res=%+v", err, parityHubRes)
	}
	t.Logf("Grand Parity Hub: rounds=%d/%d (%.1f%%), badge=%s, readiness=%s, domains=%d, tests=%d",
		parityHubRes.CompletedRounds, parityHubRes.TotalRounds, parityHubRes.ParityPercentage,
		parityHubRes.CertificationBadge, parityHubRes.SystemReadiness,
		len(parityHubRes.Domains), parityHubRes.TotalTestCoverageCount)

	// ==========================================
	// ==========================================
	// ==========================================

	// Round 121: Test AuditAudioBankDiff
	// ==========================================
	{
		r121AudioDir := filepath.Join(tempAssetRepo, "Audio", "Wwise")
		_ = os.MkdirAll(r121AudioDir, 0755)
		sampleBnk := filepath.Join(r121AudioDir, "Weapons.bnk")
		_ = os.WriteFile(sampleBnk, []byte("BKHD\x00\x00\x00\x10AKBK\x00\x00\x00\x00Event_Shoot_Ak47\x00Event_Reload\x00"), 0644)
		res, err := svc.AuditAudioBankDiff(tempAssetRepo, "Audio/Wwise/Weapons.bnk", "WORKTREE", "WORKTREE")
		if err != nil || res == nil {
			t.Fatalf("AuditAudioBankDiff failed: %v", err)
		}
		t.Logf("AudioBank Diff: engine=%s, bank=%s, added=%d, mod=%d, summary=%s",
			res.EngineType, res.BankName, len(res.AddedEvents), len(res.ModifiedEvents), res.Summary)
	}

	// ==========================================
	// Round 122: Test AuditPhysicsMeshCollision
	// ==========================================
	{
		r122PhysDir := filepath.Join(tempAssetRepo, "Assets", "PhysicsTest")
		_ = os.MkdirAll(r122PhysDir, 0755)
		physMatPath := filepath.Join(r122PhysDir, "IceFriction.physicMaterial")
		_ = os.WriteFile(physMatPath, []byte("%YAML 1.1\nPhysicMaterial:\nm_DynamicFriction: 0.05\nm_StaticFriction: 0.05\n"), 0644)
		res, err := svc.AuditPhysicsMeshCollision(tempAssetRepo, "Assets/PhysicsTest")
		if err != nil || res == nil {
			t.Fatalf("AuditPhysicsMeshCollision failed: %v", err)
		}
		t.Logf("Physics Mesh Collision: totalColliders=%d, convexCount=%d, score=%d",
			res.TotalColliders, res.ConvexHullsCount, res.HealthScore)
	}

	// ==========================================
	// Round 123: Test DiffWorldDataLayers
	// ==========================================
	{
		dataLayerPath := filepath.Join(tempAssetRepo, "WorldPartition", "DataLayer_Cinematics.asset")
		_ = os.MkdirAll(filepath.Dir(dataLayerPath), 0755)
		_ = os.WriteFile(dataLayerPath, []byte("DataLayerInstance: CinematicLayer\nm_InitialState: Activated\n"), 0644)
		res, err := svc.DiffWorldDataLayers(tempAssetRepo, "HEAD", "HEAD")
		if err != nil || res == nil {
			t.Fatalf("DiffWorldDataLayers failed: %v", err)
		}
		t.Logf("World Data Layers Diff: totalLayers=%d, conflicts=%d, summary=%s",
			res.TotalDataLayers, len(res.StreamingConflicts), res.Summary)
	}

	// ==========================================
	// Round 124: Test AuditMaterialInstanceOverrides
	// ==========================================
	{
		r124MatDir := filepath.Join(tempAssetRepo, "Assets", "Materials")
		_ = os.MkdirAll(r124MatDir, 0755)
		matInstPath := filepath.Join(r124MatDir, "Hero_Armor_Inst.mat")
		_ = os.WriteFile(matInstPath, []byte("%YAML 1.1\nMaterial:\nm_Shader: {guid: 1234}\nm_SavedProperties:\n  m_TexEnvs:\n    - _MainTex: {guid: hero_alb}\n"), 0644)
		res, err := svc.AuditMaterialInstanceOverrides(tempAssetRepo, "Assets/Materials")
		if err != nil || res == nil {
			t.Fatalf("AuditMaterialInstanceOverrides failed: %v", err)
		}
		t.Logf("Material Instance Overrides: instances=%d, overridden=%d, dead=%d",
			res.TotalInstances, res.OverriddenParametersCount, res.DeadParametersCount)
	}

	// ==========================================
	// Round 125: Test AuditSymbolsAndDefines
	// ==========================================
	{
		rspPath := filepath.Join(tempAssetRepo, "csc.rsp")
		_ = os.WriteFile(rspPath, []byte("-define:DEBUG_LOG;ENABLE_PROFILER;UNIT_TEST\n"), 0644)
		res, err := svc.AuditSymbolsAndDefines(tempAssetRepo, "HEAD", "HEAD")
		if err != nil || res == nil {
			t.Fatalf("AuditSymbolsAndDefines failed: %v", err)
		}
		t.Logf("Symbols & Defines: total=%d, active=%d, dangerous=%d, severity=%s",
			res.TotalDefines, len(res.ActiveDefines), len(res.DangerousDebugDefines), res.Severity)
	}

	// ==========================================
	// Round 126: Test DetectAssetCyclicDependencies
	// ==========================================
	{
		r126CycleDir := filepath.Join(tempAssetRepo, "Assets", "Cycles")
		_ = os.MkdirAll(r126CycleDir, 0755)
		prefabA := filepath.Join(r126CycleDir, "PrefabA.prefab")
		prefabB := filepath.Join(r126CycleDir, "PrefabB.prefab")
		_ = os.WriteFile(prefabA, []byte("PrefabA guid: 11111111111111111111111111111111 ref: 22222222222222222222222222222222\n"), 0644)
		_ = os.WriteFile(prefabB, []byte("PrefabB guid: 22222222222222222222222222222222 ref: 11111111111111111111111111111111\n"), 0644)
		res, err := svc.DetectAssetCyclicDependencies(tempAssetRepo, "Assets/Cycles")
		if err != nil || res == nil {
			t.Fatalf("DetectAssetCyclicDependencies failed: %v", err)
		}
		t.Logf("Asset Cyclic Dependencies: scanned=%d, cyclesCount=%d, hasCycles=%v",
			res.TotalAssetsScanned, res.CyclesCount, res.HasCycles)
	}

	// ==========================================
	// Round 127: Test AuditTextureChannelPacking
	// ==========================================
	{
		r127TexDir := filepath.Join(tempAssetRepo, "Assets", "Textures")
		_ = os.MkdirAll(r127TexDir, 0755)
		ormPath := filepath.Join(r127TexDir, "Character_Hero_ORM.png")
		_ = os.WriteFile(ormPath, []byte("fake_orm_png_bytes"), 0644)
		ormMeta := filepath.Join(r127TexDir, "Character_Hero_ORM.png.meta")
		_ = os.WriteFile(ormMeta, []byte("sRGBTexture: 0\ntextureType: 0\n"), 0644)
		res, err := svc.AuditTextureChannelPacking(tempAssetRepo, "Assets/Textures")
		if err != nil || res == nil {
			t.Fatalf("AuditTextureChannelPacking failed: %v", err)
		}
		t.Logf("Texture Channel Packing: scanned=%d, ormCompliant=%d, unpacked=%d",
			res.TotalTexturesScanned, res.OrmCompliantCount, res.UnpackedTexturesCount)
	}

	// ==========================================
	// Round 128: Test ManageStudioWebhooks
	// ==========================================
	{
		resList, err := svc.ManageStudioWebhooks(tempAssetRepo, "list", "", "", "")
		if err != nil || resList == nil {
			t.Fatalf("ManageStudioWebhooks list failed: %v", err)
		}
		resAdd, err := svc.ManageStudioWebhooks(tempAssetRepo, "add", "WECOM", "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=mock", "COMMIT")
		if err != nil || resAdd == nil {
			t.Fatalf("ManageStudioWebhooks add failed: %v", err)
		}
		t.Logf("Studio Webhooks: total=%d, success=%v, msg=%s",
			resAdd.TotalWebhooks, resAdd.Success, resAdd.Message)
	}

	// ==========================================
	// Round 129: Test ManageStudioDDC
	// ==========================================
	{
		res, err := svc.ManageStudioDDC(tempAssetRepo, "status")
		if err != nil || res == nil {
			t.Fatalf("ManageStudioDDC failed: %v", err)
		}
		t.Logf("Studio DDC: localCache=%s, sharedPath=%s, isConnected=%v, hitRatio=%.2f, latency=%dms",
			res.LocalCachePath, res.SharedCachePath, res.IsSharedConnected, res.CacheHitRatio, res.LatencyMs)
	}

	// ==========================================
	// Round 130: Test AnalyzeBundleDiff
	// ==========================================
	{
		catV1 := filepath.Join(tempAssetRepo, "catalog_v1.json")
		catV2 := filepath.Join(tempAssetRepo, "catalog_v2.json")
		_ = os.WriteFile(catV1, []byte(`{"version":"1.0","bundles":[{"name":"ui_core","size":1048576,"hash":"aaa111"}]}`), 0644)
		_ = os.WriteFile(catV2, []byte(`{"version":"1.1","bundles":[{"name":"ui_core","size":1200000,"hash":"bbb222"},{"name":"hero_bundle","size":500000,"hash":"ccc333"}]}`), 0644)
		res, err := svc.AnalyzeBundleDiff(tempAssetRepo, "catalog_v1.json", "catalog_v2.json")
		if err != nil || res == nil {
			t.Fatalf("AnalyzeBundleDiff failed: %v", err)
		}
		t.Logf("Bundle Diff: base=%d, target=%d, changed=%d, initialDelta=%s, patchBytes=%s",
			res.TotalBundlesBase, res.TotalBundlesTarget, len(res.ChangedBundles), res.InitialInstallDeltaStr, res.PatchDownloadStr)
	}

	// ==========================================
	// Round 131: Test GenerateOfflinePatchBundle
	// ==========================================
	{
		res, err := svc.GenerateOfflinePatchBundle(tempAssetRepo, "HEAD~1", "HEAD", filepath.Join(t.TempDir(), "offline_patch.wgitbundle"))
		if err != nil || res == nil {
			t.Fatalf("GenerateOfflinePatchBundle failed: %v", err)
		}
		t.Logf("Offline Patch Bundle: path=%s, commits=%d, lfs=%d, totalSize=%s, success=%v",
			res.BundlePath, res.CommitsCount, res.LfsBlobsCount, res.TotalSizeStr, res.Success)
	}

	// ==========================================
	// Round 132: Test AuditLightmapBakeSettings
	// ==========================================
	{
		r132LightingDir := filepath.Join(tempAssetRepo, "Assets", "Lighting")
		_ = os.MkdirAll(r132LightingDir, 0755)
		lightSettingPath := filepath.Join(r132LightingDir, "MainLevelLighting.lighting")
		_ = os.WriteFile(lightSettingPath, []byte("LightingSettings:\nm_LightmapResolution: 32\nm_Bounces: 3\nm_LightmapMaxSize: 1024\n"), 0644)
		res, err := svc.AuditLightmapBakeSettings(tempAssetRepo, "Assets/Lighting")
		if err != nil || res == nil {
			t.Fatalf("AuditLightmapBakeSettings failed: %v", err)
		}
		t.Logf("Lightmap Bake Settings: totalMeshes=%d, overlapUv2=%d, score=%d",
			res.TotalMeshes, res.OverlappingUv2Count, res.HealthScore)
	}

	// ==========================================
	// Round 133: Test AuditPrivacyAndCompliance
	// ==========================================
	{
		r133PrivDir := filepath.Join(tempAssetRepo, "App")
		_ = os.MkdirAll(r133PrivDir, 0755)
		iosPriv := filepath.Join(r133PrivDir, "PrivacyInfo.xcprivacy")
		_ = os.WriteFile(iosPriv, []byte(`<?xml version="1.0" encoding="UTF-8"?><plist version="1.0"><dict><key>NSPrivacyTracking</key><false/><key>NSPrivacyAccessedAPITypes</key><array><dict><key>NSPrivacyAccessedAPIType</key><string>NSPrivacyAccessedAPICategoryUserDefaults</string><key>NSPrivacyAccessedAPITypeReasons</key><array><string>CA92.1</string></array></dict></array></dict></plist>`), 0644)
		res, err := svc.AuditPrivacyAndCompliance(tempAssetRepo)
		if err != nil || res == nil {
			t.Fatalf("AuditPrivacyAndCompliance failed: %v", err)
		}
		t.Logf("Privacy Compliance: platform=%s, permissions=%d, sensitive=%d, grade=%s, ready=%v",
			res.Platform, res.TotalPermissionsDeclared, res.SensitivePermissionsCount, res.StoreReadinessGrade, res.IsStoreReady)
	}

	// ==========================================
	// Round 134: Test AuditFontLicenses
	// ==========================================
	{
		r134FontDir := filepath.Join(tempAssetRepo, "Assets", "Fonts")
		_ = os.MkdirAll(r134FontDir, 0755)
		fontPath := filepath.Join(r134FontDir, "GameFont_OFL.ttf")
		_ = os.WriteFile(fontPath, []byte("\x00\x01\x00\x00\x00\x01\x00\x00\x00\x00\x00\x00name\x00\x00\x00\x00\x00\x00\x00\x20SIL Open Font License Source Han Sans\n"), 0644)
		res, err := svc.AuditFontLicenses(tempAssetRepo)
		if err != nil || res == nil {
			t.Fatalf("AuditFontLicenses failed: %v", err)
		}
		t.Logf("Font License Audit: total=%d, safe=%d, restricted=%d, riskScore=%s",
			res.TotalFonts, res.CommercialSafeCount, res.RestrictedFontsCount, res.LegalRiskScore)
	}

	// ==========================================
	// Round 135: Missing hotfix branches must fail instead of returning fake 3/3
	// ==========================================
	{
		if _, err := svc.TraceHotfixBackportStatus(tempAssetRepo, "hotfix/does-not-exist", "main"); err == nil {
			t.Fatal("TraceHotfixBackportStatus must fail for a missing hotfix branch")
		}
	}

	// ==========================================
	// Round 136: Test AuditCommitContentSafety
	// ==========================================
	{
		res, err := svc.AuditCommitContentSafety(tempAssetRepo, 5)
		if err != nil || res == nil {
			t.Fatalf("AuditCommitContentSafety failed: %v", err)
		}
		t.Logf("Commit Content Safety: audited=%d, violations=%d, passed=%v, summary=%s",
			res.CommitsAudited, res.ViolationsCount, res.Passed, res.Summary)
	}

	// ==========================================

	// ==========================================
	// Test Project Source Code Browser
	// ==========================================
	{
		testFile := filepath.Join(tempAssetRepo, "main.go")
		_ = os.WriteFile(testFile, []byte("package main\n\nfunc main() {\n\tprintln(\"hello world\")\n}\n"), 0644)
		_, _ = ExecGit(tempAssetRepo, "add", "main.go")
		_, _ = ExecGit(tempAssetRepo, "commit", "-m", "feat: add main.go for source browser test")

		files, err := svc.GetProjectSourceFiles(tempAssetRepo)
		if err != nil || len(files) == 0 {
			t.Fatalf("GetProjectSourceFiles failed: %v", err)
		}
		t.Logf("Project source files count: %d", len(files))

		detail, err := svc.GetSourceFileDetail(tempAssetRepo, "main.go", "HEAD")
		if err != nil || detail.Language != "Go" || detail.LineCount < 4 {
			t.Fatalf("GetSourceFileDetail failed: %v, detail=%+v", err, detail)
		}
		t.Logf("Source file detail: path=%s, lang=%s, lines=%d, size=%s",
			detail.Path, detail.Language, detail.LineCount, detail.SizeFormatted)

		// Test GetSourceFileDiff
		diffStr, err := svc.GetSourceFileDiff(tempAssetRepo, "main.go", "HEAD")
		if err != nil {
			t.Fatalf("GetSourceFileDiff failed: %v", err)
		}
		t.Logf("GetSourceFileDiff: len=%d", len(diffStr))
	}

	// Round 139: Test ShaderSelfHealingWizard
	// ==========================================
	{
		res, err := svc.ShaderSelfHealingWizard(tempAssetRepo, "diagnose")
		if err != nil || res == nil {
			t.Fatalf("ShaderSelfHealingWizard failed: %v", err)
		}
		t.Logf("Shader Self-Healing Wizard: engine=%s, broken=%d, blackMeshes=%d, grade=%s, success=%v",
			res.EngineType, res.BrokenShaderCount, res.BlackRenderMeshesCount, res.HealthGrade, res.Success)
	}

	// ==========================================
	// Round 140: Test GetPinnacleParityStatusSummary
	// ==========================================
	{
		res, err := svc.GetPinnacleParityStatusSummary(tempAssetRepo)
		if err != nil || res.CompletedRounds != 140 || res.ParityPercentage != 100.0 {
			t.Fatalf("GetPinnacleParityStatusSummary failed: %v, res=%+v", err, res)
		}
		t.Logf("Grand Pinnacle Master Parity Hub: rounds=%d/%d (%.1f%%), badge=%s, readiness=%s, domains=%d, tests=%d, tools=%d",
			res.CompletedRounds, res.TotalRounds, res.ParityPercentage,
			res.CertificationBadge, res.SystemReadiness,
			len(res.Domains), res.TotalTestCoverageCount, res.TotalToolsCount)
	}

}
