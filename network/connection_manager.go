// network/connection_manager.go - 连接管理器

package network

import (
	"sync"
	"sync/atomic"
)

// ConnectionManager 管理连接状态和优雅关闭
type ConnectionManager struct {
	shutdownRequested int32
	activeConnections int32
	shutdownChan      chan struct{}
	mutex             sync.RWMutex
	onShutdown        []func()
}

var globalConnectionManager = &ConnectionManager{
	shutdownChan: make(chan struct{}),
	onShutdown:   make([]func(), 0),
}

// NotifyShutdown 通知所有连接开始关闭
func NotifyShutdown() {
	if atomic.CompareAndSwapInt32(&globalConnectionManager.shutdownRequested, 0, 1) {
		close(globalConnectionManager.shutdownChan)
		
		// 执行所有注册的关闭回调
		globalConnectionManager.mutex.RLock()
		for _, callback := range globalConnectionManager.onShutdown {
			go callback()
		}
		globalConnectionManager.mutex.RUnlock()
	}
}

// IsShutdownRequested 检查是否请求关闭
func IsShutdownRequested() bool {
	return atomic.LoadInt32(&globalConnectionManager.shutdownRequested) == 1
}

// RegisterShutdownCallback 注册关闭回调
func RegisterShutdownCallback(callback func()) {
	globalConnectionManager.mutex.Lock()
	globalConnectionManager.onShutdown = append(globalConnectionManager.onShutdown, callback)
	globalConnectionManager.mutex.Unlock()
}

// GetShutdownChannel 获取关闭信号通道
func GetShutdownChannel() <-chan struct{} {
	return globalConnectionManager.shutdownChan
}

// IncrementConnections 增加活跃连接数
func IncrementConnections() {
	atomic.AddInt32(&globalConnectionManager.activeConnections, 1)
}

// DecrementConnections 减少活跃连接数
func DecrementConnections() {
	atomic.AddInt32(&globalConnectionManager.activeConnections, -1)
}

// GetActiveConnections 获取活跃连接数
func GetActiveConnections() int32 {
	return atomic.LoadInt32(&globalConnectionManager.activeConnections)
}