// audio/notification.go - 完整的音频通知系统

package audio

import (
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/gordonklaus/portaudio"
	"RemoteAudioCLI/utils"
)

// NotificationPlayer 用于播放通知音效
type NotificationPlayer struct {
	device   *DeviceInfo
	config   *utils.Config
	logger   *utils.Logger
	mutex    sync.Mutex
}

// NewNotificationPlayer 创建新的通知播放器
func NewNotificationPlayer(device *DeviceInfo, config *utils.Config, logger *utils.Logger) *NotificationPlayer {
	return &NotificationPlayer{
		device: device,
		config: config,
		logger: logger,
	}
}

// PlayConnectionSound 播放连接提示音
func (np *NotificationPlayer) PlayConnectionSound() {
	np.mutex.Lock()
	defer np.mutex.Unlock()

	np.logger.Info("🔊 Playing connection sound")

	// 查找连接音频文件
	soundPath := np.findSoundFile("connecting")
	if soundPath != "" {
		np.logger.Infof("🎵 Found connection sound: %s", soundPath)
		if err := np.playAudioFile(soundPath); err != nil {
			np.logger.Warnf("Failed to play connection sound: %v, using system beep", err)
			np.playSystemBeep()
		}
	} else {
		np.logger.Warn("Connection sound file not found, using system beep")
		np.playSystemBeep()
	}
}

// PlayDisconnectionSound 播放断开连接提示音
func (np *NotificationPlayer) PlayDisconnectionSound() {
	np.mutex.Lock()
	defer np.mutex.Unlock()

	np.logger.Info("🔈 Playing disconnection sound")
	
	// 查找断开连接音频文件
	soundPath := np.findSoundFile("disconnecting")
	if soundPath != "" {
		np.logger.Infof("🎵 Found disconnection sound: %s", soundPath)
		if err := np.playAudioFile(soundPath); err != nil {
			np.logger.Warnf("Failed to play disconnection sound: %v, using system beep", err)
			np.playDoubleBeep()
		}
	} else {
		np.logger.Warn("Disconnection sound file not found, using system beep")
		np.playDoubleBeep()
	}
}

// findSoundFile 查找音频文件
func (np *NotificationPlayer) findSoundFile(soundType string) string {
	// 可能的音频文件路径和扩展名
	basePaths := []string{
		"sound",
		"sounds",
		"./sound",
		"./sounds",
		"assets",
		"media",
	}
	
	extensions := []string{".mp3", ".wav", ".m4a", ".ogg"}
	
	// 获取可执行文件目录
	execDir, err := os.Executable()
	if err == nil {
		execDir = filepath.Dir(execDir)
		for _, basePath := range basePaths {
			for _, ext := range extensions {
				fullPath := filepath.Join(execDir, basePath, soundType+ext)
				if _, err := os.Stat(fullPath); err == nil {
					return fullPath
				}
			}
		}
	}

	// 检查当前工作目录
	for _, basePath := range basePaths {
		for _, ext := range extensions {
			fullPath := filepath.Join(basePath, soundType+ext)
			if _, err := os.Stat(fullPath); err == nil {
				return fullPath
			}
		}
	}

	return ""
}

// playSystemBeep 播放系统蜂鸣声
func (np *NotificationPlayer) playSystemBeep() {
	// 生成一个简单的蜂鸣声
	np.generateBeepTone(800, 300) // 800Hz, 300ms
}

// playDoubleBeep 播放双声蜂鸣 (用于断开连接)
func (np *NotificationPlayer) playDoubleBeep() {
	// 播放两声短促的蜂鸣声表示断开连接
	np.generateBeepTone(600, 150) // 第一声: 600Hz, 150ms
	time.Sleep(100 * time.Millisecond)
	np.generateBeepTone(400, 150) // 第二声: 400Hz, 150ms (更低音调)
}

// generateBeepTone 生成蜂鸣声音调
func (np *NotificationPlayer) generateBeepTone(frequency float64, durationMs int) {
	// 简化的蜂鸣声生成
	sampleRate := 44100
	duration := time.Duration(durationMs) * time.Millisecond
	samples := int(float64(sampleRate) * duration.Seconds())
	
	// 生成正弦波
	audioData := make([]int16, samples)
	for i := 0; i < samples; i++ {
		t := float64(i) / float64(sampleRate)
		amplitude := 0.3 // 降低音量
		sample := amplitude * 32767 * math.Sin(2*math.Pi*frequency*t)
		audioData[i] = int16(sample)
	}

	// 使用临时播放器播放
	np.playRawAudio(audioData, sampleRate)
}

// playRawAudio 播放原始音频数据
func (np *NotificationPlayer) playRawAudio(audioData []int16, sampleRate int) {
	// 获取 PortAudio 设备
	paDevice, err := GetPortAudioDevice(np.device)
	if err != nil {
		np.logger.Errorf("Failed to get PortAudio device: %v", err)
		return
	}

	// 创建输出参数
	outputParams := portaudio.StreamParameters{
		Output: portaudio.StreamDeviceParameters{
			Device:   paDevice,
			Channels: 1, // 单声道
			Latency:  paDevice.DefaultLowOutputLatency,
		},
		SampleRate:      float64(sampleRate),
		FramesPerBuffer: 512,
	}

	// 创建输出缓冲区
	outputBuffer := make([]int16, 512)

	// 创建流
	stream, err := portaudio.OpenStream(outputParams, outputBuffer)
	if err != nil {
		np.logger.Errorf("Failed to open audio stream: %v", err)
		return
	}
	defer stream.Close()

	// 启动流
	if err := stream.Start(); err != nil {
		np.logger.Errorf("Failed to start audio stream: %v", err)
		return
	}
	defer stream.Stop()

	// 播放音频数据
	for i := 0; i < len(audioData); i += len(outputBuffer) {
		// 清空缓冲区
		for j := range outputBuffer {
			outputBuffer[j] = 0
		}

		// 复制音频数据到缓冲区
		end := i + len(outputBuffer)
		if end > len(audioData) {
			end = len(audioData)
		}

		copy(outputBuffer, audioData[i:end])

		// 写入流
		if err := stream.Write(); err != nil {
			np.logger.Errorf("Failed to write to audio stream: %v", err)
			return
		}
	}
}

// playAudioFile 播放音频文件
func (np *NotificationPlayer) playAudioFile(filePath string) error {
	ext := strings.ToLower(filepath.Ext(filePath))
	
	switch ext {
	case ".mp3":
		return np.playMP3File(filePath)
	case ".wav":
		return np.playWAVFile(filePath)
	case ".m4a", ".ogg":
		return np.playWithSystemPlayer(filePath)
	default:
		return fmt.Errorf("unsupported audio format: %s", ext)
	}
}

// playMP3File 播放 MP3 文件
func (np *NotificationPlayer) playMP3File(filePath string) error {
	// 尝试使用系统播放器播放 MP3
	return np.playWithSystemPlayer(filePath)
}

// playWAVFile 播放 WAV 文件
func (np *NotificationPlayer) playWAVFile(filePath string) error {
	// 尝试使用系统播放器播放 WAV
	return np.playWithSystemPlayer(filePath)
}

// playWithSystemPlayer 使用系统播放器播放音频文件
func (np *NotificationPlayer) playWithSystemPlayer(filePath string) error {
	var cmd *exec.Cmd
	
	switch runtime.GOOS {
	case "windows":
		// Windows: 使用 PowerShell 播放音频
		script := fmt.Sprintf(`
			try {
				if ('%s' -match '\.wav$') {
					$player = New-Object System.Media.SoundPlayer '%s'
					$player.PlaySync()
				} else {
					Add-Type -AssemblyName presentationCore
					$mediaPlayer = New-Object System.Windows.Media.MediaPlayer
					$mediaPlayer.open('%s')
					$mediaPlayer.Play()
					Start-Sleep -Seconds 3
					$mediaPlayer.Stop()
					$mediaPlayer.Close()
				}
			} catch {
				Write-Host "Failed to play audio file"
			}
		`, filePath, filePath, filePath)
		
		cmd = exec.Command("powershell", "-Command", script)
		
	case "darwin":
		// macOS: 使用 afplay
		cmd = exec.Command("afplay", filePath)
		
	case "linux":
		// Linux: 尝试多个播放器
		players := []string{"aplay", "paplay", "mpg123", "ffplay"}
		for _, player := range players {
			if _, err := exec.LookPath(player); err == nil {
				if player == "ffplay" {
					cmd = exec.Command(player, "-nodisp", "-autoexit", filePath)
				} else {
					cmd = exec.Command(player, filePath)
				}
				break
			}
		}
		if cmd == nil {
			return fmt.Errorf("no suitable audio player found on Linux")
		}
		
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
	
	// 异步播放，避免阻塞
	go func() {
		if err := cmd.Run(); err != nil {
			np.logger.Warnf("System player failed: %v", err)
		}
	}()
	
	return nil
}