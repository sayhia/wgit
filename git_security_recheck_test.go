package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// git_security_recheck_test.go 安全复查回归测试（config.go / git_ai_service.go
// / git_asset_service.go 的 Wails RPC 复查）。每个用例钉死复查表中一个已修复
// 风险点或一条"无需修复"结论（结论用例写明理由，防止后续回归悄悄推翻）。

// =========================================================================
// 修复点 1：.wgit/assets.json 并发写（SetAssetState 丢更新 / 半截文件）
// =========================================================================

// TestSecurityAssetStatesConcurrentSetNoLostUpdate 复现两个窗口并发调用
// SetAssetState 的场景：修复前两个调用各自读到旧状态表、各自整文件重写，
// 后写者会覆盖先写者的更新（丢更新）；修复后读-改-写被 assetStatesMu
// 序列化，两个并发写入的状态必须同时存活。
func TestSecurityAssetStatesConcurrentSetNoLostUpdate(t *testing.T) {
	realGit := assetBoardLookupGit(t)
	repo := newAssetBoardRepo(t, realGit)
	installAssetLockStub(t, realGit, "[]")
	svc := AssetService{}

	for round := 0; round < 3; round++ {
		// 每轮换一对资产与目标状态，放大读写交叉窗口。
		pairs := [][2]string{
			{"Assets/Textures/hero.png", assetStateApproved},
			{"Assets/Audio/bgm.wav", assetStateReview},
		}
		errs := make([]error, len(pairs))
		var wg sync.WaitGroup
		for i, pair := range pairs {
			wg.Add(1)
			go func(i int, path, state string) {
				defer wg.Done()
				errs[i] = svc.SetAssetState(repo, path, state)
			}(i, pair[0], pair[1])
		}
		wg.Wait()
		for i, err := range errs {
			if err != nil {
				t.Fatalf("round %d SetAssetState(%s) failed: %v", round, pairs[i][0], err)
			}
		}
	}

	states, err := svc.GetAssetStates(repo)
	if err != nil {
		t.Fatalf("GetAssetStates failed: %v", err)
	}
	if states["Assets/Textures/hero.png"] != assetStateApproved {
		t.Errorf("hero.png state = %q, want %q (concurrent write was lost)",
			states["Assets/Textures/hero.png"], assetStateApproved)
	}
	if states["Assets/Audio/bgm.wav"] != assetStateReview {
		t.Errorf("bgm.wav state = %q, want %q (concurrent write was lost)",
			states["Assets/Audio/bgm.wav"], assetStateReview)
	}
	if len(states) != 2 {
		t.Errorf("unexpected extra states: %+v", states)
	}
}

// TestSecurityAssetStatesConcurrentWriteNoTornFile 在并发写的同时开读方
// （GetAssetStates / GetAssetBoard）持续读取：原子替换保证读方永远看到完整
// 的旧文件或新文件；写完后 assets.json 必须可解析、条目齐全，且 .wgit 目录
// 不残留任何临时文件。
func TestSecurityAssetStatesConcurrentWriteNoTornFile(t *testing.T) {
	realGit := assetBoardLookupGit(t)
	repo := newAssetBoardRepo(t, realGit)
	installAssetLockStub(t, realGit, "[]")
	svc := AssetService{}

	paths := []string{
		"Assets/Textures/hero.png",
		"Assets/Textures/icon.png",
		"Assets/Models/hero.uasset",
		"Assets/Audio/bgm.wav",
	}
	var wg sync.WaitGroup
	errCh := make(chan error, len(paths)*2)
	for _, p := range paths {
		for _, st := range []string{assetStateReview, assetStateApproved} {
			wg.Add(1)
			go func(path, state string) {
				defer wg.Done()
				errCh <- svc.SetAssetState(repo, path, state)
			}(p, st)
		}
	}
	readStop := make(chan struct{})
	var readWg sync.WaitGroup
	readWg.Add(2)
	go func() {
		defer readWg.Done()
		for {
			select {
			case <-readStop:
				return
			default:
				_, _ = svc.GetAssetStates(repo)
			}
		}
	}()
	go func() {
		defer readWg.Done()
		for {
			select {
			case <-readStop:
				return
			default:
				_, _ = svc.GetAssetBoard(repo, 10)
			}
		}
	}()
	wg.Wait()
	close(readStop)
	readWg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatalf("concurrent SetAssetState failed: %v", err)
		}
	}

	raw, err := os.ReadFile(assetStatesPath(repo))
	if err != nil {
		t.Fatalf("read assets.json after concurrent writes: %v", err)
	}
	var parsed map[string]assetStateRecord
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("assets.json is torn/invalid after concurrent writes: %v (%s)", err, raw)
	}
	if len(parsed) != len(paths) {
		t.Errorf("assets.json holds %d entries, want %d: %s", len(parsed), len(paths), raw)
	}
	entries, err := os.ReadDir(filepath.Join(repo, assetStatesDirName))
	if err != nil {
		t.Fatalf("read .wgit dir: %v", err)
	}
	for _, e := range entries {
		if e.Name() != assetStatesFile {
			t.Errorf("leftover temp file in .wgit after atomic save: %s", e.Name())
		}
	}
}

// TestSecurityGetAssetStatesEmptyRepoPathRejected：空 repoPath 会拼出进程
// 工作目录下的相对路径 .wgit/assets.json（读错位置），必须与 GetAssetBoard /
// SetAssetState 一致地显式拒绝。
func TestSecurityGetAssetStatesEmptyRepoPathRejected(t *testing.T) {
	svc := AssetService{}
	for _, empty := range []string{"", "   \t"} {
		states, err := svc.GetAssetStates(empty)
		if err == nil {
			t.Fatalf("GetAssetStates(%q) succeeded with %+v, want an empty-repoPath error", empty, states)
		}
		if !strings.Contains(err.Error(), "repoPath") {
			t.Errorf("GetAssetStates(%q) error = %v, want a repoPath mention", empty, err)
		}
	}
}

// =========================================================================
// 修复点 2：SaveConfig 原子化（SaveAppFlags / SaveAIConfig 并发写 config.json）
// =========================================================================

// TestSecuritySaveConfigConcurrentWritersStayValid：修复前 SaveConfig 用
// os.WriteFile 直接整文件重写，两个并发保存者交错写会留下撕裂的 JSON；
// 修复后（tmp+rename）并发写后 config.json 必须始终完整可解析，且目录不
// 残留临时文件。读-改-写层面的丢更新（两个保存者各自覆盖）在桌面单用户
// 场景按"后写者为准"语义接受，本用例只钉死"文件永不撕裂"。
func TestSecuritySaveConfigConcurrentWritersStayValid(t *testing.T) {
	writeFeatureFlagsConfigFile(t, `{"theme":"dark","schemaVersion":1}`)

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			cfg := LoadConfig()
			cfg.Flags = FeatureFlags{AICommitMsg: i%2 == 0, AISummary: i%3 == 0}
			SaveConfig(cfg)
		}(i)
	}
	wg.Wait()

	raw, err := os.ReadFile(getConfigPath())
	if err != nil {
		t.Fatalf("read config.json after concurrent saves: %v", err)
	}
	var cfg AppConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("config.json torn after concurrent saves: %v (%s)", err, raw)
	}
	if cfg.SchemaVersion != CurrentSchemaVersion {
		t.Errorf("schemaVersion = %d, want %d (config.json must stay well-formed)", cfg.SchemaVersion, CurrentSchemaVersion)
	}
	dir := filepath.Dir(getConfigPath())
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read config dir: %v", err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp-") {
			t.Errorf("leftover temp file next to config.json: %s", e.Name())
		}
	}
}

// =========================================================================
// 结论用例：SaveAppFlags 的 bool 输入无需白名单校验
// =========================================================================

// TestSecuritySaveAppFlagsEveryBoolCombinationRoundTrips：FeatureFlags 的
// 全部字段均为 bool——bool 不存在"非法值"，非 bool 类型在 JSON 反序列化
// 阶段即被拒绝，任何 true/false 组合都是合法输入，因此 SaveAppFlags 无需
// 额外的合法性校验。该用例遍历全部 32 种组合把这一结论钉死为回归。
func TestSecuritySaveAppFlagsEveryBoolCombinationRoundTrips(t *testing.T) {
	writeFeatureFlagsConfigFile(t, `{"theme":"dark","schemaVersion":1}`)

	for mask := 0; mask < 32; mask++ {
		want := FeatureFlags{
			AICommitMsg:     mask&1 != 0,
			AISummary:       mask&2 != 0,
			AIConflictBatch: mask&4 != 0,
			ReviewFlow:      mask&8 != 0,
			AssetBoard:      mask&16 != 0,
		}
		if err := (GitService{}).SaveAppFlags(want); err != nil {
			t.Fatalf("SaveAppFlags(%+v) error = %v", want, err)
		}
		if got := (GitService{}).GetAppFlags(); got != want {
			t.Fatalf("mask %d: GetAppFlags after save = %+v, want %+v", mask, got, want)
		}
	}
}

// =========================================================================
// 修复点 3：aiHTTPCall 请求构造错误未脱敏（URL 内嵌凭证回显）
// =========================================================================

// TestSecurityAIRequestErrorRedactsEmbeddedCredentials：BaseURL 带内嵌 userinfo
// 凭证且无法解析（host 含空格）时，url.Parse 的错误文本会原样回显完整 URL，
// 而该错误会被前端渲染。修复后 aiHTTPCall 在返回前经 redactLogSecrets 脱敏，
// 凭证不得出现在错误文本中。
func TestSecurityAIRequestErrorRedactsEmbeddedCredentials(t *testing.T) {
	writeFeatureFlagsConfigFile(t, `{"theme":"dark"}`)
	resetAISessionKeyForTest(t)

	svc := AiService{}
	// host 中的空格使 url.Parse 失败：错误消息是 `parse "<原始URL>": ...`，
	// 含完整原始 URL（含凭证）。SaveAISettings now rejects unparseable URLs at
	// the boundary, so seed the config directly to exercise aiHTTPCall.
	bad := "http://user:" + aiTestURLSecret + "@ho st/v1"
	cfg := LoadConfig()
	cfg.AI = AIConfig{Mode: "api", BaseURL: bad, Model: "m", TimeoutSeconds: 5}
	cfg.SchemaVersion = CurrentSchemaVersion
	if err := SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}

	err := svc.TestAIConnection(context.Background())
	if err == nil {
		t.Fatalf("TestAIConnection with an unparseable BaseURL succeeded, want an error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "构造 AI 请求失败") {
		t.Errorf("error = %v, want the request-construction context", err)
	}
	if strings.Contains(msg, aiTestURLSecret) {
		t.Errorf("error leaked the URL-embedded credential: %v", err)
	}
	if !strings.Contains(msg, "://***@") {
		t.Errorf("credential was not masked in the error: %v", err)
	}
}

// =========================================================================
// 修复点 4：SummarizeCommit 的 commitHash 选项注入防护
// =========================================================================

// TestSecuritySummarizeCommitRejectsOptionLikeHash：commitHash 作为 git 的
// 位置参数传入 `git show`，以 "-" 开头的值会被 git 当作选项解析——例如
// --output=<path> 可把提交内容写到任意可写路径。合法 hash 与 ref 名都不会
// 以 "-" 开头，必须在进入 git 之前拒绝，且注入目标文件不得被创建。
func TestSecuritySummarizeCommitRejectsOptionLikeHash(t *testing.T) {
	writeFeatureFlagsConfigFile(t, `{"flags":{"aiSummary":true},"schemaVersion":1}`)
	resetAISessionKeyForTest(t)

	svc := AiService{}
	repo := summarizeTestRepo(t)
	target := filepath.Join(t.TempDir(), "pwned-summary.txt")

	out, err := svc.SummarizeCommit(repo, "--output="+target)
	if err == nil {
		t.Fatalf("option-like hash accepted, want a rejection error")
	}
	if !strings.Contains(err.Error(), "非法 commit hash") {
		t.Errorf("error = %v, want an illegal-hash mention", err)
	}
	if out != "" {
		t.Errorf("SummarizeCommit returned %q alongside an error", out)
	}
	if _, statErr := os.Stat(target); statErr == nil {
		t.Fatalf("git wrote %s via an injected --output option; option injection is not blocked", target)
	}

	// 正常 hash 不受影响：守卫只拒绝 "-" 开头的值。
	aiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"feat: ok"}}]}`))
	}))
	defer aiSrv.Close()
	if err := svc.SaveAIConfig(aiSrv.URL, "m", 5, aiTestKey); err != nil {
		t.Fatalf("SaveAIConfig failed: %v", err)
	}
	hashOut, err := ExecGit(repo, "rev-parse", "HEAD")
	if err != nil {
		t.Fatalf("rev-parse HEAD: %v", err)
	}
	if _, err := svc.SummarizeCommit(repo, strings.TrimSpace(hashOut)); err != nil {
		t.Fatalf("SummarizeCommit with a real hash should pass the guard: %v", err)
	}
}

// =========================================================================
// 结论用例：明文密钥绝不跨进程边界（GetAIConfig / SaveAIConfig / complete）
// =========================================================================

// TestSecurityNoPlaintextKeyInConfigOrSanitizedViewAfterFailure：一次失败的
// AI 调用（端点 500）之后，持久化文件、脱敏视图与活动日志都不得出现明文
// key——错误路径的审计边界与成功路径一致。
func TestSecurityNoPlaintextKeyInConfigOrSanitizedViewAfterFailure(t *testing.T) {
	writeFeatureFlagsConfigFile(t, `{"theme":"dark"}`)
	resetAISessionKeyForTest(t)
	ClearRecentActivityLogs()
	defer ClearRecentActivityLogs()

	failSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer failSrv.Close()

	svc := AiService{}
	if err := svc.SaveAIConfig(failSrv.URL, "m", 5, aiTestKey); err != nil {
		t.Fatalf("SaveAIConfig failed: %v", err)
	}
	if _, err := svc.completeAPI(context.Background(), "prompt with "+aiTestKey); err == nil {
		t.Fatalf("complete against a failing endpoint succeeded, want an error")
	}

	serialized, err := json.Marshal(svc.GetAIConfig())
	if err != nil {
		t.Fatalf("marshal sanitized config: %v", err)
	}
	rawConfig, err := os.ReadFile(getConfigPath())
	if err != nil {
		t.Fatalf("read config.json: %v", err)
	}
	var oplog strings.Builder
	for _, entry := range GetRecentActivityLogs() {
		oplog.WriteString(entry.Command + "\n" + entry.Output + "\n" + entry.Error + "\n")
	}
	for name, sink := range map[string]string{
		"sanitized view": string(serialized),
		"config.json":    string(rawConfig),
		"activity log":   oplog.String(),
	} {
		if strings.Contains(sink, aiTestKey) {
			t.Fatalf("plaintext key leaked into the %s after a failed call", name)
		}
	}
}
