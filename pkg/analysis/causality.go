package analysis

import (
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/operator-replay-debugger/internal/assert"
	"github.com/operator-replay-debugger/pkg/storage"
)

const (
	maxCausalityNodes  = 20000
	maxCausalityEdges  = 50000
	defaultMaxChains   = 20
	defaultMaxDepth    = 6
	maxCausalityChains = 100
)

// CausalityNodeType defines node kinds in the graph.
type CausalityNodeType string

const (
	NodeTypeOperation CausalityNodeType = "op"
	NodeTypeSpan      CausalityNodeType = "span"
)

// CausalityEdgeType defines edge kinds in the graph.
type CausalityEdgeType string

const (
	EdgeTypeOpToSpan CausalityEdgeType = "op_to_span"
	EdgeTypeSpanToOp CausalityEdgeType = "span_to_op"
)

// CausalityNode represents a node in the causality graph.
type CausalityNode struct {
	ID           string            `json:"id"`
	Type         CausalityNodeType `json:"type"`
	ActorID      string            `json:"actor_id,omitempty"`
	Kind         string            `json:"kind,omitempty"`
	Namespace    string            `json:"ns,omitempty"`
	Name         string            `json:"name,omitempty"`
	Timestamp    time.Time         `json:"ts,omitempty"`
	ResourceVer  string            `json:"rv,omitempty"`
	UID          string            `json:"uid,omitempty"`
	DurationMs   int64             `json:"duration_ms,omitempty"`
	Error        string            `json:"error,omitempty"`
	ResourceData string            `json:"resource_data,omitempty"`
}

// CausalityEdge represents a directed edge in the graph.
type CausalityEdge struct {
	From string            `json:"from"`
	To   string            `json:"to"`
	Type CausalityEdgeType `json:"type"`
}

// CausalityGraph represents the causal relationship graph.
type CausalityGraph struct {
	Nodes []CausalityNode `json:"nodes"`
	Edges []CausalityEdge `json:"edges"`
}

// CausalityOptions controls graph construction.
type CausalityOptions struct {
	IncludePayloads bool
}

// CausalityChain represents a causal chain for text output.
type CausalityChain struct {
	NodeIDs []string
	Length  int
	FanOut  int
}

type opWithIndex struct {
	op    storage.Operation
	index int
}

type rvOp struct {
	op    storage.Operation
	index int
	rv    int64
}

type writeIndexes struct {
	writeOps      []opWithIndex
	writesByActor map[string][]opWithIndex
	exactByKey    map[string][]opWithIndex
	rvByUID       map[string][]rvOp
}

type causalityBuilder struct {
	opts      CausalityOptions
	nodes     map[string]CausalityNode
	edges     []CausalityEdge
	edgeIndex map[string]bool
}

// BuildCausalityGraph builds a causality graph from operations and spans.
func BuildCausalityGraph(
	ops []storage.Operation,
	spans []storage.ReconcileSpan,
	opts CausalityOptions,
) (*CausalityGraph, []string, error) {
	err := assert.AssertInRange(len(ops), 0, maxAnalysisOperations, "operation count")
	if err != nil {
		return nil, nil, err
	}

	err = assert.AssertInRange(len(spans), 0, maxAnalysisOperations, "span count")
	if err != nil {
		return nil, nil, err
	}

	warnings := make([]string, 0, 5)
	if len(spans) == 0 {
		warnings = append(warnings, "No reconcile spans found; causality requires spans.")
	}

	indexes, idxWarnings, err := collectWriteIndexes(ops)
	if err != nil {
		return nil, nil, err
	}
	warnings = append(warnings, idxWarnings...)

	sortRVIndexes(indexes.rvByUID)

	builder := newCausalityBuilder(opts)
	buildSpanEdges(builder, spans, indexes)

	graph := builder.graph()
	if len(graph.Edges) == 0 {
		warnings = append(warnings, "No causality edges found; data may be incomplete.")
	}

	return graph, warnings, nil
}

func collectWriteIndexes(ops []storage.Operation) (*writeIndexes, []string, error) {
	err := assert.AssertInRange(len(ops), 0, maxAnalysisOperations, "operation count")
	if err != nil {
		return nil, nil, err
	}
	err = assert.AssertInRange(len(ops), 0, maxAnalysisOperations, "operation count limit")
	if err != nil {
		return nil, nil, err
	}

	indexes := &writeIndexes{
		writeOps:      make([]opWithIndex, 0, len(ops)),
		writesByActor: make(map[string][]opWithIndex, 50),
		exactByKey:    make(map[string][]opWithIndex, 200),
		rvByUID:       make(map[string][]rvOp, 200),
	}
	populateWriteIndexes(indexes, ops)
	return indexes, indexWarnings(indexes), nil
}

func populateWriteIndexes(indexes *writeIndexes, ops []storage.Operation) {
	if indexes == nil {
		return
	}

	maxOps := len(ops)
	if maxOps > maxAnalysisOperations {
		maxOps = maxAnalysisOperations
	}

	for i := 0; i < maxOps; i++ {
		op := ops[i]
		if !isWriteOperation(op.OperationType) {
			continue
		}

		entry := opWithIndex{op: op, index: i}
		indexes.writeOps = append(indexes.writeOps, entry)

		if len(op.ActorID) > 0 {
			indexes.writesByActor[op.ActorID] = append(indexes.writesByActor[op.ActorID], entry)
		}

		if len(op.UID) > 0 && len(op.ResourceVersion) > 0 {
			key := fmt.Sprintf("%s|%s", op.UID, op.ResourceVersion)
			indexes.exactByKey[key] = append(indexes.exactByKey[key], entry)

			rvValue, parseErr := strconv.ParseInt(op.ResourceVersion, 10, 64)
			if parseErr == nil {
				indexes.rvByUID[op.UID] = append(indexes.rvByUID[op.UID], rvOp{
					op:    op,
					index: i,
					rv:    rvValue,
				})
			}
		}
	}
}

func indexWarnings(indexes *writeIndexes) []string {
	warnings := make([]string, 0, 2)
	if indexes == nil {
		return warnings
	}

	if len(indexes.writeOps) == 0 {
		warnings = append(warnings, "No write operations found; causality links require writes.")
	}
	if len(indexes.exactByKey) == 0 {
		warnings = append(warnings, "Operations missing uid/resource_version; write-to-reconcile linking limited.")
	}

	return warnings
}

func sortRVIndexes(rvByUID map[string][]rvOp) {
	if rvByUID == nil {
		return
	}

	keys := make([]string, 0, len(rvByUID))
	count := 0
	maxKeys := len(rvByUID)
	if maxKeys > maxAnalysisOperations {
		maxKeys = maxAnalysisOperations
	}
	for uid := range rvByUID {
		if count >= maxKeys {
			break
		}
		keys = append(keys, uid)
		count = count + 1
	}

	for i := 0; i < maxKeys; i++ {
		uid := keys[i]
		sort.Slice(rvByUID[uid], func(i, j int) bool {
			if rvByUID[uid][i].rv == rvByUID[uid][j].rv {
				return rvByUID[uid][i].op.Timestamp.Before(rvByUID[uid][j].op.Timestamp)
			}
			return rvByUID[uid][i].rv < rvByUID[uid][j].rv
		})
	}
}

func newCausalityBuilder(opts CausalityOptions) *causalityBuilder {
	return &causalityBuilder{
		opts:      opts,
		nodes:     make(map[string]CausalityNode, 1000),
		edges:     make([]CausalityEdge, 0, 2000),
		edgeIndex: make(map[string]bool, 2000),
	}
}

func buildSpanEdges(builder *causalityBuilder, spans []storage.ReconcileSpan, indexes *writeIndexes) {
	if builder == nil || indexes == nil {
		return
	}

	maxSpans := len(spans)
	if maxSpans > maxAnalysisOperations {
		maxSpans = maxAnalysisOperations
	}

	for i := 0; i < maxSpans; i++ {
		span := spans[i]
		if len(span.TriggerUID) > 0 && len(span.TriggerResourceVersion) > 0 {
			if match := findExactMatch(indexes.exactByKey, span); match != nil {
				opID := builder.ensureOpNode(match.op, match.index)
				spanID := builder.ensureSpanNode(span)
				builder.addEdge(opID, spanID, EdgeTypeOpToSpan)
			} else if fallback := findFallbackMatch(indexes.rvByUID, span); fallback != nil {
				opID := builder.ensureOpNode(fallback.op, fallback.index)
				spanID := builder.ensureSpanNode(span)
				builder.addEdge(opID, spanID, EdgeTypeOpToSpan)
			}
		}

		if span.EndTime.IsZero() || span.EndTime.Before(span.StartTime) {
			continue
		}

		actorOps := indexes.writesByActor[span.ActorID]
		maxActorOps := len(actorOps)
		if maxActorOps > maxAnalysisOperations {
			maxActorOps = maxAnalysisOperations
		}
		for j := 0; j < maxActorOps; j++ {
			opEntry := actorOps[j]
			op := opEntry.op
			if op.Timestamp.Before(span.StartTime) || op.Timestamp.After(span.EndTime) {
				continue
			}
			opID := builder.ensureOpNode(op, opEntry.index)
			spanID := builder.ensureSpanNode(span)
			builder.addEdge(spanID, opID, EdgeTypeSpanToOp)
		}
	}
}

func (b *causalityBuilder) addEdge(fromID, toID string, edgeType CausalityEdgeType) {
	if b == nil {
		return
	}

	key := fmt.Sprintf("%s|%s|%s", fromID, toID, edgeType)
	if b.edgeIndex[key] {
		return
	}
	if len(b.edges) >= maxCausalityEdges {
		return
	}

	b.edges = append(b.edges, CausalityEdge{
		From: fromID,
		To:   toID,
		Type: edgeType,
	})
	b.edgeIndex[key] = true
}

func (b *causalityBuilder) ensureOpNode(op storage.Operation, index int) string {
	id := opNodeID(op, index)
	if b == nil {
		return id
	}
	if _, ok := b.nodes[id]; ok {
		return id
	}
	if len(b.nodes) >= maxCausalityNodes {
		return id
	}

	node := CausalityNode{
		ID:          id,
		Type:        NodeTypeOperation,
		ActorID:     op.ActorID,
		Kind:        op.ResourceKind,
		Namespace:   op.Namespace,
		Name:        op.Name,
		Timestamp:   op.Timestamp,
		ResourceVer: op.ResourceVersion,
		UID:         op.UID,
		DurationMs:  op.DurationMs,
		Error:       op.Error,
	}
	if b.opts.IncludePayloads {
		node.ResourceData = op.ResourceData
	}

	b.nodes[id] = node
	return id
}

func (b *causalityBuilder) ensureSpanNode(span storage.ReconcileSpan) string {
	id := spanNodeID(span)
	if b == nil {
		return id
	}
	if _, ok := b.nodes[id]; ok {
		return id
	}
	if len(b.nodes) >= maxCausalityNodes {
		return id
	}

	node := CausalityNode{
		ID:          id,
		Type:        NodeTypeSpan,
		ActorID:     span.ActorID,
		Kind:        span.Kind,
		Namespace:   span.Namespace,
		Name:        span.Name,
		Timestamp:   span.StartTime,
		ResourceVer: span.TriggerResourceVersion,
		UID:         span.TriggerUID,
		DurationMs:  span.DurationMs,
		Error:       span.Error,
	}
	b.nodes[id] = node
	return id
}

func (b *causalityBuilder) graph() *CausalityGraph {
	if b == nil {
		return &CausalityGraph{Nodes: []CausalityNode{}, Edges: []CausalityEdge{}}
	}

	graph := &CausalityGraph{
		Nodes: make([]CausalityNode, 0, len(b.nodes)),
		Edges: b.edges,
	}

	for _, node := range b.nodes {
		graph.Nodes = append(graph.Nodes, node)
	}

	sort.Slice(graph.Nodes, func(i, j int) bool {
		return graph.Nodes[i].ID < graph.Nodes[j].ID
	})

	return graph
}

// BuildCausalityChains builds chains from the graph for text output.
func BuildCausalityChains(
	graph *CausalityGraph,
	maxDepth int,
	maxChains int,
) []CausalityChain {
	err := assert.AssertNotNil(graph, "graph")
	if err != nil {
		return nil
	}
	err = assert.AssertInRange(maxDepth, 0, maxAnalysisOperations, "max depth")
	if err != nil {
		return nil
	}

	maxDepth, maxChains = normalizeChainLimits(maxDepth, maxChains)

	nodeByID := indexNodes(graph.Nodes)
	adj, fanOut := buildAdjacency(graph.Edges)
	roots := collectRootNodes(adj, fanOut, nodeByID)

	chains := generateChains(roots, adj, fanOut, maxDepth, maxChains)
	sortChains(chains)
	return chains
}

func normalizeChainLimits(maxDepth, maxChains int) (int, int) {
	if maxDepth <= 0 {
		maxDepth = defaultMaxDepth
	}
	if maxChains <= 0 {
		maxChains = defaultMaxChains
	}
	if maxChains > maxCausalityChains {
		maxChains = maxCausalityChains
	}
	return maxDepth, maxChains
}

func indexNodes(nodes []CausalityNode) map[string]CausalityNode {
	nodeByID := make(map[string]CausalityNode, len(nodes))
	maxNodes := len(nodes)
	if maxNodes > maxCausalityNodes {
		maxNodes = maxCausalityNodes
	}
	for i := 0; i < maxNodes; i++ {
		nodeByID[nodes[i].ID] = nodes[i]
	}
	return nodeByID
}

func buildAdjacency(edges []CausalityEdge) (map[string][]string, map[string]int) {
	adj := make(map[string][]string, len(edges))
	fanOut := make(map[string]int, len(edges))

	maxEdges := len(edges)
	if maxEdges > maxCausalityEdges {
		maxEdges = maxCausalityEdges
	}

	for i := 0; i < maxEdges; i++ {
		edge := edges[i]
		adj[edge.From] = append(adj[edge.From], edge.To)
		if edge.Type == EdgeTypeOpToSpan {
			fanOut[edge.From] = fanOut[edge.From] + 1
		}
	}

	keys := make([]string, 0, len(adj))
	count := 0
	maxKeys := len(adj)
	if maxKeys > maxAnalysisOperations {
		maxKeys = maxAnalysisOperations
	}
	for fromID := range adj {
		if count >= maxKeys {
			break
		}
		keys = append(keys, fromID)
		count = count + 1
	}

	for i := 0; i < maxKeys; i++ {
		fromID := keys[i]
		sort.Slice(adj[fromID], func(i, j int) bool {
			return adj[fromID][i] < adj[fromID][j]
		})
	}

	return adj, fanOut
}

func collectRootNodes(
	adj map[string][]string,
	fanOut map[string]int,
	nodeByID map[string]CausalityNode,
) []string {
	rootSet := make(map[string]bool, len(adj))
	count := 0
	maxAdj := len(adj)
	if maxAdj > maxAnalysisOperations {
		maxAdj = maxAnalysisOperations
	}
	for fromID := range adj {
		if count >= maxAdj {
			break
		}
		if fanOut[fromID] > 0 {
			rootSet[fromID] = true
		}
		count = count + 1
	}

	roots := make([]string, 0, len(rootSet))
	count = 0
	maxRoots := len(rootSet)
	if maxRoots > maxAnalysisOperations {
		maxRoots = maxAnalysisOperations
	}
	for rootID := range rootSet {
		if count >= maxRoots {
			break
		}
		if nodeByID[rootID].Type != NodeTypeOperation {
			continue
		}
		roots = append(roots, rootID)
		count = count + 1
	}

	sort.Strings(roots)
	return roots
}

func generateChains(
	roots []string,
	adj map[string][]string,
	fanOut map[string]int,
	maxDepth int,
	maxChains int,
) []CausalityChain {
	chains := make([]CausalityChain, 0, maxChains)
	maxRoots := len(roots)
	if maxRoots > maxAnalysisOperations {
		maxRoots = maxAnalysisOperations
	}

	for i := 0; i < maxRoots && len(chains) < maxChains; i++ {
		rootID := roots[i]
		stack := make([][]string, 0, 50)
		stack = append(stack, []string{rootID})

		for len(stack) > 0 && len(chains) < maxChains {
			path := stack[len(stack)-1]
			stack = stack[:len(stack)-1]

			lastID := path[len(path)-1]
			nextNodes := adj[lastID]

			if len(path) >= maxDepth || len(nextNodes) == 0 {
				chains = append(chains, CausalityChain{
					NodeIDs: path,
					Length:  len(path),
					FanOut:  fanOut[rootID],
				})
				continue
			}

			maxNext := len(nextNodes)
			if maxNext > maxAnalysisOperations {
				maxNext = maxAnalysisOperations
			}
			for j := 0; j < maxNext && len(chains) < maxChains; j++ {
				nextID := nextNodes[j]
				if pathContains(path, nextID) {
					continue
				}
				nextPath := make([]string, len(path)+1)
				copy(nextPath, path)
				nextPath[len(path)] = nextID
				stack = append(stack, nextPath)
			}
		}
	}

	return chains
}

func sortChains(chains []CausalityChain) {
	sort.Slice(chains, func(i, j int) bool {
		if chains[i].Length == chains[j].Length {
			return chains[i].FanOut > chains[j].FanOut
		}
		return chains[i].Length > chains[j].Length
	})
}

func opNodeID(op storage.Operation, index int) string {
	if op.SequenceNumber > 0 {
		return fmt.Sprintf("op:%d", op.SequenceNumber)
	}
	if op.ID > 0 {
		return fmt.Sprintf("op:%d", op.ID)
	}
	return fmt.Sprintf("op:%d", index)
}

func spanNodeID(span storage.ReconcileSpan) string {
	if len(span.ID) > 0 {
		return fmt.Sprintf("span:%s", span.ID)
	}
	return fmt.Sprintf("span:%d", span.StartTime.UnixNano())
}

func findExactMatch(
	exactByKey map[string][]opWithIndex,
	span storage.ReconcileSpan,
) *opWithIndex {
	key := fmt.Sprintf("%s|%s", span.TriggerUID, span.TriggerResourceVersion)
	candidates := exactByKey[key]
	if len(candidates) == 0 {
		return nil
	}

	var best *opWithIndex
	maxCandidates := len(candidates)
	if maxCandidates > maxAnalysisOperations {
		maxCandidates = maxAnalysisOperations
	}
	for i := 0; i < maxCandidates; i++ {
		candidate := candidates[i]
		if candidate.op.Timestamp.After(span.StartTime) {
			continue
		}
		if best == nil || candidate.op.Timestamp.After(best.op.Timestamp) {
			copyCandidate := candidate
			best = &copyCandidate
		}
	}

	return best
}

func findFallbackMatch(
	rvByUID map[string][]rvOp,
	span storage.ReconcileSpan,
) *opWithIndex {
	targetRV, err := strconv.ParseInt(span.TriggerResourceVersion, 10, 64)
	if err != nil {
		return nil
	}

	candidates := rvByUID[span.TriggerUID]
	if len(candidates) == 0 {
		return nil
	}

	var best *rvOp
	maxCandidates := len(candidates)
	if maxCandidates > maxAnalysisOperations {
		maxCandidates = maxAnalysisOperations
	}
	for i := 0; i < maxCandidates; i++ {
		candidate := candidates[i]
		if candidate.rv > targetRV {
			continue
		}
		if candidate.op.Timestamp.After(span.StartTime) {
			continue
		}
		if best == nil {
			copyCandidate := candidate
			best = &copyCandidate
			continue
		}
		if candidate.rv > best.rv {
			copyCandidate := candidate
			best = &copyCandidate
			continue
		}
		if candidate.rv == best.rv && candidate.op.Timestamp.After(best.op.Timestamp) {
			copyCandidate := candidate
			best = &copyCandidate
		}
	}

	if best == nil {
		return nil
	}

	return &opWithIndex{op: best.op, index: best.index}
}

func pathContains(path []string, nodeID string) bool {
	maxPath := len(path)
	if maxPath > maxAnalysisOperations {
		maxPath = maxAnalysisOperations
	}
	for i := 0; i < maxPath; i++ {
		if path[i] == nodeID {
			return true
		}
	}
	return false
}
