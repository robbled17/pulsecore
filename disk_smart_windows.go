//go:build windows

package main

import (
	"encoding/binary"
	"fmt"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	ioctlStorageQueryProperty  = 0x002d1400
	ioctlATAPassThrough        = 0x0004d02c
	ioctlATAPassThroughDirect  = 0x0004d030
	storageDeviceProperty      = 0
	storageDeviceProtoSpecific = 49 // StorageDeviceProtocolSpecificProperty
	protocolTypeNvme           = 3
	nvmeDataTypeLogPage        = 2
	nvmeLogSmartHealth         = 0x02
	ataPassThroughTimeout      = 10
	ataFlagsDRFRDY             = 0x01
	ataFlagsDataIn             = 0x02
	smartCmd                   = 0xB0
	smartReadData              = 0xD0
	smartReadThresholds        = 0xD1
	smartCylinderLow           = 0x4F
	smartCylinderHigh          = 0xC2
)

// SmartAttribute is one ATA SMART attribute row (CrystalDiskInfo-style).
type SmartAttribute struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	NameRu    string `json:"nameRu"`
	Current   int    `json:"current"`
	Worst     int    `json:"worst"`
	Threshold int    `json:"threshold"`
	Raw       string `json:"raw"`
	RawValue  uint64 `json:"rawValue"`
}

// smartIOCTLResult is raw data from native SMART/NVMe health pages.
type smartIOCTLResult struct {
	OK            bool
	Bus           string // nvme|ata|unknown
	NeedsAdmin    bool
	AdminReasonRu string
	AdminReasonEn string
	UnavailableRu string
	UnavailableEn string

	Firmware      string
	TemperatureC  float64
	TempOK        bool
	HealthPercent float64
	HealthOK      bool
	HealthSource  string // nvme_percentage_used|sata_wear|hdd_sectors|storage_api
	AvailableSpare       int
	AvailableSpareThresh int
	PercentageUsed       int

	DataUnitsRead    uint64
	DataUnitsWritten uint64
	HostReadGB       float64
	HostWriteGB      float64
	PowerOnHours     float64
	PowerCycles      float64
	UnsafeShutdowns  float64
	MediaErrors      float64

	Reallocated int64
	Pending     int64
	Uncorrectable int64
	CriticalWarning byte

	// IdentifyWord217 is ATA IDENTIFY DEVICE word 217 (Nominal Media Rotation Rate).
	// 1 = SSD; 0x0401-0xFFFE = HDD RPM; 0/0xFFFF = not reported; -1 = not read.
	IdentifyWord217   int
	IdentifyWord217OK bool

	Attributes []SmartAttribute
}

type storagePropertyQueryHdr struct {
	PropertyID          uint32
	QueryType           uint32
	AdditionalParameters [1]byte
}

type storageProtocolSpecificData struct {
	ProtocolType                 uint32
	DataType                     uint32
	ProtocolDataRequestValue     uint32
	ProtocolDataRequestSubValue  uint32
	ProtocolDataRequestSubValue2 uint32
	ProtocolDataRequestSubValue3 uint32
	ProtocolDataOffset           uint32
	ProtocolDataLength           uint32
	FixedProtocolReturnData      uint32
	Reserved1                    [3]uint32
}

type ataPassThroughEx struct {
	Length              uint16
	AtaFlags            uint16
	PathId              uint8
	TargetId            uint8
	Lun                 uint8
	ReservedAsUchar     uint8
	DataTransferLength  uint32
	TimeOutValue        uint32
	ReservedAsUlong     uint32
	DataBufferOffset    uint64
	PreviousTaskFile    [8]uint8
	CurrentTaskFile     [8]uint8
}

func openPhysicalDrive(index int) (windows.Handle, error) {
	path, err := windows.UTF16PtrFromString(fmt.Sprintf(`\\.\PhysicalDrive%d`, index))
	if err != nil {
		return 0, err
	}
	share := uint32(windows.FILE_SHARE_READ | windows.FILE_SHARE_WRITE)
	h, err := windows.CreateFile(
		path,
		windows.GENERIC_READ,
		share,
		nil,
		windows.OPEN_EXISTING,
		0,
		0,
	)
	if err != nil {
		return 0, err
	}
	return h, nil
}

func isAccessDenied(err error) bool {
	if err == nil {
		return false
	}
	if errno, ok := err.(syscall.Errno); ok {
		return errno == windows.ERROR_ACCESS_DENIED
	}
	return strings.Contains(strings.ToLower(err.Error()), "access is denied") ||
		strings.Contains(strings.ToLower(err.Error()), "отказано")
}

func deviceIoControl(h windows.Handle, code uint32, in, out []byte) (uint32, error) {
	var ret uint32
	var inPtr, outPtr *byte
	var inLen, outLen uint32
	if len(in) > 0 {
		inPtr = &in[0]
		inLen = uint32(len(in))
	}
	if len(out) > 0 {
		outPtr = &out[0]
		outLen = uint32(len(out))
	}
	err := windows.DeviceIoControl(h, code, inPtr, inLen, outPtr, outLen, &ret, nil)
	return ret, err
}

// readDiskSMARTIOCTL opens PhysicalDriveN and reads NVMe health log or ATA SMART.
func readDiskSMARTIOCTL(diskIndex int, preferNVMe bool) smartIOCTLResult {
	out := smartIOCTLResult{Bus: "unknown"}
	if diskIndex < 0 {
		out.UnavailableRu = "Не удалось определить физический диск."
		out.UnavailableEn = "Could not resolve the physical disk."
		return out
	}
	h, err := openPhysicalDrive(diskIndex)
	if err != nil {
		if isAccessDenied(err) {
			out.NeedsAdmin = true
			out.AdminReasonRu = "Для подробных данных диска нужны права администратора"
			out.AdminReasonEn = "Detailed disk data needs administrator rights"
		} else {
			out.UnavailableRu = "Не удалось открыть диск: " + err.Error()
			out.UnavailableEn = "Could not open disk: " + err.Error()
		}
		return out
	}
	defer windows.CloseHandle(h)

	if preferNVMe {
		if nvme, ok := readNVMeHealthLog(h); ok {
			return nvme
		}
	}
	if ata, ok := readATASMART(h); ok {
		return ata
	}
	// Try NVMe even if bus hint was SATA (some RAID/NVMe mislabeled)
	if !preferNVMe {
		if nvme, ok := readNVMeHealthLog(h); ok {
			return nvme
		}
	}
	out.UnavailableRu = "SMART недоступен без прав администратора или драйвер не отдаёт данные."
	out.UnavailableEn = "SMART unavailable without admin rights, or the driver does not expose it."
	out.NeedsAdmin = !IsProcessElevated()
	if out.NeedsAdmin {
		out.AdminReasonRu = "Для подробных данных диска нужны права администратора"
		out.AdminReasonEn = "Detailed disk data needs administrator rights"
	}
	return out
}

func readNVMeHealthLog(h windows.Handle) (smartIOCTLResult, bool) {
	const logSize = 512
	hdrSize := uint32(unsafe.Sizeof(storageProtocolSpecificData{}))
	bufSize := int(4+4+hdrSize) + logSize + 64 // PropertyId+QueryType + protocol + payload slack
	buf := make([]byte, bufSize)
	binary.LittleEndian.PutUint32(buf[0:], storageDeviceProtoSpecific)
	binary.LittleEndian.PutUint32(buf[4:], 0) // PropertyStandardQuery
	protoOff := 8
	binary.LittleEndian.PutUint32(buf[protoOff+0:], protocolTypeNvme)
	binary.LittleEndian.PutUint32(buf[protoOff+4:], nvmeDataTypeLogPage)
	binary.LittleEndian.PutUint32(buf[protoOff+8:], nvmeLogSmartHealth)
	binary.LittleEndian.PutUint32(buf[protoOff+12:], 0xFFFFFFFF) // NSID all / controller
	binary.LittleEndian.PutUint32(buf[protoOff+24:], hdrSize)    // ProtocolDataOffset relative to protocol struct
	binary.LittleEndian.PutUint32(buf[protoOff+28:], logSize)

	outBuf := make([]byte, bufSize+logSize)
	copy(outBuf, buf)
	_, err := deviceIoControl(h, ioctlStorageQueryProperty, buf, outBuf)
	if err != nil {
		// Alternate: StorageAdapterProtocolSpecificProperty = 48
		binary.LittleEndian.PutUint32(buf[0:], 48)
		copy(outBuf, buf)
		_, err = deviceIoControl(h, ioctlStorageQueryProperty, buf, outBuf)
		if err != nil {
			return smartIOCTLResult{}, false
		}
	}

	// Payload starts after PropertyId+QueryType+STORAGE_PROTOCOL_SPECIFIC_DATA
	dataOff := 8 + int(hdrSize)
	if dataOff+logSize > len(outBuf) {
		return smartIOCTLResult{}, false
	}
	log := outBuf[dataOff : dataOff+logSize]
	return parseNVMeHealthLog(log), true
}

func parseNVMeHealthLog(log []byte) smartIOCTLResult {
	r := smartIOCTLResult{OK: true, Bus: "nvme"}
	if len(log) < 192 {
		r.OK = false
		return r
	}
	r.CriticalWarning = log[0]
	kelvin := binary.LittleEndian.Uint16(log[1:3])
	if kelvin > 0 {
		r.TemperatureC = float64(kelvin) - 273.0
		r.TempOK = r.TemperatureC > -20 && r.TemperatureC < 150
	}
	r.AvailableSpare = int(log[3])
	r.AvailableSpareThresh = int(log[4])
	r.PercentageUsed = int(log[5])
	r.HealthPercent = float64(100 - r.PercentageUsed)
	if r.HealthPercent < 0 {
		r.HealthPercent = 0
	}
	if r.HealthPercent > 100 {
		r.HealthPercent = 100
	}
	r.HealthOK = true
	r.HealthSource = "nvme_percentage_used"

	r.DataUnitsRead = readNVMeU128Lo(log[32:48])
	r.DataUnitsWritten = readNVMeU128Lo(log[48:64])
	// NVMe Data Units are in thousands of 512-byte units
	r.HostReadGB = float64(r.DataUnitsRead) * 512000.0 / (1024 * 1024 * 1024)
	r.HostWriteGB = float64(r.DataUnitsWritten) * 512000.0 / (1024 * 1024 * 1024)
	r.PowerCycles = float64(readNVMeU128Lo(log[112:128]))
	r.PowerOnHours = float64(readNVMeU128Lo(log[128:144]))
	r.UnsafeShutdowns = float64(readNVMeU128Lo(log[144:160]))
	r.MediaErrors = float64(readNVMeU128Lo(log[160:176]))
	// Do not map media errors → reallocated / Bad; status is decided by EvaluateDiskHealth.
	return r
}

func readNVMeU128Lo(b []byte) uint64 {
	if len(b) < 8 {
		return 0
	}
	return binary.LittleEndian.Uint64(b[0:8])
}

func readATASMART(h windows.Handle) (smartIOCTLResult, bool) {
	data, err := ataSMARTCommand(h, smartReadData)
	if err != nil {
		return smartIOCTLResult{}, false
	}
	thresh, _ := ataSMARTCommand(h, smartReadThresholds)
	r := parseATASMART(data, thresh)
	if id, ok := readATAIdentifyWord217(h); ok {
		r.IdentifyWord217 = id
		r.IdentifyWord217OK = true
	}
	return r, r.OK
}

// ATA IDENTIFY DEVICE (0xEC) → word 217 Nominal Media Rotation Rate.
func readATAIdentifyWord217(h windows.Handle) (int, bool) {
	const dataLen = 512
	const ataIdentify = 0xEC
	size := int(unsafe.Sizeof(ataPassThroughEx{})) + dataLen
	buf := make([]byte, size)
	pt := (*ataPassThroughEx)(unsafe.Pointer(&buf[0]))
	pt.Length = uint16(unsafe.Sizeof(ataPassThroughEx{}))
	pt.AtaFlags = ataFlagsDRFRDY | ataFlagsDataIn
	pt.DataTransferLength = dataLen
	pt.TimeOutValue = ataPassThroughTimeout
	pt.DataBufferOffset = uint64(unsafe.Sizeof(ataPassThroughEx{}))
	pt.CurrentTaskFile[1] = 1 // sector count
	pt.CurrentTaskFile[6] = ataIdentify
	_, err := deviceIoControl(h, ioctlATAPassThrough, buf, buf)
	if err != nil {
		_, err = deviceIoControl(h, ioctlATAPassThroughDirect, buf, buf)
		if err != nil {
			return 0, false
		}
	}
	off := int(pt.DataBufferOffset)
	if off <= 0 || off+dataLen > len(buf) {
		off = int(unsafe.Sizeof(ataPassThroughEx{}))
	}
	// Word 217 = bytes 434-435 little-endian
	if off+436 > len(buf) {
		return 0, false
	}
	w := int(binary.LittleEndian.Uint16(buf[off+434 : off+436]))
	return w, true
}

func ataSMARTCommand(h windows.Handle, feature uint8) ([]byte, error) {
	const dataLen = 512
	size := int(unsafe.Sizeof(ataPassThroughEx{})) + dataLen
	buf := make([]byte, size)
	pt := (*ataPassThroughEx)(unsafe.Pointer(&buf[0]))
	pt.Length = uint16(unsafe.Sizeof(ataPassThroughEx{}))
	pt.AtaFlags = ataFlagsDRFRDY | ataFlagsDataIn
	pt.DataTransferLength = dataLen
	pt.TimeOutValue = ataPassThroughTimeout
	pt.DataBufferOffset = uint64(unsafe.Sizeof(ataPassThroughEx{}))
	pt.CurrentTaskFile[0] = feature // Features
	pt.CurrentTaskFile[1] = 1      // Sector count
	pt.CurrentTaskFile[3] = smartCylinderLow
	pt.CurrentTaskFile[4] = smartCylinderHigh
	pt.CurrentTaskFile[6] = smartCmd

	_, err := deviceIoControl(h, ioctlATAPassThrough, buf, buf)
	if err != nil {
		_, err = deviceIoControl(h, ioctlATAPassThroughDirect, buf, buf)
		if err != nil {
			return nil, err
		}
	}
	off := int(pt.DataBufferOffset)
	if off <= 0 || off+dataLen > len(buf) {
		off = int(unsafe.Sizeof(ataPassThroughEx{}))
	}
	out := make([]byte, dataLen)
	copy(out, buf[off:off+dataLen])
	return out, nil
}

func parseATASMART(data, thresh []byte) smartIOCTLResult {
	r := smartIOCTLResult{OK: true, Bus: "ata"}
	if len(data) < 362 {
		r.OK = false
		return r
	}
	threshMap := map[int]int{}
	if len(thresh) >= 362 {
		for i := 0; i < 30; i++ {
			off := 2 + i*12
			id := int(thresh[off])
			if id == 0 {
				continue
			}
			threshMap[id] = int(thresh[off+1])
		}
	}

	attrs := make([]SmartAttribute, 0, 30)
	var wearNorm = -1
	var tempRaw float64
	var tempOK bool
	var realloc, pending, uncorr int64

	for i := 0; i < 30; i++ {
		off := 2 + i*12
		id := int(data[off])
		if id == 0 {
			continue
		}
		flags := binary.LittleEndian.Uint16(data[off+1 : off+3])
		_ = flags
		current := int(data[off+3])
		worst := int(data[off+4])
		raw6 := data[off+5 : off+11]
		rawVal := uint64(raw6[0]) | uint64(raw6[1])<<8 | uint64(raw6[2])<<16 |
			uint64(raw6[3])<<24 | uint64(raw6[4])<<32 | uint64(raw6[5])<<40
		nameEn, nameRu := smartAttrName(id)
		th := threshMap[id]
		attrs = append(attrs, SmartAttribute{
			ID: id, Name: nameEn, NameRu: nameRu,
			Current: current, Worst: worst, Threshold: th,
			Raw: formatSMARTRaw(raw6), RawValue: rawVal,
		})
		switch id {
		case 5:
			realloc = int64(rawVal & 0xFFFF)
		case 197:
			pending = int64(rawVal & 0xFFFF)
		case 198:
			uncorr = int64(rawVal & 0xFFFF)
		case 9:
			r.PowerOnHours = float64(rawVal & 0xFFFFFFFF)
		case 12:
			r.PowerCycles = float64(rawVal & 0xFFFFFFFF)
		case 194, 190:
			// Often raw[0]=temp C
			t := float64(raw6[0])
			if t > 0 && t < 125 {
				tempRaw = t
				tempOK = true
			}
		case 241: // Total LBAs Written
			r.HostWriteGB = float64(rawVal) * 512.0 / (1024 * 1024 * 1024)
		case 242: // Total LBAs Read
			r.HostReadGB = float64(rawVal) * 512.0 / (1024 * 1024 * 1024)
		case 173, 177, 231, 232, 233, 202, 245:
			if current > 0 && current <= 100 && (wearNorm < 0 || current < wearNorm) {
				wearNorm = current
			}
		}
	}
	r.Attributes = attrs
	r.Reallocated = realloc
	r.Pending = pending
	r.Uncorrectable = uncorr
	if tempOK {
		r.TemperatureC = tempRaw
		r.TempOK = true
	}

	// Percent / status are computed later by EvaluateDiskHealth (CrystalDiskInfo rules).
	_ = wearNorm
	r.HealthOK = false
	return r
}

func mathMin(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func formatSMARTRaw(b []byte) string {
	if len(b) < 6 {
		return "0"
	}
	// CrystalDiskInfo-style: show 6 bytes hex-ish decimal concatenation commonly as number
	v := uint64(b[0]) | uint64(b[1])<<8 | uint64(b[2])<<16 | uint64(b[3])<<24 | uint64(b[4])<<32 | uint64(b[5])<<40
	return fmt.Sprintf("%d", v)
}

func smartAttrName(id int) (en, ru string) {
	m := map[int][2]string{
		1:   {"Raw Read Error Rate", "Частота ошибок чтения"},
		3:   {"Spin-Up Time", "Время раскрутки"},
		4:   {"Start/Stop Count", "Число запусков"},
		5:   {"Reallocated Sectors Count", "Переназначенные сектора"},
		7:   {"Seek Error Rate", "Ошибки позиционирования"},
		9:   {"Power-On Hours", "Время работы"},
		10:  {"Spin Retry Count", "Повторы раскрутки"},
		12:  {"Power Cycle Count", "Число включений"},
		174: {"Unexpected Power Loss", "Внезапные отключения"},
		175: {"Program Fail Count", "Сбои программирования"},
		176: {"Erase Fail Count", "Сбои стирания"},
		177: {"Wear Leveling Count", "Износ (Wear Leveling)"},
		179: {"Used Reserved Block Count", "Использованные резервные блоки"},
		181: {"Non-4K Aligned Access", "Невыровненный доступ"},
		182: {"Erase Fail Count", "Сбои стирания"},
		183: {"Runtime Bad Block", "Плохие блоки"},
		184: {"End-to-End Error", "Ошибки End-to-End"},
		187: {"Reported Uncorrectable", "Неисправимые ошибки"},
		188: {"Command Timeout", "Таймауты команд"},
		190: {"Airflow Temperature", "Температура потока"},
		194: {"Temperature", "Температура"},
		195: {"Hardware ECC Recovered", "ECC аппаратный"},
		196: {"Reallocation Event Count", "События переназначения"},
		197: {"Current Pending Sector", "Ожидающие сектора"},
		198: {"Offline Uncorrectable", "Офлайн неисправимые"},
		199: {"UDMA CRC Error Count", "CRC ошибки UDMA"},
		169: {"Remain Life", "Остаток ресурса"},
		202: {"Percentage Lifetime Remain", "Остаток ресурса"},
		231: {"SSD Life Left", "Остаток ресурса SSD"},
		232: {"Available Reserved Space", "Доступный резерв"},
		233: {"Media Wearout Indicator", "Индикатор износа"},
		241: {"Total LBAs Written", "Всего записано LBA"},
		242: {"Total LBAs Read", "Всего прочитано LBA"},
		245: {"Percentage Used Endurance", "Использованный ресурс"},
	}
	if v, ok := m[id]; ok {
		return v[0], v[1]
	}
	return fmt.Sprintf("Attribute %d", id), fmt.Sprintf("Атрибут %d", id)
}

func healthSourceLabel(src string, ru bool) string {
	switch src {
	case "nvme_percentage_used":
		if ru {
			return "NVMe: 100 − Percentage Used"
		}
		return "NVMe: 100 − Percentage Used"
	case "sata_wear":
		if ru {
			return "SATA SSD: нормализованный атрибут износа"
		}
		return "SATA SSD: normalized wear attribute"
	case "hdd_sectors":
		if ru {
			return "HDD: переназначенные / ожидающие сектора"
		}
		return "HDD: reallocated / pending sectors"
	case "storage_api":
		if ru {
			return "Windows Storage API (Wear)"
		}
		return "Windows Storage API (Wear)"
	default:
		if ru {
			return "неизвестно"
		}
		return "unknown"
	}
}

func diskStatusWord(healthPct float64, critical bool, ru bool) string {
	if critical || healthPct < 50 {
		if ru {
			return "Плохо"
		}
		return "Bad"
	}
	if healthPct < 90 {
		if ru {
			return "Внимание"
		}
		return "Caution"
	}
	if ru {
		return "Хорошо"
	}
	return "Good"
}
