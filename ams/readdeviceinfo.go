package ams

type ReadDeviceInfoRequest struct {
	tcpHeader TCPHeader
	amsHeader AMSHeader
}

func NewReadDeviceInfoRequest(target, sender Addr) *ReadDeviceInfoRequest {
	return &ReadDeviceInfoRequest{
		tcpHeader: TCPHeader{
			Length: amsHeaderLen,
		},
		amsHeader: AMSHeader{
			Target:     target,
			Sender:     sender,
			CmdID:      CmdADSReadDeviceInfo,
			StateFlags: StateADSCommand,
		},
	}
}

func (r *ReadDeviceInfoRequest) Header() *AMSHeader {
	return &r.amsHeader
}

func (r *ReadDeviceInfoRequest) Encode(b *Buffer) error {
	b.WriteStruct(&r.tcpHeader)
	b.WriteStruct(&r.amsHeader)
	return b.Err()
}

func (r *ReadDeviceInfoRequest) Decode(b *Buffer) error {
	b.ReadStruct(&r.tcpHeader)
	b.ReadStruct(&r.amsHeader)
	return b.Err()
}

func IsReadDeviceInfoRequest(h AMSHeader) bool {
	return h.CmdID == CmdADSReadDeviceInfo && h.StateFlags == StateADSCommand
}

type ReadDeviceInfoResponse struct {
	tcpHeader    TCPHeader
	amsHeader    AMSHeader
	Result       uint32
	MajorVersion uint8
	MinorVersion uint8
	BuildVersion uint16
	DeviceName   [16]byte
}

func NewReadDeviceInfoResponse(target, sender Addr, result uint32, major, minor uint8, build uint16, deviceName string) *ReadDeviceInfoResponse {
	resp := &ReadDeviceInfoResponse{
		tcpHeader: TCPHeader{
			Length: amsHeaderLen + 24,
		},
		amsHeader: AMSHeader{
			Target:     target,
			Sender:     sender,
			CmdID:      CmdADSReadDeviceInfo,
			StateFlags: StateADSCommand | StateResponse,
			Length:     24,
		},
		Result:       result,
		MajorVersion: major,
		MinorVersion: minor,
		BuildVersion: build,
	}

	// Copy device name into fixed-size array
	copy(resp.DeviceName[:], deviceName)

	return resp
}

func (r *ReadDeviceInfoResponse) Header() *AMSHeader {
	return &r.amsHeader
}

func (r *ReadDeviceInfoResponse) Encode(b *Buffer) error {
	b.WriteStruct(&r.tcpHeader)
	b.WriteStruct(&r.amsHeader)
	b.WriteUint32(r.Result)
	b.WriteUint8(r.MajorVersion)
	b.WriteUint8(r.MinorVersion)
	b.WriteUint16(r.BuildVersion)
	b.Write(r.DeviceName[:])
	return b.Err()
}

func (r *ReadDeviceInfoResponse) Decode(b *Buffer) error {
	b.ReadStruct(&r.tcpHeader)
	b.ReadStruct(&r.amsHeader)
	r.Result = b.ReadUint32()
	r.MajorVersion = b.ReadUint8()
	r.MinorVersion = b.ReadUint8()
	r.BuildVersion = b.ReadUint16()
	b.Read(r.DeviceName[:])
	return b.Err()
}

// GetDeviceName returns the device name as a string, trimming null bytes
func (r *ReadDeviceInfoResponse) GetDeviceName() string {
	// Find the first null byte
	for i, b := range r.DeviceName {
		if b == 0 {
			return string(r.DeviceName[:i])
		}
	}
	return string(r.DeviceName[:])
}

// IsReadDeviceInfoResponse returns true if the packet is a read device info response.
func IsReadDeviceInfoResponse(h AMSHeader) bool {
	return h.CmdID == CmdADSReadDeviceInfo && HasState(h, StateResponse)
}
