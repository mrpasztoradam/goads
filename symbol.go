// Copyright 2021 gotwincat authors. All rights reserved.
// Use of this source code is governed by a MIT-style license that can be
// found in the LICENSE file.

package twincat

import (
	"context"
	"encoding/binary"
	"fmt"

	"github.com/mrpasztoradam/goads/ams"
)

// StructField represents a field within a struct
type StructField struct {
	Name     string        `json:"name"`
	DataType string        `json:"type"`
	Offset   uint32        `json:"offset"`
	Size     uint32        `json:"size"`
	Value    interface{}   `json:"value,omitempty"`
	Fields   []StructField `json:"fields,omitempty"`
}

// Symbol represents a PLC symbol
type Symbol struct {
	Name     string        `json:"name"`
	DataType string        `json:"type"`
	Size     uint32        `json:"size"`
	Fields   []StructField `json:"fields,omitempty"`
}

// GetSymbol retrieves full symbol information including data type and fields
func (c *Client) GetSymbol(ctx context.Context, targetAddr, senderAddr ams.Addr, name string) (*Symbol, error) {
	// Read symbol info by name using ReadWrite command
	// IndexGroup 0xF009 = ADSIGRP_SYM_INFOBYNAMEEX
	nameBytes := []byte(name)
	nameBytes = append(nameBytes, 0) // Null terminator

	req := ams.NewReadWriteRequest(
		targetAddr,
		senderAddr,
		0xF009, // ADSIGRP_SYM_INFOBYNAMEEX
		0x0,
		0xFFFF, // Max response size
		nameBytes,
	)
	resp, err := c.ReadWrite(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get symbol info: %w", err)
	}

	if len(resp.Data) < 32 {
		return nil, fmt.Errorf("invalid symbol info response (length: %d)", len(resp.Data))
	}

	// Parse ADS symbol entry structure:
	// Offset 0: entryLength (4 bytes)
	// Offset 4: iGroup (4 bytes)
	// Offset 8: iOffs (4 bytes)
	// Offset 12: size (4 bytes)
	// Offset 16: dataType (4 bytes)
	// Offset 20: flags (4 bytes)
	// Offset 24: nameLength (2 bytes)
	// Offset 26: typeLength (2 bytes)
	// Offset 28: commentLength (2 bytes)
	// Offset 30: name (variable)
	// Offset 30+nameLength: type (variable)
	// Offset 30+nameLength+typeLength: comment (variable)

	size := binary.LittleEndian.Uint32(resp.Data[12:16])
	nameLength := binary.LittleEndian.Uint16(resp.Data[24:26])
	typeLength := binary.LittleEndian.Uint16(resp.Data[26:28])

	// Extract type name
	typeStart := 30 + int(nameLength) + 1 // +1 to skip null terminator after name
	typeEnd := typeStart + int(typeLength)

	var dataType string
	if typeEnd <= len(resp.Data) {
		dataType = string(resp.Data[typeStart:typeEnd])
		// Remove null terminator if present
		if idx := 0; idx < len(dataType) && dataType[idx] == 0 {
			dataType = dataType[:idx]
		} else {
			for i := 0; i < len(dataType); i++ {
				if dataType[i] == 0 {
					dataType = dataType[:i]
					break
				}
			}
		}
	} else {
		dataType = "UNKNOWN"
	}

	symbol := &Symbol{
		Name:     name,
		DataType: dataType,
		Size:     size,
	}

	return symbol, nil
}

// GetDataTypeInfo retrieves the field information for a data type
func (c *Client) GetDataTypeInfo(ctx context.Context, targetAddr, senderAddr ams.Addr, typeName string) ([]StructField, error) {
	// Read data type info by name using ReadWrite command
	// IndexGroup 0xF011 = ADSIGRP_SYM_DT_UPLOAD (data type upload)
	typeBytes := []byte(typeName)
	typeBytes = append(typeBytes, 0) // Null terminator

	req := ams.NewReadWriteRequest(
		targetAddr,
		senderAddr,
		0xF011, // ADSIGRP_SYM_DT_UPLOAD
		0x0,
		0xFFFF, // Max response size
		typeBytes,
	)
	resp, err := c.ReadWrite(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get data type info: %w", err)
	}

	if len(resp.Data) < 32 {
		return nil, fmt.Errorf("invalid data type info response")
	}

	// Parse data type entry structure:
	// Offset 0: entryLength (4 bytes)
	// Offset 4: version (4 bytes)
	// Offset 8: hashValue (4 bytes)
	// Offset 12: typeHashValue (4 bytes)
	// Offset 16: size (4 bytes)
	// Offset 20: offs (4 bytes)
	// Offset 24: dataType (4 bytes)
	// Offset 28: flags (4 bytes)
	// Offset 32: nameLength (2 bytes)
	// Offset 34: typeLength (2 bytes)
	// Offset 36: commentLength (2 bytes)
	// Offset 38: arrayDim (2 bytes)
	// Offset 40: subItems (2 bytes) <- number of fields in struct

	if len(resp.Data) < 42 {
		return nil, fmt.Errorf("response too short for data type info")
	}

	subItems := binary.LittleEndian.Uint16(resp.Data[40:42])
	if subItems == 0 {
		return nil, nil // No fields (primitive type)
	}

	nameLength := binary.LittleEndian.Uint16(resp.Data[32:34])
	typeLength := binary.LittleEndian.Uint16(resp.Data[34:36])
	commentLength := binary.LittleEndian.Uint16(resp.Data[36:38])

	// Calculate offset to sub-items
	offset := 42 + int(nameLength) + 1 + int(typeLength) + 1 + int(commentLength) + 1

	fields := make([]StructField, 0, subItems)

	// Parse each sub-item (field)
	for i := 0; i < int(subItems) && offset < len(resp.Data); i++ {
		if offset+42 > len(resp.Data) {
			break
		}

		// Parse sub-item structure (same as parent)
		fieldSize := binary.LittleEndian.Uint32(resp.Data[offset+16 : offset+20])
		fieldOffset := binary.LittleEndian.Uint32(resp.Data[offset+20 : offset+24])
		fieldNameLen := binary.LittleEndian.Uint16(resp.Data[offset+32 : offset+34])
		fieldTypeLen := binary.LittleEndian.Uint16(resp.Data[offset+34 : offset+36])

		// Extract field name
		fieldNameStart := offset + 42
		fieldNameEnd := fieldNameStart + int(fieldNameLen)
		if fieldNameEnd > len(resp.Data) {
			break
		}
		fieldName := string(resp.Data[fieldNameStart:fieldNameEnd])
		for idx := 0; idx < len(fieldName); idx++ {
			if fieldName[idx] == 0 {
				fieldName = fieldName[:idx]
				break
			}
		}

		// Extract field type
		fieldTypeStart := fieldNameEnd + 1 // Skip null terminator
		fieldTypeEnd := fieldTypeStart + int(fieldTypeLen)
		if fieldTypeEnd > len(resp.Data) {
			break
		}
		fieldType := string(resp.Data[fieldTypeStart:fieldTypeEnd])
		for idx := 0; idx < len(fieldType); idx++ {
			if fieldType[idx] == 0 {
				fieldType = fieldType[:idx]
				break
			}
		}

		fields = append(fields, StructField{
			Name:     fieldName,
			DataType: fieldType,
			Offset:   fieldOffset,
			Size:     fieldSize,
		})

		// Move to next sub-item using entryLength from header
		entryLength := binary.LittleEndian.Uint32(resp.Data[offset : offset+4])
		offset += int(entryLength)
	}

	return fields, nil
}

// FindFieldByPath recursively finds a field by path in the struct hierarchy
func FindFieldByPath(fields []StructField, path []string) (*StructField, error) {
	if len(path) == 0 {
		return nil, fmt.Errorf("empty path")
	}

	for i := range fields {
		if fields[i].Name == path[0] {
			if len(path) == 1 {
				return &fields[i], nil
			}
			// Recurse into nested fields
			return FindFieldByPath(fields[i].Fields, path[1:])
		}
	}

	return nil, fmt.Errorf("field %s not found", path[0])
}

// FindFieldByPathWithOffset recursively finds a field and calculates its absolute offset from the root
func FindFieldByPathWithOffset(fields []StructField, path []string, baseOffset uint32) (*StructField, uint32, error) {
	if len(path) == 0 {
		return nil, 0, fmt.Errorf("empty path")
	}

	for i := range fields {
		if fields[i].Name == path[0] {
			currentOffset := baseOffset + fields[i].Offset
			if len(path) == 1 {
				return &fields[i], currentOffset, nil
			}
			// Recurse into nested fields with updated base offset
			return FindFieldByPathWithOffset(fields[i].Fields, path[1:], currentOffset)
		}
	}

	return nil, 0, fmt.Errorf("field %s not found", path[0])
}

// FindNestedField recursively searches for a field by path (e.g., ["stTest", "sTest"])
func FindNestedField(fields []StructField, fieldPath []string, parentData []byte) (*StructField, []byte, error) {
	if len(fieldPath) == 0 {
		return nil, nil, fmt.Errorf("empty field path")
	}

	// Find the field in the current level
	var targetField *StructField
	for i := range fields {
		if fields[i].Name == fieldPath[0] {
			targetField = &fields[i]
			break
		}
	}

	if targetField == nil {
		return nil, nil, fmt.Errorf("field %s not found", fieldPath[0])
	}

	// Extract the data for this field
	fieldEnd := int(targetField.Offset) + int(targetField.Size)
	if fieldEnd > len(parentData) {
		return nil, nil, fmt.Errorf("field data out of range")
	}
	fieldData := parentData[targetField.Offset:fieldEnd]

	// If this is the final field in the path, return it
	if len(fieldPath) == 1 {
		return targetField, fieldData, nil
	}

	// Otherwise, recurse into nested fields
	if len(targetField.Fields) == 0 {
		return nil, nil, fmt.Errorf("field %s is not a struct", fieldPath[0])
	}

	return FindNestedField(targetField.Fields, fieldPath[1:], fieldData)
}
