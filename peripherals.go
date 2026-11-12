package main

import (
	"strings"

	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
)

// PeripheralItem is a connected external / input / audio / display device.
type PeripheralItem struct {
	Name     string `json:"name"`
	DeviceID string `json:"deviceID"`
	Service  string `json:"service"`
	Class    string `json:"class"`
	Kind     string `json:"kind"`   // keyboard|mouse|hid|audio|camera|storage|monitor
	Group    string `json:"group"`  // input|audio|storage|display
	Icon     string `json:"icon"`
	Status   string `json:"status"`
}

var junkNameTokens = []string{
	"контроллер", "хост", "том", "концентратор", "шина",
	"controller", "host", "volume", "hub", "bus", "root", "generic",
	"i2c", "acpi", "pci", "enum", "miniport", "filter", "фильтр",
	"shadow", "virtual", "виртуаль", "storahci", "nvme", "ahci",
	"usb printing", "composite", "составн", "hid-compliant vendor",
	"microsoft gs wavetable", "stereo mix", "primary sound driver",
	"default device", "mapper", "proxy", "bridge", "мост",
	"представитель служб потоков", "microsoft streaming", "stream proxy",
	"service proxy", "swenum", "kmixer", "system speaker", "high definition audio bus",
	"intel(r) smart sound", "nvidia virtual", "amd high definition audio device",
	"realtek hardware support", "служба потоков", "input configuration",
	"microsoft input configuration", "устройство ввода конфигурац",
	"hid-совместимое устройство", "hid-compliant device",
}

func isJunkPeripheralName(name string) bool {
	n := strings.ToLower(strings.TrimSpace(name))
	if n == "" {
		return true
	}
	for _, tok := range junkNameTokens {
		if strings.Contains(n, tok) {
			return true
		}
	}
	return false
}

func peripheralIcon(kind, name string) string {
	lower := strings.ToLower(name)
	if strings.Contains(lower, "тачпад") || strings.Contains(lower, "touchpad") {
		return "🖐"
	}
	switch kind {
	case "keyboard":
		return "⌨"
	case "mouse":
		return "🖱"
	case "hid":
		return "🖱"
	case "audio":
		return "🎧"
	case "camera":
		return "📷"
	case "storage":
		return "💾"
	case "monitor":
		return "🖥"
	default:
		return "🔌"
	}
}

func peripheralGroup(kind string) string {
	switch kind {
	case "keyboard", "mouse", "hid":
		return "input"
	case "audio", "camera":
		return "audio"
	case "storage":
		return "storage"
	case "monitor":
		return "display"
	default:
		return ""
	}
}

func humanizePeripheralName(name, kind, deviceID string) string {
	n := strings.TrimSpace(name)
	lower := strings.ToLower(n)
	id := strings.ToLower(deviceID)

	switch {
	case strings.Contains(lower, "сенсорная панель") || strings.Contains(lower, "touch pad") || strings.Contains(lower, "touchpad") ||
		(strings.Contains(lower, "hid") && strings.Contains(lower, "touch")):
		return "Встроенный тачпад ноутбука"
	case strings.Contains(lower, "hid-совместимая мышь") || strings.Contains(lower, "hid-compatible mouse") ||
		(strings.Contains(lower, "hid") && strings.Contains(lower, "мышь")) ||
		(kind == "mouse" && (strings.Contains(lower, "hid") || strings.Contains(lower, "compatible"))):
		return "Оптическая мышь (USB)"
	case strings.Contains(lower, "integrated camera") || strings.Contains(lower, "встроенная камера") ||
		(kind == "camera" && (strings.Contains(lower, "integrated") || strings.Contains(lower, "встроен"))):
		return "Встроенная веб-камера (HD)"
	case strings.Contains(lower, "составн") || strings.Contains(lower, "composite") ||
		strings.Contains(lower, "usb input device") || strings.Contains(lower, "usb-устройство"):
		if kind == "keyboard" || strings.Contains(id, "kbd") {
			return "Беспроводной USB-адаптер клавиатуры"
		}
		if kind == "mouse" || strings.Contains(id, "mouse") {
			return "Беспроводной USB-адаптер мыши"
		}
		return "Беспроводной USB-адаптер мыши/клавиатуры"
	}
	return n
}

func shouldDropPeripheral(name, kind string, seenKinds map[string]bool) bool {
	lower := strings.ToLower(name)
	if strings.Contains(lower, "input configuration") || strings.Contains(lower, "устройство ввода") && !strings.Contains(lower, "клавиатур") && !strings.Contains(lower, "мыш") {
		if seenKinds["keyboard"] || seenKinds["mouse"] || seenKinds["hid"] {
			return true
		}
	}
	if strings.Contains(lower, "microsoft input configuration") {
		return true
	}
	return false
}

func classifyCleanPeripheral(pnpClass, name, deviceID, service string) (kind string, ok bool) {
	cls := strings.ToLower(strings.TrimSpace(pnpClass))
	blob := strings.ToLower(name + " " + deviceID + " " + service + " " + cls)
	id := strings.ToLower(deviceID)

	switch cls {
	case "keyboard":
		return "keyboard", true
	case "mouse":
		return "mouse", true
	case "hidclass":
		if strings.Contains(blob, "keyboard") || strings.Contains(blob, "kbd") || strings.Contains(blob, "клавиатур") {
			return "keyboard", true
		}
		if strings.Contains(blob, "mouse") || strings.Contains(blob, "pointing") || strings.Contains(blob, "мыш") ||
			strings.Contains(blob, "touchpad") || strings.Contains(blob, "trackpad") || strings.Contains(blob, "сенсорная панель") {
			return "mouse", true
		}
		// Skip anonymous HID vendor devices — usually noise.
		if strings.Contains(blob, "hid-compliant") || strings.Contains(blob, "hid-совместим") || strings.Contains(blob, "vendor-defined") {
			return "", false
		}
		return "hid", true
	case "audioendpoint", "media", "sound":
		if strings.Contains(blob, "stereo mix") || strings.Contains(blob, "mapper") {
			return "", false
		}
		return "audio", true
	case "camera", "image":
		return "camera", true
	case "monitor", "desktopmonitor", "display":
		return "monitor", true
	case "diskdrive", "wpd", "cdrom":
		// Only real removable / USB external storage.
		if strings.Contains(id, "usbstor") || strings.Contains(id, "usbstor#") ||
			strings.Contains(blob, "usb") && (strings.Contains(blob, "flash") || strings.Contains(blob, "external") || strings.Contains(blob, "portable") || strings.Contains(cls, "wpd")) ||
			cls == "wpd" {
			return "storage", true
		}
		return "", false
	}

	// Name-based fallbacks for localized Windows.
	switch {
	case strings.Contains(blob, "keyboard") || strings.Contains(blob, "клавиатур"):
		return "keyboard", true
	case strings.Contains(blob, "mouse") || strings.Contains(blob, "мышь") || strings.Contains(blob, "touchpad") || strings.Contains(blob, "сенсорная панель"):
		return "mouse", true
	case strings.Contains(blob, "camera") || strings.Contains(blob, "webcam") || strings.Contains(blob, "камер"):
		return "camera", true
	case strings.Contains(blob, "headset") || strings.Contains(blob, "headphone") || strings.Contains(blob, "speaker") ||
		strings.Contains(blob, "realtek") || strings.Contains(blob, "наушник") || strings.Contains(blob, "динамик") ||
		strings.Contains(blob, "microphone") || strings.Contains(blob, "микрофон"):
		return "audio", true
	case strings.Contains(blob, "monitor") || strings.Contains(blob, "display") || strings.Contains(blob, "монитор") || strings.Contains(blob, "дисплей"):
		return "monitor", true
	case strings.Contains(id, "usbstor") || (strings.Contains(blob, "flash") && strings.Contains(blob, "usb")):
		return "storage", true
	default:
		return "", false
	}
}

// GetConnectedPeripherals returns cleaned, grouped PnP peripherals.
func (a *App) GetConnectedPeripherals() []PeripheralItem {
	return a.collectConnectedDevices()
}

// GetConnectedDevices is an alias used by the UI layer / docs.
func (a *App) GetConnectedDevices() []PeripheralItem {
	return a.collectConnectedDevices()
}

func (a *App) collectConnectedDevices() []PeripheralItem {
	ensureOLE()
	out := make([]PeripheralItem, 0, 16)

	locator, err := oleutil.CreateObject("WbemScripting.SWbemLocator")
	if err != nil {
		return out
	}
	defer locator.Release()
	dl, err := locator.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return out
	}
	defer dl.Release()

	serviceRaw, err := oleutil.CallMethod(dl, "ConnectServer", nil, `root\cimv2`)
	if err != nil {
		return out
	}
	service := serviceRaw.ToIDispatch()
	defer service.Release()

	resultRaw, err := oleutil.CallMethod(service, "ExecQuery",
		"SELECT Name, DeviceID, Service, PNPClass, Status FROM Win32_PnPEntity WHERE Status = 'OK'")
	if err != nil {
		return out
	}
	result := resultRaw.ToIDispatch()
	defer result.Release()

	countVar, _ := oleutil.GetProperty(result, "Count")
	count := int(countVar.Val)
	seen := make(map[string]struct{})
	seenKinds := make(map[string]bool)

	for i := 0; i < count; i++ {
		itemRaw, err := oleutil.CallMethod(result, "ItemIndex", i)
		if err != nil {
			continue
		}
		item := itemRaw.ToIDispatch()
		nameVar, _ := oleutil.GetProperty(item, "Name")
		idVar, _ := oleutil.GetProperty(item, "DeviceID")
		svcVar, _ := oleutil.GetProperty(item, "Service")
		clsVar, _ := oleutil.GetProperty(item, "PNPClass")
		name := strings.TrimSpace(nameVar.ToString())
		deviceID := strings.TrimSpace(idVar.ToString())
		serviceName := strings.TrimSpace(svcVar.ToString())
		pnpClass := strings.TrimSpace(clsVar.ToString())
		item.Release()

		if isJunkPeripheralName(name) {
			continue
		}
		kind, ok := classifyCleanPeripheral(pnpClass, name, deviceID, serviceName)
		if !ok {
			continue
		}
		if shouldDropPeripheral(name, kind, seenKinds) {
			continue
		}
		group := peripheralGroup(kind)
		if group == "" {
			continue
		}
		display := humanizePeripheralName(name, kind, deviceID)
		if shouldDropPeripheral(display, kind, seenKinds) {
			continue
		}
		key := strings.ToLower(display + "|" + group)
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		seenKinds[kind] = true
		out = append(out, PeripheralItem{
			Name:     display,
			DeviceID: deviceID,
			Service:  serviceName,
			Class:    pnpClass,
			Kind:     kind,
			Group:    group,
			Icon:     peripheralIcon(kind, display),
			Status:   "connected",
		})
		if len(out) >= 28 {
			break
		}
	}

	order := map[string]int{"input": 0, "audio": 1, "storage": 2, "display": 3}
	for i := 1; i < len(out); i++ {
		j := i
		for j > 0 && order[out[j].Group] < order[out[j-1].Group] {
			out[j], out[j-1] = out[j-1], out[j]
			j--
		}
	}
	return out
}
