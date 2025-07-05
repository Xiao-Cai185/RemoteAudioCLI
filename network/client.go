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
	
	// Statistics
	stats *utils.NetworkStats
	
	// Control channels
	stopChan   chan struct{}
	errorChan  chan error
	wg         sync.WaitGroup
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
	
	// Start background routines
	c.wg.Add(3)
	go c.audioStreamingLoop()
	go c.heartbeatLoop()
	go c.errorHandlingLoop()
	
	// Monitor shutdown signals
	go c.monitorShutdown()
	
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
	c.logger.Info("🛑 Stopping client...")
	
	// Mark as disconnected
	if atomic.LoadInt32(&c.connected) == 1 {
		atomic.StoreInt32(&c.connected, 0)
		DecrementConnections()
	}
	
	// Stop audio capture
	if c.capturer != nil {
		c.capturer.Stop()
		c.capturer.Terminate()
	}
	
	// Close connection
	if c.conn != nil {
		c.conn.Close()
	}
	
	// Signal stop to all goroutines
	close(c.stopChan)
	
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
	
	c.logger.Info("✅ Client stopped")
}

// monitorShutdown 监控关闭信号
func (c *Client) monitorShutdown() {
	select {
	case <-GetShutdownChannel():
		c.logger.Info("🛑 Shutdown signal received")
		c.Stop()
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
	
	// Create handshake configuration
	handshakeConfig := &HandshakeConfig{
		SampleRate:      uint32(c.config.SampleRate),
		Channels:        uint8(c.config.Channels),
		BitDepth:        uint8(c.config.BitDepth),
		FramesPerBuffer: uint16(c.config.FramesPerBuffer),
		BufferCount:     uint8(c.config.BufferCount),
		Compression:     0, // No compression for now
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
	
	c.logger.Infof("✅ Handshake successful - Sample Rate: %dHz, Channels: %d, Bit Depth: %d",
		serverConfig.SampleRate, serverConfig.Channels, serverConfig.BitDepth)
	
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
	
	// Create and send audio packet
	sequence := atomic.AddUint32(&c.sequence, 1)
	audioPacket := NewAudioPacket(audioData, sequence)
	
	// Set write timeout
	c.conn.SetWriteDeadline(time.Now().Add(c.config.WriteTimeout))
	
	if err := WritePacket(c.conn, audioPacket); err != nil {
		if atomic.LoadInt32(&c.connected) == 1 {
			c.errorChan <- utils.WrapError(err, utils.ErrNetwork, "failed to send audio packet")
		}
		return
	}
	
	// Update statistics
	atomic.AddInt64(&c.stats.BytesSent, int64(len(audioData)+HeaderSize))
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
	
	ticker := time.NewTicker(5 * time.Second)
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
				
				c.conn.SetWriteDeadline(time.Now().Add(c.config.WriteTimeout))
				if err := WritePacket(c.conn, heartbeatPacket); err != nil {
					if atomic.LoadInt32(&c.connected) == 1 {
						c.errorChan <- utils.WrapError(err, utils.ErrNetwork, "failed to send heartbeat")
					}
				} else {
					c.lastHeartbeat = time.Now()
					// 计算 RTT (Round Trip Time)
					c.stats.RoundTripTime = time.Since(heartbeatStart)
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