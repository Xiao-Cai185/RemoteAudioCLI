// utils/config.go - 正确的修复版本

package utils

import (
	"fmt"
	"time"
)

// Config holds the application configuration
type Config struct {
	// Operating mode: "server" or "client"
	Mode string

	// Network settings
	Host string
	Port int

	// Audio device settings (string identifiers)
	InputDevice  string
	OutputDevice string

	// Audio device objects (使用 interface{} 避免循环导入)
	SelectedInputDevice  interface{}
	SelectedOutputDevice interface{}

	// Audio parameters
	SampleRate    int
	FramesPerBuffer int
	Channels      int
	BitDepth      int

	// Network buffer settings
	BufferSize    int
	BufferCount   int
	ConnTimeout   time.Duration
	ReadTimeout   time.Duration
	WriteTimeout  time.Duration

	// Quality settings
	Compression   bool
	NoiseReduction bool
}

// NewDefaultConfig creates a new configuration with default values
func NewDefaultConfig() *Config {
	return &Config{
		Mode:            "",
		Host:            "localhost",
		Port:            8080,
		InputDevice:     "",
		OutputDevice:    "",
		SelectedInputDevice:  nil,
		SelectedOutputDevice: nil,
		SampleRate:      44100,
		FramesPerBuffer: 1024,
		Channels:        2,
		BitDepth:        16,
		BufferSize:      4096,
		BufferCount:     4,
		ConnTimeout:     10 * time.Second,
		ReadTimeout:     5 * time.Second,
		WriteTimeout:    5 * time.Second,
		Compression:     false,
		NoiseReduction:  false,
	}
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.Mode != "server" && c.Mode != "client" {
		return NewAppError(ErrInvalidConfig, "mode must be 'server' or 'client'")
	}

	if c.Port <= 0 || c.Port > 65535 {
		return NewAppError(ErrInvalidConfig, "port must be between 1 and 65535")
	}

	if c.SampleRate <= 0 {
		return NewAppError(ErrInvalidConfig, "sample rate must be positive")
	}

	if c.FramesPerBuffer <= 0 {
		return NewAppError(ErrInvalidConfig, "frames per buffer must be positive")
	}

	if c.Channels <= 0 || c.Channels > 8 {
		return NewAppError(ErrInvalidConfig, "channels must be between 1 and 8")
	}

	if c.BitDepth != 16 && c.BitDepth != 24 && c.BitDepth != 32 {
		return NewAppError(ErrInvalidConfig, "bit depth must be 16, 24, or 32")
	}

	return nil
}

// GetFrameSize returns the size of one audio frame in bytes
func (c *Config) GetFrameSize() int {
	return c.Channels * (c.BitDepth / 8)
}

// GetBufferSizeInFrames returns the buffer size in audio frames
func (c *Config) GetBufferSizeInFrames() int {
	return c.BufferSize / c.GetFrameSize()
}

// GetNetworkAddress returns the complete network address
func (c *Config) GetNetworkAddress() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}