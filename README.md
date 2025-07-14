# 🎧 **Remote Audio CLI**

A lightweight, command-line remote audio streaming application written in Go.
<<<<<<< HEAD
Stream audio between devices over the network with low latency and compression support.
=======
Stream audio between devices over the network with low latency.
>>>>>>> f22ae08551c5c9d0a35b183a89426ada56f9bc31

---

## ✨ **Features**

* 🔊 Real-time audio capture and playback
* 🌐 TCP-based network transmission
* 💻 Cross-platform audio device support
* ⚡ Low-latency streaming
* 🛠️ Command-line interface
<<<<<<< HEAD
* 🎵 Audio compression support (Opus codec)
* 🎚️ Multiple stream quality modes
* 🔄 Excitation mode (pause streaming when silent)
* 🎛️ Custom audio parameters
* 🔔 Audio notifications (connection/disconnection sounds)
* ⏰ Graceful shutdown with countdown
* 🔒 Client IP whitelist support
* 🎯 Configurable excitation timeout
=======
>>>>>>> f22ae08551c5c9d0a35b183a89426ada56f9bc31

---

## 🚀 **Usage**

### 🖥️ **Server Mode** (System default output device)

```bash
./RemoteAudioCli.exe -mode=server -port=8080
```

---

### 🎚️ **Server Mode** (Select a specific output device)

```bash
./RemoteAudioCli.exe -mode=server -port=8080 -output-device int/"string"
```

#### Example:

```bash
./RemoteAudioCli.exe -mode=server -port=8080 -output-device 1
```

```bash
./RemoteAudioCli.exe -mode=server -port=8080 -output-device "Speakers (Realtek(R) Audio)"
```

---

### 🎤 **Client Mode** (System default input device)

```bash
./RemoteAudioCli.exe -mode=client -host=localhost -port=8080
```

---

### 🎛️ **Client Mode** (Select a specific input device)

```bash
./RemoteAudioCli.exe -mode=client -host=localhost -port=8080 -input-device int/"string"
```

#### Example:

```bash
./RemoteAudioCli.exe -mode=client -host=localhost -port=8080 -input-device 1
```

```bash
./RemoteAudioCli.exe -mode=client -host=localhost -port=8080 -input-device "Microphone (Realtek(R) Audio)"
```

---

<<<<<<< HEAD
### 🎵 **Stream Quality Modes**

#### **Very Low Quality** (8kHz, mono, 16-bit)
```bash
./RemoteAudioCli.exe -mode=client -host=localhost -port=8080 -quality=verylow
```

#### **Low Quality** (16kHz, mono, 16-bit)
```bash
./RemoteAudioCli.exe -mode=client -host=localhost -port=8080 -quality=low
```

#### **Normal Quality** (24kHz, stereo, 16-bit) - **Default**
```bash
./RemoteAudioCli.exe -mode=client -host=localhost -port=8080 -quality=normal
```

#### **High Quality** (48kHz, stereo, 16-bit)
```bash
./RemoteAudioCli.exe -mode=client -host=localhost -port=8080 -quality=high
```

#### **Lossless Quality** (48kHz, stereo, 24-bit)
```bash
./RemoteAudioCli.exe -mode=client -host=localhost -port=8080 -quality=lossless
```

### 🎵 **Compression Modes**

#### **Opus Compression** (Default - Lower bandwidth)
```bash
./RemoteAudioCli.exe -mode=client -host=localhost -port=8080 -compress=yes
```

#### **PCM Uncompressed** (Higher quality, higher bandwidth)
```bash
./RemoteAudioCli.exe -mode=client -host=localhost -port=8080 -compress=no
```

---

### 🔄 **Excitation Mode** (Pause streaming when silent)

Enable excitation mode to reduce bandwidth usage when no audio is detected:

```bash
./RemoteAudioCli.exe -mode=client -host=localhost -port=8080 -excitation
```

#### **Custom Excitation Settings**

```bash
./RemoteAudioCli.exe -mode=client -host="192.168.1.10" -input-device "Microphone (Realtek(R) Audio)" -port=8080 -excitation -excitation-threshold=-45.0 -excitation-timeout=10
```

* `-excitation-threshold`: Audio level threshold in dB (default: -45.0)
* `-excitation-timeout`: Timeout in seconds before resuming (default: 10)

---

### 🔒 **Client IP Whitelist** (Server security)

Restrict server connections to specific client IPs:

```bash
./RemoteAudioCli.exe -mode=server -port=8080 -allow-client="192.168.1.100,127.0.0.1"
```

* `-allow-client`: Comma-separated list of allowed client IPs
* Leave empty to allow all clients (default behavior)
* Server will reject connections from non-whitelisted IPs with warning logs

---

=======
>>>>>>> f22ae08551c5c9d0a35b183a89426ada56f9bc31
### 🎙️ **List Available Audio Devices**

```bash
./RemoteAudioCli.exe -list-devices
```

---

### 🧙‍♂️ **Wizard Mode (Interactive setup)**

Just run without any parameters:

```bash
./RemoteAudioCli.exe
```

<<<<<<< HEAD
The wizard will guide you through:
* Mode selection (Server/Client)
* Device selection
* Stream quality configuration (sample rate, channels, bit depth)
* Compression mode selection (Opus/PCM)
* Excitation mode settings
* Excitation timeout configuration (when enabled)
* Client IP whitelist (server mode)
* Custom audio parameters

=======
>>>>>>> f22ae08551c5c9d0a35b183a89426ada56f9bc31
---

### 📖 **Show Help Information**

Display all available command-line options:

```bash
./RemoteAudioCli.exe -help
```

---

## 🛠️ **Build**

Make sure you have [Go](https://golang.org) installed.

### ✅ **Standard Build (Recommended for modern systems)**

```bash
go build -o RemoteAudioCli.exe main.go
```

* Requires **Go 1.18+**
* Uses the latest version of [portaudio](https://github.com/gordonklaus/portaudio)

---

### 🖥️ **Windows 7 Compatible Build**

If you need to run on **Windows 7**, please use the following configuration for better compatibility:

1. Use **Go 1.16**
2. In your `go.mod` file, replace the dependencies as shown below:

```go
go 1.16

require github.com/gordonklaus/portaudio v0.0.0-20221027163845-7c3b689db3cc
```

3. Then build:

```bash
go build -o RemoteAudioCli.exe main.go
```

⚠️ *The latest versions of `portaudio` may not work properly on Windows 7 due to compatibility issues.*

---

<<<<<<< HEAD
### 🔧 **MSYS2 MINGW64 Shell Build**

For building on Windows with MSYS2 MINGW64 Shell, install the required dependencies:

```bash
# Install GCC and pkg-config
pacman -S mingw-w64-x86_64-gcc mingw-w64-x86_64-pkg-config

# Install PortAudio
pacman -S mingw-w64-x86_64-portaudio

# Install Opus (for compression support)
pacman -S mingw-w64-x86_64-opus mingw-w64-x86_64-pkg-config
pacman -S mingw-w64-x86_64-opusfile

# Set environment variables
export PATH=/mingw64/bin:$PATH
export PKG_CONFIG_PATH=/mingw64/lib/pkgconfig

# Build the application
go build -o RemoteAudioCli.exe main.go
```

---

## 📦 **Dependencies**

* [github.com/gordonklaus/portaudio](https://github.com/gordonklaus/portaudio) - Audio capture and playback
* [gopkg.in/hraban/opus.v2](https://gopkg.in/hraban/opus.v2) - Opus audio codec for compression

---

## 🎵 **Audio Quality Modes**

| Mode | Sample Rate | Channels | Bit Depth | Use Case |
|------|-------------|----------|-----------|----------|
| **Very Low** | 8 kHz | Mono | 16-bit | Low bandwidth, voice only |
| **Low** | 16 kHz | Mono | 16-bit | Voice calls, limited bandwidth |
| **Normal** | 24 kHz | Stereo | 16-bit | General purpose, balanced |
| **High** | 48 kHz | Stereo | 16-bit | High quality, moderate bandwidth |
| **Lossless** | 48 kHz | Stereo | 24-bit | Studio quality, high bandwidth |

## 🎵 **Compression Modes**

| Mode | Compression | Bandwidth | Quality | Use Case |
|------|-------------|-----------|---------|----------|
| **Opus** | Yes | Lower | Good | Default, most scenarios |
| **PCM** | No | Higher | Best | Studio, lossless requirements |

---

## 🔄 **Excitation Mode**

Excitation mode intelligently pauses audio streaming when no significant audio is detected, reducing bandwidth usage:

* **Automatic Detection**: Monitors audio levels in real-time
* **Configurable Threshold**: Adjustable sensitivity (-60dB to 0dB)
* **Configurable Timeout**: Set pause duration before resuming (default: 5 seconds)
* **Bandwidth Savings**: Reduces network usage during silence
* **Interactive Configuration**: Wizard mode allows easy timeout setup

---

## 🔒 **Client IP Whitelist**

Server security feature to restrict connections to trusted clients:

* **IP-based Access Control**: Allow only specific client IPs to connect
* **Comma-separated List**: Support multiple allowed IPs
* **Default Open**: No whitelist means all clients are allowed
* **Connection Logging**: Clear warnings when unauthorized clients attempt connection
* **Interactive Setup**: Wizard mode supports easy whitelist configuration

---

## 🔔 **Audio Notifications**

The application provides audio feedback for connection events:

* **Connection Sound**: Plays when client successfully connects
* **Disconnection Sound**: Plays when client disconnects
* **Startup Beep**: 4-tone beep sequence on server startup
* **Fade-in Effect**: Smooth audio transition after connection
* **Handshake Information**: Client logs show compression status (Opus ON/OFF)

---

## ⏰ **Graceful Shutdown**

The application supports graceful shutdown with countdown:

* **Immediate Exit**: `-help` and `-list-devices` commands exit immediately
* **Graceful Exit**: Other scenarios show 5-second countdown
* **Resource Cleanup**: Properly closes connections and releases resources
* **User Feedback**: Clear status messages during shutdown process
* **Connection Safety**: Handles early client disconnections without crashes

---

## 📋 **Complete Usage Examples**

### **Server with Security**
```bash
# Allow only specific clients
./RemoteAudioCli.exe -mode=server -port=8080 -allow-client="192.168.1.100,127.0.0.1"
```

### **High-Quality Client**
```bash
# Lossless audio with PCM compression
./RemoteAudioCli.exe -mode=client -host="192.168.1.100" -port=8080 -quality=lossless -compress=no
```

### **Bandwidth-Optimized Client**
```bash
# Low quality with Opus compression and excitation mode
./RemoteAudioCli.exe -mode=client -host=192.168.1.100 -port=8080 -quality=low -compress=yes -excitation -excitation-timeout=3
```

### **Interactive Setup**
```bash
# Complete guided configuration
./RemoteAudioCli.exe
```
=======
## 📦 **Dependencies**

* [github.com/gordonklaus/portaudio](https://github.com/gordonklaus/portaudio)

---
>>>>>>> f22ae08551c5c9d0a35b183a89426ada56f9bc31
