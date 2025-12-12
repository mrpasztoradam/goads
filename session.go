// Copyright 2021 gotwincat authors. All rights reserved.
// Use of this source code is governed by a MIT-style license that can be
// found in the LICENSE file.

package twincat

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/mrpasztoradam/goads/ams"
)

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Session represents a cached ADS session with a specific target
type Session struct {
	client     *Client
	targetAddr ams.Addr
	senderAddr ams.Addr
	registry   *SymbolRegistry
	mu         sync.RWMutex
}

// SymbolInfo contains cached information about a PLC symbol
type SymbolInfo struct {
	Name       string        `json:"name"`
	DataType   string        `json:"dataType"`
	Size       uint32        `json:"size"`
	IndexGroup uint32        `json:"indexGroup"`
	IndexOffset uint32       `json:"indexOffset"`
	Handle     uint32        `json:"handle,omitempty"`
	Comment    string        `json:"comment,omitempty"`
	Fields     []StructField `json:"fields,omitempty"`
}

// SymbolRegistry holds cached symbol information
type SymbolRegistry struct {
	symbols map[string]*SymbolInfo
	mu      sync.RWMutex
}

// NewSymbolRegistry creates a new symbol registry
func NewSymbolRegistry() *SymbolRegistry {
	return &SymbolRegistry{
		symbols: make(map[string]*SymbolInfo),
	}
}

// Get retrieves a symbol from the registry
func (r *SymbolRegistry) Get(name string) (*SymbolInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	info, ok := r.symbols[name]
	return info, ok
}

// Set adds or updates a symbol in the registry
func (r *SymbolRegistry) Set(name string, info *SymbolInfo) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.symbols[name] = info
}

// GetAll returns all symbols
func (r *SymbolRegistry) GetAll() map[string]*SymbolInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	// Return a copy to avoid concurrent access issues
	result := make(map[string]*SymbolInfo, len(r.symbols))
	for k, v := range r.symbols {
		result[k] = v
	}
	return result
}

// Count returns the number of cached symbols
func (r *SymbolRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.symbols)
}

// NewSession creates a new ADS session with the specified target
func (c *Client) NewSession(targetAddr, senderAddr ams.Addr) *Session {
	return &Session{
		client:     c,
		targetAddr: targetAddr,
		senderAddr: senderAddr,
		registry:   NewSymbolRegistry(),
	}
}

// LoadSymbolTable loads the entire symbol table from the PLC using ADS native upload
// This is the most efficient way to load all symbols at once
func (s *Session) LoadSymbolTable(ctx context.Context) error {
	// First, try to get upload info (0xF00C ADSIGRP_SYM_UPLOADINFO2)
	// This tells us the size of the symbol table
	infoReq := ams.NewReadRequest(
		s.targetAddr,
		s.senderAddr,
		0xF00C, // ADSIGRP_SYM_UPLOADINFO2
		0x0,
		0x30, // 48 bytes for upload info structure
	)

	infoResp, err := s.client.Read(ctx, infoReq)
	if err != nil {
		return fmt.Errorf("failed to get symbol upload info: %w", err)
	}

	// Debug: log info response
	fmt.Printf("DEBUG: Upload info response size: %d bytes\n", len(infoResp.Data))
	if len(infoResp.Data) >= 24 {
		symbolCount := binary.LittleEndian.Uint32(infoResp.Data[0:4])
		symbolLength := binary.LittleEndian.Uint32(infoResp.Data[4:8])
		fmt.Printf("DEBUG: Symbol count from info: %d, total length: %d bytes\n", symbolCount, symbolLength)
		
		// If no symbols, return early
		if symbolCount == 0 {
			return nil
		}
	}

	// Now upload the actual symbol table (0xF00B ADSIGRP_SYM_UPLOAD)
	req := ams.NewReadRequest(
		s.targetAddr,
		s.senderAddr,
		0xF00B, // ADSIGRP_SYM_UPLOAD
		0x0,
		0xFFFFFF, // Request large buffer for symbol table
	)

	resp, err := s.client.Read(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to upload symbol table: %w", err)
	}

	// Debug: log response size
	fmt.Printf("DEBUG: Symbol table upload response size: %d bytes\n", len(resp.Data))
	if len(resp.Data) > 0 {
		fmt.Printf("DEBUG: First 64 bytes: %X\n", resp.Data[:min(64, len(resp.Data))])
	}

	// Parse the symbol table
	offset := 0
	symbolCount := 0

	for offset < len(resp.Data) {
		// Check if we have enough data for the header
		if offset+30 > len(resp.Data) {
			break
		}

		// Parse symbol entry structure
		entryLength := binary.LittleEndian.Uint32(resp.Data[offset : offset+4])
		if entryLength == 0 || offset+int(entryLength) > len(resp.Data) {
			break
		}

		indexGroup := binary.LittleEndian.Uint32(resp.Data[offset+4 : offset+8])
		indexOffset := binary.LittleEndian.Uint32(resp.Data[offset+8 : offset+12])
		size := binary.LittleEndian.Uint32(resp.Data[offset+12 : offset+16])
		nameLength := binary.LittleEndian.Uint16(resp.Data[offset+24 : offset+26])
		typeLength := binary.LittleEndian.Uint16(resp.Data[offset+26 : offset+28])
		commentLength := binary.LittleEndian.Uint16(resp.Data[offset+28 : offset+30])

		// Extract name
		nameStart := offset + 30
		nameEnd := nameStart + int(nameLength)
		if nameEnd > len(resp.Data) {
			break
		}
		name := nullTerminatedString(resp.Data[nameStart:nameEnd])

		// Extract type
		typeStart := nameEnd + 1 // Skip null terminator
		typeEnd := typeStart + int(typeLength)
		if typeEnd > len(resp.Data) {
			break
		}
		dataType := nullTerminatedString(resp.Data[typeStart:typeEnd])

		// Extract comment (optional)
		var comment string
		if commentLength > 0 {
			commentStart := typeEnd + 1
			commentEnd := commentStart + int(commentLength)
			if commentEnd <= len(resp.Data) {
				comment = nullTerminatedString(resp.Data[commentStart:commentEnd])
			}
		}

		// Store in registry
		info := &SymbolInfo{
			Name:        name,
			DataType:    dataType,
			Size:        size,
			IndexGroup:  indexGroup,
			IndexOffset: indexOffset,
			Comment:     comment,
		}
		s.registry.Set(name, info)
		symbolCount++

		// Move to next entry
		offset += int(entryLength)
	}

	return nil
}

// GetSymbol retrieves symbol information, using cache if available
func (s *Session) GetSymbol(ctx context.Context, name string) (*SymbolInfo, error) {
	// Check cache first
	if info, ok := s.registry.Get(name); ok {
		return info, nil
	}

	// Not in cache, fetch from PLC
	symbol, err := s.client.GetSymbol(ctx, s.targetAddr, s.senderAddr, name)
	if err != nil {
		return nil, err
	}

	// Create info and cache it
	info := &SymbolInfo{
		Name:     symbol.Name,
		DataType: symbol.DataType,
		Size:     symbol.Size,
		Fields:   symbol.Fields,
	}
	s.registry.Set(name, info)

	return info, nil
}

// getOrCreateHandle gets a symbol handle, using cache if available
func (s *Session) getOrCreateHandle(ctx context.Context, name string) (uint32, error) {
	// Check if we have it in registry with handle
	if info, ok := s.registry.Get(name); ok && info.Handle != 0 {
		return info.Handle, nil
	}

	// Get handle from PLC
	handle, err := s.client.GetSymHandleByName(ctx, s.targetAddr, s.senderAddr, name)
	if err != nil {
		return 0, err
	}

	// Update cache
	if info, ok := s.registry.Get(name); ok {
		info.Handle = handle
		s.registry.Set(name, info)
	} else {
		// Create minimal info with handle
		s.registry.Set(name, &SymbolInfo{
			Name:   name,
			Handle: handle,
		})
	}

	return handle, nil
}

// Read reads a variable value from the PLC (cached handle)
func (s *Session) Read(ctx context.Context, name string) ([]byte, *SymbolInfo, error) {
	// Get symbol info (from cache or PLC)
	info, err := s.GetSymbol(ctx, name)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get symbol info: %w", err)
	}

	// Get or create handle
	handle, err := s.getOrCreateHandle(ctx, name)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get handle: %w", err)
	}

	// Read the value
	req := ams.NewReadRequest(
		s.targetAddr,
		s.senderAddr,
		0xF005, // ADSIGRP_SYM_VALBYHND
		handle,
		info.Size,
	)
	resp, err := s.client.Read(ctx, req)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read %s: %w", name, err)
	}

	return resp.Data, info, nil
}

// Write writes a variable value to the PLC (cached handle)
func (s *Session) Write(ctx context.Context, name string, data []byte) error {
	// Get or create handle
	handle, err := s.getOrCreateHandle(ctx, name)
	if err != nil {
		return fmt.Errorf("failed to get handle: %w", err)
	}

	// Write the value
	req := ams.NewWriteRequest(
		s.targetAddr,
		s.senderAddr,
		0xF005, // ADSIGRP_SYM_VALBYHND
		handle,
		data,
	)
	_, err = s.client.Write(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to write %s: %w", name, err)
	}

	return nil
}

// WriteNestedField writes a value to a nested field within a struct
func (s *Session) WriteNestedField(ctx context.Context, rootVar string, fieldPath []string, fieldData []byte) error {
	// Get symbol info
	info, err := s.GetSymbol(ctx, rootVar)
	if err != nil {
		return fmt.Errorf("failed to get symbol info: %w", err)
	}

	// Get or create handle
	handle, err := s.getOrCreateHandle(ctx, rootVar)
	if err != nil {
		return fmt.Errorf("failed to get handle: %w", err)
	}

	// Read current struct data
	req := ams.NewReadRequest(
		s.targetAddr,
		s.senderAddr,
		0xF005,
		handle,
		info.Size,
	)
	resp, err := s.client.Read(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to read struct: %w", err)
	}

	// Load fields if needed
	if len(info.Fields) == 0 {
		fields, err := s.client.GetDataTypeInfo(ctx, s.targetAddr, s.senderAddr, info.DataType)
		if err != nil {
			return fmt.Errorf("failed to get data type info: %w", err)
		}
		info.Fields = fields
		s.registry.Set(rootVar, info)
	}

	// Find field and update data
	field, absoluteOffset, err := FindFieldByPathWithOffset(info.Fields, fieldPath, 0)
	if err != nil {
		return fmt.Errorf("field not found: %w", err)
	}

	fieldEnd := int(absoluteOffset) + int(field.Size)
	if fieldEnd > len(resp.Data) || len(fieldData) != int(field.Size) {
		return fmt.Errorf("field data size mismatch")
	}
	copy(resp.Data[absoluteOffset:fieldEnd], fieldData)

	// Write back
	writeReq := ams.NewWriteRequest(
		s.targetAddr,
		s.senderAddr,
		0xF005,
		handle,
		resp.Data,
	)
	_, err = s.client.Write(ctx, writeReq)
	return err
}

// ReleaseHandle releases a symbol handle
func (s *Session) ReleaseHandle(ctx context.Context, handle uint32) error {
	// Use ADSIGRP_SYM_RELEASEHND (0xF006)
	data := make([]byte, 4)
	binary.LittleEndian.PutUint32(data, handle)

	req := ams.NewWriteRequest(
		s.targetAddr,
		s.senderAddr,
		0xF006, // ADSIGRP_SYM_RELEASEHND
		0,
		data,
	)
	_, err := s.client.Write(ctx, req)
	return err
}

// Close releases all cached handles
func (s *Session) Close(ctx context.Context) error {
	allSymbols := s.registry.GetAll()
	
	var firstErr error
	for _, info := range allSymbols {
		if info.Handle != 0 {
			if err := s.ReleaseHandle(ctx, info.Handle); err != nil && firstErr == nil {
				firstErr = err
			}
		}
	}
	
	return firstErr
}

// ExportSymbolsToJSON exports the symbol registry to a JSON file
func (s *Session) ExportSymbolsToJSON(filename string) error {
	allSymbols := s.registry.GetAll()

	// Convert map to sorted slice for better readability
	symbols := make([]*SymbolInfo, 0, len(allSymbols))
	for _, info := range allSymbols {
		symbols = append(symbols, info)
	}

	data, err := json.MarshalIndent(symbols, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal symbols: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// GetSymbolCount returns the number of cached symbols
func (s *Session) GetSymbolCount() int {
	return s.registry.Count()
}

// HasSymbol checks if a symbol exists in the cache
func (s *Session) HasSymbol(name string) bool {
	_, ok := s.registry.Get(name)
	return ok
}

// nullTerminatedString extracts a null-terminated string from a byte slice
func nullTerminatedString(data []byte) string {
	for i, b := range data {
		if b == 0 {
			return string(data[:i])
		}
	}
	return string(data)
}
