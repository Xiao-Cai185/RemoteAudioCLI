// network/server.go - 实时统计显示版本

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

// Server represents a network server for audio streaming
type Server struct {
	config             *utils.Config
	logger             *utils.Logger
	listener           net.Listener
	player             *audio.Player
	notificationPlayer *audio.NotificationPlayer
	
	// Connection state
	running     int32 // atomic bool
	clientConn  net.Conn
	connected   int32 // atomic bool
	
	// Audio configuration (negotiated during handshake)
	audioConfig *HandshakeConfig
	
	// Statistics
	stats *utils.NetworkStats
	
	// Control channels for main server loop
	stopChan   chan struct{}
	errorChan  chan error
	
	// Control channels for client session - 使用指针以便重新创建
	clientStopChan *chan struct{}
	clientWg       sync.WaitGroup
	
	// Connection management
	connectionMutex sync.Mutex
}

// NewServer creates a new network server
func NewServer(config *utils.Config, logger *utils.Logger) *Server {
	return &Server{
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

// Start initiates the server and begins listening for connections
func (s *Server) Start(outputDevice *audio.DeviceInfo) error {
	s.logger.Info("🔊 Starting audio server...")
	
	// 注册关闭回调
	RegisterShutdownCallback(func() {
		s.Stop()
	})

	// 创建通知播放器
	s.notificationPlayer = audio.NewNotificationPlayer(outputDevice, s.config, s.logger)
	
	// Start listening
	if err := s.startListening(); err != nil {
		return utils.WrapError(err, utils.ErrNetwork, "failed to start listening")
	}
	
	s.logger.Infof("📡 Server listening on %s", s.config.GetNetworkAddress())
	s.logger.Info("💡 Press Ctrl+C to stop the server")
	atomic.StoreInt32(&s.running, 1)
	
	// Accept connections in a loop
	for atomic.LoadInt32(&s.running) == 1 && !IsShutdownRequested() {
		// 设置接受连接的超时，以便检查关闭信号
		if tcpListener, ok := s.listener.(*net.TCPListener); ok {
			tcpListener.SetDeadline(time.Now().Add(1 * time.Second))
		}

		conn, err := s.listener.Accept()
		if err != nil {
			if atomic.LoadInt32(&s.running) == 0 || IsShutdownRequested() {
				break // Server is shutting down
			}
			
			// 检查是否是超时错误
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue // 超时，继续监听
			}
			
			s.logger.Error(fmt.Sprintf("Failed to accept connection: %v", err))
			continue
		}
		
		s.logger.Info("🔗 Client connected from: " + conn.RemoteAddr().String())
		
		// 使用互斥锁保护连接状态检查
		s.connectionMutex.Lock()
		if atomic.LoadInt32(&s.connected) == 1 {
			s.logger.Warn("Another client is already connected, closing new connection")
			conn.Close()
			s.connectionMutex.Unlock()
			continue
		}
		
		// 设置连接状态
		atomic.StoreInt32(&s.connected, 1)
		s.connectionMutex.Unlock()
		
		// 播放连接提示音
		go s.notificationPlayer.PlayConnectionSound()
		
		// Handle the client connection in a separate goroutine
		// 关键修改：使用 goroutine 处理客户端连接，避免阻塞主循环
		go s.handleClient(conn, outputDevice)
	}
	
	s.logger.Info("✅ Server stopped")
	return nil
}

// Stop gracefully shuts down the server
func (s *Server) Stop() {
	s.logger.Info("🛑 Stopping server...")
	
	// Mark as not running
	atomic.StoreInt32(&s.running, 0)
	
	// Stop current client session
	s.forceStopClientSession()
	
	// Close listener
	if s.listener != nil {
		s.listener.Close()
	}
	
	// Signal stop to main server
	close(s.stopChan)
	
	s.logger.Info("✅ Server stopped")
}

// forceStopClientSession 强制停止当前客户端会话
func (s *Server) forceStopClientSession() {
	s.connectionMutex.Lock()
	defer s.connectionMutex.Unlock()
	
	if atomic.LoadInt32(&s.connected) == 0 {
		return // 没有活跃连接
	}
	
	s.logger.Info("🔌 Force stopping client session...")
	
	// 强制关闭连接来中断阻塞的读取
	if s.clientConn != nil {
		s.clientConn.Close()
	}
	
	// 等待 handleClient 完成清理
	// 注意：不要在这里关闭 clientStopChan，让 handleClient 的 defer 处理
	time.Sleep(100 * time.Millisecond)
}

// cleanupClientSession 清理客户端会话 (在 handleClient 中调用)
func (s *Server) cleanupClientSession() {
	s.logger.Info("🔌 Cleaning up client session...")
	
	// 播放断开连接提示音
	if s.notificationPlayer != nil {
		go s.notificationPlayer.PlayDisconnectionSound()
	}
	
	// 注意：不在这里关闭 clientStopChan，因为 handleClient 的 defer 函数会处理它
	
	// 等待客户端 goroutine 结束（这个等待已在 handleClient 的 defer 中完成）
	// 这里不需要再次等待，避免死锁
	
	// Stop audio player
	if s.player != nil {
		s.player.Stop()
		s.player.Terminate()
		s.player = nil
	}
	
	// Close client connection
	if s.clientConn != nil {
		s.clientConn.Close()
		s.clientConn = nil
	}
	
	// Reset connection state
	atomic.StoreInt32(&s.connected, 0)
	DecrementConnections()
	
	// Reset statistics
	atomic.StoreInt64(&s.stats.BytesSent, 0)
	atomic.StoreInt64(&s.stats.BytesReceived, 0)
	atomic.StoreInt64(&s.stats.ErrorCount, 0)
	
	s.logger.Info("✅ Client session cleaned up")
	
	// 关键修改：显式记录准备接受新连接的状态
	s.logger.Info("🔄 Ready for new client connections...")
}

// startListening creates and starts the TCP listener
func (s *Server) startListening() error {
	address := s.config.GetNetworkAddress()
	
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", address, err)
	}
	
	s.listener = listener
	return nil
}

// handleClient handles a single client connection
func (s *Server) handleClient(conn net.Conn, outputDevice *audio.DeviceInfo) {
	// 为这个客户端会话创建新的控制通道
	clientStopChan := make(chan struct{})
	s.clientStopChan = &clientStopChan
	s.clientConn = conn
	IncrementConnections()
	
	// 创建一个用于协调清理的context
	sessionDone := make(chan struct{})
	
	// 用于防止多次关闭 channel
	var stopChanClosed int32 // atomic bool
	
	// 安全关闭 clientStopChan 的函数
	closeClientStopChan := func() {
		if atomic.CompareAndSwapInt32(&stopChanClosed, 0, 1) {
			close(clientStopChan)
		}
	}
	
	// 确保在函数结束时清理会话
	defer func() {
		s.logger.Info("🔌 Client session ended")
		
		// 安全关闭 clientStopChan 通知所有 goroutine 停止
		closeClientStopChan()
		
		// 等待所有 goroutine 结束，但设置超时
		done := make(chan struct{})
		go func() {
			s.clientWg.Wait()
			close(done)
		}()
		
		select {
		case <-done:
			s.logger.Debug("All client goroutines stopped normally")
		case <-time.After(3 * time.Second):
			s.logger.Warn("Client goroutines did not stop within timeout, proceeding with cleanup")
		}
		
		// 执行清理
		s.cleanupClientSession()
		close(sessionDone)
	}()
	
	// Perform handshake
	if err := s.performHandshake(conn); err != nil {
		s.logger.Error(fmt.Sprintf("Handshake failed: %v", err))
		return
	}
	
	s.logger.Info("🤝 Handshake completed with client")
	
	// Initialize audio player with negotiated configuration
	s.player = audio.NewPlayer(outputDevice, s.config, s.logger)
	if err := s.player.Initialize(); err != nil {
		s.logger.Error(fmt.Sprintf("Failed to initialize audio player: %v", err))
		return
	}
	
	s.logger.Info("🔊 Audio player initialized")
	
	// Start audio playback
	if err := s.player.Start(); err != nil {
		s.logger.Error(fmt.Sprintf("Failed to start audio player: %v", err))
		return
	}
	
	s.logger.Info("🚀 Server ready - waiting for audio data...")
	s.logger.Info("📊 Real-time statistics will appear below:")
	
	// Start background routines for this client session
	s.clientWg.Add(2)
	go s.statisticsLoop(clientStopChan, sessionDone)
	go s.connectionMonitorLoop(conn, clientStopChan, sessionDone)
	
	// 主要的数据处理循环 (阻塞)
	s.packetProcessingLoop(conn, clientStopChan)
	
	// 数据处理循环结束，意味着客户端断开连接
	s.logger.Info("📤 Packet processing ended, client disconnected")
}

// connectionMonitorLoop 监控连接状态
func (s *Server) connectionMonitorLoop(conn net.Conn, stopChan chan struct{}, sessionDone chan struct{}) {
	defer s.clientWg.Done()
	
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-stopChan:
			s.logger.Debug("Connection monitor loop stopped by signal")
			return
		case <-sessionDone:
			s.logger.Debug("Connection monitor loop stopped by session end")
			return
		case <-GetShutdownChannel():
			s.logger.Info("🛑 Shutdown signal received, closing client connection")
			conn.Close()
			return
		case <-ticker.C:
			// 定期检查连接状态
			if atomic.LoadInt32(&s.connected) == 0 {
				return
			}
		}
	}
}

// performHandshake handles the handshake protocol with the client
func (s *Server) performHandshake(conn net.Conn) error {
	// Set read timeout for handshake
	conn.SetReadDeadline(time.Now().Add(s.config.ReadTimeout))
	defer conn.SetReadDeadline(time.Time{})
	
	// Read handshake packet from client
	handshakePacket, err := ReadPacket(conn)
	if err != nil {
		return fmt.Errorf("failed to read handshake packet: %w", err)
	}
	
	if handshakePacket.Header.Type != PacketTypeHandshake {
		return fmt.Errorf("expected handshake packet, got %s", handshakePacket.Header.Type)
	}
	
	// Parse client configuration
	var clientConfig HandshakeConfig
	if err := clientConfig.FromBytes(handshakePacket.Payload); err != nil {
		return fmt.Errorf("failed to parse client config: %w", err)
	}
	
	// Validate client configuration
	if err := clientConfig.Validate(); err != nil {
		return fmt.Errorf("invalid client config: %w", err)
	}
	
	s.logger.Infof("Client config - Sample Rate: %dHz, Channels: %d, Bit Depth: %d",
		clientConfig.SampleRate, clientConfig.Channels, clientConfig.BitDepth)
	
	// Create server response (accepting client's configuration for now)
	serverConfig := clientConfig // Accept client's settings
	s.audioConfig = &serverConfig
	
	// Update server configuration
	s.updateConfigFromHandshake(&serverConfig)
	
	// Send response
	responsePacket := NewHandshakePacket(&serverConfig)
	
	conn.SetWriteDeadline(time.Now().Add(s.config.WriteTimeout))
	if err := WritePacket(conn, responsePacket); err != nil {
		return fmt.Errorf("failed to send handshake response: %w", err)
	}
	
	return nil
}

// updateConfigFromHandshake updates server config based on handshake
func (s *Server) updateConfigFromHandshake(handshakeConfig *HandshakeConfig) {
	s.config.SampleRate = int(handshakeConfig.SampleRate)
	s.config.Channels = int(handshakeConfig.Channels)
	s.config.BitDepth = int(handshakeConfig.BitDepth)
	s.config.FramesPerBuffer = int(handshakeConfig.FramesPerBuffer)
	s.config.BufferCount = int(handshakeConfig.BufferCount)
}

// packetProcessingLoop processes incoming packets from the client
func (s *Server) packetProcessingLoop(conn net.Conn, stopChan chan struct{}) {
	s.logger.Debug("Starting packet processing loop")
	
	for {
		select {
		case <-stopChan:
			s.logger.Debug("Packet processing loop stopped by signal")
			return
		default:
			// Continue processing
		}
		
		// Set read timeout
		conn.SetReadDeadline(time.Now().Add(s.config.ReadTimeout))
		
		packet, err := ReadPacket(conn)
		if err != nil {
			s.logger.Error(fmt.Sprintf("Failed to read packet: %v", err))
			atomic.AddInt64(&s.stats.ErrorCount, 1)
			
			// 网络错误，客户端已断开连接
			s.logger.Info("🔌 Client appears to have disconnected")
			return
		}
		
		// Update statistics
		atomic.AddInt64(&s.stats.BytesReceived, int64(len(packet.Payload)+HeaderSize))
		
		// Process packet based on type
		switch packet.Header.Type {
		case PacketTypeAudio:
			s.handleAudioPacket(packet)
			
		case PacketTypeHeartbeat:
			s.handleHeartbeatPacket(conn, packet)
			
		case PacketTypeError:
			s.handleErrorPacket(packet)
			
		default:
			s.logger.Warnf("Unknown packet type received: %s", packet.Header.Type)
		}
	}
}

// handleAudioPacket processes an audio packet
func (s *Server) handleAudioPacket(packet *Packet) {
	if s.player == nil {
		return
	}
	
	// Queue audio data for playback
	if err := s.player.QueueAudio(packet.Payload); err != nil {
		s.logger.Error(fmt.Sprintf("Failed to queue audio: %v", err))
		atomic.AddInt64(&s.stats.ErrorCount, 1)
	}
}

// handleHeartbeatPacket processes a heartbeat packet
func (s *Server) handleHeartbeatPacket(conn net.Conn, packet *Packet) {
	// Respond with heartbeat
	responsePacket := NewHeartbeatPacket()
	
	conn.SetWriteDeadline(time.Now().Add(s.config.WriteTimeout))
	if err := WritePacket(conn, responsePacket); err != nil {
		s.logger.Error(fmt.Sprintf("Failed to send heartbeat response: %v", err))
		atomic.AddInt64(&s.stats.ErrorCount, 1)
	} else {
		atomic.AddInt64(&s.stats.BytesSent, int64(HeaderSize))
	}
}

// handleErrorPacket processes an error packet
func (s *Server) handleErrorPacket(packet *Packet) {
	errorMessage := string(packet.Payload)
	s.logger.Error(fmt.Sprintf("Client error: %s", errorMessage))
}

// statisticsLoop periodically logs server statistics
func (s *Server) statisticsLoop(stopChan chan struct{}, sessionDone chan struct{}) {
	defer s.clientWg.Done()
	
	// 每100ms刷新一次统计信息
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	
	s.logger.Debug("Starting statistics loop")
	
	for {
		select {
		case <-stopChan:
			s.logger.Debug("Statistics loop stopped by signal")
			return
		case <-sessionDone:
			s.logger.Debug("Statistics loop stopped by session end")
			return
		case <-ticker.C:
			if atomic.LoadInt32(&s.connected) == 1 {
				networkStats := s.GetStats()
				
				var audioStats *utils.AudioStats
				if s.player != nil {
					audioStats = s.player.GetStats()
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
				s.logger.LogRealTimeStats(networkStats, audioStats)
			}
		}
	}
}

// IsRunning returns whether the server is currently running
func (s *Server) IsRunning() bool {
	return atomic.LoadInt32(&s.running) == 1
}

// IsConnected returns whether a client is currently connected
func (s *Server) IsConnected() bool {
	return atomic.LoadInt32(&s.connected) == 1
}

// GetStats returns current network statistics
func (s *Server) GetStats() *utils.NetworkStats {
	return &utils.NetworkStats{
		BytesSent:      atomic.LoadInt64(&s.stats.BytesSent),
		BytesReceived:  atomic.LoadInt64(&s.stats.BytesReceived),
		RoundTripTime:  s.stats.RoundTripTime,
		ErrorCount:     atomic.LoadInt64(&s.stats.ErrorCount),
	}
}