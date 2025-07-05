// main.go - 修复 PortAudio 初始化时机问题的版本

package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"embed"
	"io/fs"
	"io/ioutil"
	"path/filepath"

	"RemoteAudioCLI/audio"
	"RemoteAudioCLI/network"
	"RemoteAudioCLI/utils"
)

func main() {
	// exportPortAudioDLL()
	exportSoundFiles()

	var (
		mode         = flag.String("mode", "", "Operating mode: 'server' or 'client'")
		host         = flag.String("host", "", "Server host address")
		port         = flag.Int("port", 0, "Server port")
		inputDevice  = flag.String("input-device", "", "Input audio device name or index")
		outputDevice = flag.String("output-device", "", "Output audio device name or index")
		listDevices  = flag.Bool("list-devices", false, "List all available audio devices")
		help         = flag.Bool("help", false, "Show help information")
	)

	flag.Parse()

	// Show help information
	if *help {
		showHelp()
		return
	}

	// Initialize logger
	logger := utils.NewLogger()
	logger.Info("🎵 Remote Audio Go - Starting Application")

	// Initialize audio system EARLY - before any device operations
	if err := audio.Initialize(); err != nil {
		logger.Error(fmt.Sprintf("Failed to initialize audio system: %v", err))
		os.Exit(1)
	}
	defer audio.Terminate()

	// List audio devices if requested (now after initialization)
	if *listDevices {
		listAudioDevices(logger)
		return
	}

	// Create configuration with default values
	config := utils.NewDefaultConfig()
	
	// Check if command line arguments are provided
	hasArgs := (*mode != "" || *host != "" || *port != 0 || *inputDevice != "" || *outputDevice != "")

	if hasArgs {
		// Use command line arguments
		if *mode != "" {
			config.Mode = *mode
		}
		if *host != "" {
			config.Host = *host
		}
		if *port != 0 {
			config.Port = *port
		}
		config.InputDevice = *inputDevice
		config.OutputDevice = *outputDevice

		// If no mode specified even with other args, prompt for mode
		if config.Mode == "" {
			config.Mode = promptModeSelection(logger)
		}
	} else {
		// Interactive mode - prompt for all settings
		logger.Info("🔧 Interactive Setup Mode")
		config = interactiveSetup(logger)
	}

	// Validate mode
	if config.Mode != "server" && config.Mode != "client" {
		logger.Error("Invalid mode. Must be 'server' or 'client'")
		os.Exit(1)
	}

	logger.Info(fmt.Sprintf("Operating in %s mode", strings.ToUpper(config.Mode)))

	// Setup signal handling for graceful shutdown
	setupSignalHandling(logger)

	// Start server or client based on mode
	switch config.Mode {
	case "server":
		startServer(config, logger)
	case "client":
		startClient(config, logger)
	}
}
// //go:embed dll/libportaudio.dll
// var portAudioDLL []byte

//go:embed sound/*.mp3
var soundFiles embed.FS

// // 释放 libportaudio.dll
// func exportPortAudioDLL() {
// 	exePath, err := os.Executable()
// 	if err != nil {
// 		fmt.Printf("Failed to locate executable path: %v\n", err)
// 		os.Exit(1)
// 	}
// 	exePath, err = filepath.EvalSymlinks(exePath)
// 	if err != nil {
// 		fmt.Printf("Failed to resolve executable path: %v\n", err)
// 		os.Exit(1)
// 	}
// 	exeDir := filepath.Dir(exePath)
// 	dllPath := filepath.Join(exeDir, "libportaudio.dll")

// 	fmt.Printf("Extracting DLL to: %s\n", dllPath)

// 	err = ioutil.WriteFile(dllPath, portAudioDLL, 0644)
// 	if err != nil {
// 		fmt.Printf("Failed to extract libportaudio.dll: %v\n", err)
// 		os.Exit(1)
// 	}
// }



// 释放 sound 目录下的 mp3
func exportSoundFiles() {
	exePath, err := os.Executable()
	if err != nil {
		fmt.Printf("Failed to locate executable path: %v\n", err)
		return
	}
	exeDir := filepath.Dir(exePath)
	soundDir := filepath.Join(exeDir, "sound")

	err = os.MkdirAll(soundDir, 0755)
	if err != nil {
		fmt.Printf("Failed to create sound directory: %v\n", err)
		return
	}

	entries, err := fs.Glob(soundFiles, "sound/*.mp3")
	if err != nil {
		fmt.Printf("Failed to glob embedded sound files: %v\n", err)
		return
	}
	for _, file := range entries {
		data, err := soundFiles.ReadFile(file)
		if err != nil {
			fmt.Printf("Failed to read embedded sound file %s: %v\n", file, err)
			continue
		}
		target := filepath.Join(soundDir, filepath.Base(file))
		err = ioutil.WriteFile(target, data, 0644)
		if err != nil {
			fmt.Printf("Failed to write sound file %s: %v\n", target, err)
		}
	}
}


// interactiveSetup 交互式设置配置
func interactiveSetup(logger *utils.Logger) *utils.Config {
	config := utils.NewDefaultConfig()
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("")
	fmt.Println("🔧 Interactive Configuration Setup")
	fmt.Println("==================================")

	// Step 1: Select mode
	config.Mode = promptModeSelection(logger)

	if config.Mode == "server" {
		// Server setup
		fmt.Println("")
		fmt.Println("🔊 Server Configuration")
		fmt.Println("=======================")

		// Step 2: Select output device
		outputDevice := promptOutputDevice(logger)
		if outputDevice != nil {
			// 使用 interface{} 存储，避免类型问题
			config.SelectedOutputDevice = outputDevice
		}

		// Step 3: Set server port
		config.Port = promptServerPort(logger, reader)

		// Set default host for server
		config.Host = "0.0.0.0" // Listen on all interfaces

	} else {
		// Client setup
		fmt.Println("")
		fmt.Println("🎤 Client Configuration")
		fmt.Println("=======================")

		// Step 2: Enter server IP
		config.Host = promptServerIP(logger, reader)

		// Step 3: Enter server port
		config.Port = promptServerPort(logger, reader)

		// Step 4: Select input device
		inputDevice := promptInputDevice(logger)
		if inputDevice != nil {
			// 使用 interface{} 存储，避免类型问题
			config.SelectedInputDevice = inputDevice
		}
	}

	fmt.Println("")
	fmt.Println("✅ Configuration completed!")
	fmt.Printf("   Mode: %s\n", config.Mode)
	if config.Mode == "server" {
		fmt.Printf("   Listen on: %s:%d\n", config.Host, config.Port)
		if config.SelectedOutputDevice != nil {
			if device, ok := config.SelectedOutputDevice.(*audio.DeviceInfo); ok {
				fmt.Printf("   Output device: %s\n", device.Name)
			}
		}
	} else {
		fmt.Printf("   Server: %s:%d\n", config.Host, config.Port)
		if config.SelectedInputDevice != nil {
			if device, ok := config.SelectedInputDevice.(*audio.DeviceInfo); ok {
				fmt.Printf("   Input device: %s\n", device.Name)
			}
		}
	}

	return config
}

// promptModeSelection 询问操作模式
func promptModeSelection(logger *utils.Logger) string {
	fmt.Println("")
	fmt.Println("📡 Select Operating Mode:")
	fmt.Println("  1. Server (Receive and play audio)")
	fmt.Println("  2. Client (Capture and send audio)")

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("Enter your choice (1 or 2): ")
		input, err := reader.ReadString('\n')
		if err != nil {
			logger.Error(fmt.Sprintf("Error reading input: %v", err))
			continue
		}

		choice := strings.TrimSpace(input)
		switch choice {
		case "1":
			return "server"
		case "2":
			return "client"
		default:
			fmt.Println("❌ Invalid choice. Please enter 1 or 2.")
		}
	}
}

// promptOutputDevice 询问输出设备
func promptOutputDevice(logger *utils.Logger) *audio.DeviceInfo {
	fmt.Println("")
	fmt.Println("🔊 Available Output Devices:")
	fmt.Println("============================")

	devices, err := audio.ListDevices()
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to list audio devices: %v", err))
		return nil
	}

	outputDevices := []audio.DeviceInfo{}
	displayIndex := 0

	// 构建输出设备列表
	for _, device := range devices {
		if device.MaxOutputChannels > 0 {
			defaultMark := ""
			if device.IsDefaultOutput {
				defaultMark = " (DEFAULT)"
			}
			fmt.Printf("  [%d] %s%s\n", displayIndex, device.Name, defaultMark)
			fmt.Printf("      Channels: %d, Sample Rate: %.0f Hz, Host API: %s\n",
				device.MaxOutputChannels, device.DefaultSampleRate, device.HostAPI)
			
			outputDevices = append(outputDevices, device)
			displayIndex++
		}
	}

	if len(outputDevices) == 0 {
		fmt.Println("  ❌ No output devices found")
		return nil
	}

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("Enter output device number (or press Enter for default): ")
		input, err := reader.ReadString('\n')
		if err != nil {
			logger.Error(fmt.Sprintf("Error reading input: %v", err))
			continue
		}

		input = strings.TrimSpace(input)
		
		if input == "" {
			// Use default device
			for _, device := range outputDevices {
				if device.IsDefaultOutput {
					logger.Info(fmt.Sprintf("Using default output device: %s", device.Name))
					return &device
				}
			}
			// If no default found, use first device
			if len(outputDevices) > 0 {
				logger.Info(fmt.Sprintf("Using first available output device: %s", outputDevices[0].Name))
				return &outputDevices[0]
			}
		}

		if index, err := strconv.Atoi(input); err == nil {
			if index >= 0 && index < len(outputDevices) {
				selected := outputDevices[index]
				logger.Info(fmt.Sprintf("Selected output device [%d]: %s", index, selected.Name))
				return &selected
			}
		}

		fmt.Printf("❌ Invalid device number. Please enter 0-%d.\n", len(outputDevices)-1)
	}
}

// promptInputDevice 询问输入设备
func promptInputDevice(logger *utils.Logger) *audio.DeviceInfo {
	fmt.Println("")
	fmt.Println("🎤 Available Input Devices:")
	fmt.Println("===========================")

	devices, err := audio.ListDevices()
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to list audio devices: %v", err))
		return nil
	}

	inputDevices := []audio.DeviceInfo{}
	displayIndex := 0

	// 构建输入设备列表
	for _, device := range devices {
		if device.MaxInputChannels > 0 {
			defaultMark := ""
			if device.IsDefaultInput {
				defaultMark = " (DEFAULT)"
			}
			fmt.Printf("  [%d] %s%s\n", displayIndex, device.Name, defaultMark)
			fmt.Printf("      Channels: %d, Sample Rate: %.0f Hz, Host API: %s\n",
				device.MaxInputChannels, device.DefaultSampleRate, device.HostAPI)
			
			inputDevices = append(inputDevices, device)
			displayIndex++
		}
	}

	if len(inputDevices) == 0 {
		fmt.Println("  ❌ No input devices found")
		return nil
	}

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("Enter input device number (or press Enter for default): ")
		input, err := reader.ReadString('\n')
		if err != nil {
			logger.Error(fmt.Sprintf("Error reading input: %v", err))
			continue
		}

		input = strings.TrimSpace(input)
		
		if input == "" {
			// Use default device
			for _, device := range inputDevices {
				if device.IsDefaultInput {
					logger.Info(fmt.Sprintf("Using default input device: %s", device.Name))
					return &device
				}
			}
			// If no default found, use first device
			if len(inputDevices) > 0 {
				logger.Info(fmt.Sprintf("Using first available input device: %s", inputDevices[0].Name))
				return &inputDevices[0]
			}
		}

		if index, err := strconv.Atoi(input); err == nil {
			if index >= 0 && index < len(inputDevices) {
				selected := inputDevices[index]
				logger.Info(fmt.Sprintf("Selected input device [%d]: %s", index, selected.Name))
				return &selected
			}
		}

		fmt.Printf("❌ Invalid device number. Please enter 0-%d.\n", len(inputDevices)-1)
	}
}

// promptServerIP 询问服务器IP
func promptServerIP(logger *utils.Logger, reader *bufio.Reader) string {
	fmt.Println("")
	for {
		fmt.Print("Enter server IP address (default: localhost): ")
		input, err := reader.ReadString('\n')
		if err != nil {
			logger.Error(fmt.Sprintf("Error reading input: %v", err))
			continue
		}

		input = strings.TrimSpace(input)
		if input == "" {
			input = "localhost"
		}

		// Basic validation
		if input != "" {
			logger.Info(fmt.Sprintf("Server IP set to: %s", input))
			return input
		}

		fmt.Println("❌ Please enter a valid IP address.")
	}
}

// promptServerPort 询问服务器端口
func promptServerPort(logger *utils.Logger, reader *bufio.Reader) int {
	fmt.Println("")
	for {
		fmt.Print("Enter server port (default: 8080): ")
		input, err := reader.ReadString('\n')
		if err != nil {
			logger.Error(fmt.Sprintf("Error reading input: %v", err))
			continue
		}

		input = strings.TrimSpace(input)
		if input == "" {
			logger.Info("Using default port: 8080")
			return 8080
		}

		if port, err := strconv.Atoi(input); err == nil {
			if port > 0 && port <= 65535 {
				logger.Info(fmt.Sprintf("Server port set to: %d", port))
				return port
			}
		}

		fmt.Println("❌ Please enter a valid port number (1-65535).")
	}
}

// setupSignalHandling 设置信号处理，用于优雅关闭
func setupSignalHandling(logger *utils.Logger) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		logger.Info("\n🛑 Received shutdown signal, gracefully stopping...")
		
		// Trigger cleanup
		network.NotifyShutdown()
		
		logger.Info("✅ Shutdown complete")
		os.Exit(0)
	}()
}

func showHelp() {
	fmt.Println("🎵 Remote Audio Go - Real-time Audio Streaming")
	fmt.Println("")
	fmt.Println("USAGE:")
	fmt.Println("  RemoteAudioCli [OPTIONS]")
	fmt.Println("")
	fmt.Println("OPTIONS:")
	fmt.Println("  -mode string")
	fmt.Println("        Operating mode: 'server' or 'client'")
	fmt.Println("  -host string")
	fmt.Println("        Server host address (default: localhost)")
	fmt.Println("  -port int")
	fmt.Println("        Server port (default: 8080)")
	fmt.Println("  -input-device string")
	fmt.Println("        Input audio device name or index (client mode)")
	fmt.Println("  -output-device string")
	fmt.Println("        Output audio device name or index (server mode)")
	fmt.Println("  -list-devices")
	fmt.Println("        List all available audio devices")
	fmt.Println("  -help")
	fmt.Println("        Show this help information")
	fmt.Println("")
	fmt.Println("INTERACTIVE MODE:")
	fmt.Println("  Run without arguments for interactive setup:")
	fmt.Println("  RemoteAudioCli")
	fmt.Println("")
	fmt.Println("EXAMPLES:")
	fmt.Println("  # Interactive mode")
	fmt.Println("  RemoteAudioCli")
	fmt.Println("")
	fmt.Println("  # Start server on port 8080")
	fmt.Println("  RemoteAudioCli -mode=server -port=8080")
	fmt.Println("")
	fmt.Println("  # Connect client to server")
	fmt.Println("  RemoteAudioCli -mode=client -host=192.168.1.100 -port=8080")
	fmt.Println("")
	fmt.Println("  # List available audio devices")
	fmt.Println("  RemoteAudioCli -list-devices")
}

func listAudioDevices(logger *utils.Logger) {
	logger.Info("📋 Listing Available Audio Devices")

	devices, err := audio.ListDevices()
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to list audio devices: %v", err))
		return
	}

	fmt.Println("")
	fmt.Println("🎤 INPUT DEVICES:")
	inputCount := 0
	for i, device := range devices {
		if device.MaxInputChannels > 0 {
			defaultMark := ""
			if device.IsDefaultInput {
				defaultMark = " (DEFAULT)"
			}
			fmt.Printf("  [%d] %s%s\n", i, device.Name, defaultMark)
			fmt.Printf("      Channels: %d, Sample Rate: %.0f Hz, Host API: %s\n",
				device.MaxInputChannels, device.DefaultSampleRate, device.HostAPI)
			inputCount++
		}
	}
	if inputCount == 0 {
		fmt.Println("  No input devices found")
	}

	fmt.Println("")
	fmt.Println("🔊 OUTPUT DEVICES:")
	outputCount := 0
	for i, device := range devices {
		if device.MaxOutputChannels > 0 {
			defaultMark := ""
			if device.IsDefaultOutput {
				defaultMark = " (DEFAULT)"
			}
			fmt.Printf("  [%d] %s%s\n", i, device.Name, defaultMark)
			fmt.Printf("      Channels: %d, Sample Rate: %.0f Hz, Host API: %s\n",
				device.MaxOutputChannels, device.DefaultSampleRate, device.HostAPI)
			outputCount++
		}
	}
	if outputCount == 0 {
		fmt.Println("  No output devices found")
	}
	fmt.Println("")
}

func startServer(config *utils.Config, logger *utils.Logger) {
	logger.Info(fmt.Sprintf("🖧 Starting server on %s:%d", config.Host, config.Port))

	var outputDevice *audio.DeviceInfo
	var err error

	// 检查是否有交互式选择的设备
	if config.SelectedOutputDevice != nil {
		// 类型断言，将 interface{} 转换为 *audio.DeviceInfo
		if device, ok := config.SelectedOutputDevice.(*audio.DeviceInfo); ok {
			outputDevice = device
			logger.Info(fmt.Sprintf("Using selected output device: %s", outputDevice.Name))
		} else {
			logger.Error("Invalid selected output device type")
			os.Exit(1)
		}
	} else {
		// 使用命令行指定的设备或默认设备
		outputDevice, err = getOutputDevice(config.OutputDevice, logger)
		if err != nil {
			logger.Error(fmt.Sprintf("Failed to get output device: %v", err))
			os.Exit(1)
		}
	}

	// Create and start server
	server := network.NewServer(config, logger)
	if err := server.Start(outputDevice); err != nil {
		logger.Error(fmt.Sprintf("Server failed: %v", err))
		os.Exit(1)
	}
}

func startClient(config *utils.Config, logger *utils.Logger) {
	logger.Info(fmt.Sprintf("🖥️ Starting client, connecting to %s:%d", config.Host, config.Port))

	var inputDevice *audio.DeviceInfo
	var err error

	// 检查是否有交互式选择的设备
	if config.SelectedInputDevice != nil {
		// 类型断言，将 interface{} 转换为 *audio.DeviceInfo
		if device, ok := config.SelectedInputDevice.(*audio.DeviceInfo); ok {
			inputDevice = device
			logger.Info(fmt.Sprintf("Using selected input device: %s", inputDevice.Name))
		} else {
			logger.Error("Invalid selected input device type")
			os.Exit(1)
		}
	} else {
		// 使用命令行指定的设备或默认设备
		inputDevice, err = getInputDevice(config.InputDevice, logger)
		if err != nil {
			logger.Error(fmt.Sprintf("Failed to get input device: %v", err))
			os.Exit(1)
		}
	}

	// Create and start client
	client := network.NewClient(config, logger)
	if err := client.Start(inputDevice); err != nil {
		logger.Error(fmt.Sprintf("Client failed: %v", err))
		os.Exit(1)
	}
}

// getInputDevice 获取输入设备 - 改进错误处理和设备索引验证
func getInputDevice(deviceSpec string, logger *utils.Logger) (*audio.DeviceInfo, error) {
	devices, err := audio.ListDevices()
	if err != nil {
		return nil, err
	}

	// If no device specified, use default input device
	if deviceSpec == "" {
		defaultDevice, err := audio.GetDefaultInputDevice()
		if err != nil {
			return nil, err
		}
		logger.Info(fmt.Sprintf("Using default input device: %s", defaultDevice.Name))
		return defaultDevice, nil
	}

	// Try to parse as device index
	if index, err := strconv.Atoi(deviceSpec); err == nil {
		// Validate index range
		if index < 0 || index >= len(devices) {
			return nil, fmt.Errorf("device index %d out of range (0-%d)", index, len(devices)-1)
		}
		
		// Check if device has input channels
		if devices[index].MaxInputChannels <= 0 {
			return nil, fmt.Errorf("device [%d] %s has no input channels", index, devices[index].Name)
		}
		
		logger.Info(fmt.Sprintf("Using input device [%d]: %s", index, devices[index].Name))
		return &devices[index], nil
	}

	// Try to find by name
	for i, device := range devices {
		if device.MaxInputChannels > 0 && strings.Contains(strings.ToLower(device.Name), strings.ToLower(deviceSpec)) {
			logger.Info(fmt.Sprintf("Using input device [%d]: %s", i, device.Name))
			return &device, nil
		}
	}

	return nil, fmt.Errorf("input device not found: %s", deviceSpec)
}

// getOutputDevice 获取输出设备 - 改进错误处理和设备索引验证
func getOutputDevice(deviceSpec string, logger *utils.Logger) (*audio.DeviceInfo, error) {
	devices, err := audio.ListDevices()
	if err != nil {
		return nil, err
	}

	// If no device specified, use default output device
	if deviceSpec == "" {
		defaultDevice, err := audio.GetDefaultOutputDevice()
		if err != nil {
			return nil, err
		}
		logger.Info(fmt.Sprintf("Using default output device: %s", defaultDevice.Name))
		return defaultDevice, nil
	}

	// Try to parse as device index
	if index, err := strconv.Atoi(deviceSpec); err == nil {
		// Validate index range
		if index < 0 || index >= len(devices) {
			return nil, fmt.Errorf("device index %d out of range (0-%d)", index, len(devices)-1)
		}
		
		// Check if device has output channels
		if devices[index].MaxOutputChannels <= 0 {
			return nil, fmt.Errorf("device [%d] %s has no output channels", index, devices[index].Name)
		}
		
		logger.Info(fmt.Sprintf("Using output device [%d]: %s", index, devices[index].Name))
		return &devices[index], nil
	}

	// Try to find by name
	for i, device := range devices {
		if device.MaxOutputChannels > 0 && strings.Contains(strings.ToLower(device.Name), strings.ToLower(deviceSpec)) {
			logger.Info(fmt.Sprintf("Using output device [%d]: %s", i, device.Name))
			return &device, nil
		}
	}

	return nil, fmt.Errorf("output device not found: %s", deviceSpec)
}