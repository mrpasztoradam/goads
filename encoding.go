package goads

import (
	"encoding/binary"
	"fmt"
	"math"
)

// EncodeValue encodes a string value into bytes based on the data type
func EncodeValue(value string, dataType string, size uint32) ([]byte, error) {
	// Handle basic types
	switch dataType {
	case "BOOL":
		var boolVal bool
		if value == "true" || value == "1" {
			boolVal = true
		}
		data := make([]byte, 1)
		if boolVal {
			data[0] = 1
		}
		return data, nil

	case "SINT":
		var val int8
		if _, err := fmt.Sscanf(value, "%d", &val); err != nil {
			return nil, fmt.Errorf("invalid SINT value: %w", err)
		}
		return []byte{byte(val)}, nil

	case "USINT", "BYTE":
		var val uint8
		if _, err := fmt.Sscanf(value, "%d", &val); err != nil {
			return nil, fmt.Errorf("invalid USINT/BYTE value: %w", err)
		}
		return []byte{val}, nil

	case "INT":
		var val int16
		if _, err := fmt.Sscanf(value, "%d", &val); err != nil {
			return nil, fmt.Errorf("invalid INT value: %w", err)
		}
		data := make([]byte, 2)
		binary.LittleEndian.PutUint16(data, uint16(val))
		return data, nil

	case "UINT", "WORD":
		var val uint16
		if _, err := fmt.Sscanf(value, "%d", &val); err != nil {
			return nil, fmt.Errorf("invalid UINT/WORD value: %w", err)
		}
		data := make([]byte, 2)
		binary.LittleEndian.PutUint16(data, val)
		return data, nil

	case "DINT":
		var val int32
		if _, err := fmt.Sscanf(value, "%d", &val); err != nil {
			return nil, fmt.Errorf("invalid DINT value: %w", err)
		}
		data := make([]byte, 4)
		binary.LittleEndian.PutUint32(data, uint32(val))
		return data, nil

	case "UDINT", "DWORD":
		var val uint32
		if _, err := fmt.Sscanf(value, "%d", &val); err != nil {
			return nil, fmt.Errorf("invalid UDINT/DWORD value: %w", err)
		}
		data := make([]byte, 4)
		binary.LittleEndian.PutUint32(data, val)
		return data, nil

	case "LINT":
		var val int64
		if _, err := fmt.Sscanf(value, "%d", &val); err != nil {
			return nil, fmt.Errorf("invalid LINT value: %w", err)
		}
		data := make([]byte, 8)
		binary.LittleEndian.PutUint64(data, uint64(val))
		return data, nil

	case "ULINT", "LWORD":
		var val uint64
		if _, err := fmt.Sscanf(value, "%d", &val); err != nil {
			return nil, fmt.Errorf("invalid ULINT/LWORD value: %w", err)
		}
		data := make([]byte, 8)
		binary.LittleEndian.PutUint64(data, val)
		return data, nil

	case "REAL":
		var val float32
		if _, err := fmt.Sscanf(value, "%f", &val); err != nil {
			return nil, fmt.Errorf("invalid REAL value: %w", err)
		}
		data := make([]byte, 4)
		binary.LittleEndian.PutUint32(data, math.Float32bits(val))
		return data, nil

	case "LREAL":
		var val float64
		if _, err := fmt.Sscanf(value, "%f", &val); err != nil {
			return nil, fmt.Errorf("invalid LREAL value: %w", err)
		}
		data := make([]byte, 8)
		binary.LittleEndian.PutUint64(data, math.Float64bits(val))
		return data, nil

	default:
		// Check for STRING type
		if len(dataType) >= 6 && dataType[:6] == "STRING" {
			data := make([]byte, size)
			copy(data, []byte(value))
			// Ensure null termination
			if len(value) < int(size) {
				data[len(value)] = 0
			}
			return data, nil
		}
	}

	return nil, fmt.Errorf("unsupported data type: %s", dataType)
}

// DecodeFieldValue decodes a field value from raw bytes based on its data type
func DecodeFieldValue(data []byte, dataType string) interface{} {
	if len(data) == 0 {
		return nil
	}

	// Handle basic types
	switch dataType {
	case "BOOL":
		if len(data) >= 1 {
			return data[0] != 0
		}
	case "SINT", "USINT", "BYTE":
		if len(data) >= 1 {
			if dataType == "SINT" {
				return int8(data[0])
			}
			return uint8(data[0])
		}
	case "INT":
		if len(data) >= 2 {
			return int16(binary.LittleEndian.Uint16(data[0:2]))
		}
	case "UINT", "WORD":
		if len(data) >= 2 {
			return binary.LittleEndian.Uint16(data[0:2])
		}
	case "DINT":
		if len(data) >= 4 {
			return int32(binary.LittleEndian.Uint32(data[0:4]))
		}
	case "UDINT", "DWORD":
		if len(data) >= 4 {
			return binary.LittleEndian.Uint32(data[0:4])
		}
	case "LINT":
		if len(data) >= 8 {
			return int64(binary.LittleEndian.Uint64(data[0:8]))
		}
	case "ULINT", "LWORD":
		if len(data) >= 8 {
			return binary.LittleEndian.Uint64(data[0:8])
		}
	case "REAL":
		if len(data) >= 4 {
			bits := binary.LittleEndian.Uint32(data[0:4])
			return math.Float32frombits(bits)
		}
	case "LREAL":
		if len(data) >= 8 {
			bits := binary.LittleEndian.Uint64(data[0:8])
			return math.Float64frombits(bits)
		}
	default:
		// Check for STRING type
		if len(dataType) >= 6 && dataType[:6] == "STRING" {
			// Find null terminator
			for i := 0; i < len(data); i++ {
				if data[i] == 0 {
					return string(data[:i])
				}
			}
			return string(data)
		}
	}

	// For unknown types, return hex string
	return fmt.Sprintf("%X", data)
}
