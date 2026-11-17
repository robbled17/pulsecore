package main

import (
	"fmt"
	"strings"
	"unsafe"

	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
)

type systemPowerStatus struct {
	ACLineStatus        byte
	BatteryFlag         byte
	BatteryLifePercent  byte
	SystemStatusFlag    byte
	BatteryLifeTime     uint32
	BatteryFullLifeTime uint32
}

var procGetSystemPowerStatus = modKernel32.NewProc("GetSystemPowerStatus")

func readSystemPowerStatus() (systemPowerStatus, bool) {
	var st systemPowerStatus
	r1, _, _ := procGetSystemPowerStatus.Call(uintptr(unsafe.Pointer(&st)))
	return st, r1 != 0
}

func wearLabel(wear float64) string {
	switch {
	case wear < 8:
		return "Отличное состояние"
	case wear < 15:
		return "Хорошее состояние"
	case wear < 30:
		return "Нормальное состояние"
	case wear < 45:
		return "Заметный износ"
	default:
		return "Сильный износ"
	}
}

func desktopPower() BatteryMetrics {
	return BatteryMetrics{
		Status:     "Desktop",
		StatusText: "Настольный ПК",
		WearLabel:  "Питание: Стандартный ATX блок питания (От сети 220V)",
		Available:  false,
		PluggedIn:  true,
		Percent:    100,
		DeviceName: "Нет аккумулятора",
	}
}

// win32BatteryCount queries Win32_Battery. ok is false when WMI itself cannot be asked.
func win32BatteryCount() (int, bool) {
	ensureOLE()
	locator, err := oleutil.CreateObject("WbemScripting.SWbemLocator")
	if err != nil {
		return 0, false
	}
	defer locator.Release()
	dl, err := locator.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return 0, false
	}
	defer dl.Release()
	cimRaw, err := oleutil.CallMethod(dl, "ConnectServer", nil, `root\cimv2`)
	if err != nil {
		return 0, false
	}
	cim := cimRaw.ToIDispatch()
	defer cim.Release()
	qRaw, err := oleutil.CallMethod(cim, "ExecQuery", "SELECT DeviceID FROM Win32_Battery")
	if err != nil {
		return 0, false
	}
	q := qRaw.ToIDispatch()
	defer q.Release()
	c, err := oleutil.GetProperty(q, "Count")
	if err != nil {
		return 0, false
	}
	return int(c.Val), true
}

// collectBattery uses GetSystemPowerStatus + Win32_Battery.
// No Win32_Battery instance means a desktop PC, even if the power API reports a phantom percent.
func collectBattery() BatteryMetrics {
	if n, ok := win32BatteryCount(); ok && n == 0 {
		return desktopPower()
	}

	out := desktopPower()

	st, ok := readSystemPowerStatus()
	if !ok {
		return enrichBatteryWMI(out)
	}

	// 255 = unknown. BatteryFlag bit 0x80 = no battery installed.
	if st.BatteryLifePercent == 255 || st.BatteryFlag&0x80 != 0 {
		return desktopPower()
	}

	out.Available = true
	out.Percent = float64(st.BatteryLifePercent)
	if out.Percent > 100 {
		out.Percent = 100
	}
	out.PluggedIn = st.ACLineStatus == 1
	if out.PluggedIn {
		if st.BatteryFlag&8 != 0 || st.ACLineStatus == 1 {
			out.Status = "Charging"
			out.StatusText = "Зарядка от сети"
		} else {
			out.Status = "AC"
			out.StatusText = "Зарядка от сети"
		}
		// When full and on AC, still "от сети".
		if st.BatteryLifePercent >= 99 && st.BatteryFlag&8 == 0 {
			out.StatusText = "Зарядка от сети"
		}
	} else {
		out.Status = "Battery"
		out.StatusText = "Работа от батареи"
	}
	if st.BatteryLifeTime != 0xFFFFFFFF && st.BatteryLifeTime > 0 {
		// Rough power draw proxy from remaining lifetime is unreliable; keep mild estimate.
		out.PowerDrawWatts = 8 + (100-out.Percent)*0.05
	}

	return enrichBatteryWMI(out)
}

func enrichBatteryWMI(base BatteryMetrics) BatteryMetrics {
	if !base.Available {
		return base
	}
	ensureOLE()

	locator, err := oleutil.CreateObject("WbemScripting.SWbemLocator")
	if err != nil {
		return base
	}
	defer locator.Release()
	dl, err := locator.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return base
	}
	defer dl.Release()

	// root\WMI: BatteryStaticData + BatteryFullChargedCapacity
	wmiRaw, err := oleutil.CallMethod(dl, "ConnectServer", nil, `root\WMI`)
	if err == nil {
		wmi := wmiRaw.ToIDispatch()
		defer wmi.Release()

		staticRaw, err := oleutil.CallMethod(wmi, "ExecQuery",
			"SELECT DeviceName, DesignedCapacity FROM BatteryStaticData")
		if err == nil {
			static := staticRaw.ToIDispatch()
			defer static.Release()
			if c, _ := oleutil.GetProperty(static, "Count"); int(c.Val) > 0 {
				itemRaw, err := oleutil.CallMethod(static, "ItemIndex", 0)
				if err == nil {
					item := itemRaw.ToIDispatch()
					nameVar, _ := oleutil.GetProperty(item, "DeviceName")
					designVar, _ := oleutil.GetProperty(item, "DesignedCapacity")
					item.Release()
					base.DeviceName = strings.TrimSpace(nameVar.ToString())
					base.DesignCapacitymWh = float64(designVar.Val)
				}
			}
		}

		fullRaw, err := oleutil.CallMethod(wmi, "ExecQuery",
			"SELECT FullChargedCapacity FROM BatteryFullChargedCapacity")
		if err == nil {
			full := fullRaw.ToIDispatch()
			defer full.Release()
			if c, _ := oleutil.GetProperty(full, "Count"); int(c.Val) > 0 {
				itemRaw, err := oleutil.CallMethod(full, "ItemIndex", 0)
				if err == nil {
					item := itemRaw.ToIDispatch()
					capVar, _ := oleutil.GetProperty(item, "FullChargedCapacity")
					item.Release()
					base.CapacitymWh = float64(capVar.Val)
				}
			}
		}
	}

	// Fallback to Win32_Battery names/capacities when root\WMI is empty.
	if base.DeviceName == "" || base.DesignCapacitymWh <= 0 || base.CapacitymWh <= 0 {
		cimRaw, err := oleutil.CallMethod(dl, "ConnectServer", nil, `root\cimv2`)
		if err == nil {
			cim := cimRaw.ToIDispatch()
			defer cim.Release()
			qRaw, err := oleutil.CallMethod(cim, "ExecQuery",
				"SELECT Name, DeviceID, FullChargedCapacity, DesignCapacity, EstimatedChargeRemaining FROM Win32_Battery")
			if err == nil {
				q := qRaw.ToIDispatch()
				defer q.Release()
				if c, _ := oleutil.GetProperty(q, "Count"); int(c.Val) > 0 {
					itemRaw, err := oleutil.CallMethod(q, "ItemIndex", 0)
					if err == nil {
						item := itemRaw.ToIDispatch()
						nameVar, _ := oleutil.GetProperty(item, "Name")
						idVar, _ := oleutil.GetProperty(item, "DeviceID")
						fullVar, _ := oleutil.GetProperty(item, "FullChargedCapacity")
						designVar, _ := oleutil.GetProperty(item, "DesignCapacity")
						pctVar, _ := oleutil.GetProperty(item, "EstimatedChargeRemaining")
						item.Release()
						if base.DeviceName == "" {
							base.DeviceName = strings.TrimSpace(nameVar.ToString())
							if base.DeviceName == "" {
								base.DeviceName = strings.TrimSpace(idVar.ToString())
							}
						}
						if base.CapacitymWh <= 0 {
							base.CapacitymWh = float64(fullVar.Val)
						}
						if base.DesignCapacitymWh <= 0 {
							base.DesignCapacitymWh = float64(designVar.Val)
						}
						if pct := float64(pctVar.Val); pct > 0 && pct <= 100 {
							base.Percent = pct
						}
					}
				}
			}
		}
	}

	if base.DesignCapacitymWh > 0 && base.CapacitymWh > 0 {
		health := (base.CapacitymWh / base.DesignCapacitymWh) * 100.0
		if health > 100 {
			health = 100
		}
		base.WearPercent = 100.0 - health
		if base.WearPercent < 0 {
			base.WearPercent = 0
		}
	}
	base.WearLabel = fmt.Sprintf("%.0f%% (%s)", base.WearPercent, wearLabel(base.WearPercent))
	if base.DeviceName == "" {
		base.DeviceName = "Аккумулятор ноутбука"
	}
	return base
}
