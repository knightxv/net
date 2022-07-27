package broadcast_test

import (
	"fmt"
	"io"
	"os"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/components"
	"github.com/go-echarts/go-echarts/v2/opts"
	"github.com/go-echarts/go-echarts/v2/types"

	"github.com/guabee/bnrtc/util"
)

// GetHex Converts a decimal number to hex representations
func getHex(num int) string {
	hex := fmt.Sprintf("%x", num)
	if len(hex) == 1 {
		hex = "0" + hex
	}
	return hex
}

func GetColorHex(r, g, b int) string {
	return "#" + getHex(r) + getHex(g) + getHex(b)
}

type BroadcastNode struct {
	// Name of data item.
	Name string `json:"name,omitempty"`

	// x value of node position.
	X float32 `json:"x,omitempty"`

	// y value of node position.
	Y float32 `json:"y,omitempty"`

	// Value of data item.
	Value float32 `json:"value,omitempty"`

	// If node are fixed when doing force directed layout.
	Fixed bool `json:"fixed,omitempty"`

	// Index of category which the data item belongs to.
	Category int `json:"category,omitempty"`

	// Symbol of node of this category.
	// Icon types provided by ECharts includes
	// 'circle', 'rect', 'roundRect', 'triangle', 'diamond', 'pin', 'arrow', 'none'
	// It can be set to an image with 'image://url' , in which URL is the link to an image, or dataURI of an image.
	Symbol string `json:"symbol,omitempty"`

	// node of this category symbol size. It can be set to single numbers like 10,
	// or use an array to represent width and height. For example, [20, 10] means symbol width is 20, and height is10.
	SymbolSize interface{} `json:"symbolSize,omitempty"`

	// The style of this node.
	ItemStyle *opts.ItemStyle `json:"itemStyle,omitempty"`
}

type BroadcastLink struct {
	Source interface{} `json:"source,omitempty"`

	// A string representing the name of target node on edge. Can also be a number representing node index.
	Target interface{} `json:"target,omitempty"`

	// value of edge, can be mapped to edge length in force graph.
	Value float32 `json:"value,omitempty"`

	LineStyle  *opts.LineStyle `json:"lineStyle,omitempty"`
	Symbol     []string        `json:"symbol,omitempty"`
	SymbolSize interface{}     `json:"symbolSize,omitempty"`
}

type BroadcastGraph struct {
	graph *charts.Graph
	nodes *util.SafeMap //   map[string]*opts.GraphNode
	links *util.SafeMap // map[string]*opts.GraphLink
}

func newBroadcastGraph() *BroadcastGraph {
	return &BroadcastGraph{
		graph: charts.NewGraph().SetGlobalOptions(
			charts.WithTitleOpts(opts.Title{Title: "broadcast graph"}),
		),
		nodes: util.NewSafeMap(), // (map[string]*opts.GraphNode),
		links: util.NewSafeMap(), // make(map[string]*opts.GraphLink),
	}
}

func (g *BroadcastGraph) AddNode(key string) {
	if g.nodes.Size() == 0 {
		g.nodes.Set(key, &BroadcastNode{Name: key, Symbol: "pin", SymbolSize: 20, ItemStyle: &opts.ItemStyle{Color: GetColorHex(0, 255, 0)}})
	} else {
		if !g.nodes.Has(key) {
			g.nodes.Set(key, &BroadcastNode{Name: key, SymbolSize: 10})
		}
	}
}

func (g BroadcastGraph) AddPair(srcKey, dstKey string, isSend bool) {
	g.AddNode(srcKey)
	g.AddNode(dstKey)

	key := srcKey + "-" + dstKey
	if !isSend {
		g.links.Set(key, &BroadcastLink{
			Source: srcKey,
			Target: dstKey,
			LineStyle: &opts.LineStyle{
				Color: GetColorHex(255, 0, 0),
				Width: 1,
				Type:  "solid",
			},
			Symbol: []string{"none", "arrow"},
		})
	} else {
		if !g.links.Has(key) {
			g.links.Set(key, &BroadcastLink{
				Source: srcKey,
				Target: dstKey,
				LineStyle: &opts.LineStyle{
					Color: GetColorHex(0, 0, 0),
					Width: 0.2,
					Type:  "dash",
				},
				Symbol: []string{"none", "arrow"},
			})
		}
	}
}

func (g *BroadcastGraph) Reset() {
	g.nodes.Clear()
	g.links.Clear()
	g.graph.MultiSeries = nil
}

func (g *BroadcastGraph) AddSeries(name string, nodes []BroadcastNode, links []BroadcastLink, options ...charts.SeriesOpts) {
	series := charts.SingleSeries{
		Name:  name,
		Type:  types.ChartGraph,
		Roam:  true,
		Links: links,
		Data:  nodes,
	}
	for _, opt := range options {
		opt(&series)
	}
	g.graph.MultiSeries = append(g.graph.MultiSeries, series)
}

func (g *BroadcastGraph) Render(path string) {
	nodes := make([]BroadcastNode, g.nodes.Size())
	i := 0
	for _, _n := range g.nodes.Items() {
		nodes[i] = *(_n.(*BroadcastNode))
		i++
	}

	links := make([]BroadcastLink, g.links.Size())
	j := 0
	for _, _l := range g.links.Items() {
		links[j] = *(_l.(*BroadcastLink))
		j++
	}

	g.AddSeries("graph", nodes, links, charts.WithGraphChartOpts(
		opts.GraphChart{
			Force:              &opts.GraphForce{Repulsion: 8000},
			Layout:             "circular",
			FocusNodeAdjacency: true,
		}))

	page := components.NewPage()
	page.AddCharts(
		g.graph,
	)

	f, err := os.Create(path)
	if err != nil {
		panic(err)

	}
	_ = page.Render(io.MultiWriter(f))
}
