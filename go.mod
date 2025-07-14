module RemoteAudioCLI

go 1.18

require github.com/gordonklaus/portaudio v0.0.0-20250206071425-98a94950218b

require github.com/hraban/opus v0.0.0-20230925203106-0188a62cb302 // indirect

// Windows 7 兼容性配置
// 使用较旧但稳定的 PortAudio & Go 版本以确保兼容性
// go 1.16
// require github.com/gordonklaus/portaudio v0.0.0-20221027163845-7c3b689db3cc
