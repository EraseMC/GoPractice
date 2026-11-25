package device

import (
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

func OSName(os protocol.DeviceOS, model string) string {
	if os == protocol.DeviceAndroid && model == "" {
		return "Linux"
	}
	switch os {
	case protocol.DeviceAndroid:
		return "Android"
	case protocol.DeviceIOS:
		return "iOS"
	case protocol.DeviceOSX:
		return "macOS"
	case protocol.DeviceFireOS:
		return "Fire OS"
	case protocol.DeviceGearVR:
		return "Gear VR"
	case protocol.DeviceHololens:
		return "Hololens"
	case protocol.DeviceWin10:
		return "Windows"
	case protocol.DeviceWin32:
		return "Windows 32"
	case protocol.DeviceDedicated:
		return "Dedicated"
	case protocol.DeviceTVOS:
		return "tvOS"
	case protocol.DeviceOrbis:
		return "PlayStation"
	case protocol.DeviceNX:
		return "Nintendo Switch"
	case protocol.DeviceXBOX:
		return "Xbox"
	case protocol.DeviceWP:
		return "Windows Phone"
	}
	return "Unknown"
}

func Icon(osName string) string {
	switch osName {
	case "Linux":
		return ""
	case "Android":
		return ""
	case "iOS", "tvOS":
		return ""
	case "macOS":
		return ""
	case "Fire OS":
		return ""
	case "Gear VR", "Hololens":
		return ""
	case "Windows", "Windows 32":
		return ""
	case "PlayStation":
		return ""
	case "Nintendo Switch":
		return ""
	case "Xbox":
		return ""
	}
	return ""
}

func InputModeName(mode uint32) string {
	switch mode {
	case packet.InputModeMouse:
		return "Keyboard & Mouse"
	case packet.InputModeTouch:
		return "Touch"
	case packet.InputModeGamePad:
		return "Gamepad"
	case packet.InputModeMotionController:
		return "Motion"
	}
	return "Unknown"
}
