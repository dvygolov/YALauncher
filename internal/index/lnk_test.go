package index

import (
	"encoding/binary"
	"testing"
)

// buildLink constructs a minimal Shell Link with the Unicode flag and the
// given string fields, following the documented layout.
func buildLink(target, args, workdir string, unicode bool) []byte {
	var out []byte
	header := make([]byte, 0x4C)
	binary.LittleEndian.PutUint32(header[0:], 0x4C)
	flags := uint32(0)
	if unicode {
		flags |= flagIsUnicode
	}
	flags |= flagHasLinkInfo | flagHasName | flagHasWorkDir | flagHasArgs
	binary.LittleEndian.PutUint32(header[20:], flags)
	out = append(out, header...)
	out = append(out, make([]byte, 16)...) // CLSID
	out = append(out, 0, 0)                // empty IDList
	// LinkInfo with the target path
	var li []byte
	if unicode {
		li = make([]byte, 0x24+(len(target)+1)*2)
		for i, r := range target {
			binary.LittleEndian.PutUint16(li[0x24+i*2:], uint16(r))
		}
		binary.LittleEndian.PutUint32(li[28:], 0x24)
	} else {
		li = make([]byte, 0x24+len(target)+1)
		copy(li[0x24:], target)
	}
	binary.LittleEndian.PutUint32(li[0:], uint32(len(li)))
	binary.LittleEndian.PutUint32(li[4:], 0x24)
	binary.LittleEndian.PutUint32(li[8:], volumeIDAndLocalBasePath)
	// LocalBasePath offset is relative to the LinkInfo header
	binary.LittleEndian.PutUint32(li[16:], 0x24)
	out = append(out, li...)

	str := func(s string) []byte {
		if unicode {
			var u []uint16
			for _, r := range s {
				u = append(u, uint16(r))
			}
			u = append(u, 0)
			b := make([]byte, 2+len(u)*2)
			binary.LittleEndian.PutUint16(b[0:], uint16(len(u)))
			for i, v := range u {
				binary.LittleEndian.PutUint16(b[2+i*2:], v)
			}
			return b
		}
		b := make([]byte, 2+len(s))
		binary.LittleEndian.PutUint16(b[0:], uint16(len(s)))
		copy(b[2:], s)
		return b
	}
	out = append(out, str("Display Name")...)
	out = append(out, str(workdir)...)
	out = append(out, str(args)...)
	return out
}

func TestParseLinkUnicode(t *testing.T) {
	b := buildLink(`C:\Program Files\Chrome\chrome.exe`, `--incognito`, `C:\Program Files\Chrome`, true)
	path := writeTemp(t, "chrome.lnk", b)
	target, args, _, dir := resolveShortcutImpl(path)
	if target != `C:\Program Files\Chrome\chrome.exe` {
		t.Fatalf("target = %q", target)
	}
	if args != "--incognito" {
		t.Fatalf("args = %q", args)
	}
	if dir != `C:\Program Files\Chrome` {
		t.Fatalf("dir = %q", dir)
	}
}

func TestParseLinkAnsi(t *testing.T) {
	b := buildLink(`C:\Tools\yi.exe`, "", "", false)
	path := writeTemp(t, "yi.lnk", b)
	target, _, _, _ := resolveShortcutImpl(path)
	if target != `C:\Tools\yi.exe` {
		t.Fatalf("target = %q", target)
	}
}
