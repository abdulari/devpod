// Code generated by lister-gen. DO NOT EDIT.

package v1

import (
	managementv1 "github.com/loft-sh/api/v4/pkg/apis/management/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	listers "k8s.io/client-go/listers"
	cache "k8s.io/client-go/tools/cache"
)

// VirtualClusterTemplateLister helps list VirtualClusterTemplates.
// All objects returned here must be treated as read-only.
type VirtualClusterTemplateLister interface {
	// List lists all VirtualClusterTemplates in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*managementv1.VirtualClusterTemplate, err error)
	// Get retrieves the VirtualClusterTemplate from the index for a given name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*managementv1.VirtualClusterTemplate, error)
	VirtualClusterTemplateListerExpansion
}

// virtualClusterTemplateLister implements the VirtualClusterTemplateLister interface.
type virtualClusterTemplateLister struct {
	listers.ResourceIndexer[*managementv1.VirtualClusterTemplate]
}

// NewVirtualClusterTemplateLister returns a new VirtualClusterTemplateLister.
func NewVirtualClusterTemplateLister(indexer cache.Indexer) VirtualClusterTemplateLister {
	return &virtualClusterTemplateLister{listers.New[*managementv1.VirtualClusterTemplate](indexer, managementv1.Resource("virtualclustertemplate"))}
}
