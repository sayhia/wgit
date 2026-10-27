package main

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// =========================================================================
// Synthetic TTF builders
//
// All builders emit just enough structure for parseTTFCmap: a 12-byte sfnt
// header, a single 16-byte table record, and a cmap table with one or more
// encoding records.
// =========================================================================

// sfntWithTable builds a font whose table directory holds exactly one record
// (tagged tag) pointing at the trailing table payload.
func sfntWithTable(tag string, table []byte) []byte {
	const tableOffset = 28 // 12-byte sfnt header + one 16-byte table record
	font := make([]byte, tableOffset+len(table))
	binary.BigEndian.PutUint32(font[0:4], 0x00010000) // sfnt version
	binary.BigEndian.PutUint16(font[4:6], 1)          // numTables
	copy(font[12:16], tag)                            // record tag
	// Table record fields: tag[4] checksum[4] offset[4] length[4].
	binary.BigEndian.PutUint32(font[20:24], tableOffset)
	binary.BigEndian.PutUint32(font[24:28], uint32(len(table)))
	copy(font[tableOffset:], table)
	return font
}

// patchTableRecord rewrites the offset/length of the single table record.
func patchTableRecord(font []byte, offset, length uint32) {
	binary.BigEndian.PutUint32(font[20:24], offset)
	binary.BigEndian.PutUint32(font[24:28], length)
}

// cmapWithSubtable wraps one platform 3 / encoding 1 encoding record pointing
// at the subtable, which starts at cmap-relative offset 12.
func cmapWithSubtable(subtable []byte) []byte {
	const subtableOffset = 12 // 4-byte cmap header + one 8-byte encoding record
	cmap := make([]byte, subtableOffset+len(subtable))
	binary.BigEndian.PutUint16(cmap[0:2], 0)               // version
	binary.BigEndian.PutUint16(cmap[2:4], 1)               // numTables
	binary.BigEndian.PutUint16(cmap[4:6], 3)               // platformID
	binary.BigEndian.PutUint16(cmap[6:8], 1)               // encodingID
	binary.BigEndian.PutUint32(cmap[8:12], subtableOffset) // subtable offset
	copy(cmap[subtableOffset:], subtable)
	return cmap
}

// format4Segment describes one cmap format 4 segment. idRangeOffset is
// relative to the start of the idRangeOffset array (16+6*segCount); callers
// that point at the trailing glyphIdArray use 2*segCount for the first entry,
// because glyphIdArray begins immediately after the idRangeOffset array
// (16+8*segCount).
type format4Segment struct {
	start, end    uint16
	idDelta       uint16
	idRangeOffset uint16
}

// buildFormat4Segments encodes a format 4 subtable. All array offsets are
// explicit:
//
//	 0                                 format (2) / length (2) / language (2)
//	 6                                 segCountX2 / searchRange /
//	                                   entrySelector / rangeShift (8 bytes)
//	14                                 endCode[segCount]           (2*segCount)
//	14+2*segCount                      reservedPad                       (2)
//	16+2*segCount                      startCode[segCount]         (2*segCount)
//	16+4*segCount                      idDelta[segCount]           (2*segCount)
//	16+6*segCount                      idRangeOffset[segCount]     (2*segCount)
//	16+8*segCount                      glyphIdArray
func buildFormat4Segments(segs []format4Segment, glyphIdArray []uint16) []byte {
	segCount := len(segs)
	const endCodeOffset = 14
	startCodeOffset := 16 + 2*segCount
	idDeltaOffset := 16 + 4*segCount
	idRangeOffsetOffset := 16 + 6*segCount
	glyphArrayOffset := 16 + 8*segCount
	sub := make([]byte, glyphArrayOffset+2*len(glyphIdArray))

	binary.BigEndian.PutUint16(sub[0:2], 4)                  // format
	binary.BigEndian.PutUint16(sub[2:4], uint16(len(sub)))   // length
	binary.BigEndian.PutUint16(sub[6:8], uint16(segCount*2)) // segCountX2
	for i, seg := range segs {
		binary.BigEndian.PutUint16(sub[endCodeOffset+2*i:endCodeOffset+2*i+2], seg.end)
		binary.BigEndian.PutUint16(sub[startCodeOffset+2*i:startCodeOffset+2*i+2], seg.start)
		binary.BigEndian.PutUint16(sub[idDeltaOffset+2*i:idDeltaOffset+2*i+2], seg.idDelta)
		binary.BigEndian.PutUint16(sub[idRangeOffsetOffset+2*i:idRangeOffsetOffset+2*i+2], seg.idRangeOffset)
	}
	for i, g := range glyphIdArray {
		off := glyphArrayOffset + 2*i
		binary.BigEndian.PutUint16(sub[off:off+2], g)
	}
	return sub
}

// buildFormat4 encodes a format 4 subtable for the given [start, end] segments
// with zero idDelta/idRangeOffset, so every code maps to itself (nonzero for
// all ASCII codes).
func buildFormat4(segments [][2]uint16) []byte {
	segs := make([]format4Segment, len(segments))
	for i, seg := range segments {
		segs[i] = format4Segment{start: seg[0], end: seg[1]}
	}
	return buildFormat4Segments(segs, nil)
}

// buildFormat12 encodes a format 12 subtable. declared may exceed len(groups)
// to simulate an nGroups count larger than the encoded group array.
func buildFormat12(declared uint32, groups [][2]uint32) []byte {
	sub := make([]byte, 16+12*len(groups))
	binary.BigEndian.PutUint16(sub[0:2], 12)
	binary.BigEndian.PutUint32(sub[4:8], uint32(len(sub)))
	binary.BigEndian.PutUint32(sub[12:16], declared)
	for i, g := range groups {
		off := 16 + i*12
		binary.BigEndian.PutUint32(sub[off:off+4], g[0])
		binary.BigEndian.PutUint32(sub[off+4:off+8], g[1])
		// startGlyphID at off+8 stays zero; the parser never reads it.
	}
	return sub
}

// buildFormat12Groups encodes a format 12 subtable with explicit
// startChar/endChar/startGlyphID triples.
func buildFormat12Groups(declared uint32, groups [][3]uint32) []byte {
	sub := make([]byte, 16+12*len(groups))
	binary.BigEndian.PutUint16(sub[0:2], 12)
	binary.BigEndian.PutUint32(sub[4:8], uint32(len(sub)))
	binary.BigEndian.PutUint32(sub[12:16], declared)
	for i, g := range groups {
		off := 16 + i*12
		binary.BigEndian.PutUint32(sub[off:off+4], g[0])   // startCharCode
		binary.BigEndian.PutUint32(sub[off+4:off+8], g[1]) // endCharCode
		binary.BigEndian.PutUint32(sub[off+8:off+12], g[2])
	}
	return sub
}

// buildTruncatedFormat4 declares one segment but stops after the reservedPad
// word, before the startCode array, so the 16+8*segCount array guard rejects it.
func buildTruncatedFormat4() []byte {
	sub := make([]byte, 16) // < 16+8*segCount == 24 for one segment
	binary.BigEndian.PutUint16(sub[0:2], 4)
	binary.BigEndian.PutUint16(sub[6:8], 2) // segCountX2 -> segCount 1
	return sub
}

func writeTempFont(t *testing.T, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "font.ttf")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write temp font: %v", err)
	}
	return path
}

func TestParseTTFCmap(t *testing.T) {
	tests := []struct {
		name        string
		buildPath   func(t *testing.T) string
		wantErr     bool
		errContains string
		wantRunes   []rune
		absentRunes []rune
		wantEmpty   bool
	}{
		{
			name: "format 4 segment range",
			buildPath: func(t *testing.T) string {
				sub := buildFormat4([][2]uint16{{'A', 'C'}, {0xFFFF, 0xFFFF}})
				return writeTempFont(t, sfntWithTable("cmap", cmapWithSubtable(sub)))
			},
			wantRunes:   []rune{'A', 'B', 'C'},
			absentRunes: []rune{'D', '@', 0xFFFF},
		},
		{
			name: "format 4 idDelta drops code that maps to glyph 0",
			buildPath: func(t *testing.T) string {
				// idDelta 0xFFBF is -65: 'A'-65 = 0 (notdef, absent),
				// 'B' -> 1 and 'C' -> 2 (present).
				sub := buildFormat4Segments([]format4Segment{
					{start: 'A', end: 'C', idDelta: 0xFFBF},
					{start: 0xFFFF, end: 0xFFFF},
				}, nil)
				return writeTempFont(t, sfntWithTable("cmap", cmapWithSubtable(sub)))
			},
			wantRunes:   []rune{'B', 'C'},
			absentRunes: []rune{'A', 'D'},
		},
		{
			name: "format 4 idRangeOffset resolves glyphIdArray entries",
			buildPath: func(t *testing.T) string {
				// segCount 2: idRangeOffset array at 16+6*2 == 28, glyphIdArray
				// at 16+8*2 == 32, so 32-28 = 4 == 2*segCount points segment 0
				// at glyphIdArray.
				sub := buildFormat4Segments([]format4Segment{
					{start: 'A', end: 'C', idRangeOffset: 4},
					{start: 0xFFFF, end: 0xFFFF},
				}, []uint16{0, 5, 0})
				return writeTempFont(t, sfntWithTable("cmap", cmapWithSubtable(sub)))
			},
			wantRunes:   []rune{'B'},
			absentRunes: []rune{'A', 'C'},
		},
		{
			name: "format 4 idRangeOffset glyphs still receive idDelta",
			buildPath: func(t *testing.T) string {
				// Same 2*segCount idRangeOffset, glyphs 1,2 with idDelta 0xFFFF
				// (-1): 1-1 = 0 (absent), 2-1 = 1 (present).
				sub := buildFormat4Segments([]format4Segment{
					{start: 'A', end: 'B', idDelta: 0xFFFF, idRangeOffset: 4},
					{start: 0xFFFF, end: 0xFFFF},
				}, []uint16{1, 2})
				return writeTempFont(t, sfntWithTable("cmap", cmapWithSubtable(sub)))
			},
			wantRunes:   []rune{'B'},
			absentRunes: []rune{'A', 'C'},
		},
		{
			name: "format 4 out-of-bounds idRangeOffset skips only that segment",
			buildPath: func(t *testing.T) string {
				// Segment 0's idRangeOffset (0xFFF0) points far past the table;
				// segment 1 keeps idRangeOffset 0 and maps to itself.
				sub := buildFormat4Segments([]format4Segment{
					{start: 'A', end: 'C', idRangeOffset: 0xFFF0},
					{start: 'D', end: 'F'},
					{start: 0xFFFF, end: 0xFFFF},
				}, nil)
				return writeTempFont(t, sfntWithTable("cmap", cmapWithSubtable(sub)))
			},
			wantRunes:   []rune{'D', 'E', 'F'},
			absentRunes: []rune{'A', 'B', 'C'},
		},
		{
			name: "format 4 glyphIdArray truncated mid-segment stops scanning",
			buildPath: func(t *testing.T) string {
				// Only one glyph for a three-code segment: 'A' resolves, then
				// the read for 'B' runs past the table and scanning stops.
				sub := buildFormat4Segments([]format4Segment{
					{start: 'A', end: 'C', idRangeOffset: 4},
					{start: 0xFFFF, end: 0xFFFF},
				}, []uint16{7})
				return writeTempFont(t, sfntWithTable("cmap", cmapWithSubtable(sub)))
			},
			wantRunes:   []rune{'A'},
			absentRunes: []rune{'B', 'C'},
		},
		{
			name: "format 4 segment with start after end is skipped",
			buildPath: func(t *testing.T) string {
				sub := buildFormat4Segments([]format4Segment{
					{start: 'C', end: 'A'},
					{start: 'B', end: 'B'},
				}, nil)
				return writeTempFont(t, sfntWithTable("cmap", cmapWithSubtable(sub)))
			},
			wantRunes:   []rune{'B'},
			absentRunes: []rune{'A', 'C', 0xFFFF},
		},
		{
			name: "format 4 arrays truncated before idRangeOffset yield empty map",
			buildPath: func(t *testing.T) string {
				sub := buildFormat4Segments([]format4Segment{
					{start: 'A', end: 'C'},
					{start: 0xFFFF, end: 0xFFFF},
				}, nil)
				// Keep header + endCode + reservedPad + startCode only:
				// 16+4*segCount == 24 bytes, short of the 16+8*segCount guard.
				sub = sub[:16+4*2]
				return writeTempFont(t, sfntWithTable("cmap", cmapWithSubtable(sub)))
			},
			wantEmpty: true,
		},
		{
			name: "format 12 sequential group",
			buildPath: func(t *testing.T) string {
				sub := buildFormat12(1, [][2]uint32{{0x4E00, 0x4E03}})
				return writeTempFont(t, sfntWithTable("cmap", cmapWithSubtable(sub)))
			},
			wantRunes:   []rune{0x4E00, 0x4E01, 0x4E02, 0x4E03},
			absentRunes: []rune{0x4DFF, 0x4E04},
		},
		{
			name: "format 4 truncated segment arrays yields empty map",
			buildPath: func(t *testing.T) string {
				return writeTempFont(t, sfntWithTable("cmap", cmapWithSubtable(buildTruncatedFormat4())))
			},
			wantEmpty: true,
		},
		{
			name: "format 12 truncated group array keeps decoded groups",
			buildPath: func(t *testing.T) string {
				sub := buildFormat12(2, [][2]uint32{{0x4E00, 0x4E03}})
				return writeTempFont(t, sfntWithTable("cmap", cmapWithSubtable(sub)))
			},
			wantRunes:   []rune{0x4E00, 0x4E03},
			absentRunes: []rune{0x4E04},
		},
		{
			name: "format 12 group with start after end is skipped",
			buildPath: func(t *testing.T) string {
				sub := buildFormat12Groups(1, [][3]uint32{{0x50, 0x40, 0}})
				return writeTempFont(t, sfntWithTable("cmap", cmapWithSubtable(sub)))
			},
			wantEmpty: true,
		},
		{
			name: "format 12 group end clamps to max Unicode",
			buildPath: func(t *testing.T) string {
				sub := buildFormat12Groups(1, [][3]uint32{{0x10FFFE, 0xFFFFFFFF, 1}})
				return writeTempFont(t, sfntWithTable("cmap", cmapWithSubtable(sub)))
			},
			wantRunes:   []rune{0x10FFFE, 0x10FFFF},
			absentRunes: []rune{0x10FFFD},
		},
		{
			name: "unsupported subtable format yields empty map",
			buildPath: func(t *testing.T) string {
				sub := make([]byte, 16)
				binary.BigEndian.PutUint16(sub[0:2], 6) // trimmed table format
				return writeTempFont(t, sfntWithTable("cmap", cmapWithSubtable(sub)))
			},
			wantEmpty: true,
		},
		{
			name: "subtable offset equal to cmap length is skipped",
			buildPath: func(t *testing.T) string {
				cmap := make([]byte, 12) // header + one record, no payload
				binary.BigEndian.PutUint16(cmap[2:4], 1)
				binary.BigEndian.PutUint32(cmap[8:12], 12)
				return writeTempFont(t, sfntWithTable("cmap", cmap))
			},
			wantEmpty: true,
		},
		{
			name: "subtable offset leaves one trailing byte is skipped",
			buildPath: func(t *testing.T) string {
				cmap := make([]byte, 13) // offset 12 -> 1 byte, less than a format word
				binary.BigEndian.PutUint16(cmap[2:4], 1)
				binary.BigEndian.PutUint32(cmap[8:12], 12)
				return writeTempFont(t, sfntWithTable("cmap", cmap))
			},
			wantEmpty: true,
		},
		{
			name: "cmap header claims more subtables than present",
			buildPath: func(t *testing.T) string {
				cmap := make([]byte, 4)
				binary.BigEndian.PutUint16(cmap[2:4], 2) // no encoding records follow
				return writeTempFont(t, sfntWithTable("cmap", cmap))
			},
			wantEmpty: true,
		},
		{
			name: "unsupported table tag reports cmap table not found",
			buildPath: func(t *testing.T) string {
				return writeTempFont(t, sfntWithTable("head", cmapWithSubtable(buildFormat4([][2]uint16{{'A', 'C'}, {0xFFFF, 0xFFFF}}))))
			},
			wantErr:     true,
			errContains: "cmap table not found",
		},
		{
			name: "cmap table length beyond EOF is rejected",
			buildPath: func(t *testing.T) string {
				font := sfntWithTable("cmap", cmapWithSubtable(buildFormat4([][2]uint16{{'A', 'C'}, {0xFFFF, 0xFFFF}})))
				patchTableRecord(font, 28, uint32(len(font))) // offset 28 + length overruns EOF
				return writeTempFont(t, font)
			},
			wantErr:     true,
			errContains: "cmap table not found",
		},
		{
			name: "cmap table offset beyond EOF is rejected",
			buildPath: func(t *testing.T) string {
				font := sfntWithTable("cmap", cmapWithSubtable(buildFormat4([][2]uint16{{'A', 'C'}, {0xFFFF, 0xFFFF}})))
				patchTableRecord(font, 0xFFFF, 12)
				return writeTempFont(t, font)
			},
			wantErr:     true,
			errContains: "cmap table not found",
		},
		{
			name: "cmap table shorter than four bytes is rejected",
			buildPath: func(t *testing.T) string {
				return writeTempFont(t, sfntWithTable("cmap", []byte{0, 0}))
			},
			wantErr:     true,
			errContains: "invalid cmap",
		},
		{
			name: "table directory truncated before record end breaks the scan",
			buildPath: func(t *testing.T) string {
				font := make([]byte, 28)
				binary.BigEndian.PutUint32(font[0:4], 0x00010000)
				binary.BigEndian.PutUint16(font[4:6], 2) // only one record physically present
				copy(font[12:16], "name")
				return writeTempFont(t, font)
			},
			wantErr:     true,
			errContains: "cmap table not found",
		},
		{
			name: "file shorter than sfnt header is rejected",
			buildPath: func(t *testing.T) string {
				return writeTempFont(t, []byte("TTF"))
			},
			wantErr:     true,
			errContains: "file too small",
		},
		{
			name: "nonexistent path returns read error",
			buildPath: func(t *testing.T) string {
				return filepath.Join(t.TempDir(), "absent.ttf")
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := tc.buildPath(t)
			runes, err := parseTTFCmap(path)

			if tc.wantErr {
				if err == nil {
					t.Fatalf("parseTTFCmap(%q) error = nil, want error", path)
				}
				if runes != nil {
					t.Fatalf("parseTTFCmap(%q) runes = %v on error, want nil", path, len(runes))
				}
				if tc.errContains != "" && !strings.Contains(err.Error(), tc.errContains) {
					t.Fatalf("error = %q, want substring %q", err, tc.errContains)
				}
				return
			}

			if err != nil {
				t.Fatalf("parseTTFCmap(%q) unexpected error: %v", path, err)
			}
			if runes == nil {
				t.Fatalf("parseTTFCmap(%q) returned nil map without error", path)
			}
			if tc.wantEmpty && len(runes) != 0 {
				t.Fatalf("parseTTFCmap(%q) mapped %d runes, want empty map", path, len(runes))
			}
			for _, r := range tc.wantRunes {
				if !runes[r] {
					t.Errorf("rune %U missing from cmap", r)
				}
			}
			for _, r := range tc.absentRunes {
				if runes[r] {
					t.Errorf("rune %U unexpectedly present in cmap", r)
				}
			}
		})
	}
}

// TestParseTTFCmapOverflowingTableRecord covers a cmap table record whose
// offset+length wraps the uint32 sum to a value inside the file, which must
// be rejected before slicing (previously panicked).
func TestParseTTFCmapOverflowingTableRecord(t *testing.T) {
	font := sfntWithTable("cmap", cmapWithSubtable(buildFormat4([][2]uint16{{'A', 'C'}, {0xFFFF, 0xFFFF}})))
	patchTableRecord(font, 0xFFFFFFF0, 0x20) // 0xFFFFFFF0+0x20 wraps to 16
	path := writeTempFont(t, font)

	type result struct {
		runes  map[rune]bool
		err    error
		panics interface{}
	}
	res := func() (out result) {
		defer func() {
			out.panics = recover()
		}()
		out.runes, out.err = parseTTFCmap(path)
		return out
	}()

	if res.panics != nil {
		t.Fatalf("parseTTFCmap panicked on overflowing cmap table record: %v", res.panics)
	}
	if res.err == nil || !strings.Contains(res.err.Error(), "cmap table not found") {
		t.Fatalf("parseTTFCmap error = %v, want error containing %q", res.err, "cmap table not found")
	}
	if res.runes != nil {
		t.Fatalf("parseTTFCmap returned %d runes on error, want nil", len(res.runes))
	}
}

// TestParseTTFCmapFormat12WrappedGroup covers a format 12 group spanning
// 0xFFFFFFF0..0xFFFFFFFF, whose uint32 loop previously wrapped at 2^32 and
// never terminated. The group lies entirely outside Unicode and must be
// skipped without error.
func TestParseTTFCmapFormat12WrappedGroup(t *testing.T) {
	sub := buildFormat12Groups(1, [][3]uint32{{0xFFFFFFF0, 0xFFFFFFFF, 1}})
	path := writeTempFont(t, sfntWithTable("cmap", cmapWithSubtable(sub)))

	type result struct {
		runes map[rune]bool
		err   error
	}
	done := make(chan result, 1)
	go func() {
		runes, err := parseTTFCmap(path)
		done <- result{runes: runes, err: err}
	}()

	select {
	case res := <-done:
		if res.err != nil {
			t.Fatalf("parseTTFCmap unexpected error: %v", res.err)
		}
		if len(res.runes) != 0 {
			t.Fatalf("parseTTFCmap mapped %d runes, want empty map for a group beyond Unicode", len(res.runes))
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("parseTTFCmap did not return within 2s: format 12 group 0xFFFFFFF0..0xFFFFFFFF wrapped the uint32 loop")
	}
}
