// ---- Types ----

export interface ActivityDay {
  /** YYYY-MM-DD */
  date: string;
  commits: number;
  additions: number;
  deletions: number;
  /** 0 - 4 heat level */
  level: number;
  weekday: number;
}

export interface AnimBoneInfo {
  name: string;
  parentName?: string;
  depth: number;
  hasNegativeScale: boolean;
  maxWeight?: number;
}

export interface AnimClipInfo {
  name: string;
  durationSeconds: number;
  sampleRate: number;
  frameCount: number;
  trackCount: number;
  isLooping: boolean;
  hasRootMotion: boolean;
  rootSpeedMax: number;
  issues: string[];
}

export interface AnimRigBudgetLimits {
  maxBones: number;
  maxInfluences: number;
  maxSampleRate: number;
  maxTrackCount: number;
}

export interface AnimRigLintResult {
  assetPath: string;
  /** "FBX", "GLTF", "GLB", "ANIM" */
  format: string;
  analysisKnown: boolean;
  /** "Mobile", "Console", "Cinematic" */
  preset: string;
  boneCount: number;
  maxInfluencesPerVertex: number;
  totalDurationSeconds: number;
  overallSampleRate: number;
  totalTrackCount: number;
  clips: AnimClipInfo[];
  bones: AnimBoneInfo[];
  hasAbnormalRootMotion: boolean;
  hasNegativeScale: boolean;
  hasExtremeKeyframeDensity: boolean;
  hasUncompressedTracks: boolean;
  isOverBudget: boolean;
  /** "Mobile Ready", "Console Tier", "Cinematic Only", "Optimization Required" */
  performanceRating: string;
  /** "OK", "WARNING", "ERROR" */
  severity: string;
  budgetLimits: AnimRigBudgetLimits;
  issues: string[];
  advice: string[];
  sizeBytes: number;
  sizeStr: string;
  message: string;
}

export interface ArtistPipelineResult {
  stagedAssetsCount: number;
  lockedAssetsCount: number;
  pushed: boolean;
  backupFilesCreated: string[];
  texturesChecked: number;
  meshesChecked: number;
  success: boolean;
  pipelineLogs: string[];
  message: string;
}

export interface AssetCherryPickResult {
  targetRef: string;
  isDryRun: boolean;
  totalRequested: number;
  totalDependencies: number;
  totalAssetsCount: number;
  totalBytes: number;
  totalBytesStr: string;
  assets: CherryPickedAsset[];
  lfsAssetsCount: number;
  metaFilesCount: number;
  success: boolean;
  summary: string;
  message: string;
}

export interface AssetCycleDetectResult {
  totalAssetsScanned: number;
  cyclesCount: number;
  hasCycles: boolean;
  cycles: AssetCycleItem[];
  summary: string;
}

export interface AssetCycleItem {
  cyclePath: string[];
  length: number;
  /** "MEDIUM", "HIGH", "CRITICAL" */
  riskLevel: string;
}

export interface AssetGraphNode {
  id: string;
  name: string;
  class: string;
  category: string;
  posX: number;
  posY: number;
  comment?: string;
  properties?: Record<string, string>;
  pins: AssetGraphPin[];
}

export interface AssetGraphPin {
  name: string;
  direction: string;
  pinType: string;
  defaultValue: string;
  connectedTo: string;
}

export interface AssetMetadata {
  path: string;
  extension: string;
  sizeBytes: number;
  sizeHuman: string;
  isLfs: boolean;
  lfsPointer?: string;
  dimensions?: string;
  duration?: string;
  mimeType: string;
  lastModTime: string;
}

export interface AssetPackageHeader {
  engineVersion: string;
  magic: string;
  assetFormat: string;
  packageFlags: string;
  nameCount: number;
  exportCount: number;
  importCount: number;
  packageGuid: string;
  totalHeaderSize: number;
  summaryTags: string[];
}

/** Modal 72: Asset Reference Inspector */
export interface AssetReferenceItem {
  filePath: string;
  /** "Prefab", "Scene", "Material", "Animation", "Script/Config", "Other" */
  fileType: string;
  matchCount: number;
  previewSnippet: string;
}

export interface AssetReferenceResult {
  targetPath: string;
  assetGuid?: string;
  referenceCount: number;
  references: AssetReferenceItem[];
  safeToDelete: boolean;
  engineHint: string;
  message: string;
  /** False when some files could not be scanned; "no references" is then unproven. */
  analysisComplete: boolean;
  warnings: string[];
}

export interface AssetSystemAttribution {
  /** "Characters", "Environments", "UI", "Audio", "VFX", "Code", "ThirdParty", "Misc" */
  systemName: string;
  totalBytes: number;
  totalSizeStr: string;
  fileCount: number;
  percentage: number;
  lfsBytes: number;
  lfsSizeStr: string;
  lfsCount: number;
  topAssets: string[];
}

export interface AudioBankDiffResult {
  /** "Wwise", "FMOD", "Standard" */
  engineType: string;
  bankName: string;
  totalEventsBase: number;
  totalEventsTarget: number;
  addedEvents: string[];
  removedEvents: string[];
  modifiedEvents: AudioEventDiffItem[];
  orphanEvents: string[];
  oversizedAudios: OversizedAudioItem[];
  baseSizeBytes: number;
  targetSizeBytes: number;
  deltaSizeBytes: number;
  deltaSizeStr: string;
  summary: string;
}

export interface AudioEventDiffItem {
  eventName: string;
  /** "ADDED", "REMOVED", "MODIFIED" */
  changeType: string;
  oldDurationSec: number;
  newDurationSec: number;
  oldSizeBytes: number;
  newSizeBytes: number;
  isLooping: boolean;
}

/** Modal 76: Audio & SFX Inspector / Player */
export interface AudioInspectResult {
  path: string;
  /** "WAV", "MP3", "OGG", "FLAC", "SoundBank" */
  format: string;
  /** duration in seconds */
  durationSec: number;
  /** e.g. "01:24 (84.2s)" */
  durationStr: string;
  /** 44100, 48000, 96000 */
  sampleRate: number;
  /** 1=Mono, 2=Stereo */
  channels: number;
  /** "单声道 (Mono)" / "立体声 (Stereo)" */
  channelStr: string;
  /** 16, 24, 32 */
  bitsPerSample: number;
  metadataKnown: boolean;
  sizeBytes: number;
  sizeStr: string;
  /** 48000Hz standard */
  isStandard48k: boolean;
  /** base64 for audio playback */
  dataUri?: string;
  warnings: string[];
  message: string;
}

export interface AuditedTableReport {
  tableName: string;
  filePath: string;
  totalRowsBase: number;
  totalRowsTarget: number;
  addedRowsCount: number;
  modifiedRowsCount: number;
  deletedRowsCount: number;
  schemaChanges: SchemaChangeItem[];
  anomalies: TableAnomalyItem[];
  rowDiffs: RowSemanticDiff[];
  semanticSummary: string;
  hasErrors: boolean;
}

export interface AuthDiagnosticResult {
  sshKeysFound: string[];
  credentialHelper: string;
  host: string;
  sshAuthSuccess: boolean;
  sshAuthOutput: string;
}

export interface BatchCherryPickResult {
  successCount: number;
  totalCount: number;
  failedCommitHash: string;
  errorMessage: string;
  hasConflict: boolean;
  appliedHashes: string[];
}

export interface BinaryArchInfo {
  /** "arm64", "x86_64", "armeabi-v7a", "x86", "arm64e" */
  architecture: string;
  /** 32 or 64 */
  bitWidth: number;
  /** "LittleEndian", "BigEndian" */
  endianness: string;
  subtype?: string;
}

export interface BisectStatus {
  isActive: boolean;
  currentHash: string;
  currentStep: string;
  badRef: string;
  goodRefs: string[];
  log: string;
}

export interface BlameLine {
  lineNumber: number;
  commitHash: string;
  shortHash: string;
  author: string;
  date: string;
  summary: string;
  content: string;
}

export interface BoundingBox3D {
  center: Vector3D;
  extents: Vector3D;
  min: Vector3D;
  max: Vector3D;
}

export interface BranchCompareResult {
  baseBranch: string;
  targetBranch: string;
  aheadCount: number;
  behindCount: number;
  aheadCommits: CommitItem[];
  behindCommits: CommitItem[];
}

export interface BranchInfo {
  name: string;
  isCurrent: boolean;
  isRemote: boolean;
  commitHash: string;
  upstream: string;
  ahead: number;
  behind: number;
}

export interface BranchInsight {
  name: string;
  isCurrent: boolean;
  isRemote: boolean;
  shortHash: string;
  lastAuthor: string;
  lastDate: string;
  lastSubject: string;
  ahead: number;
  behind: number;
  hasUpstream: boolean;
  upstream: string;
  merged: boolean;
  protected: boolean;
  daysSinceCommit: number;
  /** current | protected | merged | unpushed | stale | active */
  category: string;
  suggestion: string;
}

export interface BranchInsightSummary {
  total: number;
  current: string;
  merged: number;
  protected: number;
  unpushed: number;
  stale: number;
  cleanable: number;
  staleDays: number;
  baseBranch: string;
  hint: string;
}

export interface BranchInsightsResult {
  base: string;
  items: BranchInsight[];
  summary: BranchInsightSummary;
}

export interface BrokenMaterialItem {
  materialPath: string;
  materialName: string;
  engineType: string;
  issueType: string;
  shaderName: string;
  shaderGuid?: string;
  isPinkShader: boolean;
  missingTextures: MissingTextureRef[];
  suggestedShader: string;
  affectedPrefabs: string[];
  recommendation: string;
}

export interface BudgetOptimizationSuggestion {
  /** "Texture Compression", "Audio Compression", "Mesh Optimization", "LFS Offloading" */
  category: string;
  targetFile: string;
  currentSize: number;
  currentSizeStr: string;
  estimatedSaving: number;
  estimatedSavingStr: string;
  recommendation: string;
  /** "HIGH", "MEDIUM", "LOW" */
  priority: string;
}

export interface BulkLockResult {
  lockedFiles: string[];
  skippedFiles: string[];
  failedFiles: string[];
  totalCount: number;
  message: string;
}

export interface BundleDiffItem {
  bundleName: string;
  /** "ADDED", "MODIFIED", "REMOVED" */
  changeType: string;
  deltaBytes: number;
  deltaSizeStr: string;
  assetCount: number;
}

export interface BundleDiffResult {
  totalBundlesBase: number;
  totalBundlesTarget: number;
  initialInstallDeltaBytes: number;
  initialInstallDeltaStr: string;
  patchDownloadBytes: number;
  patchDownloadStr: string;
  duplicateAssetLeaks: DuplicateAssetItem[];
  changedBundles: BundleDiffItem[];
  summary: string;
}

export interface CIPipelineRun {
  id: string;
  name: string;
  branch: string;
  commitHash: string;
  shortHash: string;
  author: string;
  message: string;
  /** "SUCCESS", "RUNNING", "FAILED" */
  status: string;
  triggerTime: string;
  duration: string;
  stages: CIPipelineStage[];
}

/** Modal 78: Game Studio CI/CD Pipeline Monitor */
export interface CIPipelineStage {
  name: string;
  /** "SUCCESS", "RUNNING", "FAILED", "PENDING" */
  status: string;
  duration: string;
}

export interface ChangeReview {
  base: string;
  head: string;
  baseLabel: string;
  headLabel: string;
  key: string;
  createdAt: string;
  updatedAt: string;
  summary: string;
  ready: boolean;
  files: ReviewFileState[];
  comments: ReviewComment[];
  commits: ReviewCommitItem[];
  stats: ChangeReviewStats;
}

export interface ChangeReviewStats {
  total: number;
  approved: number;
  changes: number;
  pending: number;
  comments: number;
  openComments: number;
  additions: number;
  deletions: number;
  progress: number;
  verdict: string;
  verdictDetail: string;
}

export interface CheckIgnoreResult {
  sourceFile: string;
  lineNumber: number;
  pattern: string;
  targetFile: string;
  isIgnored: boolean;
}

export interface CherryPickedAsset {
  path: string;
  fileType: string;
  sizeBytes: number;
  sizeStr: string;
  isLfs: boolean;
  isDependency: boolean;
  parentAsset?: string;
  status: string;
}

export interface CloneOptions {
  url: string;
  targetPath: string;
  /** "full", "blobless", "shallow" */
  cloneType: string;
  depth: number;
  branch: string;
  recurseSubmodules: boolean;
}

export interface CommitDetail {
  commit: CommitItem;
  files: FileStatus[];
  additions: number;
  deletions: number;
  diff: string;
}

export interface CommitItem {
  hash: string;
  shortHash: string;
  parents: string[];
  authorName: string;
  authorEmail: string;
  date: string;
  relativeDate: string;
  subject: string;
  body: string;
  refs: string[];
  isHead: boolean;
  lane: number;
  /** graph connection segments */
  lines: GraphLine[];
  /** "G" (Good), "B" (Bad), "U" (Untrusted), "N" (None) */
  signatureStatus: string;
  signer: string;
}

export interface CommitLintIssue {
  /** error / warning / hint */
  severity: string;
  rule: string;
  message: string;
}

export interface CommitLintResult {
  header: string;
  type: string;
  scope: string;
  subjectLen: number;
  score: number;
  grade: string;
  compliant: boolean;
  issues: CommitLintIssue[];
  autoFixed: string;
  hasFix: boolean;
  summary: string;
}

export interface CommitNoteItem {
  commitHash: string;
  shortHash: string;
  authorName: string;
  date: string;
  subject: string;
  content: string;
  preview: string;
}

export interface CommitNotesResult {
  notes: CommitNoteItem[];
  total: number;
  ref: string;
  summary: string;
}

export interface CommitPaginationResult {
  commits: CommitItem[];
  totalCount: number;
  hasMore: boolean;
}

export interface CommitSafetyFilterResult {
  analysisKnown: boolean;
  commitsAudited: number;
  violationsCount: number;
  violations: CommitSafetyViolation[];
  passed: boolean;
  summary: string;
}

export interface CommitSafetyViolation {
  commitHash: string;
  author: string;
  subject: string;
  ruleTriggered: string;
  offendingText: string;
  recommendedFix: string;
}

export interface CommitSigningResult {
  success: boolean;
  message: string;
}

export interface CommitSigningStatus {
  gpgSignEnabled: boolean;
  signingKey: string;
  /** "openpgp" or "ssh" */
  gpgFormat: string;
  availableSshKeys: string[];
  hasGpgBinary: boolean;
  recentCommits: SignedCommitItem[];
  compliantCount: number;
  totalChecked: number;
  complianceRate: string;
}

export interface CommitTemplateInfo {
  configuredPath: string;
  content: string;
  hasTemplate: boolean;
}

export interface CommitTopologyStats {
  totalCommits: number;
  mergeCommits: number;
  directCommits: number;
  mergeRatio: number;
}

export interface ConfigEntry {
  key: string;
  value: string;
  scope: string;
}

export interface ContributorActivity {
  name: string;
  email: string;
  commits: number;
  additions: number;
  deletions: number;
  lastActive: string;
  /** percent of total commits */
  share: number;
  isMe: boolean;
}

export interface ContributorStat {
  name: string;
  email: string;
  commits: number;
}

export interface ConvexHullItem {
  meshPath: string;
  vertexCount: number;
  polygonCount: number;
  /** > 256 vertices */
  isOverbudget: boolean;
  /** "OK", "WARNING", "CRITICAL" */
  severity: string;
}

export interface CrashStackDemangleResult {
  crashReason: string;
  faultAddress: string;
  faultModule: string;
  threadId: string;
  totalFrames: number;
  demangledCount: number;
  frames: DemangledFrame[];
  culpritFrame?: DemangledFrame;
  culpritAuthor: string;
  culpritCommit: string;
  culpritFile: string;
  summary: string;
  message: string;
}

export interface CredentialItem {
  id: string;
  /** "TENCENT_CLOUD_AKSK", "AWS_AKSK", "SSH_KEY", "GIT_TOKEN", "TAPD_TOKEN", "PERFORCE_CRED", "GENERIC_SECRET" */
  type: string;
  /** Key name / file path / remote name */
  identifier: string;
  /** "GIT_CONFIG", "ENVIRONMENT", "SSH_KEYCHAIN", "STUDIO_VAULT" */
  source: string;
  /** e.g. "AKID***98a", "ghp_***4bf" */
  maskedValue: string;
  /** "LOCAL_REPO", "GLOBAL_SYSTEM", "ENV_VAR" */
  scope: string;
  /** e.g. "0600 (Secure)", "0644 (Warning: Too Open)" */
  permissions: string;
  /** "SECURE", "WARNING", "CRITICAL_EXPOSURE" */
  securityLevel: string;
  /** e.g. "2027-01-01", "Permanent / Never", "Unknown" */
  expiresAt: string;
  isMasked: boolean;
  statusMessage: string;
}

export interface CredentialVaultResult {
  totalCredentials: number;
  criticalExposures: number;
  warningsCount: number;
  secureCount: number;
  items: CredentialItem[];
  vaultStoragePath: string;
  kmsEncryptionEnabled: boolean;
  summary: string;
  message: string;
}

export interface CustomActionConfig {
  id: string;
  name: string;
  description: string;
  command: string;
  args: string;
  workingDir: string;
  icon: string;
  showInToolbar: boolean;
}

export interface CustomActionResult {
  actionId: string;
  name: string;
  command: string;
  output: string;
  exitCode: number;
  durationMs: number;
  success: boolean;
}

export interface DCCActiveProcess {
  pid: number;
  processName: string;
  dccTool: string;
}

export interface DCCAutoLockResult {
  success: boolean;
  assetPath: string;
  message: string;
}

export interface DCCLockItem {
  lockFilePath: string;
  sourceAssetPath: string;
  dccTool: string;
  lockOwner: string;
  lastModified: string;
  isActive: boolean;
  isLfs: boolean;
  lfsLockStatus: string;
  lfsLockOwner?: string;
  collisionRisk: boolean;
  recommendation: string;
}

export interface DCCLockWatchResult {
  totalDccFiles: number;
  activeLocksCount: number;
  unprotectedCount: number;
  collisionCount: number;
  runningProcesses: DCCActiveProcess[];
  items: DCCLockItem[];
  summary: string;
  message: string;
}

export interface DataLayerDiffItem {
  name: string;
  changeType?: "ADDED" | "MODIFIED" | "REMOVED";
  isRuntime: boolean;
  /** "Activated", "Loaded", "Unloaded", "Unknown" */
  initialRuntimeState: string;
  addedActors: string[];
  removedActors: string[];
  actorCount: number;
  isConflicted: boolean;
}

export interface DataTableAuditResult {
  analysisKnown: boolean;
  baseRef: string;
  targetRef: string;
  auditedTables: AuditedTableReport[];
  totalTables: number;
  criticalCount: number;
  warningCount: number;
  isValid: boolean;
  overallSummary: string;
}

export interface DemangledFrame {
  frameIndex: number;
  address: string;
  mangledSymbol: string;
  demangledSymbol: string;
  moduleName: string;
  sourceFile: string;
  lineNumber: number;
  offset: string;
  blameAuthor?: string;
  blameEmail?: string;
  blameCommit?: string;
  blameDate?: string;
  blameSummary?: string;
  codeSnippet?: string;
  isCulprit: boolean;
}

export interface DiffComparisonResult {
  refA: string;
  refB: string;
  files: FileStatus[];
  additions: number;
  deletions: number;
  diff: string;
}

export interface DiskBreakdownResult {
  totalSize: number;
  totalSizeStr: string;
  workingTreeSize: number;
  workingTreeStr: string;
  gitInternalSize: number;
  gitInternalStr: string;
  lfsStoreSize: number;
  lfsStoreStr: string;
  engineCacheSize: number;
  engineCacheStr: string;
  topFolders: DiskFolderItem[];
  assetCategories: StorageBreakdownItem[];
  recommendations: string[];
}

export interface DiskFolderItem {
  path: string;
  size: number;
  sizeStr: string;
}

export interface DuplicateAssetItem {
  assetPath: string;
  bundles: string[];
  sizeBytes: number;
  sizeStr: string;
}

export interface EngineWorkflowInfo {
  /** "unreal", "unity", "godot", "cocos", "none" */
  engineType: string;
  projectName: string;
  projectPath: string;
  engineVersion: string;
  cacheFolders: string[];
  cacheSizeBytes: number;
  cacheSizeStr: string;
  recommendedActions: string[];
}

export interface FieldSemanticDiff {
  fieldName: string;
  oldValue: string;
  newValue: string;
  numericDelta?: number;
  percentChange?: number;
  summary: string;
}

export interface FileChurnHotspot {
  path: string;
  changes: number;
  additions: number;
  deletions: number;
  authors: string[];
  category: string;
  /** 0 - 100 */
  riskScore: number;
}

export interface FileChurnStat {
  path: string;
  additions: number;
  deletions: number;
  isBinary: boolean;
  staged: boolean;
}

export interface FileRevertResult {
  success: boolean;
  filePath: string;
  commitHash: string;
  message: string;
}

/** Modal 79: Single File Historical Reversion & Timeline Diff */
export interface FileRevisionItem {
  commitHash: string;
  shortHash: string;
  authorName: string;
  date: string;
  subject: string;
  sizeBytes: number;
  sizeStr: string;
}

export interface FileStatus {
  path: string;
  oldPath?: string;
  /** "M", "A", "D", "R", "?" */
  stagedStatus: string;
  /** "M", "D", "?" */
  unstagedStatus: string;
  isStaged: boolean;
  isLfs: boolean;
  additions: number;
  deletions: number;
  size?: string;
  isBinary?: boolean;
  lockOwner?: string;
  /** UI-only marker for synthetic entries built from RepoStatus.conflictedFiles. */
  isConflicted?: boolean;
}

export interface FileTimelineResult {
  filePath: string;
  revisions: FileRevisionItem[];
  message: string;
}

export interface FontGlyphAuditSummary {
  fontPath: string;
  /** "TTF", "OTF", "WOFF" */
  fontFormat: string;
  totalFontGlyphs: number;
  totalCharsChecked: number;
  missingGlyphsCount: number;
  missingGlyphs: MissingGlyphItem[];
  coveragePercent: number;
}

export interface FontLicenseAuditResult {
  totalFonts: number;
  commercialSafeCount: number;
  restrictedFontsCount: number;
  unknownLicenseCount: number;
  fonts: FontLicenseItem[];
  /** "LOW", "MEDIUM", "HIGH" */
  legalRiskScore: string;
  summary: string;
}

export interface FontLicenseItem {
  fontPath: string;
  familyName: string;
  copyrightNotice: string;
  /** "Commercial_Safe", "Restricted", "Unknown" */
  licenseType: string;
  isCommercialSafe: boolean;
  suggestedAlternative: string;
}

export interface FullParityStatusSummary {
  /** 120 */
  totalRounds: number;
  /** 120 */
  completedRounds: number;
  /** 100.0 */
  parityPercentage: number;
  /** "Tencent UGit 100% Full Parity Gold Standard" */
  certificationBadge: string;
  /** "PRODUCTION_CERTIFIED" */
  systemReadiness: string;
  /** 5 domains, 24 modules each */
  domains: ParityDomainSummary[];
  /** 120+ tests */
  totalTestCoverageCount: number;
  goEngineVersion: string;
  clientPlatform: string;
  architectureSummary: string;
  celebrationMessage: string;
  releaseSignature: string;
}

export interface GameEngineInfo {
  engineType: string;
  hasGitAttributes: boolean;
  hasLfsRules: boolean;
  projectName: string;
  engineVersion: string;
  recommendedPreset: string;
}

/** GitCommandLog tracks git command executions for the activity drawer. */
export interface GitCommandLog {
  id: number;
  command: string;
  dir: string;
  output: string;
  error?: string;
  durationMs: number;
  time: string;
  success: boolean;
}

export interface GitFsckResult {
  isClean: boolean;
  danglingBlob: number;
  danglingTree: number;
  danglingCommit: number;
  rawErrors: string[];
}

export interface GitHookInfo {
  name: string;
  enabled: boolean;
  installed: boolean;
  path: string;
}

export interface GitProxyConfig {
  httpProxy: string;
  httpsProxy: string;
  isGlobal: boolean;
}

export interface GitUserConfig {
  userName: string;
  userEmail: string;
  isGlobal: boolean;
}

export interface GitVersionInfo {
  version: string;
  system: string;
}

export interface GraphLine {
  fromLane: number;
  toLane: number;
  color: string;
}

export interface GrepMatchItem {
  path: string;
  lineNumber: number;
  lineContent: string;
}

export interface GuidCollisionItem {
  guid: string;
  currentActorName: string;
  targetActorName: string;
  currentClass: string;
  targetClass: string;
  subLevel: string;
  isIdentical: boolean;
  recommendedFix: string;
}

export interface HealthDimension {
  key: string;
  title: string;
  icon: string;
  /** "pass" | "warn" | "fail" */
  status: string;
  /** 0 - 100 */
  score: number;
  detail: string;
  /** UI hook: settings / fsck / ignore / lfs / signing / conflict */
  action: string;
}

export interface HistoryLargeBlob {
  oid: string;
  size: number;
  sizeStr: string;
  path: string;
  commitHash: string;
  commitDate: string;
  author: string;
  subject: string;
}

export interface HotfixBackportResult {
  hotfixBranch: string;
  targetBranch: string;
  totalHotfixCommits: number;
  backportedCommits: number;
  missingBackports: HotfixCommitItem[];
  isFullyBackported: boolean;
  summary: string;
}

export interface HotfixCommitItem {
  hash: string;
  author: string;
  date: string;
  subject: string;
  changeId: string;
  cherryPickCmd: string;
}

export interface HotfixManifestResult {
  success: boolean;
  manifestVersion: string;
  appVersion: string;
  baseRef: string;
  targetRef: string;
  generatedAt: string;
  fullPackRequired: boolean;
  fullPackReason?: string;
  totalFilesCount: number;
  totalPatchBytes: number;
  totalPatchSizeStr: string;
  totalCompressedBytes: number;
  totalCompressedSizeStr: string;
  tierBreakdown: HotfixTierSummary[];
  patchFiles: HotfixPatchFileItem[];
  deltaPackageList: string[];
  manifestJson: string;
  summary: string;
}

export interface HotfixPatchFileItem {
  path: string;
  /** "ADDED", "MODIFIED", "DELETED" */
  action: string;
  /** "Code", "Config", "Resources", "Full Pack Required" */
  tier: string;
  size: number;
  sizeStr: string;
  compressedSize: number;
  compressedSizeStr: string;
  crc32: string;
  md5: string;
  sha256: string;
  requiresReboot: boolean;
}

export interface HotfixTierSummary {
  tier: string;
  count: number;
  totalBytes: number;
  totalSizeStr: string;
}

/** Modal 75: Git-Ignore Completeness & Artifact Sanitizer */
export interface IgnoreArtifactItem {
  path: string;
  /** "Unity Cache", "Unreal Build", "IDE Artifact", "OS File", "Compiler Binary" */
  category: string;
  /** dangerously tracked in git */
  isTracked: boolean;
  sizeBytes: number;
  sizeHuman: string;
  ignored: boolean;
  remedy: string;
}

export interface IgnoreSanitizeActionResult {
  success: boolean;
  rulesAdded: string[];
  untrackedFiles: string[];
  message: string;
}

export interface IgnoreSanitizerResult {
  engineType: string;
  missingRules: string[];
  hasGitIgnore: boolean;
  trackedArtifacts: IgnoreArtifactItem[];
  totalArtifacts: number;
  trackedCount: number;
  /** 0 - 100 */
  healthScore: number;
  summary: string;
}

export interface LangParityStats {
  language: string;
  totalKeys: number;
  translatedKeys: number;
  missingKeys: number;
  untranslatedCount: number;
  parityPercent: number;
  /** "EXCELLENT", "GOOD", "NEEDS_WORK", "CRITICAL" */
  status: string;
}

export interface LargeFileInfo {
  path: string;
  sizeBytes: number;
  sizeHuman: string;
  isLfs: boolean;
}

export interface LfsCacheStats {
  totalSizeBytes: number;
  totalSizeStr: string;
  fileCount: number;
  extensions: StorageBreakdownItem[];
  cachePath: string;
}

export interface LfsFileRevision {
  commitHash: string;
  shortHash: string;
  authorName: string;
  authorEmail: string;
  date: string;
  subject: string;
  oid?: string;
  sizeBytes?: number;
  sizeHuman?: string;
}

export interface LfsFilterConfig {
  fetchInclude: string[];
  fetchExclude: string[];
  isConfigured: boolean;
}

/** Modal 74: LFS Lock Activity & Conflict Dashboard */
export interface LfsLockActivityItem {
  path: string;
  owner: string;
  isMe: boolean;
  lockedAt: string;
  durationStr: string;
  /** held > 72 hours */
  isZombie: boolean;
  /** modified locally by current user while owned by teammate */
  hasConflict: boolean;
  localStatus?: string;
  category: string;
}

export interface LfsLockActivityResult {
  totalLocks: number;
  myLockCount: number;
  teamLockCount: number;
  zombieCount: number;
  collisionCount: number;
  locks: LfsLockActivityItem[];
  message: string;
}

export interface LfsLockInfo {
  id: string;
  path: string;
  owner: string;
  lockedAt: string;
}

export interface LfsPruneResult {
  dryRun: boolean;
  output: string;
  prunedFiles: number;
  reclaimedStr: string;
  success: boolean;
}

export interface LfsTrackPattern {
  pattern: string;
  source: string;
}

export interface LfsTuningConfig {
  /** default 3, recommended 8-16 */
  concurrentTransfers: number;
  /** default 1MB, recommended 512MB */
  postBufferMb: number;
  /** default 30s */
  dialTimeoutSec: number;
  /** core.compression */
  compressionLevel: number;
  isOptimized: boolean;
  activeSummary: string;
}

export interface LfsTuningResult {
  success: boolean;
  config: LfsTuningConfig;
  message: string;
}

export interface LightmapBakeAuditResult {
  totalMeshes: number;
  overlappingUv2Count: number;
  excessiveResolutionCount: number;
  estimatedBakeMinutes: number;
  overlappingMeshes: LightmapMeshItem[];
  bakeSettingsSummary: string;
  /** 0-100 */
  healthScore: number;
  summary: string;
}

export interface LightmapMeshItem {
  meshPath: string;
  resolution: number;
  hasUv2Overlap: boolean;
  uv2BoundsValid: boolean;
  /** "OK", "WARNING", "CRITICAL" */
  severity: string;
  recommendation: string;
}

export interface LocalizationAuditResult {
  analysisKnown: boolean;
  primaryLanguage: string;
  languagesFound: string[];
  totalUniqueKeys: number;
  /** 0 - 100% */
  overallParityScore: number;
  languageStats: LangParityStats[];
  untranslatedList: LocalizedEntry[];
  fontAudit?: FontGlyphAuditSummary;
  sourceFilesCount: number;
  auditedFiles: string[];
  /** "OK", "WARNING", "ERROR" */
  severity: string;
  advice: string[];
  message: string;
}

export interface LocalizedEntry {
  key: string;
  sourceText: string;
  /** lang -> text */
  translations: Record<string, string>;
  missingLangs: string[];
  /** "COMPLETE", "PARTIAL", "MISSING" */
  status: string;
}


export interface LockableSyncResult {
  totalLockableFiles: number;
  readOnlyCount: number;
  writableCount: number;
  protectedFiles: string[];
  isProtectionActive: boolean;
}

export interface LodStreamingAuditResult {
  analysisKnown: boolean;
  totalMeshesAudited: number;
  lodCompliantCount: number;
  lodMissingCount: number;
  lodUnknownCount: number;
  totalTexturesAudited: number;
  streamingCompliantCount: number;
  streamingMissingCount: number;
  streamingUnknownCount: number;
  baselineVramMb: number;
  streamingVramMb: number;
  potentialSavingsMb: number;
  savingsPercentage: number;
  /** 0 - 100 */
  overallHealthScore: number;
  meshes: MeshLodAuditItem[];
  textures: TextureStreamingAuditItem[];
  recommendations: string[];
  summary: string;
}

export interface MaterialInstanceItem {
  instancePath: string;
  parentMaterial: string;
  parentShader: string;
  overrides: MaterialParamOverride[];
  deadCount: number;
  isHealthy: boolean;
}

export interface MaterialInstanceOverrideResult {
  totalInstances: number;
  overriddenParametersCount: number;
  deadParametersCount: number;
  instances: MaterialInstanceItem[];
  summary: string;
}

export interface MaterialParamOverride {
  paramName: string;
  /** "Texture", "Scalar", "Vector" */
  paramType: string;
  value: string;
  /** True if parameter does not exist in parent shader */
  isDead: boolean;
}

/** Modal 77: 3D Model Mesh & Polygon Budget Linter */
export interface MeshLintResult {
  path: string;
  format: string;
  triangleCount: number;
  triangleCountKnown: boolean;
  vertexCount: number;
  submeshCount: number;
  materialCount: number;
  budgetPreset: string;
  budgetLimit: number;
  isOverBudget: boolean;
  /** "OK", "WARNING", "ERROR" */
  severity: string;
  sizeBytes: number;
  sizeStr: string;
  advice: string;
  warnings: string[];
}

export interface MeshLodAuditItem {
  path: string;
  /** "StaticMesh", "SkeletalMesh" */
  meshType: string;
  baseTriangles: number;
  triangleCountKnown: boolean;
  lodTiers: MeshLodTier[];
  maxLodLevel: number;
  /** Has at least LOD0..LOD2 or LOD3 */
  hasCompleteLods: boolean;
  /** "OK", "WARNING", "CRITICAL" */
  severity: string;
  recommendation: string;
}

export interface MeshLodTier {
  /** 0, 1, 2, 3 */
  lodIndex: number;
  triangleCount: number;
  triangleCountKnown: boolean;
  /** e.g. 100%, 50%, 25%, 12.5% */
  reductionPercent: number;
  /** e.g. 1.0, 0.6, 0.3, 0.15 */
  screenRelativeSize: number;
}

export interface MissingGlyphItem {
  char: string;
  unicodeHex: string;
  unicodeName: string;
  lang: string;
  usedInKeys: string[];
  sampleText: string;
}

export interface MissingMaterialScanResult {
  targetDirectory: string;
  engineType: string;
  totalMaterialsScanned: number;
  brokenMaterialsCount: number;
  pinkShaderCount: number;
  missingTexturesCount: number;
  brokenMaterials: BrokenMaterialItem[];
  healthScore: number;
  summary: string;
  message: string;
}

export interface MissingTextureRef {
  propertySlot: string;
  textureGuid: string;
  expectedPath: string;
  severity: string;
}

export interface ModifiedNodeItem {
  nodeId: string;
  nodeName: string;
  nodeClass: string;
  category: string;
  pinChanges: NodePinDiff[];
  propertyChanges: NodePropertyDiff[];
  posDeltaX: number;
  posDeltaY: number;
  summary: string;
}

export interface MultiRemoteStatusResult {
  currentBranch: string;
  remotes: RemoteSyncStatusItem[];
}

export interface MultiRemoteSyncResult {
  /** "fetch-all" or "push-all" */
  action: string;
  success: boolean;
  messages: string[];
}

export interface NativePluginItem {
  relativePath: string;
  pluginName: string;
  /** "ELF", "Mach-O", "Mach-O Fat", "PE", "AAR", "Framework" */
  format: string;
  architectures: string[];
  architecturesInfo: BinaryArchInfo[];
  /** "Android", "iOS", "macOS", "Windows", "Unknown" */
  targetPlatform: string;
  is64BitOnly: boolean;
  isUniversal: boolean;
  sizeBytes: number;
  sizeStr: string;
  hasIssue: boolean;
  issues: string[];
}

export interface NativePluginsAbiResult {
  totalPluginsCount: number;
  scannedDirectories: string[];
  plugins: NativePluginItem[];
  platformStatuses: PlatformAbiStatus[];
  compliantCount: number;
  defectCount: number;
  hasCriticalMismatch: boolean;
  /** "OK", "WARNING", "ERROR" */
  severity: string;
  summaryRecommendations: string[];
  message: string;
}

export interface NodePinDiff {
  pinName: string;
  changeType: string;
  oldValue?: string;
  newValue?: string;
  detail: string;
}

export interface NodePropertyDiff {
  property: string;
  oldValue: string;
  newValue: string;
}

export interface OfflinePatchBundleResult {
  bundlePath: string;
  commitsCount: number;
  lfsBlobsCount: number;
  totalSizeBytes: number;
  totalSizeStr: string;
  sha256Checksum: string;
  manifestEntries: string[];
  success: boolean;
  summary: string;
}

export interface OfflineQueueItem {
  id: string;
  /** "push", "lfs_lock", "lfs_unlock", "fetch", "lfs_push" */
  operation: string;
  /** Branch name, asset path, or remote target */
  target: string;
  payload?: string;
  /** "queued", "running", "succeeded", "failed", "paused", "discarded" */
  status: string;
  retryCount: number;
  maxRetries: number;
  backoffSecs: number;
  createdAt: string;
  lastAttempt?: string;
  nextRetryAt?: string;
  lastError?: string;
}

export interface OfflineQueueResult {
  items: OfflineQueueItem[];
  totalCount: number;
  queuedCount: number;
  failedCount: number;
  pausedCount: number;
  succeededCount: number;
  isOnline: boolean;
  networkSummary: string;
  message: string;
}

export interface OversizedAudioItem {
  path: string;
  sizeBytes: number;
  sizeStr: string;
  format: string;
  advice: string;
}

export interface PackageBudgetResult {
  repoPath: string;
  totalAssetsCount: number;
  totalSizeBytes: number;
  totalSizeStr: string;
  totalLfsBytes: number;
  totalLfsSizeStr: string;
  systems: AssetSystemAttribution[];
  platformBudgets: PlatformBudgetStatus[];
  optimizationTips: BudgetOptimizationSuggestion[];
  hasOverrun: boolean;
  worstPlatform: string;
  summary: string;
}

export interface ParityDomainSummary {
  domainId: string;
  domainName: string;
  domainEnName: string;
  /** 24 */
  totalModules: number;
  /** 24 */
  completedCount: number;
  /** 100.0 */
  completionRate: number;
  highlights: string[];
  keyCapabilities: string[];
}

export interface PhysicsCollisionAuditResult {
  analysisKnown: boolean;
  totalColliders: number;
  convexHullsCount: number;
  concaveMeshCount: number;
  overbudgetHulls: ConvexHullItem[];
  nonConvexDynamicColliders: string[];
  missingPhysicMaterials: string[];
  /** 0-100 */
  healthScore: number;
  summary: string;
}

export interface PickaxeHit {
  hash: string;
  shortHash: string;
  author: string;
  date: string;
  subject: string;
  added: number;
  removed: number;
  files: string[];
  snippet: string;
  /** 新增 / 移除 / 改写 / 触碰 */
  change: string;
}

export interface PickaxeResult {
  query: string;
  mode: string;
  totalHits: number;
  analyzedHits: number;
  addedLines: number;
  removedLines: number;
  fileCount: number;
  firstSeen: string;
  lastSeen: string;
  truncated: boolean;
  hits: PickaxeHit[];
  message: string;
}

export interface PinnacleDomainSummary {
  domainId: string;
  domainName: string;
  domainEnName: string;
  totalModules: number;
  completedCount: number;
  completionRate: number;
  highlights: string[];
  keyCapabilities: string[];
}

export interface PinnacleParityStatusSummary {
  /** 140 */
  totalRounds: number;
  /** 140 */
  completedRounds: number;
  /** 100.0 */
  parityPercentage: number;
  /** "Tencent UGit Pinnacle Master Gold Standard (腾讯游戏工业级终极对齐认证)" */
  certificationBadge: string;
  /** "ENTERPRISE_PINNACLE_CERTIFIED" */
  systemReadiness: string;
  /** 6 domains */
  domains: PinnacleDomainSummary[];
  /** 140+ tests */
  totalTestCoverageCount: number;
  /** 82 tools */
  totalToolsCount: number;
  goEngineVersion: string;
  clientPlatform: string;
  architectureSummary: string;
  celebrationMessage: string;
  releaseSignature: string;
}

export interface PlatformAbiStatus {
  /** "iOS", "Android", "macOS", "Windows" */
  platform: string;
  requiredArches: string[];
  availableArches: string[];
  missingArches: string[];
  pluginCount: number;
  isCompliant: boolean;
  /** "OK", "WARNING", "ERROR" */
  severity: string;
}

export interface PlatformBudgetStatus {
  /** "iOS App Store (Cellular)", "iOS App Store (Full)", "Android (APK+OBB)", "PC / Console", "WeChat Mini Game" */
  platform: string;
  maxBudgetLimit: number;
  maxBudgetLimitStr: string;
  currentSize: number;
  currentSizeStr: string;
  usageRatio: number;
  remainingBytes: number;
  remainingStr: string;
  /** "SAFE", "WARNING", "OVERRUN" */
  status: string;
}

export interface PolicyRuleCheckResult {
  ruleId: string;
  ruleName: string;
  passed: boolean;
  severity: string;
  details: string;
}

export interface PolicyViolationItem {
  /** "RULE_COMMIT_CONVENTION", "RULE_LFS_POINTER_INTEGRITY", "RULE_NO_LARGE_BLOBS", "RULE_COMMIT_SIGNATURE", "RULE_BRANCH_STATUS" */
  ruleId: string;
  /** "BLOCKING", "WARNING", "INFO" */
  severity: string;
  title: string;
  description: string;
  commitHash?: string;
  filePath?: string;
  remediation: string;
}

export interface PrePushChecklistResult {
  analysisKnown: boolean;
  branchName: string;
  remoteTracking: string;
  commitsAudited: number;
  isBlocked: boolean;
  /** 0 - 100 */
  passReadinessScore: number;
  /** "PASS", "PASS_WITH_WARNINGS", "BLOCKED" */
  readinessGrade: string;
  rulesChecked: PolicyRuleCheckResult[];
  violations: PolicyViolationItem[];
  summary: string;
  message: string;
}

export interface PrePushVerificationResult {
  unpushedCommits: CommitItem[];
  totalLfsPointers: number;
  missingLfsCount: number;
  missingLfsFiles: string[];
  oversizedBlobs: LargeFileInfo[];
  isSafe: boolean;
  statusSummary: string;
}

export interface PrivacyComplianceResult {
  /** "iOS", "Android", "CrossPlatform" */
  platform: string;
  totalPermissionsDeclared: number;
  sensitivePermissionsCount: number;
  trackingDomainsCount: number;
  hasPrivacyManifest: boolean;
  violations: PrivacyViolationItem[];
  isStoreReady: boolean;
  /** "A", "B", "FAIL" */
  storeReadinessGrade: string;
  summary: string;
}

export interface PrivacyViolationItem {
  ruleId: string;
  /** "WARNING", "BLOCKING" */
  severity: string;
  description: string;
  offenderFile: string;
  remediation: string;
}

export interface ProjectDetailItem {
  name: string;
  path: string;
  currentBranch: string;
  isDirty: boolean;
  dirtyCount: number;
  aheadCount: number;
  behindCount: number;
  lastCommitSubject: string;
  lastCommitDate: string;
  lastCommitHash: string;
  /** "Unity", "Unreal", "Godot", "Cocos", "Go", "Node", "Rust", "Git" */
  engineType: string;
  isCurrent: boolean;
  isValid: boolean;
}

export interface ProjectSourceFileItem {
  path: string;
  name: string;
  size: number;
  /** "clean", "modified", "untracked", "staged" */
  status: string;
  isBinary: boolean;
}

export interface RebaseTodoItem {
  /** "pick", "reword", "edit", "squash", "fixup", "drop" */
  action: string;
  hash: string;
  shortHash: string;
  subject: string;
  author: string;
}

export interface ReflogItem {
  selector: string;
  commitHash: string;
  action: string;
  message: string;
  date: string;
}

export interface ReleaseCheck {
  key: string;
  title: string;
  /** "pass" | "warn" | "fail" */
  status: string;
  detail: string;
  blocking: boolean;
}

export interface ReleaseCreateResult {
  success: boolean;
  tag: string;
  pushed: boolean;
  message: string;
  pushOutput: string;
}

export interface ReleasePlan {
  branch: string;
  lastTag: string;
  commitCount: number;
  aheadOfRemote: number;
  behindRemote: number;
  suggestedVersion: string;
  nextMinor: boolean;
  checks: ReleaseCheck[];
  changelog: string;
  ready: boolean;
  summary: string;
}

export interface RemoteDetail {
  name: string;
  url: string;
  /** "fetch" or "push" */
  type: string;
}

export interface RemoteSyncStatusItem {
  name: string;
  fetchUrl: string;
  pushUrl: string;
  branch: string;
  ahead: number;
  behind: number;
  lastFetched: string;
}

export interface RemoteTestResult {
  remote: string;
  success: boolean;
  latency: string;
  heads: string[];
  error?: string;
}

export interface RepoHealthReport {
  score: number;
  grade: string;
  passCount: number;
  warnCount: number;
  failCount: number;
  items: HealthDimension[];
  summary: string;
  checkedAt: string;
  repoName: string;
}

export interface RepoInfo {
  path: string;
  name: string;
  currentBranch: string;
  status: RepoStatus;
  remotes: string[];
}

export interface RepoMaintenanceStats {
  gitDirSize: string;
  packFilesCount: number;
  looseObjects: number;
  success: boolean;
  /** OK | SKIPPED | FAILED */
  remotePruneStatus: string;
  remotePruneNote: string;
  /** OK | FAILED */
  gcStatus: string;
  /** OK | FAILED */
  pruneStatus: string;
  warnings: string[];
  summary: string;
}

export interface RepoOverviewStats {
  totalCommits: number;
  totalBranches: number;
  totalTags: number;
  contributors: ContributorStat[];
}

export interface RepoStatus {
  currentBranch: string;
  trackingBranch: string;
  ahead: number;
  behind: number;
  isClean: boolean;
  stagedFiles: FileStatus[];
  unstagedFiles: FileStatus[];
  untrackedFiles: FileStatus[];
  conflictedFiles: string[];
  isMerging: boolean;
  isRebasing: boolean;
  isCherryPicking: boolean;
  rebaseHeadName?: string;
  rebaseStep?: number;
  rebaseTotal?: number;
  mergeHeadBranch?: string;
}

export interface ReviewComment {
  id: string;
  path: string;
  line: number;
  /** new | old */
  side: string;
  body: string;
  author: string;
  createdAt: string;
  resolved: boolean;
}

export interface ReviewCommitItem {
  hash: string;
  shortHash: string;
  author: string;
  date: string;
  subject: string;
}

export interface ReviewFileState {
  path: string;
  /** pending | approved | changes */
  status: string;
  changeType: string;
  note: string;
  additions: number;
  deletions: number;
  category: string;
  isBinary: boolean;
  updatedAt: string;
  commentCount: number;
  openCommentCount: number;
}

export interface ReviewSessionSummary {
  key: string;
  base: string;
  head: string;
  updatedAt: string;
  total: number;
  approved: number;
  changes: number;
  pending: number;
  comments: number;
  openComments: number;
  additions: number;
  deletions: number;
  ready: boolean;
  verdict: string;
  verdictDetail: string;
}

export interface RowSemanticDiff {
  primaryKey: string;
  /** "ADDED", "MODIFIED", "DELETED" */
  action: string;
  fieldDiffs: FieldSemanticDiff[];
  summary: string;
}

export interface SceneMergeStrategyItem {
  category: string;
  targetId: string;
  action: string;
  description: string;
  automated: boolean;
}

export interface SceneStreamAuditResult {
  currentBranch: string;
  targetBranch: string;
  auditedSubLevels: string[];
  engineType: string;
  analysisKnown: boolean;
  totalActorsCurrent: number;
  totalActorsTarget: number;
  guidCollisions: GuidCollisionItem[];
  spatialOverlaps: SpatialOverlapItem[];
  transformConflicts: TransformConflictItem[];
  safetyScore: number;
  safetyLevel: string;
  mergeVerdict: string;
  recommendedPlan: SceneMergeStrategyItem[];
  summaryMessage: string;
}

export interface SceneStreamResolveResult {
  success: boolean;
  scenePath: string;
  actorGuid: string;
  appliedAction: string;
  newActorGuid?: string;
  offsetApplied?: Vector3D;
  message: string;
}

export interface SchemaChangeItem {
  /** "COLUMN_ADDED", "COLUMN_REMOVED", "TYPE_ALTERED", "COLUMN_RENAMED" */
  changeType: string;
  columnName: string;
  oldType?: string;
  newType?: string;
  description: string;
}

export interface SecretFinding {
  filePath: string;
  lineNumber: number;
  ruleName: string;
  /** "CRITICAL", "HIGH", "WARNING" */
  severity: string;
  redactedSnippet: string;
}

export interface ShaderInspectResult {
  targetShader: string;
  shaderType: string;
  totalVariants: number;
  theoreticalMax: number;
  passCount: number;
  totalKeywords: number;
  pragmas: ShaderPragmaDirective[];
  passes: ShaderPassInfo[];
  severity: string;
  isExplosion: boolean;
  strippingAdvice: string[];
  projectShaderCount: number;
  explosionCount: number;
  offenderList: ShaderVariantOffender[];
  message: string;
}

export interface ShaderPassInfo {
  name: string;
  passIndex: number;
  pragmas: ShaderPragmaDirective[];
  variantCount: number;
}

export interface ShaderPragmaDirective {
  directive: string;
  scope: string;
  keywords: string[];
  optionCount: number;
  lineNumber: number;
  rawLine: string;
}

export interface ShaderSelfHealingResult {
  engineType: string;
  brokenShaderCount: number;
  blackRenderMeshesCount: number;
  shaderCacheCleared: boolean;
  repairedMaterials: string[];
  success: boolean;
  /** "OPTIMAL", "REPAIRED", "ATTENTION_REQUIRED" */
  healthGrade: string;
  summary: string;
}

export interface ShaderVariantOffender {
  path: string;
  shaderType: string;
  variantCount: number;
  /** "OK", "WARNING", "CRITICAL" */
  severity: string;
  keywordCount: number;
  passCount: number;
  sizeBytes: number;
  sizeStr: string;
}

export interface SignatureDetails {
  /** "G", "B", "U", "N" */
  status: string;
  signer: string;
  keyId: string;
  fingerprint: string;
  date: string;
  raw: string;
}

/** Modal 73: Commit Signing & Verification */
export interface SignedCommitItem {
  commitHash: string;
  shortHash: string;
  authorName: string;
  authorEmail: string;
  subject: string;
  date: string;
  /** "G" (Good), "B" (Bad), "U" (Untrusted), "N" (Unsigned), etc. */
  signatureStatus: string;
  signer: string;
  keyId: string;
  /** "已验证 (Verified)", "未签名 (Unsigned)", "签名失效 (Bad)" */
  statusHuman: string;
}

export interface SourceFileDetail {
  path: string;
  name: string;
  size: number;
  sizeFormatted: string;
  lineCount: number;
  language: string;
  content: string;
  isBinary: boolean;
  isImage?: boolean;
  imageData?: string;
  imageWidth?: number;
  imageHeight?: number;
  lastCommitHash: string;
  lastAuthor: string;
  lastDate: string;
  lastSubject: string;
}

export interface SpatialOverlapItem {
  actorGuidA: string;
  actorNameA: string;
  actorClassA: string;
  subLevelA: string;
  actorGuidB: string;
  actorNameB: string;
  actorClassB: string;
  subLevelB: string;
  centerDistance: number;
  overlapVolume: number;
  boundingBoxA: BoundingBox3D;
  boundingBoxB: BoundingBox3D;
  severity: string;
  recommendedFix: string;
}

/** StashFileEntry describes one file touched by a stash entry. */
export interface StashFileEntry {
  path: string;
  /** A (added) M (modified) D (deleted) R (renamed) */
  status: string;
  additions: number;
  deletions: number;
}

export interface StashInfo {
  index: number;
  message: string;
  date: string;
}

export interface StorageBreakdownItem {
  extension: string;
  count: number;
  sizeBytes: number;
  sizeHuman: string;
}

export interface StoreSecretResult {
  success: boolean;
  secretId: string;
  secretType: string;
  maskedValue: string;
  scope: string;
  vaultFile: string;
  encryptionMethod: string;
  message: string;
}

export interface StudioCIActionResult {
  success: boolean;
  message: string;
}

export interface StudioCIResult {
  /** "GitHub Actions", "Tencent 工蜂 CI", "Local Studio Runner" */
  provider: string;
  activeBranch: string;
  latestRun?: CIPipelineRun;
  historyRuns: CIPipelineRun[];
  message: string;
}

export interface StudioChangelogCategory {
  category: string;
  icon: string;
  commits: CommitItem[];
}

export interface StudioChangelogResult {
  fromRef: string;
  toRef: string;
  totalCount: number;
  categories: StudioChangelogCategory[];
  markdownDoc: string;
}

export interface StudioDDCResult {
  localCachePath: string;
  sharedCachePath: string;
  isSharedConnected: boolean;
  cacheHitRatio: number;
  totalCacheSizeBytes: number;
  totalCacheSizeStr: string;
  staleBlobsCount: number;
  latencyMs: number;
  summary: string;
}

export interface StudioProjectNode {
  path: string;
  name: string;
  /** "MainWorktree", "LinkedWorktree", "Client", "Server", "Tooling", "Docs", "AdjacentRepo" */
  category: string;
  branch: string;
  isDirty: boolean;
  dirtyFilesCount: number;
  isLocked: boolean;
  lockReason?: string;
  lastCommitHash: string;
  lastCommitSubject: string;
  lastCommitAuthor: string;
  lastCommitDate: string;
  isCurrent: boolean;
  aheadCount: number;
  behindCount: number;
}

export interface StudioSwitchboardResult {
  actionExecuted: string;
  success: boolean;
  message: string;
  activeProjectPath: string;
  totalProjects: number;
  linkedWorktrees: number;
  adjacentRepos: number;
  projects: StudioProjectNode[];
}

export interface StudioWebhookItem {
  id: string;
  name: string;
  /** "WECOM", "FEISHU", "DINGTALK", "KODING" */
  webhookType: string;
  urlMasked: string;
  eventRules: string[];
  isEnabled: boolean;
  lastPingSuccess: boolean;
}

export interface StudioWebhookResult {
  totalWebhooks: number;
  webhooks: StudioWebhookItem[];
  lastDispatchedMessage: string;
  success: boolean;
  message: string;
}

export interface SubmoduleInfo {
  name: string;
  path: string;
  url: string;
  branch: string;
  commit: string;
  /** "synced", "modified", "uninitialized" */
  status: string;
}

export interface SubtreeActionResult {
  success: boolean;
  message: string;
  output: string;
}

export interface SubtreeItem {
  prefix: string;
  remoteOrUrl: string;
  branch: string;
  lastCommit: string;
  lastSquash: string;
}

export interface SubtreeListResult {
  subtrees: SubtreeItem[];
}

export interface SymbolsDefinesAuditResult {
  analysisKnown: boolean;
  totalDefines: number;
  activeDefines: string[];
  baseDefines: string[];
  targetDefines: string[];
  addedDefines: string[];
  removedDefines: string[];
  dangerousDebugDefines: string[];
  /** "OK", "WARNING", "CRITICAL" */
  severity: string;
  summary: string;
}

export interface TAPDBindResult {
  success: boolean;
  commitHash: string;
  tapdId: string;
  previousStatus: string;
  currentStatus: string;
  updatedItem: TAPDWorkItem | null;
  message: string;
}

export interface TAPDCommitReferenceResult {
  commitMessage: string;
  foundTickets: TAPDReferenceItem[];
  hasTapdTag: boolean;
  formattedTag: string;
}

export interface TAPDReferenceItem {
  ticketId: string;
  /** "tapd", "bug", "story", "task", "hash" */
  prefixType: string;
  rawTag: string;
  workItem?: TAPDWorkItem;
}

export interface TAPDWorkItem {
  id: string;
  /** "story", "task", "bug" */
  type: string;
  title: string;
  /** "open", "in_progress", "resolved", "verified", "closed" */
  status: string;
  /** "待规划", "进行中", "已解决", "已验证", "已关闭" */
  statusLabel: string;
  /** "urgent", "high", "normal", "low" */
  priority: string;
  priorityLabel: string;
  owner: string;
  creator: string;
  iteration: string;
  createdTime: string;
  updatedTime: string;
  description?: string;
  boundCommits: string[];
  url: string;
}

export interface TAPDWorkItemListResult {
  workspaceId: string;
  totalCount: number;
  storiesCount: number;
  tasksCount: number;
  bugsCount: number;
  items: TAPDWorkItem[];
  isConnected: boolean;
  message: string;
}

export interface TableAnomalyItem {
  rowIndex: number;
  primaryKey: string;
  columnName: string;
  /** "DUPLICATE_KEY", "INVALID_FK", "NAN_OR_NULL", "TYPE_MISMATCH", "OUT_OF_BOUNDS" */
  anomalyType: string;
  invalidVal: string;
  /** "CRITICAL", "WARNING", "INFO" */
  severity: string;
  description: string;
}

export interface TagDetail {
  name: string;
  commitHash: string;
  taggerName: string;
  taggerEmail: string;
  date: string;
  message: string;
  isAnnotated: boolean;
}

export interface TagInfo {
  name: string;
  /**
   * IsAnnotated is true for tags created with -a/-s. Lightweight tags are
   * indistinguishable by message alone: %(contents:subject) falls back to the
   * commit subject, so every tag looks annotated until the ref's object type
   * is checked.
   */
  isAnnotated: boolean;
  /**
   * CommitHash is always the commit the tag resolves to, even for annotated
   * tags, whose ref itself points at a tag object with a different OID.
   */
  commitHash: string;
  message: string;
}

export interface TeamActivityResult {
  days: ActivityDay[];
  contributors: ContributorActivity[];
  hotspots: FileChurnHotspot[];
  totalCommits: number;
  activeDays: number;
  currentStreak: number;
  longestStreak: number;
  peakHour: string;
  peakWeekday: string;
  avgCommitsPerDay: number;
  mergeCommits: number;
  hourBuckets: number[];
  weekdayCommits: number[];
  rangeStart: string;
  rangeEnd: string;
  busiestDay: string;
  busiestDayCount: number;
  summary: string;
}

export interface TextureChannelAuditItem {
  path: string;
  /** "ORM", "MRAO", "SplitChannels", "Normal" */
  packingType: string;
  isCompliant: boolean;
  issue?: string;
  recommendation: string;
}

export interface TextureChannelPackingResult {
  totalTexturesScanned: number;
  ormCompliantCount: number;
  unpackedTexturesCount: number;
  invalidNormalMapsCount: number;
  vramWasteEstimateBytes: number;
  vramWasteStr: string;
  textures: TextureChannelAuditItem[];
  summary: string;
}

export interface TextureLintItem {
  path: string;
  width: number;
  height: number;
  dimensions: string;
  format: string;
  sizeBytes: number;
  sizeStr: string;
  /** Power of Two */
  isPOT: boolean;
  /** > 4096px */
  isOverSized: boolean;
  /** "OK", "WARNING", "ERROR" */
  severity: string;
  issue: string;
  advice: string;
}

export interface TextureLintResult {
  totalChecked: number;
  totalIssues: number;
  items: TextureLintItem[];
}

export interface TextureStreamingAuditItem {
  path: string;
  dimensionsKnown: boolean;
  width: number;
  height: number;
  isPot: boolean;
  mipCount: number;
  streamingEnabled: boolean;
  /** VT enabled for >= 4096 */
  virtualTexture: boolean;
  rawSizeBytes: number;
  streamSizeBytes: number;
  /** "OK", "WARNING", "CRITICAL" */
  severity: string;
  issue?: string;
}

export interface TransformConflictItem {
  guid: string;
  actorName: string;
  subLevel: string;
  currentPos: Vector3D;
  targetPos: Vector3D;
  deltaDistance: number;
  currentRot: Vector3D;
  targetRot: Vector3D;
  recommendedAction: string;
}

export interface UnityMergeStatus {
  isConfigured: boolean;
  detectedToolPath: string;
  configuredToolPath: string;
  installedVersions: string[];
  hasGitattributes: boolean;
}

export interface UnityMetaMismatch {
  assetPath: string;
  metaPath: string;
  /** "missing_meta", "orphan_meta", "unpaired_staged" */
  issueType: string;
  message: string;
}

export interface UnrealAssetDiffResult {
  assetPath: string;
  baseRef: string;
  targetRef: string;
  assetFormat: string;
  analysisKnown: boolean;
  analysisNote?: string;
  baseHeader?: AssetPackageHeader;
  targetHeader?: AssetPackageHeader;
  addedNodes: AssetGraphNode[];
  removedNodes: AssetGraphNode[];
  modifiedNodes: ModifiedNodeItem[];
  unchangedCount: number;
  totalBaseNodes: number;
  totalTargetNodes: number;
  hasGraphChanges: boolean;
  visualSummary: string;
  message: string;
}

export interface UnrealCacheCleanResult {
  cleanedFolders: string[];
  freedSizeBytes: number;
  freedSizeStr: string;
}

export interface UnrealProjectInfo {
  isUnrealProject: boolean;
  uprojectPath: string;
  projectName: string;
  engineVersion: string;
  detectedEnginePaths: string[];
  cacheFoldersFound: string[];
  cacheSizeBytes: number;
  cacheSizeStr: string;
}

export interface Vector3D {
  x: number;
  y: number;
  z: number;
}

export interface WorkspaceCategoryStat {
  category: string;
  files: number;
  additions: number;
  deletions: number;
}

export interface WorkspaceStats {
  stagedFiles: number;
  stagedAdditions: number;
  stagedDeletions: number;
  unstagedFiles: number;
  unstagedAdditions: number;
  unstagedDeletions: number;
  untrackedFiles: number;
  conflictedFiles: number;
  totalAdditions: number;
  totalDeletions: number;
  categories: WorkspaceCategoryStat[];
  files: FileChurnStat[];
  summary: string;
}

export interface WorktreeInfo {
  path: string;
  head: string;
  branch: string;
  isBare: boolean;
  isMain: boolean;
  isLocked: boolean;
  lockReason: string;
}

export interface WorldDataLayersDiffResult {
  totalDataLayers: number;
  layers: DataLayerDiffItem[];
  runtimeBudgetOverrun: boolean;
  streamingConflicts: string[];
  summary: string;
}

export interface ExternalToolInfo {
  id: string;
  name: string;
  executable: string;
  isAvailable: boolean;
  supportsDiff: boolean;
  supportsMerge: boolean;
  description: string;
}

export interface ExternalToolConfig {
  diffTool: string;
  mergeTool: string;
  customDiffCmd: string;
  customMergeCmd: string;
  isGlobal: boolean;
}

export interface RemoteWebInfo {
  hasRemote: boolean;
  remoteName: string;
  remoteUrl: string;
  provider: "github" | "gitlab" | "gitea" | "gitee" | "bitbucket" | "generic";
  webBaseUrl: string;
  commitUrlTpl: string;
  branchUrlTpl: string;
  fileUrlTpl: string;
  compareUrlTpl: string;
}

// ---- Pull, Push, Rebase, Merge Advanced Ops ----

export interface PullAdvancedOptions {
  remote: string;
  branch: string;
  strategy: "default" | "rebase" | "ff-only" | "no-ff" | "merge";
  autostash: boolean;
  prune: boolean;
  submodules: boolean;
}

export interface PullResult {
  success: boolean;
  message: string;
  updated: boolean;
  alreadyUp: boolean;
  fastForward: boolean;
  hasConflicts: boolean;
  conflicts?: string[];
}

export interface PushAdvancedOptions {
  remote: string;
  branch: string;
  remoteBranch?: string;
  forceMode: "none" | "lease" | "force";
  setUpstream: boolean;
  pushTags: boolean;
  followTags: boolean;
  skipCI: boolean;
}

export interface PushResult {
  success: boolean;
  message: string;
}

export interface MergePreviewResult {
  canMergeCleanly: boolean;
  hasConflicts: boolean;
  conflicts: string[];
  changedFiles: string[];
  aheadCount: number;
  behindCount: number;
  baseCommit: string;
  targetCommit: string;
  details: string;
}

export interface MergeAdvancedOptions {
  targetBranch: string;
  strategy: "default" | "no-ff" | "ff-only" | "squash";
  strategyOption: "" | "ours" | "theirs";
  commitMessage: string;
  autostash: boolean;
}

export interface MergeResult {
  success: boolean;
  message: string;
  hasConflicts: boolean;
  conflicts?: string[];
}

export interface RebaseAdvancedOptions {
  upstream: string;
  onto?: string;
  autostash: boolean;
  autosquash: boolean;
}

export interface RebaseResult {
  success: boolean;
  message: string;
  hasConflicts: boolean;
  conflicts?: string[];
}

export interface RebaseProgressStatus {
  isRebasing: boolean;
  headName: string;
  onto: string;
  currentStep: number;
  totalSteps: number;
  currentCommit: string;
  hasConflicts: boolean;
  conflictedFiles: string[];
}

// =========================================================================
// 闭环补齐：诊断 → 修复
// =========================================================================

export interface LfsConvertItem {
  path: string;
  pattern: string;
  status: "converted" | "skipped" | "failed";
  detail?: string;
  oldBytes: number;
  newBytes: number;
}

export interface LfsConvertResult {
  success: boolean;
  converted: number;
  failed: number;
  skipped: number;
  items: LfsConvertItem[];
  addedPatterns: string[];
  message: string;
}

export interface LfsBulkUnlockItem {
  path: string;
  owner: string;
  age: string;
  status: "unlocked" | "failed" | "skipped";
  detail?: string;
}

export interface LfsBulkUnlockResult {
  success: boolean;
  unlocked: number;
  failed: number;
  skipped: number;
  items: LfsBulkUnlockItem[];
  message: string;
}

export interface RangeDiffEntry {
  oldIndex: number;
  newIndex: number;
  kind: "=" | "!" | "<" | ">";
  oldHash: string;
  newHash: string;
  subject: string;
}

export interface RangeDiffResult {
  entries: RangeDiffEntry[];
  equal: number;
  changed: number;
  added: number;
  removed: number;
  raw: string;
  message: string;
}

export interface FixupCommitResult {
  hash: string;
  subject: string;
  mode: string;
  message: string;
}

export interface ObjectStoreTaskResult {
  task: string;
  label: string;
  success: boolean;
  skipped: boolean;
  skipReason?: string;
  output: string;
  durationMs: number;
}

export interface ObjectStoreOptimizeResult {
  success: boolean;
  results: ObjectStoreTaskResult[];
  gitDirBefore: string;
  gitDirAfter: string;
  message: string;
}

export interface OverlapFile {
  path: string;
  authors: string[];
  lastTime: string;
  myChange: boolean;
}

export interface MergeOverlapResult {
  overlapFiles: OverlapFile[];
  myFileCount: number;
  teamFileCount: number;
  authors: string[];
  riskLevel: "none" | "low" | "medium" | "high";
  message: string;
}

export interface HealthFixResult {
  key: string;
  executed: boolean;
  success: boolean;
  needsInput: boolean;
  target: string;
  output: string;
  hint: string;
}

export interface FixCenterItem {
  id: string;
  title: string;
  severity: "pass" | "warn" | "fail";
  count: number;
  summary: string;
  fixable: boolean;
  fixLabel: string;
  targets: string[];
  needsReview: boolean;
}

export interface FixCenterResult {
  items: FixCenterItem[];
  totalFixes: number;
  scannedAt: string;
  message: string;
  /** False when any diagnostic failed; "nothing to fix" is then unproven. */
  analysisComplete: boolean;
  warnings: string[];
}

export interface AuditFixResult {
  fixId: string;
  success: boolean;
  applied: number;
  skipped: number;
  failed: number;
  details: string[];
  backupDir: string;
  message: string;
}

export interface SecretRemediationResult {
  success: boolean;
  unstaged: number;
  failed: number;
  patternsAdded: string[];
  allowlistPath: string;
  details: string[];
  message: string;
}

export interface AssetDiffSide {
  exists: boolean;
  ref: string;
  size: number;
  sizeHuman: string;
  width: number;
  height: number;
  format: string;
  mime: string;
  dataUrl: string;
  previewable: boolean;
}

export interface AssetDiffResult {
  path: string;
  kind: "image" | "audio" | "model" | "binary";
  old: AssetDiffSide;
  new: AssetDiffSide;
  sizeDelta: number;
  widthDelta: number;
  heightDelta: number;
  message: string;
}

export interface LfsMigrateRow {
  pattern: string;
  files: number;
  size: string;
  progress: string;
}

export interface LfsMigratePreview {
  rows: LfsMigrateRow[];
  raw: string;
  command: string;
  message: string;
}

export interface LfsMigrateOptions {
  patterns: string[];
  everything: boolean;
  revRange: string;
  createBundle: boolean;
}

export interface LfsMigrateResult {
  success: boolean;
  rewrittenFrom: string;
  rewrittenTo: string;
  bundlePath: string;
  output: string;
  message: string;
}

export interface TagVerifyResult {
  tag: string;
  verified: boolean;
  signer: string;
  detail: string;
  message: string;
}

export interface AssetImpactRef {
  path: string;
  fileType: string;
  matchCount: number;
  depth: number;
  lastAuthor: string;
  lastTime: string;
}

export interface AssetImpactResult {
  targetPath: string;
  directCount: number;
  indirectCount: number;
  totalCount: number;
  references: AssetImpactRef[];
  owners: string[];
  riskLevel: "none" | "low" | "medium" | "high" | "unknown";
  safeToDelete: boolean;
  truncated: boolean;
  message: string;
  /** False when any scan hop failed or skipped files; counts are then a floor. */
  analysisComplete: boolean;
  warnings: string[];
}

export interface EngineBootstrapStep {
  id: string;
  label: string;
  status: "applied" | "skipped" | "failed";
  detail: string;
}

export interface EngineBootstrapOptions {
  engine: string;
  writeGitattributes: boolean;
  writeGitignore: boolean;
  untrackArtifacts: boolean;
  configureUnityMerge: boolean;
  lfsExtensions: string[];
}

export interface EngineBootstrapResult {
  engine: string;
  success: boolean;
  applied: number;
  skipped: number;
  failed: number;
  steps: EngineBootstrapStep[];
  readiness: number;
  message: string;
}

export interface OperationLogEntry {
  index: number;
  hash: string;
  shortHash: string;
  selector: string;
  action: string;
  actionLabel: string;
  subject: string;
  time: string;
  undoable: boolean;
  undoHint: string;
}

export interface OperationLogResult {
  entries: OperationLogEntry[];
  total: number;
  canUndo: boolean;
  message: string;
}

export interface UndoResult {
  success: boolean;
  action: string;
  fromHash: string;
  toHash: string;
  keptDirty: boolean;
  message: string;
}

export interface TerminalStatus {
  running: boolean;
  shell: string;
  cols: number;
  rows: number;
  dir: string;
  backlog: string;
  message: string;
}

export interface WorktreeOpResult {
  success: boolean;
  action: string;
  output: string;
  details: string[];
  message: string;
}

export interface GitHookContent {
  name: string;
  path: string;
  exists: boolean;
  enabled: boolean;
  size: number;
  content: string;
  message: string;
}

// ------------------------------------------------------------ PR / MR -------

export interface PrProviderInfo {
  supported: boolean;
  provider: "github" | "gitlab" | "gitee" | "bitbucket" | "gitea" | "unknown";
  host: string;
  owner: string;
  repo: string;
  projectPath: string;
  remoteUrl: string;
  webBaseUrl: string;
  apiBaseUrl: string;
  hasToken: boolean;
  tokenSource: string;
  account: string;
  tokenHint: string;
  message: string;
}

export interface PrSummary {
  number: number;
  title: string;
  state: "open" | "closed" | "merged";
  isDraft: boolean;
  author: string;
  sourceBranch: string;
  targetBranch: string;
  createdAt: string;
  updatedAt: string;
  commentCount: number;
  additions: number;
  deletions: number;
  changedFiles: number;
  webUrl: string;
  labels: string[];
  checksState: "success" | "failure" | "pending" | "neutral" | "";
  mergeable: boolean;
  mergeableState: string;
}

export interface PrFile {
  path: string;
  oldPath: string;
  status: string;
  additions: number;
  deletions: number;
  patch: string;
  truncated: boolean;
  binary: boolean;
}

export interface PrComment {
  id: string;
  author: string;
  body: string;
  path: string;
  line: number;
  side: string;
  createdAt: string;
  kind: "review" | "issue";
  replyTo: string;
}

export interface PrCheck {
  name: string;
  status: string;
  conclusion: string;
  url: string;
  detail: string;
}

export interface PrDetail {
  summary: PrSummary;
  body: string;
  files: PrFile[];
  comments: PrComment[];
  checks: PrCheck[];
  commitCount: number;
  baseSha: string;
  headSha: string;
  startSha: string;
  filesTruncated: boolean;
  message: string;
}

export interface PrListResult {
  provider: string;
  items: PrSummary[];
  total: number;
  hasMore: boolean;
  fetchedAt: string;
  webBaseUrl: string;
  message: string;
}

export interface PrActionResult {
  success: boolean;
  action: string;
  message: string;
  webUrl: string;
  newState: string;
}

export interface PrAnchor {
  path: string;
  line: number;
  side: string;
}
