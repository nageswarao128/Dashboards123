package utils

import (
	"fmt"
	"strings"
	"testing"
)

type TestNode struct {
	value       string
	prefix      string
	traversable bool
	children    []TestNode
}

func (n TestNode) String() string {
	return n.value
}

func (n TestNode) Children() []TreeNode {
	children := make([]TreeNode, len(n.children))
	for i := range n.children {
		children[i] = &n.children[i]
	}
	return children
}

func (n TestNode) Traversable() bool {
	return n.traversable
}

func (n *TestNode) SetTraversable(open bool, recursive bool) {
	n.traversable = open
	if recursive {
		for _, node := range n.children {
			node.SetTraversable(open, recursive)
		}
	}
}

func TestDepthFirstTraversal(t *testing.T) {
	node := TestNode{
		value:       "root",
		traversable: true,
		children: []TestNode{
			{
				value:       "a",
				traversable: true,
				children: []TestNode{
					{
						value:       "aa",
						traversable: false,
						children:    nil,
					},
					{
						value:       "ab",
						traversable: false,
						children:    nil,
					},
				},
			},
			{
				value:       "b",
				traversable: false,
				children: []TestNode{
					{
						value:       "ba",
						traversable: false,
						children:    nil,
					},
				},
			},
			{
				value:       "c",
				traversable: true,
				children: []TestNode{
					{
						value:       "ca",
						traversable: false,
						children:    nil,
					},
					{
						value:       "cb",
						traversable: false,
						children:    nil,
					},
					{
						value:       "cc",
						traversable: false,
						children:    nil,
					},
				},
			},
		},
	}

	expectedvalues := []string{"root", "a", "aa", "ab", "b", "c", "ca", "cb", "cc"}
	for i, n := range DepthFirstTraversal(&node, false) {
		if fmt.Sprintf("%s", n) != expectedvalues[i] {
			t.Fatalf("unexpected node: %q", n)
		}
	}
}

func TestGitOriginURL(t *testing.T) {
	u, _, err := GitOriginURL(".")
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(u, "nbedos/citop") {
		t.Fatalf("expected url to contain 'nbedos/citop' but got %q", u)
	}
}

func TestRepositorySlugFromURL(t *testing.T) {
	urls := []string{
		// SSH git URL
		"git@github.com:nbedos/citop.git",
		"git@github.com:nbedos/citop",
		// HTTPS git URL
		"https://github.com/nbedos/citop.git",
		// Host shouldn't matter
		"git@gitlab.com:nbedos/citop.git",
		// Web URLs should work too
		"https://gitlab.com/nbedos/citop",
		// Extraneous path components should be ignored
		"https://gitlab.com/nbedos/citop/tree/master/cache",
		// URL scheme shouldn't matter
		"http://gitlab.com/nbedos/citop",
		"gitlab.com/nbedos/citop",
	}
	expected := "nbedos/citop"

	for _, u := range urls {
		t.Run(fmt.Sprintf("URL: %v", u), func(t *testing.T) {
			slug, err := RepositorySlugFromURL(u)
			if err != nil {
				t.Fatal(err)
			}
			if slug != expected {
				t.Fatalf("expected %q but got %q for URL %q", expected, slug, u)
			}
		})
	}

	urls = []string{
		// Missing 1 path component
		"git@github.com:nbedos.git",
		// Invalid URL (colon)
		"https://github.com:nbedos/citop.git",
	}

	for _, u := range urls {
		t.Run(fmt.Sprintf("URL: %v", u), func(t *testing.T) {
			if _, err := RepositorySlugFromURL(u); err == nil {
				t.Fatalf("expected error but got nil for URL %q", u)
			}
		})
	}
}
