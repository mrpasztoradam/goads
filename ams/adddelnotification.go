// Copyright 2021 gotwincat authors. All rights reserved.
// Use of this source code is governed by a MIT-style license that can be
// found in the LICENSE file.

package ams

// AddDeviceNotificationRequest is the packet for adding a device notification.
type AddDeviceNotificationRequest struct {
	tcpHeader  TCPHeader
	amsHeader  AMSHeader
	IndexGroup uint32
	IndexOff   uint32
	Length     uint32 // Length of data to monitor
	TransMode  uint32 // Transmission mode
	MaxDelay   uint32 // Maximum delay in 100ns units
	CycleTime  uint32 // Cycle time in 100ns units
	Reserved   [16]byte
}

func (r *AddDeviceNotificationRequest) Header() *AMSHeader {
	return &r.amsHeader
}

func (r *AddDeviceNotificationRequest) Encode(b *Buffer) error {
	r.tcpHeader.Length = uint32(amsHeaderLen + 40) // 40 bytes for data
	r.amsHeader.Length = 40
	r.amsHeader.CmdID = CmdADSAddDeviceNotification
	b.WriteStruct(&r.tcpHeader)
	b.WriteStruct(&r.amsHeader)
	b.WriteUint32(r.IndexGroup)
	b.WriteUint32(r.IndexOff)
	b.WriteUint32(r.Length)
	b.WriteUint32(r.TransMode)
	b.WriteUint32(r.MaxDelay)
	b.WriteUint32(r.CycleTime)
	b.WriteN(r.Reserved[:], 16)
	return b.Err()
}

func (r *AddDeviceNotificationRequest) Decode(b *Buffer) error {
	b.ReadStruct(&r.tcpHeader)
	b.ReadStruct(&r.amsHeader)
	r.IndexGroup = b.ReadUint32()
	r.IndexOff = b.ReadUint32()
	r.Length = b.ReadUint32()
	r.TransMode = b.ReadUint32()
	r.MaxDelay = b.ReadUint32()
	r.CycleTime = b.ReadUint32()
	copy(r.Reserved[:], b.ReadN(16))
	return b.Err()
}

// AddDeviceNotificationResponse is the response for adding a device notification.
type AddDeviceNotificationResponse struct {
	tcpHeader          TCPHeader
	amsHeader          AMSHeader
	Result             uint32 // Error code
	NotificationHandle uint32 // Notification handle
}

func (r *AddDeviceNotificationResponse) Header() *AMSHeader {
	return &r.amsHeader
}

func (r *AddDeviceNotificationResponse) Encode(b *Buffer) error {
	r.tcpHeader.Length = uint32(amsHeaderLen + 8)
	r.amsHeader.Length = 8
	r.amsHeader.CmdID = CmdADSAddDeviceNotification
	r.amsHeader.StateFlags |= StateResponse
	b.WriteStruct(&r.tcpHeader)
	b.WriteStruct(&r.amsHeader)
	b.WriteUint32(r.Result)
	b.WriteUint32(r.NotificationHandle)
	return b.Err()
}

func (r *AddDeviceNotificationResponse) Decode(b *Buffer) error {
	b.ReadStruct(&r.tcpHeader)
	b.ReadStruct(&r.amsHeader)
	r.Result = b.ReadUint32()
	r.NotificationHandle = b.ReadUint32()
	return b.Err()
}

// IsAddDeviceNotificationResponse returns true if the packet is an Add Device Notification response.
func IsAddDeviceNotificationResponse(h AMSHeader) bool {
	return h.CmdID == CmdADSAddDeviceNotification && HasState(h, StateResponse)
}

// DeleteDeviceNotificationRequest is the packet for deleting a device notification.
type DeleteDeviceNotificationRequest struct {
	tcpHeader          TCPHeader
	amsHeader          AMSHeader
	NotificationHandle uint32
}

func (r *DeleteDeviceNotificationRequest) Header() *AMSHeader {
	return &r.amsHeader
}

func (r *DeleteDeviceNotificationRequest) Encode(b *Buffer) error {
	r.tcpHeader.Length = uint32(amsHeaderLen + 4)
	r.amsHeader.Length = 4
	r.amsHeader.CmdID = CmdADSDeleteDeviceNotification
	b.WriteStruct(&r.tcpHeader)
	b.WriteStruct(&r.amsHeader)
	b.WriteUint32(r.NotificationHandle)
	return b.Err()
}

func (r *DeleteDeviceNotificationRequest) Decode(b *Buffer) error {
	b.ReadStruct(&r.tcpHeader)
	b.ReadStruct(&r.amsHeader)
	r.NotificationHandle = b.ReadUint32()
	return b.Err()
}

// DeleteDeviceNotificationResponse is the response for deleting a device notification.
type DeleteDeviceNotificationResponse struct {
	tcpHeader TCPHeader
	amsHeader AMSHeader
	Result    uint32
}

func (r *DeleteDeviceNotificationResponse) Header() *AMSHeader {
	return &r.amsHeader
}

func (r *DeleteDeviceNotificationResponse) Encode(b *Buffer) error {
	r.tcpHeader.Length = uint32(amsHeaderLen + 4)
	r.amsHeader.Length = 4
	r.amsHeader.CmdID = CmdADSDeleteDeviceNotification
	r.amsHeader.StateFlags |= StateResponse
	b.WriteStruct(&r.tcpHeader)
	b.WriteStruct(&r.amsHeader)
	b.WriteUint32(r.Result)
	return b.Err()
}

func (r *DeleteDeviceNotificationResponse) Decode(b *Buffer) error {
	b.ReadStruct(&r.tcpHeader)
	b.ReadStruct(&r.amsHeader)
	r.Result = b.ReadUint32()
	return b.Err()
}

// IsDeleteDeviceNotificationResponse returns true if the packet is a Delete Device Notification response.
func IsDeleteDeviceNotificationResponse(h AMSHeader) bool {
	return h.CmdID == CmdADSDeleteDeviceNotification && HasState(h, StateResponse)
}

// NewAddDeviceNotificationRequest creates a new AddDeviceNotification request.
func NewAddDeviceNotificationRequest(
	target, sender Addr,
	indexGroup, indexOffset uint32,
	length, transMode, maxDelay, cycleTime uint32,
) *AddDeviceNotificationRequest {
	return &AddDeviceNotificationRequest{
		tcpHeader: TCPHeader{},
		amsHeader: AMSHeader{
			Target:     target,
			Sender:     sender,
			CmdID:      CmdADSAddDeviceNotification,
			StateFlags: StateADSCommand,
		},
		IndexGroup: indexGroup,
		IndexOff:   indexOffset,
		Length:     length,
		TransMode:  transMode,
		MaxDelay:   maxDelay,
		CycleTime:  cycleTime,
	}
}

// NewDeleteDeviceNotificationRequest creates a new DeleteDeviceNotification request.
func NewDeleteDeviceNotificationRequest(
	target, sender Addr,
	notificationHandle uint32,
) *DeleteDeviceNotificationRequest {
	return &DeleteDeviceNotificationRequest{
		tcpHeader: TCPHeader{},
		amsHeader: AMSHeader{
			Target:     target,
			Sender:     sender,
			CmdID:      CmdADSDeleteDeviceNotification,
			StateFlags: StateADSCommand,
		},
		NotificationHandle: notificationHandle,
	}
}
