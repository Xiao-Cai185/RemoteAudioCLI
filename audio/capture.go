// audio/capture.go - 添加分贝计算的版本

package audio

import (
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gordonklaus/portaudio"
	"RemoteAudioCLI/utils"
)

// AudioDataCallback defines the callback function for audio data
type AudioDataCallback func(audioData []byte)

// Capturer handles audio input capture
type Capturer struct {
	device   *DeviceInfo
	config   *utils.Config
	logger   *utils.Logger
	stream   *portaudio.Stream
	callback AudioDataCallback
	
	// 添加输入缓冲区引用
	inputBuffer interface{}
	
	// State management
	running      int32 // atomic bool
	initialized  int32 // atomic bool
	
	// Statistics
	stats *utils.AudioStats
	
	// 分贝计算相关
	decibelMutex sync.RWMutex
	currentDB    float64
	
	// Control
	stopChan chan struct{}
	wg       sync.WaitGroup
}

// NewCapturer creates a new audio capturer
func NewCapturer(device *DeviceInfo, config *utils.Config, logger *utils.Logger) *Capturer {
	return &Capturer{
		device:   device,
		config:   config,
		logger:   logger,
		stopChan: make(chan struct{}),
		currentDB: -60.0, // 默认静音级别
		stats: &utils.AudioStats{
			FramesProcessed: 0,
			DroppedFrames:   0,
			Latency:         0,
			BufferUsage:     0,
			DecibelLevel:    -60.0,
		},
	}
}

// calculateDecibels 计算音频数据的分贝级别
func (c *Capturer) calculateDecibels(audioData []byte) float64 {
	if len(audioData) == 0 {
		return -60.0 // 静音
	}
	
	var sum float64 = 0
	var sampleCount int = 0
	
	switch c.config.BitDepth {
	case 16:
		for i := 0; i < len(audioData)-1; i += 2 {
			// 转换为 int16
			sample := int16(audioData[i]) | (int16(audioData[i+1]) << 8)
			// 转换为 -1.0 到 1.0 的浮点数
			normalizedSample := float64(sample) / 32768.0
			sum += normalizedSample * normalizedSample
			sampleCount++
		}
	case 32:
		for i := 0; i < len(audioData)-3; i += 4 {
			// 转换为 int32
			sample := int32(audioData[i]) |
				(int32(audioData[i+1]) << 8) |
				(int32(audioData[i+2]) << 16) |
				(int32(audioData[i+3]) << 24)
			// 转换为 -1.0 到 1.0 的浮点数
			normalizedSample := float64(sample) / 2147483648.0
			sum += normalizedSample * normalizedSample
			sampleCount++
		}
	default:
		return -60.0
	}
	
	if sampleCount == 0 {
		return -60.0
	}
	
	// 计算 RMS (Root Mean Square)
	rms := math.Sqrt(sum / float64(sampleCount))
	
	// 避免 log(0)
	if rms < 1e-10 {
		return -60.0
	}
	
	// 转换为分贝 (20 * log10(rms))
	db := 20 * math.Log10(rms)
	
	// 限制范围 (-60dB 到 0dB)
	if db < -60.0 {
		db = -60.0
	} else if db > 0.0 {
		db = 0.0
	}
	
	return db
}

// updateDecibelLevel 更新当前分贝级别（带平滑处理）
func (c *Capturer) updateDecibelLevel(newDB float64) {
	c.decibelMutex.Lock()
	defer c.decibelMutex.Unlock()
	
	// 简单的指数平滑
	const smoothing = 0.3
	c.currentDB = c.currentDB*(1-smoothing) + newDB*smoothing
	c.stats.DecibelLevel = c.currentDB
}

// getCurrentDecibelLevel 获取当前分贝级别
func (c *Capturer) getCurrentDecibelLevel() float64 {
	c.decibelMutex.RLock()
	defer c.decibelMutex.RUnlock()
	return c.currentDB
}

// Initialize initializes the audio capturer
func (c *Capturer) Initialize() error {
	if atomic.LoadInt32(&c.initialized) == 1 {
		return nil
	}

	c.logger.Infof("Initializing audio capturer for device: %s", c.device.Name)

	// Validate device for input
	if err := ValidateDeviceForInput(c.device, c.config.SampleRate, c.config.Channels); err != nil {
		return utils.WrapError(err, utils.ErrAudioCapture, "device validation failed")
	}

	// Get PortAudio device
	paDevice, err := GetPortAudioDevice(c.device)
	if err != nil {
		return utils.WrapError(err, utils.ErrAudioCapture, "failed to get PortAudio device")
	}

	// Create input buffer based on bit depth
	switch c.config.BitDepth {
	case 16:
		c.inputBuffer = make([]int16, c.config.FramesPerBuffer*c.config.Channels)
	case 32:
		c.inputBuffer = make([]int32, c.config.FramesPerBuffer*c.config.Channels)
	default:
		return utils.NewAppError(utils.ErrAudioCapture, 
			fmt.Sprintf("unsupported bit depth: %d", c.config.BitDepth))
	}

	// Create stream parameters
	inputParams := portaudio.StreamParameters{
		Input: portaudio.StreamDeviceParameters{
			Device:   paDevice,
			Channels: c.config.Channels,
			Latency:  paDevice.DefaultLowInputLatency,
		},
		SampleRate:      float64(c.config.SampleRate),
		FramesPerBuffer: c.config.FramesPerBuffer,
	}

	// Create the stream
	stream, err := portaudio.OpenStream(inputParams, c.inputBuffer)
	if err != nil {
		return utils.WrapError(err, utils.ErrAudioCapture, "failed to open audio stream")
	}

	c.stream = stream
	atomic.StoreInt32(&c.initialized, 1)

	c.logger.Infof("Audio capturer initialized - Sample Rate: %dHz, Channels: %d, Bit Depth: %d, Buffer: %d frames",
		c.config.SampleRate, c.config.Channels, c.config.BitDepth, c.config.FramesPerBuffer)

	return nil
}

// Start begins audio capture
func (c *Capturer) Start(callback AudioDataCallback) error {
	if atomic.LoadInt32(&c.initialized) == 0 {
		return utils.NewAppError(utils.ErrAudioCapture, "capturer not initialized")
	}

	if atomic.LoadInt32(&c.running) == 1 {
		return utils.NewAppError(utils.ErrAudioCapture, "capturer already running")
	}

	if callback == nil {
		return utils.NewAppError(utils.ErrAudioCapture, "callback function is required")
	}

	c.callback = callback

	// Start the PortAudio stream
	if err := c.stream.Start(); err != nil {
		return utils.WrapError(err, utils.ErrAudioCapture, "failed to start audio stream")
	}

	atomic.StoreInt32(&c.running, 1)

	// Start capture loop
	c.wg.Add(1)
	go c.captureLoop()

	c.logger.Info("🎤 Audio capture started")
	return nil
}

// Stop stops audio capture
func (c *Capturer) Stop() {
	if atomic.LoadInt32(&c.running) == 0 {
		return
	}

	c.logger.Info("⏹️ Stopping audio capture...")
	atomic.StoreInt32(&c.running, 0)

	// Signal stop
	close(c.stopChan)

	// Stop the stream
	if c.stream != nil {
		c.stream.Stop()
	}

	// Wait for capture loop to finish
	c.wg.Wait()

	c.logger.Info("✅ Audio capture stopped")
}

// Terminate terminates the capturer and releases resources
func (c *Capturer) Terminate() {
	if atomic.LoadInt32(&c.initialized) == 0 {
		return
	}

	// Stop if running
	c.Stop()

	// Close the stream
	if c.stream != nil {
		c.stream.Close()
		c.stream = nil
	}

	atomic.StoreInt32(&c.initialized, 0)
	c.logger.Info("🔚 Audio capturer terminated")
}

// captureLoop is the main capture loop
func (c *Capturer) captureLoop() {
	defer c.wg.Done()

	c.logger.Debug("Audio capture loop started")

	// Create buffer for audio data
	frameSize := c.config.GetFrameSize()
	audioBuffer := make([]byte, c.config.FramesPerBuffer*frameSize)

	for atomic.LoadInt32(&c.running) == 1 {
		startTime := time.Now()

		// Read audio data from stream
		err := c.stream.Read()
		if err != nil {
			c.logger.Error(fmt.Sprintf("Failed to read from audio stream: %v", err))
			atomic.AddInt64(&c.stats.DroppedFrames, int64(c.config.FramesPerBuffer))
			
			// Check if this is a critical error
			if err == portaudio.InputOverflowed {
				c.logger.Warn("Input buffer overflow detected")
			} else {
				// For other errors, we might want to stop
				break
			}
			continue
		}

		// Convert audio data to bytes
		if err := c.convertAudioData(audioBuffer); err != nil {
			c.logger.Error(fmt.Sprintf("Failed to convert audio data: %v", err))
			atomic.AddInt64(&c.stats.DroppedFrames, int64(c.config.FramesPerBuffer))
			continue
		}

		// 计算分贝级别
		decibelLevel := c.calculateDecibels(audioBuffer)
		c.updateDecibelLevel(decibelLevel)

		// Call the callback with audio data
		if c.callback != nil {
			c.callback(audioBuffer)
		}

		// Update statistics
		atomic.AddInt64(&c.stats.FramesProcessed, int64(c.config.FramesPerBuffer))
		
		// Calculate processing latency
		processingTime := time.Since(startTime)
		c.stats.Latency = processingTime
	}

	c.logger.Debug("Audio capture loop ended")
}

// convertAudioData converts the captured audio data to bytes
func (c *Capturer) convertAudioData(output []byte) error {
	if c.inputBuffer == nil {
		return utils.NewAppError(utils.ErrAudioCapture, "input buffer is nil")
	}

	switch c.config.BitDepth {
	case 16:
		// 修复：使用保存的输入缓冲区引用
		input, ok := c.inputBuffer.([]int16)
		if !ok {
			return utils.NewAppError(utils.ErrAudioCapture, "invalid input buffer type for 16-bit")
		}
		
		for i, sample := range input {
			if i*2+1 >= len(output) {
				break
			}
			// Little-endian conversion
			output[i*2] = byte(sample & 0xFF)
			output[i*2+1] = byte((sample >> 8) & 0xFF)
		}

	case 32:
		// 修复：使用保存的输入缓冲区引用
		input, ok := c.inputBuffer.([]int32)
		if !ok {
			return utils.NewAppError(utils.ErrAudioCapture, "invalid input buffer type for 32-bit")
		}
		
		for i, sample := range input {
			if i*4+3 >= len(output) {
				break
			}
			// Little-endian conversion
			output[i*4] = byte(sample & 0xFF)
			output[i*4+1] = byte((sample >> 8) & 0xFF)
			output[i*4+2] = byte((sample >> 16) & 0xFF)
			output[i*4+3] = byte((sample >> 24) & 0xFF)
		}

	default:
		return utils.NewAppError(utils.ErrAudioCapture, 
			fmt.Sprintf("unsupported bit depth: %d", c.config.BitDepth))
	}

	return nil
}

// IsRunning returns whether the capturer is currently running
func (c *Capturer) IsRunning() bool {
	return atomic.LoadInt32(&c.running) == 1
}

// IsInitialized returns whether the capturer is initialized
func (c *Capturer) IsInitialized() bool {
	return atomic.LoadInt32(&c.initialized) == 1
}

// GetStats returns current capture statistics
func (c *Capturer) GetStats() *utils.AudioStats {
	bufferUsage := c.calculateBufferUsage()
	// 确保缓冲区使用率在0-1范围内
	if bufferUsage > 1.0 {
		bufferUsage = 1.0
	} else if bufferUsage < 0.0 {
		bufferUsage = 0.0
	}
	
	return &utils.AudioStats{
		FramesProcessed: atomic.LoadInt64(&c.stats.FramesProcessed),
		DroppedFrames:   atomic.LoadInt64(&c.stats.DroppedFrames),
		Latency:         c.stats.Latency,
		BufferUsage:     bufferUsage,
		DecibelLevel:    c.getCurrentDecibelLevel(),
	}
}

// calculateBufferUsage calculates current buffer usage
func (c *Capturer) calculateBufferUsage() float64 {
	if c.stream == nil {
		return 0.0
	}

	// 返回一个简化的缓冲区使用率 (0.0 到 1.0)
	// 在实际实现中，你可能需要更精确的跟踪
	info := c.stream.Info()
	if info != nil {
		// 将延迟转换为合理的使用率百分比 (0-1之间)
		// 假设100ms为满缓冲，将 time.Duration 转换为秒数再除以 0.1
		latencySeconds := info.InputLatency.Seconds()
		latencyRatio := latencySeconds / 0.1 // 假设100ms为满缓冲
		if latencyRatio > 1.0 {
			latencyRatio = 1.0
		}
		return latencyRatio
	}

	return 0.0
}