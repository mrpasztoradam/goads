package goads

import (
	"encoding/binary"
	"fmt"
	"math"
)

// TypedEncoder provides type-safe encoding functions for ADS/TwinCAT data types
type TypedEncoder struct{}

// NewTypedEncoder creates a new typed encoder
func NewTypedEncoder() *TypedEncoder {
	return &TypedEncoder{}
}

// EncodeBool encodes a boolean value (BOOL)
func (e *TypedEncoder) EncodeBool(value bool) []byte {
	if value {
		return []byte{1}
	}
	return []byte{0}
}

// EncodeByte encodes an unsigned 8-bit integer (BYTE/USINT)
func (e *TypedEncoder) EncodeByte(value uint8) []byte {
	return []byte{value}
}

// EncodeSInt encodes a signed 8-bit integer (SINT)
func (e *TypedEncoder) EncodeSInt(value int8) []byte {
	return []byte{byte(value)}
}

// EncodeInt16 encodes a signed 16-bit integer (INT)
func (e *TypedEncoder) EncodeInt16(value int16) []byte {
	buf := make([]byte, 2)
	binary.LittleEndian.PutUint16(buf, uint16(value))
	return buf
}

// EncodeUInt16 encodes an unsigned 16-bit integer (UINT/WORD)
func (e *TypedEncoder) EncodeUInt16(value uint16) []byte {
	buf := make([]byte, 2)
	binary.LittleEndian.PutUint16(buf, value)
	return buf
}

// EncodeInt32 encodes a signed 32-bit integer (DINT)
func (e *TypedEncoder) EncodeInt32(value int32) []byte {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, uint32(value))
	return buf
}

// EncodeUInt32 encodes an unsigned 32-bit integer (UDINT/DWORD)
func (e *TypedEncoder) EncodeUInt32(value uint32) []byte {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, value)
	return buf
}

// EncodeInt64 encodes a signed 64-bit integer (LINT)
func (e *TypedEncoder) EncodeInt64(value int64) []byte {
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, uint64(value))
	return buf
}

// EncodeUInt64 encodes an unsigned 64-bit integer (ULINT/LWORD)
func (e *TypedEncoder) EncodeUInt64(value uint64) []byte {
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, value)
	return buf
}

// EncodeFloat32 encodes a 32-bit floating point number (REAL)
func (e *TypedEncoder) EncodeFloat32(value float32) []byte {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, math.Float32bits(value))
	return buf
}

// EncodeFloat64 encodes a 64-bit floating point number (LREAL)
func (e *TypedEncoder) EncodeFloat64(value float64) []byte {
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, math.Float64bits(value))
	return buf
}

// EncodeString encodes a string with specified max length (STRING)
func (e *TypedEncoder) EncodeString(value string, maxLen int) []byte {
	buf := make([]byte, maxLen)
	copy(buf, []byte(value))
	return buf
}

// TypedDecoder provides type-safe decoding functions for ADS/TwinCAT data types
type TypedDecoder struct{}

// NewTypedDecoder creates a new typed decoder
func NewTypedDecoder() *TypedDecoder {
	return &TypedDecoder{}
}

// DecodeBool decodes a boolean value (BOOL)
func (d *TypedDecoder) DecodeBool(data []byte) (bool, error) {
	if len(data) < 1 {
		return false, fmt.Errorf("insufficient data for BOOL")
	}
	return data[0] != 0, nil
}

// DecodeByte decodes an unsigned 8-bit integer (BYTE/USINT)
func (d *TypedDecoder) DecodeByte(data []byte) (uint8, error) {
	if len(data) < 1 {
		return 0, fmt.Errorf("insufficient data for BYTE")
	}
	return data[0], nil
}

// DecodeSInt decodes a signed 8-bit integer (SINT)
func (d *TypedDecoder) DecodeSInt(data []byte) (int8, error) {
	if len(data) < 1 {
		return 0, fmt.Errorf("insufficient data for SINT")
	}
	return int8(data[0]), nil
}

// DecodeInt16 decodes a signed 16-bit integer (INT)
func (d *TypedDecoder) DecodeInt16(data []byte) (int16, error) {
	if len(data) < 2 {
		return 0, fmt.Errorf("insufficient data for INT")
	}
	return int16(binary.LittleEndian.Uint16(data[:2])), nil
}

// DecodeUInt16 decodes an unsigned 16-bit integer (UINT/WORD)
func (d *TypedDecoder) DecodeUInt16(data []byte) (uint16, error) {
	if len(data) < 2 {
		return 0, fmt.Errorf("insufficient data for UINT")
	}
	return binary.LittleEndian.Uint16(data[:2]), nil
}

// DecodeInt32 decodes a signed 32-bit integer (DINT)
func (d *TypedDecoder) DecodeInt32(data []byte) (int32, error) {
	if len(data) < 4 {
		return 0, fmt.Errorf("insufficient data for DINT")
	}
	return int32(binary.LittleEndian.Uint32(data[:4])), nil
}

// DecodeUInt32 decodes an unsigned 32-bit integer (UDINT/DWORD)
func (d *TypedDecoder) DecodeUInt32(data []byte) (uint32, error) {
	if len(data) < 4 {
		return 0, fmt.Errorf("insufficient data for UDINT")
	}
	return binary.LittleEndian.Uint32(data[:4]), nil
}

// DecodeInt64 decodes a signed 64-bit integer (LINT)
func (d *TypedDecoder) DecodeInt64(data []byte) (int64, error) {
	if len(data) < 8 {
		return 0, fmt.Errorf("insufficient data for LINT")
	}
	return int64(binary.LittleEndian.Uint64(data[:8])), nil
}

// DecodeUInt64 decodes an unsigned 64-bit integer (ULINT/LWORD)
func (d *TypedDecoder) DecodeUInt64(data []byte) (uint64, error) {
	if len(data) < 8 {
		return 0, fmt.Errorf("insufficient data for ULINT")
	}
	return binary.LittleEndian.Uint64(data[:8]), nil
}

// DecodeFloat32 decodes a 32-bit floating point number (REAL)
func (d *TypedDecoder) DecodeFloat32(data []byte) (float32, error) {
	if len(data) < 4 {
		return 0, fmt.Errorf("insufficient data for REAL")
	}
	bits := binary.LittleEndian.Uint32(data[:4])
	return math.Float32frombits(bits), nil
}

// DecodeFloat64 decodes a 64-bit floating point number (LREAL)
func (d *TypedDecoder) DecodeFloat64(data []byte) (float64, error) {
	if len(data) < 8 {
		return 0, fmt.Errorf("insufficient data for LREAL")
	}
	bits := binary.LittleEndian.Uint64(data[:8])
	return math.Float64frombits(bits), nil
}

// DecodeString decodes a null-terminated string (STRING)
func (d *TypedDecoder) DecodeString(data []byte) (string, error) {
	if len(data) == 0 {
		return "", nil
	}

	// Find null terminator
	end := len(data)
	for i, b := range data {
		if b == 0 {
			end = i
			break
		}
	}

	return string(data[:end]), nil
}
