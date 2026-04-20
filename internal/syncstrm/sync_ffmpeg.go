package syncstrm

import (
	"Q115-STRM/internal/helpers"
	"Q115-STRM/internal/models"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type ffmpegSnapshotTask struct {
	InputPath string
	OutputDir string
	VideoName string
}

func (s *SyncStrm) AddFFmpegSnapshotTask(sf *SyncFileCache, inputPath string) {
	if s.Config.EnableFFmpegSnapshot != 1 || inputPath == "" {
		return
	}
	outputPath := sf.GetLocalFilePath(s.TargetPath, s.SourcePath)
	task := ffmpegSnapshotTask{
		InputPath: inputPath,
		OutputDir: filepath.Dir(outputPath),
		VideoName: filepath.ToSlash(filepath.Join(sf.Path, sf.FileName)),
	}
	s.ffmpegTaskMu.Lock()
	s.ffmpegTasks = append(s.ffmpegTasks, task)
	s.ffmpegTaskMu.Unlock()
}

func (s *SyncStrm) RunFFmpegSnapshotTasks() {
	if s.Config.EnableFFmpegSnapshot != 1 {
		return
	}
	s.ffmpegTaskMu.Lock()
	tasks := make([]ffmpegSnapshotTask, len(s.ffmpegTasks))
	copy(tasks, s.ffmpegTasks)
	s.ffmpegTasks = nil
	s.ffmpegTaskMu.Unlock()
	if len(tasks) == 0 {
		s.Sync.Logger.Info("没有新增STRM需要补充FFmpeg截图")
		return
	}
	dedupTasks := make([]ffmpegSnapshotTask, 0, len(tasks))
	seen := make(map[string]bool)
	for _, task := range tasks {
		key := filepath.Clean(task.OutputDir)
		if seen[key] {
			continue
		}
		seen[key] = true
		dedupTasks = append(dedupTasks, task)
	}
	s.Sync.Logger.Infof("开始处理FFmpeg截图任务，共 %d 个目录", len(dedupTasks))
	for _, task := range dedupTasks {
		if err := s.runFFmpegSnapshotTask(task); err != nil {
			s.Sync.Logger.Errorf("FFmpeg截图任务失败，目录=%s，视频=%s，错误=%v", task.OutputDir, task.VideoName, err)
		}
	}
}

func (s *SyncStrm) runFFmpegSnapshotTask(task ffmpegSnapshotTask) error {
	if !helpers.PathExists(task.OutputDir) {
		return fmt.Errorf("目标目录不存在: %s", task.OutputDir)
	}
	singleVideoDir, err := s.isSingleStrmDir(task.OutputDir)
	if err != nil {
		return err
	}
	if !singleVideoDir {
		s.Sync.Logger.Infof("目录 %s 下存在多个STRM文件，跳过自动生成 folder/poster/fanart，避免封面互相覆盖", task.OutputDir)
		return nil
	}
	needPoster := !s.hasAnyArtwork(task.OutputDir, []string{"poster.jpg", "folder.jpg"})
	needFanart := !s.hasAnyArtwork(task.OutputDir, []string{"fanart.jpg", "backdrop.jpg", "background.jpg"})
	if !needPoster && !needFanart {
		s.Sync.Logger.Infof("目录 %s 已存在海报和背景图，跳过FFmpeg截图", task.OutputDir)
		return nil
	}
	durationSeconds := int64(0)
	ffprobeJson, ffprobeErr := helpers.GetFFprobeJson(task.InputPath)
	if ffprobeErr != nil {
		s.Sync.Logger.Warnf("FFprobe 获取视频时长失败，回退到首帧截图: %v", ffprobeErr)
	} else if ffprobeJson != nil {
		if seconds, parseErr := helpers.ParseDurationToSeconds(ffprobeJson.Format.Duration); parseErr == nil {
			durationSeconds = seconds
		} else {
			s.Sync.Logger.Warnf("解析视频时长失败，回退到首帧截图: %v", parseErr)
		}
	}
	if needPoster {
		posterSeekSeconds := helpers.PercentToSeconds(durationSeconds, s.Config.FFmpegPosterPercent)
		posterPath := filepath.Join(task.OutputDir, "poster.jpg")
		if err := helpers.CaptureFrameAsPoster(task.InputPath, posterPath, posterSeekSeconds); err != nil {
			return err
		}
		if err := helpers.CopyFile(posterPath, filepath.Join(task.OutputDir, "folder.jpg")); err != nil {
			return err
		}
		s.Sync.Logger.Infof("FFmpeg 已生成海报: %s", posterPath)
		if needFanart {
			s.sleepFFmpegCaptureInterval()
		}
	}
	if needFanart {
		fanartSeekSeconds := helpers.PercentToSeconds(durationSeconds, s.Config.FFmpegFanartPercent)
		fanartPath := filepath.Join(task.OutputDir, "fanart.jpg")
		if err := helpers.CaptureFrameAsFanart(task.InputPath, fanartPath, fanartSeekSeconds); err != nil {
			return err
		}
		if err := helpers.CopyFile(fanartPath, filepath.Join(task.OutputDir, "backdrop.jpg")); err != nil {
			return err
		}
		if err := helpers.CopyFile(fanartPath, filepath.Join(task.OutputDir, "background.jpg")); err != nil {
			return err
		}
		s.Sync.Logger.Infof("FFmpeg 已生成背景图: %s", fanartPath)
	}
	s.sleepFFmpegCaptureInterval()
	return nil
}

func (s *SyncStrm) isSingleStrmDir(dir string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, err
	}
	strmCount := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.EqualFold(filepath.Ext(entry.Name()), ".strm") {
			strmCount++
			if strmCount > 1 {
				return false, nil
			}
		}
	}
	return strmCount == 1, nil
}

func (s *SyncStrm) hasAnyArtwork(dir string, names []string) bool {
	for _, name := range names {
		if helpers.PathExists(filepath.Join(dir, name)) {
			return true
		}
	}
	return false
}

func (s *SyncStrm) sleepFFmpegCaptureInterval() {
	if s.Account == nil || s.Account.SourceType == models.SourceTypeLocal {
		return
	}
	delayMs := s.Config.FFmpegCaptureDelayMs
	if (s.Account.SourceType == models.SourceType115 || s.Account.SourceType == models.SourceTypeBaiduPan) && delayMs < models.DefaultFFmpegCaptureDelayMs {
		delayMs = models.DefaultFFmpegCaptureDelayMs
	}
	if delayMs <= 0 {
		return
	}
	time.Sleep(time.Duration(delayMs) * time.Millisecond)
}
