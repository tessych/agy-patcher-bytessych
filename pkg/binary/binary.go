package binary

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"sort"
)

// Range представляет диапазон байт [Start, End) в файле.
type Range struct {
	Start int64
	End   int64
}

// ExecutableInfo содержит информацию об исполняемых секциях и архитектуре файла.
type ExecutableInfo struct {
	Ranges []Range
	Arch   string // "x64", "arm64", "all"
	Format string // "elf", "pe", "macho", "raw"
}

// Machine architecture constants
const (
	ArchX64   = "x64"
	ArchARM64 = "arm64"
	ArchAll   = "all"
)

// NormalizeRanges нормализует, сортирует и объединяет пересекающиеся диапазоны.
func NormalizeRanges(ranges []Range, fileSize int64) []Range {
	var clean []Range
	for _, r := range ranges {
		start := r.Start
		end := r.End
		if start < 0 {
			start = 0
		}
		if end > fileSize {
			end = fileSize
		}
		if start < end {
			clean = append(clean, Range{Start: start, End: end})
		}
	}

	if len(clean) == 0 {
		return nil
	}

	sort.Slice(clean, func(i, j int) bool {
		return clean[i].Start < clean[j].Start
	})

	out := []Range{clean[0]}
	for _, cur := range clean[1:] {
		last := &out[len(out)-1]
		if cur.Start <= last.End {
			if cur.End > last.End {
				last.End = cur.End
			}
		} else {
			out = append(out, cur)
		}
	}
	return out
}

func readExactAt(r io.ReaderAt, offset int64, size int) ([]byte, error) {
	buf := make([]byte, size)
	n, err := r.ReadAt(buf, offset)
	if err != nil && err != io.EOF {
		return nil, err
	}
	if n != size {
		return nil, io.ErrUnexpectedEOF
	}
	return buf, nil
}

// InspectFile определяет формат, архитектуру и исполняемые диапазоны файла.
func InspectFile(path string) (*ExecutableInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	fileSize := fi.Size()
	if fileSize == 0 {
		return &ExecutableInfo{Ranges: nil, Arch: ArchAll, Format: "empty"}, nil
	}

	header := make([]byte, 16)
	n, _ := f.ReadAt(header, 0)
	if n < 4 {
		return &ExecutableInfo{
			Ranges: []Range{{Start: 0, End: fileSize}},
			Arch:   ArchAll,
			Format: "raw",
		}, nil
	}

	// 1. Проверяем ELF (\x7fELF)
	if bytes.Equal(header[:4], []byte{0x7f, 'E', 'L', 'F'}) {
		return parseELF(f, fileSize)
	}

	// 2. Проверяем PE (MZ)
	if header[0] == 'M' && header[1] == 'Z' && fileSize > 0x40 {
		info, err := parsePE(f, fileSize)
		if err == nil {
			return info, nil
		}
		// Если это не PE, проваливаемся к обычному файлу
	}

	// 3. Проверяем Mach-O
	if isMachOMagic(header[:4]) {
		info, err := parseMachO(f, fileSize)
		if err == nil {
			return info, nil
		}
	}

	// Для скриптов (например, main.js) сканируем весь файл
	return &ExecutableInfo{
		Ranges: []Range{{Start: 0, End: fileSize}},
		Arch:   ArchAll,
		Format: "raw",
	}, nil
}

func parseELF(r io.ReaderAt, fileSize int64) (*ExecutableInfo, error) {
	ident, err := readExactAt(r, 0, 16)
	if err != nil {
		return nil, err
	}

	elfClass := ident[4]    // 1 = 32-bit, 2 = 64-bit
	dataOrder := ident[5]   // 1 = little endian, 2 = big endian

	var endian binary.ByteOrder
	if dataOrder == 1 {
		endian = binary.LittleEndian
	} else if dataOrder == 2 {
		endian = binary.BigEndian
	} else {
		return nil, errors.New("unknown ELF data order")
	}

	machBytes, err := readExactAt(r, 18, 2)
	if err != nil {
		return nil, err
	}
	machine := endian.Uint16(machBytes)

	var arch string
	switch machine {
	case 0x3e: // EM_X86_64
		arch = ArchX64
	case 0xb7: // EM_AARCH64
		arch = ArchARM64
	default:
		arch = "unknown"
	}

	var ranges []Range
	if elfClass == 2 { // 64-bit
		hdr, err := readExactAt(r, 0, 64)
		if err != nil {
			return nil, err
		}
		phoff := int64(endian.Uint64(hdr[32:40]))
		phentsize := int(endian.Uint16(hdr[54:56]))
		phnum := int(endian.Uint16(hdr[56:58]))

		if phentsize >= 56 {
			for i := 0; i < phnum; i++ {
				entry, err := readExactAt(r, phoff+int64(i*phentsize), phentsize)
				if err != nil {
					break
				}
				pType := endian.Uint32(entry[0:4])
				flags := endian.Uint32(entry[4:8])
				offset := int64(endian.Uint64(entry[8:16]))
				size := int64(endian.Uint64(entry[32:40]))

				// PT_LOAD (1) и исполняемый флаг PF_X (1)
				if pType == 1 && (flags&1) != 0 {
					ranges = append(ranges, Range{Start: offset, End: offset + size})
				}
			}
		}
	} else if elfClass == 1 { // 32-bit
		hdr, err := readExactAt(r, 0, 52)
		if err != nil {
			return nil, err
		}
		phoff := int64(endian.Uint32(hdr[28:32]))
		phentsize := int(endian.Uint16(hdr[42:44]))
		phnum := int(endian.Uint16(hdr[44:46]))

		if phentsize >= 32 {
			for i := 0; i < phnum; i++ {
				entry, err := readExactAt(r, phoff+int64(i*phentsize), phentsize)
				if err != nil {
					break
				}
				pType := endian.Uint32(entry[0:4])
				offset := int64(endian.Uint32(entry[4:8]))
				size := int64(endian.Uint32(entry[16:20]))
				flags := endian.Uint32(entry[24:28])

				if pType == 1 && (flags&1) != 0 {
					ranges = append(ranges, Range{Start: offset, End: offset + size})
				}
			}
		}
	}

	return &ExecutableInfo{
		Ranges: NormalizeRanges(ranges, fileSize),
		Arch:   arch,
		Format: "elf",
	}, nil
}

func parsePE(r io.ReaderAt, fileSize int64) (*ExecutableInfo, error) {
	peOffBytes, err := readExactAt(r, 0x3c, 4)
	if err != nil {
		return nil, err
	}
	peOff := int64(binary.LittleEndian.Uint32(peOffBytes))

	peSig, err := readExactAt(r, peOff, 4)
	if err != nil || !bytes.Equal(peSig, []byte{'P', 'E', 0, 0}) {
		return nil, errors.New("invalid PE signature")
	}

	coff, err := readExactAt(r, peOff+4, 20)
	if err != nil {
		return nil, err
	}

	machine := binary.LittleEndian.Uint16(coff[0:2])
	sectionCount := int(binary.LittleEndian.Uint16(coff[2:4]))
	optionalSize := int64(binary.LittleEndian.Uint16(coff[16:18]))

	var arch string
	switch machine {
	case 0x8664: // IMAGE_FILE_MACHINE_AMD64
		arch = ArchX64
	case 0xaa64: // IMAGE_FILE_MACHINE_ARM64
		arch = ArchARM64
	default:
		arch = "unknown"
	}

	sectionTable := peOff + 24 + optionalSize
	var ranges []Range

	for i := 0; i < sectionCount; i++ {
		sec, err := readExactAt(r, sectionTable+int64(i*40), 40)
		if err != nil {
			break
		}
		rawSize := int64(binary.LittleEndian.Uint32(sec[16:20]))
		rawOffset := int64(binary.LittleEndian.Uint32(sec[20:24]))
		characteristics := binary.LittleEndian.Uint32(sec[36:40])

		// IMAGE_SCN_MEM_EXECUTE = 0x20000000
		if (characteristics & 0x20000000) != 0 {
			ranges = append(ranges, Range{Start: rawOffset, End: rawOffset + rawSize})
		}
	}

	return &ExecutableInfo{
		Ranges: NormalizeRanges(ranges, fileSize),
		Arch:   arch,
		Format: "pe",
	}, nil
}

func isMachOMagic(magic []byte) bool {
	if len(magic) < 4 {
		return false
	}
	m := binary.BigEndian.Uint32(magic)
	return m == 0xfeedfacf || m == 0xcffaedfe || // 64-bit Mach-O
		m == 0xcafebabe || m == 0xbebafeca || // FAT Mach-O 32-bit
		m == 0xcafebabf || m == 0xbfbafeca // FAT Mach-O 64-bit
}

func parseMachO(r io.ReaderAt, fileSize int64) (*ExecutableInfo, error) {
	magicBytes, err := readExactAt(r, 0, 4)
	if err != nil {
		return nil, err
	}

	// 64-bit thin Mach-O
	if bytes.Equal(magicBytes, []byte{0xcf, 0xfa, 0xed, 0xfe}) || bytes.Equal(magicBytes, []byte{0xfe, 0xed, 0xfa, 0xcf}) {
		return parseMachOSlice(r, fileSize, 0, fileSize)
	}

	// Fat Mach-O
	var endian binary.ByteOrder
	var is64 bool
	switch {
	case bytes.Equal(magicBytes, []byte{0xca, 0xfe, 0xba, 0xbe}):
		endian = binary.BigEndian
	case bytes.Equal(magicBytes, []byte{0xbe, 0xba, 0xfe, 0xca}):
		endian = binary.LittleEndian
	case bytes.Equal(magicBytes, []byte{0xca, 0xfe, 0xba, 0xbf}):
		endian = binary.BigEndian
		is64 = true
	case bytes.Equal(magicBytes, []byte{0xbf, 0xba, 0xfe, 0xca}):
		endian = binary.LittleEndian
		is64 = true
	default:
		return nil, errors.New("unsupported Mach-O format")
	}

	countBytes, err := readExactAt(r, 4, 4)
	if err != nil {
		return nil, err
	}
	count := int(endian.Uint32(countBytes))
	entrySize := 20
	if is64 {
		entrySize = 32
	}

	var allRanges []Range
	var arch string
	for i := 0; i < count; i++ {
		entry, err := readExactAt(r, int64(8+i*entrySize), entrySize)
		if err != nil {
			break
		}
		var offset, size int64
		if is64 {
			offset = int64(endian.Uint64(entry[8:16]))
			size = int64(endian.Uint64(entry[16:24]))
		} else {
			offset = int64(endian.Uint32(entry[8:12]))
			size = int64(endian.Uint32(entry[12:16]))
		}
		sliceInfo, err := parseMachOSlice(r, fileSize, offset, size)
		if err == nil {
			allRanges = append(allRanges, sliceInfo.Ranges...)
			if arch == "" {
				arch = sliceInfo.Arch
			}
		}
	}

	return &ExecutableInfo{
		Ranges: NormalizeRanges(allRanges, fileSize),
		Arch:   arch,
		Format: "macho",
	}, nil
}

func parseMachOSlice(r io.ReaderAt, fileSize, base, sliceSize int64) (*ExecutableInfo, error) {
	magicBytes, err := readExactAt(r, base, 4)
	if err != nil {
		return nil, err
	}

	var endian binary.ByteOrder
	if bytes.Equal(magicBytes, []byte{0xcf, 0xfa, 0xed, 0xfe}) {
		endian = binary.LittleEndian
	} else if bytes.Equal(magicBytes, []byte{0xfe, 0xed, 0xfa, 0xcf}) {
		endian = binary.BigEndian
	} else {
		return nil, errors.New("unsupported Mach-O slice magic")
	}

	header, err := readExactAt(r, base, 32)
	if err != nil {
		return nil, err
	}

	cputype := endian.Uint32(header[4:8])
	var arch string
	switch cputype {
	case 0x01000007: // CPU_TYPE_X86_64
		arch = ArchX64
	case 0x0100000c: // CPU_TYPE_ARM64
		arch = ArchARM64
	default:
		arch = "unknown"
	}

	commandCount := int(endian.Uint32(header[16:20]))
	pos := base + 32
	var ranges []Range

	for i := 0; i < commandCount; i++ {
		cmdHeader, err := readExactAt(r, pos, 8)
		if err != nil {
			break
		}
		cmd := endian.Uint32(cmdHeader[0:4])
		cmdSize := int64(endian.Uint32(cmdHeader[4:8]))

		if cmdSize < 8 || pos+cmdSize > base+sliceSize {
			break
		}

		// LC_SEGMENT_64 = 0x19
		if cmd == 0x19 {
			seg, err := readExactAt(r, pos, int(cmdSize))
			if err == nil && len(seg) >= 72 {
				sectionCount := int(endian.Uint32(seg[64:68]))
				secPos := 72
				for s := 0; s < sectionCount && secPos+80 <= len(seg); s++ {
					sec := seg[secPos : secPos+80]
					secName := bytes.TrimRight(sec[0:16], "\x00")
					segName := bytes.TrimRight(sec[16:32], "\x00")
					size := int64(endian.Uint64(sec[40:48]))
					offset := int64(endian.Uint32(sec[48:52]))
					flags := endian.Uint32(sec[64:68])

					isCode := (string(segName) == "__TEXT" && string(secName) == "__text") ||
						(flags&(0x80000000|0x00000400)) != 0

					if isCode {
						ranges = append(ranges, Range{
							Start: base + offset,
							End:   base + offset + size,
						})
					}
					secPos += 80
				}
			}
		}
		pos += cmdSize
	}

	return &ExecutableInfo{
		Ranges: NormalizeRanges(ranges, fileSize),
		Arch:   arch,
		Format: "macho",
	}, nil
}
