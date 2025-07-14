// ===============================================
// audio/device.go - 修复版本
// ===============================================

package audio

import (
	"fmt"

	"github.com/gordonklaus/portaudio"
	"RemoteAudioCLI/utils"
)

// DeviceInfo represents information about an audio device
type DeviceInfo struct {
	Index              int
	Name               string
	MaxInputChannels   int
	MaxOutputChannels  int
	DefaultSampleRate  float64
	HostAPI            string
	IsDefaultInput     bool
	IsDefaultOutput    bool
}

// AudioSystem manages the PortAudio system
var audioSystemInitialized = false

// Initialize initializes the PortAudio system
func Initialize() error {
	if audioSystemInitialized {
		return nil
	}

	if err := portaudio.Initialize(); err != nil {
		return utils.WrapError(err, utils.ErrAudioDevice, "failed to initialize PortAudio")
	}

	audioSystemInitialized = true
	return nil
}

// Terminate terminates the PortAudio system
func Terminate() error {
	if !audioSystemInitialized {
		return nil
	}

	if err := portaudio.Terminate(); err != nil {
		return utils.WrapError(err, utils.ErrAudioDevice, "failed to terminate PortAudio")
	}

	audioSystemInitialized = false
	return nil
}

// ListDevices returns a list of all available audio devices
func ListDevices() ([]DeviceInfo, error) {
	if !audioSystemInitialized {
		return nil, utils.NewAppError(utils.ErrAudioDevice, "PortAudio not initialized")
	}

	devices, err := portaudio.Devices()
	if err != nil {
		return nil, utils.WrapError(err, utils.ErrAudioDevice, "failed to enumerate audio devices")
	}

	defaultInputDevice, err := portaudio.DefaultInputDevice()
	if err != nil {
		// Log warning but continue
		defaultInputDevice = nil
	}

	defaultOutputDevice, err := portaudio.DefaultOutputDevice()
	if err != nil {
		// Log warning but continue
		defaultOutputDevice = nil
	}

	var deviceList []DeviceInfo
	for i, device := range devices {
		// 修复：直接访问 HostApi 字段而不是调用方法
		hostAPI := device.HostApi
		var hostAPIName string
		if hostAPI != nil {
			hostAPIName = hostAPI.Name
		} else {
			hostAPIName = "Unknown"
		}

		isDefaultInput := defaultInputDevice != nil && device == defaultInputDevice
		isDefaultOutput := defaultOutputDevice != nil && device == defaultOutputDevice

		deviceInfo := DeviceInfo{
			Index:              i,
			Name:               device.Name,
			MaxInputChannels:   device.MaxInputChannels,
			MaxOutputChannels:  device.MaxOutputChannels,
			DefaultSampleRate:  device.DefaultSampleRate,
			HostAPI:            hostAPIName,
			IsDefaultInput:     isDefaultInput,
			IsDefaultOutput:    isDefaultOutput,
		}
		deviceList = append(deviceList, deviceInfo)
	}

	return deviceList, nil
}

// GetDefaultInputDevice returns the default input device
func GetDefaultInputDevice() (*DeviceInfo, error) {
	if !audioSystemInitialized {
		return nil, utils.NewAppError(utils.ErrAudioDevice, "PortAudio not initialized")
	}

	device, err := portaudio.DefaultInputDevice()
	if err != nil {
		return nil, utils.WrapError(err, utils.ErrAudioDevice, "failed to get default input device")
	}

	if device.MaxInputChannels == 0 {
		return nil, utils.NewAppError(utils.ErrAudioDevice, "default input device has no input channels")
	}

	// 修复：直接访问 HostApi 字段
	hostAPI := device.HostApi
	var hostAPIName string
	if hostAPI != nil {
		hostAPIName = hostAPI.Name
	} else {
		hostAPIName = "Unknown"
	}

	devices, _ := portaudio.Devices()
	var deviceIndex int
	for i, d := range devices {
		if d == device {
			deviceIndex = i
			break
		}
	}

	return &DeviceInfo{
		Index:              deviceIndex,
		Name:               device.Name,
		MaxInputChannels:   device.MaxInputChannels,
		MaxOutputChannels:  device.MaxOutputChannels,
		DefaultSampleRate:  device.DefaultSampleRate,
		HostAPI:            hostAPIName,
		IsDefaultInput:     true,
		IsDefaultOutput:    false,
	}, nil
}

// GetDefaultOutputDevice returns the default output device
func GetDefaultOutputDevice() (*DeviceInfo, error) {
	if !audioSystemInitialized {
		return nil, utils.NewAppError(utils.ErrAudioDevice, "PortAudio not initialized")
	}

	device, err := portaudio.DefaultOutputDevice()
	if err != nil {
		return nil, utils.WrapError(err, utils.ErrAudioDevice, "failed to get default output device")
	}

	if device.MaxOutputChannels == 0 {
		return nil, utils.NewAppError(utils.ErrAudioDevice, "default output device has no output channels")
	}

	// 修复：直接访问 HostApi 字段
	hostAPI := device.HostApi
	var hostAPIName string
	if hostAPI != nil {
		hostAPIName = hostAPI.Name
	} else {
		hostAPIName = "Unknown"
	}

	devices, _ := portaudio.Devices()
	var deviceIndex int
	for i, d := range devices {
		if d == device {
			deviceIndex = i
			break
		}
	}

	return &DeviceInfo{
		Index:              deviceIndex,
		Name:               device.Name,
		MaxInputChannels:   device.MaxInputChannels,
		MaxOutputChannels:  device.MaxOutputChannels,
		DefaultSampleRate:  device.DefaultSampleRate,
		HostAPI:            hostAPIName,
		IsDefaultInput:     false,
		IsDefaultOutput:    true,
	}, nil
}

// GetDeviceByIndex returns a device by its index
func GetDeviceByIndex(index int) (*DeviceInfo, error) {
	devices, err := ListDevices()
	if err != nil {
		return nil, err
	}

	if index < 0 || index >= len(devices) {
		return nil, utils.NewAppError(utils.ErrAudioDevice, fmt.Sprintf("invalid device index: %d", index))
	}

	return &devices[index], nil
}

// GetPortAudioDevice returns the actual PortAudio device for a DeviceInfo
func GetPortAudioDevice(deviceInfo *DeviceInfo) (*portaudio.DeviceInfo, error) {
	if !audioSystemInitialized {
		return nil, utils.NewAppError(utils.ErrAudioDevice, "PortAudio not initialized")
	}

	devices, err := portaudio.Devices()
	if err != nil {
		return nil, utils.WrapError(err, utils.ErrAudioDevice, "failed to enumerate PortAudio devices")
	}

	if deviceInfo.Index < 0 || deviceInfo.Index >= len(devices) {
		return nil, utils.NewAppError(utils.ErrAudioDevice, fmt.Sprintf("invalid device index: %d", deviceInfo.Index))
	}

	return devices[deviceInfo.Index], nil
}

// ValidateDeviceForInput checks if a device is suitable for input
func ValidateDeviceForInput(deviceInfo *DeviceInfo, sampleRate int, channels int) error {
	if deviceInfo.MaxInputChannels == 0 {
		return utils.NewAppError(utils.ErrAudioDevice, "device has no input channels")
	}

	if deviceInfo.MaxInputChannels < channels {
		return utils.NewAppError(utils.ErrAudioDevice, 
			fmt.Sprintf("device has only %d input channels, but %d requested", 
			deviceInfo.MaxInputChannels, channels))
	}

	// Check if sample rate is supported (basic check)
	if sampleRate <= 0 {
		return utils.NewAppError(utils.ErrAudioDevice, "invalid sample rate")
	}

	return nil
}

// ValidateDeviceForOutput checks if a device is suitable for output
func ValidateDeviceForOutput(deviceInfo *DeviceInfo, sampleRate int, channels int) error {
	if deviceInfo.MaxOutputChannels == 0 {
		return utils.NewAppError(utils.ErrAudioDevice, "device has no output channels")
	}

	if deviceInfo.MaxOutputChannels < channels {
		return utils.NewAppError(utils.ErrAudioDevice, 
			fmt.Sprintf("device has only %d output channels, but %d requested", 
			deviceInfo.MaxOutputChannels, channels))
	}

	// Check if sample rate is supported (basic check)
	if sampleRate <= 0 {
		return utils.NewAppError(utils.ErrAudioDevice, "invalid sample rate")
	}

	return nil
}
