package main

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"
)

func buildTestFont(family, copyright, license string) []byte {
	type nameRecord struct {
		id   uint16
		text string
	}
	records := []nameRecord{{0, copyright}, {1, family}, {13, license}}
	stringData := []byte{}
	offsets := make([]uint16, len(records))
	lengths := make([]uint16, len(records))
	for i, record := range records {
		offsets[i] = uint16(len(stringData))
		units := utf16.Encode([]rune(record.text))
		lengths[i] = uint16(len(units) * 2)
		for _, unit := range units {
			stringData = binary.BigEndian.AppendUint16(stringData, unit)
		}
	}

	nameTable := []byte{}
	appendU16 := func(v uint16) { nameTable = binary.BigEndian.AppendUint16(nameTable, v) }
	appendU16(0)
	appendU16(uint16(len(records)))
	appendU16(uint16(6 + 12*len(records)))
	for i, record := range records {
		appendU16(3)
		appendU16(1)
		appendU16(0x409)
		appendU16(record.id)
		appendU16(lengths[i])
		appendU16(offsets[i])
	}
	nameTable = append(nameTable, stringData...)

	tableOffset := 12 + 16
	data := make([]byte, tableOffset+len(nameTable))
	binary.BigEndian.PutUint32(data[0:4], 0x00010000)
	binary.BigEndian.PutUint16(data[4:6], 1)
	copy(data[12:16], "name")
	binary.BigEndian.PutUint32(data[20:24], uint32(tableOffset))
	binary.BigEndian.PutUint32(data[24:28], uint32(len(nameTable)))
	copy(data[tableOffset:], nameTable)
	return data
}

func TestAuditFontLicensesReadsFontNameTable(t *testing.T) {
	repo := t.TempDir()
	fontDir := filepath.Join(repo, "Assets", "Fonts")
	if err := os.MkdirAll(fontDir, 0o755); err != nil {
		t.Fatal(err)
	}
	fonts := map[string][]byte{
		"Safe.ttf":       buildTestFont("Studio Sans", "Copyright 2026", "SIL Open Font License 1.1"),
		"Restricted.ttf": buildTestFont("Trial Sans", "Copyright 2026", "Not for commercial use"),
		"Unknown.ttf":    buildTestFont("Mystery Sans", "Copyright 2026", "Proprietary"),
	}
	for name, data := range fonts {
		if err := os.WriteFile(filepath.Join(fontDir, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	res, err := (GitService{}).AuditFontLicenses(repo)
	if err != nil {
		t.Fatalf("AuditFontLicenses failed: %v", err)
	}
	if res.TotalFonts != 3 || res.CommercialSafeCount != 1 || res.RestrictedFontsCount != 1 || res.UnknownLicenseCount != 1 {
		t.Fatalf("unexpected license counts: %+v", res)
	}
	if res.LegalRiskScore != "HIGH" {
		t.Fatalf("restricted font should raise risk: %+v", res)
	}
	byPath := map[string]FontLicenseItem{}
	for _, font := range res.Fonts {
		byPath[font.FontPath] = font
	}
	safe := byPath["Assets/Fonts/Safe.ttf"]
	if safe.FamilyName != "Studio Sans" || safe.LicenseType != "Commercial_Safe" || !safe.IsCommercialSafe || !strings.Contains(safe.CopyrightNotice, "2026") {
		t.Fatalf("safe font metadata wrong: %+v", safe)
	}
	if restricted := byPath["Assets/Fonts/Restricted.ttf"]; restricted.LicenseType != "Restricted" || restricted.IsCommercialSafe {
		t.Fatalf("restricted font metadata wrong: %+v", restricted)
	}

	empty, err := (GitService{}).AuditFontLicenses(t.TempDir())
	if err != nil || empty.TotalFonts != 0 || empty.LegalRiskScore != "LOW" || !strings.Contains(empty.Summary, "未发现字体") {
		t.Fatalf("empty repo font audit wrong: %+v err=%v", empty, err)
	}
}
