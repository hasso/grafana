package librarypanels

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"gopkg.in/macaron.v1"

	"github.com/grafana/grafana/pkg/api/response"
	"github.com/grafana/grafana/pkg/bus"
	"github.com/grafana/grafana/pkg/components/simplejson"
	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/registry"
	"github.com/grafana/grafana/pkg/services/dashboards"
	"github.com/grafana/grafana/pkg/services/sqlstore"
	"github.com/grafana/grafana/pkg/setting"
)

func TestCreateLibraryPanel(t *testing.T) {
	scenarioWithLibraryPanel(t, "When an admin tries to create a library panel that already exists, it should fail",
		func(t *testing.T, sc scenarioContext) {
			command := getCreateCommand(sc.folder.Id, "Text - Library Panel")
			resp := sc.service.createHandler(sc.reqContext, command)
			require.Equal(t, 400, resp.Status())
		})

	scenarioWithLibraryPanel(t, "When an admin tries to create a library panel that does not exists, it should succeed",
		func(t *testing.T, sc scenarioContext) {
			var expected = libraryPanelResult{
				Result: libraryPanel{
					ID:       1,
					OrgID:    1,
					FolderID: 1,
					UID:      sc.initialResult.Result.UID,
					Name:     "Text - Library Panel",
					Model: map[string]interface{}{
						"datasource": "${DS_GDEV-TESTDATA}",
						"id":         float64(1),
						"title":      "Text - Library Panel",
						"type":       "text",
					},
					Meta: LibraryPanelDTOMeta{
						CanEdit:             true,
						ConnectedDashboards: 0,
						Created:             sc.initialResult.Result.Meta.Created,
						Updated:             sc.initialResult.Result.Meta.Updated,
						CreatedBy: LibraryPanelDTOMetaUser{
							ID:        1,
							Name:      "signed_in_user",
							AvatarUrl: "/avatar/37524e1eb8b3e32850b57db0a19af93b",
						},
						UpdatedBy: LibraryPanelDTOMetaUser{
							ID:        1,
							Name:      "signed_in_user",
							AvatarUrl: "/avatar/37524e1eb8b3e32850b57db0a19af93b",
						},
					},
				},
			}
			if diff := cmp.Diff(expected, sc.initialResult, getCompareOptions()...); diff != "" {
				t.Fatalf("Result mismatch (-want +got):\n%s", diff)
			}
		})

	testScenario(t, "When an admin tries to create a library panel where name and panel title differ, it should update panel title",
		func(t *testing.T, sc scenarioContext) {
			command := getCreateCommand(1, "Library Panel Name")
			resp := sc.service.createHandler(sc.reqContext, command)
			var result = validateAndUnMarshalResponse(t, resp)
			var expected = libraryPanelResult{
				Result: libraryPanel{
					ID:       1,
					OrgID:    1,
					FolderID: 1,
					UID:      result.Result.UID,
					Name:     "Library Panel Name",
					Model: map[string]interface{}{
						"datasource": "${DS_GDEV-TESTDATA}",
						"id":         float64(1),
						"title":      "Library Panel Name",
						"type":       "text",
					},
					Meta: LibraryPanelDTOMeta{
						CanEdit:             true,
						ConnectedDashboards: 0,
						Created:             result.Result.Meta.Created,
						Updated:             result.Result.Meta.Updated,
						CreatedBy: LibraryPanelDTOMetaUser{
							ID:        1,
							Name:      "signed_in_user",
							AvatarUrl: "/avatar/37524e1eb8b3e32850b57db0a19af93b",
						},
						UpdatedBy: LibraryPanelDTOMetaUser{
							ID:        1,
							Name:      "signed_in_user",
							AvatarUrl: "/avatar/37524e1eb8b3e32850b57db0a19af93b",
						},
					},
				},
			}
			if diff := cmp.Diff(expected, result, getCompareOptions()...); diff != "" {
				t.Fatalf("Result mismatch (-want +got):\n%s", diff)
			}
		})
}

func TestConnectLibraryPanel(t *testing.T) {
	scenarioWithLibraryPanel(t, "When an admin tries to create a connection for a library panel that does not exist, it should fail",
		func(t *testing.T, sc scenarioContext) {
			sc.reqContext.ReplaceAllParams(map[string]string{":uid": "unknown", ":dashboardId": "1"})
			resp := sc.service.connectHandler(sc.reqContext)
			require.Equal(t, 404, resp.Status())
		})

	scenarioWithLibraryPanel(t, "When an admin tries to create a connection that already exists, it should succeed",
		func(t *testing.T, sc scenarioContext) {
			sc.reqContext.ReplaceAllParams(map[string]string{":uid": sc.initialResult.Result.UID, ":dashboardId": "1"})
			resp := sc.service.connectHandler(sc.reqContext)
			require.Equal(t, 200, resp.Status())

			resp = sc.service.connectHandler(sc.reqContext)
			require.Equal(t, 200, resp.Status())
		})
}

func TestDeleteLibraryPanel(t *testing.T) {
	scenarioWithLibraryPanel(t, "When an admin tries to delete a library panel that does not exist, it should fail",
		func(t *testing.T, sc scenarioContext) {
			resp := sc.service.deleteHandler(sc.reqContext)
			require.Equal(t, 404, resp.Status())
		})

	scenarioWithLibraryPanel(t, "When an admin tries to delete a library panel that exists, it should succeed",
		func(t *testing.T, sc scenarioContext) {
			sc.reqContext.ReplaceAllParams(map[string]string{":uid": sc.initialResult.Result.UID})
			resp := sc.service.deleteHandler(sc.reqContext)
			require.Equal(t, 200, resp.Status())
		})

	scenarioWithLibraryPanel(t, "When an admin tries to delete a library panel in another org, it should fail",
		func(t *testing.T, sc scenarioContext) {
			sc.reqContext.ReplaceAllParams(map[string]string{":uid": sc.initialResult.Result.UID})
			sc.reqContext.SignedInUser.OrgId = 2
			sc.reqContext.SignedInUser.OrgRole = models.ROLE_ADMIN
			resp := sc.service.deleteHandler(sc.reqContext)
			require.Equal(t, 404, resp.Status())
		})
}

func TestDisconnectLibraryPanel(t *testing.T) {
	scenarioWithLibraryPanel(t, "When an admin tries to remove a connection with a library panel that does not exist, it should fail",
		func(t *testing.T, sc scenarioContext) {
			sc.reqContext.ReplaceAllParams(map[string]string{":uid": "unknown", ":dashboardId": "1"})
			resp := sc.service.disconnectHandler(sc.reqContext)
			require.Equal(t, 404, resp.Status())
		})

	scenarioWithLibraryPanel(t, "When an admin tries to remove a connection that does not exist, it should fail",
		func(t *testing.T, sc scenarioContext) {
			sc.reqContext.ReplaceAllParams(map[string]string{":uid": sc.initialResult.Result.UID, ":dashboardId": "1"})
			resp := sc.service.disconnectHandler(sc.reqContext)
			require.Equal(t, 404, resp.Status())
		})

	scenarioWithLibraryPanel(t, "When an admin tries to remove a connection that does exist, it should succeed",
		func(t *testing.T, sc scenarioContext) {
			sc.reqContext.ReplaceAllParams(map[string]string{":uid": sc.initialResult.Result.UID, ":dashboardId": "1"})
			resp := sc.service.connectHandler(sc.reqContext)
			require.Equal(t, 200, resp.Status())
			resp = sc.service.disconnectHandler(sc.reqContext)
			require.Equal(t, 200, resp.Status())
		})
}

func TestGetLibraryPanel(t *testing.T) {
	scenarioWithLibraryPanel(t, "When an admin tries to get a library panel that does not exist, it should fail",
		func(t *testing.T, sc scenarioContext) {
			sc.reqContext.ReplaceAllParams(map[string]string{":uid": "unknown"})
			resp := sc.service.getHandler(sc.reqContext)
			require.Equal(t, 404, resp.Status())
		})

	scenarioWithLibraryPanel(t, "When an admin tries to get a library panel that exists, it should succeed and return correct result",
		func(t *testing.T, sc scenarioContext) {
			sc.reqContext.ReplaceAllParams(map[string]string{":uid": sc.initialResult.Result.UID})
			resp := sc.service.getHandler(sc.reqContext)
			var result = validateAndUnMarshalResponse(t, resp)
			var expected = libraryPanelResult{
				Result: libraryPanel{
					ID:       1,
					OrgID:    1,
					FolderID: 1,
					UID:      result.Result.UID,
					Name:     "Text - Library Panel",
					Model: map[string]interface{}{
						"datasource": "${DS_GDEV-TESTDATA}",
						"id":         float64(1),
						"title":      "Text - Library Panel",
						"type":       "text",
					},
					Meta: LibraryPanelDTOMeta{
						CanEdit:             true,
						ConnectedDashboards: 0,
						Created:             result.Result.Meta.Created,
						Updated:             result.Result.Meta.Updated,
						CreatedBy: LibraryPanelDTOMetaUser{
							ID:        1,
							Name:      "user_in_db",
							AvatarUrl: "/avatar/402d08de060496d6b6874495fe20f5ad",
						},
						UpdatedBy: LibraryPanelDTOMetaUser{
							ID:        1,
							Name:      "user_in_db",
							AvatarUrl: "/avatar/402d08de060496d6b6874495fe20f5ad",
						},
					},
				},
			}
			if diff := cmp.Diff(expected, result, getCompareOptions()...); diff != "" {
				t.Fatalf("Result mismatch (-want +got):\n%s", diff)
			}
		})

	scenarioWithLibraryPanel(t, "When an admin tries to get a library panel that exists in an other org, it should fail",
		func(t *testing.T, sc scenarioContext) {
			sc.reqContext.ReplaceAllParams(map[string]string{":uid": sc.initialResult.Result.UID})
			sc.reqContext.SignedInUser.OrgId = 2
			sc.reqContext.SignedInUser.OrgRole = models.ROLE_ADMIN
			resp := sc.service.getHandler(sc.reqContext)
			require.Equal(t, 404, resp.Status())
		})

	scenarioWithLibraryPanel(t, "When an admin tries to get a library panel with 2 connected dashboards, it should succeed and return correct connected dashboards",
		func(t *testing.T, sc scenarioContext) {
			sc.reqContext.ReplaceAllParams(map[string]string{":uid": sc.initialResult.Result.UID, ":dashboardId": "1"})
			resp := sc.service.connectHandler(sc.reqContext)
			require.Equal(t, 200, resp.Status())
			sc.reqContext.ReplaceAllParams(map[string]string{":uid": sc.initialResult.Result.UID, ":dashboardId": "2"})
			resp = sc.service.connectHandler(sc.reqContext)
			require.Equal(t, 200, resp.Status())

			sc.reqContext.ReplaceAllParams(map[string]string{":uid": sc.initialResult.Result.UID})
			resp = sc.service.getHandler(sc.reqContext)
			require.Equal(t, 200, resp.Status())
			var result = validateAndUnMarshalResponse(t, resp)
			require.Equal(t, int64(2), result.Result.Meta.ConnectedDashboards)
		})
}

func TestGetAllLibraryPanels(t *testing.T) {
	testScenario(t, "When an admin tries to get all library panels and none exists, it should return none",
		func(t *testing.T, sc scenarioContext) {
			resp := sc.service.getAllHandler(sc.reqContext)
			require.Equal(t, 200, resp.Status())

			var result libraryPanelsResult
			err := json.Unmarshal(resp.Body(), &result)
			require.NoError(t, err)
			require.NotNil(t, result.Result)
			require.Equal(t, 0, len(result.Result))
		})

	scenarioWithLibraryPanel(t, "When an admin tries to get all library panels and two exist, it should succeed",
		func(t *testing.T, sc scenarioContext) {
			command := getCreateCommand(sc.folder.Id, "Text - Library Panel2")
			resp := sc.service.createHandler(sc.reqContext, command)
			require.Equal(t, 200, resp.Status())

			resp = sc.service.getAllHandler(sc.reqContext)
			require.Equal(t, 200, resp.Status())

			var result libraryPanelsResult
			err := json.Unmarshal(resp.Body(), &result)
			require.NoError(t, err)
			var expected = libraryPanelsResult{
				Result: []libraryPanel{
					{
						ID:       1,
						OrgID:    1,
						FolderID: 1,
						UID:      result.Result[0].UID,
						Name:     "Text - Library Panel",
						Model: map[string]interface{}{
							"datasource": "${DS_GDEV-TESTDATA}",
							"id":         float64(1),
							"title":      "Text - Library Panel",
							"type":       "text",
						},
						Meta: LibraryPanelDTOMeta{
							CanEdit:             true,
							ConnectedDashboards: 0,
							Created:             result.Result[0].Meta.Created,
							Updated:             result.Result[0].Meta.Updated,
							CreatedBy: LibraryPanelDTOMetaUser{
								ID:        1,
								Name:      "user_in_db",
								AvatarUrl: "/avatar/402d08de060496d6b6874495fe20f5ad",
							},
							UpdatedBy: LibraryPanelDTOMetaUser{
								ID:        1,
								Name:      "user_in_db",
								AvatarUrl: "/avatar/402d08de060496d6b6874495fe20f5ad",
							},
						},
					},
					{
						ID:       2,
						OrgID:    1,
						FolderID: 1,
						UID:      result.Result[1].UID,
						Name:     "Text - Library Panel2",
						Model: map[string]interface{}{
							"datasource": "${DS_GDEV-TESTDATA}",
							"id":         float64(1),
							"title":      "Text - Library Panel2",
							"type":       "text",
						},
						Meta: LibraryPanelDTOMeta{
							CanEdit:             true,
							ConnectedDashboards: 0,
							Created:             result.Result[1].Meta.Created,
							Updated:             result.Result[1].Meta.Updated,
							CreatedBy: LibraryPanelDTOMetaUser{
								ID:        1,
								Name:      "user_in_db",
								AvatarUrl: "/avatar/402d08de060496d6b6874495fe20f5ad",
							},
							UpdatedBy: LibraryPanelDTOMetaUser{
								ID:        1,
								Name:      "user_in_db",
								AvatarUrl: "/avatar/402d08de060496d6b6874495fe20f5ad",
							},
						},
					},
				},
			}
			if diff := cmp.Diff(expected, result, getCompareOptions()...); diff != "" {
				t.Fatalf("Result mismatch (-want +got):\n%s", diff)
			}
		})

	scenarioWithLibraryPanel(t, "When an admin tries to get all library panels and two exist but only one is connected, it should succeed and return correct connected dashboards",
		func(t *testing.T, sc scenarioContext) {
			command := getCreateCommand(sc.folder.Id, "Text - Library Panel2")
			resp := sc.service.createHandler(sc.reqContext, command)
			require.Equal(t, 200, resp.Status())
			var result = validateAndUnMarshalResponse(t, resp)

			sc.reqContext.ReplaceAllParams(map[string]string{":uid": result.Result.UID, ":dashboardId": "1"})
			resp = sc.service.connectHandler(sc.reqContext)
			require.Equal(t, 200, resp.Status())
			sc.reqContext.ReplaceAllParams(map[string]string{":uid": result.Result.UID, ":dashboardId": "2"})
			resp = sc.service.connectHandler(sc.reqContext)
			require.Equal(t, 200, resp.Status())

			resp = sc.service.getAllHandler(sc.reqContext)
			require.Equal(t, 200, resp.Status())

			var results libraryPanelsResult
			err := json.Unmarshal(resp.Body(), &results)
			require.NoError(t, err)
			require.Equal(t, int64(0), results.Result[0].Meta.ConnectedDashboards)
			require.Equal(t, int64(2), results.Result[1].Meta.ConnectedDashboards)
		})

	scenarioWithLibraryPanel(t, "When an admin tries to get all library panels in a different org, none should be returned",
		func(t *testing.T, sc scenarioContext) {
			resp := sc.service.getAllHandler(sc.reqContext)
			require.Equal(t, 200, resp.Status())

			var result libraryPanelsResult
			err := json.Unmarshal(resp.Body(), &result)
			require.NoError(t, err)
			require.Equal(t, 1, len(result.Result))
			require.Equal(t, int64(1), result.Result[0].FolderID)
			require.Equal(t, "Text - Library Panel", result.Result[0].Name)

			sc.reqContext.SignedInUser.OrgId = 2
			sc.reqContext.SignedInUser.OrgRole = models.ROLE_ADMIN
			resp = sc.service.getAllHandler(sc.reqContext)
			require.Equal(t, 200, resp.Status())

			result = libraryPanelsResult{}
			err = json.Unmarshal(resp.Body(), &result)
			require.NoError(t, err)
			require.NotNil(t, result.Result)
			require.Equal(t, 0, len(result.Result))
		})

	testScenario(t, "When an user tries to get all library panels, library panels in folders where the user has no access should not be returned",
		func(t *testing.T, sc scenarioContext) {
			updateFolderACL(t, sc, models.ROLE_EDITOR, models.PERMISSION_EDIT)

			command := getCreateCommand(sc.folder.Id, "Editor - Library Panel")
			resp := sc.service.createHandler(sc.reqContext, command)
			require.Equal(t, 200, resp.Status())

			cmd := models.CreateFolderCommand{
				Uid:   "AdminOnlyFolder",
				Title: "Admin Only Folder",
			}
			createFolder(t, &sc, &cmd)
			updateFolderACL(t, sc, models.ROLE_ADMIN, models.PERMISSION_ADMIN)

			command = getCreateCommand(sc.folder.Id, "Admin - Library Panel")
			resp = sc.service.createHandler(sc.reqContext, command)
			require.Equal(t, 200, resp.Status())

			resp = sc.service.getAllHandler(sc.reqContext)
			require.Equal(t, 200, resp.Status())

			var result libraryPanelsResult
			err := json.Unmarshal(resp.Body(), &result)
			require.NoError(t, err)
			require.Equal(t, 2, len(result.Result))
			require.Equal(t, int64(1), result.Result[0].FolderID)
			require.Equal(t, int64(2), cmd.Result.Id)
			require.Equal(t, "Editor - Library Panel", result.Result[0].Name)
			require.Equal(t, "Admin - Library Panel", result.Result[1].Name)

			sc.reqContext.SignedInUser.OrgRole = models.ROLE_EDITOR
			resp = sc.service.getAllHandler(sc.reqContext)
			require.Equal(t, 200, resp.Status())

			result = libraryPanelsResult{}
			err = json.Unmarshal(resp.Body(), &result)
			require.NoError(t, err)
			require.Equal(t, 1, len(result.Result))
			require.Equal(t, int64(1), result.Result[0].FolderID)
			require.Equal(t, "Editor - Library Panel", result.Result[0].Name)

			sc.reqContext.SignedInUser.OrgRole = models.ROLE_VIEWER
			resp = sc.service.getAllHandler(sc.reqContext)
			require.Equal(t, 200, resp.Status())

			result = libraryPanelsResult{}
			err = json.Unmarshal(resp.Body(), &result)
			require.NoError(t, err)
			require.NotNil(t, result.Result)
			require.Equal(t, 0, len(result.Result))
		})
}

func TestGetConnectedDashboards(t *testing.T) {
	scenarioWithLibraryPanel(t, "When an admin tries to get connected dashboards for a library panel that does not exist, it should fail",
		func(t *testing.T, sc scenarioContext) {
			sc.reqContext.ReplaceAllParams(map[string]string{":uid": "unknown"})
			resp := sc.service.getConnectedDashboardsHandler(sc.reqContext)
			require.Equal(t, 404, resp.Status())
		})

	scenarioWithLibraryPanel(t, "When an admin tries to get connected dashboards for a library panel that exists, but has no connections, it should return none",
		func(t *testing.T, sc scenarioContext) {
			sc.reqContext.ReplaceAllParams(map[string]string{":uid": sc.initialResult.Result.UID})
			resp := sc.service.getConnectedDashboardsHandler(sc.reqContext)
			require.Equal(t, 200, resp.Status())

			var dashResult libraryPanelDashboardsResult
			err := json.Unmarshal(resp.Body(), &dashResult)
			require.NoError(t, err)
			require.Equal(t, 0, len(dashResult.Result))
		})

	scenarioWithLibraryPanel(t, "When an admin tries to get connected dashboards for a library panel that exists and has connections, it should return connected dashboard IDs",
		func(t *testing.T, sc scenarioContext) {
			sc.reqContext.ReplaceAllParams(map[string]string{":uid": sc.initialResult.Result.UID, ":dashboardId": "11"})
			resp := sc.service.connectHandler(sc.reqContext)
			require.Equal(t, 200, resp.Status())
			sc.reqContext.ReplaceAllParams(map[string]string{":uid": sc.initialResult.Result.UID, ":dashboardId": "12"})
			resp = sc.service.connectHandler(sc.reqContext)
			require.Equal(t, 200, resp.Status())

			sc.reqContext.ReplaceAllParams(map[string]string{":uid": sc.initialResult.Result.UID})
			resp = sc.service.getConnectedDashboardsHandler(sc.reqContext)
			require.Equal(t, 200, resp.Status())

			var dashResult libraryPanelDashboardsResult
			err := json.Unmarshal(resp.Body(), &dashResult)
			require.NoError(t, err)
			require.Equal(t, 2, len(dashResult.Result))
			require.Equal(t, int64(11), dashResult.Result[0])
			require.Equal(t, int64(12), dashResult.Result[1])
		})
}

func TestPatchLibraryPanel(t *testing.T) {
	scenarioWithLibraryPanel(t, "When an admin tries to patch a library panel that does not exist, it should fail",
		func(t *testing.T, sc scenarioContext) {
			cmd := patchLibraryPanelCommand{}
			sc.reqContext.ReplaceAllParams(map[string]string{":uid": "unknown"})
			resp := sc.service.patchHandler(sc.reqContext, cmd)
			require.Equal(t, 404, resp.Status())
		})

	scenarioWithLibraryPanel(t, "When an admin tries to patch a library panel that exists, it should succeed",
		func(t *testing.T, sc scenarioContext) {
			sc.reqContext.ReplaceAllParams(map[string]string{":uid": sc.initialResult.Result.UID, ":dashboardId": "1"})
			resp := sc.service.connectHandler(sc.reqContext)
			require.Equal(t, 200, resp.Status())
			sc.reqContext.ReplaceAllParams(map[string]string{":uid": sc.initialResult.Result.UID, ":dashboardId": "2"})
			resp = sc.service.connectHandler(sc.reqContext)
			require.Equal(t, 200, resp.Status())

			cmd := patchLibraryPanelCommand{
				FolderID: 2,
				Name:     "Panel - New name",
				Model: []byte(`
								{
								  "datasource": "${DS_GDEV-TESTDATA}",
								  "id": 1,
								  "title": "Model - New name",
								  "type": "text"
								}
							`),
			}
			sc.reqContext.ReplaceAllParams(map[string]string{":uid": sc.initialResult.Result.UID})
			resp = sc.service.patchHandler(sc.reqContext, cmd)
			require.Equal(t, 200, resp.Status())
			var result = validateAndUnMarshalResponse(t, resp)
			var expected = libraryPanelResult{
				Result: libraryPanel{
					ID:       1,
					OrgID:    1,
					FolderID: 2,
					UID:      sc.initialResult.Result.UID,
					Name:     "Panel - New name",
					Model: map[string]interface{}{
						"datasource": "${DS_GDEV-TESTDATA}",
						"id":         float64(1),
						"title":      "Panel - New name",
						"type":       "text",
					},
					Meta: LibraryPanelDTOMeta{
						CanEdit:             true,
						ConnectedDashboards: 2,
						Created:             sc.initialResult.Result.Meta.Created,
						Updated:             result.Result.Meta.Updated,
						CreatedBy: LibraryPanelDTOMetaUser{
							ID:        1,
							Name:      "user_in_db",
							AvatarUrl: "/avatar/402d08de060496d6b6874495fe20f5ad",
						},
						UpdatedBy: LibraryPanelDTOMetaUser{
							ID:        1,
							Name:      "signed_in_user",
							AvatarUrl: "/avatar/37524e1eb8b3e32850b57db0a19af93b",
						},
					},
				},
			}
			if diff := cmp.Diff(expected, result, getCompareOptions()...); diff != "" {
				t.Fatalf("Result mismatch (-want +got):\n%s", diff)
			}
		})

	scenarioWithLibraryPanel(t, "When an admin tries to patch a library panel with folder only, it should change folder successfully and return correct result",
		func(t *testing.T, sc scenarioContext) {
			cmd := patchLibraryPanelCommand{
				FolderID: 100,
			}
			sc.reqContext.ReplaceAllParams(map[string]string{":uid": sc.initialResult.Result.UID})
			resp := sc.service.patchHandler(sc.reqContext, cmd)
			require.Equal(t, 200, resp.Status())
			var result = validateAndUnMarshalResponse(t, resp)
			sc.initialResult.Result.FolderID = int64(100)
			sc.initialResult.Result.Meta.CreatedBy.Name = "user_in_db"
			sc.initialResult.Result.Meta.CreatedBy.AvatarUrl = "/avatar/402d08de060496d6b6874495fe20f5ad"
			if diff := cmp.Diff(sc.initialResult.Result, result.Result, getCompareOptions()...); diff != "" {
				t.Fatalf("Result mismatch (-want +got):\n%s", diff)
			}
		})

	scenarioWithLibraryPanel(t, "When an admin tries to patch a library panel with name only, it should change name successfully, sync title and return correct result",
		func(t *testing.T, sc scenarioContext) {
			cmd := patchLibraryPanelCommand{
				Name: "New Name",
			}
			sc.reqContext.ReplaceAllParams(map[string]string{":uid": sc.initialResult.Result.UID})
			resp := sc.service.patchHandler(sc.reqContext, cmd)
			var result = validateAndUnMarshalResponse(t, resp)
			sc.initialResult.Result.Name = "New Name"
			sc.initialResult.Result.Meta.CreatedBy.Name = "user_in_db"
			sc.initialResult.Result.Meta.CreatedBy.AvatarUrl = "/avatar/402d08de060496d6b6874495fe20f5ad"
			sc.initialResult.Result.Model["title"] = "New Name"
			if diff := cmp.Diff(sc.initialResult.Result, result.Result, getCompareOptions()...); diff != "" {
				t.Fatalf("Result mismatch (-want +got):\n%s", diff)
			}
		})

	scenarioWithLibraryPanel(t, "When an admin tries to patch a library panel with model only, it should change model successfully and return correct result",
		func(t *testing.T, sc scenarioContext) {
			cmd := patchLibraryPanelCommand{
				Model: []byte(`{ "title": "New Model Title", "name": "New Model Name" }`),
			}
			sc.reqContext.ReplaceAllParams(map[string]string{":uid": sc.initialResult.Result.UID})
			resp := sc.service.patchHandler(sc.reqContext, cmd)
			var result = validateAndUnMarshalResponse(t, resp)
			sc.initialResult.Result.Model = map[string]interface{}{
				"title": "Text - Library Panel",
				"name":  "New Model Name",
			}
			sc.initialResult.Result.Meta.CreatedBy.Name = "user_in_db"
			sc.initialResult.Result.Meta.CreatedBy.AvatarUrl = "/avatar/402d08de060496d6b6874495fe20f5ad"
			if diff := cmp.Diff(sc.initialResult.Result, result.Result, getCompareOptions()...); diff != "" {
				t.Fatalf("Result mismatch (-want +got):\n%s", diff)
			}
		})

	scenarioWithLibraryPanel(t, "When another admin tries to patch a library panel, it should change UpdatedBy successfully and return correct result",
		func(t *testing.T, sc scenarioContext) {
			cmd := patchLibraryPanelCommand{}
			sc.reqContext.UserId = 2
			sc.reqContext.ReplaceAllParams(map[string]string{":uid": sc.initialResult.Result.UID})
			resp := sc.service.patchHandler(sc.reqContext, cmd)
			var result = validateAndUnMarshalResponse(t, resp)
			sc.initialResult.Result.Meta.UpdatedBy.ID = int64(2)
			sc.initialResult.Result.Meta.CreatedBy.Name = "user_in_db"
			sc.initialResult.Result.Meta.CreatedBy.AvatarUrl = "/avatar/402d08de060496d6b6874495fe20f5ad"
			if diff := cmp.Diff(sc.initialResult.Result, result.Result, getCompareOptions()...); diff != "" {
				t.Fatalf("Result mismatch (-want +got):\n%s", diff)
			}
		})

	scenarioWithLibraryPanel(t, "When an admin tries to patch a library panel with a name that already exists, it should fail",
		func(t *testing.T, sc scenarioContext) {
			command := getCreateCommand(sc.folder.Id, "Another Panel")
			resp := sc.service.createHandler(sc.reqContext, command)
			var result = validateAndUnMarshalResponse(t, resp)
			cmd := patchLibraryPanelCommand{
				Name: "Text - Library Panel",
			}
			sc.reqContext.ReplaceAllParams(map[string]string{":uid": result.Result.UID})
			resp = sc.service.patchHandler(sc.reqContext, cmd)
			require.Equal(t, 400, resp.Status())
		})

	scenarioWithLibraryPanel(t, "When an admin tries to patch a library panel with a folder where a library panel with the same name already exists, it should fail",
		func(t *testing.T, sc scenarioContext) {
			createFolder(t, &sc, &models.CreateFolderCommand{
				Uid:   "NewTestFolder",
				Title: "New Test Folder",
			})
			command := getCreateCommand(sc.folder.Id, "Text - Library Panel")
			resp := sc.service.createHandler(sc.reqContext, command)
			var result = validateAndUnMarshalResponse(t, resp)
			cmd := patchLibraryPanelCommand{
				FolderID: 1,
			}
			sc.reqContext.ReplaceAllParams(map[string]string{":uid": result.Result.UID})
			resp = sc.service.patchHandler(sc.reqContext, cmd)
			require.Equal(t, 400, resp.Status())
		})

	scenarioWithLibraryPanel(t, "When an admin tries to patch a library panel in another org, it should fail",
		func(t *testing.T, sc scenarioContext) {
			cmd := patchLibraryPanelCommand{
				FolderID: sc.folder.Id,
			}
			sc.reqContext.OrgId = 2
			sc.reqContext.ReplaceAllParams(map[string]string{":uid": sc.initialResult.Result.UID})
			resp := sc.service.patchHandler(sc.reqContext, cmd)
			require.Equal(t, 404, resp.Status())
		})
}

func TestLoadLibraryPanelsForDashboard(t *testing.T) {
	scenarioWithLibraryPanel(t, "When an admin tries to load a dashboard with a library panel, it should copy JSON properties from library panel",
		func(t *testing.T, sc scenarioContext) {
			sc.reqContext.ReplaceAllParams(map[string]string{":uid": sc.initialResult.Result.UID, ":dashboardId": "1"})
			resp := sc.service.connectHandler(sc.reqContext)
			require.Equal(t, 200, resp.Status())

			dashJSON := map[string]interface{}{
				"panels": []interface{}{
					map[string]interface{}{
						"id": int64(1),
						"gridPos": map[string]interface{}{
							"h": 6,
							"w": 6,
							"x": 0,
							"y": 0,
						},
					},
					map[string]interface{}{
						"id": int64(2),
						"gridPos": map[string]interface{}{
							"h": 6,
							"w": 6,
							"x": 6,
							"y": 0,
						},
						"libraryPanel": map[string]interface{}{
							"uid":  sc.initialResult.Result.UID,
							"name": sc.initialResult.Result.Name,
						},
					},
				},
			}
			dash := models.Dashboard{
				Id:   1,
				Data: simplejson.NewFromAny(dashJSON),
			}

			err := sc.service.LoadLibraryPanelsForDashboard(sc.reqContext, &dash)
			require.NoError(t, err)
			expectedJSON := map[string]interface{}{
				"panels": []interface{}{
					map[string]interface{}{
						"id": int64(1),
						"gridPos": map[string]interface{}{
							"h": 6,
							"w": 6,
							"x": 0,
							"y": 0,
						},
					},
					map[string]interface{}{
						"id": int64(2),
						"gridPos": map[string]interface{}{
							"h": 6,
							"w": 6,
							"x": 6,
							"y": 0,
						},
						"datasource": "${DS_GDEV-TESTDATA}",
						"libraryPanel": map[string]interface{}{
							"uid":  sc.initialResult.Result.UID,
							"name": sc.initialResult.Result.Name,
							"meta": map[string]interface{}{
								"canEdit":             false,
								"connectedDashboards": int64(1),
								"created":             sc.initialResult.Result.Meta.Created,
								"updated":             sc.initialResult.Result.Meta.Updated,
								"createdBy": map[string]interface{}{
									"id":        sc.initialResult.Result.Meta.CreatedBy.ID,
									"name":      "user_in_db",
									"avatarUrl": "/avatar/402d08de060496d6b6874495fe20f5ad",
								},
								"updatedBy": map[string]interface{}{
									"id":        sc.initialResult.Result.Meta.UpdatedBy.ID,
									"name":      "user_in_db",
									"avatarUrl": "/avatar/402d08de060496d6b6874495fe20f5ad",
								},
							},
						},
						"title": "Text - Library Panel",
						"type":  "text",
					},
				},
			}
			expected := simplejson.NewFromAny(expectedJSON)
			if diff := cmp.Diff(expected.Interface(), dash.Data.Interface(), getCompareOptions()...); diff != "" {
				t.Fatalf("Result mismatch (-want +got):\n%s", diff)
			}
		})

	scenarioWithLibraryPanel(t, "When an admin tries to load a dashboard with a library panel without uid, it should fail",
		func(t *testing.T, sc scenarioContext) {
			sc.reqContext.ReplaceAllParams(map[string]string{":uid": sc.initialResult.Result.UID, ":dashboardId": "1"})
			resp := sc.service.connectHandler(sc.reqContext)
			require.Equal(t, 200, resp.Status())

			dashJSON := map[string]interface{}{
				"panels": []interface{}{
					map[string]interface{}{
						"id": int64(1),
						"gridPos": map[string]interface{}{
							"h": 6,
							"w": 6,
							"x": 0,
							"y": 0,
						},
					},
					map[string]interface{}{
						"id": int64(2),
						"gridPos": map[string]interface{}{
							"h": 6,
							"w": 6,
							"x": 6,
							"y": 0,
						},
						"libraryPanel": map[string]interface{}{
							"name": sc.initialResult.Result.Name,
						},
					},
				},
			}
			dash := models.Dashboard{
				Id:   1,
				Data: simplejson.NewFromAny(dashJSON),
			}

			err := sc.service.LoadLibraryPanelsForDashboard(sc.reqContext, &dash)
			require.EqualError(t, err, errLibraryPanelHeaderUIDMissing.Error())
		})

	scenarioWithLibraryPanel(t, "When an admin tries to load a dashboard with a library panel that is not connected, it should set correct JSON and continue",
		func(t *testing.T, sc scenarioContext) {
			dashJSON := map[string]interface{}{
				"panels": []interface{}{
					map[string]interface{}{
						"id": int64(1),
						"gridPos": map[string]interface{}{
							"h": 6,
							"w": 6,
							"x": 0,
							"y": 0,
						},
					},
					map[string]interface{}{
						"id": int64(2),
						"gridPos": map[string]interface{}{
							"h": 6,
							"w": 6,
							"x": 6,
							"y": 0,
						},
						"libraryPanel": map[string]interface{}{
							"uid":  sc.initialResult.Result.UID,
							"name": sc.initialResult.Result.Name,
						},
					},
				},
			}
			dash := models.Dashboard{
				Id:   1,
				Data: simplejson.NewFromAny(dashJSON),
			}

			err := sc.service.LoadLibraryPanelsForDashboard(sc.reqContext, &dash)
			require.NoError(t, err)
			expectedJSON := map[string]interface{}{
				"panels": []interface{}{
					map[string]interface{}{
						"id": int64(1),
						"gridPos": map[string]interface{}{
							"h": 6,
							"w": 6,
							"x": 0,
							"y": 0,
						},
					},
					map[string]interface{}{
						"id": int64(2),
						"gridPos": map[string]interface{}{
							"h": 6,
							"w": 6,
							"x": 6,
							"y": 0,
						},
						"libraryPanel": map[string]interface{}{
							"uid":  sc.initialResult.Result.UID,
							"name": sc.initialResult.Result.Name,
						},
						"type": fmt.Sprintf("Name: \"%s\", UID: \"%s\"", sc.initialResult.Result.Name, sc.initialResult.Result.UID),
					},
				},
			}
			expected := simplejson.NewFromAny(expectedJSON)
			if diff := cmp.Diff(expected.Interface(), dash.Data.Interface(), getCompareOptions()...); diff != "" {
				t.Fatalf("Result mismatch (-want +got):\n%s", diff)
			}
		})
}

func TestCleanLibraryPanelsForDashboard(t *testing.T) {
	scenarioWithLibraryPanel(t, "When an admin tries to store a dashboard with a library panel, it should just keep the correct JSON properties in library panel",
		func(t *testing.T, sc scenarioContext) {
			dashJSON := map[string]interface{}{
				"panels": []interface{}{
					map[string]interface{}{
						"id": int64(1),
						"gridPos": map[string]interface{}{
							"h": 6,
							"w": 6,
							"x": 0,
							"y": 0,
						},
					},
					map[string]interface{}{
						"id": int64(2),
						"gridPos": map[string]interface{}{
							"h": 6,
							"w": 6,
							"x": 6,
							"y": 0,
						},
						"datasource": "${DS_GDEV-TESTDATA}",
						"libraryPanel": map[string]interface{}{
							"uid":  sc.initialResult.Result.UID,
							"name": sc.initialResult.Result.Name,
						},
						"title": "Text - Library Panel",
						"type":  "text",
					},
				},
			}
			dash := models.Dashboard{
				Id:   1,
				Data: simplejson.NewFromAny(dashJSON),
			}

			err := sc.service.CleanLibraryPanelsForDashboard(&dash)
			require.NoError(t, err)
			expectedJSON := map[string]interface{}{
				"panels": []interface{}{
					map[string]interface{}{
						"id": int64(1),
						"gridPos": map[string]interface{}{
							"h": 6,
							"w": 6,
							"x": 0,
							"y": 0,
						},
					},
					map[string]interface{}{
						"id": int64(2),
						"gridPos": map[string]interface{}{
							"h": 6,
							"w": 6,
							"x": 6,
							"y": 0,
						},
						"libraryPanel": map[string]interface{}{
							"uid":  sc.initialResult.Result.UID,
							"name": sc.initialResult.Result.Name,
						},
					},
				},
			}
			expected := simplejson.NewFromAny(expectedJSON)
			if diff := cmp.Diff(expected.Interface(), dash.Data.Interface(), getCompareOptions()...); diff != "" {
				t.Fatalf("Result mismatch (-want +got):\n%s", diff)
			}
		})

	scenarioWithLibraryPanel(t, "When an admin tries to store a dashboard with a library panel without uid, it should fail",
		func(t *testing.T, sc scenarioContext) {
			dashJSON := map[string]interface{}{
				"panels": []interface{}{
					map[string]interface{}{
						"id": int64(1),
						"gridPos": map[string]interface{}{
							"h": 6,
							"w": 6,
							"x": 0,
							"y": 0,
						},
					},
					map[string]interface{}{
						"id": int64(2),
						"gridPos": map[string]interface{}{
							"h": 6,
							"w": 6,
							"x": 6,
							"y": 0,
						},
						"datasource": "${DS_GDEV-TESTDATA}",
						"libraryPanel": map[string]interface{}{
							"name": sc.initialResult.Result.Name,
						},
						"title": "Text - Library Panel",
						"type":  "text",
					},
				},
			}
			dash := models.Dashboard{
				Id:   1,
				Data: simplejson.NewFromAny(dashJSON),
			}

			err := sc.service.CleanLibraryPanelsForDashboard(&dash)
			require.EqualError(t, err, errLibraryPanelHeaderUIDMissing.Error())
		})

	scenarioWithLibraryPanel(t, "When an admin tries to store a dashboard with a library panel without name, it should fail",
		func(t *testing.T, sc scenarioContext) {
			dashJSON := map[string]interface{}{
				"panels": []interface{}{
					map[string]interface{}{
						"id": int64(1),
						"gridPos": map[string]interface{}{
							"h": 6,
							"w": 6,
							"x": 0,
							"y": 0,
						},
					},
					map[string]interface{}{
						"id": int64(2),
						"gridPos": map[string]interface{}{
							"h": 6,
							"w": 6,
							"x": 6,
							"y": 0,
						},
						"datasource": "${DS_GDEV-TESTDATA}",
						"libraryPanel": map[string]interface{}{
							"uid": sc.initialResult.Result.UID,
						},
						"title": "Text - Library Panel",
						"type":  "text",
					},
				},
			}
			dash := models.Dashboard{
				Id:   1,
				Data: simplejson.NewFromAny(dashJSON),
			}

			err := sc.service.CleanLibraryPanelsForDashboard(&dash)
			require.EqualError(t, err, errLibraryPanelHeaderNameMissing.Error())
		})
}

func TestConnectLibraryPanelsForDashboard(t *testing.T) {
	scenarioWithLibraryPanel(t, "When an admin tries to store a dashboard with a library panel, it should connect the two",
		func(t *testing.T, sc scenarioContext) {
			dashJSON := map[string]interface{}{
				"panels": []interface{}{
					map[string]interface{}{
						"id": int64(1),
						"gridPos": map[string]interface{}{
							"h": 6,
							"w": 6,
							"x": 0,
							"y": 0,
						},
					},
					map[string]interface{}{
						"id": int64(2),
						"gridPos": map[string]interface{}{
							"h": 6,
							"w": 6,
							"x": 6,
							"y": 0,
						},
						"datasource": "${DS_GDEV-TESTDATA}",
						"libraryPanel": map[string]interface{}{
							"uid":  sc.initialResult.Result.UID,
							"name": sc.initialResult.Result.Name,
						},
						"title": "Text - Library Panel",
						"type":  "text",
					},
				},
			}
			dash := models.Dashboard{
				Id:   int64(1),
				Data: simplejson.NewFromAny(dashJSON),
			}

			err := sc.service.ConnectLibraryPanelsForDashboard(sc.reqContext, &dash)
			require.NoError(t, err)

			sc.reqContext.ReplaceAllParams(map[string]string{":uid": sc.initialResult.Result.UID})
			resp := sc.service.getConnectedDashboardsHandler(sc.reqContext)
			require.Equal(t, 200, resp.Status())

			var dashResult libraryPanelDashboardsResult
			err = json.Unmarshal(resp.Body(), &dashResult)
			require.NoError(t, err)
			require.Len(t, dashResult.Result, 1)
			require.Equal(t, int64(1), dashResult.Result[0])
		})

	scenarioWithLibraryPanel(t, "When an admin tries to store a dashboard with a library panel without uid, it should fail",
		func(t *testing.T, sc scenarioContext) {
			dashJSON := map[string]interface{}{
				"panels": []interface{}{
					map[string]interface{}{
						"id": int64(1),
						"gridPos": map[string]interface{}{
							"h": 6,
							"w": 6,
							"x": 0,
							"y": 0,
						},
					},
					map[string]interface{}{
						"id": int64(2),
						"gridPos": map[string]interface{}{
							"h": 6,
							"w": 6,
							"x": 6,
							"y": 0,
						},
						"datasource": "${DS_GDEV-TESTDATA}",
						"libraryPanel": map[string]interface{}{
							"name": sc.initialResult.Result.Name,
						},
						"title": "Text - Library Panel",
						"type":  "text",
					},
				},
			}
			dash := models.Dashboard{
				Id:   int64(1),
				Data: simplejson.NewFromAny(dashJSON),
			}

			err := sc.service.ConnectLibraryPanelsForDashboard(sc.reqContext, &dash)
			require.EqualError(t, err, errLibraryPanelHeaderUIDMissing.Error())
		})

	scenarioWithLibraryPanel(t, "When an admin tries to store a dashboard with unused/removed library panels, it should disconnect unused/removed library panels",
		func(t *testing.T, sc scenarioContext) {
			command := getCreateCommand(sc.folder.Id, "Unused Libray Panel")
			resp := sc.service.createHandler(sc.reqContext, command)
			var unused = validateAndUnMarshalResponse(t, resp)
			sc.reqContext.ReplaceAllParams(map[string]string{":uid": unused.Result.UID, ":dashboardId": "1"})
			resp = sc.service.connectHandler(sc.reqContext)
			require.Equal(t, 200, resp.Status())

			dashJSON := map[string]interface{}{
				"panels": []interface{}{
					map[string]interface{}{
						"id": int64(1),
						"gridPos": map[string]interface{}{
							"h": 6,
							"w": 6,
							"x": 0,
							"y": 0,
						},
					},
					map[string]interface{}{
						"id": int64(2),
						"gridPos": map[string]interface{}{
							"h": 6,
							"w": 6,
							"x": 6,
							"y": 0,
						},
						"datasource": "${DS_GDEV-TESTDATA}",
						"libraryPanel": map[string]interface{}{
							"uid":  sc.initialResult.Result.UID,
							"name": sc.initialResult.Result.Name,
						},
						"title": "Text - Library Panel",
						"type":  "text",
					},
				},
			}
			dash := models.Dashboard{
				Id:   int64(1),
				Data: simplejson.NewFromAny(dashJSON),
			}

			err := sc.service.ConnectLibraryPanelsForDashboard(sc.reqContext, &dash)
			require.NoError(t, err)

			sc.reqContext.ReplaceAllParams(map[string]string{":uid": sc.initialResult.Result.UID})
			resp = sc.service.getConnectedDashboardsHandler(sc.reqContext)
			require.Equal(t, 200, resp.Status())

			var existingResult libraryPanelDashboardsResult
			err = json.Unmarshal(resp.Body(), &existingResult)
			require.NoError(t, err)
			require.Len(t, existingResult.Result, 1)
			require.Equal(t, int64(1), existingResult.Result[0])

			sc.reqContext.ReplaceAllParams(map[string]string{":uid": unused.Result.UID})
			resp = sc.service.getConnectedDashboardsHandler(sc.reqContext)
			require.Equal(t, 200, resp.Status())

			var unusedResult libraryPanelDashboardsResult
			err = json.Unmarshal(resp.Body(), &unusedResult)
			require.NoError(t, err)
			require.Len(t, unusedResult.Result, 0)
		})
}

func TestDisconnectLibraryPanelsForDashboard(t *testing.T) {
	scenarioWithLibraryPanel(t, "When an admin tries to delete a dashboard with a library panel, it should disconnect the two",
		func(t *testing.T, sc scenarioContext) {
			sc.reqContext.ReplaceAllParams(map[string]string{":uid": sc.initialResult.Result.UID, ":dashboardId": "1"})
			resp := sc.service.connectHandler(sc.reqContext)
			require.Equal(t, 200, resp.Status())

			dashJSON := map[string]interface{}{
				"panels": []interface{}{
					map[string]interface{}{
						"id": int64(1),
						"gridPos": map[string]interface{}{
							"h": 6,
							"w": 6,
							"x": 0,
							"y": 0,
						},
					},
					map[string]interface{}{
						"id": int64(2),
						"gridPos": map[string]interface{}{
							"h": 6,
							"w": 6,
							"x": 6,
							"y": 0,
						},
						"datasource": "${DS_GDEV-TESTDATA}",
						"libraryPanel": map[string]interface{}{
							"uid":  sc.initialResult.Result.UID,
							"name": sc.initialResult.Result.Name,
						},
						"title": "Text - Library Panel",
						"type":  "text",
					},
				},
			}
			dash := models.Dashboard{
				Id:   int64(1),
				Data: simplejson.NewFromAny(dashJSON),
			}

			err := sc.service.DisconnectLibraryPanelsForDashboard(sc.reqContext, &dash)
			require.NoError(t, err)

			sc.reqContext.ReplaceAllParams(map[string]string{":uid": sc.initialResult.Result.UID})
			resp = sc.service.getConnectedDashboardsHandler(sc.reqContext)
			require.Equal(t, 200, resp.Status())

			var dashResult libraryPanelDashboardsResult
			err = json.Unmarshal(resp.Body(), &dashResult)
			require.NoError(t, err)
			require.Empty(t, dashResult.Result)
		})

	scenarioWithLibraryPanel(t, "When an admin tries to delete a dashboard with a library panel without uid, it should fail",
		func(t *testing.T, sc scenarioContext) {
			sc.reqContext.ReplaceAllParams(map[string]string{":uid": sc.initialResult.Result.UID, ":dashboardId": "1"})
			resp := sc.service.connectHandler(sc.reqContext)
			require.Equal(t, 200, resp.Status())

			dashJSON := map[string]interface{}{
				"panels": []interface{}{
					map[string]interface{}{
						"id": int64(1),
						"gridPos": map[string]interface{}{
							"h": 6,
							"w": 6,
							"x": 0,
							"y": 0,
						},
					},
					map[string]interface{}{
						"id": int64(2),
						"gridPos": map[string]interface{}{
							"h": 6,
							"w": 6,
							"x": 6,
							"y": 0,
						},
						"datasource": "${DS_GDEV-TESTDATA}",
						"libraryPanel": map[string]interface{}{
							"name": sc.initialResult.Result.Name,
						},
						"title": "Text - Library Panel",
						"type":  "text",
					},
				},
			}
			dash := models.Dashboard{
				Id:   int64(1),
				Data: simplejson.NewFromAny(dashJSON),
			}

			err := sc.service.DisconnectLibraryPanelsForDashboard(sc.reqContext, &dash)
			require.EqualError(t, err, errLibraryPanelHeaderUIDMissing.Error())
		})
}

type libraryPanel struct {
	ID       int64                  `json:"id"`
	OrgID    int64                  `json:"orgId"`
	FolderID int64                  `json:"folderId"`
	UID      string                 `json:"uid"`
	Name     string                 `json:"name"`
	Model    map[string]interface{} `json:"model"`
	Meta     LibraryPanelDTOMeta    `json:"meta"`
}

type libraryPanelResult struct {
	Result libraryPanel `json:"result"`
}

type libraryPanelsResult struct {
	Result []libraryPanel `json:"result"`
}

type libraryPanelDashboardsResult struct {
	Result []int64 `json:"result"`
}

func overrideLibraryPanelServiceInRegistry(cfg *setting.Cfg) LibraryPanelService {
	lps := LibraryPanelService{
		SQLStore: nil,
		Cfg:      cfg,
	}

	overrideServiceFunc := func(d registry.Descriptor) (*registry.Descriptor, bool) {
		descriptor := registry.Descriptor{
			Name:         "LibraryPanelService",
			Instance:     &lps,
			InitPriority: 0,
		}

		return &descriptor, true
	}

	registry.RegisterOverride(overrideServiceFunc)

	return lps
}

func getCreateCommand(folderID int64, name string) createLibraryPanelCommand {
	command := createLibraryPanelCommand{
		FolderID: folderID,
		Name:     name,
		Model: []byte(`
			{
			  "datasource": "${DS_GDEV-TESTDATA}",
			  "id": 1,
			  "title": "Text - Library Panel",
			  "type": "text"
			}
		`),
	}

	return command
}

type scenarioContext struct {
	ctx           *macaron.Context
	service       *LibraryPanelService
	reqContext    *models.ReqContext
	user          models.SignedInUser
	folder        *models.Folder
	initialResult libraryPanelResult
}

func createFolder(t *testing.T, sc *scenarioContext, cmd *models.CreateFolderCommand) {
	s := dashboards.NewFolderService(sc.user.OrgId, &sc.user)
	err := s.CreateFolder(cmd)
	require.NoError(t, err)
	sc.folder = cmd.Result
}

func updateFolderACL(t *testing.T, sc scenarioContext, roleType models.RoleType, permission models.PermissionType) {
	cmd := models.UpdateDashboardAclCommand{
		DashboardID: sc.folder.Id,
		Items: []*models.DashboardAcl{
			{
				DashboardID: sc.folder.Id,
				Role:        &roleType,
				Permission:  permission,
				Created:     time.Now(),
				Updated:     time.Now(),
			},
		},
	}
	err := bus.Dispatch(&cmd)
	require.NoError(t, err)
}

func validateAndUnMarshalResponse(t *testing.T, resp response.Response) libraryPanelResult {
	require.Equal(t, 200, resp.Status())

	var result = libraryPanelResult{}
	err := json.Unmarshal(resp.Body(), &result)
	require.NoError(t, err)

	return result
}

func scenarioWithLibraryPanel(t *testing.T, desc string, fn func(t *testing.T, sc scenarioContext)) {
	testScenario(t, desc, func(t *testing.T, sc scenarioContext) {
		command := getCreateCommand(sc.folder.Id, "Text - Library Panel")
		resp := sc.service.createHandler(sc.reqContext, command)
		sc.initialResult = validateAndUnMarshalResponse(t, resp)

		fn(t, sc)
	})
}

// testScenario is a wrapper around t.Run performing common setup for library panel tests.
// It takes your real test function as a callback.
func testScenario(t *testing.T, desc string, fn func(t *testing.T, sc scenarioContext)) {
	t.Helper()

	t.Run(desc, func(t *testing.T) {
		t.Cleanup(registry.ClearOverrides)

		ctx := macaron.Context{
			Req: macaron.Request{Request: &http.Request{}},
		}
		orgID := int64(1)
		role := models.ROLE_ADMIN

		cfg := setting.NewCfg()
		// Everything in this service is behind the feature toggle "panelLibrary"
		cfg.FeatureToggles = map[string]bool{"panelLibrary": true}
		// Because the LibraryPanelService is behind a feature toggle, we need to override the service in the registry
		// with a Cfg that contains the feature toggle so migrations are run properly
		service := overrideLibraryPanelServiceInRegistry(cfg)

		// We need to assign SQLStore after the override and migrations are done
		sqlStore := sqlstore.InitTestDB(t)
		service.SQLStore = sqlStore

		user := models.SignedInUser{
			UserId:     1,
			Name:       "Signed In User",
			Login:      "signed_in_user",
			Email:      "signed.in.user@test.com",
			OrgId:      orgID,
			OrgRole:    role,
			LastSeenAt: time.Now(),
		}

		// deliberate difference between signed in user and user in db to make it crystal clear
		// what to expect in the tests
		// In the real world these are identical
		cmd := &models.CreateUserCommand{
			Email: "user.in.db@test.com",
			Name:  "User In DB",
			Login: "user_in_db",
		}
		err := sqlstore.CreateUser(context.Background(), cmd)
		require.NoError(t, err)

		sc := scenarioContext{
			user:    user,
			ctx:     &ctx,
			service: &service,
			reqContext: &models.ReqContext{
				Context:      &ctx,
				SignedInUser: &user,
			},
		}

		folderCmd := models.CreateFolderCommand{
			Uid:   "testFolder",
			Title: "TestFolder",
		}

		createFolder(t, &sc, &folderCmd)

		fn(t, sc)
	})
}

func getCompareOptions() []cmp.Option {
	return []cmp.Option{
		cmp.Transformer("Time", func(in time.Time) int64 {
			return in.UTC().Unix()
		}),
	}
}
