// Copyright 2021 gotwincat authors. All rights reserved.
// Use of this source code is governed by a MIT-style license that can be
// found in the LICENSE file.

package ams

// DeviceNotificationRequest is the packet for an ADS Device Notification.
type DeviceNotificationRequest struct {
	tcpHeader  TCPHeader
	amsHeader  AMSHeader
	Length     uint32 // Length of notification data in bytes
	StampCount uint32 // Number of stamps
	Stamps     []NotificationStamp
}

// NotificationStamp represents a single notification in a device notification packet
type NotificationStamp struct {
	Timestamp   uint64 // Windows FILETIME (100ns intervals since 1601-01-01)
	SampleCount uint32 // Number of samples in this stamp
	Samples     []NotificationSample
}

// NotificationSample represents a single sample within a notification stamp
type NotificationSample struct {
	Handle uint32 // Notification handle
	Size   uint32 // Size of data
	Data   []byte // Notification data
}

func (r *DeviceNotificationRequest) Header() *AMSHeader {
	return &r.amsHeader
}

func (r *DeviceNotificationRequest) Encode(b *Buffer) error {
	b.WriteStruct(&r.tcpHeader)
	b.WriteStruct(&r.amsHeader)
	b.WriteUint32(r.Length)
	b.WriteUint32(r.StampCount)

	for i := range r.Stamps {
		// Write timestamp as 2x uint32 (little-endian uint64)
		b.WriteUint32(uint32(r.Stamps[i].Timestamp))
		b.WriteUint32(uint32(r.Stamps[i].Timestamp >> 32))
		b.WriteUint32(r.Stamps[i].SampleCount)

		for j := range r.Stamps[i].Samples {
			b.WriteUint32(r.Stamps[i].Samples[j].Handle)
			b.WriteUint32(r.Stamps[i].Samples[j].Size)
			b.WriteN(r.Stamps[i].Samples[j].Data, r.Stamps[i].Samples[j].Size)
		}
	}

	return b.Err()
}

func (r *DeviceNotificationRequest) Decode(b *Buffer) error {
	b.ReadStruct(&r.tcpHeader)
	b.ReadStruct(&r.amsHeader)

	// Check for errors after reading headers
	if b.Err() != nil {
		return b.Err()
	}

	// Read stream header: Length (uint32) and StampCount (uint32)
	r.Length = b.ReadUint32()
	if b.Err() != nil {
		return b.Err()
	}

	r.StampCount = b.ReadUint32()
	if b.Err() != nil {
		return b.Err()
	}

	// Empty notification
	if r.StampCount == 0 {
		r.Stamps = make([]NotificationStamp, 0)
		return b.Err()
	}

	// Parse stamps (loop StampCount times)
	r.Stamps = make([]NotificationStamp, r.StampCount)

	for i := uint32(0); i < r.StampCount; i++ {
		// Read timestamp (uint64) = 8 bytes
		low := b.ReadUint32()
		high := b.ReadUint32()
		r.Stamps[i].Timestamp = uint64(low) | (uint64(high) << 32)
		if b.Err() != nil {
			return b.Err()
		}

		// Read sample count = 4 bytes
		r.Stamps[i].SampleCount = b.ReadUint32()
		if b.Err() != nil {
			return b.Err()
		}

		// Read samples
		r.Stamps[i].Samples = make([]NotificationSample, r.Stamps[i].SampleCount)

		for j := uint32(0); j < r.Stamps[i].SampleCount; j++ {
			// Read handle = 4 bytes
			r.Stamps[i].Samples[j].Handle = b.ReadUint32()
			if b.Err() != nil {
				return b.Err()
			}

			// Read size = 4 bytes
			r.Stamps[i].Samples[j].Size = b.ReadUint32()
			if b.Err() != nil {
				return b.Err()
			}

			// Read data = size bytes
			r.Stamps[i].Samples[j].Data = b.ReadN(int(r.Stamps[i].Samples[j].Size))
			if b.Err() != nil {
				return b.Err()
			}
		}
	}

	return b.Err()
}

// IsDeviceNotificationRequest returns true if the packet is an ADS Device Notification.
func IsDeviceNotificationRequest(h AMSHeader) bool {
	return h.CmdID == CmdADSDeviceNotification
}
