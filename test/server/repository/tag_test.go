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

func setupTagRepository(t *testing.T) repository.TagRepository {
	t.Helper()

	db := openMigratedTestDB(t)
	logger, _ := newObservedLogger(zapcore.InfoLevel)
	repo := repository.NewRepository(logger, db)
	return repository.NewTagRepository(repo)
}

func setupTagContentRepositoriesWithDB(t *testing.T) (*gorm.DB, repository.FeedRepository, repository.ContentItemRepository, repository.TagRepository) {
	t.Helper()

	db := openMigratedTestDB(t)
	logger, _ := newObservedLogger(zapcore.InfoLevel)
	repo := repository.NewRepository(logger, db)
	return db, repository.NewFeedRepository(repo), repository.NewContentItemRepository(repo), repository.NewTagRepository(repo)
}

func TestTagRepository_CreateAndListTags(t *testing.T) {
	tagRepo := setupTagRepository(t)
	ctx := context.Background()
	tag := &model.Tag{Name: "sqlite"}

	require.NoError(t, tagRepo.Create(ctx, tag))
	require.NotZero(t, tag.Id)

	tags, err := tagRepo.List(ctx)
	require.NoError(t, err)
	require.Len(t, tags, 1)
	assert.Equal(t, tag.Id, tags[0].Id)
	assert.Equal(t, "sqlite", tags[0].Name)
}

func TestTagRepository_CreateDuplicateNameFails(t *testing.T) {
	tagRepo := setupTagRepository(t)
	ctx := context.Background()

	require.NoError(t, tagRepo.Create(ctx, &model.Tag{Name: "sqlite"}))
	err := tagRepo.Create(ctx, &model.Tag{Name: "sqlite"})

	require.Error(t, err)
	assert.ErrorIs(t, err, gorm.ErrDuplicatedKey)
}

func TestTagRepository_RenameAndDeleteTag(t *testing.T) {
	tagRepo := setupTagRepository(t)
	ctx := context.Background()
	tag := &model.Tag{Name: "sqlite"}
	require.NoError(t, tagRepo.Create(ctx, tag))

	require.NoError(t, tagRepo.Rename(ctx, tag.Id, "postgres"))
	renamed, err := tagRepo.GetByID(ctx, tag.Id)
	require.NoError(t, err)
	assert.Equal(t, "postgres", renamed.Name)

	require.NoError(t, tagRepo.Delete(ctx, tag.Id))
	_, err = tagRepo.GetByID(ctx, tag.Id)
	assert.ErrorIs(t, err, v1.ErrNotFound)
	assert.ErrorIs(t, tagRepo.Rename(ctx, tag.Id, "missing"), v1.ErrNotFound)
	assert.ErrorIs(t, tagRepo.Delete(ctx, tag.Id), v1.ErrNotFound)
}

func TestContentItemRepository_AssignRemoveAndFilterTags(t *testing.T) {
	db, feedRepo, contentRepo, tagRepo := setupTagContentRepositoriesWithDB(t)
	ctx := context.Background()
	feed := &model.Feed{FeedURL: "https://example.com/tag-assignment.xml", Type: model.FeedTypeRSS, FetchStatus: model.FeedFetchStatusIdle}
	require.NoError(t, feedRepo.Create(ctx, feed))
	item := &model.ContentItem{FeedID: feed.Id, DedupeKey: "tagged-item", Type: model.ContentItemTypeText, Title: "Tagged item", AvailableText: "Tagged item"}
	require.NoError(t, contentRepo.Create(ctx, item))
	sqliteTag := &model.Tag{Name: "sqlite"}
	goTag := &model.Tag{Name: "go"}
	unusedTag := &model.Tag{Name: "unused"}
	require.NoError(t, tagRepo.Create(ctx, sqliteTag))
	require.NoError(t, tagRepo.Create(ctx, goTag))
	require.NoError(t, tagRepo.Create(ctx, unusedTag))

	require.NoError(t, contentRepo.AssignTags(ctx, item.Id, []uint{sqliteTag.Id, goTag.Id}))
	require.NoError(t, contentRepo.AssignTags(ctx, item.Id, []uint{sqliteTag.Id}))
	var relationCount int64
	require.NoError(t, db.Table("content_item_tags").Where("content_item_id = ?", item.Id).Count(&relationCount).Error)
	assert.Equal(t, int64(2), relationCount)

	textType := model.ContentItemTypeText
	filter := repository.ContentItemListFilter{ContentType: &textType, TagID: &sqliteTag.Id}
	tagged, total, err := contentRepo.List(ctx, filter, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, tagged, 1)
	assert.Equal(t, item.Id, tagged[0].Id)

	require.NoError(t, contentRepo.RemoveTags(ctx, item.Id, []uint{sqliteTag.Id, unusedTag.Id}))
	tagged, total, err = contentRepo.List(ctx, filter, 10, 0)
	require.NoError(t, err)
	assert.Zero(t, total)
	assert.Empty(t, tagged)

	filter.TagID = &goTag.Id
	tagged, total, err = contentRepo.List(ctx, filter, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, tagged, 1)
	assert.Equal(t, item.Id, tagged[0].Id)
}

func TestContentItemRepository_AssignTagsRejectsMissingTagID(t *testing.T) {
	db, feedRepo, contentRepo, _ := setupTagContentRepositoriesWithDB(t)
	ctx := context.Background()
	feed := &model.Feed{FeedURL: "https://example.com/missing-tag-assignment.xml", Type: model.FeedTypeRSS, FetchStatus: model.FeedFetchStatusIdle}
	require.NoError(t, feedRepo.Create(ctx, feed))
	item := &model.ContentItem{FeedID: feed.Id, DedupeKey: "missing-tagged-item", Type: model.ContentItemTypeText, Title: "Missing tagged item", AvailableText: "Missing tagged item"}
	require.NoError(t, contentRepo.Create(ctx, item))

	err := contentRepo.AssignTags(ctx, item.Id, []uint{999999})

	assert.ErrorIs(t, err, v1.ErrNotFound)
	var relationCount int64
	require.NoError(t, db.Table("content_item_tags").Where("content_item_id = ?", item.Id).Count(&relationCount).Error)
	assert.Zero(t, relationCount)
}

func TestTagRepository_DeletePreservesContentItemsAndClearsRelations(t *testing.T) {
	db, feedRepo, contentRepo, tagRepo := setupTagContentRepositoriesWithDB(t)
	ctx := context.Background()
	feed := &model.Feed{FeedURL: "https://example.com/tag-delete.xml", Type: model.FeedTypeRSS, FetchStatus: model.FeedFetchStatusIdle}
	require.NoError(t, feedRepo.Create(ctx, feed))
	item := &model.ContentItem{FeedID: feed.Id, DedupeKey: "tag-delete-item", Type: model.ContentItemTypeText, Title: "Tag delete item", AvailableText: "Tag delete item"}
	require.NoError(t, contentRepo.Create(ctx, item))
	tag := &model.Tag{Name: "delete-me"}
	require.NoError(t, tagRepo.Create(ctx, tag))
	require.NoError(t, contentRepo.AssignTags(ctx, item.Id, []uint{tag.Id}))

	require.NoError(t, tagRepo.Delete(ctx, tag.Id))

	_, err := tagRepo.GetByID(ctx, tag.Id)
	assert.ErrorIs(t, err, v1.ErrNotFound)
	kept, err := contentRepo.GetByID(ctx, item.Id)
	require.NoError(t, err)
	assert.Equal(t, item.Id, kept.Id)
	var relationCount int64
	require.NoError(t, db.Table("content_item_tags").Where("content_item_id = ?", item.Id).Count(&relationCount).Error)
	assert.Zero(t, relationCount)
}
