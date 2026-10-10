/* eslint-disable @typescript-eslint/no-explicit-any */
// 由 scripts/gen_shims.mjs 生成。Wails v3 兼容层：保留 v2 具名方法，
// 类型解析为具体模型、返回去空，实际调用由 v3 官方绑定（app.js）处理。
import * as $app from "@/lib/bindings/github.com/sigpanic/goink/app/app.js";
import type * as Models from "../models";

function toBase64(data: any): string {
  if (typeof data === "string") return data;
  const bytes = data instanceof Uint8Array ? data : new Uint8Array(data ?? []);
  let bin = "";
  for (let i = 0; i < bytes.length; i++) bin += String.fromCharCode(bytes[i]);
  return btoa(bin);
}

export function ApproveTool(toolID: string, approved: boolean, feedback: string): Promise<void> {
  return $app.ApproveTool((toolID as any), (approved as any), (feedback as any)) as any;
}
export function CancelChat(sessionID: string): Promise<void> {
  return $app.CancelChat((sessionID as any)) as any;
}
export function CancelExtract(taskID: string): Promise<void> {
  return $app.CancelExtract((taskID as any)) as any;
}
export function CancelExtractPattern(novelID: number): Promise<void> {
  return $app.CancelExtractPattern((novelID as any)) as any;
}
export function CancelQuit(): Promise<void> {
  return $app.CancelQuit() as any;
}
export function Chat(input: Models.app.ChatInput): Promise<Models.app.ChatResult> {
  return $app.Chat((input as any)) as any;
}
export function CheckUpdate(skipDismiss: boolean): Promise<Models.update.CheckResult> {
  return $app.CheckUpdate((skipDismiss as any)) as any;
}
export function CompressContext(input: Models.app.CompressInput): Promise<Models.app.CompressResult> {
  return $app.CompressContext((input as any)) as any;
}
export function ComputeStyleStats(input: Models.app.ComputeStyleStatsInput): Promise<Models.style.Stats> {
  return $app.ComputeStyleStats((input as any)) as any;
}
export function ConfirmQuit(): Promise<void> {
  return $app.ConfirmQuit() as any;
}
export function CreateArcNode(novelID: number, input: Models.app.CreateArcNodeInput): Promise<Models.storyarc.ArcNode> {
  return $app.CreateArcNode((novelID as any), (input as any)) as any;
}
export function CreateChapter(input: Models.app.CreateChapterInput): Promise<Models.chapter.Chapter> {
  return $app.CreateChapter((input as any)) as any;
}
export function CreateCharacter(novelID: number, input: Models.app.CreateCharacterInput): Promise<Models.character.Character> {
  return $app.CreateCharacter((novelID as any), (input as any)) as any;
}
export function CreateLocation(novelID: number, input: Models.app.CreateLocationInput): Promise<Models.location.Location> {
  return $app.CreateLocation((novelID as any), (input as any)) as any;
}
export function CreateNovel(input: Models.app.CreateNovelInput): Promise<Models.novel.Novel> {
  return $app.CreateNovel((input as any)) as any;
}
export function CreateNovelSetting(novelID: number, input: Models.app.CreateNovelSettingInput): Promise<Models.setting.SettingItem> {
  return $app.CreateNovelSetting((novelID as any), (input as any)) as any;
}
export function CreatePreference(novelID: number, input: Models.app.CreatePreferenceInput): Promise<Models.preference.PreferenceItem> {
  return $app.CreatePreference((novelID as any), (input as any)) as any;
}
export function CreateReaderPerspective(novelID: number, input: Models.app.CreateReaderPerspectiveInput): Promise<Models.reader.ReaderPerspective> {
  return $app.CreateReaderPerspective((novelID as any), (input as any)) as any;
}
export function CreateStoryArc(novelID: number, input: Models.app.CreateStoryArcInput): Promise<Models.storyarc.StoryArc> {
  return $app.CreateStoryArc((novelID as any), (input as any)) as any;
}
export function CreateStyleSample(input: Models.app.CreateStyleSampleInput): Promise<Models.style.Sample> {
  return $app.CreateStyleSample((input as any)) as any;
}
export function CreateTimelineEntry(novelID: number, input: Models.app.CreateTimelineEntryInput): Promise<Models.timeline.TimelineEntry> {
  return $app.CreateTimelineEntry((novelID as any), (input as any)) as any;
}
export function DeleteArcNode(novelID: number, nodeID: number): Promise<void> {
  return $app.DeleteArcNode((novelID as any), (nodeID as any)) as any;
}
export function DeleteChapter(novelID: number, chapterID: number): Promise<Models.chapter.DeleteResult> {
  return $app.DeleteChapter((novelID as any), (chapterID as any)) as any;
}
export function DeleteCharacter(novelID: number, charID: number): Promise<void> {
  return $app.DeleteCharacter((novelID as any), (charID as any)) as any;
}
export function DeleteCover(novelID: number): Promise<void> {
  return $app.DeleteCover((novelID as any)) as any;
}
export function DeleteLocation(novelID: number, locID: number): Promise<void> {
  return $app.DeleteLocation((novelID as any), (locID as any)) as any;
}
export function DeleteNovel(novelID: number): Promise<void> {
  return $app.DeleteNovel((novelID as any)) as any;
}
export function DeleteNovelSetting(id: number): Promise<void> {
  return $app.DeleteNovelSetting((id as any)) as any;
}
export function DeletePreference(id: number): Promise<void> {
  return $app.DeletePreference((id as any)) as any;
}
export function DeleteReaderPerspective(id: number, novelID: number): Promise<void> {
  return $app.DeleteReaderPerspective((id as any), (novelID as any)) as any;
}
export function DeleteSession(sessionID: string): Promise<void> {
  return $app.DeleteSession((sessionID as any)) as any;
}
export function DeleteSkill(input: Models.app.DeleteSkillInput): Promise<void> {
  return $app.DeleteSkill((input as any)) as any;
}
export function DeleteStoryArc(novelID: number, arcID: number): Promise<void> {
  return $app.DeleteStoryArc((novelID as any), (arcID as any)) as any;
}
export function DeleteStyleSample(input: Models.app.DeleteStyleSampleInput): Promise<void> {
  return $app.DeleteStyleSample((input as any)) as any;
}
export function DeleteTimelineEntry(novelID: number, entryID: number): Promise<void> {
  return $app.DeleteTimelineEntry((novelID as any), (entryID as any)) as any;
}
export function DeleteVolume(novelID: number, volumeID: number): Promise<void> {
  return $app.DeleteVolume((novelID as any), (volumeID as any)) as any;
}
export function DiscoverModels(providerName: string, chatURL: string, apiKey: string): Promise<Models.llm.ModelInfo[]> {
  return $app.DiscoverModels((providerName as any), (chatURL as any), (apiKey as any)) as any;
}
export function DismissUpdate(tagName: string): Promise<void> {
  return $app.DismissUpdate((tagName as any)) as any;
}
export function ExportNovel(novelID: number, format: string): Promise<void> {
  return $app.ExportNovel((novelID as any), (format as any)) as any;
}
export function ExtractPattern(input: Models.app.ExtractPatternInput): Promise<Models.app.ExtractPatternResult> {
  return $app.ExtractPattern((input as any)) as any;
}
export function ExtractStyle(input: Models.app.ExtractStyleInput): Promise<Models.style.ExtractResult> {
  return $app.ExtractStyle((input as any)) as any;
}
export function FrontendReady(): Promise<Models.app.StartupState> {
  return $app.FrontendReady() as any;
}
export function GetAppConfig(): Promise<{ [_ in string]?: any }> {
  return $app.GetAppConfig() as any;
}
export function GetArcNodes(novelID: number): Promise<Models.storyarc.ArcNode[]> {
  return $app.GetArcNodes((novelID as any)) as any;
}
export function GetChapterPlans(novelID: number): Promise<Models.timeline.ChapterPlan[]> {
  return $app.GetChapterPlans((novelID as any)) as any;
}
export function GetChapters(novelID: number): Promise<Models.chapter.Chapter[]> {
  return $app.GetChapters((novelID as any)) as any;
}
export function GetCharacterRelations(novelID: number): Promise<Models.character.CharacterRelation[]> {
  return $app.GetCharacterRelations((novelID as any)) as any;
}
export function GetCharacters(novelID: number): Promise<Models.character.Character[]> {
  return $app.GetCharacters((novelID as any)) as any;
}
export function GetCommitFileList(novelID: number, hash: string): Promise<Models.app.CommitFileListResult> {
  return $app.GetCommitFileList((novelID as any), (hash as any)) as any;
}
export function GetCommitLog(novelID: number, n: number, afterHash: string): Promise<Models.git.CommitInfo[]> {
  return $app.GetCommitLog((novelID as any), (n as any), (afterHash as any)) as any;
}
export function GetContent(novelID: number, path: string): Promise<string> {
  return $app.GetContent((novelID as any), (path as any)) as any;
}
export function GetFileDiff(novelID: number, hash: string, filePath: string): Promise<Models.git.FileDiff> {
  return $app.GetFileDiff((novelID as any), (hash as any), (filePath as any)) as any;
}
export function GetLLMConfig(): Promise<Models.llm.LLMConfigView> {
  return $app.GetLLMConfig() as any;
}
export function GetLocationRelations(novelID: number): Promise<Models.location.LocationRelation[]> {
  return $app.GetLocationRelations((novelID as any)) as any;
}
export function GetLocations(novelID: number): Promise<Models.location.Location[]> {
  return $app.GetLocations((novelID as any)) as any;
}
export function GetMaxChapterNumber(novelID: number): Promise<number> {
  return $app.GetMaxChapterNumber((novelID as any)) as any;
}
export function GetModels(): Promise<Models.llm.AvailableModel[]> {
  return $app.GetModels() as any;
}
export function GetNovelSettings(novelID: number): Promise<Models.app.SettingResult> {
  return $app.GetNovelSettings((novelID as any)) as any;
}
export function GetNovels(): Promise<Models.novel.Novel[]> {
  return $app.GetNovels() as any;
}
export function GetPlatform(): Promise<{ [_ in string]?: any }> {
  return $app.GetPlatform() as any;
}
export function GetPreferences(novelID: number): Promise<Models.app.PreferenceResult> {
  return $app.GetPreferences((novelID as any)) as any;
}
export function GetReaderPerspectives(novelID: number): Promise<Models.reader.ReaderPerspective[]> {
  return $app.GetReaderPerspectives((novelID as any)) as any;
}
export function GetRemoteSkillContent(name: string): Promise<Models.apperr.Result<string>> {
  return $app.GetRemoteSkillContent((name as any)) as any;
}
export function GetSession(sessionID: string): Promise<Models.app.SessionDetail> {
  return $app.GetSession((sessionID as any)) as any;
}
export function GetSessionMessages(sessionID: string): Promise<Models.session.Message[]> {
  return $app.GetSessionMessages((sessionID as any)) as any;
}
export function GetSessions(input: Models.app.GetSessionsInput): Promise<Models.storage.PageResult<Models.app.SessionMeta>> {
  return $app.GetSessions((input as any)) as any;
}
export function GetSettings(): Promise<Models.config.AppSettings> {
  return $app.GetSettings() as any;
}
export function GetStartupState(): Promise<Models.app.StartupState> {
  return $app.GetStartupState() as any;
}
export function GetStoryArcs(novelID: number): Promise<Models.storyarc.StoryArc[]> {
  return $app.GetStoryArcs((novelID as any)) as any;
}
export function GetStyleSample(id: number): Promise<Models.style.Sample> {
  return $app.GetStyleSample((id as any)) as any;
}
export function GetTimelineEntries(novelID: number): Promise<Models.timeline.TimelineEntry[]> {
  return $app.GetTimelineEntries((novelID as any)) as any;
}
export function GetVersion(): Promise<string> {
  return $app.GetVersion() as any;
}
export function GetVolumes(novelID: number): Promise<Models.volume.Volume[]> {
  return $app.GetVolumes((novelID as any)) as any;
}
export function GetWritingActivity(months: number): Promise<Models.writing.DailyActivity[]> {
  return $app.GetWritingActivity((months as any)) as any;
}
export function GetWritingStats(): Promise<Models.writing.WritingStats> {
  return $app.GetWritingStats() as any;
}
export function ImportNovel(input: Models.app.ImportNovelInput): Promise<Models.app.ImportNovelResult> {
  return $app.ImportNovel((input as any)) as any;
}
export function ImportWithLLM(input: Models.app.ImportWithLLMInput): Promise<Models.app.ImportNovelResult> {
  return $app.ImportWithLLM((input as any)) as any;
}
export function Initialize(dataDir: string): Promise<void> {
  return $app.Initialize((dataDir as any)) as any;
}
export function InstallRemoteSkill(input: Models.app.InstallRemoteSkillInput): Promise<Models.apperr.Result<Models.apperr.Empty>> {
  return $app.InstallRemoteSkill((input as any)) as any;
}
export function IsInitialized(): Promise<boolean> {
  return $app.IsInitialized() as any;
}
export function ListRemoteSkills(input: Models.app.ListRemoteSkillsInput): Promise<Models.apperr.Result<Models.storage.PageResult<Models.remote.RemoteSkillMeta> | null>> {
  return $app.ListRemoteSkills((input as any)) as any;
}
export function ListSkills(input: Models.app.ListSkillsInput): Promise<Models.skill.SkillMeta[]> {
  return $app.ListSkills((input as any)) as any;
}
export function ListSlashCommands(input: Models.app.ListSlashCommandsInput): Promise<Models.app.SlashCommand[]> {
  return $app.ListSlashCommands((input as any)) as any;
}
export function ListStyleSamples(input: Models.app.ListStyleSamplesInput): Promise<Models.storage.PageResult<Models.style.Sample>> {
  return $app.ListStyleSamples((input as any)) as any;
}
export function OnWindowClosing(): Promise<boolean> {
  return $app.OnWindowClosing() as any;
}
export function PickAndImportNovel(): Promise<Models.app.ImportNovelResult> {
  return $app.PickAndImportNovel() as any;
}
export function PlaceChapter(input: Models.app.PlaceChapterInput): Promise<Models.chapter.Chapter> {
  return $app.PlaceChapter((input as any)) as any;
}
export function PlaceVolume(input: Models.app.PlaceVolumeInput): Promise<Models.volume.Volume> {
  return $app.PlaceVolume((input as any)) as any;
}
export function RebuildNovelIndex(novelID: number): Promise<void> {
  return $app.RebuildNovelIndex((novelID as any)) as any;
}
export function RetryStartup(): Promise<void> {
  return $app.RetryStartup() as any;
}
export function SaveAvatar(data: number[]): Promise<void> {
  return $app.SaveAvatar(toBase64(data) as any) as any;
}
export function SaveContent(input: Models.app.SaveContentInput): Promise<void> {
  return $app.SaveContent((input as any)) as any;
}
export function SaveCover(novelID: number, data: number[]): Promise<void> {
  return $app.SaveCover((novelID as any), toBase64(data) as any) as any;
}
export function SaveGitConfig(name: string, email: string): Promise<void> {
  return $app.SaveGitConfig((name as any), (email as any)) as any;
}
export function SaveLLMConfig(input: Models.llm.LLMConfigView): Promise<void> {
  return $app.SaveLLMConfig((input as any)) as any;
}
export function SaveSettings(input: Models.app.SaveSettingsInput): Promise<void> {
  return $app.SaveSettings((input as any)) as any;
}
export function SaveUserName(name: string): Promise<void> {
  return $app.SaveUserName((name as any)) as any;
}
export function SearchAll(novelID: number, query: string): Promise<Models.search.Result[]> {
  return $app.SearchAll((novelID as any), (query as any)) as any;
}
export function SetActiveNovel(input: Models.app.SetActiveNovelInput): Promise<void> {
  return $app.SetActiveNovel((input as any)) as any;
}
export function SetApprovalMode(mode: string): Promise<void> {
  return $app.SetApprovalMode((mode as any)) as any;
}
export function SetLastSession(sessionID: string): Promise<void> {
  return $app.SetLastSession((sessionID as any)) as any;
}
export function SetReasoningEffort(effort: string): Promise<void> {
  return $app.SetReasoningEffort((effort as any)) as any;
}
export function SetSelectedModel(key: string, effort: string): Promise<void> {
  return $app.SetSelectedModel((key as any), (effort as any)) as any;
}
export function TestConnection(input: Models.app.TestConnectionInput): Promise<Models.llm.TestConnectionResult> {
  return $app.TestConnection((input as any)) as any;
}
export function UpdateArcNode(novelID: number, nodeID: number, input: Models.app.UpdateArcNodeInput): Promise<void> {
  return $app.UpdateArcNode((novelID as any), (nodeID as any), (input as any)) as any;
}
export function UpdateChapterPlan(novelID: number, input: Models.app.UpdateChapterPlanInput): Promise<void> {
  return $app.UpdateChapterPlan((novelID as any), (input as any)) as any;
}
export function UpdateChapterTitle(novelID: number, chapterID: number, title: string): Promise<void> {
  return $app.UpdateChapterTitle((novelID as any), (chapterID as any), (title as any)) as any;
}
export function UpdateCharacter(novelID: number, charID: number, input: Models.app.UpdateCharacterInput): Promise<void> {
  return $app.UpdateCharacter((novelID as any), (charID as any), (input as any)) as any;
}
export function UpdateDataDir(newPath: string): Promise<void> {
  return $app.UpdateDataDir((newPath as any)) as any;
}
export function UpdateLocation(novelID: number, locID: number, input: Models.app.UpdateLocationInput): Promise<void> {
  return $app.UpdateLocation((novelID as any), (locID as any), (input as any)) as any;
}
export function UpdateNovel(novelID: number, input: Models.app.UpdateNovelInput): Promise<Models.novel.Novel> {
  return $app.UpdateNovel((novelID as any), (input as any)) as any;
}
export function UpdateNovelSetting(novelID: number, id: number, input: Models.app.UpdateNovelSettingInput): Promise<Models.setting.SettingItem> {
  return $app.UpdateNovelSetting((novelID as any), (id as any), (input as any)) as any;
}
export function UpdatePreference(novelID: number, id: number, input: Models.app.UpdatePreferenceInput): Promise<Models.preference.PreferenceItem> {
  return $app.UpdatePreference((novelID as any), (id as any), (input as any)) as any;
}
export function UpdateReaderPerspective(id: number, novelID: number, input: Models.app.UpdateReaderPerspectiveInput): Promise<void> {
  return $app.UpdateReaderPerspective((id as any), (novelID as any), (input as any)) as any;
}
export function UpdateStoryArc(novelID: number, arcID: number, input: Models.app.UpdateStoryArcInput): Promise<void> {
  return $app.UpdateStoryArc((novelID as any), (arcID as any), (input as any)) as any;
}
export function UpdateStyleSample(input: Models.app.UpdateStyleSampleInput): Promise<Models.style.Sample> {
  return $app.UpdateStyleSample((input as any)) as any;
}
export function UpdateTimelineEntry(novelID: number, entryID: number, input: Models.app.UpdateTimelineEntryInput): Promise<void> {
  return $app.UpdateTimelineEntry((novelID as any), (entryID as any), (input as any)) as any;
}
export function UpdateVolume(novelID: number, volumeID: number, name: string): Promise<void> {
  return $app.UpdateVolume((novelID as any), (volumeID as any), (name as any)) as any;
}
