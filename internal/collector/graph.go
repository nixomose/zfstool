package collector

import (
	"context"

	"github.com/nixomose/zfstool/internal/api"
)

// BuildDatasetGraph builds parent/child edges from origin + name hierarchy.
func BuildDatasetGraph(ctx context.Context, pool string) ([]api.DatasetGraphNode, error) {
	rows, err := ListDatasets(ctx, pool)
	if err != nil {
		return nil, err
	}
	byName := make(map[string]*api.DatasetGraphNode)
	for _, r := range rows {
		byName[r.Name] = &api.DatasetGraphNode{Name: r.Name, Type: r.Type, Origin: r.Origin}
	}
	// link children: snapshot/clone origin -> this node
	for _, r := range rows {
		if r.Origin != "" && r.Origin != "-" {
			if p, ok := byName[r.Origin]; ok {
				p.Children = append(p.Children, r.Name)
			}
		}
	}
	out := make([]api.DatasetGraphNode, 0, len(byName))
	for _, n := range byName {
		out = append(out, *n)
	}
	return out, nil
}
