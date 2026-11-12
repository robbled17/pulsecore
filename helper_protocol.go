package main

import (
	"time"

	"github.com/robbled17/zeropulse/loc"
)

// Helper protocol v1 - read-only JSON lines over a named pipe.
const helperProtocolVersion = 1

const helperPipeName = "PulseCoreHelper"
const helperTaskName = "PulseCoreHelper"
const helperStateDirName = "PulseCore"

// HelperRequest is sent by the unelevated UI process.
type HelperRequest struct {
	V  int    `json:"v"`
	Op string `json:"op"` // ping | get
}

// HelperResponse is returned by the elevated helper.
type HelperResponse struct {
	V      int            `json:"v"`
	OK     bool           `json:"ok"`
	Error  string         `json:"error,omitempty"`
	Pong   bool           `json:"pong,omitempty"`
	Driver HelperDriver   `json:"driver"`
	CPU    HelperCPU      `json:"cpu"`
	Disks  []HelperDisk   `json:"disks"`
	TS     string         `json:"ts"`
}

// HelperDriver describes PawnIO / backend health.
type HelperDriver struct {
	PawnIORunning  bool   `json:"pawnioRunning"`
	PawnIOPresent  bool   `json:"pawnioPresent"`
	Blocked        bool   `json:"blocked"`
	BlockedReason  string `json:"blockedReason,omitempty"`
	Backend        string `json:"backend"` // PawnIO | none
}

// HelperCPU carries package/CCD temps, load, clocks, power.
type HelperCPU struct {
	Name        string    `json:"name"`
	TctlC       *float64  `json:"tctlC"`
	TdieC       *float64  `json:"tdieC"`
	CcdTempsC   []float64 `json:"ccdTempsC"`
	LoadPercent *float64  `json:"loadPercent"`
	ClockMHz    *float64  `json:"clockMHz"`
	PowerW      *float64  `json:"powerW"`
	OK          bool      `json:"ok"`
}

// HelperDisk is a compact SMART/health snapshot from the helper.
type HelperDisk struct {
	Letter           string             `json:"letter"`
	Model            string             `json:"model"`
	Serial           string             `json:"serial"`
	Interface        string             `json:"interface"`
	MediaKind        string             `json:"mediaKind"` // ssd|hdd|nvme|unknown
	IsSSD            *bool              `json:"isSsd"`
	IsNVMe           bool               `json:"isNvme"`
	RotationRPM      *int               `json:"rotationRpm"`
	IdentifyWord217  *int               `json:"identifyWord217"`
	TempC            *float64           `json:"tempC"`
	TempOK           bool               `json:"tempOk"`
	HealthPercent    *float64           `json:"healthPercent"`
	HealthOK         bool               `json:"healthOk"`
	PowerOnHours     *float64           `json:"powerOnHours"`
	TotalWritesBytes *float64           `json:"totalWritesBytes"`
	Status           string             `json:"status"` // good|caution|bad|unknown
	Attrs            []HelperSMARTAttr  `json:"attrs"`
}

// HelperSMARTAttr is one SMART attribute row.
type HelperSMARTAttr struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Current   int    `json:"current"`
	Worst     int    `json:"worst"`
	Threshold int    `json:"threshold"`
	Raw       string `json:"raw"`
}

// AdvancedModeState is exposed to the frontend.
type AdvancedModeState struct {
	Enabled              bool   `json:"enabled"`
	HelperRunning        bool   `json:"helperRunning"`
	Status               string `json:"status"` // off|starting|ok|error|driver_blocked|needs_pawnio
	Phase                string `json:"phase"`  // off|starting|ok|error
	StatusRu             string `json:"statusRu"`
	StatusEn             string `json:"statusEn"`
	DriverBlocked        bool   `json:"driverBlocked"`
	DriverBlockedReason  string `json:"driverBlockedReason"`
	PawnIOPresent        bool   `json:"pawnioPresent"`
	PawnIOOwnedByUs      bool   `json:"pawnioOwnedByUs"`
	ExplainRu            string `json:"explainRu"`
	ExplainEn            string `json:"explainEn"`
	LastError            string `json:"lastError,omitempty"`
	ErrorCode            string `json:"errorCode,omitempty"`
	ActionRu             string `json:"actionRu,omitempty"`
	ActionEn             string `json:"actionEn,omitempty"`
	TaskRunning          bool   `json:"taskRunning"`
	ProcessRunning       bool   `json:"processRunning"`
	ProcessPID           int    `json:"processPid"`
	PipePresent          bool   `json:"pipePresent"`
	ProtocolVersion      int    `json:"protocolVersion"`
	HelperVersion        string `json:"helperVersion"`
	LastHelperReplyAt    string `json:"lastHelperReplyAt,omitempty"`
	UpdatedAt            string `json:"updatedAt"`
}

// AdvancedModeActionResult is returned by enable/disable.
type AdvancedModeActionResult struct {
	OK                bool              `json:"ok"`
	Message           loc.Msg           `json:"message"`
	MessageRu         string            `json:"messageRu"`
	MessageEn         string            `json:"messageEn"`
	State             AdvancedModeState `json:"state"`
	NeedConsentPawnIO bool              `json:"needConsentPawnIO,omitempty"`
	ErrorCode         string            `json:"errorCode,omitempty"`
	Phase             string            `json:"phase,omitempty"` // starting|waiting_pipe|done
}

type helperStateFile struct {
	UserSID              string    `json:"userSid"`
	InstalledByPulseCore bool      `json:"installedByPulseCore"`
	PawnIOInstalledByUs  bool      `json:"pawnioInstalledByUs"`
	PawnIOWasPresent     bool      `json:"pawnioWasPresent"`
	TaskName             string    `json:"taskName"`
	HelperPath           string    `json:"helperPath"`
	Enabled              bool      `json:"enabled"`
	HelperVersion        string    `json:"helperVersion"`
	ProtocolVersion      int       `json:"protocolVersion"`
	HelperSHA256         string    `json:"helperSha256"`
	InstalledAt          time.Time `json:"installedAt"`
}

func ptrFloat(v float64) *float64 { return &v }

func helperCPUPackageTemp(c HelperCPU) (float64, bool) {
	if c.TctlC != nil && *c.TctlC > 0 {
		return *c.TctlC, true
	}
	if c.TdieC != nil && *c.TdieC > 0 {
		return *c.TdieC, true
	}
	for _, t := range c.CcdTempsC {
		if t > 0 {
			return t, true
		}
	}
	return 0, false
}
