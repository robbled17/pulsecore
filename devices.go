package main

import (
	"fmt"
	"strings"

	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
	"github.com/robbled17/zeropulse/loc"
)

// PnPDeviceError is one device with ConfigManagerErrorCode > 0.
type PnPDeviceError struct {
	Name              string  `json:"name"`
	DeviceID          string  `json:"deviceId"`
	ErrorCode         int     `json:"errorCode"`
	StatusDescription string  `json:"statusDescription"`
	ExplainCode       string  `json:"explainCode"` // i18n key
	Explain           loc.Msg `json:"explain"`
	ActionID          string  `json:"actionId"` // restart_driver|find_driver|device_props
	ActionLabelCode   string  `json:"actionLabelCode"`
	Action            loc.Msg `json:"action"`
}

// ListPnPDeviceErrors returns problematic PnP devices with human explanations.
func (a *App) ListPnPDeviceErrors() []PnPDeviceError {
	return listPnPDeviceErrors()
}

func listPnPDeviceErrors() []PnPDeviceError {
	raw, ok := queryPnPDeviceErrors()
	if !ok {
		return nil
	}
	out := make([]PnPDeviceError, 0, len(raw))
	for _, d := range raw {
		exCode, actID, actLabel := pnpErrorDict(d.ErrorCode)
		out = append(out, PnPDeviceError{
			Name: d.Name, DeviceID: d.DeviceID, ErrorCode: d.ErrorCode,
			StatusDescription: d.StatusDescription,
			ExplainCode: exCode, Explain: loc.M(exCode, loc.P("code", d.ErrorCode)),
			ActionID: actID, ActionLabelCode: actLabel, Action: loc.M(actLabel, nil),
		})
	}
	return out
}

type pnpRaw struct {
	Name, DeviceID, StatusDescription string
	ErrorCode                         int
}

func queryPnPDeviceErrors() ([]pnpRaw, bool) {
	_ = ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED)
	defer ole.CoUninitialize()
	unknown, err := oleutil.CreateObject("WbemScripting.SWbemLocator")
	if err != nil {
		return nil, false
	}
	defer unknown.Release()
	wmi, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return nil, false
	}
	defer wmi.Release()
	serviceRaw, err := oleutil.CallMethod(wmi, "ConnectServer", nil, `root\cimv2`)
	if err != nil {
		return nil, false
	}
	service := serviceRaw.ToIDispatch()
	defer service.Release()
	resultRaw, err := oleutil.CallMethod(service, "ExecQuery",
		"SELECT Name, DeviceID, ConfigManagerErrorCode, Status FROM Win32_PnPEntity WHERE ConfigManagerErrorCode > 0")
	if err != nil {
		return nil, false
	}
	result := resultRaw.ToIDispatch()
	defer result.Release()
	countVar, err := oleutil.GetProperty(result, "Count")
	if err != nil {
		return nil, false
	}
	n := int(countVar.Val)
	out := make([]pnpRaw, 0, n)
	for i := 0; i < n; i++ {
		itemRaw, err := oleutil.CallMethod(result, "ItemIndex", i)
		if err != nil {
			continue
		}
		item := itemRaw.ToIDispatch()
		name := oleString(item, "Name")
		id := oleString(item, "DeviceID")
		status := oleString(item, "Status")
		code := 0
		if v, err := oleutil.GetProperty(item, "ConfigManagerErrorCode"); err == nil {
			code = int(v.Val)
			v.Clear()
		}
		item.Release()
		if name == "" || code <= 0 {
			continue
		}
		out = append(out, pnpRaw{Name: name, DeviceID: id, StatusDescription: status, ErrorCode: code})
	}
	return out, true
}

func oleString(disp *ole.IDispatch, prop string) string {
	v, err := oleutil.GetProperty(disp, prop)
	if err != nil {
		return ""
	}
	defer v.Clear()
	return strings.TrimSpace(v.ToString())
}

func pnpErrorDict(code int) (explainKey, actionID, actionLabel string) {
	switch code {
	case 43:
		return "pnpExplain43", "restart_driver", "pnpActionRestart"
	case 28, 1:
		return "pnpExplain28", "find_driver", "pnpActionFindDriver"
	case 10:
		return "pnpExplain10", "device_props", "pnpActionProps"
	case 22:
		return "pnpExplain22", "device_props", "pnpActionProps"
	case 31:
		return "pnpExplain31", "find_driver", "pnpActionFindDriver"
	default:
		return "pnpExplainGeneric", "device_props", "pnpActionProps"
	}
}

// RestartPnPDevice disables+enables a device instance (best-effort, may need admin).
func (a *App) RestartPnPDevice(deviceID string) map[string]interface{} {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return map[string]interface{}{"ok": false, "message": "empty device id"}
	}
	// Escape single quotes for PowerShell.
	esc := strings.ReplaceAll(deviceID, "'", "''")
	script := fmt.Sprintf(
		`$d = Get-PnpDevice -InstanceId '%s' -ErrorAction SilentlyContinue; if (-not $d) { exit 2 }; Disable-PnpDevice -InstanceId '%s' -Confirm:$false -ErrorAction SilentlyContinue; Start-Sleep -Milliseconds 600; Enable-PnpDevice -InstanceId '%s' -Confirm:$false -ErrorAction Stop`,
		esc, esc, esc,
	)
	err := RunHiddenCommand("powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", script).Run()
	ok := err == nil
	msg := "ok"
	if err != nil {
		msg = err.Error()
		// Fallback: open Device Manager so user can restart manually.
		_ = runHiddenStart("devmgmt.msc")
	}
	return map[string]interface{}{"ok": ok, "message": msg}
}

// OpenPnPDeviceAction runs the recommended action for a device error.
func (a *App) OpenPnPDeviceAction(actionID, deviceID string) map[string]interface{} {
	actionID = strings.TrimSpace(actionID)
	switch actionID {
	case "restart_driver":
		return a.RestartPnPDevice(deviceID)
	case "find_driver":
		_ = a.openSettingsURL("ms-settings:windowsupdate")
		return map[string]interface{}{"ok": true, "message": "opened windows update"}
	case "device_props":
		_ = runHiddenStart("devmgmt.msc")
		return map[string]interface{}{"ok": true, "message": "opened device manager"}
	default:
		_ = runHiddenStart("devmgmt.msc")
		return map[string]interface{}{"ok": true, "message": "opened device manager"}
	}
}

// queryDeviceErrorCount uses detailed list when possible.
func queryDeviceErrorCount() (n int, ok bool) {
	list, ok := queryPnPDeviceErrors()
	if !ok {
		return 0, false
	}
	return len(list), true
}
