package goads

import (
	"context"
	"encoding/binary"
	"fmt"
	"sync"
	"time"

	"github.com/mrpasztoradam/goads/ams"
)

// NotificationMode defines how notifications are triggered
type NotificationMode uint32

const (
	// NotificationCyclic triggers notifications cyclically
	NotificationCyclic NotificationMode = 3
	// NotificationOnChange triggers notifications when value changes
	NotificationOnChange NotificationMode = 4
)

// NotificationTransMode defines when notifications are transmitted
type NotificationTransMode uint32

const (
	// TransModeServerCycle transmits notifications on server cycle
	TransModeServerCycle NotificationTransMode = 3
	// TransModeServerOnChange transmits notifications immediately on change
	TransModeServerOnChange NotificationTransMode = 4
	// TransModeCyclic transmits notifications cyclically
	TransModeCyclic NotificationTransMode = 10
)

// NotificationAttribs defines attributes for a notification request
type NotificationAttribs struct {
	Length    uint32                // Length of data in bytes
	TransMode NotificationTransMode // Transmission mode
	MaxDelay  uint32                // Maximum delay in 100ns units
	CycleTime uint32                // Cycle time in 100ns units
	Reserved  [16]byte              // Reserved for future use
}

// NotificationSample contains a notification data sample
type NotificationSample struct {
	Handle    uint32    // Notification handle
	Timestamp time.Time // Timestamp of notification
	Data      []byte    // Notification data
}

// NotificationCallback is called when a notification is received
type NotificationCallback func(sample NotificationSample)

// notificationHandler manages notifications for a specific handle
type notificationHandler struct {
	handle     uint32
	varName    string
	varHandle  uint32 // ADS variable handle
	callback   NotificationCallback
	symbolInfo *SymbolInfo
}

// NotificationManager manages ADS device notifications
type NotificationManager struct {
	session  *Session
	handlers map[uint32]*notificationHandler
	mu       sync.RWMutex
	stopCh   chan struct{}
	running  bool
}

// NewNotificationManager creates a new notification manager for a session
func (s *Session) NewNotificationManager() *NotificationManager {
	return &NotificationManager{
		session:  s,
		handlers: make(map[uint32]*notificationHandler),
		stopCh:   make(chan struct{}),
	}
}

// Subscribe creates a notification subscription for a variable
func (nm *NotificationManager) Subscribe(
	ctx context.Context,
	varName string,
	cycleTime time.Duration,
	callback NotificationCallback,
) (uint32, error) {
	// Get or create variable handle
	handle, err := nm.session.getOrCreateHandle(ctx, varName)
	if err != nil {
		return 0, fmt.Errorf("failed to get handle for %s: %w", varName, err)
	}

	// Get symbol info for data length
	symbolInfo, ok := nm.session.registry.Get(varName)
	if !ok {
		return 0, fmt.Errorf("symbol %s not found in registry", varName)
	}

	// Create notification attributes
	attribs := NotificationAttribs{
		Length:    symbolInfo.Size,
		TransMode: TransModeServerOnChange,
		MaxDelay:  uint32(cycleTime.Nanoseconds() / 100), // Convert to 100ns units
		CycleTime: uint32(cycleTime.Nanoseconds() / 100),
	}

	// Build request data: handle (4) + attribs (32)
	reqData := make([]byte, 36)
	binary.LittleEndian.PutUint32(reqData[0:4], handle)
	binary.LittleEndian.PutUint32(reqData[4:8], attribs.Length)
	binary.LittleEndian.PutUint32(reqData[8:12], uint32(attribs.TransMode))
	binary.LittleEndian.PutUint32(reqData[12:16], attribs.MaxDelay)
	binary.LittleEndian.PutUint32(reqData[16:20], attribs.CycleTime)

	// Send add device notification request using ReadWrite
	// Note: We need to use the low-level client methods here
	// For now, this is a simplified implementation that doesn't fully support notifications
	// A complete implementation would require modifying the client's receive loop

	// This is a placeholder - actual notification support requires:
	// 1. Modifying client.go to handle CmdADSDeviceNotification packets
	// 2. Dispatching those packets to the notification manager
	// 3. Parsing notification stamps and calling callbacks

	req := ams.NewReadWriteRequest(
		nm.session.targetAddr,
		nm.session.senderAddr,
		ams.IdxReadWriteSymValueByHandle,
		handle,
		4, // Read 4 bytes (notification handle)
		reqData,
	)

	// For now, return an error indicating this feature is not yet fully implemented
	_ = req
	return 0, fmt.Errorf("notification support requires client modifications (not yet implemented)")
	/*
		// This code would work if client supported notifications:

		resp, err := nm.session.client.SendRequest(ctx, req)
		if err != nil {
			return 0, fmt.Errorf("failed to add notification: %w", err)
		}

		respRW, ok := resp.(*ams.ReadWriteResponse)
		if !ok {
			return 0, fmt.Errorf("unexpected response type")
		}

		if respRW.Result != 0 {
			return 0, fmt.Errorf("add notification error: %d", respRW.Result)
		}

		// Extract notification handle from response (first 4 bytes)
		if len(respRW.Data) < 4 {
			return 0, fmt.Errorf("invalid notification response")
		}
		notificationHandle := binary.LittleEndian.Uint32(respRW.Data[0:4])

		// Store handler
		nm.mu.Lock()
		nm.handlers[notificationHandle] = &notificationHandler{
			handle:     notificationHandle,
			varName:    varName,
			varHandle:  handle,
			callback:   callback,
			symbolInfo: symbolInfo,
		}
		nm.mu.Unlock()

		return notificationHandle, nil
	*/
}

// Unsubscribe removes a notification subscription
func (nm *NotificationManager) Unsubscribe(ctx context.Context, notificationHandle uint32) error {
	nm.mu.Lock()
	handler, exists := nm.handlers[notificationHandle]
	if !exists {
		nm.mu.Unlock()
		return fmt.Errorf("notification handle %d not found", notificationHandle)
	}
	delete(nm.handlers, notificationHandle)
	nm.mu.Unlock()

	// Build request data: notification handle (4 bytes)
	reqData := make([]byte, 4)
	binary.LittleEndian.PutUint32(reqData, notificationHandle)

	// This is a placeholder - actual implementation would send delete notification
	_ = reqData

	// Release the variable handle if no longer needed
	// Note: In practice, you may want to keep handles cached
	_ = handler.varHandle

	return nil
}

// Start begins processing notifications
func (nm *NotificationManager) Start() error {
	nm.mu.Lock()
	if nm.running {
		nm.mu.Unlock()
		return fmt.Errorf("notification manager already running")
	}
	nm.running = true
	nm.stopCh = make(chan struct{})
	nm.mu.Unlock()

	// Start goroutine to process notifications
	go nm.processNotifications()

	return nil
}

// Stop stops processing notifications
func (nm *NotificationManager) Stop() {
	nm.mu.Lock()
	if !nm.running {
		nm.mu.Unlock()
		return
	}
	nm.running = false
	close(nm.stopCh)
	nm.mu.Unlock()
}

// processNotifications processes incoming notification packets
func (nm *NotificationManager) processNotifications() {
	// This is a placeholder - in a real implementation, you would:
	// 1. Listen for ADS device notification packets (command ID 8)
	// 2. Parse the notification stamps from the packet
	// 3. Look up the handler for each notification handle
	// 4. Call the callback with the notification data
	//
	// For now, this would require integration with the client's receive loop
	// to dispatch notification packets separately from request/response packets

	<-nm.stopCh
}

// UnsubscribeAll removes all notification subscriptions
func (nm *NotificationManager) UnsubscribeAll(ctx context.Context) error {
	nm.mu.Lock()
	handles := make([]uint32, 0, len(nm.handlers))
	for h := range nm.handlers {
		handles = append(handles, h)
	}
	nm.mu.Unlock()

	var lastErr error
	for _, h := range handles {
		if err := nm.Unsubscribe(ctx, h); err != nil {
			lastErr = err
		}
	}

	return lastErr
}
