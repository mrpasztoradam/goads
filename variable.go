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

// ReadVariable reads a variable value from the PLC
// Returns the raw data and the symbol information
func (c *Client) ReadVariable(ctx context.Context, targetAddr, senderAddr ams.Addr, name string) ([]byte, *Symbol, error) {
	// Get symbol information first to determine size
	symbol, err := c.GetSymbol(ctx, targetAddr, senderAddr, name)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get symbol info for %s: %w", name, err)
	}

	// Get symbol handle
	handle, err := c.GetSymHandleByName(ctx, targetAddr, senderAddr, name)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get handle for %s: %w", name, err)
	}

	// Read the value using the symbol size
	req := ams.NewReadRequest(
		targetAddr,
		senderAddr,
		0xF005, // ADSIGRP_SYM_VALBYHND
		handle,
		symbol.Size,
	)
	resp, err := c.Read(ctx, req)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read %s: %w", name, err)
	}

	return resp.Data, symbol, nil
}

// WriteVariable writes a variable value to the PLC
// The data should be properly encoded for the variable type
func (c *Client) WriteVariable(ctx context.Context, targetAddr, senderAddr ams.Addr, name string, data []byte) error {
	// Get symbol handle
	handle, err := c.GetSymHandleByName(ctx, targetAddr, senderAddr, name)
	if err != nil {
		return fmt.Errorf("failed to get handle for %s: %w", name, err)
	}

	// Write the value
	req := ams.NewWriteRequest(
		targetAddr,
		senderAddr,
		0xF005, // ADSIGRP_SYM_VALBYHND
		handle,
		data,
	)
	_, err = c.Write(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to write %s: %w", name, err)
	}

	return nil
}

// WriteNestedField writes a value to a nested field within a struct
func (c *Client) WriteNestedField(ctx context.Context, targetAddr, senderAddr ams.Addr, rootVar string, fieldPath []string, fieldData []byte) error {
	// First, get the symbol info to understand the structure
	symbol, err := c.GetSymbol(ctx, targetAddr, senderAddr, rootVar)
	if err != nil {
		return fmt.Errorf("failed to get symbol info: %w", err)
	}

	// Read the current struct data
	handle, err := c.GetSymHandleByName(ctx, targetAddr, senderAddr, rootVar)
	if err != nil {
		return fmt.Errorf("failed to get handle: %w", err)
	}

	req := ams.NewReadRequest(
		targetAddr,
		senderAddr,
		0xF005, // ADSIGRP_SYM_VALBYHND
		handle,
		symbol.Size,
	)
	resp, err := c.Read(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to read struct: %w", err)
	}

	// Get data type info if not already loaded
	if len(symbol.Fields) == 0 {
		fields, err := c.GetDataTypeInfo(ctx, targetAddr, senderAddr, symbol.DataType)
		if err != nil {
			return fmt.Errorf("failed to get data type info: %w", err)
		}
		symbol.Fields = fields
	}

	// Find the target field and calculate absolute offset
	field, absoluteOffset, err := FindFieldByPathWithOffset(symbol.Fields, fieldPath, 0)
	if err != nil {
		return fmt.Errorf("field not found: %w", err)
	}

	// Update the field data in the struct bytes using absolute offset
	fieldEnd := int(absoluteOffset) + int(field.Size)
	if fieldEnd > len(resp.Data) {
		return fmt.Errorf("field offset out of range")
	}
	if len(fieldData) != int(field.Size) {
		return fmt.Errorf("field data size mismatch: got %d, expected %d", len(fieldData), field.Size)
	}
	copy(resp.Data[absoluteOffset:fieldEnd], fieldData)

	// Write the entire struct back
	writeReq := ams.NewWriteRequest(
		targetAddr,
		senderAddr,
		0xF005, // ADSIGRP_SYM_VALBYHND
		handle,
		resp.Data,
	)
	_, err = c.Write(ctx, writeReq)
	if err != nil {
		return fmt.Errorf("failed to write struct: %w", err)
	}

	return nil
}

// PopulateFieldValues recursively populates field values from raw data
func PopulateFieldValues(c *Client, ctx context.Context, targetAddr, senderAddr ams.Addr, fields []StructField, data []byte) error {
	for i := range fields {
		fieldEnd := int(fields[i].Offset) + int(fields[i].Size)
		if fieldEnd > len(data) {
			continue
		}
		fieldData := data[fields[i].Offset:fieldEnd]

		// Check if this field is a struct itself
		if fields[i].Size > 8 {
			nestedFields, err := c.GetDataTypeInfo(ctx, targetAddr, senderAddr, fields[i].DataType)
			if err == nil && len(nestedFields) > 0 {
				// It's a nested struct - populate its fields recursively
				if err := PopulateFieldValues(c, ctx, targetAddr, senderAddr, nestedFields, fieldData); err != nil {
					return err
				}
				fields[i].Fields = nestedFields
				continue
			}
		}

		// It's a primitive type - decode the value
		fields[i].Value = DecodeFieldValue(fieldData, fields[i].DataType)
	}
	return nil
}
