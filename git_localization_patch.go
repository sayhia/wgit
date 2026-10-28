package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// =========================================================================
// 本地化缺失 key 片段生成
//
// AuditLocalizationParity 能列出「哪个 key 在哪个语言缺」，但用户拿到清单后
// 还得自己抄进语言文件。这里把清单变成可以直接粘贴的片段——但**不写文件**：
// 语言文件的组织方式各项目不同（嵌套 / 扁平 / 带注释），自动改容易毁掉格式。
// =========================================================================

type LocalizationKeyPatch struct {
	TargetLanguage string   `json:"targetLanguage"`
	Format         string   `json:"format"`
	MissingCount   int      `json:"missingCount"`
	Keys           []string `json:"keys"`
	Content        string   `json:"content"`
	SuggestedPath  string   `json:"suggestedPath"`
	Message        string   `json:"message"`
}

var localizationPatchFormats = map[string]bool{"json": true, "csv": true, "ts": true}

// renderKeyPatch builds the snippet. Values are always left empty: filling in
// the source language's text would produce a file that looks translated but
// is not, and that is worse than an obvious blank.
func renderKeyPatch(entries []LocalizedEntry, lang string, format string) (string, error) {
	sorted := make([]LocalizedEntry, 0, len(entries))
	sorted = append(sorted, entries...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Key < sorted[j].Key })

	var buf bytes.Buffer
	switch format {
	case "json":
		buf.WriteString("{\n")
		for i, entry := range sorted {
			keyJSON, _ := json.Marshal(entry.Key)
			if i > 0 {
				buf.WriteString(",\n")
			}
			buf.WriteString("  ")
			buf.Write(keyJSON)
			buf.WriteString(`: ""`)
		}
		buf.WriteString("\n}\n")

	case "csv":
		w := csv.NewWriter(&buf)
		if err := w.Write([]string{"key", "sourceText", "translation"}); err != nil {
			return "", err
		}
		for _, entry := range sorted {
			if err := w.Write([]string{entry.Key, entry.SourceText, ""}); err != nil {
				return "", err
			}
		}
		w.Flush()
		if err := w.Error(); err != nil {
			return "", err
		}

	case "ts":
		buf.WriteString("// 待翻译 key，来源语言文本见行尾注释\n")
		for _, entry := range sorted {
			keyJSON, _ := json.Marshal(entry.Key)
			buf.WriteString("  ")
			buf.Write(keyJSON)
			buf.WriteString(`: "",`)
			if comment := strings.TrimSpace(entry.SourceText); comment != "" {
				buf.WriteString(" // ")
				buf.WriteString(strings.ReplaceAll(comment, "\n", " "))
			}
			buf.WriteString("\n")
		}

	default:
		return "", fmt.Errorf("不支持的格式: %s", format)
	}
	return buf.String(), nil
}

// GenerateLocalizationKeyPatch lists the keys missing in targetLang and renders
// them as a paste-ready snippet. It never touches the repository.
//
// targetLang may be empty, in which case the first language that has gaps wins —
// asking the user to type an exact tag before seeing anything is a poor trade.
func (s GitService) GenerateLocalizationKeyPatch(repoPath, targetLang, format string) (*LocalizationKeyPatch, error) {
	if strings.TrimSpace(repoPath) == "" {
		return nil, errors.New("repository path is required")
	}
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "" {
		format = "json"
	}
	if !localizationPatchFormats[format] {
		return nil, fmt.Errorf("不支持的格式 %q，可选 json / csv / ts", format)
	}

	audit, err := s.AuditLocalizationParity(repoPath, "")
	if err != nil {
		return nil, err
	}

	target := strings.TrimSpace(targetLang)
	if target == "" {
		// First gap in the audit's own language order keeps the result stable.
		for _, lang := range audit.LanguagesFound {
			for _, entry := range audit.UntranslatedList {
				if containsString(entry.MissingLangs, lang) {
					target = lang
					break
				}
			}
			if target != "" {
				break
			}
		}
	}
	if target == "" {
		return &LocalizationKeyPatch{
			Format:  format,
			Message: "所有语言都没有缺失 key，无需生成补丁。",
		}, nil
	}

	missing := make([]LocalizedEntry, 0, len(audit.UntranslatedList))
	for _, entry := range audit.UntranslatedList {
		if !containsString(entry.MissingLangs, target) {
			continue
		}
		// The audit fills SourceText from its "primary" language, which it takes
		// from a map iteration — so it can land on the very language that is
		// missing. Fall back to any language that actually has text, preferring
		// en-US, so the snippet always shows the translator something to work from.
		if strings.TrimSpace(entry.SourceText) == "" {
			for _, candidate := range []string{"en-US", audit.PrimaryLanguage} {
				if text := strings.TrimSpace(entry.Translations[candidate]); text != "" {
					entry.SourceText = text
					break
				}
			}
		}
		missing = append(missing, entry)
	}
	if len(missing) == 0 {
		return &LocalizationKeyPatch{
			TargetLanguage: target,
			Format:         format,
			Message:        fmt.Sprintf("%s 没有缺失 key。", target),
		}, nil
	}

	content, err := renderKeyPatch(missing, target, format)
	if err != nil {
		return nil, err
	}

	keys := make([]string, 0, len(missing))
	for _, entry := range missing {
		keys = append(keys, entry.Key)
	}
	sort.Strings(keys)

	suggested := ""
	if len(audit.AuditedFiles) > 0 {
		// Point at a file that actually belongs to the target language; falling
		// back to the first audited file would suggest writing English keys into
		// a Chinese file.
		for _, file := range audit.AuditedFiles {
			lower := strings.ToLower(file)
			if strings.Contains(lower, strings.ToLower(target)) {
				suggested = file
				break
			}
		}
	}
	if suggested == "" {
		suggested = filepath.ToSlash(fmt.Sprintf("locales/%s.%s", target, format))
	}

	return &LocalizationKeyPatch{
		TargetLanguage: target,
		Format:         format,
		MissingCount:   len(missing),
		Keys:           keys,
		Content:        content,
		SuggestedPath:  suggested,
		Message: fmt.Sprintf(
			"%s 缺 %d 个 key，已生成 %s 片段。请手动粘贴到 %s —— 语言文件的组织方式各项目不同，自动写入容易破坏格式。",
			target, len(missing), format, suggested,
		),
	}, nil
}

func containsString(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}
