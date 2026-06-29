package repository

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"
	"gorm.io/gorm"

	v1 "shiliu/api/v1"
	"shiliu/internal/model"
	"shiliu/internal/repository"
)

func setupFolderRepositoriesWithDB(t *testing.T) (*gorm.DB, repository.FolderRepository, repository.FeedRepository, repository.ContentItemRepository) {
	t.Helper()

	db := openMigratedTestDB(t)
	logger, _ := newObservedLogger(zapcore.InfoLevel)
	repo := repository.NewRepository(logger, db)
	return db, repository.NewFolderRepository(repo), repository.NewFeedRepository(repo), repository.NewContentItemRepository(repo)
}

func TestFolderRepository_CRUDUniqueAndDeleteClearsFeedFolder(t *testing.T) {
	db, folderRepo, feedRepo, contentRepo := setupFolderRepositoriesWithDB(t)
	ctx := context.Background()
	folder := &model.Folder{Name: "Engineering"}

	require.NoError(t, folderRepo.Create(ctx, folder))
	require.NotZero(t, folder.Id)
	duplicateErr := folderRepo.Create(ctx, &model.Folder{Name: "Engineering"})
	require.Error(t, duplicateErr)
	assert.ErrorIs(t, duplicateErr, gorm.ErrDuplicatedKey)

	folders, err := folderRepo.List(ctx)
	require.NoError(t, err)
	require.Len(t, folders, 1)
	assert.Equal(t, folder.Id, folders[0].Id)
	assert.Equal(t, "Engineering", folders[0].Name)

	require.NoError(t, folderRepo.Rename(ctx, folder.Id, "Research"))
	renamed, err := folderRepo.GetByID(ctx, folder.Id)
	require.NoError(t, err)
	assert.Equal(t, "Research", renamed.Name)

	feed := &model.Feed{FeedURL: "https://example.com/folder-delete.xml", Type: model.FeedTypeRSS, FetchStatus: model.FeedFetchStatusIdle, FolderID: &folder.Id}
	require.NoError(t, feedRepo.Create(ctx, feed))
	item := &model.ContentItem{FeedID: feed.Id, DedupeKey: "folder-delete-item", Type: model.ContentItemTypeText, Title: "Folder delete item", AvailableText: "Folder delete item"}
	require.NoError(t, contentRepo.Create(ctx, item))

	require.NoError(t, folderRepo.Delete(ctx, folder.Id))
	_, err = folderRepo.GetByID(ctx, folder.Id)
	assert.ErrorIs(t, err, v1.ErrNotFound)
	keptFeed, err := feedRepo.GetByID(ctx, feed.Id)
	require.NoError(t, err)
	assert.Nil(t, keptFeed.FolderID)
	keptItem, err := contentRepo.GetByID(ctx, item.Id)
	require.NoError(t, err)
	assert.Equal(t, item.Id, keptItem.Id)
	var residualFeeds int64
	require.NoError(t, db.Model(&model.Feed{}).Where("folder_id = ?", folder.Id).Count(&residualFeeds).Error)
	assert.Zero(t, residualFeeds)

	assert.ErrorIs(t, folderRepo.Rename(ctx, folder.Id, "missing"), v1.ErrNotFound)
	assert.ErrorIs(t, folderRepo.Delete(ctx, folder.Id), v1.ErrNotFound)
}

func TestContentItemRepository_ListFiltersByFolderIDWithAndSemantics(t *testing.T) {
	_, folderRepo, feedRepo, contentRepo := setupFolderRepositoriesWithDB(t)
	ctx := context.Background()
	folder := &model.Folder{Name: "Engineering"}
	require.NoError(t, folderRepo.Create(ctx, folder))

	targetFeed := &model.Feed{FeedURL: "https://example.com/folder-filter-target.xml", Type: model.FeedTypeRSS, FetchStatus: model.FeedFetchStatusIdle, FolderID: &folder.Id}
	otherFolderFeed := &model.Feed{FeedURL: "https://example.com/folder-filter-audio.xml", Type: model.FeedTypePodcast, FetchStatus: model.FeedFetchStatusIdle, FolderID: &folder.Id}
	unfiledFeed := &model.Feed{FeedURL: "https://example.com/folder-filter-unfiled.xml", Type: model.FeedTypeRSS, FetchStatus: model.FeedFetchStatusIdle}
	require.NoError(t, feedRepo.Create(ctx, targetFeed))
	require.NoError(t, feedRepo.Create(ctx, otherFolderFeed))
	require.NoError(t, feedRepo.Create(ctx, unfiledFeed))
	targetItem := &model.ContentItem{FeedID: targetFeed.Id, DedupeKey: "target", Type: model.ContentItemTypeText, Title: "Target", AvailableText: "Target"}
	otherFolderItem := &model.ContentItem{FeedID: otherFolderFeed.Id, DedupeKey: "audio", Type: model.ContentItemTypeAudio, Title: "Audio", AvailableText: "Audio"}
	unfiledItem := &model.ContentItem{FeedID: unfiledFeed.Id, DedupeKey: "unfiled", Type: model.ContentItemTypeText, Title: "Unfiled", AvailableText: "Unfiled"}
	require.NoError(t, contentRepo.Create(ctx, targetItem))
	require.NoError(t, contentRepo.Create(ctx, otherFolderItem))
	require.NoError(t, contentRepo.Create(ctx, unfiledItem))

	textType := model.ContentItemTypeText
	items, total, err := contentRepo.List(ctx, repository.ContentItemListFilter{ContentType: &textType, FolderID: &folder.Id}, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, items, 1)
	assert.Equal(t, targetItem.Id, items[0].Id)
}
