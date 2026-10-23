package main

import (
	"errors"
	"sort"
	"strconv"
	"strings"
)

// StashFileEntry describes one file touched by a stash entry.
type StashFileEntry struct {
	Path      string `json:"path"`
	Status    string `json:"status"` // A (added) M (modified) D (deleted) R (renamed)
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
}

// GetStashFiles lists the files a stash entry touches, with per-file line
// counts. It exists because the only other way to answer "what is in this
// stash" was to parse the unified diff in the UI, which silently breaks on
// binary files and on paths containing spaces.
//
// stashIndex is taken as a string to match ApplyFileFromStash, so both can be
// handed the same value straight from the stash list.
func (GitService) GetStashFiles(repoPath string, stashIndex string) ([]StashFileEntry, error) {
	if repoPath == "" {
		return nil, errors.New("repository path is required")
	}
	stashIndex = strings.TrimSpace(stashIndex)
	if stashIndex == "" {
		return nil, errors.New("stash index is required")
	}
	ref := "stash@{" + stashIndex + "}"

	// --numstat is the only source of line counts, but it prints a bare
	// "-\t-" for binaries, so the --name-status pass below decides what
	// actually appears in the list.
	counts := map[string][2]int{}
	if raw, err := ExecGitRaw(repoPath, "stash", "show", "--numstat", "--find-renames", ref); err == nil {
		for _, line := range strings.Split(raw, "\n") {
			line = strings.TrimRight(line, "\r")
			if strings.TrimSpace(line) == "" {
				continue
			}
			f := strings.Split(line, "\t")
			if len(f) < 3 {
				continue
			}
			add, _ := strconv.Atoi(strings.TrimSpace(f[0]))
			del, _ := strconv.Atoi(strings.TrimSpace(f[1]))
			counts[newPathFromNumstat(f[2])] = [2]int{add, del}
		}
	}

	statusRaw, err := ExecGitRaw(repoPath, "stash", "show", "--name-status", "--find-renames", "-z", ref)
	if err != nil {
		return nil, err
	}

	out := []StashFileEntry{}
	for _, rec := range parseNameStatusZ(statusRaw) {
		c := counts[rec.Path]
		out = append(out, StashFileEntry{
			Path:      rec.Path,
			Status:    rec.Status,
			Additions: c[0],
			Deletions: c[1],
		})
	}

	// `git stash show` only reports changes to tracked files. Stashes created
	// with -u keep their untracked files in a third parent commit, which show
	// never reaches — so those files would be invisible and unrestorable.
	if _, err := ExecGit(repoPath, "rev-parse", "--verify", "--quiet", ref+"^3"); err == nil {
		raw, err := ExecGitRaw(repoPath, "ls-tree", "-r", "--name-only", "-z", ref+"^3")
		if err == nil {
			for _, p := range strings.Split(raw, "\x00") {
				p = strings.TrimSpace(p)
				if p == "" {
					continue
				}
				c := counts[p]
				out = append(out, StashFileEntry{
					Path:      p,
					Status:    "A",
					Additions: c[0],
					Deletions: c[1],
				})
			}
		}
	}

	sort.SliceStable(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

// nameStatusRecord is one -z separated name-status pair.
type nameStatusRecord struct {
	Status string
	Path   string
}

// parseNameStatusZ decodes `git ... --name-status -z` output, whose records are
// NUL separated fields: status, path, and — for R/C — an extra source path.
//
// Verified against git: a normal entry is "M\0path\0" and a rename is
// "R100\0old\0new\0". Note that -z terminates both the status and the path
// with NUL, so a plain Split(raw, "\x00") yields status and path as separate
// fields rather than a single "M\tpath" column.
func parseNameStatusZ(raw string) []nameStatusRecord {
	raw = strings.TrimRight(raw, "\x00")
	if raw == "" {
		return nil
	}
	fields := strings.Split(raw, "\x00")

	recs := []nameStatusRecord{}
	for i := 0; i < len(fields); {
		status := strings.TrimSpace(fields[i])
		i++
		if status == "" {
			continue
		}
		code := status[:1]

		if code == "R" || code == "C" {
			// Skip the source path; the destination is what a user restores.
			i++
			if i >= len(fields) {
				break
			}
			recs = append(recs, nameStatusRecord{Status: code, Path: fields[i]})
			i++
			continue
		}

		if i >= len(fields) {
			break
		}
		recs = append(recs, nameStatusRecord{Status: code, Path: fields[i]})
		i++
	}
	return recs
}

// newPathFromNumstat maps a --numstat path column to the destination path,
// undoing both the "old => new" form and the brace shorthand git emits when
// the two paths share a prefix ("dir/{a => b}.txt").
func newPathFromNumstat(p string) string {
	p = strings.TrimSpace(p)

	// Brace shorthand: dir/{old => new}.txt -> dir/new.txt
	if open := strings.Index(p, "{"); open >= 0 {
		if close := strings.Index(p[open:], "}"); close > 0 {
			close += open
			inner := p[open+1 : close]
			if arrow := strings.Index(inner, "=>"); arrow >= 0 {
				return p[:open] + strings.TrimSpace(inner[arrow+2:]) + p[close+1:]
			}
		}
	}

	// Plain rename form: old => new
	if arrow := strings.Index(p, " => "); arrow >= 0 {
		return strings.TrimSpace(p[arrow+4:])
	}
	return p
}
