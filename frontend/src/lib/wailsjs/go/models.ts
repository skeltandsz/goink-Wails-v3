/* eslint-disable @typescript-eslint/no-explicit-any */
// 由 scripts/gen_shims.mjs 生成。Wails v3 兼容层：按 v2 包命名空间组织模型。
// Go 指针字段可选（field?: T），值字段必填（field: T），并带索引签名。

export namespace app {
  export type Loose = { [k: string]: any };
  export interface ChatInput {
    session_id: string;
    novel_id: number;
    message: string;
    provider_name: string;
    model_id: string;
    reasoning_effort: string;
    [k: string]: any;
  }
  export interface ChatResult {
    session_id: string;
    turn_id: number;
    final_text: string;
    [k: string]: any;
  }
  export interface CommitFileListResult {
    commit?: git.CommitInfo;
    files: git.FileEntry[];
    [k: string]: any;
  }
  export interface CompressInput {
    session_id: string;
    provider_name: string;
    model_id: string;
    [k: string]: any;
  }
  export interface CompressResult {
    turn_id: number;
    [k: string]: any;
  }
  export interface ComputeStyleStatsInput {
    sample_ids: number[];
    [k: string]: any;
  }
  export interface CreateArcNodeInput {
    story_arc_id: number;
    title: string;
    description?: string;
    target_reading_number: number;
    [k: string]: any;
  }
  export interface CreateChapterInput {
    novel_id: number;
    title: string;
    [k: string]: any;
  }
  export interface CreateCharacterInput {
    name: string;
    description?: string;
    personality?: string;
    abilities?: string;
    [k: string]: any;
  }
  export interface CreateLocationInput {
    name: string;
    location_type?: string;
    description?: string;
    detail_json?: string;
    parent_location_id?: number;
    tags?: string;
    [k: string]: any;
  }
  export interface CreateNovelInput {
    title: string;
    description?: string;
    genre?: string;
    [k: string]: any;
  }
  export interface CreateNovelSettingInput {
    category: string;
    content: string;
    [k: string]: any;
  }
  export interface CreatePreferenceInput {
    is_global: boolean;
    category: string;
    content: string;
    [k: string]: any;
  }
  export interface CreateReaderPerspectiveInput {
    type: string;
    content: string;
    planted_chapter_id: number;
    related_truth?: string;
    revealed_chapter_id?: number;
    [k: string]: any;
  }
  export interface CreateStoryArcInput {
    name: string;
    arc_type: string;
    description?: string;
    importance?: number;
    [k: string]: any;
  }
  export interface CreateStyleSampleInput {
    novel_id: number;
    is_global: boolean;
    name: string;
    content: string;
    tags: string[];
    [k: string]: any;
  }
  export interface CreateTimelineEntryInput {
    category: string;
    title: string;
    content?: string;
    detail_json?: string;
    target_reading_number: number;
    importance?: number;
    source_chapter_id?: number;
    source?: string;
    [k: string]: any;
  }
  export interface DeleteSkillInput {
    novel_id: number;
    name: string;
    source: string;
    [k: string]: any;
  }
  export interface DeleteStyleSampleInput {
    id: number;
    [k: string]: any;
  }
  export interface ExtractStyleInput {
    task_id: string;
    sample_ids: number[];
    provider_name: string;
    model_id: string;
    reasoning_effort: string;
    [k: string]: any;
  }
  export interface GetSessionsInput {
    novel_id: number;
    page: number;
    size: number;
    search: string;
    [k: string]: any;
  }
  export interface ImportNovelInput {
    file_path: string;
    [k: string]: any;
  }
  export interface ImportWithLLMInput {
    file_path: string;
    provider_name: string;
    model_id: string;
    [k: string]: any;
  }
  export interface InstallRemoteSkillInput {
    name: string;
    target: string;
    novel_id: number;
    expected_content?: string;
    [k: string]: any;
  }
  export interface ListRemoteSkillsInput {
    page: number;
    size: number;
    query: string;
    [k: string]: any;
  }
  export interface ListSkillsInput {
    novel_id: number;
    [k: string]: any;
  }
  export interface ListSlashCommandsInput {
    novel_id: number;
    [k: string]: any;
  }
  export interface ListStyleSamplesInput {
    novel_id: number;
    page: number;
    size: number;
    search: string;
    [k: string]: any;
  }
  export interface PreferenceResult {
    global: preference.PreferenceItem[];
    novel: preference.PreferenceItem[];
    token_count: number;
    over_budget: boolean;
    [k: string]: any;
  }
  export interface SaveContentInput {
    novel_id: number;
    path: string;
    content: string;
    expected_content?: string;
    [k: string]: any;
  }
  export interface SaveSettingsInput {
    [k: string]: any;
  }
  export interface SessionDetail {
    session_id: string;
    novel_id: number;
    title: string;
    model: string;
    reasoning_effort: string;
    active_version: number;
    last_turn_id: number;
    usage?: any;
    created_at: string;
    updated_at: string;
    [k: string]: any;
  }
  export interface SessionMeta {
    session_id: string;
    title: string;
    updated_at: string;
    [k: string]: any;
  }
  export interface SetActiveNovelInput {
    novel_id: number;
    [k: string]: any;
  }
  export interface SettingResult {
    items: setting.SettingItem[];
    token_count: number;
    over_budget: boolean;
    [k: string]: any;
  }
  export interface SlashCommand {
    name: string;
    description: string;
    type: string;
    [k: string]: any;
  }
  export interface StartupState {
    phase: StartupPhase;
    error?: string;
    version: number;
    [k: string]: any;
  }
  export interface TestConnectionInput {
    provider_name: string;
    chat_url: string;
    api_key: string;
    model_id: string;
    [k: string]: any;
  }
  export interface UpdateArcNodeInput {
    title: string;
    description: string;
    target_reading_number: number;
    actual_chapter_id?: number;
    status: string;
    [k: string]: any;
  }
  export interface UpdateChapterPlanInput {
    scope: string;
    content: string;
    [k: string]: any;
  }
  export interface UpdateCharacterInput {
    name: string;
    description: string;
    abilities: string;
    [k: string]: any;
  }
  export interface UpdateLocationInput {
    name: string;
    location_type: string;
    description: string;
    parent_location_id?: number;
    tags: string;
    [k: string]: any;
  }
  export interface UpdateNovelInput {
    title: string;
    description: string;
    genre: string;
    [k: string]: any;
  }
  export interface UpdateNovelSettingInput {
    category: string;
    content: string;
    [k: string]: any;
  }
  export interface UpdatePreferenceInput {
    category: string;
    content: string;
    is_global: boolean;
    [k: string]: any;
  }
  export interface UpdateReaderPerspectiveInput {
    type: string;
    content: string;
    planted_chapter_id?: number;
    related_truth: string;
    revealed_chapter_id?: number;
    [k: string]: any;
  }
  export interface UpdateStoryArcInput {
    name: string;
    description: string;
    arc_type: string;
    importance: number;
    status: string;
    [k: string]: any;
  }
  export interface UpdateStyleSampleInput {
    id: number;
    name: string;
    content: string;
    tags: string[];
    is_global: boolean;
    novel_id: number;
    [k: string]: any;
  }
  export interface UpdateTimelineEntryInput {
    title: string;
    content: string;
    target_reading_number: number;
    importance: number;
    status: string;
    resolved_chapter_id?: number;
    [k: string]: any;
  }
  export type ExtractPatternInput = pattern.ExtractPatternInput;
  export type ExtractPatternResult = pattern.ExtractPatternResult;
  export type ImportNovelResult = imp.ImportResult;
  export type PlaceChapterInput = chapter.PlaceInput;
  export type PlaceVolumeInput = volume.PlaceInput;
  export type StartupPhase = string;
}

export namespace apperr {
  export type Loose = { [k: string]: any };
  export interface Result<T> {
    data?: T;
    err_code: Code;
    err_msg?: string;
    [k: string]: any;
  }
  export type Empty = { [k: string]: any };
  export type Code = string;
}

export namespace chapter {
  export type Loose = { [k: string]: any };
  export interface Chapter {
    id: number;
    novel_id: number;
    volume_id?: number;
    sort_order: number;
    title: string;
    summary: string;
    word_count: number;
    created_at: string;
    updated_at: string;
    file_path: string;
    outline_file_path: string;
    reading_number: number;
    [k: string]: any;
  }
  export interface DeleteResult {
    deleted: boolean;
    references: deletion.Blocker[];
    [k: string]: any;
  }
  export interface PlaceInput {
    novel_id: number;
    source_chapter_id?: number;
    title?: string;
    target_volume_id?: number;
    before_chapter_id?: number;
    [k: string]: any;
  }
}

export namespace character {
  export type Loose = { [k: string]: any };
  export interface Character {
    id: number;
    novel_id: number;
    name: string;
    description: string;
    personality: string;
    abilities: string;
    created_at: string;
    updated_at: string;
    [k: string]: any;
  }
  export interface CharacterRelation {
    id: number;
    novel_id: number;
    source_character_id: number;
    target_character_id: number;
    relation_describe: string;
    description: string;
    chapter_id?: number;
    is_current: boolean;
    created_at: string;
    [k: string]: any;
  }
}

export namespace config {
  export type Loose = { [k: string]: any };
  export interface AppSettings {
    ID: number;
    last_novel_id: number;
    selected_model_key: string;
    reasoning_effort: string;
    approval_mode: string;
    last_session_id: string;
    user_name: string;
    git_name: string;
    git_email: string;
    dismissed_version: string;
    last_update_check_at: string;
    [k: string]: any;
  }
}

export namespace deletion {
  export type Loose = { [k: string]: any };
  export interface Blocker {
    kind: string;
    id: number;
    label: string;
    [k: string]: any;
  }
}

export namespace git {
  export type Loose = { [k: string]: any };
  export interface CommitInfo {
    hash: string;
    shortHash: string;
    message: string;
    time: string;
    authorName: string;
    authorEmail: string;
    filesChanged: number;
    insertions: number;
    deletions: number;
    [k: string]: any;
  }
  export interface FileDiff {
    path: string;
    changeType: string;
    original: string;
    modified: string;
    [k: string]: any;
  }
  export interface FileEntry {
    path: string;
    changeType: string;
    oldPath: string;
    [k: string]: any;
  }
}

export namespace imp {
  export type Loose = { [k: string]: any };
  export interface ImportResult {
    novel_id: number;
    title: string;
    chapter_count: number;
    skipped_count: number;
    skipped_chapters: imp.SkippedChapter[];
    needs_llm: boolean;
    file_path?: string;
    [k: string]: any;
  }
  export interface SkippedChapter {
    title: string;
    reason: string;
    [k: string]: any;
  }
}

export namespace llm {
  export type Loose = { [k: string]: any };
  export interface AvailableModel {
    Key: string;
    ModelID: string;
    ProviderKey: string;
    ProviderName: string;
    ModelName: string;
    ContextWindow: number;
    MaxOutputTokens: number;
    SupportsThinking: boolean;
    ReasoningLevels: string[];
    SupportsVision: boolean;
    [k: string]: any;
  }
  export interface LLMConfigView {
    providers: llm.ProviderView[];
    [k: string]: any;
  }
  export interface ModelInfo {
    id: string;
    name: string;
    context_window: number;
    max_output_tokens: number;
    supports_thinking: boolean;
    reasoning_levels?: string[];
    supports_vision: boolean;
    [k: string]: any;
  }
  export interface ProviderView {
    key: string;
    name: string;
    chat_url: string;
    api_key: string;
    platform_url: string;
    help_text: string;
    temperature: number;
    source: string;
    builtin_models: llm.ModelInfo[];
    custom_models: llm.ModelInfo[];
    [k: string]: any;
  }
  export interface TestConnectionResult {
    url: string;
    warning: string;
    [k: string]: any;
  }
}

export namespace location {
  export type Loose = { [k: string]: any };
  export interface Location {
    id: number;
    novel_id: number;
    name: string;
    location_type: string;
    description: string;
    detail_json: string;
    parent_location_id?: number;
    tags: string;
    created_at: string;
    updated_at: string;
    [k: string]: any;
  }
  export interface LocationRelation {
    id: number;
    novel_id: number;
    location_a_id: number;
    location_b_id: number;
    relation_type: string;
    description: string;
    created_at: string;
    updated_at: string;
    [k: string]: any;
  }
}

export namespace novel {
  export type Loose = { [k: string]: any };
  export interface Novel {
    id: number;
    title: string;
    genre: string;
    description: string;
    created_at: string;
    updated_at: string;
    [k: string]: any;
  }
}

export namespace pattern {
  export type Loose = { [k: string]: any };
  export interface BatchTrace {
    index: number;
    input_count: number;
    output_count: number;
    approx_tokens: number;
    [k: string]: any;
  }
  export interface BoundaryHint {
    start_chapter_id: number;
    end_chapter_id: number;
    hint: string;
    [k: string]: any;
  }
  export interface ChapterSummaryItem {
    chapter_id: number;
    summary: string;
    [k: string]: any;
  }
  export interface Chunk {
    name: string;
    start_chapter_id: number;
    end_chapter_id: number;
    content: string;
    [k: string]: any;
  }
  export interface ChunkRoundTrace {
    round: number;
    input_count: number;
    output_count: number;
    token_count: number;
    batches: pattern.BatchTrace[];
    chunks: pattern.Chunk[];
    [k: string]: any;
  }
  export interface ExtractPatternInput {
    task_id?: string;
    novel_id: number;
    provider_name: string;
    model_id: string;
    reasoning_effort: string;
    chapter_ids?: number[];
    [k: string]: any;
  }
  export interface ExtractPatternResult {
    task_id?: string;
    name: string;
    description: string;
    raw_content: string;
    file_path: string;
    trace?: pattern.Trace;
    [k: string]: any;
  }
  export interface Trace {
    task_id?: string;
    novel_id: number;
    chapter_count: number;
    context_window: number;
    batch_budget: number;
    boundaries: pattern.BoundaryHint[];
    summaries: pattern.ChapterSummaryItem[];
    chunk_rounds: pattern.ChunkRoundTrace[];
    final_tokens: number;
    [k: string]: any;
  }
}

export namespace preference {
  export type Loose = { [k: string]: any };
  export interface PreferenceItem {
    id: number;
    novel_id: number;
    is_global: boolean;
    category: string;
    content: string;
    created_at: string;
    updated_at: string;
    [k: string]: any;
  }
}

export namespace reader {
  export type Loose = { [k: string]: any };
  export interface ReaderPerspective {
    id: number;
    novel_id: number;
    type: string;
    content: string;
    related_truth: string;
    planted_chapter_id?: number;
    revealed_chapter_id?: number;
    created_at: string;
    [k: string]: any;
  }
}

export namespace remote {
  export type Loose = { [k: string]: any };
  export interface RemoteSkillMeta {
    name: string;
    description: string;
    category: string;
    mode: string;
    author: string;
    version: number;
    file: string;
    [k: string]: any;
  }
}

export namespace search {
  export type Loose = { [k: string]: any };
  export interface Result {
    type: string;
    id: number;
    title: string;
    subtitle: string;
    chapter_id: number;
    reading_number: number;
    file_path: string;
    match_prefix: string;
    match_hit: string;
    match_suffix: string;
    match_position: number;
    match_len: number;
    relevance: number;
    panel_id: string;
    [k: string]: any;
  }
}

export namespace session {
  export type Loose = { [k: string]: any };
  export interface Message {
    id: number;
    session_id: string;
    turn_id: number;
    role: string;
    content: string;
    thinking_content?: string;
    token_count: number;
    extra_metadata?: string;
    version: number;
    to_api: boolean;
    to_frontend: boolean;
    event_type?: EventType;
    agent_type: string;
    sub_task_id?: string;
    created_at: string;
    [k: string]: any;
  }
  export type EventType = string;
}

export namespace setting {
  export type Loose = { [k: string]: any };
  export interface SettingItem {
    id: number;
    novel_id: number;
    category: string;
    content: string;
    created_at: string;
    updated_at: string;
    [k: string]: any;
  }
}

export namespace skill {
  export type Loose = { [k: string]: any };
  export interface SkillMeta {
    name: string;
    description: string;
    category: string;
    mode: string;
    author: string;
    version: number;
    source: string;
    [k: string]: any;
  }
}

export namespace storage {
  export type Loose = { [k: string]: any };
  export interface PageResult<T> {
    items: T[];
    total: number;
    page: number;
    size: number;
    total_pages: number;
    [k: string]: any;
  }
}

export namespace storyarc {
  export type Loose = { [k: string]: any };
  export interface ArcNode {
    id: number;
    novel_id: number;
    story_arc_id: number;
    title: string;
    description: string;
    target_reading_number: number;
    actual_chapter_id?: number;
    status: string;
    created_at: string;
    updated_at: string;
    [k: string]: any;
  }
  export interface StoryArc {
    id: number;
    novel_id: number;
    name: string;
    description: string;
    arc_type: string;
    importance: number;
    status: string;
    reactivate_at: string;
    created_at: string;
    updated_at: string;
    [k: string]: any;
  }
}

export namespace style {
  export type Loose = { [k: string]: any };
  export interface ExtractResult {
    name: string;
    description: string;
    raw_content: string;
    file_path: string;
    [k: string]: any;
  }
  export interface Sample {
    id: number;
    novel_id: number;
    is_global: boolean;
    name: string;
    content: string;
    preview: string;
    tags: style.StringSlice;
    word_count: number;
    created_at: string;
    updated_at: string;
    [k: string]: any;
  }
  export interface Stats {
    total_chars: number;
    total_words: number;
    sentence_count: number;
    short_sent_pct: number;
    mid_sent_pct: number;
    long_sent_pct: number;
    avg_sent_len: number;
    sent_len_std_dev: number;
    comma_density: number;
    period_density: number;
    exclaim_density: number;
    question_density: number;
    quote_density: number;
    paragraph_count: number;
    avg_para_len: number;
    [k: string]: any;
  }
  export type StringSlice = string[];
}

export namespace timeline {
  export type Loose = { [k: string]: any };
  export interface ChapterPlan {
    novel_id: number;
    scope: string;
    content: string;
    [k: string]: any;
  }
  export interface TimelineEntry {
    id: number;
    novel_id: number;
    category: string;
    status: string;
    title: string;
    content: string;
    detail_json: string;
    target_reading_number: number;
    importance: number;
    source_chapter_id?: number;
    source: string;
    resolved_chapter_id?: number;
    created_at: string;
    updated_at: string;
    [k: string]: any;
  }
}

export namespace update {
  export type Loose = { [k: string]: any };
  export interface CheckResult {
    currentVersion: string;
    latest: update.ReleaseInfo;
    hasUpdate: boolean;
    [k: string]: any;
  }
  export interface ReleaseInfo {
    tag_name: string;
    name: string;
    body: string;
    html_url: string;
    published_at: string;
    [k: string]: any;
  }
}

export namespace volume {
  export type Loose = { [k: string]: any };
  export interface PlaceInput {
    novel_id: number;
    source_volume_id?: number;
    name?: string;
    before_volume_id?: number;
    [k: string]: any;
  }
  export interface Volume {
    id: number;
    novel_id: number;
    name: string;
    sort_order: number;
    created_at: string;
    updated_at: string;
    outline_file_path: string;
    [k: string]: any;
  }
}

export namespace writing {
  export type Loose = { [k: string]: any };
  export interface DailyActivity {
    date: string;
    words: number;
    [k: string]: any;
  }
  export interface WritingStats {
    total_words: number;
    total_days_active: number;
    current_streak: number;
    longest_streak: number;
    total_novels: number;
    total_chapters: number;
    [k: string]: any;
  }
}
