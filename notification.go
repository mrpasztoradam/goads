package goads

import (
	"context"
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

	// Create AddDeviceNotification request
	req := ams.NewAddDeviceNotificationRequest(
		nm.session.targetAddr,
		nm.session.senderAddr,
		symbolInfo.IndexGroup,
		symbolInfo.IndexOffset,
		attribs.Length,
		uint32(attribs.TransMode),
		attribs.MaxDelay,
		attribs.CycleTime,
	)

	// Send the request
	resp, err := nm.session.client.AddDeviceNotification(ctx, req)
	if err != nil {
		return 0, fmt.Errorf("failed to add notification: %w", err)
	}

	if resp.Result != ams.NoError {
		return 0, fmt.Errorf("add notification error: %d", resp.Result)
	}

	notificationHandle := resp.NotificationHandle

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

	// Create DeleteDeviceNotification request
	req := ams.NewDeleteDeviceNotificationRequest(
		nm.session.targetAddr,
		nm.session.senderAddr,
		notificationHandle,
	)

	// Send the request
	resp, err := nm.session.client.DeleteDeviceNotification(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to delete notification: %w", err)
	}

	if resp.Result != ams.NoError {
		return fmt.Errorf("delete notification error: %d", resp.Result)
	}

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
	// Set up the client callback to receive notifications
	nm.session.client.SetNotificationCallback(func(req *ams.DeviceNotificationRequest) {
		// Process each stamp in the notification
		for _, stamp := range req.Stamps {
			// Convert Windows FILETIME to Go time
			// FILETIME is 100-nanosecond intervals since January 1, 1601
			const ticksPerSecond = 10000000
			const epochDiff = 11644473600 // Seconds between 1601 and 1970
			secs := int64(stamp.Timestamp)/ticksPerSecond - epochDiff
			nsecs := (int64(stamp.Timestamp) % ticksPerSecond) * 100
			timestamp := time.Unix(secs, nsecs)

			// Process each sample in the stamp
			for _, sample := range stamp.Samples {
				nm.mu.RLock()
				handler, ok := nm.handlers[sample.Handle]
				nm.mu.RUnlock()

				if ok && handler.callback != nil {
					// Call the user's callback with the notification data
					handler.callback(NotificationSample{
						Handle:    sample.Handle,
						Timestamp: timestamp,
						Data:      sample.Data,
					})
				}
			}
		}
	})

	<-nm.stopCh

	// Clear the callback when stopping
	nm.session.client.SetNotificationCallback(nil)
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
