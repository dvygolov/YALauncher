package index

import (
	"encoding/binary"
	"os"
	"strings"
	"unicode/utf16"
)

const (
	lnkHeaderSize = 0x4C

	// Link flags
	flagHasLinkInfo = 0x00000002
	flagHasName     = 0x00000004
	flagHasRelPath  = 0x00000008
	flagHasWorkDir  = 0x00000010
	flagHasArgs     = 0x00000020
	flagHasIconLoc  = 0x00000040
	flagIsUnicode   = 0x00000080
	flagHasExpIcon  = 0x00000100
	flagForceNoLnk  = 0x00000001

	// VolumeID / LinkInfo constants
	volumeIDAndLocalBasePath = 0x00000001
	commonNetworkRelative    = 0x00000002
)

// resolveShortcutImpl reads the fields of a Shell Link file.
func resolveShortcutImpl(path string) (target, args, description, workingDir string) {
	data, err := os.ReadFile(path)
	if err != nil || len(data) < lnkHeaderSize {
		return "", "", "", ""
	}
	if binary.LittleEndian.Uint32(data[0:]) != 0x4C {
		return "", "", "", ""
	}
	flags := binary.LittleEndian.Uint32(data[20:])
	unicode := flags&flagIsUnicode != 0
	off := lnkHeaderSize + 16

	// The LinkInfo block must be located without trusting the IDList size
	// field: we agree with os.ReadFile about nothing else, but a tag says the
	// shell writes an unaligned first bytes copy and the real block sits on a
	// later 4 byte boundary. Every boundary is scored on its size words and
	// the best scoring offset wins.
	start := lnkHeaderSize + 16
	idListSize := 0
	if start+2 <= len(data) {
		idListSize = int(binary.LittleEndian.Uint16(data[start:]))
	}
	limit := len(data)
	off = 0
	best := -1
	for p := limit - 0x24; p >= start; p-- {
		score := linkInfoScore(data, p, p == start+2+idListSize)
		if score > best {
			best, off = score, p
		}
	}
	if best <= 0 {
		off = start + 2 + idListSize
	}

	linkInfoSize := 0
	if off+4 <= len(data) {
		linkInfoSize = int(binary.LittleEndian.Uint32(data[off:]))
	}
	if flags&flagHasLinkInfo == 0 {
		// The header says there is no LinkInfo: the file right after the id
		// list is the optional description, and the target only lives in the
		// id list.
		return targetFromIDList(data), "", "", ""
	}
	var linkInfo []byte
	if linkInfoSize > 0 && off+linkInfoSize <= len(data) && validLinkInfoAt(data, off) {
		linkInfo = data[off : off+linkInfoSize]
	}
	off += linkInfoSize

	readStringField := func() string {
		if off+2 > len(data) {
			return ""
		}
		var count int
		if unicode {
			count = int(binary.LittleEndian.Uint16(data[off:]))
			off += 2
			need := count * 2
			if count == 0 || off+need > len(data) {
				return ""
			}
			u := make([]uint16, count)
			for i := 0; i < count; i++ {
				u[i] = binary.LittleEndian.Uint16(data[off+i*2:])
			}
			off += need
			return trimNUL(decodeU16(u))
		}
		count = int(binary.LittleEndian.Uint16(data[off:]))
		off += 2
		if count == 0 || off+count > len(data) {
			return ""
		}
		s := decodeCP1251(data[off : off+count])
		off += count
		return trimNUL(s)
	}

	skipField := func() {
		if off+2 > len(data) {
			return
		}
		count := int(binary.LittleEndian.Uint16(data[off:]))
		off += 2
		if count == 0 {
			return
		}
		if unicode {
			off += count * 2
		} else {
			off += count
		}
		if off > len(data) {
			off = len(data)
		}
	}

	// The order of the optional fields is fixed by the format.
	if flags&flagHasName != 0 {
		skipField()
	}
	if flags&flagHasRelPath != 0 {
		skipField()
	}
	if flags&flagHasWorkDir != 0 {
		workingDir = readStringField()
	}
	if flags&flagHasArgs != 0 {
		args = readStringField()
	}
	if flags&flagHasIconLoc != 0 {
		skipField()
	}

	target = targetFromLinkInfo(linkInfo)
	if target == "" {
		target = targetFromIDList(data)
	}
	return target, args, description, workingDir
}

// targetFromLinkInfo reads LocalBasePath, the Unicode variant or the network
// share path. Only paths that look like a Windows path are accepted: a wrong
// offset usually lands in binary data and must not be mistaken for a target.
func targetFromLinkInfo(b []byte) string {
	if len(b) < 0x1C {
		return ""
	}
	flags := binary.LittleEndian.Uint32(b[8:])
	// The path offsets are relative to the LinkInfo header, whose first field
	// is the block size. Older headers (0x1C) are padded by four bytes so the
	// Kernel ID starts aligned; the Unicode offsets live after it.
	headerSize := int(binary.LittleEndian.Uint32(b[4:]))
	// LinkInfo offsets are relative to the block start minus the size field:
	// the size is followed by the header, and both the local base path and
	// the Unicode path are stored from that point on.
	baseOff := int(binary.LittleEndian.Uint32(b[16:]))
	uniBase := 0
	if headerSize >= 0x24 {
		uniBase = int(binary.LittleEndian.Uint32(b[28:]))
	}
	_ = headerSize
	if flags&volumeIDAndLocalBasePath != 0 {
		u := ""
		if uniBase > 0 && uniBase < len(b) {
			u = readUTF16Z(b, uniBase)
		}
		a := readCP1251Z(b, baseOff)
		vol := volumeOf(b)
		// Windows usually stores a full absolute path in both fields. When it
		// only stores a suffix ("\Program Files\..."), the drive comes from
		// the VolumeID, and when all it stored is the drive root the shortcut
		// does not know where the program lives - the id list does.
		if absolutePath(u) && !bareRoot(u) {
			return u
		}
		if absolutePath(a) && !bareRoot(a) {
			return a
		}
		for _, rel := range []string{a, u} {
			if relativePath(rel) && vol != "" {
				return vol + strings.ReplaceAll(rel, "/", `\`)
			}
		}
	}
	if flags&commonNetworkRelative != 0 {
		cnrlOff := int(binary.LittleEndian.Uint32(b[20:]))
		if cnrlOff > 0 && cnrlOff+8 <= len(b) {
			netNameOff := int(binary.LittleEndian.Uint32(b[cnrlOff+4:]))
			prefix := readCP1251Z(b, cnrlOff+netNameOff)
			suffix := readCP1251Z(b, int(binary.LittleEndian.Uint32(b[24:])))
			if prefix != "" {
				// The share name is stored without its leading backslashes;
				// the common suffix normally starts with one.
				share := `\\` + strings.TrimLeft(prefix, `\\`)
				if !strings.HasPrefix(suffix, `\`) {
					suffix = `\` + suffix
				}
				return strings.TrimRight(share, `\\`) + suffix
			}
		}
	}
	return ""
}

// volumeOf decodes the VolumeID at the start of a LinkInfo block. The drive
// letter lives in the volume label field, which is "C" or "System\C" on the
// systems that write the ANSI path without a drive prefix.
func volumeOf(b []byte) string {
	if len(b) >= 0x18 {
		volOff := int(binary.LittleEndian.Uint32(b[12:]))
		if volOff > 0 && volOff+16 <= len(b) {
			labelOff := int(binary.LittleEndian.Uint32(b[volOff+12:]))
			if labelOff >= 16 && volOff+labelOff < len(b) {
				if vol := normalizeVolume(readCP1251Z(b, volOff+labelOff)); vol != "" {
					return vol
				}
			}
		}
	}
	// Some writers leave the VolumeID header fields unset. The record is
	// still there right after it: a size, a drive type, a serial, a 16 byte
	// label offset and the label, the label ending in a single drive letter
	// (or "System\C" on the systems that name the volume root).
	// The volume used to hold the drive as a single letter in the label
	// field, which is what every Windows version before Vista wrote.
	for i := 0x18; i+17 <= len(b); i += 4 {
		size := binary.LittleEndian.Uint32(b[i:])
		if size < 16 || size > 32 {
			continue
		}
		label := readCP1251Z(b, i+16)
		if len(label) != 1 {
			continue
		}
		c := label[0]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
			return strings.ToUpper(label) + ":"
		}
	}
	return ""
}

// normalizeVolume turns "System\C" or "C" into "C:".
func normalizeVolume(label string) string {
	label = strings.TrimRight(label, "\\")
	if i := strings.LastIndexByte(label, '\\'); i >= 0 {
		label = label[i+1:]
	}
	if label == "" {
		return ""
	}
	if len(label) == 1 && label[0] >= 'a' && label[0] <= 'z' {
		label = strings.ToUpper(label)
	}
	return label + ":"
}

// utf16Mojibake spots a Unicode field read from an ANSI offset. A real .lnk on
// a Cyrillic or with after a rewrite on a shared path still parses as text
// whose code units all fit in the ASCII range; a misread UTF-16 string has
// stray NULs and control characters.
func utf16Mojibake(b []byte, off int) bool {
	if off <= 0 || off >= len(b) {
		return true
	}
	units := 0
	for i := off; i+1 < len(b) && units < 256; i += 2 {
		u := binary.LittleEndian.Uint16(b[i:])
		if u == 0 {
			break
		}
		if u < 0x20 || (u >= 0x7F && u < 0x100) {
			return true
		}
		units++
	}
	return false
}

// absolutePath reports a path with its own drive letter.
func absolutePath(p string) bool {
	if len(p) < 3 || p[1] != ':' {
		return false
	}
	c := p[0]
	if !(c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z') {
		return false
	}
	return p[2] == '\\' || p[2] == '/'
}

// relativePath reports a stored suffix such as "\Program Files\...".
func relativePath(p string) bool {
	return len(p) > 1 && (p[0] == '\\' || p[0] == '/')
}

// onRecordedDrive reports whether a target lives on the drive the shortcut was
// created on. A shortcut that was moved from another machine carries a stale
// VolumeID, and then any absolute path is fine.
func onRecordedDrive(path, vol string) bool {
	if vol == "" {
		return true
	}
	return strings.EqualFold(path[:2], vol)
}

// bareRoot reports a path that is only a drive root: such a shortcut does not
// say where the program starts.
func bareRoot(p string) bool {
	return len(p) == 3 && p[1] == ':' && (p[2] == '\\' || p[2] == '/')
}

// labelPath hides a bare drive root: an installer that stored "C:\" is far
// less useful in a launcher than the drive label Windows shows.
func labelPath(_, _, label string) string { return "" }

// validLocalPath accepts "C:\..." and network shares, and rejects empty
// strings, control characters and anything without a drive or a UNC prefix.
func validLocalPath(p string) bool {
	if len(p) < 3 || len(p) > 4096 {
		return false
	}
	if strings.HasPrefix(p, `\\`) {
		return true
	}
	if !(p[1] == ':' && ((p[0] >= 'A' && p[0] <= 'Z') || (p[0] >= 'a' && p[0] <= 'z'))) {
		return false
	}
	if p[2] != '\\' && p[2] != '/' {
		return false
	}
	for _, r := range p {
		if r < 0x20 {
			return false
		}
	}
	return true
}

// targetFromIDList picks the file entry out of the shell item id list. It is
// the fallback for links that were saved without a LinkInfo block.
func targetFromIDList(data []byte) string {
	if len(data) < 0x14 {
		return ""
	}
	off := lnkHeaderSize + 16
	for off+2 <= len(data) {
		size := int(binary.LittleEndian.Uint16(data[off:]))
		if size == 0 {
			break
		}
		if size < 2 || off+size > len(data) {
			break
		}
		item := data[off : off+size]
		// Only file entry items carry a useful name; their type byte is 0x31
		// or 0x32, and the ANSI name is at a fixed offset inside the item.
		if len(item) > 0x30 && (item[2] == 0x31 || item[2] == 0x32) {
			if name := readCP1251Z(item, 0x0E); name != "" {
				return name
			}
		}
		off += size
	}
	return ""
}

// validLinkInfoAt reports whether a LinkInfo header lives at pos.
func validLinkInfoAt(data []byte, pos int) bool {
	if pos < 0 || pos+0x1C > len(data) {
		return false
	}
	size := int(binary.LittleEndian.Uint32(data[pos:]))
	if size < 0x1C || size > 0x8000 || pos+size > len(data) {
		return false
	}
	header := int(binary.LittleEndian.Uint32(data[pos+4:]))
	if header != 0x1C && header < 0x24 {
		return false
	}
	flags := binary.LittleEndian.Uint32(data[pos+8:])
	return flags&^(uint32(0x7)) == 0 && flags != 0
}

// linkInfoScore rates a position as the start of a LinkInfo header. The size
// words are shared with the IDList walk, so a candidate whose size matches
// what comes right after it (a Unicode string count, or the end of the file)
// scores highest.
func linkInfoScore(data []byte, pos int, expected bool) int {
	if pos+0x1C > len(data) {
		return -1
	}
	if !validLinkInfoAt(data, pos) {
		return -1
	}
	size := int(binary.LittleEndian.Uint32(data[pos:]))
	score := 1
	if expected {
		score += 10
	}
	next := pos + size
	if next+2 > len(data) {
		return score
	}
	count := int(binary.LittleEndian.Uint16(data[next:]))
	if next+2+count*2 <= len(data) && count > 0 {
		score += 2
	}
	// The block is usually followed by a NUL terminated path and then the
	// Unicode string fields: a lot of zero bytes nearby is a good sign.
	zeros := 0
	for i := next; i < next+16 && i < len(data); i++ {
		if data[i] == 0 {
			zeros++
		}
	}
	if zeros >= 6 {
		score++
	}
	return score
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func headBytes(b []byte, n int) []byte {
	if len(b) < n {
		return b
	}
	return b[:n]
}

func readUTF16Z(b []byte, off int) string {
	if off <= 0 || off+2 > len(b) {
		return ""
	}
	for i := off; i+2 <= len(b); i += 2 {
		if binary.LittleEndian.Uint16(b[i:]) == 0 {
			u := make([]uint16, 0, (i-off)/2)
			for j := off; j < i; j += 2 {
				u = append(u, binary.LittleEndian.Uint16(b[j:]))
			}
			return decodeU16(u)
		}
	}
	return ""
}

func readCP1251Z(b []byte, off int) string {
	if off <= 0 || off >= len(b) {
		return ""
	}
	end := off
	for end < len(b) && b[end] != 0 {
		end++
	}
	return decodeCP1251(b[off:end])
}

func decodeU16(u []uint16) string { return string(utf16.Decode(u)) }

func trimNUL(s string) string { return strings.TrimRight(s, "\x00") }

// decodeCP1251 converts the legacy code page used by older shortcuts. The
// bytes above 0x7F are mapped through a compact table; anything unknown is
// replaced, which is fine for file names.
func decodeCP1251(b []byte) string {
	var sb strings.Builder
	sb.Grow(len(b))
	for _, c := range b {
		if c < 0x80 {
			sb.WriteByte(c)
			continue
		}
		sb.WriteRune(cp1251[c-0x80])
	}
	return sb.String()
}

var cp1251 = []rune{
	'Ђ', 'Ѓ', '‚', 'ѓ', '„', '…', '†', '‡', '€', '‰', 'Љ', '‹', 'Њ', 'Ќ', 'Ћ', 'Џ',
	'ђ', '‘', '’', '“', '”', '•', '–', '—', '�', '™', 'љ', '›', 'њ', 'ќ', 'ћ', 'џ',
	' ', 'Ў', 'ў', 'Ј', '¤', 'Ґ', '¦', '§', 'Ё', '©', 'Є', '«', '¬', '­', '®', 'Ї',
	'°', '±', 'І', 'і', 'ґ', 'µ', '¶', '·', 'ё', '№', 'є', '»', 'ј', 'Ѕ', 'ѕ', 'ї',
	'А', 'Б', 'В', 'Г', 'Д', 'Е', 'Ж', 'З', 'И', 'Й', 'К', 'Л', 'М', 'Н', 'О', 'П',
	'Р', 'С', 'Т', 'У', 'Ф', 'Х', 'Ц', 'Ч', 'Ш', 'Щ', 'Ъ', 'Ы', 'Ь', 'Э', 'Ю', 'Я',
	'а', 'б', 'в', 'г', 'д', 'е', 'ж', 'з', 'и', 'й', 'к', 'л', 'м', 'н', 'о', 'п',
	'р', 'с', 'т', 'у', 'ф', 'х', 'ц', 'ч', 'ш', 'щ', 'ъ', 'ы', 'ь', 'э', 'ю', 'я',
}
