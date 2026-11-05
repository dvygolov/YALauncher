//go:build windows

package win

// Constants gathered in one place. The names mirror the SDK headers so that a
// call site can be checked against Microsoft's documentation one to one.

// Window styles
const (
	WS_OVERLAPPED    = 0x00000000
	WS_POPUP         = 0x80000000
	WS_CHILD         = 0x40000000
	WS_VISIBLE       = 0x10000000
	WS_CLIPCHILDREN  = 0x02000000
	WS_CLIPSIBLINGS  = 0x04000000
	WS_BORDER        = 0x00800000
	WS_DLGFRAME      = 0x00400000
	WS_EX_TOPMOST    = 0x00000008
	WS_EX_TOOLWINDOW = 0x00000080
	WS_EX_NOACTIVATE = 0x08000000
	WS_EX_LAYERED    = 0x00080000
	WS_EX_APPWINDOW  = 0x00040000
)

// Window messages
const (
	WM_NULL                   = 0x0000
	WM_CREATE                 = 0x0001
	WM_DESTROY                = 0x0002
	WM_MOVE                   = 0x0003
	WM_SIZE                   = 0x0005
	WM_ACTIVATE               = 0x0006
	WM_SETFOCUS               = 0x0007
	WM_KILLFOCUS              = 0x0008
	WM_CLOSE                  = 0x0010
	WM_QUIT                   = 0x0012
	WM_ERASEBKGND             = 0x0014
	WM_SHOWWINDOW             = 0x0018
	WM_ACTIVATEAPP            = 0x001C
	WM_SETCURSOR              = 0x0020
	WM_GETMINMAXINFO          = 0x0024
	WM_PAINT                  = 0x000F
	WM_PRINTCLIENT            = 0x0318
	WM_WINDOWPOSCHANGING      = 0x0046
	WM_KEYDOWN                = 0x0100
	WM_KEYUP                  = 0x0101
	WM_CHAR                   = 0x0102
	WM_SYSKEYDOWN             = 0x0104
	WM_SYSKEYUP               = 0x0105
	WM_COMMAND                = 0x0111
	WM_TIMER                  = 0x0113
	WM_INITMENUPOPUP          = 0x0117
	WM_MOUSEMOVE              = 0x0200
	WM_LBUTTONDOWN            = 0x0201
	WM_LBUTTONUP              = 0x0202
	WM_LBUTTONDBLCLK          = 0x0203
	WM_RBUTTONDOWN            = 0x0204
	WM_RBUTTONUP              = 0x0205
	WM_MOUSEWHEEL             = 0x020A
	WM_MOUSEHWHEEL            = 0x020E
	WM_MOUSELEAVE             = 0x02A3
	WM_GETDLGCODE             = 0x0087
	DLGC_WANTALLKEYS          = 0x0004
	WM_CONTEXTMENU            = 0x007B
	WM_IME_STARTCOMPOSITION   = 0x010D
	WM_IME_COMPOSITION        = 0x010F
	WM_APP                    = 0x8000
	WM_USER                   = 0x0400
	WM_KEYUP_UNUSED           = 0
	WM_INPUTLANGCHANGEREQUEST = 0x0050
	WM_NCHITTEST              = 0x0084
	WM_DPICHANGED             = 0x02E0
	WM_DROPFILES              = 0x0233
	WM_HOTKEY                 = 0x0312
)

// Mouse key states
const (
	MK_LBUTTON = 0x0001
	MK_RBUTTON = 0x0002
)

// Show window commands
const (
	SW_HIDE           = 0
	SW_SHOWNORMAL     = 1
	gwlStyle          = -16
	gwlExStyle        = -20
	SW_SHOWNOACTIVATE = 4
	SW_SHOW           = 5
	SW_SHOWNA         = 8
)

// SetWindowPos
const (
	SWP_NOSIZE       = 0x0001
	SWP_NOMOVE       = 0x0002
	SWP_NOZORDER     = 0x0004
	SWP_NOREDRAW     = 0x0008
	SWP_NOACTIVATE   = 0x0010
	SWP_FRAMECHANGED = 0x0020
	SWP_SHOWWINDOW   = 0x0040
	HWND_TOPMOST     = uintptr(^uintptr(0)) // (HWND)-1
	HWND_TOP         = 0
)

// SetForegroundWindow constraints are loosened by these window messages so the
// launcher can steal the focus without a stray click.
const (
	ASFW_ANY = uint32(^uint32(0))
)

// GetSystemMetrics
const (
	SM_CXSCREEN        = 0
	SM_CYSCREEN        = 1
	SM_CXSMICON        = 49
	SM_CYSMICON        = 50
	SM_CXICON          = 11
	SM_CYICON          = 12
	SM_XVIRTUALSCREEN  = 76
	SM_YVIRTUALSCREEN  = 77
	SM_CXVIRTUALSCREEN = 78
	SM_CYVIRTUALSCREEN = 79
)

// Painting primitives
const (
	PS_SOLID        = 0
	TRANSPARENT     = 1
	DT_LEFT         = 0x00000000
	DT_CENTER       = 0x00000001
	DT_RIGHT        = 0x00000002
	DT_VCENTER      = 0x00000004
	DT_SINGLELINE   = 0x00000020
	DT_END_ELLIPSIS = 0x00008000
	DT_NOPREFIX     = 0x00000800
	DT_NOCLIP       = 0x00000100
)

// Font weights
const (
	FW_NORMAL           = 400
	FW_MEDIUM           = 500
	FW_SEMIBOLD         = 600
	FW_BOLD             = 700
	DEFAULT_CHARSET     = 1
	OUT_DEFAULT_PRECIS  = 0
	CLIP_DEFAULT_PRECIS = 0
	CLEARTYPE_QUALITY   = 5
	DEFAULT_PITCH       = 0
	FF_DONTCARE         = 0
)

// DWM
const (
	DWMWA_USE_IMMERSIVE_DARK_MODE     = 20
	DWMWA_USE_IMMERSIVE_DARK_MODE_OLD = 19
	DWMWA_WINDOW_CORNER_PREFERENCE    = 33
	DWMWA_BORDER_COLOR                = 34
	DWMWCP_ROUND                      = 2
)

// Shell / icons
const (
	SHGFI_ICON              = 0x00000100
	SHGFI_LARGEICON         = 0x000000000
	SHGFI_SMALLICON         = 0x000000001
	SHGFI_SYSICONINDEX      = 0x00004000
	SHGFI_SHELLICONSIZE     = 0x00000004
	SHGFI_USEFILEATTRIBUTES = 0x000000010
	SHGFI_OPENICON          = 0x000000002
	FILE_ATTRIBUTE_NORMAL   = 0x00000080
	MAX_PATH                = 260
	DI_NORMAL               = 0x0003
	IMAGE_ICON              = 1
	LR_LOADFROMFILE         = 0x00000010
	LR_DEFAULTSIZE          = 0x00000040
	DI_MASK                 = 0x0001
	SRCCOPY                 = 0x00CC0020
	HALFTONE                = 4
	COLORONCOLOR            = 3
	DIB_RGB_COLORS          = 0
	BI_RGB                  = 0
	AC_SRC_OVER             = 0
	AC_SRC_ALPHA            = 1
)

// Tray
const (
	NIM_ADD              = 0
	NIM_MODIFY           = 1
	NIM_DELETE           = 2
	NIM_SETVERSION       = 4
	NIF_MESSAGE          = 0x00000001
	NIF_ICON             = 0x00000002
	NIF_TIP              = 0x00000004
	NIF_INFO             = 0x00000010
	NIF_SHOWTIP          = 0x00000080
	NIIF_INFO            = 0x00000001
	NIIF_WARNING         = 0x00000002
	NOTIFYICON_VERSION_4 = 4
)

// Menus
const (
	MF_STRING       = 0x00000000
	MF_SEPARATOR    = 0x00000800
	MF_CHECKED      = 0x00000008
	MF_GRAYED       = 0x00000001
	MF_DISABLED     = 0x00000002
	MF_DEFAULT      = 0x00001000
	TPM_RETURNCMD   = 0x0100
	TPM_RIGHTBUTTON = 0x0002
	TPM_NONOTIFY    = 0x0080
	TPM_LEFTALIGN   = 0x0000
)

// MessageBox
const (
	MB_OK              = 0x00000000
	MB_OKCANCEL        = 0x00000001
	MB_YESNO           = 0x00000004
	MB_ICONINFORMATION = 0x00000040
	MB_ICONWARNING     = 0x00000030
	MB_ICONQUESTION    = 0x00000020
	MB_ICONERROR       = 0x00000010
	MB_SETFOREGROUND   = 0x00010000
	IDOK               = 1
	IDCANCEL           = 2
	IDYES              = 6
	IDNO               = 7
)

// Keyboard
const (
	WH_KEYBOARD_LL     = 13
	HC_ACTION          = 0
	KEYEVENTF_KEYUP    = 0x0002
	KEYEVENTF_UNICODE  = 0x0004
	KEYEVENTF_SCANCODE = 0x0008
	VK_LBUTTON         = 0x01
	VK_RBUTTON         = 0x02
	VK_LSHIFT          = 0xA0
	VK_RSHIFT          = 0xA1
	VK_LCONTROL        = 0xA2
	VK_RCONTROL        = 0xA3
	VK_LMENU           = 0xA4
	VK_RMENU           = 0xA5
	VK_CANCEL          = 0x03
	VK_MBUTTON         = 0x04
	VK_BACK            = 0x08
	VK_TAB             = 0x09
	VK_CLEAR           = 0x0C
	VK_RETURN          = 0x0D
	VK_SHIFT           = 0x10
	VK_CONTROL         = 0x11
	VK_MENU            = 0x12
	VK_PAUSE           = 0x13
	VK_CAPITAL         = 0x14
	VK_ESCAPE          = 0x1B
	VK_SPACE           = 0x20
	VK_PRIOR           = 0x21
	VK_NEXT            = 0x22
	VK_END             = 0x23
	VK_HOME            = 0x24
	VK_LEFT            = 0x25
	VK_UP              = 0x26
	VK_RIGHT           = 0x27
	VK_DOWN            = 0x28
	VK_SELECT          = 0x29
	VK_PRINT           = 0x2A
	VK_EXECUTE         = 0x2B
	VK_SNAPSHOT        = 0x2C
	VK_INSERT          = 0x2D
	VK_DELETE          = 0x2E
	VK_HELP            = 0x2F
	VK_LWIN            = 0x5B
	VK_RWIN            = 0x5C
	VK_APPS            = 0x5D
	VK_NUMPAD0         = 0x60
	VK_MULTIPLY        = 0x6A
	VK_ADD             = 0x6B
	VK_SUBTRACT        = 0x6D
	VK_DECIMAL         = 0x6E
	VK_DIVIDE          = 0x6F
	VK_F1              = 0x70
	VK_F24             = 0x87
	VK_OEM_1           = 0xBA
	VK_OEM_PLUS        = 0xBB
	VK_OEM_COMMA       = 0xBC
	VK_OEM_MINUS       = 0xBD
	VK_OEM_PERIOD      = 0xBE
	VK_OEM_2           = 0xBF
	VK_OEM_3           = 0xC0
	VK_OEM_4           = 0xDB
	VK_OEM_5           = 0xDC
	VK_OEM_6           = 0xDD
	VK_OEM_7           = 0xDE
	LLKHF_INJECTED     = 0x10
)

// Timer ids and control ids are app specific and live next to their users.

// Token
const (
	TOKEN_QUERY          = 0x0008
	TokenElevation       = 20
	ERROR_ALREADY_EXISTS = 183
)

// Registry
const (
	HKEY_CURRENT_USER    = uintptr(0x80000001)
	HKEY_LOCAL_MACHINE   = uintptr(0x80000002)
	KEY_READ             = 0x00020019
	KEY_WRITE            = 0x00020006
	REG_SZ               = 1
	REG_DWORD            = 4
	REG_EXPAND_SZ        = 2
	RRF_RT_REG_SZ        = 0x00000002
	RRF_RT_REG_EXPAND_SZ = 0x00000004
	RRF_RT_REG_DWORD     = 0x00000010
	RRF_NOEXPAND         = 0x10000000
)

// COM
const (
	COINIT_APARTMENTTHREADED = 0x2
	CLSCTX_INPROC_SERVER     = 0x1
	CSIDL_APPDATA            = 0x001A
	CSIDL_LOCAL_APPDATA      = 0x001C
	CSIDL_PROGRAMS           = 0x0002
	CSIDL_COMMON_PROGRAMS    = 0x0017
	CSIDL_PERSONAL           = 0x0005
	CSIDL_DESKTOPDIRECTORY   = 0x0010
	CSIDL_STARTMENU          = 0x000B
	CSIDL_COMMON_STARTMENU   = 0x0016
	CSIDL_STARTUP            = 0x0007
)

// DPI awareness contexts
const (
	DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2         = uintptr(^uintptr(3)) // (DPI_AWARENESS_CONTEXT)-4
	MDT_EFFECTIVE_DPI                                  = 0
	IDC_ARROW                                          = 32512
	IDC_HAND                                           = 32649
	IDC_IBEAM                                          = 32513
	CREATE_ICON_VERSION                                = 0x00030000
	MagicInjectedInput                         uintptr = 0x59414C47 // 'YALG'
	PM_REMOVE                                          = 0x0001
)
