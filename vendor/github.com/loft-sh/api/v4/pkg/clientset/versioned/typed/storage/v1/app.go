// Code generated by client-gen. DO NOT EDIT.

package v1

import (
	context "context"

	storagev1 "github.com/loft-sh/api/v4/pkg/apis/storage/v1"
	scheme "github.com/loft-sh/api/v4/pkg/clientset/versioned/scheme"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	gentype "k8s.io/client-go/gentype"
)

// AppsGetter has a method to return a AppInterface.
// A group's client should implement this interface.
type AppsGetter interface {
	Apps() AppInterface
}

// AppInterface has methods to work with App resources.
type AppInterface interface {
	Create(ctx context.Context, app *storagev1.App, opts metav1.CreateOptions) (*storagev1.App, error)
	Update(ctx context.Context, app *storagev1.App, opts metav1.UpdateOptions) (*storagev1.App, error)
	// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
	UpdateStatus(ctx context.Context, app *storagev1.App, opts metav1.UpdateOptions) (*storagev1.App, error)
	Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Get(ctx context.Context, name string, opts metav1.GetOptions) (*storagev1.App, error)
	List(ctx context.Context, opts metav1.ListOptions) (*storagev1.AppList, error)
	Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *storagev1.App, err error)
	AppExpansion
}

// apps implements AppInterface
type apps struct {
	*gentype.ClientWithList[*storagev1.App, *storagev1.AppList]
}

// newApps returns a Apps
func newApps(c *StorageV1Client) *apps {
	return &apps{
		gentype.NewClientWithList[*storagev1.App, *storagev1.AppList](
			"apps",
			c.RESTClient(),
			scheme.ParameterCodec,
			"",
			func() *storagev1.App { return &storagev1.App{} },
			func() *storagev1.AppList { return &storagev1.AppList{} },
		),
	}
}
