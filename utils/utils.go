package utils

import (
	"database/sql"
	"fmt"
	"github.com/mattn/go-runewidth"
	"gopkg.in/src-d/go-git.v4"
	"net/url"
	"regexp"
	"strings"
	"time"
)

func MaxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func MinInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func Bounded(a, lower, upper int) int {
	return MaxInt(lower, MinInt(a, upper))
}

type TreeNode interface {
	Children() []TreeNode
	Traversable() bool
	SetPrefix(prefix string) // meh. This forces us to pass a pointer to DepthFirstTraversal which is read only.
}

func DepthFirstTraversal(node TreeNode, traverseAll bool) []TreeNode {
	explored := make([]TreeNode, 0)
	toBeExplored := []TreeNode{node}

	for len(toBeExplored) > 0 {
		node = toBeExplored[len(toBeExplored)-1]
		toBeExplored = toBeExplored[:len(toBeExplored)-1]
		if traverseAll || node.Traversable() {
			children := node.Children()
			for i := len(children) - 1; i >= 0; i-- {
				toBeExplored = append(toBeExplored, children[i])
			}
		}

		explored = append(explored, node)
	}

	return explored
}

func DepthFirstTraversalPrefixing(node TreeNode) {
	depthFirstTraversalPrefixing(node, "", true)
}

func depthFirstTraversalPrefixing(node TreeNode, indent string, last bool) {
	var prefix string
	// Special behavior for the root node which is prefixed by "+" if its children are hidden
	if indent == "" {
		if len(node.Children()) == 0 || node.Traversable() {
			prefix = "-"
		} else {
			prefix = "+"
		}
	} else {
		if last {
			prefix = "└─"
		} else {
			prefix = "├─"
		}

		if len(node.Children()) == 0 || node.Traversable() {
			prefix += "─ "
		} else {
			prefix += "+ "
		}
	}

	node.SetPrefix(indent + prefix)

	if node.Traversable() {
		children := node.Children()
		for i := range children {
			var childIndent string
			if last {
				childIndent = " "
			} else {
				childIndent = "│"
			}

			paddingLength := runewidth.StringWidth(prefix) - runewidth.StringWidth(childIndent)
			childIndent += strings.Repeat(" ", paddingLength)

			depthFirstTraversalPrefixing(children[i], indent+childIndent, i == len(children)-1)
		}
	}
}

func Coalesce(times ...sql.NullTime) sql.NullTime {
	for _, t := range times {
		if t.Valid {
			return t
		}
	}

	return sql.NullTime{}
}

func RepositorySlugFromURL(repositoryURL string) (string, error) {
	// Turn "git@host:path.git" into "host/path" so that it is compatible with url.Parse()
	if strings.HasPrefix(repositoryURL, "git@") {
		repositoryURL = strings.TrimPrefix(repositoryURL, "git@")
		repositoryURL = strings.TrimSuffix(repositoryURL, ".git")
		repositoryURL = strings.Replace(repositoryURL, ":", "/", 1)
	}

	u, err := url.Parse(repositoryURL)
	if err != nil {
		return "", err
	}

	components := strings.Split(u.Path, "/")
	if len(components) < 3 {
		return "", fmt.Errorf("invalid repository path: '%s' (expected at least three components)",
			u.Path)
	}

	return strings.Join(components[1:3], "/"), nil
}

func Prefix(s string, prefix string) string {
	builder := strings.Builder{}
	for _, line := range strings.Split(s, "\n") {
		builder.WriteString(fmt.Sprintf("%s%s\n", prefix, line))
	}

	return builder.String()
}

func NullTimeFrom(t *time.Time) sql.NullTime {
	if t == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{
		Time:  *t,
		Valid: true,
	}
}

func NullTimeFromString(s string) (t sql.NullTime, err error) {
	if s != "" {
		t.Time, err = time.Parse(time.RFC3339, s)
		t.Valid = err == nil
	}

	return
}

var deleteEraseInLine = regexp.MustCompile(".*\x1b\\[0K")
var deleteUntilCarriageReturn = regexp.MustCompile(`.*\r([^\r\n])`)

// Is this specific to Travis?
// FIXME Does not work for streaming
func PostProcess(log string) string {
	tmp := deleteEraseInLine.ReplaceAllString(log, "")
	return deleteUntilCarriageReturn.ReplaceAllString(tmp, "$1")
}

func GitOriginURL(path string) (string, error) {
	r, err := git.PlainOpenWithOptions(path, &git.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return "", err
	}

	remote, err := r.Remote("origin")
	if err != nil {
		return "", err
	}

	if len(remote.Config().URLs) == 0 {
		return "", fmt.Errorf("GIT repository '%s': remote 'origin' has not associated URL", path)
	}

	return remote.Config().URLs[0], nil
}