//go:build windows

// Package win is the minimal set of Win32 bindings YALauncher needs: a
// borderless tool window, keyboard/mouse hooks, hotkeys, DPI and monitor
// geometry, GDI drawing, shell icons, a tray icon, registry and shell helpers.
//
// Everything is loaded lazily from system DLLs through syscall.NewLazyDLL: no
// CGO, no third party packages, a small exe.
package win

import "syscall"

var (
	dllUser32   = syscall.NewLazyDLL("user32.dll")
	dllKernel32 = syscall.NewLazyDLL("kernel32.dll")
	dllShell32  = syscall.NewLazyDLL("shell32.dll")
	dllAdvapi32 = syscall.NewLazyDLL("advapi32.dll")
	dllGdi32    = syscall.NewLazyDLL("gdi32.dll")
	dllDwmapi   = syscall.NewLazyDLL("dwmapi.dll")
	dllShcore   = syscall.NewLazyDLL("shcore.dll")
	dllOle32    = syscall.NewLazyDLL("ole32.dll")
	dllComctl32 = syscall.NewLazyDLL("comctl32.dll")
	dllMsimg32  = syscall.NewLazyDLL("msimg32.dll")
)

// user32
var (
	pRegisterClassExW  = dllUser32.NewProc("RegisterClassExW")
	pCreateWindowExW   = dllUser32.NewProc("CreateWindowExW")
	pDefWindowProcW    = dllUser32.NewProc("DefWindowProcW")
	pDestroyWindow     = dllUser32.NewProc("DestroyWindow")
	pIsWindow          = dllUser32.NewProc("IsWindow")
	pGetMessageW       = dllUser32.NewProc("GetMessageW")
	pPeekMessageW      = dllUser32.NewProc("PeekMessageW")
	pTranslateMessage  = dllUser32.NewProc("TranslateMessage")
	pDispatchMessageW  = dllUser32.NewProc("DispatchMessageW")
	pPostQuitMessage   = dllUser32.NewProc("PostQuitMessage")
	pPostMessageW      = dllUser32.NewProc("PostMessageW")
	pSendMessageW      = dllUser32.NewProc("SendMessageW")
	pFindWindowW       = dllUser32.NewProc("FindWindowW")
	pRegisterWindowMsg = dllUser32.NewProc("RegisterWindowMessageW")
	pSetTimer          = dllUser32.NewProc("SetTimer")
	pKillTimer         = dllUser32.NewProc("KillTimer")
	pMessageBoxW       = dllUser32.NewProc("MessageBoxW")
	pGetCursorPos      = dllUser32.NewProc("GetCursorPos")
	pSetForegroundW    = dllUser32.NewProc("SetForegroundWindow")
	pGetForeground     = dllUser32.NewProc("GetForegroundWindow")
	pSetActiveWindow   = dllUser32.NewProc("SetActiveWindow")
	pSetFocus          = dllUser32.NewProc("SetFocus")
	pGetFocus          = dllUser32.NewProc("GetFocus")
	pGetSystemMetrics  = dllUser32.NewProc("GetSystemMetrics")
	pLoadCursorW       = dllUser32.NewProc("LoadCursorW")
	pDestroyIcon       = dllUser32.NewProc("DestroyIcon")
	pCreateIconFromRes = dllUser32.NewProc("CreateIconFromResourceEx")
	pCreatePopupMenu   = dllUser32.NewProc("CreatePopupMenu")
	pAppendMenuW       = dllUser32.NewProc("AppendMenuW")
	pSetMenuDefault    = dllUser32.NewProc("SetMenuDefaultItem")
	pTrackPopupMenu    = dllUser32.NewProc("TrackPopupMenu")
	pDestroyMenu       = dllUser32.NewProc("DestroyMenu")
	pShowWindow        = dllUser32.NewProc("ShowWindow")
	pSetWindowPos      = dllUser32.NewProc("SetWindowPos")
	pUpdateWindow      = dllUser32.NewProc("UpdateWindow")

	// keyboard: global low level hook + injected input
	pSetWindowsHookExW   = dllUser32.NewProc("SetWindowsHookExW")
	pCallNextHookEx      = dllUser32.NewProc("CallNextHookEx")
	pUnhookWindowsHookEx = dllUser32.NewProc("UnhookWindowsHookEx")
	pSendInput           = dllUser32.NewProc("SendInput")
	pGetAsyncKeyState    = dllUser32.NewProc("GetAsyncKeyState")
	pGetKeyState         = dllUser32.NewProc("GetKeyState")
	pGetKeyboardLayout   = dllUser32.NewProc("GetKeyboardLayout")
	pGetKeyboardLayouts  = dllUser32.NewProc("GetKeyboardLayoutList")

	// monitors and DPI
	pEnumDisplayMonitors  = dllUser32.NewProc("EnumDisplayMonitors")
	pGetMonitorInfoW      = dllUser32.NewProc("GetMonitorInfoW")
	pMonitorFromPoint     = dllUser32.NewProc("MonitorFromPoint")
	pMonitorFromWindow    = dllUser32.NewProc("MonitorFromWindow")
	pGetDpiForWindow      = dllUser32.NewProc("GetDpiForWindow")
	pGetDpiForMonitor     = dllShcore.NewProc("GetDpiForMonitor")
	pGetSystemDpiForProc  = dllUser32.NewProc("GetDpiForSystem")
	pSetProcessDPIAware   = dllUser32.NewProc("SetProcessDPIAware")
	pSetProcessDpiContext = dllUser32.NewProc("SetProcessDpiAwarenessContext")
	pSetCursorPos         = dllUser32.NewProc("SetCursorPos")
	pTrackMouseEvent      = dllUser32.NewProc("TrackMouseEvent")
	pSetCapture           = dllUser32.NewProc("SetCapture")
	pReleaseCapture       = dllUser32.NewProc("ReleaseCapture")
	pClipCursor           = dllUser32.NewProc("ClipCursor")
	pBringWindowToTop     = dllUser32.NewProc("BringWindowToTop")
	pAllowSetForeground   = dllUser32.NewProc("AllowSetForegroundWindow")
	pAttachThreadInput    = dllUser32.NewProc("AttachThreadInput")
	pGetWindowThreadPID   = dllUser32.NewProc("GetWindowThreadProcessId")
	pGetCurrentThreadID   = dllKernel32.NewProc("GetCurrentThreadId")
	pFindFirstChange      = dllKernel32.NewProc("FindFirstChangeNotificationW")
	pFindNextChange       = dllKernel32.NewProc("FindNextChangeNotification")
	pFindCloseChange      = dllKernel32.NewProc("FindCloseChangeNotification")
	pWaitForMultiple      = dllKernel32.NewProc("WaitForMultipleObjects")
)

// gdi32
var (
	pGetDC              = dllUser32.NewProc("GetDC")
	pReleaseDC          = dllUser32.NewProc("ReleaseDC")
	pBeginPaint         = dllUser32.NewProc("BeginPaint")
	pEndPaint           = dllUser32.NewProc("EndPaint")
	pInvalidateRect     = dllUser32.NewProc("InvalidateRect")
	pGetClientRect      = dllUser32.NewProc("GetClientRect")
	pGetWindowRect      = dllUser32.NewProc("GetWindowRect")
	pGetWindowLongPtr   = dllUser32.NewProc("GetWindowLongPtrW")
	pCreateSolidBr      = dllGdi32.NewProc("CreateSolidBrush")
	pCreatePen          = dllGdi32.NewProc("CreatePen")
	pDeleteObject       = dllGdi32.NewProc("DeleteObject")
	pSelectObject       = dllGdi32.NewProc("SelectObject")
	pRoundRect          = dllGdi32.NewProc("RoundRect")
	pRectangle          = dllGdi32.NewProc("Rectangle")
	pMoveToEx           = dllGdi32.NewProc("MoveToEx")
	pLineTo             = dllGdi32.NewProc("LineTo")
	pFillRect           = dllUser32.NewProc("FillRect")
	pFillSolidRect      = dllUser32.NewProc("FillRect")
	pSetBkMode          = dllGdi32.NewProc("SetBkMode")
	pSetTextColor       = dllGdi32.NewProc("SetTextColor")
	pCreateFontW        = dllGdi32.NewProc("CreateFontW")
	pTextOutW           = dllGdi32.NewProc("TextOutW")
	pDrawTextW          = dllUser32.NewProc("DrawTextW")
	pGetTextExtentP     = dllGdi32.NewProc("GetTextExtentPoint32W")
	pGetTextMetricsW    = dllGdi32.NewProc("GetTextMetricsW")
	pGetObjectW         = dllGdi32.NewProc("GetObjectW")
	pCreateCompatibleDC = dllGdi32.NewProc("CreateCompatibleDC")
	pCreateDIBSection   = dllGdi32.NewProc("CreateDIBSection")
	pDeleteDC           = dllGdi32.NewProc("DeleteDC")
	pBitBlt             = dllGdi32.NewProc("BitBlt")
	pAlphaBlend         = dllMsimg32.NewProc("AlphaBlend")
	pGdiSetBatchLimit   = dllGdi32.NewProc("GdiSetBatchLimit")
	pSetStretchBltMode  = dllGdi32.NewProc("SetStretchBltMode")
	pStretchBlt         = dllGdi32.NewProc("StretchBlt")
)

// shell32
var (
	pShellNotifyIconW = dllShell32.NewProc("Shell_NotifyIconW")
	pShellExecuteW    = dllShell32.NewProc("ShellExecuteW")
	pSHGetFileInfoW   = dllShell32.NewProc("SHGetFileInfoW")
	pExtractIconExW   = dllShell32.NewProc("ExtractIconExW")
	pDragAcceptFiles  = dllShell32.NewProc("DragAcceptFiles")
	pDrawIconEx       = dllUser32.NewProc("DrawIconEx")
	pLoadImageW       = dllUser32.NewProc("LoadImageW")
	pIsWindowVisible  = dllUser32.NewProc("IsWindowVisible")
	pDragQueryFileW   = dllShell32.NewProc("DragQueryFileW")
	pDragFinish       = dllShell32.NewProc("DragFinish")
	pSHGetFolderPathW = dllShell32.NewProc("SHGetFolderPathW")
)

// advapi32
var (
	pOpenProcessToken    = dllAdvapi32.NewProc("OpenProcessToken")
	pGetTokenInformation = dllAdvapi32.NewProc("GetTokenInformation")
	pRegCreateKeyExW     = dllAdvapi32.NewProc("RegCreateKeyExW")
	pRegSetValueExW      = dllAdvapi32.NewProc("RegSetValueExW")
	pRegGetValueW        = dllAdvapi32.NewProc("RegGetValueW")
	pRegDeleteValueW     = dllAdvapi32.NewProc("RegDeleteValueW")
	pRegOpenKeyExW       = dllAdvapi32.NewProc("RegOpenKeyExW")
	pRegEnumKeyExW       = dllAdvapi32.NewProc("RegEnumKeyExW")
	pRegCloseKey         = dllAdvapi32.NewProc("RegCloseKey")
)

// kernel32
var (
	pGetModuleHandleW       = dllKernel32.NewProc("GetModuleHandleW")
	pGetCurrentProcess      = dllKernel32.NewProc("GetCurrentProcess")
	pCreateMutexW           = dllKernel32.NewProc("CreateMutexW")
	pCloseHandle            = dllKernel32.NewProc("CloseHandle")
	pQueryFullProcessImageW = dllKernel32.NewProc("QueryFullProcessImageNameW")
	pSleep                  = dllKernel32.NewProc("Sleep")
	pGetModuleFileNameW     = dllKernel32.NewProc("GetModuleFileNameW")
	pGetTickCount           = dllKernel32.NewProc("GetTickCount64")
	pGetLogicalDrives       = dllKernel32.NewProc("GetLogicalDrives")
	pGetVolumeInfo          = dllKernel32.NewProc("GetVolumeInformationW")
	pLocalFree              = dllKernel32.NewProc("LocalFree")
)

// dwmapi
var (
	pDwmSetWindowAttr = dllDwmapi.NewProc("DwmSetWindowAttribute")
)

func syscallUTF16Ptr(s string) (*uint16, error) {
	return syscall.UTF16PtrFromString(s)
}

func pDwmSetWindowAttribute(attr int, hwnd HWND, value uintptr) {
	pDwmSetWindowAttr.Call(uintptr(hwnd), uintptr(attr), value, 4)
}

// com
var (
	pCoInitializeEx   = dllOle32.NewProc("CoInitializeEx")
	pCoUninitialize   = dllOle32.NewProc("CoUninitialize")
	pCLSIDFromString  = dllOle32.NewProc("CLSIDFromString")
	pCoCreateInstance = dllOle32.NewProc("CoCreateInstance")
)
