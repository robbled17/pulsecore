package main

import (
	"path/filepath"
	"strings"
)

var friendlyProcessNames = map[string]string{
	"msmpeng.exe":           "Защитник Windows (Антивирусный скан)",
	"nissrv.exe":            "Защитник Windows (Сетевая защита)",
	"securityhealthservice.exe": "Служба безопасности Windows",
	"searchindexer.exe":     "Индексатор поиска Windows",
	"searchhost.exe":        "Поиск Windows",
	"tiworker.exe":          "Центр обновления Windows",
	"trustedinstaller.exe":  "Центр обновления Windows",
	"usoclient.exe":         "Центр обновления Windows",
	"wuauclt.exe":           "Центр обновления Windows",
	"comppkgsrv.exe":        "Компоненты оформления Windows",
	"dwm.exe":               "Менеджер окон рабочего стола",
	"audiodg.exe":           "Аудиодвижок Windows",
	"runtimebroker.exe":     "Брокер разрешений приложений",
	"shellexperiencehost.exe": "Оболочка Windows (уведомления/меню)",
	"startmenuexperiencehost.exe": "Меню Пуск",
	"sihost.exe":            "Хост оболочки Windows",
	"ctfmon.exe":            "Служба ввода / язык клавиатуры",
	"fontdrvhost.exe":       "Хост шрифтов Windows",
	"spoolsv.exe":           "Диспетчер печати",
	"taskhostw.exe":         "Хост фоновых задач Windows",
	"svchost.exe":           "Узел служб Windows",
	"system":                "Ядро Windows",
	"registry":              "Реестр Windows",
	"memory compression":    "Сжатие памяти Windows",
	"chrome.exe":            "Google Chrome",
	"msedge.exe":            "Microsoft Edge",
	"firefox.exe":           "Mozilla Firefox",
	"discord.exe":           "Discord",
	"steam.exe":             "Steam",
	"code.exe":              "Visual Studio Code",
	"cursor.exe":            "Cursor",
}

// friendlyProcessName returns a human-readable explanation for common process names.
func friendlyProcessName(name string) string {
	base := strings.ToLower(filepath.Base(strings.TrimSpace(name)))
	if base == "" {
		return "Неизвестный процесс"
	}
	if label, ok := friendlyProcessNames[base]; ok {
		return label
	}
	// Strip extension for display fallback.
	return strings.TrimSuffix(name, filepath.Ext(name))
}
