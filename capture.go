package openapiparser

import (
	"fmt"
	"math"
	"strconv"

	"golang.org/x/sync/errgroup"
	"gopkg.in/yaml.v3"
)

type captureFunc func(node *yaml.Node) error

type captureMap map[string]captureFunc

// Capture runs capture functions concurrently on the given keys.
func capture(content []*yaml.Node, captures captureMap) error {
	var g errgroup.Group
	pairs, err := mapPairs(content)
	if err != nil {
		return err
	}
	for _, v := range pairs {
		if fx, ok := captures[v.key]; ok {
			fx := fx
			n := v.node
			g.Go(func() error {
				return fx(n)
			})
			continue
		}
		return fmt.Errorf("unsupported key capturing content: %s", v.key)
	}
	return g.Wait()
}

func captureStringSlice(ss *[]string) captureFunc {
	return func(node *yaml.Node) error {
		if err := assertKind(node, yaml.SequenceNode); err != nil {
			return err
		}
		for _, n := range node.Content {
			if err := assertKind(n, yaml.ScalarNode); err != nil {
				return err
			}
			*ss = append(*ss, n.Value)
		}
		return nil
	}
}

func captureEnumX(x *EnumX) captureFunc {
	return func(node *yaml.Node) error {
		pairs, err := mapPairs(node.Content)
		if err != nil {
			return err
		}
		for _, v := range pairs {
			if err := assertScalar(v.node); err != nil {
				return err
			}
			x.Options = append(x.Options, EnumOptionX{
				Key:         v.key,
				Description: v.node.Value,
			})
		}
		return nil
	}
}

func captureString(s *string) captureFunc {
	return func(node *yaml.Node) error {
		if err := assertScalar(node); err != nil {
			return err
		}
		*s = node.Value
		return nil
	}
}

func captureBool(b *bool) captureFunc {
	return func(node *yaml.Node) error {
		if err := assertScalar(node); err != nil {
			return err
		}
		x, err := strconv.ParseBool(node.Value)
		if err != nil {
			return fmt.Errorf("cannot parse %q as boolean: %v", node.Value, err)
		}
		*b = x
		return nil
	}
}

func captureFloat(f *float64) captureFunc {
	return func(node *yaml.Node) error {
		if err := assertScalar(node); err != nil {
			return err
		}
		x, err := strconv.ParseFloat(node.Value, 64)
		if err != nil {
			return fmt.Errorf("cannot parse %q as float: %v", node.Value, err)
		}
		*f = x
		return nil
	}
}

func captureInt(i *int) captureFunc {
	return func(node *yaml.Node) error {
		if !isScalar(node) {
			return fmt.Errorf("expected scalar node")
		}
		x, err := strconv.ParseInt(node.Value, 10, 0)
		if err != nil {
			return fmt.Errorf("expected number but cannot parse as float %q: %v", node.Value, err)
		}
		*i = int(x)
		return nil
	}
}

func captureNodeDecode(v interface{}) captureFunc {
	return func(node *yaml.Node) error {
		return node.Decode(v)
	}
}

func captureRawFunc(raw *string) captureFunc {
	return func(node *yaml.Node) error {
		b, err := yaml.Marshal(node)
		if err != nil {
			return fmt.Errorf("cannot marshal raw node content: %v", err)
		}
		*raw = string(b)
		return nil
	}
}

type pair struct {
	key  string
	node *yaml.Node
}

func mapPairs(nodes []*yaml.Node) ([]pair, error) {
	length := len(nodes)
	if math.Mod(float64(length), 2) != 0 {
		return nil, fmt.Errorf("asked for pairs on an off number of nodes: %d", length)
	}
	pairs := make([]pair, length/2)
	for i, j := 0, 1; j < length; i, j = i+2, j+2 {
		var (
			key   = nodes[i]
			value = nodes[j]
		)
		if key.Kind != yaml.ScalarNode {
			return nil, fmt.Errorf("first element of a map pair should be an scalar, got: %v", key.Kind)
		}
		pairs[i/2] = pair{
			key:  key.Value,
			node: value,
		}
	}
	return pairs, nil
}
