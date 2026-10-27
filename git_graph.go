package main

import (
	"strings"
)

var LaneColors = []string{
	"#3b82f6", // Blue
	"#10b981", // Emerald
	"#8b5cf6", // Purple
	"#f59e0b", // Amber
	"#ec4899", // Pink
	"#06b6d4", // Cyan
	"#f97316", // Orange
	"#6366f1", // Indigo
}

func getLaneColor(lane int) string {
	if lane < 0 {
		lane = 0
	}
	return LaneColors[lane%len(LaneColors)]
}

// BuildCommitGraph processes a raw list of parsed commits and assigns lanes and connections.
func BuildCommitGraph(commits []CommitItem) []CommitItem {
	if len(commits) == 0 {
		return commits
	}

	// activeLanes keeps track of which commit hash is expected next in each lane column
	var activeLanes []string
	hashToLane := make(map[string]int, 32)

	for i := range commits {
		commit := &commits[i]
		hash := commit.Hash

		// Find if this commit is already expected in an existing lane
		laneIndex, exists := hashToLane[hash]
		if !exists || laneIndex >= len(activeLanes) || activeLanes[laneIndex] != hash {
			// Fallback search or find first empty slot
			laneIndex = -1
			for l, expectedHash := range activeLanes {
				if expectedHash == hash {
					laneIndex = l
					break
				}
			}
			if laneIndex == -1 {
				for l, expectedHash := range activeLanes {
					if expectedHash == "" {
						laneIndex = l
						break
					}
				}
				if laneIndex == -1 {
					laneIndex = len(activeLanes)
					activeLanes = append(activeLanes, "")
				}
			}
		}

		delete(hashToLane, hash)
		commit.Lane = laneIndex
		var lines []GraphLine

		// Process parents
		if len(commit.Parents) > 0 {
			firstParent := commit.Parents[0]
			activeLanes[laneIndex] = firstParent
			hashToLane[firstParent] = laneIndex

			// Line continuing down on same lane
			lines = append(lines, GraphLine{
				FromLane: laneIndex,
				ToLane:   laneIndex,
				Color:    getLaneColor(laneIndex),
			})

			// Additional parents (merges)
			for pIdx := 1; pIdx < len(commit.Parents); pIdx++ {
				mergeParent := commit.Parents[pIdx]
				mParentLane, mExists := hashToLane[mergeParent]
				if !mExists || mParentLane >= len(activeLanes) || activeLanes[mParentLane] != mergeParent {
					mParentLane = -1
					for l, expectedHash := range activeLanes {
						if expectedHash == mergeParent {
							mParentLane = l
							break
						}
					}
					if mParentLane == -1 {
						for l, expectedHash := range activeLanes {
							if expectedHash == "" {
								mParentLane = l
								break
							}
						}
						if mParentLane == -1 {
							mParentLane = len(activeLanes)
							activeLanes = append(activeLanes, "")
						}
					}
					activeLanes[mParentLane] = mergeParent
					hashToLane[mergeParent] = mParentLane
				}

				lines = append(lines, GraphLine{
					FromLane: laneIndex,
					ToLane:   mParentLane,
					Color:    getLaneColor(mParentLane),
				})
			}
		} else {
			// Root commit (no parents)
			activeLanes[laneIndex] = ""
		}

		// Also pass through any other active lanes that are continuing past this commit
		for l, expectedHash := range activeLanes {
			if l != laneIndex && expectedHash != "" {
				lines = append(lines, GraphLine{
					FromLane: l,
					ToLane:   l,
					Color:    getLaneColor(l),
				})
			}
		}

		commit.Lines = lines

		// Trim trailing empty lanes in activeLanes to avoid unbounded growth
		for len(activeLanes) > 0 && activeLanes[len(activeLanes)-1] == "" {
			activeLanes = activeLanes[:len(activeLanes)-1]
		}
	}

	return commits
}

// ParseRawGitLog parses output from `git log --pretty=format:...`
func ParseRawGitLog(raw string) []CommitItem {
	if strings.TrimSpace(raw) == "" {
		return []CommitItem{}
	}

	// We delimit commits with a unique token "\x1e" (Record Separator)
	// and fields with "\x1f" (Unit Separator)
	records := strings.Split(raw, "\x1e")
	items := make([]CommitItem, 0, len(records))

	for _, rec := range records {
		rec = strings.TrimSpace(rec)
		if rec == "" {
			continue
		}
		fields := strings.Split(rec, "\x1f")
		if len(fields) < 7 {
			continue
		}

		hash := strings.TrimSpace(fields[0])
		shortHash := strings.TrimSpace(fields[1])
		parentsStr := strings.TrimSpace(fields[2])
		author := strings.TrimSpace(fields[3])
		email := strings.TrimSpace(fields[4])
		date := strings.TrimSpace(fields[5])
		relDate := strings.TrimSpace(fields[6])
		subject := ""
		body := ""
		refsStr := ""
		sigStatus := ""
		signer := ""

		if len(fields) > 7 {
			subject = strings.TrimSpace(fields[7])
		}
		if len(fields) > 8 {
			body = strings.TrimSpace(fields[8])
		}
		if len(fields) > 9 {
			refsStr = strings.TrimSpace(fields[9])
		}
		if len(fields) > 10 {
			sigStatus = strings.TrimSpace(fields[10])
		}
		if len(fields) > 11 {
			signer = strings.TrimSpace(fields[11])
		}

		var parents []string
		if parentsStr != "" {
			for _, p := range strings.Split(parentsStr, " ") {
				p = strings.TrimSpace(p)
				if p != "" {
					parents = append(parents, p)
				}
			}
		}

		var refs []string
		isHead := false
		if refsStr != "" {
			// e.g. "HEAD -> main, origin/main, tag: v1.0.0"
			for _, ref := range strings.Split(refsStr, ",") {
				ref = strings.TrimSpace(ref)
				if ref != "" {
					refs = append(refs, ref)
					if strings.Contains(ref, "HEAD") {
						isHead = true
					}
				}
			}
		}

		items = append(items, CommitItem{
			Hash:            hash,
			ShortHash:       shortHash,
			Parents:         parents,
			AuthorName:      author,
			AuthorEmail:     email,
			Date:            date,
			RelativeDate:    relDate,
			Subject:         subject,
			Body:            body,
			Refs:            refs,
			IsHead:          isHead,
			SignatureStatus: sigStatus,
			Signer:          signer,
		})
	}

	return BuildCommitGraph(items)
}
