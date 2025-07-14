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
	"sync/atomic"
	"syscall"
	"embed"
	"io/fs"
	"io/ioutil"
	"path/filepath"
	"time"

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
		quality      = flag.String("quality", "normal", "Stream quality: verylow, low, normal, high, lossless")
		compress     = flag.String("compress", "", "Compression mode: 'yes' (Opus) or 'no' (PCM)")
		excitation   = flag.Bool("excitation", false, "Enable excitation mode (pause streaming when silent)")
		excitationThreshold = flag.Float64("excitation-threshold", -45.0, "Excitation threshold in dB")
		excitationTimeout   = flag.Int("excitation-timeout", 10, "Excitation timeout in seconds")
		allowClient = flag.String("allow-client", "", "Comma-separated list of allowed client IPs (whitelist, default: allow all)")
	)

	flag.Parse()

	// Show help information
	if *help {
		showHelp()
		return
	}

	// Initialize logger
	logger := utils.NewLogger()
	logger.Info("🎵 Remote Audio CLI - Starting Application")

	// Initialize audio system EARLY - before any device operations
	if err := audio.Initialize(); err != nil {
		logger.Error(fmt.Sprintf("Failed to initialize audio system: %v", err))
		gracefulExitWithCode(logger, 1)
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

		config.StreamQuality = parseQualityArg(*quality)
		applyQualityParams(config)
		config.Compression = parseCompressionArg(*compress)
		config.EnableExcitation = *excitation
		config.ExcitationThreshold = *excitationThreshold
		config.ExcitationTimeout = *excitationTimeout
		if *allowClient != "" {
			ips := strings.Split(*allowClient, ",")
			for i := range ips {
				ips[i] = strings.TrimSpace(ips[i])
			}
			config.AllowClients = ips
		}
	} else {
		// Interactive mode - prompt for all settings
		logger.Info("🔧 Interactive Setup Mode")
		config = interactiveSetup(logger)
	}

	// Validate mode
	if config.Mode != "server" && config.Mode != "client" {
		logger.Error("Invalid mode. Must be 'server' or 'client'")
		gracefulExitWithCode(logger, 1)
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
	
	// 如果程序执行到这里，说明服务端或客户端已经正常退出
	// 检查是否已经在关闭过程中
	if atomic.LoadInt32(&isShuttingDown) == 0 {
		logger.Info("🔄 Service stopped, preparing to exit...")
		gracefulExit(logger)
	} else {
		// 如果已经在关闭过程中，等待信号处理完成
		logger.Debug("Shutdown already in progress, waiting for completion...")
		// 等待足够时间让信号处理完成
		time.Sleep(10 * time.Second)
	}
}

//go:embed sound/*.mp3
var soundFiles embed.FS

// 全局变量用于管理退出状态
var (
	isShuttingDown int32 // atomic bool
)


// gracefulExit 优雅退出函数，带倒计时
func gracefulExit(logger *utils.Logger) {
	gracefulExitWithCode(logger, 0)
}

// gracefulExitWithCode 带退出码的优雅退出函数
func gracefulExitWithCode(logger *utils.Logger, exitCode int) {
	// 使用 CompareAndSwap 确保只有一个 goroutine 执行倒计时
	if atomic.CompareAndSwapInt32(&isShuttingDown, 0, 1) {
		logger.Info("✅ Shutdown complete")
		
		if exitCode == 0 {
			logger.Info("🔚 The program will exit after 5 seconds...")
		} else {
			logger.Error(fmt.Sprintf("❌ Error occurred (code: %d), the program will exit after 5 seconds...", exitCode))
		}
		for i := 5; i > 0; i-- {
			logger.Info(fmt.Sprintf("⏰ Exiting in %d seconds...", i))
			time.Sleep(1 * time.Second)
		}
		
		if exitCode == 0 {
			logger.Info("👋 Goodbye!")
		} else {
			logger.Error("💥 Program terminated due to error")
		}
		os.Exit(exitCode)
	} else {
		// 如果已经在关闭过程中，等待一下再退出
		logger.Debug("Already in shutdown process, waiting...")
		time.Sleep(100 * time.Millisecond)
	}
}

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

		// Step 4: Set allowed client IPs (whitelist)
		config.AllowClients = promptAllowedClientIPs(logger)

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
		// Step 5: Select stream quality
		config.StreamQuality = promptStreamQuality(logger)
		if config.StreamQuality == "custom" {
			promptCustomAudioParams(config, logger)
		}
		applyQualityParams(config)
		
		// Step 6: Select compression mode
		config.Compression = promptCompressionMode(logger)
		
		// Step 7: Enable excitation streaming?
		config.EnableExcitation = promptEnableExcitation(logger)
		if config.EnableExcitation {
			config.ExcitationTimeout = promptExcitationTimeout(logger)
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
		if len(config.AllowClients) > 0 {
			fmt.Printf("   Allowed Clients: %s\n", strings.Join(config.AllowClients, ", "))
		}
	} else {
		fmt.Printf("   Server: %s:%d\n", config.Host, config.Port)
		if config.SelectedInputDevice != nil {
			if device, ok := config.SelectedInputDevice.(*audio.DeviceInfo); ok {
				fmt.Printf("   Input device: %s\n", device.Name)
			}
		}
		fmt.Printf("   Quality: %s\n", config.StreamQuality)
		fmt.Printf("   Compression: %s\n", getCompressionModeName(config.Compression))
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
		
		// 立即触发网络模块关闭，执行程序终止操作
		network.NotifyShutdown()
		
		// 等待网络模块完全停止
		logger.Info("⏳ Waiting for services to stop...")
		time.Sleep(2 * time.Second) // 给服务端/客户端足够时间停止
		
		// 然后进行倒计时退出
		gracefulExit(logger)
	}()
}

func showHelp() {
	fmt.Println("🎵 Remote Audio CLI - Real-time Audio Streaming")
	fmt.Println("")
	fmt.Println("USAGE:")
	fmt.Println("  RemoteAudioCLI [OPTIONS]")
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
	fmt.Println("  -quality string")
	fmt.Println("        Stream quality: verylow, low, normal, high, lossless (default: normal)")
	fmt.Println("  -compress string")
	fmt.Println("        Compression mode: 'yes' (Opus) or 'no' (PCM) (default: yes)")
	fmt.Println("  -excitation")
	fmt.Println("        Enable excitation mode (pause streaming when silent)")
	fmt.Println("  -excitation-threshold float")
	fmt.Println("        Excitation threshold in dB (default: -45.0)")
	fmt.Println("  -excitation-timeout int")
	fmt.Println("        Excitation timeout in seconds (default: 10)")
	fmt.Println("  -allow-client string")
	fmt.Println("        Comma-separated list of allowed client IPs (whitelist, default: allow all)")
	fmt.Println("")
	fmt.Println("INTERACTIVE MODE:")
	fmt.Println("  Run without arguments for interactive setup:")
	fmt.Println("  RemoteAudioCLI")
	fmt.Println("")
	fmt.Println("EXAMPLES:")
	fmt.Println("  # Interactive mode")
	fmt.Println("  RemoteAudioCLI")
	fmt.Println("")
	fmt.Println("  # Start server on port 8080")
	fmt.Println("  RemoteAudioCLI -mode=server -port=8080")
	fmt.Println("")
	fmt.Println("  # Connect client to server")
	fmt.Println("  RemoteAudioCLI -mode=client -host=\"192.168.1.100\" -port=8080")
	fmt.Println("")
	fmt.Println("  # Connect with specific quality and compression")
	fmt.Println("  RemoteAudioCLI -mode=client -host=\"192.168.1.100\" -port=8080 -quality=high -compress=yes")
	fmt.Println("")
	fmt.Println("  # Connect with PCM uncompressed audio")
	fmt.Println("  RemoteAudioCLI -mode=client -host=\"192.168.1.100\" -port=8080 -quality=lossless -compress=no")
	fmt.Println("")
	fmt.Println("  # List available audio devices")
	fmt.Println("  RemoteAudioCLI -list-devices")
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
		if device, ok := config.SelectedOutputDevice.(*audio.DeviceInfo); ok {
			outputDevice = device
			logger.Info(fmt.Sprintf("Using selected output device: %s", outputDevice.Name))
		} else {
			logger.Error("Invalid selected output device type")
			gracefulExitWithCode(logger, 1)
		}
	} else {
		outputDevice, err = getOutputDevice(config.OutputDevice, logger)
		if err != nil {
			logger.Error(fmt.Sprintf("Failed to get output device: %v", err))
			gracefulExitWithCode(logger, 1)
		}
	}

	// Create and start server
	server := network.NewServer(config, logger)
	if err := server.Start(outputDevice); err != nil {
		logger.Error(fmt.Sprintf("Server failed: %v", err))
		gracefulExitWithCode(logger, 1)
	}
}

// 在 startClient 里捕获 capturer 初始化失败时自动回退 bit depth
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
			gracefulExitWithCode(logger, 1)
		}
	} else {
		// 使用命令行指定的设备或默认设备
		inputDevice, err = getInputDevice(config.InputDevice, logger)
		if err != nil {
			logger.Error(fmt.Sprintf("Failed to get input device: %v", err))
			gracefulExitWithCode(logger, 1)
		}
	}

	client := network.NewClient(config, logger)
	// 捕获 bit depth 24 不支持时自动回退
	retry := false
	for {
		err = client.Start(inputDevice)
		if err == nil {
			break
		}
		if strings.Contains(err.Error(), "unsupported bit depth: 24") && config.BitDepth == 24 && !retry {
			logger.Warn("24-bit audio not supported by device, falling back to 16-bit.")
			config.BitDepth = 16
			retry = true
			continue
		}
		logger.Error(fmt.Sprintf("Client failed: %v", err))
		gracefulExitWithCode(logger, 1)
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

func promptStreamQuality(logger *utils.Logger) string {
	fmt.Println("")
	fmt.Println("🎚️  Select Stream Quality:")
	fmt.Println("  1. Very Low (lowest bandwidth, 8000Hz, 16bit)")
	fmt.Println("  2. Low (low bandwidth, 16000Hz, 16bit)")
	fmt.Println("  3. Normal (default, 24000Hz, 16bit)")
	fmt.Println("  4. High (higher quality, 48000Hz, 16bit)")
	fmt.Println("  5. Lossless (best quality, 48000Hz, 24bit)")
	fmt.Println("  6. Custom (user defined)")
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("Enter your choice (1-6, default 3): ")
		input, err := reader.ReadString('\n')
		if err != nil {
			logger.Error(fmt.Sprintf("Error reading input: %v", err))
			continue
		}
		input = strings.TrimSpace(input)
		if input == "6" || strings.ToLower(input) == "custom" {
			return "custom"
		}
		return parseQualityArg(input)
	}
}

func getCompressionModeName(compression bool) string {
	if compression {
		return "Opus"
	}
	return "PCM"
}

func promptCompressionMode(logger *utils.Logger) bool {
	fmt.Println("")
	fmt.Println("🎵 Select Compression Mode:")
	fmt.Println("  1. PCM (uncompressed, higher bandwidth)")
	fmt.Println("  2. Opus (compressed, lower bandwidth)")
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("Enter your choice (1 or 2, default 2): ")
		input, err := reader.ReadString('\n')
		if err != nil {
			logger.Error(fmt.Sprintf("Error reading input: %v", err))
			continue
		}
		input = strings.TrimSpace(input)
		switch input {
		case "1", "pcm":
			return false
		case "2", "opus", "":
			return true
		default:
			fmt.Println("❌ Invalid choice. Please enter 1 or 2.")
		}
	}
}

func promptEnableExcitation(logger *utils.Logger) bool {
	fmt.Println("")
	fmt.Println("⚡ Enable Excitation Streaming (pause streaming when silent)?")
	fmt.Println("  1. Yes (recommended for saving bandwidth)")
	fmt.Println("  2. No (always stream audio)")
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("Enter your choice (1 or 2, default 2): ")
		input, err := reader.ReadString('\n')
		if err != nil {
			logger.Error(fmt.Sprintf("Error reading input: %v", err))
			continue
		}
		input = strings.TrimSpace(input)
		switch input {
		case "1":
			return true
		case "2", "":
			return false
		default:
			fmt.Println("❌ Invalid choice. Please enter 1 or 2.")
		}
	}
}

// compression 参数解析
func parseCompressionArg(c string) bool {
	switch strings.ToLower(c) {
	case "yes", "opus", "true", "1":
		return true
	case "no", "pcm", "false", "0":
		return false
	default:
		return true // 默认使用Opus压缩
	}
}

// quality 参数支持数字和单词
func parseQualityArg(q string) string {
	switch strings.ToLower(q) {
	case "1", "verylow", "very-low":
		return "verylow"
	case "2", "low":
		return "low"
	case "3", "normal", "default":
		return "normal"
	case "4", "high":
		return "high"
	case "5", "lossless", "max":
		return "lossless"
	default:
		return "normal"
	}
}

func applyQualityParams(config *utils.Config) {
	// 根据 StreamQuality 设置音频参数
	switch config.StreamQuality {
	case "verylow":
		config.SampleRate = 8000
		config.Channels = 1
		config.BitDepth = 16
		config.FramesPerBuffer = 160 // 8000Hz * 20ms = 160 samples
	case "low":
		config.SampleRate = 16000
		config.Channels = 1
		config.BitDepth = 16
		config.FramesPerBuffer = 320 // 16000Hz * 20ms = 320 samples
	case "normal":
		config.SampleRate = 24000
		config.Channels = 2
		config.BitDepth = 16
		config.FramesPerBuffer = 480 // 24000Hz * 20ms = 480 samples
	case "high":
		config.SampleRate = 48000
		config.Channels = 2
		config.BitDepth = 16
		config.FramesPerBuffer = 960 // 48000Hz * 20ms = 960 samples
	case "lossless":
		config.SampleRate = 48000
		config.Channels = 2
		config.BitDepth = 24
		config.FramesPerBuffer = 960 // 48000Hz * 20ms = 960 samples
	case "custom":
		// 已由 promptCustomAudioParams 设置
		return
	default:
		config.SampleRate = 24000
		config.Channels = 2
		config.BitDepth = 16
		config.FramesPerBuffer = 480 // 24000Hz * 20ms = 480 samples
	}
}

func promptCustomAudioParams(config *utils.Config, logger *utils.Logger) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("")
	fmt.Println("🔧 Custom Audio Parameters:")
	// Sample Rate
	for {
		fmt.Print("Enter sample rate (8000, 12000, 16000, 24000, 48000): ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if sr, err := strconv.Atoi(input); err == nil && sr > 0 {
			// 检查是否为 Opus 支持的采样率
			validRates := []int{8000, 12000, 16000, 24000, 48000}
			valid := false
			for _, rate := range validRates {
				if sr == rate {
					valid = true
					break
				}
			}
			if valid {
				config.SampleRate = sr
				break
			}
		}
		fmt.Println("❌ Invalid sample rate. Must be one of: 8000, 12000, 16000, 24000, 48000")
	}
	// Channels
	for {
		fmt.Print("Enter number of channels (1=mono, 2=stereo): ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if ch, err := strconv.Atoi(input); err == nil && (ch == 1 || ch == 2) {
			config.Channels = ch
			break
		}
		fmt.Println("❌ Invalid channel count.")
	}
	// Bit Depth
	for {
		fmt.Print("Enter bit depth (16, 24): ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if bd, err := strconv.Atoi(input); err == nil && (bd == 16 || bd == 24) {
			config.BitDepth = bd
			break
		}
		fmt.Println("❌ Invalid bit depth.")
	}
	// Frames Per Buffer - 只允许 Opus 支持的帧长
	for {
		fmt.Print("Enter frames per buffer (Opus supported: 40, 80, 120, 160, 240, 320, 480, 960): ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if fpb, err := strconv.Atoi(input); err == nil && fpb > 0 {
			// 检查是否为 Opus 支持的帧长
			validFrames := []int{40, 80, 120, 160, 240, 320, 480, 960}
			valid := false
			for _, frame := range validFrames {
				if fpb == frame {
					valid = true
					break
				}
			}
			if valid {
				config.FramesPerBuffer = fpb
				break
			}
		}
		fmt.Println("❌ Invalid frames per buffer. Must be one of: 40, 80, 120, 160, 240, 320, 480, 960")
	}
}

// 新增允许客户端IP问询函数
func promptAllowedClientIPs(logger *utils.Logger) []string {
	fmt.Println("")
	fmt.Println("🔒 Enter allowed client IPs (comma separated, leave blank to allow all):")
	fmt.Print("Allowed IPs: ")
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input == "" {
		return nil // 允许所有
	}
	ips := strings.Split(input, ",")
	for i := range ips {
		ips[i] = strings.TrimSpace(ips[i])
	}
	return ips
}

// 新增 promptExcitationTimeout 函数
func promptExcitationTimeout(logger *utils.Logger) int {
	fmt.Print("Enter excitation pause timeout in seconds (default: 5): ")
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input == "" {
		return 5
	}
	if val, err := strconv.Atoi(input); err == nil && val > 0 {
		return val
	}
	fmt.Println("Invalid input, using default 5 seconds.")
	return 5
}