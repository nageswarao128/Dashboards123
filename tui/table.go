package tui

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/mattn/go-runewidth"
	"github.com/nbedos/cistern/utils"
)

type nodeID interface{}

type TableNode interface {
	// Unique identifier of this node among its siblings
	NodeID() interface{}
	NodeChildren() []TableNode
	Values(loc *time.Location) map[ColumnID]StyledString
	InheritedValues() []ColumnID
}

func (n *innerTableNode) setPrefix(parent string, isLastChild bool) {
	if parent == "" {
		switch {
		case len(n.children) == 0:
			n.prefix = " "
		case n.traversable:
			n.prefix = "-"
		default:
			n.prefix = "+"
		}
		for i, child := range n.children {
			child.setPrefix(" ", i == len(n.children)-1)
		}
	} else {
		n.prefix = parent
		if isLastChild {
			n.prefix += "└─"
		} else {
			n.prefix += "├─"
		}

		if len(n.children) == 0 || n.traversable {
			n.prefix += "─ "
		} else {
			n.prefix += "+ "
		}

		for i, child := range n.children {
			if childIsLastChild := i == len(n.children)-1; isLastChild {
				child.setPrefix(parent+"    ", childIsLastChild)
			} else {
				child.setPrefix(parent+"│   ", childIsLastChild)
			}
		}
	}
}

type Column struct {
	Header     string
	Order      int
	MaxWidth   int
	Alignment  Alignment
	TreePrefix bool
}

type ColumnID int

type ColumnConfiguration map[ColumnID]*Column

func (c ColumnConfiguration) columnIDs() []ColumnID {
	ids := make([]ColumnID, 0, len(c))
	for id := range c {
		ids = append(ids, id)
	}

	sort.Slice(ids, func(i, j int) bool {
		return c[ids[i]].Order < c[ids[j]].Order
	})

	return ids
}

type nullInt struct {
	Valid bool
	Int   int
}

func (i nullInt) Diff(other nullInt) string {
	return cmp.Diff(i, other, cmp.AllowUnexported(nullInt{}))
}

const maxTreeDepth = 10

type nodePath struct {
	ids [maxTreeDepth]nodeID
	len int
}

func nodePathFromIDs(ids ...nodeID) nodePath {
	return nodePath{}.append(ids...)
}

func (p nodePath) append(ids ...nodeID) nodePath {
	for _, id := range ids {
		if p.len >= len(p.ids) {
			panic(fmt.Sprintf("path length cannot exceed %d", len(p.ids)))
		}

		p.ids[p.len] = id
		p.len++
	}
	return p
}

type innerTableNode struct {
	path        nodePath
	prefix      string
	traversable bool
	values      map[ColumnID]StyledString
	children    []*innerTableNode
}

func (n innerTableNode) depthFirstTraversal(traverseAll bool) []*innerTableNode {
	explored := make([]*innerTableNode, 0)
	toBeExplored := []*innerTableNode{&n}

	for len(toBeExplored) > 0 {
		node := toBeExplored[len(toBeExplored)-1]
		toBeExplored = toBeExplored[:len(toBeExplored)-1]
		if traverseAll || node.traversable {
			for i := len(node.children) - 1; i >= 0; i-- {
				toBeExplored = append(toBeExplored, node.children[i])
			}
		}
		explored = append(explored, node)
	}

	return explored
}

func toInnerTableNode(n TableNode, parent innerTableNode, traversable map[nodePath]bool, loc *time.Location) innerTableNode {
	path := parent.path.append(n.NodeID())
	s := innerTableNode{
		path:        path,
		values:      n.Values(loc),
		traversable: traversable[path],
	}

	for _, c := range n.InheritedValues() {
		s.values[c] = parent.values[c]
	}

	for _, child := range n.NodeChildren() {
		innerNode := toInnerTableNode(child, s, traversable, loc)
		s.children = append(s.children, &innerNode)
	}

	return s
}

func (n *innerTableNode) Map(f func(n *innerTableNode)) {
	f(n)

	for _, child := range n.children {
		child.Map(f)
	}
}

func (t *HierarchicalTable) lookup(path nodePath) *innerTableNode {
	children := make([]*innerTableNode, 0, len(t.nodes))
	for i := range t.nodes {
		children = append(children, &t.nodes[i])
	}

pathLoop:
	for i := 0; i < path.len; i++ {
		for _, c := range children {
			if c.path.ids[i] == path.ids[i] {
				if c.path == path {
					return c
				}
				children = c.children
				continue pathLoop
			}
		}
		return nil
	}

	return nil
}

type HierarchicalTable struct {
	// List of the top-level nodes
	nodes []innerTableNode
	// Depth first traversal of all the top-level nodes. Needs updating if `nodes` or `traversable` changes
	rows []*innerTableNode
	// Index in `rows` of the first node of the current page
	pageIndex nullInt
	// Index in `rows` of the node where the cursor is located
	cursorIndex nullInt
	height      int
	width       int
	sep         string
	location    *time.Location
	conf        ColumnConfiguration
	columnWidth map[ColumnID]int
}

func NewHierarchicalTable(conf ColumnConfiguration, nodes []TableNode, width int, height int, loc *time.Location) (HierarchicalTable, error) {
	if width < 0 || height < 0 {
		return HierarchicalTable{}, errors.New("table width and height must be >= 0")
	}

	table := HierarchicalTable{
		height:      height,
		width:       width,
		sep:         "  ", // FIXME Move this out of here
		location:    loc,
		conf:        conf,
		columnWidth: make(map[ColumnID]int),
	}

	for id, column := range conf {
		table.columnWidth[id] = utils.MaxInt(table.columnWidth[id], runewidth.StringWidth(column.Header))
	}

	table.Replace(nodes)

	return table, nil
}

func (t HierarchicalTable) depthFirstTraversal(traverseAll bool) []*innerTableNode {
	explored := make([]*innerTableNode, 0)
	for _, n := range t.nodes {
		explored = append(explored, n.depthFirstTraversal(traverseAll)...)
	}

	return explored
}

// Number of rows visible on screen
func (t HierarchicalTable) PageSize() int {
	return utils.MaxInt(0, t.height-1)
}

func (t *HierarchicalTable) computeTraversal() {
	// Save current paths of page and cursor
	var pageNodePath nodePath
	if t.pageIndex.Valid {
		pageNodePath = t.rows[t.pageIndex.Int].path
	}

	var cursorNodePath nodePath
	if t.cursorIndex.Valid {
		cursorNodePath = t.rows[t.cursorIndex.Int].path
	}

	// Update node prefixes
	for i := range t.nodes {
		t.nodes[i].setPrefix("", false)
	}

	// Reset page and cursor indexes
	t.pageIndex = nullInt{}
	t.cursorIndex = nullInt{}

	t.rows = t.depthFirstTraversal(false)

	// Adjust value of pageIndex and cursorIndex
	for i, row := range t.rows {
		if row.path == pageNodePath {
			t.pageIndex = nullInt{
				Valid: true,
				Int:   i,
			}
		}
		if row.path == cursorNodePath {
			t.cursorIndex = nullInt{
				Valid: true,
				Int:   i,
			}
		}
	}

	// If no match was found, default to the first row
	if len(t.rows) > 0 {
		if !t.pageIndex.Valid {
			t.pageIndex = nullInt{
				Valid: true,
				Int:   0,
			}
		}
		if !t.cursorIndex.Valid {
			t.cursorIndex = nullInt{
				Valid: true,
				Int:   0,
			}
		}
	}

	for _, row := range t.rows {
		for _, id := range t.conf.columnIDs() {
			w := row.values[id].Length()
			if t.conf[id].TreePrefix {
				w += runewidth.StringWidth(row.prefix)
			}
			t.columnWidth[id] = utils.MaxInt(t.columnWidth[id], w)
		}
	}
}

func (t *HierarchicalTable) Replace(nodes []TableNode) {
	// Save traversable state
	traversable := make(map[nodePath]bool, 0)
	for _, node := range t.depthFirstTraversal(true) {
		traversable[node.path] = node.traversable
	}

	// Copy node hierarchy and compute the path of each node along the way
	t.nodes = make([]innerTableNode, 0, len(nodes))
	for _, n := range nodes {
		t.nodes = append(t.nodes, toInnerTableNode(n, innerTableNode{}, traversable, t.location))
	}

	t.computeTraversal()
}

func (t *HierarchicalTable) SetTraversable(traversable bool, recursive bool) {
	if t.cursorIndex.Valid {
		if n := t.lookup(t.rows[t.cursorIndex.Int].path); n != nil {
			if recursive {
				n.Map(func(node *innerTableNode) {
					node.traversable = traversable
				})
			} else {
				n.traversable = traversable
			}
		}
		t.computeTraversal()
	}
}

func (t *HierarchicalTable) Scroll(amount int) {
	if !t.cursorIndex.Valid || !t.pageIndex.Valid {
		return
	}

	t.cursorIndex.Int = utils.Bounded(t.cursorIndex.Int+amount, 0, len(t.rows)-1)

	switch {
	case t.cursorIndex.Int < t.pageIndex.Int:
		// Scroll up
		t.pageIndex.Int = t.cursorIndex.Int
	case t.cursorIndex.Int > t.pageIndex.Int+t.PageSize()-1:
		// Scroll down
		scrollAmount := t.cursorIndex.Int - (t.pageIndex.Int + t.PageSize() - 1)
		t.pageIndex.Int = utils.Bounded(t.pageIndex.Int+scrollAmount, 0, len(t.rows)-1)
		t.cursorIndex.Int = t.pageIndex.Int + t.PageSize() - 1
	}
}

func (t *HierarchicalTable) Top() {
	t.Scroll(-len(t.rows))
}

func (t *HierarchicalTable) Bottom() {
	t.Scroll(len(t.rows))
}

func (t *HierarchicalTable) ScrollToMatch(s string, ascending bool) bool {
	if !t.cursorIndex.Valid {
		return false
	}

	step := 1
	if !ascending {
		step = -1
	}

	start := utils.Modulo(t.cursorIndex.Int+step, len(t.rows))
	next := func(i int) int {
		return utils.Modulo(i+step, len(t.rows))
	}
	for i := start; i != t.cursorIndex.Int; i = next(i) {
		for id := range t.conf {
			if t.rows[i].values[id].Contains(s) {
				t.Scroll(i - t.cursorIndex.Int)
				return true
			}
		}
	}

	return false
}

func (t HierarchicalTable) header() StyledString {
	values := make(map[ColumnID]StyledString)
	for id, column := range t.conf {
		values[id] = NewStyledString(column.Header)
	}

	s := t.styledString(values, "")
	s.Add(TableHeader)

	return s
}

func (t HierarchicalTable) styledString(values map[ColumnID]StyledString, prefix string) StyledString {
	paddedColumns := make([]StyledString, 0)
	for _, id := range t.conf.columnIDs() {
		alignment := t.conf[id].Alignment
		v := values[id]
		if t.conf[id].TreePrefix {
			prefixedValue := NewStyledString(prefix)
			prefixedValue.AppendString(v)
			v = prefixedValue
		}
		w := utils.MinInt(t.columnWidth[id], t.conf[id].MaxWidth)
		v.Fit(alignment, w)
		paddedColumns = append(paddedColumns, v)
	}
	line := Join(paddedColumns, NewStyledString(t.sep))
	line.Fit(Left, t.width)

	return line
}

func (t HierarchicalTable) Size() (int, int) {
	return t.width, t.height
}

func (t *HierarchicalTable) Resize(width int, height int) {
	t.width = utils.MaxInt(0, width)
	t.height = utils.MaxInt(0, height)

	if t.PageSize() > 0 {
		if t.cursorIndex.Valid && t.pageIndex.Valid {
			upperBound := utils.Bounded(t.pageIndex.Int+t.PageSize()-1, 0, len(t.rows)-1)
			t.cursorIndex.Int = utils.Bounded(t.cursorIndex.Int, t.pageIndex.Int, upperBound)
		} else if len(t.rows) > 0 {
			t.pageIndex = nullInt{
				Valid: true,
				Int:   0,
			}
			t.cursorIndex = t.pageIndex
		}
	} else {
		t.cursorIndex = nullInt{}
		t.pageIndex = nullInt{}
	}
}

func (t *HierarchicalTable) Text() []LocalizedStyledString {
	texts := make([]LocalizedStyledString, 0)

	y := 0
	if t.height > 0 {
		texts = append(texts, LocalizedStyledString{
			X: 0,
			Y: y,
			S: t.header(),
		})
		y++
	}

	if t.pageIndex.Valid && t.cursorIndex.Valid {
		for i, row := range t.rows[t.pageIndex.Int:utils.MinInt(t.pageIndex.Int+t.PageSize(), len(t.rows))] {
			s := LocalizedStyledString{
				X: 0,
				Y: y,
				S: t.styledString(row.values, row.prefix),
			}
			y++

			if t.cursorIndex.Int == i+t.pageIndex.Int {
				s.S.Add(ActiveRow)
			}

			texts = append(texts, s)
		}
	}

	return texts
}

func (t *HierarchicalTable) ActiveNodePath() []interface{} {
	if !t.cursorIndex.Valid {
		return nil
	}

	path := t.rows[t.cursorIndex.Int].path
	slicedPath := make([]interface{}, 0)
	for _, id := range path.ids[:path.len] {
		slicedPath = append(slicedPath, id)
	}

	return slicedPath
}
