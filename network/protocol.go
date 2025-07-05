package network

import (
	"encoding/binary"
	"fmt"
	"io"
	"time"
)

// Protocol constants
const (
	ProtocolVersion = 1
	MagicNumber     = 0x41554449 // "AUDI" in ASCII
	HeaderSize      = 20         // Size of packet header in bytes
	MaxPayloadSize  = 65536      // Maximum payload size in bytes
)

// PacketType represents different types of packets
type PacketType uint8

const (
	PacketTypeHandshake PacketType = iota
	PacketTypeAudio
	PacketTypeControl
	PacketTypeHeartbeat
	PacketTypeError
)

// String returns the string representation of packet type
func (pt PacketType) String() string {
	switch pt {
	case PacketTypeHandshake:
		return "Handshake"
	case PacketTypeAudio:
		return "Audio"
	case PacketTypeControl:
		return "Control"
	case PacketTypeHeartbeat:
		return "Heartbeat"
	case PacketTypeError:
		return "Error"
	default:
		return "Unknown"
	}
}

// PacketHeader represents the header of a network packet
type PacketHeader struct {
	Magic       uint32    // Magic number for validation
	Version     uint8     // Protocol version
	Type        PacketType // Packet type
	Flags       uint8     // Various flags
	Reserved    uint8     // Reserved for future use
	Sequence    uint32    // Sequence number
	PayloadSize uint32    // Size of payload data
	Timestamp   uint32    // Timestamp (Unix time in seconds)
}

// Packet represents a complete network packet
type Packet struct {
	Header  PacketHeader
	Payload []byte
}

// NewPacket creates a new packet with the specified type and payload
func NewPacket(packetType PacketType, payload []byte) *Packet {
	return &Packet{
		Header: PacketHeader{
			Magic:       MagicNumber,
			Version:     ProtocolVersion,
			Type:        packetType,
			Flags:       0,
			Reserved:    0,
			Sequence:    0,
			PayloadSize: uint32(len(payload)),
			Timestamp:   uint32(time.Now().Unix()),
		},
		Payload: payload,
	}
}

// NewAudioPacket creates a new audio packet
func NewAudioPacket(audioData []byte, sequence uint32) *Packet {
	packet := NewPacket(PacketTypeAudio, audioData)
	packet.Header.Sequence = sequence
	return packet
}

// NewHandshakePacket creates a new handshake packet
func NewHandshakePacket(config *HandshakeConfig) *Packet {
	payload := config.ToBytes()
	return NewPacket(PacketTypeHandshake, payload)
}

// NewHeartbeatPacket creates a new heartbeat packet
func NewHeartbeatPacket() *Packet {
	return NewPacket(PacketTypeHeartbeat, nil)
}

// NewErrorPacket creates a new error packet
func NewErrorPacket(errorMessage string) *Packet {
	payload := []byte(errorMessage)
	return NewPacket(PacketTypeError, payload)
}

// WritePacket writes a packet to the provided writer
func WritePacket(writer io.Writer, packet *Packet) error {
	// Validate packet
	if packet.Header.Magic != MagicNumber {
		return fmt.Errorf("invalid magic number: 0x%08X", packet.Header.Magic)
	}
	
	if packet.Header.PayloadSize > MaxPayloadSize {
		return fmt.Errorf("payload too large: %d bytes", packet.Header.PayloadSize)
	}
	
	if len(packet.Payload) != int(packet.Header.PayloadSize) {
		return fmt.Errorf("payload size mismatch: header=%d, actual=%d", 
			packet.Header.PayloadSize, len(packet.Payload))
	}

	// Write header
	headerBytes := make([]byte, HeaderSize)
	binary.BigEndian.PutUint32(headerBytes[0:4], packet.Header.Magic)
	headerBytes[4] = packet.Header.Version
	headerBytes[5] = uint8(packet.Header.Type)
	headerBytes[6] = packet.Header.Flags
	headerBytes[7] = packet.Header.Reserved
	binary.BigEndian.PutUint32(headerBytes[8:12], packet.Header.Sequence)
	binary.BigEndian.PutUint32(headerBytes[12:16], packet.Header.PayloadSize)
	binary.BigEndian.PutUint32(headerBytes[16:20], packet.Header.Timestamp)

	if _, err := writer.Write(headerBytes); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	// Write payload if present
	if packet.Header.PayloadSize > 0 {
		if _, err := writer.Write(packet.Payload); err != nil {
			return fmt.Errorf("failed to write payload: %w", err)
		}
	}

	return nil
}

// ReadPacket reads a packet from the provided reader
func ReadPacket(reader io.Reader) (*Packet, error) {
	// Read header
	headerBytes := make([]byte, HeaderSize)
	if _, err := io.ReadFull(reader, headerBytes); err != nil {
		return nil, fmt.Errorf("failed to read header: %w", err)
	}

	// Parse header
	header := PacketHeader{
		Magic:       binary.BigEndian.Uint32(headerBytes[0:4]),
		Version:     headerBytes[4],
		Type:        PacketType(headerBytes[5]),
		Flags:       headerBytes[6],
		Reserved:    headerBytes[7],
		Sequence:    binary.BigEndian.Uint32(headerBytes[8:12]),
		PayloadSize: binary.BigEndian.Uint32(headerBytes[12:16]),
		Timestamp:   binary.BigEndian.Uint32(headerBytes[16:20]),
	}

	// Validate header
	if header.Magic != MagicNumber {
		return nil, fmt.Errorf("invalid magic number: 0x%08X", header.Magic)
	}

	if header.Version != ProtocolVersion {
		return nil, fmt.Errorf("unsupported protocol version: %d", header.Version)
	}

	if header.PayloadSize > MaxPayloadSize {
		return nil, fmt.Errorf("payload too large: %d bytes", header.PayloadSize)
	}

	// Read payload
	var payload []byte
	if header.PayloadSize > 0 {
		payload = make([]byte, header.PayloadSize)
		if _, err := io.ReadFull(reader, payload); err != nil {
			return nil, fmt.Errorf("failed to read payload: %w", err)
		}
	}

	return &Packet{
		Header:  header,
		Payload: payload,
	}, nil
}

// HandshakeConfig represents the configuration sent during handshake
type HandshakeConfig struct {
	SampleRate      uint32
	Channels        uint8
	BitDepth        uint8
	FramesPerBuffer uint16
	BufferCount     uint8
	Compression     uint8
}

// ToBytes converts handshake config to byte array
func (hc *HandshakeConfig) ToBytes() []byte {
	data := make([]byte, 12)
	binary.BigEndian.PutUint32(data[0:4], hc.SampleRate)
	data[4] = hc.Channels
	data[5] = hc.BitDepth
	binary.BigEndian.PutUint16(data[6:8], hc.FramesPerBuffer)
	data[8] = hc.BufferCount
	data[9] = hc.Compression
	// data[10:12] reserved for future use
	return data
}

// FromBytes parses handshake config from byte array
func (hc *HandshakeConfig) FromBytes(data []byte) error {
	if len(data) < 12 {
		return fmt.Errorf("handshake data too short: %d bytes", len(data))
	}

	hc.SampleRate = binary.BigEndian.Uint32(data[0:4])
	hc.Channels = data[4]
	hc.BitDepth = data[5]
	hc.FramesPerBuffer = binary.BigEndian.Uint16(data[6:8])
	hc.BufferCount = data[8]
	hc.Compression = data[9]

	return nil
}

// Validate checks if the handshake config is valid
func (hc *HandshakeConfig) Validate() error {
	if hc.SampleRate < 8000 || hc.SampleRate > 192000 {
		return fmt.Errorf("invalid sample rate: %d", hc.SampleRate)
	}

	if hc.Channels == 0 || hc.Channels > 8 {
		return fmt.Errorf("invalid channel count: %d", hc.Channels)
	}

	if hc.BitDepth != 16 && hc.BitDepth != 24 && hc.BitDepth != 32 {
		return fmt.Errorf("invalid bit depth: %d", hc.BitDepth)
	}

	if hc.FramesPerBuffer == 0 || hc.FramesPerBuffer > 8192 {
		return fmt.Errorf("invalid frames per buffer: %d", hc.FramesPerBuffer)
	}

	if hc.BufferCount == 0 || hc.BufferCount > 16 {
		return fmt.Errorf("invalid buffer count: %d", hc.BufferCount)
	}

	return nil
}