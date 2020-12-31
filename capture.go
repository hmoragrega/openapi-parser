package openapiparser

import (
	"fmt"
	"math"
	"strconv"
	"sync"

	"gopkg.in/yaml.v3"
)

type captureFunc func(node *yaml.Node)

type captureMap map[string]captureFunc

// Capture runs capture functions concurrently on the given keys.
func capture(content []*yaml.Node, captures captureMap) {
	var wg sync.WaitGroup
	async := func(fx captureFunc, node *yaml.Node) {
		wg.Add(1)
		go func() {
			fx(node)
			wg.Done()
		}()
	}
	for _, v := range mapPairs(content) {
		if fx, ok := captures[v.key]; ok {
			async(fx, v.node)
			continue
		}
		panic(fmt.Errorf("unsupported key capturing content: %s", v.key))
	}
	wg.Wait()
}

func captureStringSlice(ss *[]string) captureFunc {
	return func(node *yaml.Node) {
		assertKind(node, yaml.SequenceNode)
		for _, n := range node.Content {
			assertKind(n, yaml.ScalarNode)
			*ss = append(*ss, n.Value)
		}
	}
}

func captureEnumX(x *EnumX) captureFunc {
	return func(node *yaml.Node) {
		for _, v := range mapPairs(node.Content) {
			assertKind(v.node, yaml.ScalarNode)
			x.Options = append(x.Options, EnumOptionX{
				Key:         v.key,
				Description: v.node.Value,
			})
		}
	}
}

func captureString(s *string) captureFunc {
	return func(node *yaml.Node) {
		*s = assertScalar(node).Value
	}
}

func captureBool(b *bool) captureFunc {
	return func(node *yaml.Node) {
		s := assertScalar(node).Value
		x, err := strconv.ParseBool(s)
		if err != nil {
			panic(fmt.Errorf("expected boolean string representation but got %q", s))
		}
		*b = x
	}
}

func captureFloat(f *float64) captureFunc {
	return func(node *yaml.Node) {
		s := assertScalar(node).Value
		x, err := strconv.ParseFloat(s, 64)
		if err != nil {
			panic(fmt.Errorf("expected number but cannot parse as float %q: %v", s, err))
		}
		*f = x
	}
}
func captureInt(i *int) captureFunc {
	return func(node *yaml.Node) {
		s := assertScalar(node).Value
		x, err := strconv.ParseInt(s, 10, 0)
		if err != nil {
			panic(fmt.Errorf("expected number but cannot parse as float %q: %v", s, err))
		}
		*i = int(x)
	}
}

func captureNodeDecode(v interface{}) captureFunc {
	return func(node *yaml.Node) {
		panicIfErr(node.Decode(v))
	}
}

func captureRawFunc(raw *string) captureFunc {
	return func(node *yaml.Node) {
		b, err := yaml.Marshal(node)
		if err != nil {
			panic(fmt.Errorf("cannot capture raw node content: %v", err))
		}
		*raw = string(b)
	}
}

type pair struct {
	key  string
	node *yaml.Node
}

func mapPairs(nodes []*yaml.Node) []pair {
	length := len(nodes)
	if math.Mod(float64(length), 2) != 0 {
		panic(fmt.Errorf("asked for pairs on an off number of nodes: %d", length))
	}
	pairs := make([]pair, length/2)
	for i, j := 0, 1; j < length; i, j = i+2, j+2 {
		var (
			key   = nodes[i]
			value = nodes[j]
		)
		if key.Kind != yaml.ScalarNode {
			panic(fmt.Errorf("first element of a map pair should be an scalar, got: %v", key.Kind))
		}
		pairs[i/2] = pair{
			key:  key.Value,
			node: value,
		}
	}
	return pairs
}
