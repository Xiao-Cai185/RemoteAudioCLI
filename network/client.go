// network/client.go - 实时统计显示版本

package network

import (
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"RemoteAudioCLI/audio"
	"RemoteAudioCLI/utils"
<<<<<<< HEAD
	"github.com/hraban/opus"
=======
>>>>>>> f22ae08551c5c9d0a35b183a89426ada56f9bc31
)

// Client represents a network client for audio streaming
type Client struct {
	config   *utils.Config
	logger   *utils.Logger
	conn     net.Conn
	capturer *audio.Capturer
	
	// Connection state
	connected    int32 // atomic bool
	sequence     uint32
	lastHeartbeat time.Time
	
<<<<<<< HEAD
	// Heartbeat tracking
	heartbeatMutex sync.RWMutex
	lastHeartbeatSent time.Time
	lastHeartbeatReceived time.Time
	
=======
>>>>>>> f22ae08551c5c9d0a35b183a89426ada56f9bc31
	// Statistics
	stats *utils.NetworkStats
	
	// Control channels
	stopChan   chan struct{}
	errorChan  chan error
	wg         sync.WaitGroup
<<<<<<< HEAD
	
	opusEncoder *opus.Encoder
	useOpus     bool
=======
>>>>>>> f22ae08551c5c9d0a35b183a89426ada56f9bc31
}

// NewClient creates a new network client
func NewClient(config *utils.Config, logger *utils.Logger) *Client {
	return &Client{
		config:    config,
		logger:    logger,
		stopChan:  make(chan struct{}),
		errorChan: make(chan error, 10),
		stats: &utils.NetworkStats{
			BytesSent:     0,
			BytesReceived: 0,
			ErrorCount:    0,
		},
	}
}

// Start initiates the client connection and audio streaming
func (c *Client) Start(inputDevice *audio.DeviceInfo) error {
	c.logger.Info("🔗 Connecting to server...")
	
	// 注册关闭回调
	RegisterShutdownCallback(func() {
		c.Stop()
	})
	
	// Connect to server
	if err := c.connect(); err != nil {
		return utils.WrapError(err, utils.ErrConnection, "failed to connect to server")
	}
	
	c.logger.Info("✅ Connected to server successfully")
	
	// Perform handshake
	if err := c.handshake(); err != nil {
		c.conn.Close()
		return utils.WrapError(err, utils.ErrProtocol, "handshake failed")
	}
	
	c.logger.Info("🤝 Handshake completed")
	
	// Initialize audio capturer
	c.capturer = audio.NewCapturer(inputDevice, c.config, c.logger)
	if err := c.capturer.Initialize(); err != nil {
		c.conn.Close()
		return utils.WrapError(err, utils.ErrAudioCapture, "failed to initialize audio capturer")
	}
	
	c.logger.Info("🎤 Audio capturer initialized")
	
<<<<<<< HEAD
	// 初始化心跳包时间
	c.heartbeatMutex.Lock()
	c.lastHeartbeatSent = time.Now()
	c.lastHeartbeatReceived = time.Now()
	c.heartbeatMutex.Unlock()
	
	// Start background routines
	c.wg.Add(4) // 增加到4个goroutine
	go c.audioStreamingLoop()
	go c.heartbeatLoop()
	go c.packetProcessingLoop() // 新增：处理服务端数据包
=======
	// Start background routines
	c.wg.Add(3)
	go c.audioStreamingLoop()
	go c.heartbeatLoop()
>>>>>>> f22ae08551c5c9d0a35b183a89426ada56f9bc31
	go c.errorHandlingLoop()
	
	// Monitor shutdown signals
	go c.monitorShutdown()
	
<<<<<<< HEAD
	c.useOpus = c.config.Compression
	if c.useOpus {
		validOpusRates := map[int]bool{8000: true, 12000: true, 16000: true, 24000: true, 48000: true}
		if !validOpusRates[c.config.SampleRate] {
			return utils.NewAppError(utils.ErrAudioCapture, fmt.Sprintf("Opus only supports sample rates: 8000, 12000, 16000, 24000, 48000 Hz, got %d", c.config.SampleRate))
		}
		var err error
		c.opusEncoder, err = opus.NewEncoder(c.config.SampleRate, c.config.Channels, opus.AppAudio)
		if err != nil {
			return utils.WrapError(err, utils.ErrAudioCapture, "failed to initialize Opus encoder")
		}
	}
	
=======
>>>>>>> f22ae08551c5c9d0a35b183a89426ada56f9bc31
	// Start audio capture
	if err := c.capturer.Start(c.onAudioData); err != nil {
		c.Stop()
		return utils.WrapError(err, utils.ErrAudioCapture, "failed to start audio capture")
	}
	
	c.logger.Info("🚀 Client started successfully - streaming audio...")
	c.logger.Info("💡 Press Ctrl+C to stop the client")
	c.logger.Info("📊 Real-time statistics will appear below:")
	atomic.StoreInt32(&c.connected, 1)
	IncrementConnections()
	
	// Wait for shutdown
	c.wg.Wait()
	
	return nil
}

// Stop gracefully shuts down the client
func (c *Client) Stop() {
<<<<<<< HEAD
	// 使用原子操作确保只执行一次
	oldValue := atomic.SwapInt32(&c.connected, 0)
	if oldValue == 0 {
		// 已经在停止过程中或已经停止
		return
	}
	
	c.logger.Info("🛑 Stopping client...")
	
=======
	c.logger.Info("🛑 Stopping client...")
	
	// Mark as disconnected
	if atomic.LoadInt32(&c.connected) == 1 {
		atomic.StoreInt32(&c.connected, 0)
		DecrementConnections()
	}
	
>>>>>>> f22ae08551c5c9d0a35b183a89426ada56f9bc31
	// Stop audio capture
	if c.capturer != nil {
		c.capturer.Stop()
		c.capturer.Terminate()
	}
	
	// Close connection
	if c.conn != nil {
		c.conn.Close()
	}
	
<<<<<<< HEAD
	// Signal stop to all goroutines (使用安全的关闭方式)
	select {
	case <-c.stopChan:
		// 通道已经关闭，不需要再次关闭
	default:
		close(c.stopChan)
	}
=======
	// Signal stop to all goroutines
	close(c.stopChan)
>>>>>>> f22ae08551c5c9d0a35b183a89426ada56f9bc31
	
	// Wait for goroutines to finish with timeout
	done := make(chan struct{})
	go func() {
		c.wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		c.logger.Info("✅ All client goroutines stopped")
	case <-time.After(3 * time.Second):
		c.logger.Warn("⚠️  Client goroutines did not stop within timeout")
	}
	
<<<<<<< HEAD
	// 减少连接计数
	DecrementConnections()
	
=======
>>>>>>> f22ae08551c5c9d0a35b183a89426ada56f9bc31
	c.logger.Info("✅ Client stopped")
}

// monitorShutdown 监控关闭信号
func (c *Client) monitorShutdown() {
	select {
	case <-GetShutdownChannel():
		c.logger.Info("🛑 Shutdown signal received")
<<<<<<< HEAD
		// 只有在还连接时才调用Stop
		if atomic.LoadInt32(&c.connected) == 1 {
			c.Stop()
		}
=======
		c.Stop()
>>>>>>> f22ae08551c5c9d0a35b183a89426ada56f9bc31
	case <-c.stopChan:
		return
	}
}

// connect establishes a TCP connection to the server
func (c *Client) connect() error {
	address := c.config.GetNetworkAddress()
	
	c.logger.Infof("🔗 Connecting to %s...", address)
	
	conn, err := net.DialTimeout("tcp", address, c.config.ConnTimeout)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", address, err)
	}
	
	c.conn = conn
	c.logger.Infof("✅ TCP connection established")
	return nil
}

// handshake performs the initial handshake with the server
func (c *Client) handshake() error {
	c.logger.Info("🤝 Starting handshake...")
	
<<<<<<< HEAD
	var compression uint8 = 0
	if c.config.Compression {
		compression = 1
	}
=======
	// Create handshake configuration
>>>>>>> f22ae08551c5c9d0a35b183a89426ada56f9bc31
	handshakeConfig := &HandshakeConfig{
		SampleRate:      uint32(c.config.SampleRate),
		Channels:        uint8(c.config.Channels),
		BitDepth:        uint8(c.config.BitDepth),
		FramesPerBuffer: uint16(c.config.FramesPerBuffer),
		BufferCount:     uint8(c.config.BufferCount),
<<<<<<< HEAD
		Compression:     compression,
=======
		Compression:     0, // No compression for now
>>>>>>> f22ae08551c5c9d0a35b183a89426ada56f9bc31
	}
	
	// Validate configuration
	if err := handshakeConfig.Validate(); err != nil {
		return fmt.Errorf("invalid handshake config: %w", err)
	}
	
	// Send handshake packet
	handshakePacket := NewHandshakePacket(handshakeConfig)
	if err := WritePacket(c.conn, handshakePacket); err != nil {
		return fmt.Errorf("failed to send handshake: %w", err)
	}
	
	c.logger.Debug("📤 Handshake packet sent")
	
	// Set read timeout for handshake response
	c.conn.SetReadDeadline(time.Now().Add(c.config.ReadTimeout))
	defer c.conn.SetReadDeadline(time.Time{})
	
	// Read handshake response
	responsePacket, err := ReadPacket(c.conn)
	if err != nil {
		return fmt.Errorf("failed to read handshake response: %w", err)
	}
	
	if responsePacket.Header.Type != PacketTypeHandshake {
		return fmt.Errorf("unexpected packet type in handshake response: %s", responsePacket.Header.Type)
	}
	
	// Parse server configuration
	var serverConfig HandshakeConfig
	if err := serverConfig.FromBytes(responsePacket.Payload); err != nil {
		return fmt.Errorf("failed to parse server config: %w", err)
	}
	
	// Update client configuration with server's preferred settings
	c.updateConfigFromServer(&serverConfig)
	
<<<<<<< HEAD
	c.logger.Infof("✅ Handshake successful - Sample Rate: %dHz, Channels: %d, Bit Depth: %d, compress: Opus %s",
		serverConfig.SampleRate, serverConfig.Channels, serverConfig.BitDepth,
		map[bool]string{true: "ON", false: "OFF"}[c.config.Compression])
=======
	c.logger.Infof("✅ Handshake successful - Sample Rate: %dHz, Channels: %d, Bit Depth: %d",
		serverConfig.SampleRate, serverConfig.Channels, serverConfig.BitDepth)
>>>>>>> f22ae08551c5c9d0a35b183a89426ada56f9bc31
	
	return nil
}

// updateConfigFromServer updates client config based on server response
func (c *Client) updateConfigFromServer(serverConfig *HandshakeConfig) {
	// Use server's preferred settings
	c.config.SampleRate = int(serverConfig.SampleRate)
	c.config.Channels = int(serverConfig.Channels)
	c.config.BitDepth = int(serverConfig.BitDepth)
	c.config.FramesPerBuffer = int(serverConfig.FramesPerBuffer)
	c.config.BufferCount = int(serverConfig.BufferCount)
}

// onAudioData is called when audio data is captured
func (c *Client) onAudioData(audioData []byte) {
	if atomic.LoadInt32(&c.connected) == 0 || IsShutdownRequested() {
		return
	}
<<<<<<< HEAD
	var payload []byte
	if c.useOpus && c.opusEncoder != nil {
		// PCM []byte 转 []int16
		sampleCount := len(audioData) / 2
		pcm16 := make([]int16, sampleCount)
		for i := 0; i < sampleCount; i++ {
			pcm16[i] = int16(audioData[2*i]) | int16(audioData[2*i+1])<<8
		}
		maxDataBytes := 4000
		opusBuf := make([]byte, maxDataBytes)
		lenOut, err := c.opusEncoder.Encode(pcm16, opusBuf)
		if err != nil {
			c.logger.Error(fmt.Sprintf("Opus encode error: %v", err))
			return
		}
		payload = opusBuf[:lenOut]
	} else {
		// PCM 直传
		payload = audioData
	}
	sequence := atomic.AddUint32(&c.sequence, 1)
	audioPacket := NewAudioPacket(payload, sequence)
	c.conn.SetWriteDeadline(time.Now().Add(c.config.WriteTimeout))
=======
	
	// Create and send audio packet
	sequence := atomic.AddUint32(&c.sequence, 1)
	audioPacket := NewAudioPacket(audioData, sequence)
	
	// Set write timeout
	c.conn.SetWriteDeadline(time.Now().Add(c.config.WriteTimeout))
	
>>>>>>> f22ae08551c5c9d0a35b183a89426ada56f9bc31
	if err := WritePacket(c.conn, audioPacket); err != nil {
		if atomic.LoadInt32(&c.connected) == 1 {
			c.errorChan <- utils.WrapError(err, utils.ErrNetwork, "failed to send audio packet")
		}
		return
	}
<<<<<<< HEAD
	atomic.AddInt64(&c.stats.BytesSent, int64(len(payload)+HeaderSize))
=======
	
	// Update statistics
	atomic.AddInt64(&c.stats.BytesSent, int64(len(audioData)+HeaderSize))
>>>>>>> f22ae08551c5c9d0a35b183a89426ada56f9bc31
}

// audioStreamingLoop handles the main audio streaming logic
func (c *Client) audioStreamingLoop() {
	defer c.wg.Done()
	
	// 每100ms刷新一次统计信息
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	
	for {
		select {
		case <-c.stopChan:
			return
		case <-GetShutdownChannel():
			return
		case <-ticker.C:
			// 实时显示统计信息
			if atomic.LoadInt32(&c.connected) == 1 {
				networkStats := c.GetStats()
				
				var audioStats *utils.AudioStats
				if c.capturer != nil {
					audioStats = c.capturer.GetStats()
				} else {
					// 创建默认的音频统计
					audioStats = &utils.AudioStats{
						FramesProcessed: 0,
						DroppedFrames:   0,
						Latency:         0,
						BufferUsage:     0,
						DecibelLevel:    -60.0,
					}
				}
				
				// 使用新的实时统计显示方法
				c.logger.LogRealTimeStats(networkStats, audioStats)
			}
		}
	}
}

// heartbeatLoop sends periodic heartbeat packets
func (c *Client) heartbeatLoop() {
	defer c.wg.Done()
	
<<<<<<< HEAD
	// 使用配置中的心跳包间隔
	ticker := time.NewTicker(c.config.HeartbeatInterval)
=======
	ticker := time.NewTicker(5 * time.Second)
>>>>>>> f22ae08551c5c9d0a35b183a89426ada56f9bc31
	defer ticker.Stop()
	
	for {
		select {
		case <-c.stopChan:
			return
		case <-GetShutdownChannel():
			return
		case <-ticker.C:
			if atomic.LoadInt32(&c.connected) == 1 {
				heartbeatStart := time.Now()
				heartbeatPacket := NewHeartbeatPacket()
				
<<<<<<< HEAD
				// 更新发送时间
				c.heartbeatMutex.Lock()
				c.lastHeartbeatSent = time.Now()
				c.heartbeatMutex.Unlock()
				
=======
>>>>>>> f22ae08551c5c9d0a35b183a89426ada56f9bc31
				c.conn.SetWriteDeadline(time.Now().Add(c.config.WriteTimeout))
				if err := WritePacket(c.conn, heartbeatPacket); err != nil {
					if atomic.LoadInt32(&c.connected) == 1 {
						c.errorChan <- utils.WrapError(err, utils.ErrNetwork, "failed to send heartbeat")
					}
				} else {
					c.lastHeartbeat = time.Now()
					// 计算 RTT (Round Trip Time)
					c.stats.RoundTripTime = time.Since(heartbeatStart)
<<<<<<< HEAD
					c.logger.Debug("💓 Heartbeat sent")
=======
>>>>>>> f22ae08551c5c9d0a35b183a89426ada56f9bc31
				}
			}
		}
	}
}

// errorHandlingLoop handles errors from other goroutines
func (c *Client) errorHandlingLoop() {
	defer c.wg.Done()
	
	for {
		select {
		case <-c.stopChan:
			return
		case <-GetShutdownChannel():
			return
		case err := <-c.errorChan:
			c.logger.Error(fmt.Sprintf("Client error: %v", err))
			atomic.AddInt64(&c.stats.ErrorCount, 1)
			
			// For critical errors, stop the client
			if utils.IsErrorType(err, utils.ErrConnection) || utils.IsErrorType(err, utils.ErrNetwork) {
				c.logger.Error("Critical error detected, stopping client...")
				go c.Stop()
				return
			}
		}
	}
}

<<<<<<< HEAD
// packetProcessingLoop processes incoming packets from the server
func (c *Client) packetProcessingLoop() {
	defer c.wg.Done()
	
	c.logger.Debug("Starting packet processing loop")
	
	for {
		select {
		case <-c.stopChan:
			c.logger.Debug("Packet processing loop stopped by signal")
			return
		case <-GetShutdownChannel():
			c.logger.Debug("Packet processing loop stopped by shutdown signal")
			return
		default:
			// Continue processing
		}
		
		// Set read timeout
		c.conn.SetReadDeadline(time.Now().Add(c.config.ReadTimeout))
		
		packet, err := ReadPacket(c.conn)
		if err != nil {
			if atomic.LoadInt32(&c.connected) == 1 {
				c.logger.Error(fmt.Sprintf("Failed to read packet: %v", err))
				c.errorChan <- utils.WrapError(err, utils.ErrNetwork, "failed to read packet")
			}
			return
		}
		
		// Update statistics
		atomic.AddInt64(&c.stats.BytesReceived, int64(len(packet.Payload)+HeaderSize))
		
		// Process packet based on type
		switch packet.Header.Type {
		case PacketTypeHeartbeat:
			// 更新心跳包接收时间
			c.heartbeatMutex.Lock()
			c.lastHeartbeatReceived = time.Now()
			c.heartbeatMutex.Unlock()
			c.logger.Debug("💓 Heartbeat response received")
			
		case PacketTypeError:
			errorMessage := string(packet.Payload)
			c.logger.Error(fmt.Sprintf("Server error: %s", errorMessage))
			
		default:
			c.logger.Warnf("Unknown packet type received: %s", packet.Header.Type)
		}
	}
}

=======
>>>>>>> f22ae08551c5c9d0a35b183a89426ada56f9bc31
// IsConnected returns whether the client is currently connected
func (c *Client) IsConnected() bool {
	return atomic.LoadInt32(&c.connected) == 1
}

// GetStats returns current network statistics
func (c *Client) GetStats() *utils.NetworkStats {
	return &utils.NetworkStats{
		BytesSent:      atomic.LoadInt64(&c.stats.BytesSent),
		BytesReceived:  atomic.LoadInt64(&c.stats.BytesReceived),
		RoundTripTime:  c.stats.RoundTripTime,
		ErrorCount:     atomic.LoadInt64(&c.stats.ErrorCount),
	}
}