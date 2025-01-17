/*
 * Copyright 2018- The Pixie Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package controller_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/gofrs/uuid"
	"github.com/gogo/protobuf/proto"
	"github.com/gogo/protobuf/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"px.dev/pixie/src/api/proto/cloudpb"
	"px.dev/pixie/src/api/proto/uuidpb"
	"px.dev/pixie/src/api/proto/vispb"
	"px.dev/pixie/src/cloud/api/controller"
	"px.dev/pixie/src/cloud/api/controller/testutils"
	"px.dev/pixie/src/cloud/artifact_tracker/artifacttrackerpb"
	"px.dev/pixie/src/cloud/auth/authpb"
	"px.dev/pixie/src/cloud/autocomplete"
	mock_autocomplete "px.dev/pixie/src/cloud/autocomplete/mock"
	"px.dev/pixie/src/cloud/profile/profilepb"
	"px.dev/pixie/src/cloud/scriptmgr/scriptmgrpb"
	mock_scriptmgr "px.dev/pixie/src/cloud/scriptmgr/scriptmgrpb/mock"
	"px.dev/pixie/src/cloud/vzmgr/vzmgrpb"
	"px.dev/pixie/src/shared/artifacts/versionspb"
	"px.dev/pixie/src/shared/cvmsgspb"
	"px.dev/pixie/src/shared/k8s/metadatapb"
	"px.dev/pixie/src/utils"
)

func TestArtifactTracker_GetArtifactList(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, mockClients, cleanup := testutils.CreateTestAPIEnv(t)
	defer cleanup()
	ctx := context.Background()

	mockClients.MockArtifact.EXPECT().GetArtifactList(gomock.Any(),
		&artifacttrackerpb.GetArtifactListRequest{
			ArtifactName: "cli",
			Limit:        1,
			ArtifactType: versionspb.AT_LINUX_AMD64,
		}).
		Return(&versionspb.ArtifactSet{
			Name: "cli",
			Artifact: []*versionspb.Artifact{{
				VersionStr: "test",
			}},
		}, nil)

	artifactTrackerServer := &controller.ArtifactTrackerServer{
		ArtifactTrackerClient: mockClients.MockArtifact,
	}

	resp, err := artifactTrackerServer.GetArtifactList(ctx, &cloudpb.GetArtifactListRequest{
		ArtifactName: "cli",
		Limit:        1,
		ArtifactType: cloudpb.AT_LINUX_AMD64,
	})

	require.NoError(t, err)
	assert.Equal(t, "cli", resp.Name)
	assert.Equal(t, 1, len(resp.Artifact))
}

func TestArtifactTracker_GetDownloadLink(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, mockClients, cleanup := testutils.CreateTestAPIEnv(t)
	defer cleanup()
	ctx := context.Background()

	mockClients.MockArtifact.EXPECT().GetDownloadLink(gomock.Any(),
		&artifacttrackerpb.GetDownloadLinkRequest{
			ArtifactName: "cli",
			VersionStr:   "version",
			ArtifactType: versionspb.AT_LINUX_AMD64,
		}).
		Return(&artifacttrackerpb.GetDownloadLinkResponse{
			Url:    "http://localhost",
			SHA256: "sha",
		}, nil)

	artifactTrackerServer := &controller.ArtifactTrackerServer{
		ArtifactTrackerClient: mockClients.MockArtifact,
	}

	resp, err := artifactTrackerServer.GetDownloadLink(ctx, &cloudpb.GetDownloadLinkRequest{
		ArtifactName: "cli",
		VersionStr:   "version",
		ArtifactType: cloudpb.AT_LINUX_AMD64,
	})

	require.NoError(t, err)
	assert.Equal(t, "http://localhost", resp.Url)
	assert.Equal(t, "sha", resp.SHA256)
}

func TestVizierClusterInfo_GetClusterConnectionInfo(t *testing.T) {
	clusterID := utils.ProtoFromUUIDStrOrNil("7ba7b810-9dad-11d1-80b4-00c04fd430c8")

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, mockClients, cleanup := testutils.CreateTestAPIEnv(t)
	defer cleanup()
	ctx := CreateTestContext()

	mockClients.MockVzMgr.EXPECT().GetVizierConnectionInfo(gomock.Any(), clusterID).Return(&cvmsgspb.VizierConnectionInfo{
		IPAddress: "127.0.0.1",
		Token:     "hello",
	}, nil)

	vzClusterInfoServer := &controller.VizierClusterInfo{
		VzMgr: mockClients.MockVzMgr,
	}

	resp, err := vzClusterInfoServer.GetClusterConnectionInfo(ctx, &cloudpb.GetClusterConnectionInfoRequest{ID: clusterID})
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1", resp.IPAddress)
	assert.Equal(t, "hello", resp.Token)
}

func TestVizierClusterInfo_GetClusterInfo(t *testing.T) {
	orgID := utils.ProtoFromUUIDStrOrNil("6ba7b810-9dad-11d1-80b4-00c04fd430c8")
	clusterID := utils.ProtoFromUUIDStrOrNil("7ba7b810-9dad-11d1-80b4-00c04fd430c8")
	assert.NotNil(t, clusterID)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, mockClients, cleanup := testutils.CreateTestAPIEnv(t)
	defer cleanup()
	ctx := CreateTestContext()

	mockClients.MockVzMgr.EXPECT().GetViziersByOrg(gomock.Any(), orgID).Return(&vzmgrpb.GetViziersByOrgResponse{
		VizierIDs: []*uuidpb.UUID{clusterID},
	}, nil)

	mockClients.MockVzMgr.EXPECT().GetVizierInfos(gomock.Any(), &vzmgrpb.GetVizierInfosRequest{
		VizierIDs: []*uuidpb.UUID{clusterID},
	}).Return(&vzmgrpb.GetVizierInfosResponse{
		VizierInfos: []*cvmsgspb.VizierInfo{{
			VizierID:        clusterID,
			Status:          cvmsgspb.VZ_ST_HEALTHY,
			LastHeartbeatNs: int64(1305646598000000000),
			Config: &cvmsgspb.VizierConfig{
				PassthroughEnabled: false,
				AutoUpdateEnabled:  true,
			},
			VizierVersion:  "1.2.3",
			ClusterUID:     "a UID",
			ClusterName:    "gke_pl-dev-infra_us-west1-a_dev-cluster-zasgar-3",
			ClusterVersion: "5.6.7",
			ControlPlanePodStatuses: map[string]*cvmsgspb.PodStatus{
				"vizier-proxy": {
					Name:   "vizier-proxy",
					Status: metadatapb.RUNNING,
					Containers: []*cvmsgspb.ContainerStatus{
						{
							Name:      "my-proxy-container",
							State:     metadatapb.CONTAINER_STATE_RUNNING,
							Message:   "container message",
							Reason:    "container reason",
							CreatedAt: &types.Timestamp{Seconds: 1561230620},
						},
					},
					Events: []*cvmsgspb.K8SEvent{
						{
							Message:   "this is a test event",
							FirstTime: &types.Timestamp{Seconds: 1561230620},
							LastTime:  &types.Timestamp{Seconds: 1561230625},
						},
					},
					StatusMessage: "pod message",
					Reason:        "pod reason",
					CreatedAt:     &types.Timestamp{Seconds: 1561230621},
				},
				"vizier-query-broker": {
					Name:   "vizier-query-broker",
					Status: metadatapb.RUNNING,
				},
			},
			NumNodes:             5,
			NumInstrumentedNodes: 3,
		}},
	}, nil)

	vzClusterInfoServer := &controller.VizierClusterInfo{
		VzMgr: mockClients.MockVzMgr,
	}

	resp, err := vzClusterInfoServer.GetClusterInfo(ctx, &cloudpb.GetClusterInfoRequest{})

	expectedPodStatuses := map[string]*cloudpb.PodStatus{
		"vizier-proxy": {
			Name:   "vizier-proxy",
			Status: cloudpb.RUNNING,
			Containers: []*cloudpb.ContainerStatus{
				{
					Name:      "my-proxy-container",
					State:     cloudpb.CONTAINER_STATE_RUNNING,
					Message:   "container message",
					Reason:    "container reason",
					CreatedAt: &types.Timestamp{Seconds: 1561230620},
				},
			},
			Events: []*cloudpb.K8SEvent{
				{
					Message:   "this is a test event",
					FirstTime: &types.Timestamp{Seconds: 1561230620},
					LastTime:  &types.Timestamp{Seconds: 1561230625},
				},
			},
			StatusMessage: "pod message",
			Reason:        "pod reason",
			CreatedAt:     &types.Timestamp{Seconds: 1561230621},
		},
		"vizier-query-broker": {
			Name:      "vizier-query-broker",
			Status:    cloudpb.RUNNING,
			CreatedAt: nil,
		},
	}

	require.NoError(t, err)
	assert.Equal(t, 1, len(resp.Clusters))
	cluster := resp.Clusters[0]
	assert.Equal(t, cluster.ID, clusterID)
	assert.Equal(t, cluster.Status, cloudpb.CS_HEALTHY)
	assert.Equal(t, cluster.LastHeartbeatNs, int64(1305646598000000000))
	assert.Equal(t, cluster.Config.PassthroughEnabled, false)
	assert.Equal(t, cluster.Config.AutoUpdateEnabled, true)
	assert.Equal(t, "1.2.3", cluster.VizierVersion)
	assert.Equal(t, "a UID", cluster.ClusterUID)
	assert.Equal(t, "gke_pl-dev-infra_us-west1-a_dev-cluster-zasgar-3", cluster.ClusterName)
	assert.Equal(t, "gke:dev-cluster-zasgar-3", cluster.PrettyClusterName)
	assert.Equal(t, "5.6.7", cluster.ClusterVersion)
	assert.Equal(t, expectedPodStatuses, cluster.ControlPlanePodStatuses)
	assert.Equal(t, int32(5), cluster.NumNodes)
	assert.Equal(t, int32(3), cluster.NumInstrumentedNodes)
}

func TestVizierClusterInfo_GetClusterInfoDuplicates(t *testing.T) {
	orgID := utils.ProtoFromUUIDStrOrNil("6ba7b810-9dad-11d1-80b4-00c04fd430c8")
	clusterID := utils.ProtoFromUUIDStrOrNil("7ba7b810-9dad-11d1-80b4-00c04fd430c8")
	assert.NotNil(t, clusterID)
	clusterID2 := utils.ProtoFromUUIDStrOrNil("7ba7b810-9dad-11d1-80b4-00c04fd430c9")

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, mockClients, cleanup := testutils.CreateTestAPIEnv(t)
	defer cleanup()
	ctx := CreateTestContext()

	mockClients.MockVzMgr.EXPECT().GetViziersByOrg(gomock.Any(), orgID).Return(&vzmgrpb.GetViziersByOrgResponse{
		VizierIDs: []*uuidpb.UUID{clusterID, clusterID2},
	}, nil)

	mockClients.MockVzMgr.EXPECT().GetVizierInfos(gomock.Any(), &vzmgrpb.GetVizierInfosRequest{
		VizierIDs: []*uuidpb.UUID{clusterID, clusterID2},
	}).Return(&vzmgrpb.GetVizierInfosResponse{
		VizierInfos: []*cvmsgspb.VizierInfo{{
			VizierID:        clusterID,
			Status:          cvmsgspb.VZ_ST_HEALTHY,
			LastHeartbeatNs: int64(1305646598000000000),
			Config: &cvmsgspb.VizierConfig{
				PassthroughEnabled: false,
				AutoUpdateEnabled:  true,
			},
			VizierVersion:        "1.2.3",
			ClusterUID:           "a UID",
			ClusterName:          "gke_pl-dev-infra_us-west1-a_dev-cluster-zasgar",
			ClusterVersion:       "5.6.7",
			NumNodes:             5,
			NumInstrumentedNodes: 3,
		},
			{
				VizierID:        clusterID,
				Status:          cvmsgspb.VZ_ST_HEALTHY,
				LastHeartbeatNs: int64(1305646598000000000),
				Config: &cvmsgspb.VizierConfig{
					PassthroughEnabled: false,
					AutoUpdateEnabled:  true,
				},
				VizierVersion:        "1.2.3",
				ClusterUID:           "a UID2",
				ClusterName:          "gke_pl-pixies_us-west1-a_dev-cluster-zasgar",
				ClusterVersion:       "5.6.7",
				NumNodes:             5,
				NumInstrumentedNodes: 3,
			},
		},
	}, nil)

	vzClusterInfoServer := &controller.VizierClusterInfo{
		VzMgr: mockClients.MockVzMgr,
	}

	resp, err := vzClusterInfoServer.GetClusterInfo(ctx, &cloudpb.GetClusterInfoRequest{})

	require.NoError(t, err)
	assert.Equal(t, 2, len(resp.Clusters))
	assert.Equal(t, "gke:dev-cluster-zasgar (pl-dev-infra)", resp.Clusters[0].PrettyClusterName)
	assert.Equal(t, "gke:dev-cluster-zasgar (pl-pixies)", resp.Clusters[1].PrettyClusterName)
}

func TestVizierClusterInfo_GetClusterInfoWithID(t *testing.T) {
	clusterID := utils.ProtoFromUUIDStrOrNil("7ba7b810-9dad-11d1-80b4-00c04fd430c8")
	assert.NotNil(t, clusterID)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, mockClients, cleanup := testutils.CreateTestAPIEnv(t)
	defer cleanup()
	ctx := CreateTestContext()

	mockClients.MockVzMgr.EXPECT().GetVizierInfos(gomock.Any(), &vzmgrpb.GetVizierInfosRequest{
		VizierIDs: []*uuidpb.UUID{clusterID},
	}).Return(&vzmgrpb.GetVizierInfosResponse{
		VizierInfos: []*cvmsgspb.VizierInfo{{
			VizierID:        clusterID,
			Status:          cvmsgspb.VZ_ST_HEALTHY,
			LastHeartbeatNs: int64(1305646598000000000),
			Config: &cvmsgspb.VizierConfig{
				PassthroughEnabled: false,
				AutoUpdateEnabled:  true,
			},
			VizierVersion:  "1.2.3",
			ClusterUID:     "a UID",
			ClusterName:    "some cluster",
			ClusterVersion: "5.6.7",
		},
		},
	}, nil)

	vzClusterInfoServer := &controller.VizierClusterInfo{
		VzMgr: mockClients.MockVzMgr,
	}

	resp, err := vzClusterInfoServer.GetClusterInfo(ctx, &cloudpb.GetClusterInfoRequest{
		ID: clusterID,
	})

	require.NoError(t, err)
	assert.Equal(t, 1, len(resp.Clusters))
	cluster := resp.Clusters[0]
	assert.Equal(t, cluster.ID, clusterID)
	assert.Equal(t, cluster.Status, cloudpb.CS_HEALTHY)
	assert.Equal(t, cluster.LastHeartbeatNs, int64(1305646598000000000))
	assert.Equal(t, cluster.Config.PassthroughEnabled, false)
	assert.Equal(t, cluster.Config.AutoUpdateEnabled, true)
}

func TestVizierClusterInfo_UpdateClusterVizierConfig(t *testing.T) {
	clusterID := utils.ProtoFromUUIDStrOrNil("7ba7b810-9dad-11d1-80b4-00c04fd430c8")
	assert.NotNil(t, clusterID)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, mockClients, cleanup := testutils.CreateTestAPIEnv(t)
	defer cleanup()
	ctx := CreateTestContext()

	updateReq := &cvmsgspb.UpdateVizierConfigRequest{
		VizierID: clusterID,
		ConfigUpdate: &cvmsgspb.VizierConfigUpdate{
			PassthroughEnabled: &types.BoolValue{Value: true},
			AutoUpdateEnabled:  &types.BoolValue{Value: false},
		},
	}

	mockClients.MockVzMgr.EXPECT().UpdateVizierConfig(gomock.Any(), updateReq).Return(&cvmsgspb.UpdateVizierConfigResponse{}, nil)

	vzClusterInfoServer := &controller.VizierClusterInfo{
		VzMgr: mockClients.MockVzMgr,
	}

	resp, err := vzClusterInfoServer.UpdateClusterVizierConfig(ctx, &cloudpb.UpdateClusterVizierConfigRequest{
		ID: clusterID,
		ConfigUpdate: &cloudpb.VizierConfigUpdate{
			PassthroughEnabled: &types.BoolValue{Value: true},
			AutoUpdateEnabled:  &types.BoolValue{Value: false},
		},
	})

	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestVizierClusterInfo_UpdateOrInstallCluster(t *testing.T) {
	clusterID := utils.ProtoFromUUIDStrOrNil("7ba7b810-9dad-11d1-80b4-00c04fd430c8")
	assert.NotNil(t, clusterID)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, mockClients, cleanup := testutils.CreateTestAPIEnv(t)
	defer cleanup()
	ctx := CreateTestContext()

	updateReq := &cvmsgspb.UpdateOrInstallVizierRequest{
		VizierID: clusterID,
		Version:  "0.1.30",
	}

	mockClients.MockVzMgr.EXPECT().UpdateOrInstallVizier(gomock.Any(), updateReq).Return(&cvmsgspb.UpdateOrInstallVizierResponse{UpdateStarted: true}, nil)

	mockClients.MockArtifact.EXPECT().
		GetDownloadLink(gomock.Any(), &artifacttrackerpb.GetDownloadLinkRequest{
			ArtifactName: "vizier",
			VersionStr:   "0.1.30",
			ArtifactType: versionspb.AT_CONTAINER_SET_YAMLS,
		}).
		Return(nil, nil)

	vzClusterInfoServer := &controller.VizierClusterInfo{
		VzMgr:                 mockClients.MockVzMgr,
		ArtifactTrackerClient: mockClients.MockArtifact,
	}

	resp, err := vzClusterInfoServer.UpdateOrInstallCluster(ctx, &cloudpb.UpdateOrInstallClusterRequest{
		ClusterID: clusterID,
		Version:   "0.1.30",
	})

	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestVizierDeploymentKeyServer_Create(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, mockClients, cleanup := testutils.CreateTestAPIEnv(t)
	defer cleanup()
	ctx := CreateTestContext()

	vzreq := &vzmgrpb.CreateDeploymentKeyRequest{Desc: "test key"}
	vzresp := &vzmgrpb.DeploymentKey{
		ID:        utils.ProtoFromUUIDStrOrNil("6ba7b810-9dad-11d1-80b4-00c04fd430c8"),
		Key:       "foobar",
		CreatedAt: types.TimestampNow(),
	}
	mockClients.MockVzDeployKey.EXPECT().
		Create(gomock.Any(), vzreq).Return(vzresp, nil)

	vzDeployKeyServer := &controller.VizierDeploymentKeyServer{
		VzDeploymentKey: mockClients.MockVzDeployKey,
	}

	resp, err := vzDeployKeyServer.Create(ctx, &cloudpb.CreateDeploymentKeyRequest{Desc: "test key"})
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, resp.ID, vzresp.ID)
	assert.Equal(t, resp.Key, vzresp.Key)
	assert.Equal(t, resp.CreatedAt, vzresp.CreatedAt)
}

func TestVizierDeploymentKeyServer_List(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, mockClients, cleanup := testutils.CreateTestAPIEnv(t)
	defer cleanup()
	ctx := CreateTestContext()

	vzreq := &vzmgrpb.ListDeploymentKeyRequest{}
	vzresp := &vzmgrpb.ListDeploymentKeyResponse{
		Keys: []*vzmgrpb.DeploymentKey{
			{
				ID:        utils.ProtoFromUUIDStrOrNil("6ba7b810-9dad-11d1-80b4-00c04fd430c8"),
				Key:       "foobar",
				CreatedAt: types.TimestampNow(),
				Desc:      "this is a key",
			},
		},
	}
	mockClients.MockVzDeployKey.EXPECT().
		List(gomock.Any(), vzreq).Return(vzresp, nil)

	vzDeployKeyServer := &controller.VizierDeploymentKeyServer{
		VzDeploymentKey: mockClients.MockVzDeployKey,
	}

	resp, err := vzDeployKeyServer.List(ctx, &cloudpb.ListDeploymentKeyRequest{})
	require.NoError(t, err)
	assert.NotNil(t, resp)
	for i, key := range resp.Keys {
		assert.Equal(t, key.ID, vzresp.Keys[i].ID)
		assert.Equal(t, key.Key, vzresp.Keys[i].Key)
		assert.Equal(t, key.CreatedAt, vzresp.Keys[i].CreatedAt)
		assert.Equal(t, key.Desc, vzresp.Keys[i].Desc)
	}
}

func TestVizierDeploymentKeyServer_Get(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, mockClients, cleanup := testutils.CreateTestAPIEnv(t)
	defer cleanup()
	ctx := CreateTestContext()

	id := utils.ProtoFromUUIDStrOrNil("6ba7b810-9dad-11d1-80b4-00c04fd430c8")
	vzreq := &vzmgrpb.GetDeploymentKeyRequest{
		ID: id,
	}
	vzresp := &vzmgrpb.GetDeploymentKeyResponse{
		Key: &vzmgrpb.DeploymentKey{
			ID:        id,
			Key:       "foobar",
			CreatedAt: types.TimestampNow(),
			Desc:      "this is a key",
		},
	}
	mockClients.MockVzDeployKey.EXPECT().
		Get(gomock.Any(), vzreq).Return(vzresp, nil)

	vzDeployKeyServer := &controller.VizierDeploymentKeyServer{
		VzDeploymentKey: mockClients.MockVzDeployKey,
	}
	resp, err := vzDeployKeyServer.Get(ctx, &cloudpb.GetDeploymentKeyRequest{
		ID: id,
	})
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, resp.Key.ID, vzresp.Key.ID)
	assert.Equal(t, resp.Key.Key, vzresp.Key.Key)
	assert.Equal(t, resp.Key.CreatedAt, vzresp.Key.CreatedAt)
	assert.Equal(t, resp.Key.Desc, vzresp.Key.Desc)
}

func TestVizierDeploymentKeyServer_Delete(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, mockClients, cleanup := testutils.CreateTestAPIEnv(t)
	defer cleanup()
	ctx := CreateTestContext()

	id := utils.ProtoFromUUIDStrOrNil("6ba7b810-9dad-11d1-80b4-00c04fd430c8")
	vzresp := &types.Empty{}
	mockClients.MockVzDeployKey.EXPECT().
		Delete(gomock.Any(), id).Return(vzresp, nil)

	vzDeployKeyServer := &controller.VizierDeploymentKeyServer{
		VzDeploymentKey: mockClients.MockVzDeployKey,
	}
	resp, err := vzDeployKeyServer.Delete(ctx, id)
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, resp, vzresp)
}

func TestAPIKeyServer_Create(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, mockClients, cleanup := testutils.CreateTestAPIEnv(t)
	defer cleanup()
	ctx := CreateTestContext()

	vzreq := &authpb.CreateAPIKeyRequest{Desc: "test key"}
	vzresp := &authpb.APIKey{
		ID:        utils.ProtoFromUUIDStrOrNil("6ba7b810-9dad-11d1-80b4-00c04fd430c8"),
		Key:       "foobar",
		CreatedAt: types.TimestampNow(),
	}
	mockClients.MockAPIKey.EXPECT().
		Create(gomock.Any(), vzreq).Return(vzresp, nil)

	vzAPIKeyServer := &controller.APIKeyServer{
		APIKeyClient: mockClients.MockAPIKey,
	}

	resp, err := vzAPIKeyServer.Create(ctx, &cloudpb.CreateAPIKeyRequest{Desc: "test key"})
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, resp.ID, vzresp.ID)
	assert.Equal(t, resp.Key, vzresp.Key)
	assert.Equal(t, resp.CreatedAt, vzresp.CreatedAt)
}

func TestAPIKeyServer_List(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, mockClients, cleanup := testutils.CreateTestAPIEnv(t)
	defer cleanup()
	ctx := CreateTestContext()

	vzreq := &authpb.ListAPIKeyRequest{}
	vzresp := &authpb.ListAPIKeyResponse{
		Keys: []*authpb.APIKey{
			{
				ID:        utils.ProtoFromUUIDStrOrNil("6ba7b810-9dad-11d1-80b4-00c04fd430c8"),
				Key:       "foobar",
				CreatedAt: types.TimestampNow(),
				Desc:      "this is a key",
			},
		},
	}
	mockClients.MockAPIKey.EXPECT().
		List(gomock.Any(), vzreq).Return(vzresp, nil)

	vzAPIKeyServer := &controller.APIKeyServer{
		APIKeyClient: mockClients.MockAPIKey,
	}

	resp, err := vzAPIKeyServer.List(ctx, &cloudpb.ListAPIKeyRequest{})
	require.NoError(t, err)
	assert.NotNil(t, resp)
	for i, key := range resp.Keys {
		assert.Equal(t, key.ID, vzresp.Keys[i].ID)
		assert.Equal(t, key.Key, vzresp.Keys[i].Key)
		assert.Equal(t, key.CreatedAt, vzresp.Keys[i].CreatedAt)
		assert.Equal(t, key.Desc, vzresp.Keys[i].Desc)
	}
}

func TestAPIKeyServer_Get(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, mockClients, cleanup := testutils.CreateTestAPIEnv(t)
	defer cleanup()
	ctx := CreateTestContext()

	id := utils.ProtoFromUUIDStrOrNil("6ba7b810-9dad-11d1-80b4-00c04fd430c8")
	vzreq := &authpb.GetAPIKeyRequest{
		ID: id,
	}
	vzresp := &authpb.GetAPIKeyResponse{
		Key: &authpb.APIKey{
			ID:        id,
			Key:       "foobar",
			CreatedAt: types.TimestampNow(),
			Desc:      "this is a key",
		},
	}
	mockClients.MockAPIKey.EXPECT().
		Get(gomock.Any(), vzreq).Return(vzresp, nil)

	vzAPIKeyServer := &controller.APIKeyServer{
		APIKeyClient: mockClients.MockAPIKey,
	}
	resp, err := vzAPIKeyServer.Get(ctx, &cloudpb.GetAPIKeyRequest{
		ID: id,
	})
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, resp.Key.ID, vzresp.Key.ID)
	assert.Equal(t, resp.Key.Key, vzresp.Key.Key)
	assert.Equal(t, resp.Key.CreatedAt, vzresp.Key.CreatedAt)
	assert.Equal(t, resp.Key.Desc, vzresp.Key.Desc)
}

func TestAPIKeyServer_Delete(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, mockClients, cleanup := testutils.CreateTestAPIEnv(t)
	defer cleanup()
	ctx := CreateTestContext()

	id := utils.ProtoFromUUIDStrOrNil("6ba7b810-9dad-11d1-80b4-00c04fd430c8")
	vzresp := &types.Empty{}
	mockClients.MockAPIKey.EXPECT().
		Delete(gomock.Any(), id).Return(vzresp, nil)

	vzAPIKeyServer := &controller.APIKeyServer{
		APIKeyClient: mockClients.MockAPIKey,
	}
	resp, err := vzAPIKeyServer.Delete(ctx, id)
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, resp, vzresp)
}

func TestAutocompleteService_Autocomplete(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orgID, err := uuid.FromString("6ba7b810-9dad-11d1-80b4-00c04fd430c8")
	require.NoError(t, err)
	ctx := CreateTestContext()

	s := mock_autocomplete.NewMockSuggester(ctrl)

	requests := [][]*autocomplete.SuggestionRequest{
		{
			{
				OrgID:        orgID,
				ClusterUID:   "test",
				Input:        "px/svc_info",
				AllowedKinds: []cloudpb.AutocompleteEntityKind{cloudpb.AEK_POD, cloudpb.AEK_SVC, cloudpb.AEK_NAMESPACE, cloudpb.AEK_SCRIPT},
				AllowedArgs:  []cloudpb.AutocompleteEntityKind{},
			},
			{
				OrgID:        orgID,
				ClusterUID:   "test",
				Input:        "pl/test",
				AllowedKinds: []cloudpb.AutocompleteEntityKind{cloudpb.AEK_POD, cloudpb.AEK_SVC, cloudpb.AEK_NAMESPACE, cloudpb.AEK_SCRIPT},
				AllowedArgs:  []cloudpb.AutocompleteEntityKind{},
			},
		},
	}

	responses := [][]*autocomplete.SuggestionResult{
		{
			{
				Suggestions: []*autocomplete.Suggestion{
					{
						Name:     "px/svc_info",
						Score:    1,
						ArgNames: []string{"svc_name"},
						ArgKinds: []cloudpb.AutocompleteEntityKind{cloudpb.AEK_SVC},
					},
				},
				ExactMatch: true,
			},
			{
				Suggestions: []*autocomplete.Suggestion{
					{
						Name:  "px/test",
						Score: 1,
					},
				},
				ExactMatch: true,
			},
		},
	}

	suggestionCalls := 0
	s.EXPECT().
		GetSuggestions(gomock.Any()).
		DoAndReturn(func(req []*autocomplete.SuggestionRequest) ([]*autocomplete.SuggestionResult, error) {
			assert.ElementsMatch(t, requests[suggestionCalls], req)
			resp := responses[suggestionCalls]
			suggestionCalls++
			return resp, nil
		}).
		Times(len(requests))

	autocompleteServer := &controller.AutocompleteServer{
		Suggester: s,
	}

	resp, err := autocompleteServer.Autocomplete(ctx, &cloudpb.AutocompleteRequest{
		Input:      "px/svc_info pl/test",
		CursorPos:  0,
		Action:     cloudpb.AAT_EDIT,
		ClusterUID: "test",
	})
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "${2:$0px/svc_info} ${1:pl/test}", resp.FormattedInput)
	assert.False(t, resp.IsExecutable)
	assert.Equal(t, 2, len(resp.TabSuggestions))
}

func TestAutocompleteService_AutocompleteField(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orgID, err := uuid.FromString("6ba7b810-9dad-11d1-80b4-00c04fd430c8")
	require.NoError(t, err)
	ctx := CreateTestContext()

	s := mock_autocomplete.NewMockSuggester(ctrl)

	requests := [][]*autocomplete.SuggestionRequest{
		{
			{
				OrgID:        orgID,
				ClusterUID:   "test",
				Input:        "px/svc_info",
				AllowedKinds: []cloudpb.AutocompleteEntityKind{cloudpb.AEK_SVC},
				AllowedArgs:  []cloudpb.AutocompleteEntityKind{},
			},
		},
	}

	responses := []*autocomplete.SuggestionResult{
		{
			Suggestions: []*autocomplete.Suggestion{
				{
					Name:  "px/svc_info",
					Score: 1,
					State: cloudpb.AES_RUNNING,
				},
				{
					Name:  "px/svc_info2",
					Score: 1,
					State: cloudpb.AES_TERMINATED,
				},
			},
			ExactMatch: true,
		},
	}

	s.EXPECT().
		GetSuggestions(gomock.Any()).
		DoAndReturn(func(req []*autocomplete.SuggestionRequest) ([]*autocomplete.SuggestionResult, error) {
			assert.ElementsMatch(t, requests[0], req)
			return responses, nil
		})

	autocompleteServer := &controller.AutocompleteServer{
		Suggester: s,
	}

	resp, err := autocompleteServer.AutocompleteField(ctx, &cloudpb.AutocompleteFieldRequest{
		Input:      "px/svc_info",
		FieldType:  cloudpb.AEK_SVC,
		ClusterUID: "test",
	})
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 2, len(resp.Suggestions))
}

func toAny(t *testing.T, msg proto.Message) *types.Any {
	any, err := types.MarshalAny(msg)
	require.NoError(t, err)
	return any
}

func TestScriptMgr(t *testing.T) {
	var testVis = &vispb.Vis{
		Widgets: []*vispb.Widget{
			{
				FuncOrRef: &vispb.Widget_Func_{
					Func: &vispb.Widget_Func{
						Name: "my_func",
					},
				},
				DisplaySpec: toAny(t, &vispb.VegaChart{
					Spec: "{}",
				}),
			},
		},
	}

	ID1 := uuid.Must(uuid.NewV4())
	ID2 := uuid.Must(uuid.NewV4())
	testCases := []struct {
		name         string
		endpoint     string
		smReq        proto.Message
		smResp       proto.Message
		req          proto.Message
		expectedResp proto.Message
	}{
		{
			name:     "GetLiveViews correctly translates from scriptmgrpb to cloudpb.",
			endpoint: "GetLiveViews",
			smReq:    &scriptmgrpb.GetLiveViewsReq{},
			smResp: &scriptmgrpb.GetLiveViewsResp{
				LiveViews: []*scriptmgrpb.LiveViewMetadata{
					{
						ID:   utils.ProtoFromUUID(ID1),
						Name: "liveview1",
						Desc: "liveview1 desc",
					},
					{
						ID:   utils.ProtoFromUUID(ID2),
						Name: "liveview2",
						Desc: "liveview2 desc",
					},
				},
			},
			req: &cloudpb.GetLiveViewsReq{},
			expectedResp: &cloudpb.GetLiveViewsResp{
				LiveViews: []*cloudpb.LiveViewMetadata{
					{
						ID:   ID1.String(),
						Name: "liveview1",
						Desc: "liveview1 desc",
					},
					{
						ID:   ID2.String(),
						Name: "liveview2",
						Desc: "liveview2 desc",
					},
				},
			},
		},
		{
			name:     "GetLiveViewContents correctly translates between scriptmgr and cloudpb.",
			endpoint: "GetLiveViewContents",
			smReq: &scriptmgrpb.GetLiveViewContentsReq{
				LiveViewID: utils.ProtoFromUUID(ID1),
			},
			smResp: &scriptmgrpb.GetLiveViewContentsResp{
				Metadata: &scriptmgrpb.LiveViewMetadata{
					ID:   utils.ProtoFromUUID(ID1),
					Name: "liveview1",
					Desc: "liveview1 desc",
				},
				PxlContents: "liveview1 pxl",
				Vis:         testVis,
			},
			req: &cloudpb.GetLiveViewContentsReq{
				LiveViewID: ID1.String(),
			},
			expectedResp: &cloudpb.GetLiveViewContentsResp{
				Metadata: &cloudpb.LiveViewMetadata{
					ID:   ID1.String(),
					Name: "liveview1",
					Desc: "liveview1 desc",
				},
				PxlContents: "liveview1 pxl",
				Vis:         testVis,
			},
		},
		{
			name:     "GetScripts correctly translates between scriptmgr and cloudpb.",
			endpoint: "GetScripts",
			smReq:    &scriptmgrpb.GetScriptsReq{},
			smResp: &scriptmgrpb.GetScriptsResp{
				Scripts: []*scriptmgrpb.ScriptMetadata{
					{
						ID:          utils.ProtoFromUUID(ID1),
						Name:        "script1",
						Desc:        "script1 desc",
						HasLiveView: false,
					},
					{
						ID:          utils.ProtoFromUUID(ID2),
						Name:        "liveview1",
						Desc:        "liveview1 desc",
						HasLiveView: true,
					},
				},
			},
			req: &cloudpb.GetScriptsReq{},
			expectedResp: &cloudpb.GetScriptsResp{
				Scripts: []*cloudpb.ScriptMetadata{
					{
						ID:          ID1.String(),
						Name:        "script1",
						Desc:        "script1 desc",
						HasLiveView: false,
					},
					{
						ID:          ID2.String(),
						Name:        "liveview1",
						Desc:        "liveview1 desc",
						HasLiveView: true,
					},
				},
			},
		},
		{
			name:     "GetScriptContents correctly translates between scriptmgr and cloudpb.",
			endpoint: "GetScriptContents",
			smReq: &scriptmgrpb.GetScriptContentsReq{
				ScriptID: utils.ProtoFromUUID(ID1),
			},
			smResp: &scriptmgrpb.GetScriptContentsResp{
				Metadata: &scriptmgrpb.ScriptMetadata{
					ID:          utils.ProtoFromUUID(ID1),
					Name:        "Script1",
					Desc:        "Script1 desc",
					HasLiveView: false,
				},
				Contents: "Script1 pxl",
			},
			req: &cloudpb.GetScriptContentsReq{
				ScriptID: ID1.String(),
			},
			expectedResp: &cloudpb.GetScriptContentsResp{
				Metadata: &cloudpb.ScriptMetadata{
					ID:          ID1.String(),
					Name:        "Script1",
					Desc:        "Script1 desc",
					HasLiveView: false,
				},
				Contents: "Script1 pxl",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockScriptMgr := mock_scriptmgr.NewMockScriptMgrServiceClient(ctrl)
			ctx := CreateTestContext()

			reflect.ValueOf(mockScriptMgr.EXPECT()).
				MethodByName(tc.endpoint).
				Call([]reflect.Value{
					reflect.ValueOf(gomock.Any()),
					reflect.ValueOf(tc.smReq),
				})[0].Interface().(*gomock.Call).
				Return(tc.smResp, nil)

			scriptMgrServer := &controller.ScriptMgrServer{
				ScriptMgr: mockScriptMgr,
			}

			returnVals := reflect.ValueOf(scriptMgrServer).
				MethodByName(tc.endpoint).
				Call([]reflect.Value{
					reflect.ValueOf(ctx),
					reflect.ValueOf(tc.req),
				})
			assert.Nil(t, returnVals[1].Interface())
			resp := returnVals[0].Interface().(proto.Message)

			assert.Equal(t, tc.expectedResp, resp)
		})
	}
}

func TestProfileServer_GetOrgInfo(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	orgID := utils.ProtoFromUUIDStrOrNil("6ba7b810-9dad-11d1-80b4-00c04fd430c8")

	_, mockClients, cleanup := testutils.CreateTestAPIEnv(t)
	defer cleanup()
	ctx := CreateTestContext()

	mockClients.MockProfile.EXPECT().GetOrg(gomock.Any(), orgID).
		Return(&profilepb.OrgInfo{
			OrgName: "someOrg",
			ID:      orgID,
		}, nil)

	profileServer := &controller.ProfileServer{mockClients.MockProfile}

	resp, err := profileServer.GetOrgInfo(ctx, orgID)

	require.NoError(t, err)
	assert.Equal(t, "someOrg", resp.OrgName)
	assert.Equal(t, orgID, resp.ID)
}

func TestOrganizationServiceServer_InviteUser(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, mockClients, cleanup := testutils.CreateTestAPIEnv(t)
	defer cleanup()
	ctx := CreateTestContext()
	mockReq := &profilepb.InviteUserRequest{
		OrgID:          utils.ProtoFromUUIDStrOrNil("6ba7b810-9dad-11d1-80b4-00c04fd430c8"),
		MustCreateUser: true,
		Email:          "bobloblaw@lawblog.law",
		FirstName:      "bob",
		LastName:       "loblaw",
	}

	mockClients.MockProfile.EXPECT().InviteUser(gomock.Any(), mockReq).
		Return(&profilepb.InviteUserResponse{
			Email:      "bobloblaw@lawblog.law",
			InviteLink: "withpixie.ai/invite&id=abcd",
		}, nil)

	os := &controller.OrganizationServiceServer{mockClients.MockProfile}

	resp, err := os.InviteUser(ctx, &cloudpb.InviteUserRequest{
		Email:     "bobloblaw@lawblog.law",
		FirstName: "bob",
		LastName:  "loblaw",
	})

	require.NoError(t, err)
	assert.Equal(t, mockReq.Email, resp.Email)
	assert.Equal(t, "withpixie.ai/invite&id=abcd", resp.InviteLink)
}
