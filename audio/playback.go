// audio/playback.go - 添加分贝计算的版本

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

// AudioBuffer represents a circular buffer for audio data
type AudioBuffer struct {
	data     [][]byte
	readPos  int
	writePos int
	size     int
	mutex    sync.RWMutex
	full     bool
}

// NewAudioBuffer creates a new audio buffer
func NewAudioBuffer(size int) *AudioBuffer {
	return &AudioBuffer{
		data: make([][]byte, size),
		size: size,
	}
}

// Write writes audio data to the buffer
func (ab *AudioBuffer) Write(data []byte) bool {
	ab.mutex.Lock()
	defer ab.mutex.Unlock()

	// Check if buffer is full
	nextWritePos := (ab.writePos + 1) % ab.size
	if nextWritePos == ab.readPos && ab.full {
		return false // Buffer is full
	}

	// Copy data
	ab.data[ab.writePos] = make([]byte, len(data))
	copy(ab.data[ab.writePos], data)

	ab.writePos = nextWritePos
	if ab.writePos == ab.readPos {
		ab.full = true
	}

	return true
}

// Read reads audio data from the buffer
func (ab *AudioBuffer) Read() ([]byte, bool) {
	ab.mutex.Lock()
	defer ab.mutex.Unlock()

	// Check if buffer is empty
	if ab.readPos == ab.writePos && !ab.full {
		return nil, false
	}

	data := ab.data[ab.readPos]
	ab.readPos = (ab.readPos + 1) % ab.size
	ab.full = false

	return data, true
}

// Usage returns the current buffer usage as a percentage
func (ab *AudioBuffer) Usage() float64 {
	ab.mutex.RLock()
	defer ab.mutex.RUnlock()

	if ab.full {
		return 1.0
	}

	var used int
	if ab.writePos >= ab.readPos {
		used = ab.writePos - ab.readPos
	} else {
		used = ab.size - ab.readPos + ab.writePos
	}

	return float64(used) / float64(ab.size)
}

// Clear clears the buffer
func (ab *AudioBuffer) Clear() {
	ab.mutex.Lock()
	defer ab.mutex.Unlock()

	ab.readPos = 0
	ab.writePos = 0
	ab.full = false
}

// Player handles audio output playback
type Player struct {
	device   *DeviceInfo
	config   *utils.Config
	logger   *utils.Logger
	stream   *portaudio.Stream
	buffer   *AudioBuffer
	
	// 添加输出缓冲区引用
	outputBuffer interface{}
	
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

// NewPlayer creates a new audio player
func NewPlayer(device *DeviceInfo, config *utils.Config, logger *utils.Logger) *Player {
	return &Player{
		device:   device,
		config:   config,
		logger:   logger,
		buffer:   NewAudioBuffer(config.BufferCount * 2), // Extra buffers for safety
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
func (p *Player) calculateDecibels(audioData []byte) float64 {
	if len(audioData) == 0 {
		return -60.0 // 静音
	}
	
	var sum float64 = 0
	var sampleCount int = 0
	
	switch p.config.BitDepth {
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
func (p *Player) updateDecibelLevel(newDB float64) {
	p.decibelMutex.Lock()
	defer p.decibelMutex.Unlock()
	
	// 简单的指数平滑
	const smoothing = 0.3
	p.currentDB = p.currentDB*(1-smoothing) + newDB*smoothing
	p.stats.DecibelLevel = p.currentDB
}

// getCurrentDecibelLevel 获取当前分贝级别
func (p *Player) getCurrentDecibelLevel() float64 {
	p.decibelMutex.RLock()
	defer p.decibelMutex.RUnlock()
	return p.currentDB
}

// Initialize initializes the audio player
func (p *Player) Initialize() error {
	if atomic.LoadInt32(&p.initialized) == 1 {
		return nil
	}

	p.logger.Infof("Initializing audio player for device: %s", p.device.Name)

	// Validate device for output
	if err := ValidateDeviceForOutput(p.device, p.config.SampleRate, p.config.Channels); err != nil {
		return utils.WrapError(err, utils.ErrAudioPlayback, "device validation failed")
	}

	// Get PortAudio device
	paDevice, err := GetPortAudioDevice(p.device)
	if err != nil {
		return utils.WrapError(err, utils.ErrAudioPlayback, "failed to get PortAudio device")
	}

	// Create output buffer based on bit depth
	switch p.config.BitDepth {
	case 16:
		p.outputBuffer = make([]int16, p.config.FramesPerBuffer*p.config.Channels)
	case 32:
		p.outputBuffer = make([]int32, p.config.FramesPerBuffer*p.config.Channels)
	default:
		return utils.NewAppError(utils.ErrAudioPlayback, 
			fmt.Sprintf("unsupported bit depth: %d", p.config.BitDepth))
	}

	// Create stream parameters
	outputParams := portaudio.StreamParameters{
		Output: portaudio.StreamDeviceParameters{
			Device:   paDevice,
			Channels: p.config.Channels,
			Latency:  paDevice.DefaultLowOutputLatency,
		},
		SampleRate:      float64(p.config.SampleRate),
		FramesPerBuffer: p.config.FramesPerBuffer,
	}

	// Create the stream
	stream, err := portaudio.OpenStream(outputParams, p.outputBuffer)
	if err != nil {
		return utils.WrapError(err, utils.ErrAudioPlayback, "failed to open audio stream")
	}

	p.stream = stream
	atomic.StoreInt32(&p.initialized, 1)

	p.logger.Infof("Audio player initialized - Sample Rate: %dHz, Channels: %d, Bit Depth: %d, Buffer: %d frames",
		p.config.SampleRate, p.config.Channels, p.config.BitDepth, p.config.FramesPerBuffer)

	return nil
}

// Start begins audio playback
func (p *Player) Start() error {
	if atomic.LoadInt32(&p.initialized) == 0 {
		return utils.NewAppError(utils.ErrAudioPlayback, "player not initialized")
	}

	if atomic.LoadInt32(&p.running) == 1 {
		return utils.NewAppError(utils.ErrAudioPlayback, "player already running")
	}

	// Start the PortAudio stream
	if err := p.stream.Start(); err != nil {
		return utils.WrapError(err, utils.ErrAudioPlayback, "failed to start audio stream")
	}

	atomic.StoreInt32(&p.running, 1)

	// Start playback loop
	p.wg.Add(1)
	go p.playbackLoop()

	p.logger.Info("🔊 Audio playback started")
	return nil
}

// Stop stops audio playback
func (p *Player) Stop() {
	if atomic.LoadInt32(&p.running) == 0 {
		return
	}

	p.logger.Info("⏹️ Stopping audio playback...")
	atomic.StoreInt32(&p.running, 0)

	// Signal stop
	close(p.stopChan)

	// Stop the stream
	if p.stream != nil {
		p.stream.Stop()
	}

	// Wait for playback loop to finish
	p.wg.Wait()

	// Clear buffer
	p.buffer.Clear()

	p.logger.Info("✅ Audio playback stopped")
}

// Terminate terminates the player and releases resources
func (p *Player) Terminate() {
	if atomic.LoadInt32(&p.initialized) == 0 {
		return
	}

	// Stop if running
	p.Stop()

	// Close the stream
	if p.stream != nil {
		p.stream.Close()
		p.stream = nil
	}

	atomic.StoreInt32(&p.initialized, 0)
	p.logger.Info("🔚 Audio player terminated")
}

// QueueAudio queues audio data for playback
func (p *Player) QueueAudio(audioData []byte) error {
	if atomic.LoadInt32(&p.initialized) == 0 {
		return utils.NewAppError(utils.ErrAudioPlayback, "player not initialized")
	}

	// Try to write to buffer
	if !p.buffer.Write(audioData) {
		atomic.AddInt64(&p.stats.DroppedFrames, int64(p.config.FramesPerBuffer))
		return utils.NewAppError(utils.ErrBuffer, "audio buffer is full")
	}

	return nil
}

// playbackLoop is the main playback loop
func (p *Player) playbackLoop() {
	defer p.wg.Done()

	p.logger.Debug("Audio playback loop started")

	// Create silence buffer for when no data is available
	frameSize := p.config.GetFrameSize()
	silenceBuffer := make([]byte, p.config.FramesPerBuffer*frameSize)

	for atomic.LoadInt32(&p.running) == 1 {
		startTime := time.Now()

		// Try to get audio data from buffer
		audioData, hasData := p.buffer.Read()
		
		var dataToPlay []byte
		if hasData && len(audioData) == p.config.FramesPerBuffer*frameSize {
			dataToPlay = audioData
			
			// 计算播放音频的分贝级别
			decibelLevel := p.calculateDecibels(audioData)
			p.updateDecibelLevel(decibelLevel)
		} else {
			// No data available or incorrect size, play silence
			dataToPlay = silenceBuffer
			p.updateDecibelLevel(-60.0) // 静音
			if !hasData {
				atomic.AddInt64(&p.stats.DroppedFrames, int64(p.config.FramesPerBuffer))
			}
		}

		// Convert audio data and write to stream
		if err := p.convertAndWriteAudioData(dataToPlay); err != nil {
			p.logger.Error(fmt.Sprintf("Failed to write audio data: %v", err))
			atomic.AddInt64(&p.stats.DroppedFrames, int64(p.config.FramesPerBuffer))
			continue
		}

		// Write audio data to stream
		err := p.stream.Write()
		if err != nil {
			p.logger.Error(fmt.Sprintf("Failed to write to audio stream: %v", err))
			atomic.AddInt64(&p.stats.DroppedFrames, int64(p.config.FramesPerBuffer))
			
			// Check if this is a critical error
			if err == portaudio.OutputUnderflowed {
				p.logger.Warn("Output buffer underflow detected")
			} else {
				// For other errors, we might want to stop
				break
			}
			continue
		}

		// Update statistics
		atomic.AddInt64(&p.stats.FramesProcessed, int64(p.config.FramesPerBuffer))
		
		// Calculate processing latency
		processingTime := time.Since(startTime)
		p.stats.Latency = processingTime
		p.stats.BufferUsage = p.buffer.Usage()
	}

	p.logger.Debug("Audio playback loop ended")
}

// convertAndWriteAudioData converts bytes to the appropriate format and writes to stream buffer
func (p *Player) convertAndWriteAudioData(audioData []byte) error {
	if p.outputBuffer == nil {
		return utils.NewAppError(utils.ErrAudioPlayback, "output buffer is nil")
	}

	switch p.config.BitDepth {
	case 16:
		// 修复：使用保存的输出缓冲区引用
		output, ok := p.outputBuffer.([]int16)
		if !ok {
			return utils.NewAppError(utils.ErrAudioPlayback, "invalid output buffer type for 16-bit")
		}

		sampleCount := len(audioData) / 2
		if sampleCount > len(output) {
			sampleCount = len(output)
		}

		for i := 0; i < sampleCount; i++ {
			if i*2+1 < len(audioData) {
				// Little-endian conversion
				sample := int16(audioData[i*2]) | (int16(audioData[i*2+1]) << 8)
				output[i] = sample
			}
		}

		// Fill remaining with silence if needed
		for i := sampleCount; i < len(output); i++ {
			output[i] = 0
		}

	case 32:
		// 修复：使用保存的输出缓冲区引用
		output, ok := p.outputBuffer.([]int32)
		if !ok {
			return utils.NewAppError(utils.ErrAudioPlayback, "invalid output buffer type for 32-bit")
		}

		sampleCount := len(audioData) / 4
		if sampleCount > len(output) {
			sampleCount = len(output)
		}

		for i := 0; i < sampleCount; i++ {
			if i*4+3 < len(audioData) {
				// Little-endian conversion
				sample := int32(audioData[i*4]) |
					(int32(audioData[i*4+1]) << 8) |
					(int32(audioData[i*4+2]) << 16) |
					(int32(audioData[i*4+3]) << 24)
				output[i] = sample
			}
		}

		// Fill remaining with silence if needed
		for i := sampleCount; i < len(output); i++ {
			output[i] = 0
		}

	default:
		return utils.NewAppError(utils.ErrAudioPlayback, 
			fmt.Sprintf("unsupported bit depth: %d", p.config.BitDepth))
	}

	return nil
}

// IsRunning returns whether the player is currently running
func (p *Player) IsRunning() bool {
	return atomic.LoadInt32(&p.running) == 1
}

// IsInitialized returns whether the player is initialized
func (p *Player) IsInitialized() bool {
	return atomic.LoadInt32(&p.initialized) == 1
}

// GetStats returns current playback statistics
func (p *Player) GetStats() *utils.AudioStats {
	bufferUsage := p.buffer.Usage()
	// 确保缓冲区使用率在0-1范围内
	if bufferUsage > 1.0 {
		bufferUsage = 1.0
	} else if bufferUsage < 0.0 {
		bufferUsage = 0.0
	}
	
	return &utils.AudioStats{
		FramesProcessed: atomic.LoadInt64(&p.stats.FramesProcessed),
		DroppedFrames:   atomic.LoadInt64(&p.stats.DroppedFrames),
		Latency:         p.stats.Latency,
		BufferUsage:     bufferUsage,
		DecibelLevel:    p.getCurrentDecibelLevel(),
	}
}

// GetBufferUsage returns current buffer usage
func (p *Player) GetBufferUsage() float64 {
	return p.buffer.Usage()
}

// ClearBuffer clears the audio buffer
func (p *Player) ClearBuffer() {
	p.buffer.Clear()
}