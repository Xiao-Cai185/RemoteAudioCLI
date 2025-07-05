// utils/logger.go - 支持一行刷新的版本
package utils

import (
	"fmt"
	"log"
	"os"
	"time"
)

// LogLevel represents the severity of a log message
type LogLevel int

const (
	LogLevelDebug LogLevel = iota
	LogLevelInfo
	LogLevelWarn
	LogLevelError
)

// String returns the string representation of the log level
func (l LogLevel) String() string {
	switch l {
	case LogLevelDebug:
		return "DEBUG"
	case LogLevelInfo:
		return "INFO"
	case LogLevelWarn:
		return "WARN"
	case LogLevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// Logger provides structured logging functionality
type Logger struct {
	level           LogLevel
	logger          *log.Logger
	lastStatsOutput time.Time
	statsMode       bool // 是否处于统计显示模式
}

// NewLogger creates a new logger with INFO level
func NewLogger() *Logger {
	return &Logger{
		level:  LogLevelInfo,
		logger: log.New(os.Stdout, "", 0),
	}
}

// NewLoggerWithLevel creates a new logger with specified level
func NewLoggerWithLevel(level LogLevel) *Logger {
	return &Logger{
		level:  level,
		logger: log.New(os.Stdout, "", 0),
	}
}

// SetLevel sets the minimum log level
func (l *Logger) SetLevel(level LogLevel) {
	l.level = level
}

// GetLevel returns the current log level
func (l *Logger) GetLevel() LogLevel {
	return l.level
}

// log writes a log message with the specified level
func (l *Logger) log(level LogLevel, message string) {
	if level < l.level {
		return
	}

	// 如果处于统计模式，需要换行再输出普通日志
	if l.statsMode {
		fmt.Print("\n")
		l.statsMode = false
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	levelStr := level.String()
	
	// Add color codes for different log levels
	var colorCode string
	switch level {
	case LogLevelDebug:
		colorCode = "\033[36m" // Cyan
	case LogLevelInfo:
		colorCode = "\033[32m" // Green
	case LogLevelWarn:
		colorCode = "\033[33m" // Yellow
	case LogLevelError:
		colorCode = "\033[31m" // Red
	}
	resetCode := "\033[0m"

	formattedMessage := fmt.Sprintf("%s[%s] %s%s %s",
		colorCode, timestamp, levelStr, resetCode, message)
	
	l.logger.Println(formattedMessage)
}

// Debug logs a debug message
func (l *Logger) Debug(message string) {
	l.log(LogLevelDebug, message)
}

// Info logs an info message
func (l *Logger) Info(message string) {
	l.log(LogLevelInfo, message)
}

// Warn logs a warning message
func (l *Logger) Warn(message string) {
	l.log(LogLevelWarn, message)
}

// Error logs an error message
func (l *Logger) Error(message string) {
	l.log(LogLevelError, message)
}

// Debugf logs a formatted debug message
func (l *Logger) Debugf(format string, args ...interface{}) {
	l.Debug(fmt.Sprintf(format, args...))
}

// Infof logs a formatted info message
func (l *Logger) Infof(format string, args ...interface{}) {
	l.Info(fmt.Sprintf(format, args...))
}

// Warnf logs a formatted warning message
func (l *Logger) Warnf(format string, args ...interface{}) {
	l.Warn(fmt.Sprintf(format, args...))
}

// Errorf logs a formatted error message
func (l *Logger) Errorf(format string, args ...interface{}) {
	l.Error(fmt.Sprintf(format, args...))
}

// getLatencyIndicator 根据延迟返回相应的emoji指示器
func (l *Logger) getLatencyIndicator(latencyMs float64) string {
	if latencyMs <= 100 {
		return "🟢" // 绿色点 - 低延迟
	} else if latencyMs <= 500 {
		return "🟡" // 黄色点 - 中延迟
	} else {
		return "🔴" // 红色点 - 高延迟
	}
}

// LogRealTimeStats 实时显示网络和音频统计信息（一行刷新）
func (l *Logger) LogRealTimeStats(networkStats *NetworkStats, audioStats *AudioStats) {
	if l.level > LogLevelInfo {
		return
	}

	// 计算延迟毫秒数
	latencyMs := networkStats.RoundTripTime.Seconds() * 1000
	latencyIndicator := l.getLatencyIndicator(latencyMs)
	
	// 格式化统计信息
	timestamp := time.Now().Format("15:04:05")
	
	// 网络统计
	networkInfo := fmt.Sprintf("🌐 %s %.0fms %s | ↑%.2fMB ↓%.2fMB | ❌%d",
		latencyIndicator,
		latencyMs,
		"RTT",
		float64(networkStats.BytesSent)/(1024*1024),
		float64(networkStats.BytesReceived)/(1024*1024),
		networkStats.ErrorCount)
	
	// 音频统计 - 如果分贝低于-59.9dB则显示为--dB
	var decibelDisplay string
	if audioStats.DecibelLevel < -59.9 {
		decibelDisplay = "--dB"
	} else {
		decibelDisplay = fmt.Sprintf("%.1fdB", audioStats.DecibelLevel)
	}
	
	audioInfo := fmt.Sprintf("📊 %s | 🎵%dk | ⚡%.1fms | ⏳%.1f%%",
		decibelDisplay,
		audioStats.FramesProcessed/1000,
		audioStats.Latency.Seconds()*1000,
		audioStats.BufferUsage*100)
	
	// 使用 \r 实现一行刷新
	statsLine := fmt.Sprintf("\r[%s] %s | %s", timestamp, networkInfo, audioInfo)
	
	// 确保行的长度足够覆盖之前的内容
	const minLineLength = 120
	if len(statsLine) < minLineLength {
		padding := make([]byte, minLineLength-len(statsLine))
		for i := range padding {
			padding[i] = ' '
		}
		statsLine += string(padding)
	}
	
	fmt.Print(statsLine)
	l.statsMode = true
	l.lastStatsOutput = time.Now()
}

// LogAudioStats logs audio statistics (保留原有方法以兼容性)
func (l *Logger) LogAudioStats(stats *AudioStats) {
	if l.level > LogLevelInfo {
		return
	}
	
	// 如果处于统计模式，需要换行
	if l.statsMode {
		fmt.Print("\n")
		l.statsMode = false
	}
	
	l.Infof("📊 Audio Stats - Frames: %d, Dropped: %d, Latency: %.2fms, Buffer: %.1f%%, Volume: %.1fdB",
		stats.FramesProcessed,
		stats.DroppedFrames,
		stats.Latency.Seconds()*1000,
		stats.BufferUsage*100,
		stats.DecibelLevel)
}

// LogNetworkStats logs network statistics (保留原有方法以兼容性)
func (l *Logger) LogNetworkStats(stats *NetworkStats) {
	if l.level > LogLevelInfo {
		return
	}
	
	// 如果处于统计模式，需要换行
	if l.statsMode {
		fmt.Print("\n")
		l.statsMode = false
	}
	
	latencyMs := stats.RoundTripTime.Seconds() * 1000
	latencyIndicator := l.getLatencyIndicator(latencyMs)
	
	l.Infof("🌐 Network Stats %s - Sent: %d KB, Received: %d KB, RTT: %.2fms, Errors: %d",
		latencyIndicator,
		stats.BytesSent/1024,
		stats.BytesReceived/1024,
		latencyMs,
		stats.ErrorCount)
}

// AudioStats represents audio processing statistics
type AudioStats struct {
	FramesProcessed int64
	DroppedFrames   int64
	Latency         time.Duration
	BufferUsage     float64
	DecibelLevel    float64 // 新增：当前分贝级别
}

// NetworkStats represents network transmission statistics
type NetworkStats struct {
	BytesSent      int64
	BytesReceived  int64
	RoundTripTime  time.Duration
	ErrorCount     int64
}