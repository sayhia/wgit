package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type FileStatus struct {
	Path           string `json:"path"`
	OldPath        string `json:"oldPath,omitempty"`
	StagedStatus   string `json:"stagedStatus"`   // "M", "A", "D", "R", "?"
	UnstagedStatus string `json:"unstagedStatus"` // "M", "D", "?"
	IsStaged       bool   `json:"isStaged"`
	IsLFS          bool   `json:"isLfs"`
	Additions      int    `json:"additions"`
	Deletions      int    `json:"deletions"`
	Size           string `json:"size,omitempty"`
	IsBinary       bool   `json:"isBinary,omitempty"`
	LockOwner      string `json:"lockOwner,omitempty"`
}

type RepoStatus struct {
	CurrentBranch   string       `json:"currentBranch"`
	TrackingBranch  string       `json:"trackingBranch"`
	Ahead           int          `json:"ahead"`
	Behind          int          `json:"behind"`
	IsClean         bool         `json:"isClean"`
	StagedFiles     []FileStatus `json:"stagedFiles"`
	UnstagedFiles   []FileStatus `json:"unstagedFiles"`
	UntrackedFiles  []FileStatus `json:"untrackedFiles"`
	ConflictedFiles []string     `json:"conflictedFiles"`
	IsMerging       bool         `json:"isMerging"`
	IsRebasing      bool         `json:"isRebasing"`
	IsCherryPicking bool         `json:"isCherryPicking"`
	RebaseHeadName  string       `json:"rebaseHeadName,omitempty"`
	RebaseStep      int          `json:"rebaseStep,omitempty"`
	RebaseTotal     int          `json:"rebaseTotal,omitempty"`
	MergeHeadBranch string       `json:"mergeHeadBranch,omitempty"`
	IsInitialCommit bool         `json:"isInitialCommit"`
	HeadCommit      string       `json:"headCommit,omitempty"`
	TotalCommits    int          `json:"totalCommits"`
}

type CommitItem struct {
	Hash            string      `json:"hash"`
	ShortHash       string      `json:"shortHash"`
	Parents         []string    `json:"parents"`
	AuthorName      string      `json:"authorName"`
	AuthorEmail     string      `json:"authorEmail"`
	Date            string      `json:"date"`
	RelativeDate    string      `json:"relativeDate"`
	Subject         string      `json:"subject"`
	Body            string      `json:"body"`
	Refs            []string    `json:"refs"`
	IsHead          bool        `json:"isHead"`
	Lane            int         `json:"lane"`
	Lines           []GraphLine `json:"lines"`           // graph connection segments
	SignatureStatus string      `json:"signatureStatus"` // "G" (Good), "B" (Bad), "U" (Untrusted), "N" (None)
	Signer          string      `json:"signer"`
}

type GraphLine struct {
	FromLane int    `json:"fromLane"`
	ToLane   int    `json:"toLane"`
	Color    string `json:"color"`
}

type CommitDetail struct {
	Commit    CommitItem   `json:"commit"`
	Files     []FileStatus `json:"files"`
	Additions int          `json:"additions"`
	Deletions int          `json:"deletions"`
	Diff      string       `json:"diff"`
	// DiffTruncated reports that the patch exceeded the large-repo payload guard
	// (see git_large_repo.go) and was cut for webview safety.
	DiffTruncated bool `json:"diffTruncated"`
}

// CommitDetailsBatchResult couples the successfully resolved commit details
// with the hashes that could not be resolved, so bulk lookups can surface
// partial failures instead of silently dropping commits.
type CommitDetailsBatchResult struct {
	Items  []CommitItem       `json:"items"`
	Failed []FailedCommitHash `json:"failed,omitempty"`
}

// FailedCommitHash names one commit hash that failed to resolve in a bulk
// commit-details request, together with the reason git reported.
type FailedCommitHash struct {
	Hash   string `json:"hash"`
	Reason string `json:"reason"`
}

type SubmoduleInfo struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	URL    string `json:"url"`
	Branch string `json:"branch"`
	Commit string `json:"commit"`
	Status string `json:"status"` // "synced", "modified", "uninitialized"
}

type LfsLockInfo struct {
	ID       string `json:"id"`
	Path     string `json:"path"`
	Owner    string `json:"owner"`
	LockedAt string `json:"lockedAt"`
}

type GitHookInfo struct {
	Name      string `json:"name"`
	Enabled   bool   `json:"enabled"`
	Installed bool   `json:"installed"`
	Path      string `json:"path"`
}

type GitProxyConfig struct {
	HttpProxy  string `json:"httpProxy"`
	HttpsProxy string `json:"httpsProxy"`
	IsGlobal   bool   `json:"isGlobal"`
}

type CloneOptions struct {
	URL               string `json:"url"`
	TargetPath        string `json:"targetPath"`
	CloneType         string `json:"cloneType"` // "full", "blobless", "shallow"
	Depth             int    `json:"depth"`
	Branch            string `json:"branch"`
	RecurseSubmodules bool   `json:"recurseSubmodules"`
}

type SecretFinding struct {
	FilePath        string `json:"filePath"`
	LineNumber      int    `json:"lineNumber"`
	RuleName        string `json:"ruleName"`
	Severity        string `json:"severity"` // "CRITICAL", "HIGH", "WARNING"
	RedactedSnippet string `json:"redactedSnippet"`
}

type BatchCherryPickResult struct {
	SuccessCount     int      `json:"successCount"`
	TotalCount       int      `json:"totalCount"`
	FailedCommitHash string   `json:"failedCommitHash"`
	ErrorMessage     string   `json:"errorMessage"`
	HasConflict      bool     `json:"hasConflict"`
	AppliedHashes    []string `json:"appliedHashes"`
}

type HistoryLargeBlob struct {
	OID        string `json:"oid"`
	Size       int64  `json:"size"`
	SizeStr    string `json:"sizeStr"`
	Path       string `json:"path"`
	CommitHash string `json:"commitHash"`
	CommitDate string `json:"commitDate"`
	Author     string `json:"author"`
	Subject    string `json:"subject"`
}

type CommitTemplateInfo struct {
	ConfiguredPath string `json:"configuredPath"`
	Content        string `json:"content"`
	HasTemplate    bool   `json:"hasTemplate"`
}

type UnityMergeStatus struct {
	IsConfigured       bool     `json:"isConfigured"`
	DetectedToolPath   string   `json:"detectedToolPath"`
	ConfiguredToolPath string   `json:"configuredToolPath"`
	InstalledVersions  []string `json:"installedVersions"`
	HasGitattributes   bool     `json:"hasGitattributes"`
}

type PrePushVerificationResult struct {
	UnpushedCommits  []CommitItem    `json:"unpushedCommits"`
	TotalLfsPointers int             `json:"totalLfsPointers"`
	MissingLfsCount  int             `json:"missingLfsCount"`
	MissingLfsFiles  []string        `json:"missingLfsFiles"`
	OversizedBlobs   []LargeFileInfo `json:"oversizedBlobs"`
	IsSafe           bool            `json:"isSafe"`
	StatusSummary    string          `json:"statusSummary"`
}

type LfsCacheStats struct {
	TotalSizeBytes int64                  `json:"totalSizeBytes"`
	TotalSizeStr   string                 `json:"totalSizeStr"`
	FileCount      int                    `json:"fileCount"`
	Extensions     []StorageBreakdownItem `json:"extensions"`
	CachePath      string                 `json:"cachePath"`
}

type LfsFilterConfig struct {
	FetchInclude []string `json:"fetchInclude"`
	FetchExclude []string `json:"fetchExclude"`
	IsConfigured bool     `json:"isConfigured"`
}

type UnrealProjectInfo struct {
	IsUnrealProject     bool     `json:"isUnrealProject"`
	UProjectPath        string   `json:"uprojectPath"`
	ProjectName         string   `json:"projectName"`
	EngineVersion       string   `json:"engineVersion"`
	DetectedEnginePaths []string `json:"detectedEnginePaths"`
	CacheFoldersFound   []string `json:"cacheFoldersFound"`
	CacheSizeBytes      int64    `json:"cacheSizeBytes"`
	CacheSizeStr        string   `json:"cacheSizeStr"`
}

type UnrealCacheCleanResult struct {
	CleanedFolders []string `json:"cleanedFolders"`
	FreedSizeBytes int64    `json:"freedSizeBytes"`
	FreedSizeStr   string   `json:"freedSizeStr"`
}

type LockableSyncResult struct {
	TotalLockableFiles int      `json:"totalLockableFiles"`
	ReadOnlyCount      int      `json:"readOnlyCount"`
	WritableCount      int      `json:"writableCount"`
	ProtectedFiles     []string `json:"protectedFiles"`
	IsProtectionActive bool     `json:"isProtectionActive"`
	FailedCount        int      `json:"failedCount"`
	FailedFiles        []string `json:"failedFiles"`
}

type StudioChangelogCategory struct {
	Category string       `json:"category"`
	Icon     string       `json:"icon"`
	Commits  []CommitItem `json:"commits"`
}

type StudioChangelogResult struct {
	FromRef     string                    `json:"fromRef"`
	ToRef       string                    `json:"toRef"`
	TotalCount  int                       `json:"totalCount"`
	Categories  []StudioChangelogCategory `json:"categories"`
	MarkdownDoc string                    `json:"markdownDoc"`
}

type CustomActionConfig struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	Command       string `json:"command"`
	Args          string `json:"args"`
	WorkingDir    string `json:"workingDir"`
	Icon          string `json:"icon"`
	ShowInToolbar bool   `json:"showInToolbar"`
}

type CustomActionResult struct {
	ActionID   string `json:"actionId"`
	Name       string `json:"name"`
	Command    string `json:"command"`
	Output     string `json:"output"`
	ExitCode   int    `json:"exitCode"`
	DurationMs int64  `json:"durationMs"`
	Success    bool   `json:"success"`
}

type EngineWorkflowInfo struct {
	EngineType         string   `json:"engineType"` // "unreal", "unity", "godot", "cocos", "none"
	ProjectName        string   `json:"projectName"`
	ProjectPath        string   `json:"projectPath"`
	EngineVersion      string   `json:"engineVersion"`
	CacheFolders       []string `json:"cacheFolders"`
	CacheSizeBytes     int64    `json:"cacheSizeBytes"`
	CacheSizeStr       string   `json:"cacheSizeStr"`
	RecommendedActions []string `json:"recommendedActions"`
}

type DiskFolderItem struct {
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	SizeStr string `json:"sizeStr"`
}

type DiskBreakdownResult struct {
	TotalSize       int64                  `json:"totalSize"`
	TotalSizeStr    string                 `json:"totalSizeStr"`
	WorkingTreeSize int64                  `json:"workingTreeSize"`
	WorkingTreeStr  string                 `json:"workingTreeStr"`
	GitInternalSize int64                  `json:"gitInternalSize"`
	GitInternalStr  string                 `json:"gitInternalStr"`
	LfsStoreSize    int64                  `json:"lfsStoreSize"`
	LfsStoreStr     string                 `json:"lfsStoreStr"`
	EngineCacheSize int64                  `json:"engineCacheSize"`
	EngineCacheStr  string                 `json:"engineCacheStr"`
	TopFolders      []DiskFolderItem       `json:"topFolders"`
	AssetCategories []StorageBreakdownItem `json:"assetCategories"`
	Recommendations []string               `json:"recommendations"`
}

type RemoteSyncStatusItem struct {
	Name          string `json:"name"`
	FetchURL      string `json:"fetchUrl"`
	PushURL       string `json:"pushUrl"`
	Branch        string `json:"branch"`
	Ahead         int    `json:"ahead"`
	Behind        int    `json:"behind"`
	LastFetched   string `json:"lastFetched"`
	StatusWarning string `json:"statusWarning,omitempty"`
}

type MultiRemoteStatusResult struct {
	CurrentBranch string                 `json:"currentBranch"`
	Remotes       []RemoteSyncStatusItem `json:"remotes"`
}

type MultiRemoteSyncResult struct {
	Action   string   `json:"action"` // "fetch-all" or "push-all"
	Success  bool     `json:"success"`
	Messages []string `json:"messages"`
}

type TextureLintItem struct {
	Path        string `json:"path"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	Dimensions  string `json:"dimensions"`
	Format      string `json:"format"`
	SizeBytes   int64  `json:"sizeBytes"`
	SizeStr     string `json:"sizeStr"`
	IsPOT       bool   `json:"isPOT"`       // Power of Two
	IsOverSized bool   `json:"isOverSized"` // > 4096px
	Severity    string `json:"severity"`    // "OK", "WARNING", "ERROR"
	Issue       string `json:"issue"`
	Advice      string `json:"advice"`
}

type TextureLintResult struct {
	TotalChecked int               `json:"totalChecked"`
	TotalIssues  int               `json:"totalIssues"`
	Items        []TextureLintItem `json:"items"`
}

type SubtreeItem struct {
	Prefix      string `json:"prefix"`
	RemoteOrURL string `json:"remoteOrUrl"`
	Branch      string `json:"branch"`
	LastCommit  string `json:"lastCommit"`
	LastSquash  string `json:"lastSquash"`
}

type SubtreeListResult struct {
	Subtrees []SubtreeItem `json:"subtrees"`
}

type SubtreeActionResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Output  string `json:"output"`
}
type LfsTuningConfig struct {
	ConcurrentTransfers int    `json:"concurrentTransfers"` // default 3, recommended 8-16
	PostBufferMB        int    `json:"postBufferMb"`        // default 1MB, recommended 512MB
	DialTimeoutSec      int    `json:"dialTimeoutSec"`      // default 30s
	CompressionLevel    int    `json:"compressionLevel"`    // core.compression
	IsOptimized         bool   `json:"isOptimized"`
	ActiveSummary       string `json:"activeSummary"`
}

type LfsTuningResult struct {
	Success bool            `json:"success"`
	Config  LfsTuningConfig `json:"config"`
	Message string          `json:"message"`
}

// Modal 72: Asset Reference Inspector
type AssetReferenceItem struct {
	FilePath       string `json:"filePath"`
	FileType       string `json:"fileType"` // "Prefab", "Scene", "Material", "Animation", "Script/Config", "Other"
	MatchCount     int    `json:"matchCount"`
	PreviewSnippet string `json:"previewSnippet"`
}

type AssetReferenceResult struct {
	TargetPath     string               `json:"targetPath"`
	AssetGuid      string               `json:"assetGuid,omitempty"`
	ReferenceCount int                  `json:"referenceCount"`
	References     []AssetReferenceItem `json:"references"`
	SafeToDelete   bool                 `json:"safeToDelete"`
	EngineHint     string               `json:"engineHint"`
	Message        string               `json:"message"`
	// False when some files could not be scanned; "no references" is then unproven.
	AnalysisComplete bool     `json:"analysisComplete"`
	Warnings         []string `json:"warnings"`
}

// Modal 73: Commit Signing & Verification
type SignedCommitItem struct {
	CommitHash      string `json:"commitHash"`
	ShortHash       string `json:"shortHash"`
	AuthorName      string `json:"authorName"`
	AuthorEmail     string `json:"authorEmail"`
	Subject         string `json:"subject"`
	Date            string `json:"date"`
	SignatureStatus string `json:"signatureStatus"` // "G" (Good), "B" (Bad), "U" (Untrusted), "N" (Unsigned), etc.
	Signer          string `json:"signer"`
	KeyID           string `json:"keyId"`
	StatusHuman     string `json:"statusHuman"` // "已验证 (Verified)", "未签名 (Unsigned)", "签名失效 (Bad)"
}

type CommitSigningStatus struct {
	GpgSignEnabled   bool               `json:"gpgSignEnabled"`
	SigningKey       string             `json:"signingKey"`
	GpgFormat        string             `json:"gpgFormat"` // "openpgp" or "ssh"
	AvailableSSHKeys []string           `json:"availableSshKeys"`
	HasGpgBinary     bool               `json:"hasGpgBinary"`
	RecentCommits    []SignedCommitItem `json:"recentCommits"`
	CompliantCount   int                `json:"compliantCount"`
	TotalChecked     int                `json:"totalChecked"`
	ComplianceRate   string             `json:"complianceRate"`
}

type CommitSigningResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// Modal 74: LFS Lock Activity & Conflict Dashboard
type LfsLockActivityItem struct {
	Path        string `json:"path"`
	Owner       string `json:"owner"`
	IsMe        bool   `json:"isMe"`
	LockedAt    string `json:"lockedAt"`
	DurationStr string `json:"durationStr"`
	IsZombie    bool   `json:"isZombie"`    // held > 72 hours
	HasConflict bool   `json:"hasConflict"` // modified locally by current user while owned by teammate
	LocalStatus string `json:"localStatus,omitempty"`
	Category    string `json:"category"`
}

type LfsLockActivityResult struct {
	TotalLocks     int                   `json:"totalLocks"`
	MyLockCount    int                   `json:"myLockCount"`
	TeamLockCount  int                   `json:"teamLockCount"`
	ZombieCount    int                   `json:"zombieCount"`
	CollisionCount int                   `json:"collisionCount"`
	Locks          []LfsLockActivityItem `json:"locks"`
	Message        string                `json:"message"`
}

// Modal 75: Git-Ignore Completeness & Artifact Sanitizer
type IgnoreArtifactItem struct {
	Path      string `json:"path"`
	Category  string `json:"category"`  // "Unity Cache", "Unreal Build", "IDE Artifact", "OS File", "Compiler Binary"
	IsTracked bool   `json:"isTracked"` // dangerously tracked in git
	SizeBytes int64  `json:"sizeBytes"`
	SizeHuman string `json:"sizeHuman"`
	Ignored   bool   `json:"ignored"`
	Remedy    string `json:"remedy"`
}

type IgnoreSanitizerResult struct {
	EngineType       string               `json:"engineType"`
	MissingRules     []string             `json:"missingRules"`
	HasGitIgnore     bool                 `json:"hasGitIgnore"`
	TrackedArtifacts []IgnoreArtifactItem `json:"trackedArtifacts"`
	TotalArtifacts   int                  `json:"totalArtifacts"`
	TrackedCount     int                  `json:"trackedCount"`
	HealthScore      int                  `json:"healthScore"` // 0 - 100
	Summary          string               `json:"summary"`
}

type IgnoreSanitizeActionResult struct {
	Success        bool     `json:"success"`
	RulesAdded     []string `json:"rulesAdded"`
	UntrackedFiles []string `json:"untrackedFiles"`
	FailedFiles    []string `json:"failedFiles"`
	Message        string   `json:"message"`
}

// Modal 76: Audio & SFX Inspector / Player
type AudioInspectResult struct {
	Path          string   `json:"path"`
	Format        string   `json:"format"`        // "WAV", "MP3", "OGG", "FLAC", "SoundBank"
	DurationSec   float64  `json:"durationSec"`   // duration in seconds
	DurationStr   string   `json:"durationStr"`   // e.g. "01:24 (84.2s)"
	SampleRate    int      `json:"sampleRate"`    // 44100, 48000, 96000
	Channels      int      `json:"channels"`      // 1=Mono, 2=Stereo
	ChannelStr    string   `json:"channelStr"`    // "单声道 (Mono)" / "立体声 (Stereo)"
	BitsPerSample int      `json:"bitsPerSample"` // 16, 24, 32
	MetadataKnown bool     `json:"metadataKnown"`
	SizeBytes     int64    `json:"sizeBytes"`
	SizeStr       string   `json:"sizeStr"`
	IsStandard48k bool     `json:"isStandard48k"`     // 48000Hz standard
	DataUri       string   `json:"dataUri,omitempty"` // base64 for audio playback
	Warnings      []string `json:"warnings"`
	Message       string   `json:"message"`
}

// Modal 77: 3D Model Mesh & Polygon Budget Linter
type MeshLintResult struct {
	Path               string   `json:"path"`
	Format             string   `json:"format"`
	TriangleCount      int      `json:"triangleCount"`
	TriangleCountKnown bool     `json:"triangleCountKnown"`
	VertexCount        int      `json:"vertexCount"`
	SubmeshCount       int      `json:"submeshCount"`
	MaterialCount      int      `json:"materialCount"`
	BudgetPreset       string   `json:"budgetPreset"`
	BudgetLimit        int      `json:"budgetLimit"`
	IsOverBudget       bool     `json:"isOverBudget"`
	Severity           string   `json:"severity"` // "OK", "WARNING", "ERROR"
	SizeBytes          int64    `json:"sizeBytes"`
	SizeStr            string   `json:"sizeStr"`
	Advice             string   `json:"advice"`
	Warnings           []string `json:"warnings"`
}

// Modal 78: Game Studio CI/CD Pipeline Monitor
type CIPipelineStage struct {
	Name     string `json:"name"`
	Status   string `json:"status"` // "SUCCESS", "RUNNING", "FAILED", "PENDING"
	Duration string `json:"duration"`
}

type CIPipelineRun struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Branch      string            `json:"branch"`
	CommitHash  string            `json:"commitHash"`
	ShortHash   string            `json:"shortHash"`
	Author      string            `json:"author"`
	Message     string            `json:"message"`
	Status      string            `json:"status"` // "SUCCESS", "RUNNING", "FAILED"
	TriggerTime string            `json:"triggerTime"`
	Duration    string            `json:"duration"`
	Stages      []CIPipelineStage `json:"stages"`
}

type StudioCIResult struct {
	Provider     string          `json:"provider"` // "GitHub Actions", "Tencent 工蜂 CI", "Local Studio Runner"
	ActiveBranch string          `json:"activeBranch"`
	LatestRun    *CIPipelineRun  `json:"latestRun,omitempty"`
	HistoryRuns  []CIPipelineRun `json:"historyRuns"`
	Message      string          `json:"message"`
}

type StudioCIActionResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// Modal 79: Single File Historical Reversion & Timeline Diff
type FileRevisionItem struct {
	CommitHash string `json:"commitHash"`
	ShortHash  string `json:"shortHash"`
	AuthorName string `json:"authorName"`
	Date       string `json:"date"`
	Subject    string `json:"subject"`
	SizeBytes  int64  `json:"sizeBytes"`
	SizeStr    string `json:"sizeStr"`
}

type FileTimelineResult struct {
	FilePath  string             `json:"filePath"`
	Revisions []FileRevisionItem `json:"revisions"`
	Message   string             `json:"message"`
}

type FileRevertResult struct {
	Success    bool   `json:"success"`
	FilePath   string `json:"filePath"`
	CommitHash string `json:"commitHash"`
	Message    string `json:"message"`
}

// ==========================================
// Round 101: Shader Variant & Cache Inspector
// ==========================================

type ShaderPragmaDirective struct {
	Directive   string   `json:"directive"`
	Scope       string   `json:"scope"`
	Keywords    []string `json:"keywords"`
	OptionCount int      `json:"optionCount"`
	LineNumber  int      `json:"lineNumber"`
	RawLine     string   `json:"rawLine"`
}

type ShaderPassInfo struct {
	Name         string                  `json:"name"`
	PassIndex    int                     `json:"passIndex"`
	Pragmas      []ShaderPragmaDirective `json:"pragmas"`
	VariantCount int64                   `json:"variantCount"`
}

type ShaderVariantOffender struct {
	Path         string `json:"path"`
	ShaderType   string `json:"shaderType"`
	VariantCount int64  `json:"variantCount"`
	Severity     string `json:"severity"` // "OK", "WARNING", "CRITICAL"
	KeywordCount int    `json:"keywordCount"`
	PassCount    int    `json:"passCount"`
	SizeBytes    int64  `json:"sizeBytes"`
	SizeStr      string `json:"sizeStr"`
}

type ShaderInspectResult struct {
	TargetShader       string                  `json:"targetShader"`
	ShaderType         string                  `json:"shaderType"`
	TotalVariants      int64                   `json:"totalVariants"`
	TheoreticalMax     int64                   `json:"theoreticalMax"`
	PassCount          int                     `json:"passCount"`
	TotalKeywords      int                     `json:"totalKeywords"`
	Pragmas            []ShaderPragmaDirective `json:"pragmas"`
	Passes             []ShaderPassInfo        `json:"passes"`
	Severity           string                  `json:"severity"`
	IsExplosion        bool                    `json:"isExplosion"`
	StrippingAdvice    []string                `json:"strippingAdvice"`
	ProjectShaderCount int                     `json:"projectShaderCount"`
	ExplosionCount     int                     `json:"explosionCount"`
	OffenderList       []ShaderVariantOffender `json:"offenderList"`
	Message            string                  `json:"message"`
}

// ==========================================
// Round 102: DCC Asset Watcher & Auto-Lock Guard
// ==========================================

type DCCActiveProcess struct {
	PID         int    `json:"pid"`
	ProcessName string `json:"processName"`
	DCCTool     string `json:"dccTool"`
}

type DCCLockItem struct {
	LockFilePath    string `json:"lockFilePath"`
	SourceAssetPath string `json:"sourceAssetPath"`
	DCCTool         string `json:"dccTool"`
	LockOwner       string `json:"lockOwner"`
	LastModified    string `json:"lastModified"`
	IsActive        bool   `json:"isActive"`
	IsLFS           bool   `json:"isLfs"`
	LfsLockStatus   string `json:"lfsLockStatus"`
	LfsLockOwner    string `json:"lfsLockOwner,omitempty"`
	CollisionRisk   bool   `json:"collisionRisk"`
	Recommendation  string `json:"recommendation"`
}

type DCCLockWatchResult struct {
	TotalDCCFiles    int                `json:"totalDccFiles"`
	ActiveLocksCount int                `json:"activeLocksCount"`
	UnprotectedCount int                `json:"unprotectedCount"`
	CollisionCount   int                `json:"collisionCount"`
	RunningProcesses []DCCActiveProcess `json:"runningProcesses"`
	Items            []DCCLockItem      `json:"items"`
	Summary          string             `json:"summary"`
	Message          string             `json:"message"`
}

type DCCAutoLockResult struct {
	Success   bool   `json:"success"`
	AssetPath string `json:"assetPath"`
	Message   string `json:"message"`
}

// ============================================================================
// Round 103: Unreal Blueprint & Binary AST Diff (Tencent UGit Parity)
// ============================================================================

type AssetGraphPin struct {
	Name         string `json:"name"`
	Direction    string `json:"direction"`
	PinType      string `json:"pinType"`
	DefaultValue string `json:"defaultValue"`
	ConnectedTo  string `json:"connectedTo"`
}

type AssetGraphNode struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Class      string            `json:"class"`
	Category   string            `json:"category"`
	PosX       int               `json:"posX"`
	PosY       int               `json:"posY"`
	Comment    string            `json:"comment,omitempty"`
	Properties map[string]string `json:"properties,omitempty"`
	Pins       []AssetGraphPin   `json:"pins"`
}

type AssetPackageHeader struct {
	EngineVersion   string   `json:"engineVersion"`
	Magic           string   `json:"magic"`
	AssetFormat     string   `json:"assetFormat"`
	PackageFlags    string   `json:"packageFlags"`
	NameCount       int      `json:"nameCount"`
	ExportCount     int      `json:"exportCount"`
	ImportCount     int      `json:"importCount"`
	PackageGuid     string   `json:"packageGuid"`
	TotalHeaderSize int      `json:"totalHeaderSize"`
	SummaryTags     []string `json:"summaryTags"`
}

type NodePinDiff struct {
	PinName    string `json:"pinName"`
	ChangeType string `json:"changeType"`
	OldValue   string `json:"oldValue,omitempty"`
	NewValue   string `json:"newValue,omitempty"`
	Detail     string `json:"detail"`
}

type NodePropertyDiff struct {
	Property string `json:"property"`
	OldValue string `json:"oldValue"`
	NewValue string `json:"newValue"`
}

type ModifiedNodeItem struct {
	NodeID          string             `json:"nodeId"`
	NodeName        string             `json:"nodeName"`
	NodeClass       string             `json:"nodeClass"`
	Category        string             `json:"category"`
	PinChanges      []NodePinDiff      `json:"pinChanges"`
	PropertyChanges []NodePropertyDiff `json:"propertyChanges"`
	PosDeltaX       int                `json:"posDeltaX"`
	PosDeltaY       int                `json:"posDeltaY"`
	Summary         string             `json:"summary"`
}

type UnrealAssetDiffResult struct {
	AssetPath        string              `json:"assetPath"`
	BaseRef          string              `json:"baseRef"`
	TargetRef        string              `json:"targetRef"`
	AssetFormat      string              `json:"assetFormat"`
	AnalysisKnown    bool                `json:"analysisKnown"`
	AnalysisNote     string              `json:"analysisNote,omitempty"`
	BaseHeader       *AssetPackageHeader `json:"baseHeader,omitempty"`
	TargetHeader     *AssetPackageHeader `json:"targetHeader,omitempty"`
	AddedNodes       []AssetGraphNode    `json:"addedNodes"`
	RemovedNodes     []AssetGraphNode    `json:"removedNodes"`
	ModifiedNodes    []ModifiedNodeItem  `json:"modifiedNodes"`
	UnchangedCount   int                 `json:"unchangedCount"`
	TotalBaseNodes   int                 `json:"totalBaseNodes"`
	TotalTargetNodes int                 `json:"totalTargetNodes"`
	HasGraphChanges  bool                `json:"hasGraphChanges"`
	VisualSummary    string              `json:"visualSummary"`
	Message          string              `json:"message"`
}

// ============================================================================
// Round 104: Sub-Level & Scene Stream Merge Wizard (Tencent UGit Parity)
// ============================================================================

type Vector3D struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

type BoundingBox3D struct {
	Center  Vector3D `json:"center"`
	Extents Vector3D `json:"extents"`
	Min     Vector3D `json:"min"`
	Max     Vector3D `json:"max"`
}

type SceneStreamActor struct {
	Guid       string        `json:"guid"`
	Name       string        `json:"name"`
	Class      string        `json:"class"`
	SubLevel   string        `json:"subLevel"`
	DataLayer  string        `json:"dataLayer,omitempty"`
	Position   Vector3D      `json:"position"`
	Rotation   Vector3D      `json:"rotation"`
	Scale      Vector3D      `json:"scale"`
	Bounds     BoundingBox3D `json:"bounds"`
	SourceSide string        `json:"sourceSide"`
}

type GuidCollisionItem struct {
	Guid             string `json:"guid"`
	CurrentActorName string `json:"currentActorName"`
	TargetActorName  string `json:"targetActorName"`
	CurrentClass     string `json:"currentClass"`
	TargetClass      string `json:"targetClass"`
	SubLevel         string `json:"subLevel"`
	IsIdentical      bool   `json:"isIdentical"`
	RecommendedFix   string `json:"recommendedFix"`
}

type SpatialOverlapItem struct {
	ActorGuidA     string        `json:"actorGuidA"`
	ActorNameA     string        `json:"actorNameA"`
	ActorClassA    string        `json:"actorClassA"`
	SubLevelA      string        `json:"subLevelA"`
	ActorGuidB     string        `json:"actorGuidB"`
	ActorNameB     string        `json:"actorNameB"`
	ActorClassB    string        `json:"actorClassB"`
	SubLevelB      string        `json:"subLevelB"`
	CenterDistance float64       `json:"centerDistance"`
	OverlapVolume  float64       `json:"overlapVolume"`
	BoundingBoxA   BoundingBox3D `json:"boundingBoxA"`
	BoundingBoxB   BoundingBox3D `json:"boundingBoxB"`
	Severity       string        `json:"severity"`
	RecommendedFix string        `json:"recommendedFix"`
}

type TransformConflictItem struct {
	Guid              string   `json:"guid"`
	ActorName         string   `json:"actorName"`
	SubLevel          string   `json:"subLevel"`
	CurrentPos        Vector3D `json:"currentPos"`
	TargetPos         Vector3D `json:"targetPos"`
	DeltaDistance     float64  `json:"deltaDistance"`
	CurrentRot        Vector3D `json:"currentRot"`
	TargetRot         Vector3D `json:"targetRot"`
	RecommendedAction string   `json:"recommendedAction"`
}

type SceneMergeStrategyItem struct {
	Category    string `json:"category"`
	TargetID    string `json:"targetId"`
	Action      string `json:"action"`
	Description string `json:"description"`
	Automated   bool   `json:"automated"`
}

type SceneStreamAuditResult struct {
	CurrentBranch      string                   `json:"currentBranch"`
	TargetBranch       string                   `json:"targetBranch"`
	AuditedSubLevels   []string                 `json:"auditedSubLevels"`
	EngineType         string                   `json:"engineType"`
	AnalysisKnown      bool                     `json:"analysisKnown"`
	TotalActorsCurrent int                      `json:"totalActorsCurrent"`
	TotalActorsTarget  int                      `json:"totalActorsTarget"`
	GuidCollisions     []GuidCollisionItem      `json:"guidCollisions"`
	SpatialOverlaps    []SpatialOverlapItem     `json:"spatialOverlaps"`
	TransformConflicts []TransformConflictItem  `json:"transformConflicts"`
	SafetyScore        int                      `json:"safetyScore"`
	SafetyLevel        string                   `json:"safetyLevel"`
	MergeVerdict       string                   `json:"mergeVerdict"`
	RecommendedPlan    []SceneMergeStrategyItem `json:"recommendedPlan"`
	SummaryMessage     string                   `json:"summaryMessage"`
}

type SceneStreamResolveResult struct {
	Success       bool      `json:"success"`
	ScenePath     string    `json:"scenePath"`
	ActorGuid     string    `json:"actorGuid"`
	AppliedAction string    `json:"appliedAction"`
	NewActorGuid  string    `json:"newActorGuid,omitempty"`
	OffsetApplied *Vector3D `json:"offsetApplied,omitempty"`
	Message       string    `json:"message"`
}

// ============================================================================
// Round 105: Animation Rig & Skeleton Linter Structs (Tencent UGit Parity)
// ============================================================================

type AnimClipInfo struct {
	Name            string   `json:"name"`
	DurationSeconds float64  `json:"durationSeconds"`
	SampleRate      float64  `json:"sampleRate"`
	FrameCount      int      `json:"frameCount"`
	TrackCount      int      `json:"trackCount"`
	IsLooping       bool     `json:"isLooping"`
	HasRootMotion   bool     `json:"hasRootMotion"`
	RootSpeedMax    float64  `json:"rootSpeedMax"`
	Issues          []string `json:"issues"`
}

type AnimBoneInfo struct {
	Name             string  `json:"name"`
	ParentName       string  `json:"parentName,omitempty"`
	Depth            int     `json:"depth"`
	HasNegativeScale bool    `json:"hasNegativeScale"`
	MaxWeight        float64 `json:"maxWeight,omitempty"`
}

type AnimRigBudgetLimits struct {
	MaxBones      int `json:"maxBones"`
	MaxInfluences int `json:"maxInfluences"`
	MaxSampleRate int `json:"maxSampleRate"`
	MaxTrackCount int `json:"maxTrackCount"`
}

type AnimRigLintResult struct {
	AssetPath                 string              `json:"assetPath"`
	Format                    string              `json:"format"` // "FBX", "GLTF", "GLB", "ANIM"
	AnalysisKnown             bool                `json:"analysisKnown"`
	Preset                    string              `json:"preset"` // "Mobile", "Console", "Cinematic"
	BoneCount                 int                 `json:"boneCount"`
	MaxInfluencesPerVertex    int                 `json:"maxInfluencesPerVertex"`
	TotalDurationSeconds      float64             `json:"totalDurationSeconds"`
	OverallSampleRate         float64             `json:"overallSampleRate"`
	TotalTrackCount           int                 `json:"totalTrackCount"`
	Clips                     []AnimClipInfo      `json:"clips"`
	Bones                     []AnimBoneInfo      `json:"bones"`
	HasAbnormalRootMotion     bool                `json:"hasAbnormalRootMotion"`
	HasNegativeScale          bool                `json:"hasNegativeScale"`
	HasExtremeKeyframeDensity bool                `json:"hasExtremeKeyframeDensity"`
	HasUncompressedTracks     bool                `json:"hasUncompressedTracks"`
	IsOverBudget              bool                `json:"isOverBudget"`
	PerformanceRating         string              `json:"performanceRating"` // "Mobile Ready", "Console Tier", "Cinematic Only", "Optimization Required"
	Severity                  string              `json:"severity"`          // "OK", "WARNING", "ERROR"
	BudgetLimits              AnimRigBudgetLimits `json:"budgetLimits"`
	Issues                    []string            `json:"issues"`
	Advice                    []string            `json:"advice"`
	SizeBytes                 int64               `json:"sizeBytes"`
	SizeStr                   string              `json:"sizeStr"`
	Message                   string              `json:"message"`
}

// ============================================================================
// Round 106: Localization & Missing Glyphs Guard Structs (Tencent UGit Parity)
// ============================================================================

type LocalizedEntry struct {
	Key          string            `json:"key"`
	SourceText   string            `json:"sourceText"`
	Translations map[string]string `json:"translations"` // lang -> text
	MissingLangs []string          `json:"missingLangs"`
	Status       string            `json:"status"` // "COMPLETE", "PARTIAL", "MISSING"
}

type MissingGlyphItem struct {
	Char        string   `json:"char"`
	UnicodeHex  string   `json:"unicodeHex"`
	UnicodeName string   `json:"unicodeName"`
	Lang        string   `json:"lang"`
	UsedInKeys  []string `json:"usedInKeys"`
	SampleText  string   `json:"sampleText"`
}

type LangParityStats struct {
	Language          string  `json:"language"`
	TotalKeys         int     `json:"totalKeys"`
	TranslatedKeys    int     `json:"translatedKeys"`
	MissingKeys       int     `json:"missingKeys"`
	UntranslatedCount int     `json:"untranslatedCount"`
	ParityPercent     float64 `json:"parityPercent"`
	Status            string  `json:"status"` // "EXCELLENT", "GOOD", "NEEDS_WORK", "CRITICAL"
}

type FontGlyphAuditSummary struct {
	FontPath           string             `json:"fontPath"`
	FontFormat         string             `json:"fontFormat"` // "TTF", "OTF", "WOFF"
	TotalFontGlyphs    int                `json:"totalFontGlyphs"`
	TotalCharsChecked  int                `json:"totalCharsChecked"`
	MissingGlyphsCount int                `json:"missingGlyphsCount"`
	MissingGlyphs      []MissingGlyphItem `json:"missingGlyphs"`
	CoveragePercent    float64            `json:"coveragePercent"`
}

type LocalizationAuditResult struct {
	AnalysisKnown      bool                   `json:"analysisKnown"`
	PrimaryLanguage    string                 `json:"primaryLanguage"`
	LanguagesFound     []string               `json:"languagesFound"`
	TotalUniqueKeys    int                    `json:"totalUniqueKeys"`
	OverallParityScore float64                `json:"overallParityScore"` // 0 - 100%
	LanguageStats      []LangParityStats      `json:"languageStats"`
	UntranslatedList   []LocalizedEntry       `json:"untranslatedList"`
	FontAudit          *FontGlyphAuditSummary `json:"fontAudit,omitempty"`
	SourceFilesCount   int                    `json:"sourceFilesCount"`
	AuditedFiles       []string               `json:"auditedFiles"`
	Severity           string                 `json:"severity"` // "OK", "WARNING", "ERROR"
	Advice             []string               `json:"advice"`
	Message            string                 `json:"message"`
}

// ============================================================================
// Round 107: Native C++ Plugins ABI Guard Structs (Tencent UGit Parity)
// ============================================================================

type BinaryArchInfo struct {
	Architecture string `json:"architecture"` // "arm64", "x86_64", "armeabi-v7a", "x86", "arm64e"
	BitWidth     int    `json:"bitWidth"`     // 32 or 64
	Endianness   string `json:"endianness"`   // "LittleEndian", "BigEndian"
	Subtype      string `json:"subtype,omitempty"`
}

type NativePluginItem struct {
	RelativePath      string           `json:"relativePath"`
	PluginName        string           `json:"pluginName"`
	Format            string           `json:"format"` // "ELF", "Mach-O", "Mach-O Fat", "PE", "AAR", "Framework"
	Architectures     []string         `json:"architectures"`
	ArchitecturesInfo []BinaryArchInfo `json:"architecturesInfo"`
	TargetPlatform    string           `json:"targetPlatform"` // "Android", "iOS", "macOS", "Windows", "Unknown"
	Is64BitOnly       bool             `json:"is64BitOnly"`
	IsUniversal       bool             `json:"isUniversal"`
	SizeBytes         int64            `json:"sizeBytes"`
	SizeStr           string           `json:"sizeStr"`
	HasIssue          bool             `json:"hasIssue"`
	Issues            []string         `json:"issues"`
}

type PlatformAbiStatus struct {
	Platform        string   `json:"platform"` // "iOS", "Android", "macOS", "Windows"
	RequiredArches  []string `json:"requiredArches"`
	AvailableArches []string `json:"availableArches"`
	MissingArches   []string `json:"missingArches"`
	PluginCount     int      `json:"pluginCount"`
	IsCompliant     bool     `json:"isCompliant"`
	Severity        string   `json:"severity"` // "OK", "WARNING", "ERROR"
}

type NativePluginsAbiResult struct {
	TotalPluginsCount      int                 `json:"totalPluginsCount"`
	ScannedDirectories     []string            `json:"scannedDirectories"`
	Plugins                []NativePluginItem  `json:"plugins"`
	PlatformStatuses       []PlatformAbiStatus `json:"platformStatuses"`
	CompliantCount         int                 `json:"compliantCount"`
	DefectCount            int                 `json:"defectCount"`
	HasCriticalMismatch    bool                `json:"hasCriticalMismatch"`
	Severity               string              `json:"severity"` // "OK", "WARNING", "ERROR"
	SummaryRecommendations []string            `json:"summaryRecommendations"`
	Message                string              `json:"message"`
}

// ============================================================================
// Round 108: TAPD Agile Work Item & Defect Integrator Structs (Tencent UGit Parity)
// ============================================================================

type TAPDWorkItem struct {
	ID            string   `json:"id"`
	Type          string   `json:"type"` // "story", "task", "bug"
	Title         string   `json:"title"`
	Status        string   `json:"status"`      // "open", "in_progress", "resolved", "verified", "closed"
	StatusLabel   string   `json:"statusLabel"` // "待规划", "进行中", "已解决", "已验证", "已关闭"
	Priority      string   `json:"priority"`    // "urgent", "high", "normal", "low"
	PriorityLabel string   `json:"priorityLabel"`
	Owner         string   `json:"owner"`
	Creator       string   `json:"creator"`
	Iteration     string   `json:"iteration"`
	CreatedTime   string   `json:"createdTime"`
	UpdatedTime   string   `json:"updatedTime"`
	Description   string   `json:"description,omitempty"`
	BoundCommits  []string `json:"boundCommits"`
	URL           string   `json:"url"`
}

type TAPDWorkItemListResult struct {
	WorkspaceID  string         `json:"workspaceId"`
	TotalCount   int            `json:"totalCount"`
	StoriesCount int            `json:"storiesCount"`
	TasksCount   int            `json:"tasksCount"`
	BugsCount    int            `json:"bugsCount"`
	Items        []TAPDWorkItem `json:"items"`
	IsConnected  bool           `json:"isConnected"`
	Message      string         `json:"message"`
}

type TAPDReferenceItem struct {
	TicketID   string        `json:"ticketId"`
	PrefixType string        `json:"prefixType"` // "tapd", "bug", "story", "task", "hash"
	RawTag     string        `json:"rawTag"`
	WorkItem   *TAPDWorkItem `json:"workItem,omitempty"`
}

type TAPDCommitReferenceResult struct {
	CommitMessage string              `json:"commitMessage"`
	FoundTickets  []TAPDReferenceItem `json:"foundTickets"`
	HasTAPDTag    bool                `json:"hasTapdTag"`
	FormattedTag  string              `json:"formattedTag"`
}

type TAPDBindResult struct {
	Success        bool          `json:"success"`
	CommitHash     string        `json:"commitHash"`
	TapdID         string        `json:"tapdId"`
	PreviousStatus string        `json:"previousStatus"`
	CurrentStatus  string        `json:"currentStatus"`
	UpdatedItem    *TAPDWorkItem `json:"updatedItem"`
	Message        string        `json:"message"`
}

// ============================================================================
// Round 109: Game Hotfix Patch Manifest Generator (Tencent UGit Parity)
// ============================================================================

type HotfixPatchFileItem struct {
	Path              string `json:"path"`
	Action            string `json:"action"` // "ADDED", "MODIFIED", "DELETED"
	Tier              string `json:"tier"`   // "Code", "Config", "Resources", "Full Pack Required"
	Size              int64  `json:"size"`
	SizeStr           string `json:"sizeStr"`
	CompressedSize    int64  `json:"compressedSize"`
	CompressedSizeStr string `json:"compressedSizeStr"`
	CRC32             string `json:"crc32"`
	MD5               string `json:"md5"`
	SHA256            string `json:"sha256"`
	RequiresReboot    bool   `json:"requiresReboot"`
}

type HotfixTierSummary struct {
	Tier         string `json:"tier"`
	Count        int    `json:"count"`
	TotalBytes   int64  `json:"totalBytes"`
	TotalSizeStr string `json:"totalSizeStr"`
}

type HotfixManifestResult struct {
	Success                bool                  `json:"success"`
	ManifestVersion        string                `json:"manifestVersion"`
	AppVersion             string                `json:"appVersion"`
	BaseRef                string                `json:"baseRef"`
	TargetRef              string                `json:"targetRef"`
	GeneratedAt            string                `json:"generatedAt"`
	FullPackRequired       bool                  `json:"fullPackRequired"`
	FullPackReason         string                `json:"fullPackReason,omitempty"`
	TotalFilesCount        int                   `json:"totalFilesCount"`
	TotalPatchBytes        int64                 `json:"totalPatchBytes"`
	TotalPatchSizeStr      string                `json:"totalPatchSizeStr"`
	TotalCompressedBytes   int64                 `json:"totalCompressedBytes"`
	TotalCompressedSizeStr string                `json:"totalCompressedSizeStr"`
	TierBreakdown          []HotfixTierSummary   `json:"tierBreakdown"`
	PatchFiles             []HotfixPatchFileItem `json:"patchFiles"`
	DeltaPackageList       []string              `json:"deltaPackageList"`
	ManifestJSON           string                `json:"manifestJson"`
	Summary                string                `json:"summary"`
}

// ============================================================================
// Round 110: Package Size Attribution & Budget Dashboard (Tencent UGit Parity)
// ============================================================================

type AssetSystemAttribution struct {
	SystemName   string   `json:"systemName"` // "Characters", "Environments", "UI", "Audio", "VFX", "Code", "ThirdParty", "Misc"
	TotalBytes   int64    `json:"totalBytes"`
	TotalSizeStr string   `json:"totalSizeStr"`
	FileCount    int      `json:"fileCount"`
	Percentage   float64  `json:"percentage"`
	LfsBytes     int64    `json:"lfsBytes"`
	LfsSizeStr   string   `json:"lfsSizeStr"`
	LfsCount     int      `json:"lfsCount"`
	TopAssets    []string `json:"topAssets"`
}

type PlatformBudgetStatus struct {
	Platform          string  `json:"platform"` // "iOS App Store (Cellular)", "iOS App Store (Full)", "Android (APK+OBB)", "PC / Console", "WeChat Mini Game"
	MaxBudgetLimit    int64   `json:"maxBudgetLimit"`
	MaxBudgetLimitStr string  `json:"maxBudgetLimitStr"`
	CurrentSize       int64   `json:"currentSize"`
	CurrentSizeStr    string  `json:"currentSizeStr"`
	UsageRatio        float64 `json:"usageRatio"`
	RemainingBytes    int64   `json:"remainingBytes"`
	RemainingStr      string  `json:"remainingStr"`
	Status            string  `json:"status"` // "SAFE", "WARNING", "OVERRUN"
}

type BudgetOptimizationSuggestion struct {
	Category           string `json:"category"` // "Texture Compression", "Audio Compression", "Mesh Optimization", "LFS Offloading"
	TargetFile         string `json:"targetFile"`
	CurrentSize        int64  `json:"currentSize"`
	CurrentSizeStr     string `json:"currentSizeStr"`
	EstimatedSaving    int64  `json:"estimatedSaving"`
	EstimatedSavingStr string `json:"estimatedSavingStr"`
	Recommendation     string `json:"recommendation"`
	Priority           string `json:"priority"` // "HIGH", "MEDIUM", "LOW"
}

type PackageBudgetResult struct {
	RepoPath         string                         `json:"repoPath"`
	TotalAssetsCount int                            `json:"totalAssetsCount"`
	TotalSizeBytes   int64                          `json:"totalSizeBytes"`
	TotalSizeStr     string                         `json:"totalSizeStr"`
	TotalLfsBytes    int64                          `json:"totalLfsBytes"`
	TotalLfsSizeStr  string                         `json:"totalLfsSizeStr"`
	Systems          []AssetSystemAttribution       `json:"systems"`
	PlatformBudgets  []PlatformBudgetStatus         `json:"platformBudgets"`
	OptimizationTips []BudgetOptimizationSuggestion `json:"optimizationTips"`
	HasOverrun       bool                           `json:"hasOverrun"`
	WorstPlatform    string                         `json:"worstPlatform"`
	Summary          string                         `json:"summary"`
}

// ============================================================================
// Round 111: Multi-Project Worktree Switchboard (Tencent UGit Parity)
// ============================================================================

type StudioProjectNode struct {
	Path              string `json:"path"`
	Name              string `json:"name"`
	Category          string `json:"category"` // "MainWorktree", "LinkedWorktree", "Client", "Server", "Tooling", "Docs", "AdjacentRepo"
	Branch            string `json:"branch"`
	IsDirty           bool   `json:"isDirty"`
	DirtyFilesCount   int    `json:"dirtyFilesCount"`
	IsLocked          bool   `json:"isLocked"`
	LockReason        string `json:"lockReason,omitempty"`
	LastCommitHash    string `json:"lastCommitHash"`
	LastCommitSubject string `json:"lastCommitSubject"`
	LastCommitAuthor  string `json:"lastCommitAuthor"`
	LastCommitDate    string `json:"lastCommitDate"`
	IsCurrent         bool   `json:"isCurrent"`
	AheadCount        int    `json:"aheadCount"`
	BehindCount       int    `json:"behindCount"`
	StatusWarning     string `json:"statusWarning,omitempty"`
}

type StudioSwitchboardResult struct {
	ActionExecuted    string              `json:"actionExecuted"`
	Success           bool                `json:"success"`
	Message           string              `json:"message"`
	ActiveProjectPath string              `json:"activeProjectPath"`
	TotalProjects     int                 `json:"totalProjects"`
	LinkedWorktrees   int                 `json:"linkedWorktrees"`
	AdjacentRepos     int                 `json:"adjacentRepos"`
	Projects          []StudioProjectNode `json:"projects"`
}

// ============================================================================
// Round 112: Game Design Data Table Validator (Tencent UGit Parity)
// ============================================================================

type SchemaChangeItem struct {
	ChangeType  string `json:"changeType"` // "COLUMN_ADDED", "COLUMN_REMOVED", "TYPE_ALTERED", "COLUMN_RENAMED"
	ColumnName  string `json:"columnName"`
	OldType     string `json:"oldType,omitempty"`
	NewType     string `json:"newType,omitempty"`
	Description string `json:"description"`
}

type TableAnomalyItem struct {
	RowIndex    int    `json:"rowIndex"`
	PrimaryKey  string `json:"primaryKey"`
	ColumnName  string `json:"columnName"`
	AnomalyType string `json:"anomalyType"` // "DUPLICATE_KEY", "INVALID_FK", "NAN_OR_NULL", "TYPE_MISMATCH", "OUT_OF_BOUNDS"
	InvalidVal  string `json:"invalidVal"`
	Severity    string `json:"severity"` // "CRITICAL", "WARNING", "INFO"
	Description string `json:"description"`
}

type FieldSemanticDiff struct {
	FieldName     string  `json:"fieldName"`
	OldValue      string  `json:"oldValue"`
	NewValue      string  `json:"newValue"`
	NumericDelta  float64 `json:"numericDelta,omitempty"`
	PercentChange float64 `json:"percentChange,omitempty"`
	Summary       string  `json:"summary"`
}

type RowSemanticDiff struct {
	PrimaryKey string              `json:"primaryKey"`
	Action     string              `json:"action"` // "ADDED", "MODIFIED", "DELETED"
	FieldDiffs []FieldSemanticDiff `json:"fieldDiffs"`
	Summary    string              `json:"summary"`
}

type AuditedTableReport struct {
	TableName         string             `json:"tableName"`
	FilePath          string             `json:"filePath"`
	TotalRowsBase     int                `json:"totalRowsBase"`
	TotalRowsTarget   int                `json:"totalRowsTarget"`
	AddedRowsCount    int                `json:"addedRowsCount"`
	ModifiedRowsCount int                `json:"modifiedRowsCount"`
	DeletedRowsCount  int                `json:"deletedRowsCount"`
	SchemaChanges     []SchemaChangeItem `json:"schemaChanges"`
	Anomalies         []TableAnomalyItem `json:"anomalies"`
	RowDiffs          []RowSemanticDiff  `json:"rowDiffs"`
	SemanticSummary   string             `json:"semanticSummary"`
	HasErrors         bool               `json:"hasErrors"`
}

type DataTableAuditResult struct {
	AnalysisKnown  bool                 `json:"analysisKnown"`
	BaseRef        string               `json:"baseRef"`
	TargetRef      string               `json:"targetRef"`
	AuditedTables  []AuditedTableReport `json:"auditedTables"`
	TotalTables    int                  `json:"totalTables"`
	CriticalCount  int                  `json:"criticalCount"`
	WarningCount   int                  `json:"warningCount"`
	IsValid        bool                 `json:"isValid"`
	OverallSummary string               `json:"overallSummary"`
}

// ============================================================================
// Round 113: Studio Credential Vault & KMS (Tencent UGit Parity)
// ============================================================================

type CredentialItem struct {
	ID            string `json:"id"`
	Type          string `json:"type"`          // "TENCENT_CLOUD_AKSK", "AWS_AKSK", "SSH_KEY", "GIT_TOKEN", "TAPD_TOKEN", "PERFORCE_CRED", "GENERIC_SECRET"
	Identifier    string `json:"identifier"`    // Key name / file path / remote name
	Source        string `json:"source"`        // "GIT_CONFIG", "ENVIRONMENT", "SSH_KEYCHAIN", "STUDIO_VAULT"
	MaskedValue   string `json:"maskedValue"`   // e.g. "AKID***98a", "ghp_***4bf"
	Scope         string `json:"scope"`         // "LOCAL_REPO", "GLOBAL_SYSTEM", "ENV_VAR"
	Permissions   string `json:"permissions"`   // e.g. "0600 (Secure)", "0644 (Warning: Too Open)"
	SecurityLevel string `json:"securityLevel"` // "SECURE", "WARNING", "CRITICAL_EXPOSURE"
	ExpiresAt     string `json:"expiresAt"`     // e.g. "2027-01-01", "Permanent / Never", "Unknown"
	IsMasked      bool   `json:"isMasked"`
	StatusMessage string `json:"statusMessage"`
}

type CredentialVaultResult struct {
	TotalCredentials     int              `json:"totalCredentials"`
	CriticalExposures    int              `json:"criticalExposures"`
	WarningsCount        int              `json:"warningsCount"`
	SecureCount          int              `json:"secureCount"`
	Items                []CredentialItem `json:"items"`
	VaultStoragePath     string           `json:"vaultStoragePath"`
	KmsEncryptionEnabled bool             `json:"kmsEncryptionEnabled"`
	Summary              string           `json:"summary"`
	Message              string           `json:"message"`
}

type EncryptedVaultRecord struct {
	ID           string `json:"id"`
	SecretType   string `json:"secretType"`
	Identifier   string `json:"identifier"`
	Ciphertext   string `json:"ciphertext"` // base64 encoded AES-256-GCM payload
	Nonce        string `json:"nonce"`      // base64 encoded nonce
	MaskedValue  string `json:"maskedValue"`
	Scope        string `json:"scope"`
	CreatedAt    string `json:"createdAt"`
	LastAccessed string `json:"lastAccessed"`
}

type StoreSecretResult struct {
	Success          bool   `json:"success"`
	SecretID         string `json:"secretId"`
	SecretType       string `json:"secretType"`
	MaskedValue      string `json:"maskedValue"`
	Scope            string `json:"scope"`
	VaultFile        string `json:"vaultFile"`
	EncryptionMethod string `json:"encryptionMethod"`
	Message          string `json:"message"`
}

// ============================================================================
// Round 114: Crash Symbol Demangler & Minidump Inspector (Tencent UGit Parity)
// ============================================================================

type DemangledFrame struct {
	FrameIndex      int    `json:"frameIndex"`
	Address         string `json:"address"`
	MangledSymbol   string `json:"mangledSymbol"`
	DemangledSymbol string `json:"demangledSymbol"`
	ModuleName      string `json:"moduleName"`
	SourceFile      string `json:"sourceFile"`
	LineNumber      int    `json:"lineNumber"`
	Offset          string `json:"offset"`
	BlameAuthor     string `json:"blameAuthor,omitempty"`
	BlameEmail      string `json:"blameEmail,omitempty"`
	BlameCommit     string `json:"blameCommit,omitempty"`
	BlameDate       string `json:"blameDate,omitempty"`
	BlameSummary    string `json:"blameSummary,omitempty"`
	CodeSnippet     string `json:"codeSnippet,omitempty"`
	IsCulprit       bool   `json:"isCulprit"`
}

type CrashStackDemangleResult struct {
	CrashReason    string           `json:"crashReason"`
	FaultAddress   string           `json:"faultAddress"`
	FaultModule    string           `json:"faultModule"`
	ThreadID       string           `json:"threadId"`
	TotalFrames    int              `json:"totalFrames"`
	DemangledCount int              `json:"demangledCount"`
	Frames         []DemangledFrame `json:"frames"`
	CulpritFrame   *DemangledFrame  `json:"culpritFrame,omitempty"`
	CulpritAuthor  string           `json:"culpritAuthor"`
	CulpritCommit  string           `json:"culpritCommit"`
	CulpritFile    string           `json:"culpritFile"`
	Summary        string           `json:"summary"`
	Message        string           `json:"message"`
}

// ============================================================================
// Round 115: Selective Asset Cherry-Picker (Tencent UGit Parity)
// ============================================================================

type CherryPickedAsset struct {
	Path         string `json:"path"`
	FileType     string `json:"fileType"`
	SizeBytes    int64  `json:"sizeBytes"`
	SizeStr      string `json:"sizeStr"`
	IsLFS        bool   `json:"isLfs"`
	IsDependency bool   `json:"isDependency"`
	ParentAsset  string `json:"parentAsset,omitempty"`
	Status       string `json:"status"`
}

type AssetCherryPickResult struct {
	TargetRef         string              `json:"targetRef"`
	IsDryRun          bool                `json:"isDryRun"`
	TotalRequested    int                 `json:"totalRequested"`
	TotalDependencies int                 `json:"totalDependencies"`
	TotalAssetsCount  int                 `json:"totalAssetsCount"`
	TotalBytes        int64               `json:"totalBytes"`
	TotalBytesStr     string              `json:"totalBytesStr"`
	Assets            []CherryPickedAsset `json:"assets"`
	LfsAssetsCount    int                 `json:"lfsAssetsCount"`
	MetaFilesCount    int                 `json:"metaFilesCount"`
	Success           bool                `json:"success"`
	Summary           string              `json:"summary"`
	Message           string              `json:"message"`
}

// ============================================================================
// Round 116: Missing Material & Pink Shader Guard (Tencent UGit Parity)
// ============================================================================

type MissingTextureRef struct {
	PropertySlot string `json:"propertySlot"`
	TextureGuid  string `json:"textureGuid"`
	ExpectedPath string `json:"expectedPath"`
	Severity     string `json:"severity"`
}

type BrokenMaterialItem struct {
	MaterialPath    string              `json:"materialPath"`
	MaterialName    string              `json:"materialName"`
	EngineType      string              `json:"engineType"`
	IssueType       string              `json:"issueType"`
	ShaderName      string              `json:"shaderName"`
	ShaderGuid      string              `json:"shaderGuid,omitempty"`
	IsPinkShader    bool                `json:"isPinkShader"`
	MissingTextures []MissingTextureRef `json:"missingTextures"`
	SuggestedShader string              `json:"suggestedShader"`
	AffectedPrefabs []string            `json:"affectedPrefabs"`
	Recommendation  string              `json:"recommendation"`
}

type MissingMaterialScanResult struct {
	TargetDirectory       string               `json:"targetDirectory"`
	EngineType            string               `json:"engineType"`
	TotalMaterialsScanned int                  `json:"totalMaterialsScanned"`
	BrokenMaterialsCount  int                  `json:"brokenMaterialsCount"`
	PinkShaderCount       int                  `json:"pinkShaderCount"`
	MissingTexturesCount  int                  `json:"missingTexturesCount"`
	BrokenMaterials       []BrokenMaterialItem `json:"brokenMaterials"`
	HealthScore           int                  `json:"healthScore"`
	Summary               string               `json:"summary"`
	Message               string               `json:"message"`
}

// ============================================================================
// Round 117: Offline Replay & Retry Queue (Tencent UGit Parity)
// ============================================================================

type OfflineQueueItem struct {
	ID          string `json:"id"`
	Operation   string `json:"operation"` // "push", "lfs_lock", "lfs_unlock", "fetch", "lfs_push"
	Target      string `json:"target"`    // Branch name, asset path, or remote target
	Payload     string `json:"payload,omitempty"`
	Status      string `json:"status"` // "queued", "running", "succeeded", "failed", "paused", "discarded"
	RetryCount  int    `json:"retryCount"`
	MaxRetries  int    `json:"maxRetries"`
	BackoffSecs int    `json:"backoffSecs"`
	CreatedAt   string `json:"createdAt"`
	LastAttempt string `json:"lastAttempt,omitempty"`
	NextRetryAt string `json:"nextRetryAt,omitempty"`
	LastError   string `json:"lastError,omitempty"`
}

type OfflineQueueResult struct {
	Items          []OfflineQueueItem `json:"items"`
	TotalCount     int                `json:"totalCount"`
	QueuedCount    int                `json:"queuedCount"`
	FailedCount    int                `json:"failedCount"`
	PausedCount    int                `json:"pausedCount"`
	SucceededCount int                `json:"succeededCount"`
	IsOnline       bool               `json:"isOnline"`
	NetworkSummary string             `json:"networkSummary"`
	Message        string             `json:"message"`
}

// ============================================================================
// Round 118: Pre-Push Studio Policy Checklist (Tencent UGit Parity)
// ============================================================================

type PolicyViolationItem struct {
	RuleID      string `json:"ruleId"`   // "RULE_COMMIT_CONVENTION", "RULE_LFS_POINTER_INTEGRITY", "RULE_NO_LARGE_BLOBS", "RULE_COMMIT_SIGNATURE", "RULE_BRANCH_STATUS"
	Severity    string `json:"severity"` // "BLOCKING", "WARNING", "INFO"
	Title       string `json:"title"`
	Description string `json:"description"`
	CommitHash  string `json:"commitHash,omitempty"`
	FilePath    string `json:"filePath,omitempty"`
	Remediation string `json:"remediation"`
}

type PolicyRuleCheckResult struct {
	RuleID   string `json:"ruleId"`
	RuleName string `json:"ruleName"`
	Passed   bool   `json:"passed"`
	Severity string `json:"severity"`
	Details  string `json:"details"`
}

type PrePushChecklistResult struct {
	AnalysisKnown      bool                    `json:"analysisKnown"`
	BranchName         string                  `json:"branchName"`
	RemoteTracking     string                  `json:"remoteTracking"`
	CommitsAudited     int                     `json:"commitsAudited"`
	IsBlocked          bool                    `json:"isBlocked"`
	PassReadinessScore int                     `json:"passReadinessScore"` // 0 - 100
	ReadinessGrade     string                  `json:"readinessGrade"`     // "PASS", "PASS_WITH_WARNINGS", "BLOCKED"
	RulesChecked       []PolicyRuleCheckResult `json:"rulesChecked"`
	Violations         []PolicyViolationItem   `json:"violations"`
	Summary            string                  `json:"summary"`
	Message            string                  `json:"message"`
}

// ============================================================================
// Round 119: LOD Tier & Streaming Mip Audit (Tencent UGit Parity)
// ============================================================================

type MeshLodTier struct {
	LodIndex           int     `json:"lodIndex"` // 0, 1, 2, 3
	TriangleCount      int     `json:"triangleCount"`
	TriangleCountKnown bool    `json:"triangleCountKnown"`
	ReductionPercent   float64 `json:"reductionPercent"`   // e.g. 100%, 50%, 25%, 12.5%
	ScreenRelativeSize float64 `json:"screenRelativeSize"` // e.g. 1.0, 0.6, 0.3, 0.15
}

type MeshLodAuditItem struct {
	Path               string        `json:"path"`
	MeshType           string        `json:"meshType"` // "StaticMesh", "SkeletalMesh"
	BaseTriangles      int           `json:"baseTriangles"`
	TriangleCountKnown bool          `json:"triangleCountKnown"`
	LodTiers           []MeshLodTier `json:"lodTiers"`
	MaxLodLevel        int           `json:"maxLodLevel"`
	HasCompleteLODs    bool          `json:"hasCompleteLods"` // Has at least LOD0..LOD2 or LOD3
	Severity           string        `json:"severity"`        // "OK", "WARNING", "CRITICAL"
	Recommendation     string        `json:"recommendation"`
}

type TextureStreamingAuditItem struct {
	Path             string `json:"path"`
	DimensionsKnown  bool   `json:"dimensionsKnown"`
	Width            int    `json:"width"`
	Height           int    `json:"height"`
	IsPOT            bool   `json:"isPot"`
	MipCount         int    `json:"mipCount"`
	StreamingEnabled bool   `json:"streamingEnabled"`
	VirtualTexture   bool   `json:"virtualTexture"` // VT enabled for >= 4096
	RawSizeBytes     int64  `json:"rawSizeBytes"`
	StreamSizeBytes  int64  `json:"streamSizeBytes"`
	Severity         string `json:"severity"` // "OK", "WARNING", "CRITICAL"
	Issue            string `json:"issue,omitempty"`
}

type LodStreamingAuditResult struct {
	AnalysisKnown           bool                        `json:"analysisKnown"`
	TotalMeshesAudited      int                         `json:"totalMeshesAudited"`
	LODCompliantCount       int                         `json:"lodCompliantCount"`
	LODMissingCount         int                         `json:"lodMissingCount"`
	LODUnknownCount         int                         `json:"lodUnknownCount"`
	TotalTexturesAudited    int                         `json:"totalTexturesAudited"`
	StreamingCompliantCount int                         `json:"streamingCompliantCount"`
	StreamingMissingCount   int                         `json:"streamingMissingCount"`
	StreamingUnknownCount   int                         `json:"streamingUnknownCount"`
	BaselineVramMB          float64                     `json:"baselineVramMb"`
	StreamingVramMB         float64                     `json:"streamingVramMb"`
	PotentialSavingsMB      float64                     `json:"potentialSavingsMb"`
	SavingsPercentage       float64                     `json:"savingsPercentage"`
	OverallHealthScore      int                         `json:"overallHealthScore"` // 0 - 100
	Meshes                  []MeshLodAuditItem          `json:"meshes"`
	Textures                []TextureStreamingAuditItem `json:"textures"`
	Recommendations         []string                    `json:"recommendations"`
	Summary                 string                      `json:"summary"`
}

// ============================================================================
// Round 120: Grand Tencent UGit 100% Full Parity Hub (Tencent UGit Parity)
// ============================================================================

type ParityDomainSummary struct {
	DomainID        string   `json:"domainId"`
	DomainName      string   `json:"domainName"`
	DomainEnName    string   `json:"domainEnName"`
	TotalModules    int      `json:"totalModules"`   // 24
	CompletedCount  int      `json:"completedCount"` // 24
	CompletionRate  float64  `json:"completionRate"` // 100.0
	Highlights      []string `json:"highlights"`
	KeyCapabilities []string `json:"keyCapabilities"`
}

type FullParityStatusSummary struct {
	TotalRounds            int                   `json:"totalRounds"`            // 120
	CompletedRounds        int                   `json:"completedRounds"`        // 120
	ParityPercentage       float64               `json:"parityPercentage"`       // 100.0
	CertificationBadge     string                `json:"certificationBadge"`     // "Tencent UGit 100% Full Parity Gold Standard"
	SystemReadiness        string                `json:"systemReadiness"`        // "PRODUCTION_CERTIFIED"
	Domains                []ParityDomainSummary `json:"domains"`                // 5 domains, 24 modules each
	TotalTestCoverageCount int                   `json:"totalTestCoverageCount"` // 120+ tests
	GoEngineVersion        string                `json:"goEngineVersion"`
	ClientPlatform         string                `json:"clientPlatform"`
	ArchitectureSummary    string                `json:"architectureSummary"`
	CelebrationMessage     string                `json:"celebrationMessage"`
	ReleaseSignature       string                `json:"releaseSignature"`
}

type CommitPaginationResult struct {
	Commits    []CommitItem `json:"commits"`
	TotalCount int          `json:"totalCount"`
	HasMore    bool         `json:"hasMore"`
}

type DiffComparisonResult struct {
	RefA      string       `json:"refA"`
	RefB      string       `json:"refB"`
	Files     []FileStatus `json:"files"`
	Additions int          `json:"additions"`
	Deletions int          `json:"deletions"`
	Diff      string       `json:"diff"`
}

type BranchInfo struct {
	Name       string `json:"name"`
	IsCurrent  bool   `json:"isCurrent"`
	IsRemote   bool   `json:"isRemote"`
	CommitHash string `json:"commitHash"`
	Upstream   string `json:"upstream"`
	Ahead      int    `json:"ahead"`
	Behind     int    `json:"behind"`
}

type TagInfo struct {
	Name string `json:"name"`
	// IsAnnotated is true for tags created with -a/-s. Lightweight tags are
	// indistinguishable by message alone: %(contents:subject) falls back to the
	// commit subject, so every tag looks annotated until the ref's object type
	// is checked.
	IsAnnotated bool `json:"isAnnotated"`
	// CommitHash is always the commit the tag resolves to, even for annotated
	// tags, whose ref itself points at a tag object with a different OID.
	CommitHash string `json:"commitHash"`
	Message    string `json:"message"`
}

type StashInfo struct {
	Index   int    `json:"index"`
	Message string `json:"message"`
	Date    string `json:"date"`
}

type RepoInfo struct {
	Path          string     `json:"path"`
	Name          string     `json:"name"`
	CurrentBranch string     `json:"currentBranch"`
	Status        RepoStatus `json:"status"`
	Remotes       []string   `json:"remotes"`
}

type ReflogItem struct {
	Selector   string `json:"selector"`
	CommitHash string `json:"commitHash"`
	Action     string `json:"action"`
	Message    string `json:"message"`
	Date       string `json:"date"`
}

type BisectStatus struct {
	IsActive       bool     `json:"isActive"`
	CurrentHash    string   `json:"currentHash"`
	CurrentStep    string   `json:"currentStep"`
	BadRef         string   `json:"badRef"`
	GoodRefs       []string `json:"goodRefs"`
	Log            string   `json:"log"`
	CurrentSubject string   `json:"currentSubject,omitempty"`
	CurrentAuthor  string   `json:"currentAuthor,omitempty"`
	CurrentDate    string   `json:"currentDate,omitempty"`
}

type BisectRunResult struct {
	Success        bool          `json:"success"`
	FirstBadCommit string        `json:"firstBadCommit"`
	Log            string        `json:"log"`
	ExitCode       int           `json:"exitCode"`
	Message        string        `json:"message"`
	Status         *BisectStatus `json:"status"`
}

type WorktreeInfo struct {
	Path        string `json:"path"`
	Head        string `json:"head"`
	Branch      string `json:"branch"`
	IsBare      bool   `json:"isBare"`
	IsMain      bool   `json:"isMain"`
	IsLocked    bool   `json:"isLocked"`
	LockReason  string `json:"lockReason"`
	IsPrunable  bool   `json:"isPrunable"`
	PruneReason string `json:"pruneReason"`
	IsDetached  bool   `json:"isDetached"`
	DirtyCount  int    `json:"dirtyCount"`
}

type WorktreeStatus struct {
	Path       string `json:"path"`
	IsDirty    bool   `json:"isDirty"`
	DirtyCount int    `json:"dirtyCount"`
	Summary    string `json:"summary"`
}

type AddWorktreeAdvancedOptions struct {
	Path      string `json:"path"`
	Branch    string `json:"branch"`
	NewBranch bool   `json:"newBranch"`
	BaseRef   string `json:"baseRef"`
	Detach    bool   `json:"detach"`
}

type LfsTrackPattern struct {
	Pattern string `json:"pattern"`
	Source  string `json:"source"`
}

type RepoMaintenanceStats struct {
	GitDirSize        string   `json:"gitDirSize"`
	PackFilesCount    int      `json:"packFilesCount"`
	LooseObjects      int      `json:"looseObjects"`
	Success           bool     `json:"success"`
	RemotePruneStatus string   `json:"remotePruneStatus"` // OK | SKIPPED | FAILED
	RemotePruneNote   string   `json:"remotePruneNote"`
	GcStatus          string   `json:"gcStatus"`    // OK | FAILED
	PruneStatus       string   `json:"pruneStatus"` // OK | FAILED
	Warnings          []string `json:"warnings"`
	Summary           string   `json:"summary"`
}

type GitVersionInfo struct {
	Version string `json:"version"`
	System  string `json:"system"`
}

type RebaseTodoItem struct {
	Action    string `json:"action"` // "pick", "reword", "edit", "squash", "fixup", "drop"
	Hash      string `json:"hash"`
	ShortHash string `json:"shortHash"`
	Subject   string `json:"subject"`
	Author    string `json:"author"`
}

type SignatureDetails struct {
	Status      string `json:"status"` // "G", "B", "U", "N"
	Signer      string `json:"signer"`
	KeyID       string `json:"keyId"`
	Fingerprint string `json:"fingerprint"`
	Date        string `json:"date"`
	Raw         string `json:"raw"`
}

type StorageBreakdownItem struct {
	Extension string `json:"extension"`
	Count     int    `json:"count"`
	SizeBytes int64  `json:"sizeBytes"`
	SizeHuman string `json:"sizeHuman"`
}

type RemoteTestResult struct {
	Remote  string   `json:"remote"`
	Success bool     `json:"success"`
	Latency string   `json:"latency"`
	Heads   []string `json:"heads"`
	Error   string   `json:"error,omitempty"`
}

type AssetMetadata struct {
	Path            string `json:"path"`
	Extension       string `json:"extension"`
	SizeBytes       int64  `json:"sizeBytes"`
	SizeHuman       string `json:"sizeHuman"`
	IsLFS           bool   `json:"isLfs"`
	LFSPointer      string `json:"lfsPointer,omitempty"`
	LFSPointerError string `json:"lfsPointerError,omitempty"`
	Dimensions      string `json:"dimensions,omitempty"`
	Duration        string `json:"duration,omitempty"`
	MimeType        string `json:"mimeType"`
	LastModTime     string `json:"lastModTime"`
}

type BranchCompareResult struct {
	BaseBranch    string       `json:"baseBranch"`
	TargetBranch  string       `json:"targetBranch"`
	AheadCount    int          `json:"aheadCount"`
	BehindCount   int          `json:"behindCount"`
	AheadCommits  []CommitItem `json:"aheadCommits"`
	BehindCommits []CommitItem `json:"behindCommits"`
}

type TagDetail struct {
	Name        string `json:"name"`
	CommitHash  string `json:"commitHash"`
	TaggerName  string `json:"taggerName"`
	TaggerEmail string `json:"taggerEmail"`
	Date        string `json:"date"`
	Message     string `json:"message"`
	IsAnnotated bool   `json:"isAnnotated"`
}
type AuthDiagnosticResult struct {
	SSHKeysFound     []string `json:"sshKeysFound"`
	CredentialHelper string   `json:"credentialHelper"`
	Host             string   `json:"host"`
	SSHAuthSuccess   bool     `json:"sshAuthSuccess"`
	SSHAuthOutput    string   `json:"sshAuthOutput"`
}

type LfsFileRevision struct {
	CommitHash  string `json:"commitHash"`
	ShortHash   string `json:"shortHash"`
	AuthorName  string `json:"authorName"`
	AuthorEmail string `json:"authorEmail"`
	Date        string `json:"date"`
	Subject     string `json:"subject"`
	Oid         string `json:"oid,omitempty"`
	SizeBytes   int64  `json:"sizeBytes,omitempty"`
	SizeHuman   string `json:"sizeHuman,omitempty"`
}

type GrepMatchItem struct {
	Path        string `json:"path"`
	LineNumber  int    `json:"lineNumber"`
	LineContent string `json:"lineContent"`
}

type CheckIgnoreResult struct {
	SourceFile string `json:"sourceFile"`
	LineNumber int    `json:"lineNumber"`
	Pattern    string `json:"pattern"`
	TargetFile string `json:"targetFile"`
	IsIgnored  bool   `json:"isIgnored"`
}

type GitFsckResult struct {
	IsClean        bool     `json:"isClean"`
	DanglingBlob   int      `json:"danglingBlob"`
	DanglingTree   int      `json:"danglingTree"`
	DanglingCommit int      `json:"danglingCommit"`
	RawErrors      []string `json:"rawErrors"`
}

type ConfigEntry struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	Scope string `json:"scope"`
}

type CommitTopologyStats struct {
	TotalCommits  int     `json:"totalCommits"`
	MergeCommits  int     `json:"mergeCommits"`
	DirectCommits int     `json:"directCommits"`
	MergeRatio    float64 `json:"mergeRatio"`
}

type UnityMetaMismatch struct {
	AssetPath string `json:"assetPath"`
	MetaPath  string `json:"metaPath"`
	IssueType string `json:"issueType"` // "missing_meta", "orphan_meta", "unpaired_staged"
	Message   string `json:"message"`
}

type LfsPruneResult struct {
	DryRun       bool   `json:"dryRun"`
	Output       string `json:"output"`
	PrunedFiles  int    `json:"prunedFiles"`
	ReclaimedStr string `json:"reclaimedStr"`
	Success      bool   `json:"success"`
}

type BulkLockResult struct {
	LockedFiles  []string `json:"lockedFiles"`
	SkippedFiles []string `json:"skippedFiles"`
	FailedFiles  []string `json:"failedFiles"`
	TotalCount   int      `json:"totalCount"`
	Message      string   `json:"message"`
}

type GameEngineInfo struct {
	EngineType        string `json:"engineType"`
	HasGitAttributes  bool   `json:"hasGitAttributes"`
	HasLfsRules       bool   `json:"hasLfsRules"`
	ProjectName       string `json:"projectName"`
	EngineVersion     string `json:"engineVersion"`
	RecommendedPreset string `json:"recommendedPreset"`
}

// ParseRepoStatus parses `git status --porcelain=v1 -b` output
func ParseRepoStatus(repoPath, raw string) RepoStatus {
	status := RepoStatus{
		StagedFiles:     []FileStatus{},
		UnstagedFiles:   []FileStatus{},
		UntrackedFiles:  []FileStatus{},
		ConflictedFiles: []string{},
	}

	lines := strings.Split(raw, "\n")
	lfsExtensions := getLFSExtensions(repoPath)

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		if strings.HasPrefix(line, "## ") {
			// Branch header: ## main...origin/main [ahead 1, behind 2]
			branchPart := strings.TrimPrefix(line, "## ")
			if strings.HasPrefix(branchPart, "No commits yet on ") {
				status.CurrentBranch = strings.TrimPrefix(branchPart, "No commits yet on ")
				status.IsInitialCommit = true
				continue
			}
			if strings.HasPrefix(branchPart, "Initial commit on ") {
				status.CurrentBranch = strings.TrimPrefix(branchPart, "Initial commit on ")
				status.IsInitialCommit = true
				continue
			}

			parts := strings.Split(branchPart, "...")
			status.CurrentBranch = parts[0]
			if len(parts) > 1 {
				remoteAndAhead := parts[1]
				bracketIdx := strings.Index(remoteAndAhead, " [")
				if bracketIdx != -1 {
					status.TrackingBranch = remoteAndAhead[:bracketIdx]
					details := strings.Trim(remoteAndAhead[bracketIdx+2:], "]")
					// parse ahead / behind
					for _, item := range strings.Split(details, ", ") {
						if strings.HasPrefix(item, "ahead ") {
							status.Ahead, _ = strconv.Atoi(strings.TrimPrefix(item, "ahead "))
						} else if strings.HasPrefix(item, "behind ") {
							status.Behind, _ = strconv.Atoi(strings.TrimPrefix(item, "behind "))
						}
					}
				} else {
					status.TrackingBranch = remoteAndAhead
				}
			}
			continue
		}

		if len(line) < 3 {
			continue
		}

		indexCode := line[0]
		workTreeCode := line[1]
		filePath := strings.TrimSpace(line[3:])

		var oldPath string
		if strings.Contains(filePath, " -> ") {
			pts := strings.Split(filePath, " -> ")
			oldPath = pts[0]
			filePath = pts[1]
		}
		// Clean quotes if git quoted spaces or utf8
		filePath = strings.Trim(filePath, "\"")

		isLfs := isPathLFS(filePath, lfsExtensions)
		isBinary := isPathBinary(filePath)
		fileSizeStr := ""
		fullPath := filepath.Join(repoPath, filePath)
		if fi, err := os.Stat(fullPath); err == nil && !fi.IsDir() {
			sz := fi.Size()
			if sz < 1024 {
				fileSizeStr = strconv.FormatInt(sz, 10) + " B"
			} else if sz < 1024*1024 {
				fileSizeStr = strconv.FormatInt(sz/1024, 10) + " KB"
			} else {
				fileSizeStr = fmt.Sprintf("%.1f MB", float64(sz)/(1024*1024))
			}
		}

		// Conflict
		if (indexCode == 'U' || workTreeCode == 'U') ||
			(indexCode == 'A' && workTreeCode == 'A') ||
			(indexCode == 'D' && workTreeCode == 'D') {
			status.ConflictedFiles = append(status.ConflictedFiles, filePath)
		}

		// Untracked
		if indexCode == '?' && workTreeCode == '?' {
			status.UntrackedFiles = append(status.UntrackedFiles, FileStatus{
				Path:           filePath,
				UnstagedStatus: "?",
				IsLFS:          isLfs,
				IsBinary:       isBinary,
				Size:           fileSizeStr,
			})
			continue
		}

		// Staged
		if indexCode != ' ' && indexCode != '?' {
			status.StagedFiles = append(status.StagedFiles, FileStatus{
				Path:         filePath,
				OldPath:      oldPath,
				StagedStatus: string(indexCode),
				IsStaged:     true,
				IsLFS:        isLfs,
				IsBinary:     isBinary,
				Size:         fileSizeStr,
			})
		}

		// Unstaged
		if workTreeCode != ' ' && workTreeCode != '?' {
			status.UnstagedFiles = append(status.UnstagedFiles, FileStatus{
				Path:           filePath,
				OldPath:        oldPath,
				UnstagedStatus: string(workTreeCode),
				IsStaged:       false,
				IsLFS:          isLfs,
				IsBinary:       isBinary,
				Size:           fileSizeStr,
			})
		}
	}

	status.IsClean = len(status.StagedFiles) == 0 &&
		len(status.UnstagedFiles) == 0 &&
		len(status.UntrackedFiles) == 0 &&
		len(status.ConflictedFiles) == 0

	// Detect ongoing Merge
	stateDir := gitStateDir(repoPath)
	if _, err := os.Stat(filepath.Join(stateDir, "MERGE_HEAD")); err == nil {
		status.IsMerging = true
		if msgData, rErr := os.ReadFile(filepath.Join(stateDir, "MERGE_MSG")); rErr == nil {
			firstLine := strings.Split(string(msgData), "\n")[0]
			if strings.Contains(firstLine, "Merge branch '") {
				pts := strings.Split(firstLine, "'")
				if len(pts) >= 2 {
					status.MergeHeadBranch = pts[1]
				}
			} else if strings.Contains(firstLine, "Merge remote-tracking branch '") {
				pts := strings.Split(firstLine, "'")
				if len(pts) >= 2 {
					status.MergeHeadBranch = pts[1]
				}
			}
		}
	}

	// Detect ongoing Rebase
	rebaseMergeDir := filepath.Join(stateDir, "rebase-merge")
	rebaseApplyDir := filepath.Join(stateDir, "rebase-apply")
	if fi, err := os.Stat(rebaseMergeDir); err == nil && fi.IsDir() {
		status.IsRebasing = true
		if hn, rErr := os.ReadFile(filepath.Join(rebaseMergeDir, "head-name")); rErr == nil {
			status.RebaseHeadName = strings.TrimPrefix(strings.TrimSpace(string(hn)), "refs/heads/")
		}
		if msgNum, rErr := os.ReadFile(filepath.Join(rebaseMergeDir, "msgnum")); rErr == nil {
			status.RebaseStep, _ = strconv.Atoi(strings.TrimSpace(string(msgNum)))
		}
		if endNum, rErr := os.ReadFile(filepath.Join(rebaseMergeDir, "end")); rErr == nil {
			status.RebaseTotal, _ = strconv.Atoi(strings.TrimSpace(string(endNum)))
		}
	} else if fi, err := os.Stat(rebaseApplyDir); err == nil && fi.IsDir() {
		status.IsRebasing = true
		if hn, rErr := os.ReadFile(filepath.Join(rebaseApplyDir, "head-name")); rErr == nil {
			status.RebaseHeadName = strings.TrimPrefix(strings.TrimSpace(string(hn)), "refs/heads/")
		}
		if msgNum, rErr := os.ReadFile(filepath.Join(rebaseApplyDir, "next")); rErr == nil {
			status.RebaseStep, _ = strconv.Atoi(strings.TrimSpace(string(msgNum)))
		}
		if endNum, rErr := os.ReadFile(filepath.Join(rebaseApplyDir, "last")); rErr == nil {
			status.RebaseTotal, _ = strconv.Atoi(strings.TrimSpace(string(endNum)))
		}
	}

	// Detect ongoing Cherry-Pick
	if _, err := os.Stat(filepath.Join(stateDir, "CHERRY_PICK_HEAD")); err == nil {
		status.IsCherryPicking = true
	}

	return status
}

func getLFSExtensions(repoPath string) map[string]bool {
	exts := make(map[string]bool)
	gitattributes, err := ExecGit(repoPath, "config", "--get-regexp", "filter\\.lfs")
	if err == nil && gitattributes != "" {
		// Repo uses LFS
	}
	// Common large file extensions or check .gitattributes
	attrContent, err := ExecGit(repoPath, "check-attr", "--all", "--", "*")
	if err == nil {
		for _, line := range strings.Split(attrContent, "\n") {
			if strings.Contains(line, "filter: lfs") {
				parts := strings.Split(line, ":")
				if len(parts) > 0 {
					ext := filepath.Ext(strings.TrimSpace(parts[0]))
					if ext != "" {
						exts[ext] = true
					}
				}
			}
		}
	}
	// Default popular LFS asset types in game & large media development
	defaultLfs := []string{".psd", ".fbx", ".obj", ".blend", ".uasset", ".umap", ".mp4", ".mov", ".zip", ".tar.gz", ".png", ".jpg"}
	for _, e := range defaultLfs {
		exts[e] = true
	}
	return exts
}

func isPathLFS(path string, exts map[string]bool) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return exts[ext]
}

func isPathBinary(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	binExts := map[string]bool{
		".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".ico": true,
		".webp": true, ".pdf": true, ".zip": true, ".tar": true, ".gz": true,
		".exe": true, ".dmg": true, ".dll": true, ".so": true, ".dylib": true,
		".wasm": true, ".bin": true, ".mp4": true, ".mov": true, ".mp3": true,
		".wav": true, ".fbx": true, ".obj": true, ".blend": true, ".uasset": true,
		".umap": true, ".psd": true, ".ai": true, ".ttf": true, ".otf": true,
		".woff": true, ".woff2": true,
	}
	return binExts[ext]
}

// ============================================================================
// Round 121: Wwise / FMOD SoundBank Difference Inspector (Tencent UGit Parity)
// ============================================================================

type AudioEventDiffItem struct {
	EventName      string  `json:"eventName"`
	ChangeType     string  `json:"changeType"` // "ADDED", "REMOVED", "MODIFIED"
	OldDurationSec float64 `json:"oldDurationSec"`
	NewDurationSec float64 `json:"newDurationSec"`
	OldSizeBytes   int64   `json:"oldSizeBytes"`
	NewSizeBytes   int64   `json:"newSizeBytes"`
	IsLooping      bool    `json:"isLooping"`
}

type OversizedAudioItem struct {
	Path      string `json:"path"`
	SizeBytes int64  `json:"sizeBytes"`
	SizeStr   string `json:"sizeStr"`
	Format    string `json:"format"`
	Advice    string `json:"advice"`
}

type AudioBankDiffResult struct {
	EngineType        string               `json:"engineType"` // "Wwise", "FMOD", "Standard"
	BankName          string               `json:"bankName"`
	TotalEventsBase   int                  `json:"totalEventsBase"`
	TotalEventsTarget int                  `json:"totalEventsTarget"`
	AddedEvents       []string             `json:"addedEvents"`
	RemovedEvents     []string             `json:"removedEvents"`
	ModifiedEvents    []AudioEventDiffItem `json:"modifiedEvents"`
	OrphanEvents      []string             `json:"orphanEvents"`
	OversizedAudios   []OversizedAudioItem `json:"oversizedAudios"`
	BaseSizeBytes     int64                `json:"baseSizeBytes"`
	TargetSizeBytes   int64                `json:"targetSizeBytes"`
	DeltaSizeBytes    int64                `json:"deltaSizeBytes"`
	DeltaSizeStr      string               `json:"deltaSizeStr"`
	Summary           string               `json:"summary"`
}

// ============================================================================
// Round 122: Physics Material & Mesh Collision Auditor (Tencent UGit Parity)
// ============================================================================

type ConvexHullItem struct {
	MeshPath     string `json:"meshPath"`
	VertexCount  int    `json:"vertexCount"`
	PolygonCount int    `json:"polygonCount"`
	IsOverbudget bool   `json:"isOverbudget"` // > 256 vertices
	Severity     string `json:"severity"`     // "OK", "WARNING", "CRITICAL"
}

type PhysicsCollisionAuditResult struct {
	AnalysisKnown             bool             `json:"analysisKnown"`
	TotalColliders            int              `json:"totalColliders"`
	ConvexHullsCount          int              `json:"convexHullsCount"`
	ConcaveMeshCount          int              `json:"concaveMeshCount"`
	OverbudgetHulls           []ConvexHullItem `json:"overbudgetHulls"`
	NonConvexDynamicColliders []string         `json:"nonConvexDynamicColliders"`
	MissingPhysicMaterials    []string         `json:"missingPhysicMaterials"`
	HealthScore               int              `json:"healthScore"` // 0-100
	Summary                   string           `json:"summary"`
}

// ============================================================================
// Round 123: Unreal World Partition Data Layers Diff (Tencent UGit Parity)
// ============================================================================

type DataLayerDiffItem struct {
	Name                string   `json:"name"`
	ChangeType          string   `json:"changeType,omitempty"` // "ADDED", "MODIFIED", "REMOVED"
	IsRuntime           bool     `json:"isRuntime"`
	InitialRuntimeState string   `json:"initialRuntimeState"` // "Activated", "Loaded", "Unloaded", "Unknown"
	AddedActors         []string `json:"addedActors"`
	RemovedActors       []string `json:"removedActors"`
	ActorCount          int      `json:"actorCount"`
	IsConflicted        bool     `json:"isConflicted"`
}

type WorldDataLayersDiffResult struct {
	TotalDataLayers      int                 `json:"totalDataLayers"`
	Layers               []DataLayerDiffItem `json:"layers"`
	RuntimeBudgetOverrun bool                `json:"runtimeBudgetOverrun"`
	StreamingConflicts   []string            `json:"streamingConflicts"`
	Summary              string              `json:"summary"`
}

// ============================================================================
// Round 124: Material Instance Parameter Overrides Inspector (Tencent UGit Parity)
// ============================================================================

type MaterialParamOverride struct {
	ParamName string `json:"paramName"`
	ParamType string `json:"paramType"` // "Texture", "Scalar", "Vector"
	Value     string `json:"value"`
	IsDead    bool   `json:"isDead"` // True if parameter does not exist in parent shader
}

type MaterialInstanceItem struct {
	InstancePath   string                  `json:"instancePath"`
	ParentMaterial string                  `json:"parentMaterial"`
	ParentShader   string                  `json:"parentShader"`
	Overrides      []MaterialParamOverride `json:"overrides"`
	DeadCount      int                     `json:"deadCount"`
	IsHealthy      bool                    `json:"isHealthy"`
}

type MaterialInstanceOverrideResult struct {
	TotalInstances            int                    `json:"totalInstances"`
	OverriddenParametersCount int                    `json:"overriddenParametersCount"`
	DeadParametersCount       int                    `json:"deadParametersCount"`
	Instances                 []MaterialInstanceItem `json:"instances"`
	Summary                   string                 `json:"summary"`
}

// ============================================================================
// Round 125: Scripting Defines & Symbols Cross-Branch Auditor (Tencent UGit Parity)
// ============================================================================

type SymbolsDefinesAuditResult struct {
	AnalysisKnown         bool     `json:"analysisKnown"`
	TotalDefines          int      `json:"totalDefines"`
	ActiveDefines         []string `json:"activeDefines"`
	BaseDefines           []string `json:"baseDefines"`
	TargetDefines         []string `json:"targetDefines"`
	AddedDefines          []string `json:"addedDefines"`
	RemovedDefines        []string `json:"removedDefines"`
	DangerousDebugDefines []string `json:"dangerousDebugDefines"`
	Severity              string   `json:"severity"` // "OK", "WARNING", "CRITICAL"
	Summary               string   `json:"summary"`
}

// ============================================================================
// Round 126: Asset Cyclic Dependency & Deadlock Detector (Tencent UGit Parity)
// ============================================================================

type AssetCycleItem struct {
	CyclePath []string `json:"cyclePath"`
	Length    int      `json:"length"`
	RiskLevel string   `json:"riskLevel"` // "MEDIUM", "HIGH", "CRITICAL"
}

type AssetCycleDetectResult struct {
	TotalAssetsScanned int              `json:"totalAssetsScanned"`
	CyclesCount        int              `json:"cyclesCount"`
	HasCycles          bool             `json:"hasCycles"`
	Cycles             []AssetCycleItem `json:"cycles"`
	Summary            string           `json:"summary"`
}

// ============================================================================
// Round 127: PBR Texture Channel Packing Auditor (Tencent UGit Parity)
// ============================================================================

type TextureChannelAuditItem struct {
	Path           string `json:"path"`
	PackingType    string `json:"packingType"` // "ORM", "MRAO", "SplitChannels", "Normal"
	IsCompliant    bool   `json:"isCompliant"`
	Issue          string `json:"issue,omitempty"`
	Recommendation string `json:"recommendation"`
}

type TextureChannelPackingResult struct {
	TotalTexturesScanned   int                       `json:"totalTexturesScanned"`
	OrmCompliantCount      int                       `json:"ormCompliantCount"`
	UnpackedTexturesCount  int                       `json:"unpackedTexturesCount"`
	InvalidNormalMapsCount int                       `json:"invalidNormalMapsCount"`
	VramWasteEstimateBytes int64                     `json:"vramWasteEstimateBytes"`
	VramWasteStr           string                    `json:"vramWasteStr"`
	Textures               []TextureChannelAuditItem `json:"textures"`
	Summary                string                    `json:"summary"`
}

// ============================================================================
// Round 128: Studio Webhooks & Notification Dispatcher (Tencent UGit Parity)
// ============================================================================

type StudioWebhookItem struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	WebhookType     string   `json:"webhookType"` // "WECOM", "FEISHU", "DINGTALK", "KODING"
	UrlMasked       string   `json:"urlMasked"`
	EventRules      []string `json:"eventRules"`
	IsEnabled       bool     `json:"isEnabled"`
	LastPingSuccess bool     `json:"lastPingSuccess"`
}

type StudioWebhookResult struct {
	TotalWebhooks         int                 `json:"totalWebhooks"`
	Webhooks              []StudioWebhookItem `json:"webhooks"`
	LastDispatchedMessage string              `json:"lastDispatchedMessage"`
	Success               bool                `json:"success"`
	Message               string              `json:"message"`
}

// ============================================================================
// Round 129: Unreal DDC & Unity Cache Accelerator Sync (Tencent UGit Parity)
// ============================================================================

type StudioDDCResult struct {
	LocalCachePath      string  `json:"localCachePath"`
	SharedCachePath     string  `json:"sharedCachePath"`
	IsSharedConnected   bool    `json:"isSharedConnected"`
	CacheHitRatio       float64 `json:"cacheHitRatio"`
	TotalCacheSizeBytes int64   `json:"totalCacheSizeBytes"`
	TotalCacheSizeStr   string  `json:"totalCacheSizeStr"`
	StaleBlobsCount     int     `json:"staleBlobsCount"`
	LatencyMs           int     `json:"latencyMs"`
	Summary             string  `json:"summary"`
}

// ============================================================================
// Round 130: Addressables & AssetBundle Diff Reporter (Tencent UGit Parity)
// ============================================================================

type DuplicateAssetItem struct {
	AssetPath string   `json:"assetPath"`
	Bundles   []string `json:"bundles"`
	SizeBytes int64    `json:"sizeBytes"`
	SizeStr   string   `json:"sizeStr"`
}

type BundleDiffItem struct {
	BundleName   string `json:"bundleName"`
	ChangeType   string `json:"changeType"` // "ADDED", "MODIFIED", "REMOVED"
	DeltaBytes   int64  `json:"deltaBytes"`
	DeltaSizeStr string `json:"deltaSizeStr"`
	AssetCount   int    `json:"assetCount"`
}

type BundleDiffResult struct {
	TotalBundlesBase         int                  `json:"totalBundlesBase"`
	TotalBundlesTarget       int                  `json:"totalBundlesTarget"`
	InitialInstallDeltaBytes int64                `json:"initialInstallDeltaBytes"`
	InitialInstallDeltaStr   string               `json:"initialInstallDeltaStr"`
	PatchDownloadBytes       int64                `json:"patchDownloadBytes"`
	PatchDownloadStr         string               `json:"patchDownloadStr"`
	DuplicateAssetLeaks      []DuplicateAssetItem `json:"duplicateAssetLeaks"`
	ChangedBundles           []BundleDiffItem     `json:"changedBundles"`
	Summary                  string               `json:"summary"`
}

// ============================================================================
// Round 131: Self-Contained Offline Git Patch Bundle Generator (Tencent UGit Parity)
// ============================================================================

type OfflinePatchBundleResult struct {
	BundlePath      string   `json:"bundlePath"`
	CommitsCount    int      `json:"commitsCount"`
	LfsBlobsCount   int      `json:"lfsBlobsCount"`
	TotalSizeBytes  int64    `json:"totalSizeBytes"`
	TotalSizeStr    string   `json:"totalSizeStr"`
	Sha256Checksum  string   `json:"sha256Checksum"`
	ManifestEntries []string `json:"manifestEntries"`
	Success         bool     `json:"success"`
	Summary         string   `json:"summary"`
}

// ============================================================================
// Round 132: Level Lighting & Lightmap Bake Settings Auditor (Tencent UGit Parity)
// ============================================================================

type LightmapMeshItem struct {
	MeshPath       string `json:"meshPath"`
	Resolution     int    `json:"resolution"`
	HasUv2Overlap  bool   `json:"hasUv2Overlap"`
	Uv2BoundsValid bool   `json:"uv2BoundsValid"`
	Severity       string `json:"severity"` // "OK", "WARNING", "CRITICAL"
	Recommendation string `json:"recommendation"`
}

type LightmapBakeAuditResult struct {
	TotalMeshes              int                `json:"totalMeshes"`
	OverlappingUv2Count      int                `json:"overlappingUv2Count"`
	ExcessiveResolutionCount int                `json:"excessiveResolutionCount"`
	EstimatedBakeMinutes     int                `json:"estimatedBakeMinutes"`
	OverlappingMeshes        []LightmapMeshItem `json:"overlappingMeshes"`
	BakeSettingsSummary      string             `json:"bakeSettingsSummary"`
	HealthScore              int                `json:"healthScore"` // 0-100
	Summary                  string             `json:"summary"`
}

// ============================================================================
// Round 133: Mobile Privacy Manifest & SDK Compliance Auditor (Tencent UGit Parity)
// ============================================================================

type PrivacyViolationItem struct {
	RuleID       string `json:"ruleId"`
	Severity     string `json:"severity"` // "WARNING", "BLOCKING"
	Description  string `json:"description"`
	OffenderFile string `json:"offenderFile"`
	Remediation  string `json:"remediation"`
}

type PrivacyComplianceResult struct {
	Platform                  string                 `json:"platform"` // "iOS", "Android", "CrossPlatform"
	TotalPermissionsDeclared  int                    `json:"totalPermissionsDeclared"`
	SensitivePermissionsCount int                    `json:"sensitivePermissionsCount"`
	TrackingDomainsCount      int                    `json:"trackingDomainsCount"`
	HasPrivacyManifest        bool                   `json:"hasPrivacyManifest"`
	Violations                []PrivacyViolationItem `json:"violations"`
	IsStoreReady              bool                   `json:"isStoreReady"`
	StoreReadinessGrade       string                 `json:"storeReadinessGrade"` // "A", "B", "FAIL"
	Summary                   string                 `json:"summary"`
}

// ============================================================================
// Round 134: Game Font Commercial License & Copyright Checker (Tencent UGit Parity)
// ============================================================================

type FontLicenseItem struct {
	FontPath             string `json:"fontPath"`
	FamilyName           string `json:"familyName"`
	CopyrightNotice      string `json:"copyrightNotice"`
	LicenseType          string `json:"licenseType"` // "Commercial_Safe", "Restricted", "Unknown"
	IsCommercialSafe     bool   `json:"isCommercialSafe"`
	SuggestedAlternative string `json:"suggestedAlternative"`
}

type FontLicenseAuditResult struct {
	TotalFonts           int               `json:"totalFonts"`
	CommercialSafeCount  int               `json:"commercialSafeCount"`
	RestrictedFontsCount int               `json:"restrictedFontsCount"`
	UnknownLicenseCount  int               `json:"unknownLicenseCount"`
	Fonts                []FontLicenseItem `json:"fonts"`
	LegalRiskScore       string            `json:"legalRiskScore"` // "LOW", "MEDIUM", "HIGH"
	Summary              string            `json:"summary"`
}

// ============================================================================
// Round 135: Multi-Branch Hotfix Backport Tracker (Tencent UGit Parity)
// ============================================================================

type HotfixCommitItem struct {
	Hash          string `json:"hash"`
	Author        string `json:"author"`
	Date          string `json:"date"`
	Subject       string `json:"subject"`
	ChangeID      string `json:"changeId"`
	CherryPickCmd string `json:"cherryPickCmd"`
}

type HotfixBackportResult struct {
	HotfixBranch       string             `json:"hotfixBranch"`
	TargetBranch       string             `json:"targetBranch"`
	TotalHotfixCommits int                `json:"totalHotfixCommits"`
	BackportedCommits  int                `json:"backportedCommits"`
	MissingBackports   []HotfixCommitItem `json:"missingBackports"`
	IsFullyBackported  bool               `json:"isFullyBackported"`
	Summary            string             `json:"summary"`
}

// ============================================================================
// Round 136: Commit Message & Content Safety Filter (Tencent UGit Parity)
// ============================================================================

type CommitSafetyViolation struct {
	CommitHash     string `json:"commitHash"`
	Author         string `json:"author"`
	Subject        string `json:"subject"`
	RuleTriggered  string `json:"ruleTriggered"`
	OffendingText  string `json:"offendingText"`
	RecommendedFix string `json:"recommendedFix"`
}

type CommitSafetyFilterResult struct {
	AnalysisKnown   bool                    `json:"analysisKnown"`
	CommitsAudited  int                     `json:"commitsAudited"`
	ViolationsCount int                     `json:"violationsCount"`
	Violations      []CommitSafetyViolation `json:"violations"`
	Passed          bool                    `json:"passed"`
	Summary         string                  `json:"summary"`
	/** "default" when the built-in list applies, "custom" after a save. */
	RulesSource string   `json:"rulesSource"`
	RuleCount   int      `json:"ruleCount"`
	RuleTerms   []string `json:"ruleTerms"`
}

// ============================================================================
// Round 138: Artist One-Click Zero-Friction Delivery Pipeline (Tencent UGit Parity)
// ============================================================================

type ArtistPipelineResult struct {
	StagedAssetsCount  int      `json:"stagedAssetsCount"`
	LockedAssetsCount  int      `json:"lockedAssetsCount"`
	Pushed             bool     `json:"pushed"`
	BackupFilesCreated []string `json:"backupFilesCreated"`
	TexturesChecked    int      `json:"texturesChecked"`
	MeshesChecked      int      `json:"meshesChecked"`
	Success            bool     `json:"success"`
	PipelineLogs       []string `json:"pipelineLogs"`
	Message            string   `json:"message"`
}

// ============================================================================
// Round 139: Multi-Engine Shader Pipe & Self-Healing Wizard (Tencent UGit Parity)
// ============================================================================

type ShaderSelfHealingResult struct {
	EngineType             string   `json:"engineType"`
	BrokenShaderCount      int      `json:"brokenShaderCount"`
	BlackRenderMeshesCount int      `json:"blackRenderMeshesCount"`
	ShaderCacheCleared     bool     `json:"shaderCacheCleared"`
	RepairedMaterials      []string `json:"repairedMaterials"`
	Success                bool     `json:"success"`
	HealthGrade            string   `json:"healthGrade"` // "OPTIMAL", "REPAIRED", "ATTENTION_REQUIRED"
	Summary                string   `json:"summary"`
}

// ============================================================================
// Round 140: 🏆 腾讯 UGit 工业级终极全景控制台 (Tencent UGit Pinnacle Master Hub)
// ============================================================================

type PinnacleDomainSummary struct {
	DomainID        string   `json:"domainId"`
	DomainName      string   `json:"domainName"`
	DomainEnName    string   `json:"domainEnName"`
	TotalModules    int      `json:"totalModules"`
	CompletedCount  int      `json:"completedCount"`
	CompletionRate  float64  `json:"completionRate"`
	Highlights      []string `json:"highlights"`
	KeyCapabilities []string `json:"keyCapabilities"`
}

type PinnacleParityStatusSummary struct {
	TotalRounds            int                     `json:"totalRounds"`            // 140
	CompletedRounds        int                     `json:"completedRounds"`        // 140
	ParityPercentage       float64                 `json:"parityPercentage"`       // 100.0
	CertificationBadge     string                  `json:"certificationBadge"`     // "Tencent UGit Pinnacle Master Gold Standard (腾讯游戏工业级终极对齐认证)"
	SystemReadiness        string                  `json:"systemReadiness"`        // "ENTERPRISE_PINNACLE_CERTIFIED"
	Domains                []PinnacleDomainSummary `json:"domains"`                // 6 domains
	TotalTestCoverageCount int                     `json:"totalTestCoverageCount"` // 140+ tests
	TotalToolsCount        int                     `json:"totalToolsCount"`        // 82 tools
	GoEngineVersion        string                  `json:"goEngineVersion"`
	ClientPlatform         string                  `json:"clientPlatform"`
	ArchitectureSummary    string                  `json:"architectureSummary"`
	CelebrationMessage     string                  `json:"celebrationMessage"`
	ReleaseSignature       string                  `json:"releaseSignature"`
}

// ============================================================================
// Project Browser Module (项目浏览与多工程管理)
// ============================================================================

type ProjectDetailItem struct {
	Name              string `json:"name"`
	Path              string `json:"path"`
	CurrentBranch     string `json:"currentBranch"`
	IsDirty           bool   `json:"isDirty"`
	DirtyCount        int    `json:"dirtyCount"`
	AheadCount        int    `json:"aheadCount"`
	BehindCount       int    `json:"behindCount"`
	LastCommitSubject string `json:"lastCommitSubject"`
	LastCommitDate    string `json:"lastCommitDate"`
	LastCommitHash    string `json:"lastCommitHash"`
	EngineType        string `json:"engineType"` // "Unity", "Unreal", "Godot", "Cocos", "Go", "Node", "Rust", "Git"
	IsCurrent         bool   `json:"isCurrent"`
	IsValid           bool   `json:"isValid"`
	StatusWarning     string `json:"statusWarning,omitempty"`
}

// ============================================================================
// Source Code Browser Module (项目源码浏览模块)
// ============================================================================

type SourceFileDetail struct {
	Path           string `json:"path"`
	Name           string `json:"name"`
	Size           int64  `json:"size"`
	SizeFormatted  string `json:"sizeFormatted"`
	LineCount      int    `json:"lineCount"`
	Language       string `json:"language"`
	Content        string `json:"content"`
	IsBinary       bool   `json:"isBinary"`
	IsImage        bool   `json:"isImage"`
	ImageData      string `json:"imageData,omitempty"`
	ImageWidth     int    `json:"imageWidth,omitempty"`
	ImageHeight    int    `json:"imageHeight,omitempty"`
	LastCommitHash string `json:"lastCommitHash"`
	LastAuthor     string `json:"lastAuthor"`
	LastDate       string `json:"lastDate"`
	LastSubject    string `json:"lastSubject"`
}

type ProjectSourceFileItem struct {
	Path     string `json:"path"`
	Name     string `json:"name"`
	Size     int64  `json:"size"`
	Status   string `json:"status"` // "clean", "modified", "untracked", "staged"
	IsBinary bool   `json:"isBinary"`
}

// ============================================================================
// External Diff / Merge Tools & Web Forge Module
// ============================================================================

type ExternalToolInfo struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Executable    string `json:"executable"`
	IsAvailable   bool   `json:"isAvailable"`
	SupportsDiff  bool   `json:"supportsDiff"`
	SupportsMerge bool   `json:"supportsMerge"`
	Description   string `json:"description"`
}

type ExternalToolConfig struct {
	DiffTool       string `json:"diffTool"`
	MergeTool      string `json:"mergeTool"`
	CustomDiffCmd  string `json:"customDiffCmd"`
	CustomMergeCmd string `json:"customMergeCmd"`
	IsGlobal       bool   `json:"isGlobal"`
}

type RemoteWebInfo struct {
	HasRemote     bool   `json:"hasRemote"`
	RemoteName    string `json:"remoteName"`
	RemoteURL     string `json:"remoteUrl"`
	Provider      string `json:"provider"`      // "github", "gitlab", "gitea", "gitee", "bitbucket", "generic"
	WebBaseURL    string `json:"webBaseUrl"`    // e.g. https://github.com/owner/repo
	CommitURLTpl  string `json:"commitUrlTpl"`  // .../commit/{hash}
	BranchURLTpl  string `json:"branchUrlTpl"`  // .../tree/{branch}
	FileURLTpl    string `json:"fileUrlTpl"`    // .../blob/{branch}/{path}
	CompareURLTpl string `json:"compareUrlTpl"` // .../compare/{base}...{head}
}
