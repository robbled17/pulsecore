//go:build windows

package main

import (
	"strconv"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	storageDeviceSeekPenaltyProperty = 7
	propertyQueryStandardInfo        = 0
)

type storagePropertyQuerySeek struct {
	PropertyID           uint32
	QueryType            uint32
	AdditionalParameters [1]byte
}

type deviceSeekPenaltyDescriptor struct {
	Version           uint32
	Size              uint32
	IncursSeekPenalty byte
	_                 [3]byte
}

// querySeekPenalty returns IncursSeekPenalty for physical disk N, or nil if unknown.
func querySeekPenalty(diskIndex int) *bool {
	if diskIndex < 0 {
		return nil
	}
	path := `\\.\PhysicalDrive` + strconv.Itoa(diskIndex)
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil
	}
	h, err := windows.CreateFile(
		pathPtr,
		0,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
		nil,
		windows.OPEN_EXISTING,
		0,
		0,
	)
	if err != nil {
		return nil
	}
	defer windows.CloseHandle(h)

	var q storagePropertyQuerySeek
	q.PropertyID = storageDeviceSeekPenaltyProperty
	q.QueryType = propertyQueryStandardInfo
	var out deviceSeekPenaltyDescriptor
	var ret uint32
	err = windows.DeviceIoControl(
		h,
		ioctlStorageQueryProperty,
		(*byte)(unsafe.Pointer(&q)),
		uint32(unsafe.Sizeof(q)),
		(*byte)(unsafe.Pointer(&out)),
		uint32(unsafe.Sizeof(out)),
		&ret,
		nil,
	)
	if err != nil || ret < 9 {
		return nil
	}
	v := out.IncursSeekPenalty != 0
	return &v
}
