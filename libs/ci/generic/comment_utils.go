package generic

import (
	"flag"
	"fmt"
	"github.com/google/shlex"
	"strconv"
	"strings"
)

type CommentParts struct {
	Projects    []string
	Layer       int
	Directories []string
}

type multiFlag []string

func (m *multiFlag) String() string {
	return strings.Join(*m, ",")
}

func (m *multiFlag) Set(value string) error {
	*m = append(*m, value)
	return nil
}

// singleUseInt rejects multiple occurrences of the same flag.
type singleUseInt struct {
	val  int
	seen bool
	name string // for better error messages
}

func (s *singleUseInt) String() string { return strconv.Itoa(s.val) }
func (s *singleUseInt) Set(v string) error {
	if s.seen {
		return fmt.Errorf("--%s specified multiple times", s.name)
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return fmt.Errorf("invalid value for --%s: %w", s.name, err)
	}
	s.val = i
	s.seen = true
	return nil
}

// ParseDiggerCommentFlags parse the flags in the comment such as -p and -d and --layer
// validates that the right number of flags are specified
// Does not validate the "digger plan" part of the command that is left to a prior function
func ParseDiggerCommentFlags(comment string) (*CommentParts, bool, error) {
	comment = strings.TrimSpace(strings.ToLower(comment))

	args, err := shlex.Split(comment)
	if err != nil {
		return nil, false, fmt.Errorf("failed to split input %v", err)

	}

	if len(args) < 2 {
		return nil, false, fmt.Errorf("incorrect operation specified: (%v) err: %v", comment, err)
	}

	fs := flag.NewFlagSet("digger", flag.ContinueOnError)

	var projects multiFlag
	var directories multiFlag
	layer := &singleUseInt{val: -1, name: "layer"}

	fs.Var(&projects, "p", "project (short form)")
	fs.Var(&projects, "project", "project (long form)")

	fs.Var(&directories, "d", "directory (short form)")
	fs.Var(&directories, "directory", "directory (long form)")

	fs.Var(layer, "layer", "layer to plan or apply")

	err = fs.Parse(args[2:])
	if err != nil {
		return nil, false, fmt.Errorf("failed to parse input %v", err)
	}

	// ❗ Disallow mixing --layer with -p or -d
	if layer.val != -1 && (len(projects) > 0 || len(directories) > 0) {
		return nil, false, fmt.Errorf("cannot mix --layer with -p or -d")
	}

	return &CommentParts{
		Projects:    projects,
		Layer:       layer.val,
		Directories: directories,
	}, true, nil
}
