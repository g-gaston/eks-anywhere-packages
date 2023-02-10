package controllers

import (
	"context"
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	api "github.com/aws/eks-anywhere-packages/api/v1alpha1"
	"github.com/aws/eks-anywhere-packages/controllers/mocks"
	bundleMocks "github.com/aws/eks-anywhere-packages/pkg/bundle/mocks"
	"github.com/aws/eks-anywhere-packages/pkg/testutil"
)

func givenRequest() ctrl.Request {
	return ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "some-bundle",
			Namespace: api.PackageNamespace,
		},
	}
}

func GivenBundle() *api.PackageBundle {
	return &api.PackageBundle{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "v1-22-1001",
			Namespace: api.PackageNamespace,
		},
		Status: api.PackageBundleStatus{
			State: api.PackageBundleStateAvailable,
		},
	}
}

func doAndReturnBundle(src *api.PackageBundle) func(ctx context.Context, name types.NamespacedName, pb *api.PackageBundle, _ ...client.GetOption) error {
	return func(ctx context.Context, name types.NamespacedName, target *api.PackageBundle, _ ...client.GetOption) error {
		src.DeepCopyInto(target)
		return nil
	}
}

func TestPackageBundleReconciler_ReconcileAddUpdate(t *testing.T) {
	ctx := context.Background()
	request := givenRequest()
	statusWriter := mocks.NewMockStatusWriter(gomock.NewController(t))
	mockClient := mocks.NewMockClient(gomock.NewController(t))
	mockBundleClient := bundleMocks.NewMockClient(gomock.NewController(t))
	bm := bundleMocks.NewMockManager(gomock.NewController(t))
	myBundle := GivenBundle()
	mockClient.EXPECT().Get(ctx, request.NamespacedName, gomock.Any()).DoAndReturn(doAndReturnBundle(myBundle))
	mockClient.EXPECT().Status().Return(statusWriter)
	statusWriter.EXPECT().Update(ctx, gomock.Any(), gomock.Any()).Return(nil)
	bm.EXPECT().ProcessBundle(ctx, myBundle).Return(true, nil)
	sut := NewPackageBundleReconciler(mockClient, nil, mockBundleClient, bm, nil, logr.Discard())

	_, actualError := sut.Reconcile(ctx, request)

	assert.Nil(t, actualError)
}

func TestPackageBundleReconciler_ReconcileError(t *testing.T) {
	ctx := context.Background()
	request := givenRequest()
	mockClient := mocks.NewMockClient(gomock.NewController(t))
	mockBundleClient := bundleMocks.NewMockClient(gomock.NewController(t))
	expectedError := fmt.Errorf("error reading")
	mockClient.EXPECT().Get(ctx, request.NamespacedName, gomock.Any()).Return(expectedError)
	bm := bundleMocks.NewMockManager(gomock.NewController(t))
	sut := NewPackageBundleReconciler(mockClient, nil, mockBundleClient, bm, nil, logr.Discard())

	_, actualError := sut.Reconcile(ctx, request)

	assert.EqualError(t, actualError, "error reading")
}

func TestPackageBundleReconciler_ReconcileIgnored(t *testing.T) {
	ctx := context.Background()
	request := givenRequest()
	request.Name = "bogus"
	mockClient := mocks.NewMockClient(gomock.NewController(t))
	mockBundleClient := bundleMocks.NewMockClient(gomock.NewController(t))
	bm := bundleMocks.NewMockManager(gomock.NewController(t))
	myBundle := GivenBundle()
	mockClient.EXPECT().Get(ctx, request.NamespacedName, gomock.Any()).DoAndReturn(doAndReturnBundle(myBundle))
	bm.EXPECT().ProcessBundle(ctx, myBundle).Return(false, nil)
	sut := NewPackageBundleReconciler(mockClient, nil, mockBundleClient, bm, nil, logr.Discard())

	_, actualError := sut.Reconcile(ctx, request)

	assert.Nil(t, actualError)
}

func TestPackageBundleReconciler_ReconcileDelete(t *testing.T) {
	ctx := context.Background()
	request := givenRequest()
	groupResource := schema.GroupResource{
		Group:    request.Namespace,
		Resource: request.Name,
	}
	notFoundError := errors.NewNotFound(groupResource, request.Name)
	mockClient := mocks.NewMockClient(gomock.NewController(t))
	mockBundleClient := bundleMocks.NewMockClient(gomock.NewController(t))
	bm := bundleMocks.NewMockManager(gomock.NewController(t))
	mockRegistryClient := bundleMocks.NewMockRegistryClient(gomock.NewController(t))
	sut := NewPackageBundleReconciler(mockClient, nil, mockBundleClient, bm, mockRegistryClient, logr.Discard())
	mockClient.EXPECT().Get(ctx, request.NamespacedName, gomock.Any()).Return(notFoundError)

	_, actualError := sut.Reconcile(ctx, request)

	assert.Nil(t, actualError)
}

func TestPackageBundleReconciler_mapBundleReconcileRequests(t *testing.T) {
	ctx := context.Background()
	bundleOne, err := testutil.GivenPackageBundle("../api/testdata/bundle_one.yaml")
	assert.NoError(t, err)
	bundleTwo, err := testutil.GivenPackageBundle("../api/testdata/bundle_two.yaml")
	assert.NoError(t, err)
	mockClient := mocks.NewMockClient(gomock.NewController(t))
	mockBundleClient := bundleMocks.NewMockClient(gomock.NewController(t))
	mockClient.EXPECT().
		List(ctx, gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, bundles *api.PackageBundleList,
			_ ...*client.ListOptions,
		) error {
			bundles.Items = []api.PackageBundle{*bundleOne, *bundleTwo}
			return nil
		})
	bm := bundleMocks.NewMockManager(gomock.NewController(t))
	sut := NewPackageBundleReconciler(mockClient, nil, mockBundleClient, bm, nil, logr.Discard())

	requests := sut.mapBundleReconcileRequests(&api.PackageBundleController{})

	assert.Equal(t, 2, len(requests))
}
