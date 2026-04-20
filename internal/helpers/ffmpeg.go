package helpers

import (
	"fmt"
	"os/exec"
)

const (
	FFmpegPosterWidth  = 1000
	FFmpegPosterHeight = 1500
	FFmpegFanartWidth  = 1920
	FFmpegFanartHeight = 1080
)

func PercentToSeconds(durationInSeconds int64, percent float64) float64 {
	if durationInSeconds <= 0 || percent <= 0 {
		return 0
	}
	return float64(durationInSeconds) * percent / 100
}

func CaptureFrameAsPoster(inputPath, outputPath string, seekSeconds float64) error {
	return captureFrame(inputPath, outputPath, seekSeconds, FFmpegPosterWidth, FFmpegPosterHeight)
}

func CaptureFrameAsFanart(inputPath, outputPath string, seekSeconds float64) error {
	return captureFrame(inputPath, outputPath, seekSeconds, FFmpegFanartWidth, FFmpegFanartHeight)
}

func captureFrame(inputPath, outputPath string, seekSeconds float64, width, height int) error {
	if seekSeconds < 0 {
		seekSeconds = 0
	}
	filter := fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=increase,crop=%d:%d", width, height, width, height)
	cmd := exec.Command(
		"ffmpeg",
		"-hide_banner",
		"-loglevel", "error",
		"-y",
		"-ss", fmt.Sprintf("%.3f", seekSeconds),
		"-i", inputPath,
		"-frames:v", "1",
		"-an",
		"-vf", filter,
		outputPath,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		AppLogger.Errorf("执行 ffmpeg 截帧失败: %v, 输出: %s", err, string(output))
		return fmt.Errorf("执行 ffmpeg 截帧失败: %w", err)
	}
	return nil
}
