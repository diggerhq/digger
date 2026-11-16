package reporting

// Comment splitting for VCS providers that enforce a maximum comment body
// size (e.g. GitHub's 65,536 character limit).
//
// A report larger than the limit is broken into a chain of comments. Each
// chunk closes whatever markdown structure is open at its boundary (code
// fence, <details> block) and the next chunk reopens it, so every comment
// in the chain renders correctly on its own.

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// defaultCommentMaxSize is used when the CI service does not implement
// ci.CommentMaxLengthProvider. It matches GitHub's documented limit of
// 65,536 characters. Go len() counts bytes (>= characters), so byte-based
// checks are conservative and safe.
const defaultCommentMaxSize = 65536

// maxCommentsPerReport caps how many comments a single report may be split
// into. Beyond the cap the head of the output is truncated, preserving the
// tail which holds the plan summary, warnings and errors.
const maxCommentsPerReport = 10

// sepStartSizeSlack is extra room assumed for the start separator when
// guessing where a chunk will begin. The guess only needs to land inside the
// chunk for closure detection; the exact boundary is settled later.
const sepStartSizeSlack = 50

// prevCommentUrlPlaceholder is embedded in continuation separators and
// replaced with the previous chunk's comment URL at publish time (the URL
// only exists once the previous chunk has been posted).
const prevCommentUrlPlaceholder = "%PREVIOUS_COMMENT_URL%"

// prevCommentUrlReserve is subtracted from the chunk size limit to leave
// room for the URL being longer than the placeholder after substitution.
const prevCommentUrlReserve = 256

// fillPreviousCommentUrl substitutes the previous comment's URL into a
// chunk's continuation header. With no URL available (VCS clients that do
// not return one), the markdown link degrades to plain text.
func fillPreviousCommentUrl(chunk string, url string) string {
	if url == "" {
		return strings.ReplaceAll(chunk, "[previous comment]("+prevCommentUrlPlaceholder+")", "previous comment")
	}
	return strings.ReplaceAll(chunk, prevCommentUrlPlaceholder, url)
}

// reserveUrlRoom shrinks maxSize by prevCommentUrlReserve so that URL
// substitution cannot push a published chunk over the VCS limit.
func reserveUrlRoom(maxSize int) int {
	if maxSize > 2*prevCommentUrlReserve {
		return maxSize - prevCommentUrlReserve
	}
	return maxSize
}

// closureType represents the type of markdown closure at a given position
type closureType int

const (
	// noClosure means no special closure is needed
	noClosure closureType = iota
	// inCodeBlock means we're inside a code block (```)
	inCodeBlock
	// inDetailsBlock means we're inside a details block (<details>)
	inDetailsBlock
	// inCodeInDetails means we're inside a code block within a details block
	inCodeInDetails
	// inInlineCode means we're inside an inline code span (`)
	inInlineCode
)

// lineTokenRegex matches, within a single line, backtick runs and details
// tags (with optional attributes)
var lineTokenRegex = regexp.MustCompile("(`+)|(<details(?:\\s[^>]*)?>)|(</details>)")

// separatorSet contains the separators for a specific closure type
type separatorSet struct {
	sepEnd           string
	sepStart         string
	truncationHeader string
}

// generateSeparatorsFunc allows the separator generation to be overridden in tests
var generateSeparatorsFunc = generateSeparators

// generateSeparators creates separator sets for different closure types.
// fence is the marker of the code fence open at the split point (e.g. "```",
// "````" or "~~~"): closing it must repeat the same character at least as
// many times, and the reopened fence uses the same marker so that the
// original closing fence later in the content still terminates it.
func generateSeparators(fence string) map[closureType]separatorSet {
	separators := make(map[closureType]separatorSet)

	baseEnd := "\n<br>\n\n**Note:** the output exceeds the maximum comment size. Continued in next comment."
	baseStart := "Continued from [previous comment](" + prevCommentUrlPlaceholder + ").\n"
	baseTruncation := "> [!WARNING]\n> The output was too large to fit in the allowed number of comments. Its beginning was dropped; the end, including the summary, is preserved below. See the run logs for the full output.\n"

	closeFence := "\n" + fence + "\n"
	openFence := fence + "terraform\n"
	openDetails := "<details><summary>Continued output</summary>\n\n"

	separators[noClosure] = separatorSet{
		sepEnd:           baseEnd,
		sepStart:         baseStart,
		truncationHeader: baseTruncation,
	}

	separators[inCodeBlock] = separatorSet{
		sepEnd:           closeFence + baseEnd,
		sepStart:         baseStart + openFence,
		truncationHeader: baseTruncation + openFence,
	}

	separators[inDetailsBlock] = separatorSet{
		sepEnd:           "\n</details>\n" + baseEnd,
		sepStart:         baseStart + openDetails,
		truncationHeader: baseTruncation + openDetails,
	}

	separators[inCodeInDetails] = separatorSet{
		sepEnd:           closeFence + "</details>\n" + baseEnd,
		sepStart:         baseStart + openDetails + openFence,
		truncationHeader: baseTruncation + openDetails + openFence,
	}

	separators[inInlineCode] = separatorSet{
		sepEnd:           "`\n" + baseEnd,
		sepStart:         baseStart + "`",
		truncationHeader: baseTruncation + "`",
	}

	return separators
}

// runLen returns the length of the run of byte c at the start of s.
func runLen(s string, c byte) int {
	i := 0
	for i < len(s) && s[i] == c {
		i++
	}
	return i
}

// detectClosureType reports which markdown structures are open at position,
// along with the marker of the open code fence ("```" when none is open).
//
// It follows the fence rules the VCS renderers actually apply: a fence only
// opens or closes at the start of a line (at most 3 leading spaces), a
// backtick fence's info string cannot contain further backticks, and a
// closing fence must repeat the opening character at least as many times
// with nothing else on the line. Both backtick and tilde fences count.
// Inline backticks toggle a state local to their line, so a stray unmatched
// backtick does not leak into the rest of the comment.
func detectClosureType(comment string, position int) (closureType, string) {
	text := comment[:position]

	var fenceChar byte
	fenceLen := 0
	detailsDepth := 0
	lineInline := false

	for _, line := range strings.Split(text, "\n") {
		lineInline = false
		trimmed := strings.TrimLeft(line, " ")
		indent := len(line) - len(trimmed)

		if fenceLen > 0 {
			// inside a fence only a matching closing fence line matters;
			// everything else, including shorter runs, mid-line markers and
			// fences of the other character, is content
			if indent <= 3 {
				if n := runLen(trimmed, fenceChar); n >= fenceLen && strings.TrimSpace(trimmed[n:]) == "" {
					fenceChar, fenceLen = 0, 0
				}
			}
			continue
		}

		if indent <= 3 {
			if n := runLen(trimmed, '`'); n >= 3 && !strings.Contains(trimmed[n:], "`") {
				fenceChar, fenceLen = '`', n
				continue
			}
			if n := runLen(trimmed, '~'); n >= 3 {
				fenceChar, fenceLen = '~', n
				continue
			}
		}

		for _, loc := range lineTokenRegex.FindAllStringIndex(line, -1) {
			token := line[loc[0]:loc[1]]
			if token[0] == '`' {
				lineInline = !lineInline
			} else if strings.HasPrefix(token, "<details") {
				if !lineInline {
					detailsDepth++
				}
			} else if token == "</details>" {
				// prevent the depth from going negative on stray closing tags
				if !lineInline && detailsDepth > 0 {
					detailsDepth--
				}
			}
		}
	}

	fence := "```"
	if fenceLen > 0 {
		fence = strings.Repeat(string(fenceChar), fenceLen)
	}

	switch {
	case detailsDepth > 0 && fenceLen > 0:
		return inCodeInDetails, fence
	case fenceLen > 0:
		return inCodeBlock, fence
	case detailsDepth > 0:
		return inDetailsBlock, fence
	case lineInline:
		return inInlineCode, fence
	}
	return noClosure, fence
}

// SplitComment breaks comment into a slice of chunks, each at most maxSize
// bytes. Chunks that continue in a following comment end with a sepEnd for
// the markdown structure open at the boundary, and chunks that continue a
// preceding comment start with the matching sepStart, so each chunk renders
// as valid markdown on its own.
//
// Chunks are carved from the end of the string towards the beginning: the
// tail of a terraform plan holds the summary, warnings and errors, so it is
// the part that must never be lost. When maxComments > 0 caps the number of
// chunks, whatever does not fit is dropped from the beginning and the first
// surviving chunk is prefixed with a truncation notice.
func SplitComment(comment string, maxSize int, maxComments int) []string {
	if len(comment) <= maxSize {
		return []string{comment}
	}

	var comments []string
	upTo := len(comment)

	for upTo > 0 {
		// once the cap leaves room for only one more chunk, that chunk
		// becomes the (truncated) head of the chain
		isLastAllowed := maxComments > 0 && len(comments) == maxComments-1

		// the chunk needs a sepEnd only when a later chunk already exists
		closureAtEnd, fenceAtEnd := detectClosureType(comment, upTo)
		endSepSet := generateSeparatorsFunc(fenceAtEnd)[closureAtEnd]
		endSepLength := 0
		if len(comments) > 0 {
			endSepLength = len(endSepSet.sepEnd)
		}

		// the sepStart depends on the closure state at the chunk start,
		// which is not known until the start is chosen: probe an estimated
		// position first, settle the exact boundary below
		estimatedDownFrom := min(upTo, max(0, upTo-(maxSize-endSepLength-sepStartSizeSlack)))
		closureAtStart, fenceAtStart := detectClosureType(comment, estimatedDownFrom)
		startSepSet := generateSeparatorsFunc(fenceAtStart)[closureAtStart]

		startSepLength := len(startSepSet.sepStart)
		if isLastAllowed {
			startSepLength = len(startSepSet.truncationHeader)
		}

		maxContentSize := maxSize - endSepLength - startSepLength
		if maxContentSize <= 0 {
			// maxSize cannot even hold the separators: give up on splitting
			return []string{comment}
		}

		downFrom := max(0, upTo-maxContentSize)

		// when the remaining head fits into this chunk without a sepStart,
		// take all of it (impossible when this chunk must truncate)
		if !isLastAllowed && downFrom > 0 {
			if upTo <= (maxSize - endSepLength) {
				downFrom = 0
			}
		}

		// move the boundary to the next newline (or space) when one is
		// near, so chunks break between lines rather than mid-word; moving
		// forward only shrinks this chunk, which is always safe
		if downFrom > 0 && downFrom < len(comment) {
			// bound the scan: far-away whitespace is not worth the walk
			// through a very large comment
			const whitespaceForwardSearchWindow = 500

			searchLimit := min(upTo-1, downFrom+whitespaceForwardSearchWindow)
			foundSpace := -1
			foundNewline := -1

			for p := downFrom; p < searchLimit; p++ {
				if comment[p] == '\n' {
					foundNewline = p
					break
				}
				if comment[p] == ' ' && foundSpace == -1 {
					foundSpace = p
				}
			}

			if foundNewline != -1 {
				downFrom = foundNewline + 1
			} else if foundSpace != -1 {
				downFrom = foundSpace + 1
			}
		}

		// the boundary moved since the probe, so the closure state (and with
		// it the separator size) may have changed: re-check and shrink until
		// content plus separators fit
		for {
			closureAtStart, fenceAtStart = detectClosureType(comment, downFrom)
			startSepSet = generateSeparatorsFunc(fenceAtStart)[closureAtStart]

			actualStartSepLength := 0
			if downFrom > 0 {
				if isLastAllowed {
					actualStartSepLength = len(startSepSet.truncationHeader)
				} else {
					actualStartSepLength = len(startSepSet.sepStart)
				}
			}

			currentTotalSize := (upTo - downFrom) + actualStartSepLength + endSepLength
			if currentTotalSize <= maxSize {
				break
			}

			downFrom = min(upTo, downFrom+(currentTotalSize-maxSize))
		}

		// Never split in the middle of a multi-byte rune: advancing shrinks the
		// chunk, so the size bound still holds.
		for downFrom > 0 && downFrom < len(comment) && !utf8.RuneStart(comment[downFrom]) {
			downFrom++
		}

		portion := comment[downFrom:upTo]

		if downFrom > 0 {
			if isLastAllowed {
				portion = startSepSet.truncationHeader + portion
			} else {
				portion = startSepSet.sepStart + portion
			}
		}

		if len(comments) > 0 {
			portion += endSepSet.sepEnd
		}

		comments = append([]string{portion}, comments...)
		upTo = downFrom

		// cap reached with content still remaining: the truncation notice on
		// comments[0] accounts for the dropped head, stop here
		if isLastAllowed && downFrom > 0 {
			break
		}
	}

	return comments
}
