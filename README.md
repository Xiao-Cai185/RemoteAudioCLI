# 🎧 **Remote Audio CLI**

A lightweight, command-line remote audio streaming application written in Go.
Stream audio between devices over the network with low latency.

---

## ✨ **Features**

* 🔊 Real-time audio capture and playback
* 🌐 TCP-based network transmission
* 💻 Cross-platform audio device support
* ⚡ Low-latency streaming
* 🛠️ Command-line interface

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

## 📦 **Dependencies**

* [github.com/gordonklaus/portaudio](https://github.com/gordonklaus/portaudio)

---
